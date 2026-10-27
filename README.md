![ServeHub-2](icons/ServeHub-2-logo-matte-neutral.svg)

![Version](https://img.shields.io/github/v/release/canntstand/ServeHub-2?label=version)
![License](https://img.shields.io/github/license/canntstand/ServeHub-2)
![Last commit](https://img.shields.io/github/last-commit/canntstand/ServeHub-2)
![Stars](https://img.shields.io/github/stars/canntstand/ServeHub-2?style=social)

[English version](README-ENG.md)

# ServeHub-2

ServeHub-2 развёртывает self-hosted-инфраструктуру на двух узлах: удалённом Edge/VPS и локальном Workload-сервере. Edge обеспечивает VPN-доступ, сетевые ограничения, мониторинг и служебные компоненты; Workload запускает пользовательские приложения и хранит их данные.

Проект рассчитан прежде всего на чистые серверы под **Ubuntu Server 24.04 LTS**. Развёртывание автоматизировано через Ansible; GUI-установщик на Go/Wails помогает выбрать версию проекта и запустить установку.

## Основные возможности

- **Приложения:** Nextcloud, Vaultwarden, Gitea, Navidrome, Audiobookshelf, Homebox, Habitica и Actual Budget.
- **Сеть и безопасность:** AmneziaWG/wg-easy, AdGuard Home, Nginx, Certbot с DNS-01 через Webnames, SSH-ключи и Fail2ban.
- **Мониторинг:** Prometheus, Grafana, exporters, Blackbox Exporter, Loki, Grafana Alloy и Alertmanager с уведомлениями в Telegram.
- **Резервное копирование:** Borg/Borgmatic и зеркалирование зашифрованных репозиториев в S3-совместимое хранилище.
- **Hermes Agent:** запускается на Edge; постоянная папка workspace предназначена для файлов, к которым агенту разрешён доступ.

## Перед установкой

- Подготовьте два узла Ubuntu Server 24.04 LTS: публичный Edge/VPS и Workload-сервер, доступный по SSH.
- GUI-установщику нужны работающие Docker Engine и Docker Compose на компьютере, где он запускается.
- Подготовьте домен, Webnames API key, данные Telegram для уведомлений и заранее созданный приватный S3 bucket.
- Для VPN-only доступа DNS-записи домена и wildcard должны указывать на Workload внутри VPN: `10.8.0.2`, а не на публичный IP Edge.
- Установка меняет SSH, firewall, маршрутизацию и Docker. Сначала проверьте её на чистых виртуальных машинах и сохраните доступ к консоли VPS.

## Установка

1. Запустите GUI-установщик из `gui-installer/` либо используйте CLI-сценарий `scripts/manage_deploy.sh`.
2. Выберите версию проекта и проверьте закреплённый commit SHA.
3. Настройте `ansible/vars/secrets.yml`, ограничьте доступ к файлу правами `0600` и не добавляйте секреты в Git.
4. После установки проверьте VPN-доступ к сервисам, мониторинг, уведомления и фактическое восстановление резервной копии.

## Документация

Полное руководство и ссылки на все тематические документы находятся в [документации ServeHub-2](docs/README.md). Документация написана на русском языке и помечена как созданная с помощью ИИ. Она содержит подробности архитектуры, компонентов, установки, эксплуатации, безопасности, диагностики и резервного копирования.

## Тестирование

Для тестирования Ansible используется Vagrant и VirtualBox:

```bash
vagrant up
vagrant halt
vagrant destroy -f
```

Параметры тестовых узлов и сборка GUI-установщика описаны в [руководстве по установке](docs/INSTALLATION.md).
