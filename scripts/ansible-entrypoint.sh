#!/bin/sh
set -e

SSH_DIR="/root/.ssh"
PRIVATE_KEY="$SSH_DIR/id_ed25519"
PUBLIC_KEY="$SSH_DIR/id_ed25519.pub"

mkdir -p "$SSH_DIR"
chmod 700 "$SSH_DIR"


if [ ! -f "$PRIVATE_KEY" ]; then
    echo "==> SSH-ключ не найден. Запускаю автоматическую генерацию..."
    ssh-keygen -t ed25519 -N "" -f "$PRIVATE_KEY" -C "servehub-auto@internal"
    chmod 600 "$PRIVATE_KEY"
    chmod 644 "$PUBLIC_KEY"
    echo "==> Ключ успешно сгенерирован!"
else
    echo "==> Используется существующий SSH-ключ."
fi

echo "StrictHostKeyChecking no" > "$SSH_DIR/config"
chmod 600 "$SSH_DIR/config"

exec "$@"