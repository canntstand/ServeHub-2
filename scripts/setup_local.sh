#!/bin/bash
set -e

if [ -f .env ]; then
    export $(grep -v '^#' .env | xargs)
else
    echo "ОШИБКА: Файл .env не найден в текущей директории!"
    exit 1
fi

if [ -z "$SECRET_VAULTWARDEN_PASSWORD" ]; then
    echo "ОШИБКА: Переменная SECRET_VAULTWARDEN_PASSWORD не задана в .env"
    exit 1
fi

if [ -z "$SYNAPSE_SERVER_NAME" ]; then
    echo "Ошибка: переменная SYNAPSE_SERVER_NAME не задана в .env"
    exit 1
fi

OS_TYPE="$(uname -s)"

if [[ "$OS_TYPE" == *"MINGW"* || "$OS_TYPE" == *"MSYS"* ]]; then
    echo "Определена ОС: Windows (Git Bash)"
    MSYS_NO_PATHCONV=1 docker run -it --rm \
        -v "//$(pwd)/matrix/data:/data" \
        -e SYNAPSE_SERVER_NAME=${SYNAPSE_SERVER_NAME} \
        -e SYNAPSE_REPORT_STATS=no \
        matrixdotorg/synapse:v1.152.1 generate
else
    echo "Определена ОС: Linux"
    docker run -it --rm \
        -v "$(pwd)/matrix/data:/data" \
        -e SYNAPSE_SERVER_NAME=${SYNAPSE_SERVER_NAME} \
        -e SYNAPSE_REPORT_STATS=no \
        matrixdotorg/synapse:v1.152.1 generate
fi

CONFIG_PATH="matrix/data/homeserver.yaml"
echo "Полная перезапись конфигурации homeserver.yaml..."

sudo tee "$CONFIG_PATH" > /dev/null <<EOF
server_name: "${SYNAPSE_SERVER_NAME}"
pid_file: /data/homeserver.pid

listeners:
  - port: 8008
    resources:
      - compress: false
        names:
          - client
          - federation
    tls: false
    type: http
    x_forwarded: true

database:
  name: psycopg2
  args:
    user: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    database: ${POSTGRES_DB_SYNAPSE}
    host: synapse_db
    cp_min: 5
    cp_max: 10

log_config: "/data/${SYNAPSE_SERVER_NAME}.log.config"
media_store_path: /data/media_store
registration_shared_secret: "${SYNAPSE_REGISTRATION_SHARED_SECRET}"
report_stats: false
macaroon_secret_key: "${SYNAPSE_MACAROON_SECRET_KEY}"
form_secret: "${SYNAPSE_FORM_SECRET}"
signing_key_path: "/data/${SYNAPSE_SERVER_NAME}.signing.key"

trusted_key_servers:
  - server_name: "matrix.org"

enable_registration: true
enable_registration_captcha: true
recaptcha_siteverify_api: "https://www.google.com/recaptcha/api/siteverify"
recaptcha_public_key: "${RECAPTCHA_PUBLIC_KEY}"
recaptcha_private_key: "${RECAPTCHA_PRIVATE_KEY}"
EOF

echo "Файл homeserver.yaml успешно сгенерирован."

OS_TYPE="$(uname -s)"
if [[ "$OS_TYPE" != *"MINGW"* && "$OS_TYPE" != *"MSYS"* ]]; then
    sudo chown -R 991:991 matrix/data/
fi

sudo mkdir -p /home/r9888/NextcloudData
sudo chown -R 33:33 /home/r9888/NextcloudData

echo "Создаем хеш для vaultwarden..."
if ! command -v argon2 &> /dev/null; then
    echo "ОШИБКА: утилита argon2 не найдена. Установите её: sudo pacman -S argon2"
    exit 1
fi

if [ -f ./vaultwarden/data/config.json ]; then
    echo "Удаляем старый config.json для применения нового ADMIN_TOKEN..."
    rm -f ./vaultwarden/data/config.json
fi

SALT=$(openssl rand -hex 8)
HASH_TOKEN=$(echo -n "$SECRET_VAULTWARDEN_PASSWORD" | argon2 "$SALT" -e -id -k 19456 -t 2 -p 1 | sed 's/\$/\$\$/g')

sed -i '/^VAULTWARDEN_ADMIN_HASH=/d' .env

echo "VAULTWARDEN_ADMIN_HASH=${HASH_TOKEN}" >> .env
echo "Хэш успешно сгенерирован и добавлен в .env"

CERT_DIR="./certs/live/${SYNAPSE_SERVER_NAME}"

if [ ! -f "${CERT_DIR}/fullchain.pem" ]; then
    echo "Сертификаты не найдены. Создаем временные сертификаты для запуска Nginx..."

    mkdir -p "${CERT_DIR}"

    openssl req -x509 -nodes -days 1 \
        -newkey rsa:2048 \
        -keyout "${CERT_DIR}/privkey.pem" \
        -out "${CERT_DIR}/fullchain.pem" \
        -subj "/CN=localhost" > /dev/null 2>&1
    
    NEED_REAL_CERT=true
else
    echo "Валидные SSL-сертификаты уже существуют."
    NEED_REAL_CERT=false
fi

echo "Запуск Docker-контейнеров..."
docker compose -f docker-compose.local.yaml up -d

if [ "$NEED_REAL_CERT" = true ]; then
    echo "Запуск Certbot для получения реальных SSL-сертификатов..."
    
    docker compose -f docker-compose.local.yaml run --rm certbot
    
    echo "Перезагрузка конфигурации Nginx для применения реальных сертификатов..."
    docker compose -f docker-compose.local.yaml exec nginx_proxy nginx -s reload
fi

echo "Ожидание запуска Synapse (15 секунд)..."
sleep 15

chmod +x scripts/create_admin.sh
./scripts/create_admin.sh

echo "Все службы успешно запущены и изолированы внутри Docker!"