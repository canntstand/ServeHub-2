# Устранение неполадок

## GUI не запускает Ansible

Проверьте `docker info` и `docker compose version`: установщик запускает Ansible внутри контейнера, поэтому Docker Engine должен работать. Проверьте доступность GitHub и выбранного источника версии.

## SSH или bootstrap не проходит

Проверьте IP, имя пользователя, выбранный сценарий, приватный ключ для указанного публичного ключа и sudo-права. Сценарии без bootstrap требуют, чтобы SSH и sudo уже работали. Не отключайте host-key checking глобально. При изменении host key проверьте переустановку сервера через консоль провайдера.

## AmneziaWG/DKMS не загружается

Убедитесь, что система — Ubuntu Server 24.04 LTS, установлены headers для текущего ядра и после установки модуля выполнена необходимая перезагрузка. Диагностика: `uname -r`, `modinfo amneziawg`, `sudo modprobe amneziawg`.

## Workload preflight не видит Edge

На Edge проверьте `docker compose -f docker-compose.remote.yaml ps wg-easy` и `docker compose -f docker-compose.remote.yaml logs --tail=200 wg-easy`. Для сценариев, зависящих от Edge, недостаточно статуса `running`: требуется успешный healthcheck. На Workload проверьте профиль `apps-data/amnezia/awg0.conf`, адрес `10.8.0.2`, интерфейс `awg0` и доступность Edge по VPN.

## Nginx или TLS не работает

Проверьте DNS, `server_name`, системное время, Webnames API key и наличие сертификата в `certbot/certs/live/<домен>/`. Выполните `docker compose -f docker-compose.local.yaml config --quiet`, затем `docker compose -f docker-compose.local.yaml exec -T nginx nginx -t`. Логи Certbot: `sudo journalctl -u certbot-renew.service -n 200 --no-pager`.

## S3 или Borgmatic завершается ошибкой

Проверьте базовый S3 endpoint без bucket/пути, существование бакета, регион, ключи и права чтения/записи/удаления объектов. Проверьте `sudo journalctl -u borgmatic.service -n 300 --no-pager` и `sudo systemctl status borgmatic.timer`. Не публикуйте `/etc/servehub/rclone.conf`. Если ID или конфигурация существующего репозитория не совпадают, не удаляйте его для обхода проверки.

## wg-easy target не имеет состояния UP

1. В wg-easy откройте **Admin Panel → General** и включите **Prometheus**.
2. В Prometheus откройте **Status → Targets**. Для job `wg-easy` должен использоваться endpoint `10.8.0.1:51821/metrics/prometheus`.
3. Если в wg-easy настроен Bearer Password, проверьте, что тот же способ авторизации настроен в scrape job Prometheus.
4. Убедитесь, что TCP-порт `51821` не заблокирован между Prometheus и wg-easy, но не публикуйте его в публичный интернет.

Подробности настройки приведены в [документации компонентов](COMPONENTS.md).

## Hermes не предоставляет terminal или execute_code

Сначала проверьте журнал и состояние инструмента именно в проблемном профиле. Команды выполняйте на Edge из каталога `ServeHub-2`:

```bash
docker compose -f docker-compose.remote.yaml ps hermes
docker compose -f docker-compose.remote.yaml logs --tail=200 hermes
docker exec hermes hermes doctor
docker exec hermes hermes --profile main-bot tools --summary
docker exec hermes hermes --profile main-bot config get terminal.backend
```

Замените `main-bot` на имя нужного профиля. Список инструментов, показанный командой `tools --summary`, сам по себе не гарантирует, что инструмент передан модели в Telegram; проверьте секцию Telegram и воспроизведите реальный вызов из нового диалога.

Конфигурация может отличаться между профилями, особенно после переноса данных со старого сервера. Если меняете backend, делайте это для соответствующего профиля и перезапускайте единый gateway командой `docker compose -f docker-compose.remote.yaml restart hermes`, затем повторяйте тест. При `terminal.backend: local` команды выполняются в среде Hermes, а не в shell хоста. Не подключайте Docker socket и не включайте privileged-режим как обходной путь.

Рабочая папка внутри контейнера `/workspace` соответствует каталогу `apps-data/hermes-workspace/` на Edge. Если инструмент работает, но не видит файлы, проверьте наличие файлов на хосте, bind mount и права доступа, не выдавая каталогам права `0777`.

## Контейнеры, метрики или алерты

Проверьте логи, права каталогов, свободное место, переменные окружения, healthcheck и Prometheus targets. Не удаляйте `apps-data/` или volumes. Для бэкапов смотрите метрики `servehub_borgmatic_backup_last_run_success`, `servehub_offsite_backup_last_run_success`, `servehub_backup_pipeline_last_run_success`. Реальное восстановление следует проверять отдельно.

