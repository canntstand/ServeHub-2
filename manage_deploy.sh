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
UNDERLINE='\033[4m'
NC='\033[0m'

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE}")" && pwd)"
cd "$PROJECT_ROOT"

print_success() { echo -e "${GREEN}✔ $1${NC}"; }
print_error()   { echo -e "${RED}❌ $1${NC}"; }
print_info()    { echo -e "${CYAN}ℹ $1${NC}"; }
print_warning() { echo -e "${YELLOW}⚠ $1${NC}"; }
print_header()  { echo -e "${BLUE}${BOLD}── $1 ──${NC}"; }
print_subheader(){ echo -e "${MAGENTA}▶ $1${NC}"; }
print_separator(){ echo -e "${WHITE}${BOLD}══════════════════════════════════════════════════════════════════════${NC}"; }

if ! command -v docker &> /dev/null; then
    print_error "Docker не установлен!"
    exit 1
fi

if ! docker compose version &> /dev/null; then
    print_error "Docker Compose не установлен!"
    exit 1
fi

if ! command -v vagrant &> /dev/null; then
    print_warning "Vagrant не установлен на хосте! Пункты 8-10 (для тестирования) работать не будут."
fi

if ! command -v ansible &> /dev/null; then
    print_warning "Ansible не установлен на хосте! Пункты 8-10 (для тестирования) работать не будут."
fi

DEBUG_ARGS=""

run_ansible() {
    local args="$@"
    docker compose -f docker-compose.ansible.yaml run --rm ansible sh -c "ansible-playbook ${args} ${DEBUG_ARGS}"
}

check_inventory() {
    if [ ! -f "ansible/inventory.ini" ]; then
        print_error "ansible/inventory.ini не найден!"
        print_warning "Создайте файл из примера: cp ansible/inventory.ini.example ansible/inventory.ini"
        exit 1
    fi
}

check_secrets() {
    if [ ! -f "ansible/vars/secrets.yml" ]; then
        print_error "ansible/vars/secrets.yml не найден!"
        print_warning "Создайте файл из примера: cp ansible/vars/secrets.yml.example ansible/vars/secrets.yml"
        exit 1
    fi
}

show_header() {
    clear
    print_separator
    echo -e "${BOLD}${BLUE}                🚀 ИНТЕРАКТИВНЫЙ МАСТЕР ДЕПЛОЯ: SERVEHUB-2${NC}"
    print_separator
    echo ""
}

show_header

echo -e -n "${WHITE}Включить подробный режим отладки (-vvv)? [y/N / д/Н]: ${NC}"
read -n 1 DEBUG_CHOICE
echo ""

case "$DEBUG_CHOICE" in
    [yY]|[дД])
        DEBUG_ARGS="-vvv"
        print_warning "Режим дебага (-vvv) активирован."
        echo ""
        ;;
    *)
        DEBUG_ARGS=""
        print_info "Обычный режим вывода логов."
        echo ""
        ;;
esac

echo -e "${BOLD}${WHITE}Выберите сценарий развертывания инфраструктуры:${NC}\n"

echo -e "${BOLD}${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}${BLUE}║                  ОБЩИЕ СЦЕНАРИИ (VPS + локальный сервер)       ║${NC}"
echo -e "${BOLD}${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
echo -e " ${GREEN}1)${NC} ${BOLD}Полная установка с нуля (VPS + локальный)${NC}"
echo -e "    • Подготовит ОС на VPS и локальном сервере (пароль root)."
echo -e "    • Запустит сервисы на VPS, затем паузу для WireGuard, затем сервисы на локальном сервере.\n"
echo -e " ${GREEN}2)${NC} ${BOLD}Только разворачивание приложений (VPS + локальный), без bootstrap${NC}"
echo -e "    • Развернет сервисы на VPS, сделает паузу для WireGuard и настроит локальный сервер.\n"

echo -e "${BOLD}${YELLOW}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}${YELLOW}║                      ЛОКАЛЬНЫЙ СЕРВЕР                          ║${NC}"
echo -e "${BOLD}${YELLOW}╚════════════════════════════════════════════════════════════════╝${NC}"
echo -e " ${YELLOW}3)${NC} ${BOLD}Полная установка с нуля (локальный) + ожидание WireGuard${NC}"
echo -e "    • Подготовит ОС на локальном сервере (пароль root)."
echo -e "    • Затем пауза для настройки WireGuard и запуск сервисов на локальном сервере.\n"
echo -e " ${YELLOW}4)${NC} ${BOLD}Только разворачивание приложений (локальный) + ожидание WireGuard, без bootstrap${NC}"
echo -e "    • Пауза для WireGuard и запуск сервисов на локальном сервере (ОС уже настроена).\n"

