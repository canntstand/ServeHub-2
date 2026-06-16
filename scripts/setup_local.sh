#!/bin/bash
set -euo pipefail

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

if [[ "$EUID" -ne 0 ]]; then
    log_error "Этот скрипт должен выполняться с правами root. Используйте sudo."
    exit 1
fi

print_separator
echo -e "${CYAN}         🚀 ИНИЦИАЛИЗАЦИЯ ЛОКАЛЬНОЙ ИНФРАСТРУКТУРЫ 🚀${NC}"
print_separator

# ==========================================
log_info "Определение дистрибутива и установка зависимостей..."
if [ -f /etc/os-release ]; then
    . /etc/os-release
    DISTRO=$ID
else
    log_error "Не удалось определить дистрибутив (нет /etc/os-release)."
    exit 1
fi

if [[ "$DISTRO" == "arch" || "$DISTRO" == "endeavouros" ]]; then
    sudo pacman -Sy --noconfirm argon2 openssl
    log_warn "У вас Arch Linux, если в скрипте будут появляться ошибки, попробуйте перезагрузить систему!"
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

log_info "Ожидание готовности Docker API..."
for i in {1..10}; do
    if docker info >/dev/null 2>&1; then
        log_success "Docker активен и готов."
        break
    fi
    sleep 2
done
if ! docker info >/dev/null 2>&1; then
    log_error "Docker демон не запустился. Выход."
    exit 1
fi

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

required_vars=("SECRET_VAULTWARDEN_PASSWORD" "SYNAPSE_SERVER_NAME" "WEBNAMES_APIKEY" "LOCAL_USER")
for var in "${required_vars[@]}"; do
    if [ -z "${!var:-}" ]; then
        log_error "Не задана обязательная переменная $var в .env!"
        exit 1
    fi
done
log_success "Переменные окружения загружены и проверены."

# ==========================================
log_info "Создание необходимых директорий..."
DIRS=(
    "./apps-data/synapse" 
    "./apps-data/vaultwarden" 
    "./apps-data/navidrome" 
    "./apps-data/audiobookshelf" 
    "./apps-data/prometheus"
    "./apps-data/nextcloud" 
    "./apps-data/portainer"
)
for dir in "${DIRS[@]}"; do
    mkdir -p "$dir"
done

sudo chmod -R 777 ./apps-data

sudo mkdir -p "/home/${LOCAL_USER}/NextcloudData"
sudo chown -R 33:33 "/home/${LOCAL_USER}/NextcloudData"
log_success "Директории подготовлены."

# ==========================================
log_info "Генерация конфигурации Synapse..."
docker run -it --rm \
    -v "$(pwd)/apps-data/synapse:/data" \
    -e SYNAPSE_SERVER_NAME="${SYNAPSE_SERVER_NAME}" \
    -e SYNAPSE_REPORT_STATS=no \
    matrixdotorg/synapse:v1.152.1 generate

CONFIG_PATH="apps-data/synapse/homeserver.yaml"
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

sudo chown -R 991:991 apps-data/synapse/
log_success "Конфигурация Synapse создана."

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
print_separator
log_info "Применение системных настроек ядра Linux..."
modules=$(lsmod)

if ! echo "$modules" | grep -q amneziawg; then
    log_warn "Модуль amneziawg не загружен. Пытаюсь загрузить..."
    if sudo modprobe amneziawg 2>/dev/null; then
        log_success "Модуль amneziawg успешно загружен."
    else
        log_error "Не удалось загрузить модуль amneziawg."
        log_error "Убедитесь, что пакет amneziawg-dkms установлен."
        exit 1
    fi
else
    log_success "Модуль amneziawg уже загружен."
fi

add_sysctl_param() {
    local param="$1"
    local value="$2"
    local line="${param}=${value}"

    if [! -f /etc/sysctl.conf]; then
        sudo touch /etc/sysctl.conf
    fi

    if grep -qE "^[[:space:]]*${param}=" /etc/sysctl.conf; then
        sudo sed -i "s|^[[:space:]]*${param}=.*|${line}|" /etc/sysctl.conf
        echo -e "  [Ядро] Обновлён параметр: ${YELLOW}${line}${NC}"
    else
        echo "$line" | sudo tee -a /etc/sysctl.conf > /dev/null
        echo -e "  [Ядро] Добавлен параметр: ${GREEN}${line}${NC}"
    fi
}

add_sysctl_param "net.ipv4.ip_forward" "1"
add_sysctl_param "net.ipv4.conf.all.src_valid_mark" "1"
add_sysctl_param "net.ipv6.conf.all.disable_ipv6" "0"
add_sysctl_param "net.ipv6.conf.all.forwarding" "1"
add_sysctl_param "net.ipv6.conf.default.forwarding" "1"
sudo sysctl -p > /dev/null

