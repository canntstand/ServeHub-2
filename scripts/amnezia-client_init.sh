#!/usr/bin/env bash
set -Eeuo pipefail

CONFIG_DIR=/config
AWG_DIR=/etc/amnezia/amneziawg
declare -a ACTIVE_INTERFACES=()

cleanup() {
    local i
    for ((i=${#ACTIVE_INTERFACES[@]}-1; i>=0; i--)); do
        awg-quick down "${ACTIVE_INTERFACES[$i]}" || true
    done
}

trap cleanup EXIT
trap 'exit 0' INT TERM

install -d -m 0700 "$AWG_DIR"

# Docker restarts keep the container's writable layer. Remove stale configs
# left by a previous failed start before processing the source configs.
while IFS= read -r -d '' stale_conf; do
    stale_name="$(basename "$stale_conf" .conf)"
    awg-quick down "$stale_name" || true
    rm -f -- "$stale_conf"
done < <(find "$AWG_DIR" -maxdepth 1 -type f -name '*.conf' -print0)

while IFS= read -r -d '' src; do
    name="$(basename "$src" .conf)"

    if [[ ! "$name" =~ ^[a-zA-Z0-9_=+.-]{1,15}$ ]]; then
        echo "Invalid interface name: $name" >&2
        exit 1
    fi

    if [[ -e "$AWG_DIR/$name.conf" ]]; then
        echo "Duplicate interface config: $name" >&2
        exit 1
    fi

    install -m 0600 "$src" "$AWG_DIR/$name.conf"

    # DNS is managed on the host by systemd-resolved. Keep the original
    # /config file untouched and skip wg-quick's container-side resolvconf.
    if grep -Eq '^[[:space:]]*[Dd][Nn][Ss][[:space:]]*=' "$AWG_DIR/$name.conf"; then
        sed -i '/^[[:space:]]*[Dd][Nn][Ss][[:space:]]*=/d' "$AWG_DIR/$name.conf"
        echo "Using host systemd-resolved for DNS on interface $name"
    fi

    if ! awg-quick up "$name"; then
        echo "Failed to start interface: $name" >&2
        exit 1
    fi

    ACTIVE_INTERFACES+=("$name")
done < <(find "$CONFIG_DIR" -type f -name '*.conf' -print0)

if (("${#ACTIVE_INTERFACES[@]}" == 0)); then
    echo "No .conf files found in $CONFIG_DIR" >&2
    exit 1
fi

while true; do
    sleep 3600 &
    wait "$!" || true
done
