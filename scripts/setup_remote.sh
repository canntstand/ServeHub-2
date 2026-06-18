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
echo -e "${CYAN}         🚀 ИНИЦИАЛИЗАЦИЯ УДАЛЕННОЙ ИНФРАСТРУКТУРЫ 🚀${NC}"
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

# ==========================================
if ip link show awg0 > /dev/null 2>&1; then
    log_info "Старый интерфейс awg0 найден."
    sudo ip link delete awg0
    log_success "Интерфейс awg0 успешно удален."
else
    log_info "Интерфейс awg0 не найден. Ничего делать не нужно."
fi


# ==========================================
log_info "Проверка статуса демона Docker..."

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
print_separator
ENV_FILE=".env"
if [ ! -f "$ENV_FILE" ]; then
    log_error "Критическая ошибка: Файл $ENV_FILE не найден!"
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

log_success "Переменные окружения загружены."

# ==========================================
print_separator

CERT_DIR="./certbot/certs/live/${SYNAPSE_SERVER_NAME}"

USER_CERT_DIR="/home/${VPS_USER}/certbot/certs/live/${SYNAPSE_SERVER_NAME}"
if [ -f "${USER_CERT_DIR}/fullchain.pem" ]; then
    log_info "Сертификаты найдены в домашней директории пользователя, копирую в проект..."
    sudo cp -r "/home/${VPS_USER}/certbot/" "."
    sudo chown -R $(id -u):$(id -g) "./certbot/"
    log_success "Сертификаты скопированы."
fi

if [ ! -d "./certbot/certbot-dns-webnames" ]; then
    git clone https://github.com/regtime-ltd/certbot-dns-webnames.git ./certbot/certbot-dns-webnames
fi

log_info "Загрузка конфигурации Certbot от Webnames..."
if curl -s -o ./certbot/certbot-dns-webnames/config.sh \
    "https://www.webnames.ru/scripts/json_domain_zone_manager.pl?action=get_config_certbot&domain=${SYNAPSE_SERVER_NAME}&apikey=${WEBNAMES_APIKEY}"; then
    if [ -s ./certbot/certbot-dns-webnames/config.sh ]; then
        chmod +x ./certbot/certbot-dns-webnames/*.sh
        log_success "Конфигурация получена."
    else
        log_error "Получен пустой файл конфигурации."
        exit 1
    fi
else
    log_error "Не удалось загрузить конфигурацию Certbot."
    exit 1
fi

NEED_REAL_CERT=false
if ! sudo test -f "${CERT_DIR}/fullchain.pem"; then
    log_info "SSL-сертификаты не найдены. Будет запрошен новый."
    NEED_REAL_CERT=true
elif ! sudo openssl x509 -checkend 2592000 -noout -in "${CERT_DIR}/fullchain.pem" 2>/dev/null; then
    log_warn "Сертификат истекает скоро. Будет обновлён."
    NEED_REAL_CERT=true
else
    log_info "Действительный сертификат найден."
fi

if [ "$NEED_REAL_CERT" = true ]; then
    log_info "Запуск Certbot для получения сертификатов..."
    docker compose -f docker-compose.remote.yaml build certbot
    
    if ! sudo test -f "${CERT_DIR}/fullchain.pem"; then
        docker compose -f docker-compose.remote.yaml run --rm certbot
    else
        docker compose -f docker-compose.remote.yaml run --rm certbot renew --non-interactive
    fi
    
    log_success "Сертификаты успешно получены/обновлены."
fi

# ==========================================
PROJECT_DIR="/home/${VPS_USER}/ServeHub-2" 

log_info "Создание systemd-сервиса автообновления сертификатов..."

sudo tee /etc/systemd/system/certbot-renew.service > /dev/null << EOF
[Unit]
Description=Автоматическое обновление сертификатов Certbot/Webnames в Docker
After=docker.service

[Service]
Type=oneshot
WorkingDirectory=${PROJECT_DIR}
ExecStart=/bin/bash ./scripts/renew_certs.sh

[Install]
WantedBy=multi-user.target
EOF

if [ -f /etc/systemd/system/certbot-renew.service ]; then
    log_success "Сервис certbot-renew.service успешно создан."
else
    log_error "Сервис certbot-renew.service не удалось создать."
    exit 1
fi

sudo tee /etc/systemd/system/certbot-renew.timer > /dev/null << EOF
[Unit]
Description=Таймер для обновления сертификатов Certbot

[Timer]
OnCalendar=*-*-* 03,15:00:00
RandomizedDelaySec=900
Persistent=true

[Install]
WantedBy=timers.target
EOF

if [ -f /etc/systemd/system/certbot-renew.timer ]; then
    log_success "Таймер certbot-renew.timer успешно создан."
else
    log_error "Таймер certbot-renew.timer не удалось создать."
    exit 1
fi

sudo systemctl daemon-reload

sudo systemctl enable --now certbot-renew.timer

if systemctl is-active --quiet certbot-renew.timer; then
    log_success "Таймер certbot-renew.timer успешно запущен."
else
    log_error "Таймер certbot-renew.timer не был запущен."
fi

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

    if [ ! -f /etc/sysctl.conf ]; then
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
    elif [[ "$DISTRO" == "arch" || "$DISTRO" == "endeavouros" ]]; then
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
print_separator
log_info "Запуск контейнеров (gatus, nginx, wg-easy)..."

docker compose -f docker-compose.remote.yaml up -d gatus nginx wg-easy nginx_exporter

log_info "Ожидание запуска сервисов..."
max_wait=30
elapsed=0
while [ $elapsed -lt $max_wait ]; do
    running_count=$(docker compose -f docker-compose.remote.yaml ps --services --filter "status=running" | wc -l)
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

print_separator
echo -e "${CYAN}               📊 ТЕКУЩИЙ СТАТУС ЗАПУЩЕННЫХ СЕРВИСОВ 📊${NC}"
print_separator
docker compose -f docker-compose.remote.yaml ps

print_separator

log_success "Инициализация инфраструктуры завершена."