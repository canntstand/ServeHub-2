#!/bin/bash
set -e

if [ -f .env ]; then
    export $(grep -v '^#' .env | xargs)
else
    echo "ОШИБКА: Файл .env не найден на VPS!"
    exit 1
fi

if [ -z "$FRP_TOKEN" ]; then
    echo "ОШИБКА: Переменная FRP_TOKEN не задана в .env"
    exit 1
fi

echo "=== 1. Установка и настройка системного Nginx ==="
sudo apt update && sudo apt install nginx -y

echo "Накатываем конфигурацию Nginx..."
sudo cp ./nginx/nginx.remote.conf /etc/nginx/nginx.conf

sudo nginx -t
sudo systemctl restart nginx
sudo systemctl enable nginx

echo "=== 2. Запуск серверной части FRP (frps) ==="

sudo docker compose -f docker-compose.remote.yaml up -d --force-recreate

echo "=== НАСТРОЙКА VPS УСПЕШНО ЗАВЕРШЕНА ==="
sudo docker compose -f docker-compose.remote.yaml ps