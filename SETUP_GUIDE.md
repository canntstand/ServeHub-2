## ⚠️ Важно
1. Все команды выполняются из корня проекта.
2. Проект рассчитан на операционную систему Linux (точно будет работать на Ubuntu, Debian, Arch).

## 1. Подготовка (на удаленном и локальном серверах)
1. В DNS-панели вашего домена (на Webnames): Направить A-запись (и wildcard-запись `*`) на внутренний IP-адрес VPS в туннеле (`10.8.1.1`) для автоматического направления трафика.
2. Настроить SSH + пользователей по плану ниже.
3. Установить [Docker](https://docs.docker.com/engine/install/)
4. `sudo systemctl enable --now docker`
5. Обновить все пакеты (к прим. sudo apt update && sudo apt upgrade).
6. `git clone https://github.com/canntstand/Mini-PC-setup && cd Mini-PC-setup`
7. Создать `.env` по примеру `.env.example` (на обоих серверах файлы должны быть идентичные).


## 2. На удаленном сервере
1. `sudo docker compose -f docker-compose.remote.yaml up -d`
2. Перейти в браузере по http://server-ip:3001 в Uptime Kuma. Создать новый монитор с типом **Push** и задать ему имя `Home-Server`. Это необходимо, чтобы локальный контейнер `heartbeat_to_vps` смог считать его токен и начать ежеминутную отправку отчетов о жизнеспособности домашнего ПК.
4. Перейти в браузере по http://server-ip:3002 в WireGuard панель, создать админ пользователя и создать всех клиентов (в зависимости от того сколько устройств планируется подключить к VPN).
5. Скачать конфиг клиента по IP `10.8.1.2` (чтобы подключить локальный сервер).

## 3. На локальном сервере
1. Положить скопированный WireGuard конфиг в папку `amnezia-data/` и назвать файл `awg0.conf`.
2. `chmod +x scripts/setup_local.sh && ./scripts/setup_local.sh`.

## SSH (стоит настроить и на локальном сервере, и на удаленном vps)
1. ПРЕДУСТАНОВКА (Если вы зашли на чистый сервер под пользователем root): Создать нового пользователя `sudo adduser server-user`, добавить его в группу администраторов `sudo usermod -aG sudo server-user` и переключиться через `su - server-user`.
2. Включить службу SSH: `sudo systemctl enable --now ssh`.
3. На клиенте сгенерировать ключ безопасности: `ssh-keygen -t ed25519 -C "key-access"`.
4. Скопировать ключ с клиента на сервер через `ssh-copy-id server-user@server-ip` (лучше сразу сделать для всех клиентов, с которых нужно будет подключаться по ssh).
5. Проверить вход без пароля с клиента и на сервере открыть конфигурацию: `sudo nano /etc/ssh/sshd_config`.
6. Выставить и проверить параметры безопасности: `PasswordAuthentication no`, `PermitRootLogin no`, `MaxAuthTries 3`, `PermitEmptyPasswords no`.
7. Если вверху файла sshd_config есть строка `Include /etc/ssh/sshd_config.d/*.conf`, зайти в файлы внутри этой папки и также перевести параметр `PasswordAuthentication` в состояние `no`.
8. Применить настройки и перезапустить службу: `sudo systemctl restart ssh`.
9. ВАЖНО: Не закрывая текущий терминал, открыть новое окно на ПК и проверить вход через команду `ssh user@server-ip`.
10. Настроить защиту от брутфорса: `chmod +x scripts/fail2ban_setup.sh && ./scripts/fail2ban_setup.sh`.

## Пользование сервисами
1. После запуска перейдите по вашему домену на главную страницу (веб-портал), находясь под включенным Amnezia VPN.
2. Для Navidrome, Audiobookshelf, Vaultwarden и Portainer при первом открытии необходимо пройти быструю процедуру создания аккаунта администратора. Для системной админки Vaultwarden используется токен из переменной SECRET_VAULTWARDEN_PASSWORD.
3. Сервисы Nextcloud и Grafana разворачиваются автоматически. Данные для первого входа в них соответствуют глобальным переменным ADMIN_USER и ADMIN_PASSWORD из вашего .env.
4. Служебный аккаунт для Synapse (Matrix) создается автоматически. Скрипт деплоя направит вашей основной учетной записи инвайт в комнату. Примите его в клиенте Element, чтобы получать уведомления от Alertmanager.
5. Рекомендации по клиентам для повседневного использования:
    - VPN — Amnezia VPN или WireGuard (конфиги брать из папки VPS `./amnezia-data/`)
    - Облако — Nextcloud
    - Пароли — Bitwarden
    - Мессенджер — Element X
    - Музыка — Tempo
    - Аудиокниги — Audiobookshelf
    - VPN — AmneziaWG
6. Подключение по SSH к машинам инфраструктуры
    - К серверу: `ssh server-user@server-ip`
    - К пк: `ssh server-user@server-ip -p 2222`