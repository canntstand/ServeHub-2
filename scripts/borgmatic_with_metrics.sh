#!/usr/bin/env bash
set -uo pipefail

METRICS_DIR="${SERVEHUB_BORGMATIC_METRICS_DIR:-/var/lib/node_exporter/textfile_collector}"
METRICS_FILE="${METRICS_DIR%/}/borgmatic.prom"
BORGMATIC_CONFIG="${SERVEHUB_BORGMATIC_CONFIG:-/etc/borgmatic/config.yaml}"
BORG_REPOSITORY="${SERVEHUB_BORG_REPOSITORY:-}"
RCLONE_CONFIG="${SERVEHUB_RCLONE_CONFIG:-/etc/servehub/rclone.conf}"
RCLONE_REMOTE_PATH="${SERVEHUB_RCLONE_REMOTE_PATH:-}"
COMPOSE_DIR="${SERVEHUB_COMPOSE_DIR:-}"
COMPOSE_FILE="${SERVEHUB_COMPOSE_FILE:-}"
KEEP_SERVICES_RAW="${SERVEHUB_BACKUP_KEEP_SERVICES:-}"
BACKUP_ROLE="${SERVEHUB_BACKUP_ROLE:-unknown}"

if ! mkdir -p "$METRICS_DIR"; then
    echo "[ERROR] Cannot create metrics directory: $METRICS_DIR" >&2
    exit 70
fi

