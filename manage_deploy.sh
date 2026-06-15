#!/bin/bash

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
WHITE='\033[1;37m'
BOLD='\033[1m'
NC='\033[0m'

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE}")" && pwd)"
cd "$PROJECT_ROOT"

print_success() { echo -e "${GREEN}✔ $1${NC}"; }
print_error()   { echo -e "${RED}❌ $1${NC}"; }
print_info()    { echo -e "${CYAN}ℹ $1${NC}"; }
print_warning() { echo -e "${YELLOW}⚠ $1${NC}"; }
print_header()  { echo -e "${BLUE}${BOLD}── $1 ──${NC}"; }
print_separator(){ echo -e "${WHITE}${BOLD}══════════════════════════════════════════════════════════════════════${NC}"; }

if ! command -v docker &> /dev/null; then
    print_error "Docker не установлен!"
    exit 1
fi

if ! docker compose version &> /dev/null; then
    print_error "Docker Compose не установлен!"
    exit 1
fi

DEBUG_ARGS=""

run_ansible() {
    local args="$@"
    docker compose -f docker-compose.ansible.yaml run --rm ansible sh -c "ansible-playbook ${args} ${DEBUG_ARGS}"
}

check_inventory() {
    if [ ! -f "ansible/inventory.ini" ]; then
        print_error "ansible/inventory.ini не найден!"
        exit 1
    fi
}

check_secrets() {
    if [ ! -f "ansible/vars/secrets.yml" ]; then
        print_error "ansible/vars/secrets.yml не найден!"
        exit 1
    fi
}

clear
print_separator
echo -e "${BOLD}${BLUE}                🚀 ОСНОВНОЙ МАСТЕР ДЕПЛОЯ: SERVEHUB-2${NC}"
print_separator
echo ""

echo -e -n "${WHITE}Включить подробный режим отладки (-vvv)? [y/N / д/Н]: ${NC}"
read -n 1 DEBUG_CHOICE
echo ""

case "$DEBUG_CHOICE" in
    [yY]|[дД])
        DEBUG_ARGS="-vvv"
        print_warning "Режим дебага (-vvv) активирован.\n"
        ;;
    *)
        DEBUG_ARGS=""
        print_info "Обычный режим вывода логов.\n"
        ;;
esac

echo -e "${BOLD}${WHITE}Выберите сценарий развертывания инфраструктуры:${NC}\n"

echo -e "${BOLD}${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}${BLUE}║                  ОБЩИЕ СЦЕНАРИИ (VPS + Локальный)              ║${NC}"
echo -e "${BOLD}${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
echo -e " ${GREEN}1)${NC} ${BOLD}Полная установка с нуля (VPS + Локальный)${NC}"
echo -e "    • Первичная подготовка ОС (bootstrap) и развертывание всех приложений."
echo -e " ${GREEN}2)${NC} ${BOLD}Только разворачивание приложений (VPS + Локальный), без bootstrap${NC}"
echo -e "    • Обновление конфигурации и сервисов на работающих серверах.\n"

echo -e "${BOLD}${YELLOW}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}${YELLOW}║                        ЛОКАЛЬНЫЙ СЕРВЕР                        ║${NC}"
echo -e "${BOLD}${YELLOW}╚════════════════════════════════════════════════════════════════╝${NC}"
echo -e " ${YELLOW}3)${NC} ${BOLD}Полная установка с нуля (Локальный) + WireGuard${NC}"
echo -e " ${YELLOW}4)${NC} ${BOLD}Только разворачивание приложений (Локальный), без bootstrap${NC}\n"

echo -e "${BOLD}${CYAN}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}${CYAN}║                     УДАЛЁННЫЙ СЕРВЕР (VPS)                     ║${NC}"
echo -e "${BOLD}${CYAN}╚════════════════════════════════════════════════════════════════╝${NC}"
echo -e " ${CYAN}5)${NC} ${BOLD}Полная установка с нуля (VPS)${NC}"
echo -e " ${CYAN}6)${NC} ${BOLD}Только разворачивание приложений (VPS), без bootstrap${NC}\n"

echo -e " ${RED}7)${NC} ${BOLD}Выход из мастера деплоя${NC}\n"

echo -e -n "${WHITE}Введите номер варианта [1-7]: ${NC}"
read CHOICE

check_inventory
check_secrets

case $CHOICE in
    1)
        echo ""
        print_header "1/3 Первичная подготовка VPS и локального сервера (bootstrap)"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit vps,local --tags bootstrap"

        echo ""
        print_header "2/3 Разворачивание сервисов на VPS"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap"

        echo ""
        print_header "3/3 Настройка локального сервера"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit local,localhost --skip-tags bootstrap"
        ;;
    2)
        echo ""
        print_header "1/2 Обновление сервисов на VPS"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap"

        echo ""
        print_header "2/2 Обновление сервисов на локальном сервере"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit local,localhost --skip-tags bootstrap"
        ;;
    3)
        echo ""
        print_header "1/2 Первичная подготовка локального сервера (bootstrap)"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit local --tags bootstrap"

        echo ""
        print_header "2/2 Настройка сервисов на локальном сервере"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit local,localhost --skip-tags bootstrap"
        ;;
    4)
        echo ""
        print_header "Обновление сервисов на локальном сервере"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit local,localhost --skip-tags bootstrap"
        ;;
    5)
        echo ""
        print_header "1/2 Первичная подготовка VPS (bootstrap)"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit vps --tags bootstrap"

        echo ""
        print_header "2/2 Разворачивание сервисов на VPS"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap"
        ;;
    6)
        echo ""
        print_header "Обновление сервисов на VPS"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap"
        ;;
    7)
        echo ""
        print_info "Выход из мастера деплоя."
        exit 0
        ;;
    *)
        echo ""
        print_error "Неверный ввод!"
        exit 1
        ;;
esac

echo ""
print_separator
print_success "Операция успешно завершена!"
print_separator
echo ""