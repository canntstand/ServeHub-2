> **AI-generated documentation:** Документ подготовлен с помощью ИИ; содержание сверено с кодом ветки `dev`.

# Документация ServeHub-2

- [Архитектура](ARCHITECTURE.md) — Edge/Workload, VPN и сетевые границы.
- [Установка](INSTALLATION.md) — требования, GUI-установщик, версии и секреты.
- [Компоненты](COMPONENTS.md) — Ansible, Compose, Nginx, Certbot, мониторинг и Hermes.
- [Резервное копирование](BACKUPS.md) — Borg/Borgmatic и S3.

Целевые серверы: Ubuntu Server 24.04 LTS. Проект рассчитан на Edge/VPS и Workload/домашний сервер. Ветка `dev` предназначена для разработки; успешный CI не заменяет сквозное испытание установки и восстановления на чистых виртуальных машинах.
