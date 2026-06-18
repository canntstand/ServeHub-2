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

print_separator
ENV_FILE="./.env"
if [ ! -f "$ENV_FILE" ]; then
    log_error "Критическая ошибка: Файл $ENV_FILE не найден в $(pwd)!"
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
CERT_DIR="./certbot/certs/live/${SYNAPSE_SERVER_NAME}"

if ! sudo openssl x509 -checkend 2592000 -noout -in "${CERT_DIR}/fullchain.pem" 2>/dev/null; then
    log_warn "Сертификат истекает или не найден. Запуск обновления..."
    
    docker compose -f docker-compose.remote.yaml run --rm certbot renew --non-interactive

    log_info "Перезапуск веб-сервера Nginx..."
    docker compose -f docker-compose.remote.yaml restart nginx
    log_success "Сертификаты обновлены, Nginx перезапущен."
else
    log_info "Сертификат еще действителен (более 30 дней)."
fi