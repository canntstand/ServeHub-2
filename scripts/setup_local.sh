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
enable_registration_without_verification: true
EOF

echo "Файл homeserver.yaml успешно сгенерирован."

OS_TYPE="$(uname -s)"
if [[ "$OS_TYPE" != *"MINGW"* && "$OS_TYPE" != *"MSYS"* ]]; then
    sudo chown -R 991:991 matrix/data/
fi

sudo mkdir -p /home/${HOME}/NextcloudData
sudo chown -R 33:33 /home/${HOME}/NextcloudData

echo "Создаем хеш для vaultwarden..."
if ! command -v argon2 &> /dev/null; then
    echo "Установите argon2 через ваш менеджер пакетов (например, apt install argon2)"
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

if [ -z "$WEBNAMES_APIKEY" ]; then
    echo "ОШИБКА: Переменная WEBNAMES_APIKEY не задана в .env"
    exit 1
fi

if [ ! -d "./certbot-dns-webnames" ]; then
    echo "Клонируем официальный репозиторий certbot-dns-webnames..."
    git clone https://github.com/regtime-ltd/certbot-dns-webnames.git ./certbot-dns-webnames
fi

echo "Скачиваем конфигурационный файл конфигурации зоны с Webnames API..."
curl -s -k "https://www.webnames.ru/scripts/json_domain_zone_manager.pl?action=get_config_certbot&domain=${SYNAPSE_SERVER_NAME}&apikey=${WEBNAMES_APIKEY}" -o ./certbot-dns-webnames/config.sh


chmod +x ./certbot-dns-webnames/*.sh

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

echo "Настройка прав доступа для папок с данными..."
sudo chmod -R 777 ./grafana/ ./matrix/ ./nextcloud/ ./vaultwarden/ ./synapse/

echo "Запуск Docker-контейнеров (исключая Certbot)..."
MAIN_SERVICES="synapse synapse_db nginx nginx_exporter frpc navidrome audiobookshelf nextcloud nextcloud_db nextcloud_configure vaultwarden vaultwarden_db prometheus_init prometheus grafana node_exporter cadvisor portainer"

docker compose -f docker-compose.local.yaml up -d $MAIN_SERVICES

if [ "$NEED_REAL_CERT" = true ]; then
    echo "Запуск официального Certbot..."
    docker compose -f docker-compose.local.yaml build certbot

    echo "Удаляем временные заглушки перед получением реальных сертификатов..."
    rm -rf "${CERT_DIR:?}"/*
    docker compose -f docker-compose.local.yaml run --rm certbot
    
    echo "Перезагрузка конфигурации Nginx..."
    docker compose -f docker-compose.local.yaml exec nginx nginx -s reload
fi

echo "Ожидание запуска Synapse (15 секунд)..."
sleep 15

chmod +x scripts/create_admin.sh
./scripts/create_admin.sh

echo "Все службы успешно запущены и изолированы внутри Docker!"