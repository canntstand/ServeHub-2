> **AI-generated documentation:** Документ подготовлен с помощью ИИ; содержание сверено с кодом ветки `dev`.

# Установка

## Требования

- Два узла Ubuntu Server 24.04 LTS: публичный Edge/VPS и Workload, доступный по SSH.
- Docker Engine и Docker Compose на компьютере, где запускается GUI: Ansible выполняется в контейнере.
- SSH/root или существующий пользователь с sudo для bootstrap.
- Домен, Webnames API key, Telegram-бот и заранее созданный S3 bucket.
- Исходящий доступ к пакетным репозиториям, GitHub, Webnames API и S3.

Используйте чистые тестовые узлы. Развёртывание изменяет SSH, firewall, маршрутизацию и Docker.

## GUI-установщик

1. Выберите родительский каталог; проект будет в подкаталоге `ServeHub-2`.
2. Выберите стабильный релиз, Git-тег или `dev`. Установщик закрепляет выбранный исходный код полным commit SHA.
3. Заполните и проверьте `ansible/vars/secrets.yml`.
4. Выберите сценарий установки и пройдите preflight.
5. Дождитесь окончания Ansible и postflight.

Сценарии: 1 — полный Edge+Workload; 2 — сервисы Edge+Workload без bootstrap; 3 — полный Workload с уже подготовленным Edge; 4 — Workload без bootstrap с проверкой Edge; 5 — полный Edge; 6 — Edge без bootstrap. Сценарии 3/4 требуют здорового `wg-easy` на Edge.

## Secrets

Для CLI можно начать с шаблона:

~~~bash
cp ansible/vars/secrets.yml.example ansible/vars/secrets.yml
chmod 600 ansible/vars/secrets.yml
~~~

Важные группы: IP/пользователи (`vps_public_ip`, `vps_user`, `local_private_ip`, `local_user`), начальные SSH/sudo/root credentials, `server_name`, `email`, `admin_user`, `admin_password`, Telegram, `webnames_apikey`, PostgreSQL/Redis, SSH deploy key, `borgmatic_encryption_passphrase`, S3 endpoint/bucket/keys/prefixes, Hermes API URL/key/model и dashboard secret.

`admin_password` должен иметь минимум 12 символов; Borg passphrase — минимум 20. PostgreSQL password используется в URL, поэтому задавайте URL-safe значение. Hermes dashboard secret — 64 hex-символа. Endpoint S3 — базовый HTTPS URL без bucket и пути; бакет должен существовать. GUI ожидает обычный YAML: Ansible Vault не поддерживается сквозным процессом.

## Первый VPN-клиент Workload

Во время полного развёртывания Workload настройте профиль клиента через wg-easy. SSH-туннель к UI:

~~~bash
ssh -L 8000:127.0.0.1:51821 <vps_user>@<vps_public_ip>
~~~

Откройте `http://127.0.0.1:8000`, создайте клиента с адресом `10.8.0.2`, экспортируйте профиль и сохраните на Workload как `apps-data/amnezia/awg0.conf`. Права должны быть `0600`. Не открывайте TCP 51821 публично.

## DNS и CLI

Приложения используют отдельные поддомены; предполагаемый доступ идёт через VPN на `10.8.0.2`. Certbot на Workload использует Webnames DNS-01 для wildcard-сертификата. `./scripts/manage_deploy.sh` — CLI-альтернатива, но по умолчанию плейбук использует `dev` и не предоставляет тот же процесс выбора версии, что GUI.
