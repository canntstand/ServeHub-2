#!/usr/bin/env bash
set -uo pipefail

METRICS_DIR="${SERVEHUB_BORGMATIC_METRICS_DIR:-/var/lib/node_exporter/textfile_collector}"
METRICS_FILE="${METRICS_DIR%/}/borgmatic.prom"

if ! mkdir -p "$METRICS_DIR"; then
    echo "[ERROR] Cannot create metrics directory: $METRICS_DIR" >&2
    exit 70
fi

LAST_SUCCESS=0
if [[ -r "$METRICS_FILE" ]]; then
    candidate="$(awk '$1 == "servehub_borgmatic_backup_last_success_timestamp_seconds" { print $2; exit }' "$METRICS_FILE")"
    if [[ "$candidate" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
        LAST_SUCCESS="$candidate"
    fi
fi

/usr/local/bin/borgmatic --config /etc/borgmatic/config.yaml --verbosity -1
borgmatic_status=$?
finished_at="$(date +%s)"

if [[ "$borgmatic_status" -eq 0 ]]; then
    LAST_SUCCESS="$finished_at"
fi

if ! temporary_file="$(mktemp "${METRICS_FILE}.tmp.XXXXXX")"; then
    echo "[ERROR] Cannot create temporary Prometheus metrics file." >&2
    exit 70
fi

if ! cat > "$temporary_file" <<EOF
# HELP servehub_borgmatic_backup_last_run_success Whether the last Borgmatic run succeeded.
# TYPE servehub_borgmatic_backup_last_run_success gauge
servehub_borgmatic_backup_last_run_success $([[ "$borgmatic_status" -eq 0 ]] && printf 1 || printf 0)
# HELP servehub_borgmatic_backup_last_run_timestamp_seconds Unix timestamp of the last Borgmatic run.
# TYPE servehub_borgmatic_backup_last_run_timestamp_seconds gauge
servehub_borgmatic_backup_last_run_timestamp_seconds $finished_at
# HELP servehub_borgmatic_backup_last_success_timestamp_seconds Unix timestamp of the last successful Borgmatic backup.
# TYPE servehub_borgmatic_backup_last_success_timestamp_seconds gauge
servehub_borgmatic_backup_last_success_timestamp_seconds $LAST_SUCCESS
EOF
then
    rm -f "$temporary_file"
    echo "[ERROR] Cannot write Prometheus metrics." >&2
    exit 70
fi

if ! chmod 0644 "$temporary_file" || ! mv -f "$temporary_file" "$METRICS_FILE"; then
    rm -f "$temporary_file"
    echo "[ERROR] Cannot publish Prometheus metrics." >&2
    exit 70
fi

exit "$borgmatic_status"
