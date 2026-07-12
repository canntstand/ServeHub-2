#!/bin/sh
set -e

SSH_DIR="/root/.ssh"
PRIVATE_KEY="$SSH_DIR/id_ed25519"
PUBLIC_KEY="$SSH_DIR/id_ed25519.pub"
SECRETS_FILE="ansible/vars/secrets.yml"

mkdir -p "$SSH_DIR"
chmod 700 "$SSH_DIR"

if [ -f "$SECRETS_FILE" ]; then
    echo "==> Чтение SSH-ключей из $SECRETS_FILE..."
    
    if grep -q "^ssh_private_key:[[:space:]]*|" "$SECRETS_FILE"; then
        awk '/^ssh_private_key:/ {p=1; next} /^[^[:space:]]/ {p=0} p {print substr($0, 5)}' "$SECRETS_FILE" > "$PRIVATE_KEY"

        tr -d '\r' < "$PRIVATE_KEY" > "${PRIVATE_KEY}.tmp" && mv "${PRIVATE_KEY}.tmp" "$PRIVATE_KEY"

        chmod 600 "$PRIVATE_KEY"
        echo "==> Приватный SSH-ключ успешно импортирован (Многострочный блок YAML)"
        
        SSH_PUB_EXTRACTED=$(sed -n 's/^ssh_public_key:[[:space:]]*"\(.*\)"/\1/p' "$SECRETS_FILE")
        if [ -n "$SSH_PUB_EXTRACTED" ]; then
            echo "$SSH_PUB_EXTRACTED" > "$PUBLIC_KEY"
            chmod 644 "$PUBLIC_KEY"
            echo "==> Публичный SSH-ключ успешно импортирован"
        fi

    else
        SSH_PRIV_EXTRACTED=$(sed -n 's/^ssh_private_key:[[:space:]]*"\(.*\)"/\1/p' "$SECRETS_FILE")
        SSH_PUB_EXTRACTED=$(sed -n 's/^ssh_public_key:[[:space:]]*"\(.*\)"/\1/p' "$SECRETS_FILE")

        if [ -n "$SSH_PRIV_EXTRACTED" ]; then
            echo "$SSH_PRIV_EXTRACTED" | sed 's/\\n/\n/g' > "$PRIVATE_KEY"
            chmod 600 "$PRIVATE_KEY"
            echo "==> Приватный SSH-ключ успешно импортирован (Строка с кавычками)"
            
            if [ -n "$SSH_PUB_EXTRACTED" ]; then
                echo "$SSH_PUB_EXTRACTED" > "$PUBLIC_KEY"
                chmod 644 "$PUBLIC_KEY"
                echo "==> Публичный SSH-ключ успешно импортирован"
            fi
        fi
    fi
fi

if [ ! -f "$PRIVATE_KEY" ]; then
    echo "==> Ошибка: SSH-ключ не найден ни в secrets.yml, ни в контейнере!"
    exit 1
else
    echo "==> Использование существующего/импортированного SSH-ключа."
fi

echo "StrictHostKeyChecking no" > "$SSH_DIR/config"
chmod 600 "$SSH_DIR/config"

echo """
[defaults]
deprecation_warnings = False
command_warnings = False
action_warnings = False
display_skipped_hosts = no
interpreter_python = auto_silent""" >> /ansible/ansible.cfg

exec "$@"