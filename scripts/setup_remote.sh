#!/bin/bash
set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO] $(date '+%Y-%m-%d %H:%M:%S') - $1${NC}"; }
log_success() { echo -e "${GREEN}[SUCCESS] $1${NC}"; }
log_warn() { echo -e "${YELLOW}[WARN] $1${NC}"; }
log_error() { echo -e "${RED}[ERROR] $1${NC}"; }

print_separator() {
    echo -e "${CYAN}--------------------------------------------------------------------------------${NC}"
}

print_separator
echo -e "${CYAN}         🚀 ИНИЦИАЛИЗАЦИЯ И НАСТРОЙКА ИНФРАСТРУКТУРЫ SERVEHUB-2 🚀${NC}"
print_separator

# ==========================================
# 1. ПРОВЕРКА DOCKER
# ==========================================
log_info "Проверка статуса демона Docker..."
if ! systemctl is-enabled --quiet docker; then
    log_warn "Автозапуск Docker отключен. Включаю..."
    sudo systemctl enable docker
fi

sudo systemctl start docker
log_success "Docker активен."

# ==========================================
# 2. ЗАГРУЗКА И ВАЛИДАЦИЯ .ENV
# ==========================================
print_separator
ENV_FILE=".env"
if [ -f "$ENV_FILE" ]; then
    log_info "Загрузка переменных из $ENV_FILE..."
    while IFS= read -r line || [ -n "$line" ]; do
        [[ "$line" =~ ^[[:space:]]*# ]] && continue
        [[ -z "${line//[[:space:]]/}" ]] && continue
        
        var_name="${line%%=*}"
        var_value="${line#*=}"
        export "$var_name"="$var_value"
    done < "$ENV_FILE"
else
    log_error "Критическая ошибка: Файл $ENV_FILE не найден!"
    exit 1
fi

for var in ADMIN_USER ADMIN_PASSWORD SYNAPSE_SERVER_NAME WEBNAMES_APIKEY; do
    if [ -z "${!var}" ]; then
        log_error "Ошибка: Переменная $var не задана в файле .env!"
        exit 1
    fi
done
log_success "Файл окружения валидирован."

# ==========================================
# 3. УСТАНОВКА HTPASSWD
# ==========================================
print_separator
if ! command -v htpasswd &> /dev/null; then
    log_info "Установка утилиты htpasswd..."
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        DISTRO=$ID
    else
        log_error "Не удалось определить дистрибутив системы."
        exit 1
    fi

    if [ "$DISTRO" = "ubuntu" ] || [ "$DISTRO" = "debian" ] || [ "$DISTRO" = "pop" ] || [ "$DISTRO" = "mint" ] || [ "$DISTRO" = "linuxmint" ] || [ "$DISTRO" = "raspbian" ]; then
        sudo apt-get update -qq && sudo apt-get install -y apache2-utils -qq
    elif [ "$DISTRO" = "rhel" ] || [ "$DISTRO" = "centos" ] || [ "$DISTRO" = "rocky" ] || [ "$DISTRO" = "almalinux" ] || [ "$DISTRO" = "fedora" ]; then
        sudo dnf install -y epel-release -q && sudo dnf install -y httpd-tools -q
    elif [ "$DISTRO" = "arch" ] || [ "$DISTRO" = "manjaro" ]; then
        sudo pacman -Syu --noconfirm apache-tools > /dev/null
    elif [ "$DISTRO" = "opensuse" ] || [ "$DISTRO" = "suse" ]; then
        sudo zypper install -y apache2-utils > /dev/null
    elif [ "$DISTRO" = "alpine" ]; then
        sudo apk add apache2-utils > /dev/null
    else
        log_error "Дистрибутив '$DISTRO' не поддерживается. Установите apache2-utils вручную."
        exit 1
    fi
    log_success "Утилита htpasswd успешно установлена."
fi

# ==========================================
# 4. ГЕНЕРАЦИЯ BCRYPT ХЭША ДЛЯ GATUS
# ==========================================
print_separator
if ! grep -q "^ADMIN_PASSWORD_HASH=" "$ENV_FILE"; then
    log_info "Начинаю генерацию хэша пароля..."
    BCRYPT_HASH=$(htpasswd -bnBC 10 "" "$ADMIN_PASSWORD" | tr -d ':\n' | sed 's/\$2y\$/\$2a\$/')
    
    ESCAPED_HASH=$(echo "$BCRYPT_HASH" | sed 's/\$/\$\$/g')
    echo "ADMIN_PASSWORD_HASH=$ESCAPED_HASH" >> "$ENV_FILE"
    log_success "ADMIN_PASSWORD_HASH успешно добавлен в $ENV_FILE с экранированием."
    
    export ADMIN_PASSWORD_HASH="$ESCAPED_HASH"
else
    log_info "ADMIN_PASSWORD_HASH уже существует в .env."
    RAW_HASH=$(grep "^ADMIN_PASSWORD_HASH=" "$ENV_FILE" | cut -d= -f2-)
    if [[ "$RAW_HASH" != *"\$\$"* ]]; then
        ESCAPED_HASH=$(echo "$RAW_HASH" | sed 's/\$/\$\$/g')
        export ADMIN_PASSWORD_HASH="$ESCAPED_HASH"
    else
        export ADMIN_PASSWORD_HASH="$RAW_HASH"
    fi
fi


# ==========================================
# 5. НАСТРОЙКА SSL СЕРТИФИКАТОВ (CERTBOT)
# ==========================================
print_separator
GATUS_CONFIG_FILE="./gatus/config/config.yaml"
if [ ! -f "$GATUS_CONFIG_FILE" ]; then
    log_error "КРИТИЧЕСКАЯ ОШИБКА: Конфигурация $GATUS_CONFIG_FILE отсутствует!"
    exit 1
fi

if [ ! -d "./certbot-dns-webnames" ]; then
    git clone https://github.com/regtime-ltd/certbot-dns-webnames.git ./certbot-dns-webnames
fi

curl -s -k "https://www.webnames.ru/scripts/json_domain_zone_manager.pl?action=get_config_certbot&domain=${SYNAPSE_SERVER_NAME}&apikey=${WEBNAMES_APIKEY}" -o ./certbot-dns-webnames/config.sh
chmod +x ./certbot-dns-webnames/*.sh

NEED_REAL_CERT=false
CERT_DIR="./certs/live/${SYNAPSE_SERVER_NAME}"
if [ ! -f "${CERT_DIR}/fullchain.pem" ]; then
    log_warn "SSL-сертификаты Let's Encrypt не найдены. Создаю самоподписанную заглушку..."
    mkdir -p "${CERT_DIR}"
    openssl req -x509 -nodes -days 1 -newkey rsa:2048 -keyout "${CERT_DIR}/privkey.pem" -out "${CERT_DIR}/fullchain.pem" -subj "/CN=localhost" > /dev/null 2>&1
    NEED_REAL_CERT=true
fi

if [ "$NEED_REAL_CERT" = true ]; then
    log_info "Запуск Certbot для получения оригинальных сертификатов..."
    sudo docker compose -f docker-compose.remote.yaml build certbot
    rm -rf "${CERT_DIR:?}"/*
    sudo docker compose -f docker-compose.remote.yaml run --rm certbot
    log_success "Сертификаты успешно получены."
fi

# ==========================================
# 6. СИСТЕМНЫЕ НАСТРОЙКИ СЕТИ И FIREWALL
# ==========================================
print_separator
log_info "Применение системных настроек ядра Linux..."

add_sysctl_param() {
    local param="$1"
    local value="$2"
    local line="${param}=${value}"
    
    if grep -qxF "$line" /etc/sysctl.conf; then
        echo -e "  [Ядро] Параметр ${GREEN}$line${NC} уже активен."
    else
        echo "$line" | sudo tee -a /etc/sysctl.conf > /dev/null
        echo -e "  [Ядро] Добавлен параметр: ${YELLOW}$line${NC}"
    fi
}


add_sysctl_param "net.ipv4.ip_forward" "1"
add_sysctl_param "net.ipv4.conf.all.src_valid_mark" "1"
add_sysctl_param "net.ipv6.conf.all.disable_ipv6" "0"
add_sysctl_param "net.ipv6.conf.all.forwarding" "1"
add_sysctl_param "net.ipv6.conf.default.forwarding" "1"
sudo sysctl -p > /dev/null

log_info "Настройка брандмауэра iptables..."
if ! sudo iptables -C FORWARD -i wg0 -j ACCEPT 2>/dev/null; then sudo iptables -A FORWARD -i wg0 -j ACCEPT; fi
if ! sudo iptables -C FORWARD -o wg0 -j ACCEPT 2>/dev/null; then sudo iptables -A FORWARD -o wg0 -j ACCEPT; fi
if ! sudo iptables -t nat -C POSTROUTING -s 10.8.0.0/24 -o eth0 -j MASQUERADE 2>/dev/null; then 
    sudo iptables -t nat -A POSTROUTING -s 10.8.0.0/24 -o eth0 -j MASQUERADE
fi
log_success "Сетевые правила применены."

# ==========================================
# 7. ЗАПУСК КОНТЕЙНЕРОВ
# ==========================================
print_separator
log_info "Запуск контейнеров в Docker Compose..."

sudo docker compose -f docker-compose.remote.yaml --env-file .env up gatus nginx wg-easy -d

log_info "Ожидание инициализации (7 секунд)..."
sleep 7

print_separator
echo -e "${CYAN}               📊 ТЕКУЩИЙ СТАТУС ЗАПУЩЕННЫХ СЕРВИСОВ 📊${NC}"
print_separator
sudo docker compose -f docker-compose.remote.yaml ps

echo ""
log_info "Логи контейнера Gatus:"
sudo docker compose -f docker-compose.remote.yaml logs --tail=5 gatus
print_separator