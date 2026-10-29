#!/usr/bin/env bash
set -u

IFACE="awg0"
DNS_SERVER="10.8.0.1"

LAST_IFINDEX=""
LAST_SYNC=0

while true; do
    if [[ -r "/sys/class/net/${IFACE}/ifindex" ]]; then
        CURRENT_IFINDEX="$(cat "/sys/class/net/${IFACE}/ifindex")"

        if [[ "$CURRENT_IFINDEX" != "$LAST_IFINDEX" ]] ||
           (( SECONDS - LAST_SYNC >= 30 )); then

            if /usr/bin/resolvectl dns "$IFACE" "$DNS_SERVER" &&
               /usr/bin/resolvectl domain "$IFACE" '~.' &&
               /usr/bin/resolvectl default-route "$IFACE" yes; then

                if [[ "$CURRENT_IFINDEX" != "$LAST_IFINDEX" ]]; then
                    /usr/bin/logger -t servehub-amnezia-dns \
                        "DNS configured for ${IFACE}, ifindex=${CURRENT_IFINDEX}"
                fi

                LAST_IFINDEX="$CURRENT_IFINDEX"
                LAST_SYNC=$SECONDS
            else
                /usr/bin/logger -t servehub-amnezia-dns \
                    "Failed to configure DNS for ${IFACE}"
                LAST_IFINDEX=""
            fi
        fi
    else
        LAST_IFINDEX=""
    fi

    sleep 2
done
