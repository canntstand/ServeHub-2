#!/bin/bash
set -e

echo "Проверка системы и установка зависимостей..."

if [ -f /etc/os-release ]; then
    . /etc/os-release
    DISTRO=$ID
else
    echo "Ошибка: Не удалось определить дистрибутив."
    exit 1
fi

if [[ "$DISTRO" == "arch" ]]; then
    sudo pacman -Syu --noconfirm argon2 openssl
elif [[ "$DISTRO" == "ubuntu" || "$DISTRO" == "debian" || "$DISTRO" == "pop" || "$DISTRO" == "mint" ]]; then
    sudo apt update
    sudo apt install -y argon2 openssl
elif [[ "$DISTRO" == "centos" || "$DISTRO" == "rhel" || "$DISTRO" == "rocky" || "$DISTRO" == "almalinux" ]]; then
    sudo dnf install -y epel-release
    sudo dnf install -y argon2 openssl
else
    echo "Дистрибутив '$DISTRO' не поддерживается этим скриптом."
    exit 1
fi

echo "Проверка автозапуска Docker..."
if ! systemctl is-enabled --quiet docker; then
    echo "Включаю автозапуск Docker..."
    sudo systemctl enable docker
fi

sudo systemctl start docker

if [ -f .env ]; then
    export $(grep -v '^#' .env | xargs)
else
    echo "ОШИБКА: Файл .env не найден!"
    exit 1
fi


if [ -z "$SECRET_VAULTWARDEN_PASSWORD" ] || [ -z "$SYNAPSE_SERVER_NAME" ] || [ -z "$WEBNAMES_APIKEY" ]; then
    echo "ОШИБКА: Не заданы обязательные переменные в .env"
    exit 1
fi


echo "Подготовка директорий..."
DIRS=("./matrix/data" "./grafana/data" "./nextcloud/data" "./vaultwarden/data" "./synapse/data" "./navidrome/data" "./audiobookshelf/data" "./certs" "./prometheus/data" "./nginx/templates" "./matrix_alertmanager")
for dir in "${DIRS[@]}"; do
    mkdir -p "$dir"
done

sudo chmod -R 777 ./grafana ./matrix ./nextcloud ./vaultwarden ./synapse ./navidrome ./audiobookshelf ./certs

echo "Генерация конфигурации Synapse..."
docker run -it --rm \
    -v "$(pwd)/matrix/data:/data" \
    -e SYNAPSE_SERVER_NAME=${SYNAPSE_SERVER_NAME} \
    -e SYNAPSE_REPORT_STATS=no \
    matrixdotorg/synapse:v1.152.1 generate

CONFIG_PATH="matrix/data/homeserver.yaml"
sudo tee "$CONFIG_PATH" > /dev/null <<EOF
server_name: "${SYNAPSE_SERVER_NAME}"
pid_file: /data/homeserver.pid
listeners:
  - port: 8008
    resources:
      - compress: false
        names: [client, federation]
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

sudo chown -R 991:991 matrix/data/

echo "Генерация хэша Vaultwarden..."
SALT=$(openssl rand -hex 8)
HASH_TOKEN=$(echo -n "$SECRET_VAULTWARDEN_PASSWORD" | argon2 "$SALT" -e -id -k 19456 -t 2 -p 1 | sed 's/\$/\$\$/g')
sed -i '/^VAULTWARDEN_ADMIN_HASH=/d' .env
echo "VAULTWARDEN_ADMIN_HASH=${HASH_TOKEN}" >> .env

if [ ! -d "./certbot-dns-webnames" ]; then
    git clone https://github.com/regtime-ltd/certbot-dns-webnames.git ./certbot-dns-webnames
fi
curl -s -k "https://www.webnames.ru/scripts/json_domain_zone_manager.pl?action=get_config_certbot&domain=${SYNAPSE_SERVER_NAME}&apikey=${WEBNAMES_APIKEY}" -o ./certbot-dns-webnames/config.sh
chmod +x ./certbot-dns-webnames/*.sh

CERT_DIR="./certs/live/${SYNAPSE_SERVER_NAME}"
if [ ! -f "${CERT_DIR}/fullchain.pem" ]; then
    mkdir -p "${CERT_DIR}"
    openssl req -x509 -nodes -days 1 -newkey rsa:2048 -keyout "${CERT_DIR}/privkey.pem" -out "${CERT_DIR}/fullchain.pem" -subj "/CN=localhost"
    NEED_REAL_CERT=true
else
    NEED_REAL_CERT=false
fi

echo "Запуск инфраструктуры настройки..."
docker compose -f docker-compose.local.yaml up -d monitoring_configure

echo "Ожидание завершения настройки..."
until [ "$(docker inspect -f '{{.State.ExitCode}}' monitoring_configure 2>/dev/null)" == "0" ]; do
    EXIT_CODE=$(docker inspect -f '{{.State.ExitCode}}' monitoring_configure 2>/dev/null)
    if [ "$EXIT_CODE" != "0" ] && [ "$EXIT_CODE" != "" ] && [ "$EXIT_CODE" != "-1" ]; then
        echo "ОШИБКА: Контейнер monitoring_configure завершился с ошибкой (Код: $EXIT_CODE)!"
        echo "Логи контейнера:"
        docker compose -f docker-compose.local.yaml logs monitoring_configure
        exit 1
    fi

    echo "Жду завершения настройки мониторинга..."
    sleep 3
done

echo "Запуск остальных сервисов..."
MAIN_SERVICES="synapse synapse_db nginx nginx_exporter navidrome audiobookshelf nextcloud nextcloud_db nextcloud_configure vaultwarden vaultwarden_db prometheus_init prometheus grafana node_exporter cadvisor portainer alertmanager matrix_alertmanager"
docker compose -f docker-compose.local.yaml up -d $MAIN_SERVICES

if [ "$NEED_REAL_CERT" = true ]; then
    echo "Получение реального сертификата..."
    docker compose -f docker-compose.local.yaml build certbot
    rm -rf "${CERT_DIR:?}"/*
    docker compose -f docker-compose.local.yaml run --rm certbot
    docker compose -f docker-compose.local.yaml exec nginx nginx -s reload
fi

sleep 15
chmod +x scripts/create_admin.sh
./scripts/create_admin.sh

echo "Все службы успешно запущены!"