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
print_header()  { echo -e "${MAGENTA}${BOLD}── $1 ──${NC}"; }
print_separator(){ echo -e "${WHITE}${BOLD}══════════════════════════════════════════════════════════════════════${NC}"; }

if ! command -v ansible &> /dev/null; then
    print_error "Ansible не установлен в WSL! Тестирование локально невозможно."
    exit 1
fi

check_testing_environment() {
    if [ ! -f "vagrant_ssh_config" ]; then
        print_error "Файл 'vagrant_ssh_config' не найден!"
        print_warning "Перед запуском этого мастера убедитесь, что вы:"
        echo -e "  1. Подняли тестовые машины в Windows PowerShell: ${CYAN}vagrant up${NC}"
        echo -e "  2. Сгенерировали SSH-конфиг внутри WSL:         ${CYAN}vagrant ssh-config > vagrant_ssh_config${NC}"
        exit 1
    fi
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

DEBUG_ARGS=""

run_ansible_test() {
    local args="$@"
    ansible-playbook ${args} ${DEBUG_ARGS} --extra-vars "ansible_ssh_common_args='-F ./vagrant_ssh_config' is_vagrant=true"
}

clear
print_separator
echo -e "${BOLD}${MAGENTA}             🧪 МАСТЕР ЛОКАЛЬНОГО ТЕСТИРОВАНИЯ (VAGRANT)${NC}"
print_separator
print_info "Сценарии полностью идентичны продакшну. Выбор нод регулируется переменными в secrets.yml."
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

echo -e "${BOLD}${WHITE}Выберите сценарий ТЕСТИРОВАНИЯ инфраструктуры:${NC}\n"

echo -e "${BOLD}${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}${BLUE}║             ТЕСТ ОБЩИХ СЦЕНАРИЕВ (Тест VPS + Тест Local)        ║${NC}"
echo -e "${BOLD}${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
echo -e " ${GREEN}1)${NC} ${BOLD}Тест: Полная установка с нуля (Тест VPS + Тест Local)${NC}"
echo -e "    • Запустит bootstrap и развернет приложения на тестовых IP."
echo -e " ${GREEN}2)${NC} ${BOLD}Тест: Только разворачивание приложений, без bootstrap${NC}"
echo -e "    • Проверит деплой приложений на уже настроенные виртуалки.\n"

echo -e "${BOLD}${YELLOW}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}${YELLOW}║                     ТЕСТ ЛОКАЛЬНОГО СЕРВЕРА                    ║${NC}"
echo -e "${BOLD}${YELLOW}╚════════════════════════════════════════════════════════════════╝${NC}"
echo -e " ${YELLOW}3)${NC} ${BOLD}Тест: Полная установка с нуля (Тест Local) + WireGuard${NC}"
echo -e " ${YELLOW}4)${NC} ${BOLD}Тест: Только разворачивание приложений (Тест Local), без bootstrap${NC}\n"

echo -e "${BOLD}${CYAN}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}${CYAN}║                    ТЕСТ УДАЛЁННОГО СЕРВЕРА (VPS)               ║${NC}"
echo -e "${BOLD}${CYAN}╚════════════════════════════════════════════════════════════════╝${NC}"
echo -e " ${CYAN}5)${NC} ${BOLD}Тест: Полная установка с нуля (Тест VPS)${NC}"
echo -e " ${CYAN}6)${NC} ${BOLD}Тест: Только разворачивание приложений (Тест VPS), без bootstrap${NC}\n"

echo -e " ${RED}7)${NC} ${BOLD}Выход из мастера тестирования${NC}\n"

echo -e -n "${WHITE}Введите номер варианта [1-7]: ${NC}"
read CHOICE

check_testing_environment
check_inventory
check_secrets

case $CHOICE in
    1)
        echo ""
        print_header "Тестирование [1/3]: Первичная подготовка тестовых VPS и Local (bootstrap)"
        run_ansible_test "-i ansible/inventory.ini ansible/deploy.yml --limit vps,local --tags bootstrap"

        echo ""
        print_header "Тестирование [2/3]: Разворачивание сервисов на тестовом VPS"
        run_ansible_test "-i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap"

        echo ""
        print_header "Тестирование [3/3]: Настройка тестового локального сервера"
        run_ansible_test "-i ansible/inventory.ini ansible/deploy.yml --limit local,localhost --skip-tags bootstrap"
        ;;
    2)
        echo ""
        print_header "Тестирование [1/2]: Обновление сервисов на тестовом VPS"
        run_ansible_test "-i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap"

        echo ""
        print_header "Тестирование [2/2]: Обновление сервисов на тестовом локальном сервере"
        run_ansible_test "-i ansible/inventory.ini ansible/deploy.yml --limit local,localhost --skip-tags bootstrap"
        ;;
    3)
        echo ""
        print_header "Тестирование [1/2]: Первичная подготовка тестового локального сервера (bootstrap)"
        run_ansible_test "-i ansible/inventory.ini ansible/deploy.yml --limit local --tags bootstrap"

        echo ""
        print_header "Тестирование [2/2]: Настройка сервисов на тестовом локальном сервере"
        run_ansible_test "-i ansible/inventory.ini ansible/deploy.yml --limit local,localhost --skip-tags bootstrap"
        ;;
    4)
        echo ""
        print_header "Тестирование: Обновление сервисов на тестовом локальном сервере"
        run_ansible_test "-i ansible/inventory.ini ansible/deploy.yml --limit local,localhost --skip-tags bootstrap"
        ;;
    5)
        echo ""
        print_header "Тестирование [1/2]: Первичная подготовка тестового VPS (bootstrap)"
        run_ansible_test "-i ansible/inventory.ini ansible/deploy.yml --limit vps --tags bootstrap"

        echo ""
        print_header "Тестирование [2/2]: Разворачивание сервисов на тестовом VPS"
        run_ansible_test "-i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap"
        ;;
    6)
        echo ""
        print_header "Тестирование: Обновление сервисов на тестовом VPS"
        run_ansible_test "-i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap"
        ;;
    7)
        echo ""
        print_info "Выход из мастера тестирования."
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
print_success "Тестирование успешно завершено!"
print_separator
echo ""