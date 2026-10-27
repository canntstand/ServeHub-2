> **Документация создана с помощью ИИ.** Содержание сверено с кодом ветки `dev`; перед применением команд проверяйте актуальные файлы проекта.

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

## Контейнеры, метрики или алерты

Проверьте логи, права каталогов, свободное место, переменные окружения, healthcheck и Prometheus targets. Не удаляйте `apps-data/` или volumes. Для бэкапов смотрите метрики `servehub_borgmatic_backup_last_run_success`, `servehub_offsite_backup_last_run_success`, `servehub_backup_pipeline_last_run_success`. Реальное восстановление следует проверять отдельно.
