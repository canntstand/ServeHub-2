#!/bin/bash
if [ -f .env ]; then
    export $(grep -v '^#' .env | xargs)
fi

echo "Регистрация администратора Synapse из .env..."

docker compose -f docker-compose.local.yaml exec -it synapse register_new_matrix_user \
    -c /data/homeserver.yaml \
    -u "$ADMIN_USER" \
    -p "$ADMIN_PASSWORD" \
    --admin \
    http://127.0.0.1:8008

echo "Пользователь $ADMIN_USER зарегистрирован как администратор."