echo -e "${BOLD}${CYAN}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}${CYAN}║                      УДАЛЁННЫЙ СЕРВЕР (VPS)                    ║${NC}"
echo -e "${BOLD}${CYAN}╚════════════════════════════════════════════════════════════════╝${NC}"
echo -e " ${YELLOW}5)${NC} ${BOLD}Полная установка с нуля (VPS)${NC}"
echo -e "    • Подготовит ОС на VPS (пароль root) и запустит сервисы.\n"
echo -e " ${YELLOW}6)${NC} ${BOLD}Только разворачивание приложений (VPS), без bootstrap${NC}"
echo -e "    • Только запуск сервисов на VPS (предполагается, что ОС уже настроена, SSH ключ есть).\n"

echo -e "${BOLD}${MAGENTA}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}${MAGENTA}║                   VAGRANT (ТЕСТИРОВАНИЕ)                       ║${NC}"
echo -e "${BOLD}${MAGENTA}╚════════════════════════════════════════════════════════════════╝${NC}"
echo -e " ${MAGENTA}7)${NC} ${BOLD}Запуск инфраструктуры Vagrant${NC}"
echo -e "    • Поднимет виртуалки (up) и сгенерирует файл SSH-конфига."
echo -e " ${MAGENTA}8)${NC} ${BOLD}Деплой на Vagrant-узлы${NC}"
echo -e "    • Запустит Ansible-плейбуки внутри ваших тестовых виртуалок."
echo -e " ${MAGENTA}9)${NC} ${BOLD}Удаление среды Vagrant${NC}"
echo -e "    • Уничтожит виртуалки (destroy) и очистит временные файлы.\n"

echo -e " ${RED}10)${NC} ${BOLD}Выход из мастера деплоя${NC}\n"

echo -e -n "${WHITE}Введите номер варианта [1-10]: ${NC}"
read CHOICE

check_inventory
check_secrets

case $CHOICE in
    1)
        echo ""
        print_header "1/3 Первичная подготовка VPS и локального сервера (пароль root)"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit vps,local --tags bootstrap"

        echo ""
        print_header "2/3 Разворачивание сервисов на VPS"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap"

        echo ""
        print_header "3/3 Пауза WireGuard и настройка локального сервера"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit local,localhost --skip-tags bootstrap"
        ;;
    2)
        echo ""
        print_header "1/2 Разворачивание сервисов на VPS (без bootstrap)"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap"

        echo ""
        print_header "2/2 Пауза WireGuard и разворачивание сервисов на локальном сервере (без bootstrap)"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit local,localhost --skip-tags bootstrap"
        ;;
    3)
        echo ""
        print_header "1/2 Первичная подготовка локального сервера (пароль root)"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit local --tags bootstrap"

        echo ""
        print_header "2/2 Пауза WireGuard и настройка локального сервера"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit local,localhost --skip-tags bootstrap"
        ;;
    4)
        echo ""
        print_header "Пауза WireGuard и разворачивание сервисов на локальном сервере (без bootstrap)"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit local,localhost --skip-tags bootstrap"
        ;;
    5)
        echo ""
        print_header "1/2 Первичная подготовка VPS (пароль root)"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit vps --tags bootstrap"

        echo ""
        print_header "2/2 Разворачивание сервисов на VPS"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap"
        ;;
    6)
        echo ""
        print_header "Разворачивание сервисов на VPS (без bootstrap, по SSH-ключу)"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap"
        ;;
    7)
        echo ""
        print_header "Работа с Vagrant: Поднятие сред"
        vagrant up
        vagrant ssh-config > vagrant_ssh_config
        print_success "Виртуалки запущены, конфиг SSH готов."
        ;;
    8)
        echo ""
        print_header "Деплой на Vagrant-узлы"
        if [ ! -f "vagrant_ssh_config" ]; then
            print_error "Конфиг SSH не найден! Сначала выполните пункт 8."
        else
            run_ansible_host "-i ansible/inventory.ini ansible/deploy.yml \
                --limit vagrant \
                --extra-vars \"ansible_ssh_common_args='-F ../vagrant_ssh_config'\""
        fi
        ;;
    9)
        echo ""
        print_header "Удаление Vagrant-сред"
        vagrant destroy -f
        rm -f vagrant_ssh_config
        print_success "Виртуальные машины удалены."
        ;;
    10)
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