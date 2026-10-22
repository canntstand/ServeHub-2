#!/usr/bin/env bash
set -Eeuo pipefail

PROJECT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${PROJECT_DIR}/.env"
COMPOSE_FILE="${PROJECT_DIR}/docker-compose.local.yaml"

log_info() {
    printf '[INFO] %s\n' "$*"
}

log_warn() {
    printf '[WARN] %s\n' "$*" >&2
}

log_error() {
    printf '[ERROR] %s\n' "$*" >&2
}

cd "$PROJECT_DIR"

if [[ ! -r "$ENV_FILE" ]]; then
    log_error "Не найден файл ${ENV_FILE}"
    exit 1
fi

SERVER_NAME="$(
    awk -F= '$1 == "SERVER_NAME" { sub(/^[^=]*=/, ""); print; exit }' "$ENV_FILE"
)"

if [[ -z "$SERVER_NAME" ]]; then
    log_error "Переменная SERVER_NAME отсутствует в .env"
    exit 1
fi

CERT_FILE="${PROJECT_DIR}/certbot/certs/live/${SERVER_NAME}/fullchain.pem"

if [[ ! -s "$CERT_FILE" ]]; then
    log_error "Сертификат отсутствует: ${CERT_FILE}. Требуется первоначальный выпуск."
    exit 1
fi

if openssl x509 -checkend 2592000 -noout -in "$CERT_FILE" >/dev/null 2>&1; then
    log_info "Сертификат действителен более 30 дней; продление не требуется."
    exit 0
fi

log_warn "Сертификат истекает или не проходит проверку срока действия. Запускаю Certbot."

docker compose -f "$COMPOSE_FILE" run --rm certbot renew --non-interactive

log_info "Проверяю конфигурацию Nginx и перечитываю сертификат."

docker compose -f "$COMPOSE_FILE" exec -T nginx sh -ec \
    'nginx -t && nginx -s reload'

log_info "Продление завершено, Nginx перечитал сертификат."