previous_metric() {
    local metric="$1"
    local value=""
    if [[ -r "$METRICS_FILE" ]]; then
        value="$(awk -v metric="$metric" '$1 == metric { print $2; exit }' "$METRICS_FILE")"
    fi
    if [[ "$value" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
        printf '%s' "$value"
    else
        printf '0'
    fi
}

LAST_BORG_SUCCESS="$(previous_metric servehub_borgmatic_backup_last_success_timestamp_seconds)"
LAST_OFFSITE_SUCCESS="$(previous_metric servehub_offsite_backup_last_success_timestamp_seconds)"
LAST_PIPELINE_SUCCESS="$(previous_metric servehub_backup_pipeline_last_success_timestamp_seconds)"
LAST_RUN_TIMESTAMP=0
BORG_STATUS=1
OFFSITE_STATUS=0
STACK_RESTORE_STATUS=1
PIPELINE_STATUS=1
STACK_START_NEEDED=0
METRICS_PUBLISHED=0
RUNNING_SERVICES=()
STOP_SERVICES=()
LOCK_FILE="/run/lock/servehub-borgmatic-${BACKUP_ROLE}.lock"

exec 9>"$LOCK_FILE"
if ! flock -n 9; then
    echo "[ERROR] Another ServeHub backup pipeline is already running for role '$BACKUP_ROLE'." >&2
    exit 75
fi

restore_services() {
    if (( STACK_START_NEEDED == 0 )); then
        return 0
    fi

    if [[ -z "$COMPOSE_DIR" || -z "$COMPOSE_FILE" || ${#RUNNING_SERVICES[@]} -eq 0 ]]; then
        STACK_START_NEEDED=0
        STACK_RESTORE_STATUS=1
        return 0
    fi

    local attempt
    for attempt in 1 2 3; do
        if (cd "$COMPOSE_DIR" && docker compose -f "$COMPOSE_FILE" up -d --no-recreate "${RUNNING_SERVICES[@]}"); then
            STACK_START_NEEDED=0
            STACK_RESTORE_STATUS=1
            echo "[INFO] Previously running Compose services are back up." >&2
            return 0
        fi
        echo "[WARN] Compose restoration attempt $attempt/3 failed." >&2
        sleep 5
    done

    STACK_RESTORE_STATUS=0
    echo "[ERROR] Failed to restart previously running Compose services after three attempts." >&2
    return 1
}

on_exit() {
    local original_status=$?
    local retried_restore=0
    trap - EXIT

    if (( STACK_START_NEEDED )); then
        retried_restore=1
        restore_services >/dev/null 2>&1 || true
    fi

    # If the safety-net retry restored services after metrics were published,
    # reflect that recovery immediately rather than leaving a stale restore alert.
    if (( retried_restore && METRICS_PUBLISHED )); then
        LAST_RUN_TIMESTAMP="$(date +%s)"
        if (( BORG_STATUS == 0 && OFFSITE_STATUS == 1 && STACK_RESTORE_STATUS == 1 )); then
            PIPELINE_STATUS=0
            LAST_PIPELINE_SUCCESS="$LAST_RUN_TIMESTAMP"
        else
            PIPELINE_STATUS=1
        fi

        local temporary_file
        if temporary_file="$(mktemp "${METRICS_FILE}.tmp.XXXXXX")"; then
            if awk \
                -v stack="$STACK_RESTORE_STATUS" \
                -v pipeline="$PIPELINE_STATUS" \
                -v last_success="$LAST_PIPELINE_SUCCESS" '
                $1 == "servehub_backup_stack_restore_success" { $2 = stack }
                $1 == "servehub_backup_pipeline_last_run_success" { $2 = pipeline }
                $1 == "servehub_backup_pipeline_last_success_timestamp_seconds" { $2 = last_success }
                { print }
            ' "$METRICS_FILE" > "$temporary_file" \
                && chmod 0644 "$temporary_file" \
                && mv -f "$temporary_file" "$METRICS_FILE"; then
                original_status="$PIPELINE_STATUS"
            else
                rm -f "$temporary_file"
                echo "[ERROR] Could not update backup metrics after the final service restoration retry." >&2
                original_status=70
            fi
        else
            echo "[ERROR] Could not create temporary metrics file after service restoration retry." >&2
            original_status=70
        fi
    fi

    exit "$original_status"
}
trap on_exit EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

if [[ -z "$BORG_REPOSITORY" || -z "$RCLONE_REMOTE_PATH" || -z "$COMPOSE_DIR" || -z "$COMPOSE_FILE" ]]; then
    echo "[ERROR] Borg repository, S3 destination and Compose settings must be configured by Ansible." >&2
else
    RUNNING_SERVICES_OUTPUT="$(cd "$COMPOSE_DIR" && docker compose -f "$COMPOSE_FILE" ps --services --filter status=running)"
    DISCOVERY_STATUS=$?
    if (( DISCOVERY_STATUS != 0 )); then
        echo "[ERROR] Failed to inspect running Compose services; refusing to back up potentially changing data." >&2
    else
        while IFS= read -r service; do
            [[ -n "$service" ]] && RUNNING_SERVICES+=("$service")
        done <<< "$RUNNING_SERVICES_OUTPUT"

        read -r -a KEEP_SERVICES <<< "$KEEP_SERVICES_RAW"
        for service in "${RUNNING_SERVICES[@]}"; do
            keep=0
            for keep_service in "${KEEP_SERVICES[@]}"; do
                if [[ "$service" == "$keep_service" ]]; then
                    keep=1
                    break
                fi
            done
            if (( keep == 0 )); then
                STOP_SERVICES+=("$service")
            fi
        done

        if (( ${#STOP_SERVICES[@]} > 0 )); then
            STACK_START_NEEDED=1
            if (cd "$COMPOSE_DIR" && docker compose -f "$COMPOSE_FILE" stop "${STOP_SERVICES[@]}"); then
                echo "[INFO] Stateful Compose services are stopped; network and metrics dependencies remain available." >&2
                /usr/local/bin/borgmatic --config "$BORGMATIC_CONFIG" --verbosity -1
                BORG_STATUS=$?
            else
                echo "[ERROR] Failed to stop the backup services; refusing to back up potentially changing data." >&2
            fi
            restore_services || true
        else
            echo "[WARN] No services required stopping; running Borgmatic against the current on-disk data." >&2
            /usr/local/bin/borgmatic --config "$BORGMATIC_CONFIG" --verbosity -1
            BORG_STATUS=$?
        fi

        # Borg has released its repository lock before the S3 mirror is updated.
        if (( BORG_STATUS == 0 )); then
            if /usr/bin/rclone --config "$RCLONE_CONFIG" sync \
                --fast-list --transfers 4 --checkers 8 --stats 30s --log-level INFO \
                "$BORG_REPOSITORY" "$RCLONE_REMOTE_PATH"; then
                if /usr/bin/rclone --config "$RCLONE_CONFIG" check \
                    --one-way --size-only \
                    "$BORG_REPOSITORY" "$RCLONE_REMOTE_PATH"; then
                    OFFSITE_STATUS=1
                else
                    echo "[ERROR] rclone size verification of the off-site copy failed." >&2
                fi
            else
                echo "[ERROR] rclone synchronization to the off-site S3 repository failed." >&2
            fi
        else
            echo "[ERROR] Borgmatic failed; skipping off-site synchronization." >&2
        fi
    fi
fi

LAST_RUN_TIMESTAMP="$(date +%s)"
if (( BORG_STATUS == 0 )); then
    LAST_BORG_SUCCESS="$LAST_RUN_TIMESTAMP"
fi
if (( OFFSITE_STATUS == 1 )); then
    LAST_OFFSITE_SUCCESS="$LAST_RUN_TIMESTAMP"
fi
if (( BORG_STATUS == 0 && OFFSITE_STATUS == 1 && STACK_RESTORE_STATUS == 1 )); then
    PIPELINE_STATUS=0
    LAST_PIPELINE_SUCCESS="$LAST_RUN_TIMESTAMP"
else
    PIPELINE_STATUS=1
fi

if ! temporary_file="$(mktemp "${METRICS_FILE}.tmp.XXXXXX")"; then
    echo "[ERROR] Cannot create temporary Prometheus metrics file." >&2
    exit 70
fi

if ! cat > "$temporary_file" <<EOF
# HELP servehub_borgmatic_backup_last_run_success Whether the most recent local Borgmatic backup succeeded.
# TYPE servehub_borgmatic_backup_last_run_success gauge
servehub_borgmatic_backup_last_run_success $([[ "$BORG_STATUS" -eq 0 ]] && printf 1 || printf 0)
# HELP servehub_borgmatic_backup_last_run_timestamp_seconds Unix timestamp of the last local Borg backup attempt.
# TYPE servehub_borgmatic_backup_last_run_timestamp_seconds gauge
servehub_borgmatic_backup_last_run_timestamp_seconds $LAST_RUN_TIMESTAMP
# HELP servehub_borgmatic_backup_last_success_timestamp_seconds Unix timestamp of the last successful local Borg backup.
# TYPE servehub_borgmatic_backup_last_success_timestamp_seconds gauge
servehub_borgmatic_backup_last_success_timestamp_seconds $LAST_BORG_SUCCESS
# HELP servehub_offsite_backup_last_run_success Whether the most recent rclone synchronization and size verification succeeded.
# TYPE servehub_offsite_backup_last_run_success gauge
servehub_offsite_backup_last_run_success $OFFSITE_STATUS
# HELP servehub_offsite_backup_last_run_timestamp_seconds Unix timestamp of the most recent off-site synchronization attempt.
# TYPE servehub_offsite_backup_last_run_timestamp_seconds gauge
servehub_offsite_backup_last_run_timestamp_seconds $LAST_RUN_TIMESTAMP
# HELP servehub_offsite_backup_last_success_timestamp_seconds Unix timestamp of the most recent successful off-site synchronization and verification.
# TYPE servehub_offsite_backup_last_success_timestamp_seconds gauge
servehub_offsite_backup_last_success_timestamp_seconds $LAST_OFFSITE_SUCCESS
# HELP servehub_backup_stack_restore_success Whether the previously running Compose services were restored after backup.
# TYPE servehub_backup_stack_restore_success gauge
servehub_backup_stack_restore_success $STACK_RESTORE_STATUS
# HELP servehub_backup_pipeline_last_run_success Whether local backup, S3 sync, and Compose restoration all succeeded.
# TYPE servehub_backup_pipeline_last_run_success gauge
servehub_backup_pipeline_last_run_success $PIPELINE_STATUS
# HELP servehub_backup_pipeline_last_success_timestamp_seconds Unix timestamp of the last fully successful backup pipeline.
# TYPE servehub_backup_pipeline_last_success_timestamp_seconds gauge
servehub_backup_pipeline_last_success_timestamp_seconds $LAST_PIPELINE_SUCCESS
EOF
then
    rm -f "$temporary_file"
    echo "[ERROR] Cannot write temporary Prometheus metrics file." >&2
    exit 70
fi

if ! chmod 0644 "$temporary_file" || ! mv -f "$temporary_file" "$METRICS_FILE"; then
    rm -f "$temporary_file"
    echo "[ERROR] Cannot publish Prometheus metrics." >&2
    exit 70
fi
METRICS_PUBLISHED=1

exit "$PIPELINE_STATUS"
