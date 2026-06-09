#!/bin/bash

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$PROJECT_ROOT"

if ! command -v docker &> /dev/null; then
    echo -e "${RED}❌ Docker не установлен!${NC}"
    exit 1
fi

if ! docker compose version &> /dev/null; then
    echo -e "${RED}❌ Docker Compose не установлен!${NC}"
    exit 1
fi

run_ansible() {
    local args="$@"
    docker compose -f docker-compose.ansible.yaml run --rm ansible sh -c "ansible-playbook ${args}"
}

check_inventory() {
    if [ ! -f "ansible/inventory.ini" ]; then
        echo -e "${RED}❌ Ошибка: ansible/inventory.ini не найден!${NC}"
        echo -e "${YELLOW}💡 Создайте файл из примера: cp ansible/inventory.ini.example ansible/inventory.ini${NC}"
        exit 1
    fi
}

check_secrets() {
    if [ ! -f "ansible/vars/secrets.yml" ]; then
        echo -e "${RED}❌ Ошибка: ansible/vars/secrets.yml не найден!${NC}"
        echo -e "${YELLOW}💡 Создайте файл из примера: cp ansible/vars/secrets.yml.example ansible/vars/secrets.yml${NC}"
        exit 1
    fi
}

show_header() {
    clear
    echo -e "${BLUE}======================================================================${NC}"
    echo -e "${BLUE}          🚀 Интерактивный мастер деплоя: ServeHub-2${NC}"
    echo -e "${BLUE}======================================================================${NC}"
    echo -e "Выберите сценарий развертывания инфраструктуры:"
    echo ""
}

show_header

echo -e "1) ${GREEN}Полная установка с нуля (VPS + Локальный)${NC}"
echo -e "   • Подготовит ОС на VPS и локальном сервере (пароль root)."
echo -e "   • Запустит сервисы на VPS, затем паузу для WireGuard, затем сервисы на локальном сервере."
echo -e "----------------------------------------------------------------------"
echo -e "2) ${YELLOW}Полная установка с нуля (VPS)${NC}"
echo -e "   • Подготовит ОС на VPS (пароль root) и запустит сервисы."
echo -e "   • Локальный сервер не трогает."
echo -e "----------------------------------------------------------------------"
echo -e "3) ${YELLOW}Полная установка с нуля (Локальный) + Ожидание WireGuard${NC}"
echo -e "   • Подготовит ОС на локальном сервере (пароль root)."
echo -e "   • Затем пауза для настройки WireGuard и запуск сервисов на локальном сервере."
echo -e "----------------------------------------------------------------------"
echo -e "4) ${YELLOW}Только разворачивание приложений (VPS), без bootstrap${NC}"
echo -e "   • Только запуск сервисов на VPS (предполагается, что ОС уже настроена, SSH ключ есть)."
echo -e "----------------------------------------------------------------------"
echo -e "5) ${YELLOW}Только разворачивание приложений (Локальный) + Ожидание WireGuard, без bootstrap${NC}"
echo -e "   • Пауза для WireGuard и запуск сервисов на локальном сервере (ОС уже настроена)."
echo -e "----------------------------------------------------------------------"
echo -e "6) ${RED}Выход из мастера деплоя${NC}"
echo ""

read -p "Введите номер варианта [1-6]: " CHOICE

check_inventory
check_secrets

case $CHOICE in
    1)
        echo -e "\n${BLUE}>>> [1/3] Первичная подготовка VPS и локального сервера (пароль root)...${NC}"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit vps,local -k -u root --tags bootstrap"

        echo -e "\n${BLUE}>>> [2/3] Разворачивание сервисов на VPS...${NC}"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap"

        echo -e "\n${BLUE}>>> [3/3] Пауза WireGuard и настройка локального сервера...${NC}"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit local,localhost --skip-tags bootstrap"
        ;;
    2)
        echo -e "\n${BLUE}>>> [1/2] Первичная подготовка VPS (пароль root)...${NC}"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit vps -k -u root --tags bootstrap"

        echo -e "\n${BLUE}>>> [2/2] Разворачивание сервисов на VPS...${NC}"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap"
        ;;
    3)
        echo -e "\n${BLUE}>>> [1/2] Первичная подготовка локального сервера (пароль root)...${NC}"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit local -k -u root --tags bootstrap"

        echo -e "\n${BLUE}>>> [2/2] Пауза WireGuard и настройка локального сервера...${NC}"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit local,localhost --skip-tags bootstrap"
        ;;
    4)
        echo -e "\n${BLUE}>>> Разворачивание сервисов на VPS (без bootstrap, по SSH-ключу)...${NC}"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap"
        ;;
    5)
        echo -e "\n${BLUE}>>> Пауза WireGuard и разворачивание сервисов на локальном сервере (без bootstrap)...${NC}"
        run_ansible "-i ansible/inventory.ini ansible/deploy.yml --limit local,localhost --skip-tags bootstrap"
        ;;
    6)
        echo -e "\n${RED}Выход.${NC}"
        exit 0
        ;;
    *)
        echo -e "\n${RED}❌ Неверный ввод!${NC}"
        exit 1
        ;;
esac

echo ""
echo -e "${GREEN}✔ Операция успешно завершена!${NC}"