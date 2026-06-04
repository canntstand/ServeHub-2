#!/bin/bash
set -euo pipefail

# ------------------------------
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info()    { echo -e "${BLUE}[INFO] $(date '+%Y-%m-%d %H:%M:%S') - $1${NC}"; }
log_success() { echo -e "${GREEN}[SUCCESS] $1${NC}"; }
log_warn()    { echo -e "${YELLOW}[WARN] $1${NC}"; }
log_error()   { echo -e "${RED}[ERROR] $1${NC}"; }

print_separator() {
    echo -e "${CYAN}--------------------------------------------------------------------------------${NC}"
}

# ==========================================
print_separator
echo -e "${CYAN}         🚀 ИНИЦИАЛИЗАЦИЯ ИНФРАСТРУКТУРЫ (ЛОКАЛЬНАЯ СБОРКА) 🚀${NC}"
print_separator

# ==========================================
log_info "Определение дистрибутива и установка системных зависимостей..."
if [ -f /etc/os-release ]; then
    . /etc/os-release
    DISTRO=$ID
else
    log_error "Не удалось определить дистрибутив (нет /etc/os-release)."
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
    log_error "Дистрибутив '$DISTRO' не поддерживается."
    exit 1
fi
log_success "Зависимости установлены."

# ==========================================
log_info "Проверка и запуск Docker..."
if ! systemctl is-enabled --quiet docker; then
    log_warn "Автозапуск Docker отключен. Включаю..."
    sudo systemctl enable docker
fi
sudo systemctl start docker
log_success "Docker активен."

# ==========================================
ENV_FILE=".env"
if [ ! -f "$ENV_FILE" ]; then
    log_error "Файл $ENV_FILE не найден!"
    exit 1
fi

log_info "Загрузка переменных из $ENV_FILE..."
while IFS= read -r line || [ -n "$line" ]; do
    line="${line#"${line%%[![:space:]]*}"}"
    [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue
    var_name="${line%%=*}"
    var_value="${line#*=}"
    if [[ "$var_value" =~ ^\".*\"$ || "$var_value" =~ ^\'.*\'$ ]]; then
        var_value="${var_value:1:-1}"
    fi
    var_name_clean="$(echo -n "$var_name" | xargs)"
    export "$var_name_clean"="$var_value"
done < "$ENV_FILE"

required_vars=("SECRET_VAULTWARDEN_PASSWORD" "SYNAPSE_SERVER_NAME" "WEBNAMES_APIKEY" "HOME_USER_NAME")
for var in "${required_vars[@]}"; do
    if [ -z "${!var:-}" ]; then
        log_error "Не задана обязательная переменная $var в .env!"
        exit 1
    fi
done
log_success "Переменные окружения загружены и проверены."

# ==========================================
log_info "Создание необходимых директорий..."
DIRS=("./matrix/data" "./grafana/data" "./vaultwarden/data" "./synapse/data" "./navidrome/data" "./audiobookshelf/data" "./prometheus/data" "./matrix_alertmanager")
for dir in "${DIRS[@]}"; do
    mkdir -p "$dir"
done
sudo chmod -R 777 ./grafana ./matrix ./nextcloud ./vaultwarden ./synapse ./navidrome ./audiobookshelf
sudo mkdir -p "/home/${HOME_USER_NAME}/NextcloudData"
sudo chown -R 33:33 "/home/${HOME_USER_NAME}/NextcloudData"
log_success "Директории подготовлены."

# ==========================================
log_info "Генерация конфигурации Synapse..."
docker run -it --rm \
    -v "$(pwd)/matrix/data:/data" \
    -e SYNAPSE_SERVER_NAME="${SYNAPSE_SERVER_NAME}" \
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
log_success "Конфигурация Synapse создана."

# ==========================================
log_info "Генерация хэша пароля Vaultwarden..."
SALT=$(openssl rand -hex 8)
HASH_TOKEN=$(echo -n "$SECRET_VAULTWARDEN_PASSWORD" | argon2 "$SALT" -e -id -k 19456 -t 2 -p 1 | sed 's/\$/\$\$/g')
sed -i '/^VAULTWARDEN_ADMIN_HASH=/d' .env
echo "VAULTWARDEN_ADMIN_HASH=${HASH_TOKEN}" >> .env
log_success "Хэш Vaultwarden добавлен в .env."

# ==========================================
log_info "Запуск сервиса настройки мониторинга..."
docker compose -f docker-compose.local.yaml up -d monitoring_configure

log_info "Ожидание завершения конфигурации мониторинга..."
while true; do
    EXIT_CODE=$(docker inspect -f '{{.State.ExitCode}}' monitoring_configure 2>/dev/null || echo "-1")
    if [ "$EXIT_CODE" == "0" ]; then
        log_success "Настройка мониторинга успешно завершена."
        break
    elif [ "$EXIT_CODE" != "-1" ] && [ "$EXIT_CODE" != "" ]; then
        log_error "Контейнер monitoring_configure завершился с ошибкой (код $EXIT_CODE)."
        docker compose -f docker-compose.local.yaml logs monitoring_configure
        exit 1
    fi
    echo -n "."
    sleep 3
done

# ==========================================
log_info "Запуск основных сервисов..."
MAIN_SERVICES="synapse synapse_db nginx nginx_exporter navidrome audiobookshelf nextcloud nextcloud_db nextcloud_configure vaultwarden vaultwarden_db prometheus_init prometheus grafana node_exporter cadvisor portainer alertmanager matrix_alertmanager"
docker compose -f docker-compose.local.yaml up -d $MAIN_SERVICES

log_info "Ожидание стабилизации сервисов (15 секунд)..."
sleep 15

log_info "Создание административного пользователя..."
chmod +x scripts/create_admin.sh
./scripts/create_admin.sh

print_separator
log_success "ВСЕ СЛУЖБЫ УСПЕШНО ЗАПУЩЕНЫ!"
print_separator