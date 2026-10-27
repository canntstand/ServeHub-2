![ServeHub-2](icons/ServeHub-2-logo-matte-neutral.svg)

![Version](https://img.shields.io/github/v/release/canntstand/ServeHub-2?label=version)
![License](https://img.shields.io/github/license/canntstand/ServeHub-2)
![Last commit](https://img.shields.io/github/last-commit/canntstand/ServeHub-2)
![Stars](https://img.shields.io/github/stars/canntstand/ServeHub-2?style=social)

[English version](README-ENG.md)

Проект автоматизирует self-hosted-инфраструктуру из двух узлов: удалённого Edge/VPS и локального Workload-сервера. Edge предоставляет VPN-доступ, DNS-фильтрацию, мониторинг и служебные компоненты; Workload запускает пользовательские приложения и хранит их данные.

Проект ориентирован на **Ubuntu Server 24.04 LTS**. Развёртывание выполняется через Ansible; GUI-установщик на Go/Wails позволяет выбрать релиз, Git-тег или ветку `dev`.

## Возможности

- **Приложения:** Nextcloud, Vaultwarden, Gitea, Navidrome, Audiobookshelf, Homebox, Habitica, Actual Budget.
- **Сеть и безопасность:** AmneziaWG/wg-easy, AdGuard Home, Nginx, Certbot с DNS-01 через Webnames, SSH-ключи, Fail2ban.
- **Мониторинг:** Prometheus, Grafana, exporters, Blackbox Exporter, Loki, Grafana Alloy, Alertmanager с Telegram-уведомлениями.
- **Резервное копирование:** Borg/Borgmatic и зеркалирование зашифрованных репозиториев в S3-совместимое хранилище.
- **Hermes Agent:** компонент Edge с постоянным workspace для файлов, к которым агенту разрешён доступ.

## Что подготовить

Перед установкой подготовьте:

- **Два сервера с Ubuntu Server 24.04 LTS:** публичный Edge/VPS и Workload-сервер, доступный по SSH. Рекомендуется начинать с чистых серверов.
- **Домен и DNS:** домен с управлением DNS через Webnames, API-ключ Webnames для DNS-01 и A-запись основного домена вместе с wildcard-записью `*`, направленные на VPN-адрес Workload `10.8.0.2`. Не указывайте публичный IP Edge для сервисов, предназначенных только для VPN.
- **Telegram:** токен бота и идентификатор чата, если нужны уведомления Alertmanager.
- **S3:** заранее созданный приватный S3-совместимый бакет, endpoint API, регион (если нужен), Access Key и Secret Key с правами чтения, записи и удаления объектов. Ключи должны быть ограничены нужным бакетом/префиксами. Проект не создаёт бакет автоматически.
- **Доступ и секреты:** SSH-доступ и необходимые права root/sudo, данные пользователей и приложения для `ansible/vars/secrets.yml`. Не коммитьте файл; установите права `0600`. Сохраните `borgmatic_encryption_passphrase` отдельно от серверов и S3.
- **Компьютер для GUI:** Docker Engine и Docker Compose должны быть установлены и запущены на компьютере, где запускается GUI-установщик; Ansible выполняется в контейнере.
- **Тестовая среда (рекомендуется):** Vagrant, VirtualBox и сетевой интерфейс с доступом в интернет. При использовании Vagrant выберите правильный bridged interface и учтите маршруты VPN, чтобы не потерять SSH-доступ к VM.

Развёртывание меняет SSH, firewall, маршрутизацию и Docker. Сначала проверяйте на чистых виртуальных машинах и сохраните доступ к консоли провайдера VPS.

## Краткая установка

1. Запустите GUI-установщик из `gui-installer/` либо используйте CLI-сценарий `scripts/manage_deploy.sh`.
2. Выберите версию проекта и проверьте закреплённый commit SHA.
3. Заполните `ansible/vars/secrets.yml`, проверьте параметры и права файла.
4. После установки проверьте VPN-доступ, TLS, приложения, мониторинг и доставку уведомлений.
5. Проведите фактический тест восстановления резервной копии до того, как полагаться на неё.

## Документация

Полное руководство со ссылками на тематические документы находится в [документации ServeHub-2](docs/README.md). Документы на русском языке и помечены как созданные с помощью ИИ; перед выполнением команд сверяйте их с текущей веткой `dev`.

## Тестирование

В корне проекта доступны тестовые VM через Vagrant:

```bash
vagrant up
vagrant halt
vagrant destroy -f
```

Настройки VM, DNS/VPN, `secrets.yml`, S3 и сборка GUI-установщика описаны в [руководстве по установке](docs/INSTALLATION.md).