log_info "Настройка брандмауэра iptables..."

DEFAULT_IF=$(ip route show default 2>/dev/null | awk '/default/ {print $5; exit}')
if [ -z "$DEFAULT_IF" ]; then
    log_error "Не удалось определить внешний интерфейс. Правила MASQUERADE не будут применены."
    exit 1
else
    log_info "Внешний интерфейс: ${DEFAULT_IF}"
fi

add_iptables_rule() {
    local chain="$1"
    shift
    if ! sudo iptables -C "$chain" "$@" 2>/dev/null; then
        sudo iptables -A "$chain" "$@"
        echo -e "  [iptables] Добавлено правило: ${chain} $*"
    else
        echo -e "  [iptables] Правило уже существует: ${chain} $*"
    fi
}

add_iptables_rule FORWARD -i wg0 -j ACCEPT
add_iptables_rule FORWARD -o wg0 -j ACCEPT
add_iptables_rule POSTROUTING -t nat -s 10.8.0.0/24 -o "$DEFAULT_IF" -j MASQUERADE

# ==========================================
log_info "Сохранение правил iptables..."
saved=false

if command -v netfilter-persistent &>/dev/null; then
    netfilter-persistent save
    log_success "Правила сохранены (netfilter-persistent)."
    saved=true
else
    if [[ "$DISTRO" == "ubuntu" || "$DISTRO" == "debian" || "$DISTRO" == "pop" || "$DISTRO" == "mint" ]]; then
        log_warn "netfilter-persistent не найден. Пытаюсь установить iptables-persistent..."
        sudo DEBIAN_FRONTEND=noninteractive apt-get install -y iptables-persistent
        if command -v netfilter-persistent &>/dev/null; then
            netfilter-persistent save
            log_success "Правила сохранены (netfilter-persistent)."
            saved=true
        fi
    elif [[ "$DISTRO" == "centos" || "$DISTRO" == "rhel" || "$DISTRO" == "rocky" || "$DISTRO" == "almalinux" ]]; then
        log_warn "netfilter-persistent не найден. Пробую iptables-services..."
        if ! rpm -q iptables-services &>/dev/null; then
            sudo dnf install -y iptables-services
        fi
        sudo service iptables save
        sudo systemctl enable iptables
        log_success "Правила сохранены через iptables-services."
        saved=true
    elif [[ "$DISTRO" == "arch" ]]; then
        log_warn "Arch Linux: сохранение через iptables-save..."
        sudo mkdir -p /etc/iptables
        sudo iptables-save | sudo tee /etc/iptables/iptables.rules > /dev/null
        sudo systemctl enable --now iptables
        log_success "Правила сохранены, сервис iptables активирован."
        saved=true
    else
        log_warn "Неизвестный дистрибутив. Сохраняю через iptables-save в /etc/iptables/rules.v4"
        mkdir -p /etc/iptables
        sudo iptables-save | sudo tee /etc/iptables/rules.v4 > /dev/null
        log_info "Добавьте восстановление в автозагрузку (например, systemd unit)."
        saved=true
    fi
fi

if [ "$saved" = false ]; then
    log_error "Не удалось сохранить правила iptables. После перезагрузки они пропадут."
    exit 1
fi

log_success "Сетевые правила применены."

# ==========================================
log_info "Запуск основных сервисов..."
MAIN_SERVICES="synapse synapse_db nginx nextcloud_cron amnezia-client nginx_exporter navidrome audiobookshelf nextcloud nextcloud_db vaultwarden vaultwarden_db prometheus_init prometheus grafana node_exporter cadvisor portainer alertmanager matrix_alertmanager"
docker compose -f docker-compose.local.yaml up -d $MAIN_SERVICES

log_info "Ожидание запуска сервисов..."
max_wait=30
elapsed=0
while [ $elapsed -lt $max_wait ]; do
    running_count=$(sudo docker compose -f docker-compose.local.yaml ps --services --filter "status=running" | wc -l)
    if [ "$running_count" -ge 3 ]; then
        break
    fi
    sleep 3
    elapsed=$((elapsed + 3))
done

if [ "$running_count" -lt 3 ]; then
    log_error "Не все сервисы запустились за отведённое время ($max_wait сек)."
    exit 1
fi

log_info "Создание административного пользователя..."
chmod +x scripts/create_admin.sh
./scripts/create_admin.sh

print_separator
echo -e "${CYAN}               📊 ТЕКУЩИЙ СТАТУС ЗАПУЩЕННЫХ СЕРВИСОВ 📊${NC}"
print_separator
sudo docker compose -f docker-compose.local.yaml ps

print_separator

log_success "Инициализация инфраструктуры завершена."