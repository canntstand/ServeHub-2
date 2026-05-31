#!/bin/bash

echo "Установка и настройка системы защиты Fail2ban..."

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [ -f /etc/os-release ]; then
    . /etc/os-release
    DISTRO=$ID
else
    echo "Ошибка: Не удалось определить дистрибутив."
    exit 1
fi

if [[ "$DISTRO" == "arch" ]]; then
    echo "Обнаружен Arch Linux. Установка..."
    sudo pacman -S --noconfirm fail2ban

elif [[ "$DISTRO" == "ubuntu" || "$DISTRO" == "debian" || "$DISTRO" == "pop" || "$DISTRO" == "mint" ]]; then
    echo "Обнаружен $NAME. Установка..."
    sudo apt update && sudo apt install fail2ban -y

elif [[ "$DISTRO" == "centos" || "$DISTRO" == "rhel" || "$DISTRO" == "rocky" || "$DISTRO" == "almalinux" ]]; then
    echo "Обнаружен $NAME. Установка..."
    sudo dnf install epel-release -y 2>/dev/null || sudo yum install epel-release -y
    sudo dnf install fail2ban -y 2>/dev/null || sudo yum install fail2ban -y

else
    echo "Критическая ошибка: Дистрибутив '$DISTRO' не поддерживается."
    exit 1
fi

sudo cp "$SCRIPT_DIR/fail2ban/jail.local" /etc/fail2ban/jail.local

sudo systemctl enable --now fail2ban
sudo systemctl restart fail2ban

echo "Служба Fail2ban успешно установлена, настроена и запущена на базе $NAME."