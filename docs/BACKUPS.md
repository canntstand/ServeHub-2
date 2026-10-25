# Резервное копирование и восстановление ServeHub-2

## Архитектура

Borg/Borgmatic создаёт зашифрованный локальный репозиторий на каждом сервере. После завершения Borg и восстановления Compose-стека скрипт `scripts/borgmatic_with_metrics.sh` синхронизирует репозиторий с Timeweb Cloud S3 через `rclone` и проверяет, что объекты назначения присутствуют и имеют те же размеры.

- Workload: локальный репозиторий — `/mnt/backup_storage/server_backup.borg`, если задан `backup_disk_uuid`; иначе — `/var/lib/servehub-backup/workload-repo`.
- Edge: локальный репозиторий — `/var/lib/servehub-backup/edge-repo`.
- Off-site: один приватный бакет, например `timeweb-cloud:<bucket>/workload` и `timeweb-cloud:<bucket>/edge`.
- Оба репозитория используют единый `borgmatic_encryption_passphrase`, но имеют независимые идентификаторы Borg.
- Retention: 7 daily, 4 weekly, 6 monthly; сжатие `lz4`. Таймер Edge — 03:00, Workload — 04:00, задержка каждого запуска до 15 минут.

`rclone sync` создаёт зеркало локального Borg-репозитория, а не независимый Borg-репозиторий. В зеркале отражаются удаления после `prune`, поэтому не помещайте другие данные в префиксы `workload` и `edge`. Перед синхронизацией скрипт сверяет ID Borg-репозитория: непустой префикс без читаемого файла `config` или с другим ID не синхронизируется, чтобы не удалять чужие данные. Если самая первая выгрузка оборвалась до загрузки `config`, осмотрите и вручную очистите только соответствующий неполный префикс перед повтором. Не запускайте Borg одновременно против локального репозитория и его S3-зеркала. После синхронизации выполняется `rclone check --one-way --size-only`: это проверяет наличие файлов и размеры, но не является полным криптографическим аудитом всех байтов.

## Перед первым запуском

1. Создайте в Timeweb Cloud приватный бакет холодного класса. Создайте дополнительного S3-пользователя с правами чтения и записи только для этого бакета; не используйте главный ключ аккаунта.
2. Заполните в `ansible/vars/secrets.yml` параметры `offsite_s3_bucket`, `offsite_s3_access_key_id`, `offsite_s3_secret_access_key`, endpoint/region и разные префиксы `workload`/`edge`.
3. Задайте один и тот же `borgmatic_encryption_passphrase` на обоих серверах (не менее 20 символов). Сохраните его отдельно: потеря парольной фразы делает резервные копии недоступными.
4. Повторно выполните Ansible-деплой из ветки `dev`. Ansible установит Borgmatic и rclone, проверит чтение/запись/удаление тестового объекта в S3, создаст root-only-конфигурации и настроит таймеры. Ветка `main` не используется.
5. После завершения деплоя выполните первый запуск вручную на каждом сервере: `sudo systemctl start borgmatic.service`.
6. Проверьте состояние и журнал: `systemctl status borgmatic.service`, `systemctl list-timers borgmatic.timer`, `sudo journalctl -u borgmatic.service --since today`.
7. Проверьте Prometheus: `servehub_borgmatic_backup_last_run_success`, `servehub_offsite_backup_last_run_success`, `servehub_backup_pipeline_last_run_success` и `servehub_offsite_backup_last_success_timestamp_seconds`. Ошибки выгрузки поднимут `OffsiteBackupFailed`; отсутствие свежей выгрузки — `OffsiteBackupTooOld`.

## Что входит в копии

Workload: `apps-data/`, `configs/`, `.env`, `certbot/` и `PersonalData/`. Edge: `apps-data/`, `configs/` и `.env`. Это включает сгенерированные конфиги и состояние сервисов, которое не восстанавливается только из Git.

Перед созданием архива скрипт фиксирует список работающих Compose-сервисов, останавливает остальные сервисы с увеличенным тайм-аутом завершения и затем повторно запускает ранее работавшие сервисы, ожидая их готовности. Workload оставляет активными AmneziaWG, Node Exporter и cAdvisor. На Edge остаются только Node Exporter и cAdvisor; остальные Compose-сервисы, включая wg-easy, AdGuard Home, Prometheus, Grafana, Loki, Alertmanager и Nginx, временно остановлены и будут недоступны до окончания локального архивирования. Перед выгрузкой в S3 Borg должен завершить работу и отпустить lock. На время backup-секции сервисы будут недоступны; это ожидаемо. Если восстановление Compose не удастся, появится метрика и критический alert.

S3-копия состоит из зашифрованного Borg-репозитория. rclone получает ключи доступа, но не парольную фразу Borg и не расшифровывает данные. Первая синхронизация может быть большой; учитывайте стоимость хранения, операций и восстановления холодных объектов.

## Восстановление из S3

Восстанавливайте репозиторий в отдельный каталог на доверенной машине с совместимой версией Borg и rclone. Не восстанавливайте поверх существующего рабочего репозитория.

1. Настройте приватный rclone-конфиг с теми же S3 endpoint, region, Access Key и Secret Key.
2. Скопируйте соответствующий префикс в пустой каталог. Например, для Workload:
   `sudo mkdir -p /srv/restore/workload-repo`
   `sudo rclone --config /etc/servehub/rclone.conf copy timeweb-cloud:<bucket>/workload /srv/restore/workload-repo --progress`
   Для Edge замените `workload` на `edge`.
3. Откройте root shell и безопасно передайте парольную фразу в окружение, не записывая её в аргументы команды:
   `sudo -i`
   `read -rsp 'Borg passphrase: ' BORG_PASSPHRASE; export BORG_PASSPHRASE; echo`
4. Посмотрите архивы: `borg list /srv/restore/workload-repo`.
5. Создайте отдельный каталог и извлеките выбранный архив из него:
   `mkdir -p /srv/restore/extracted && cd /srv/restore/extracted`
   `borg extract /srv/restore/workload-repo::ARCHIVE_NAME`
6. Завершив работу, выполните `unset BORG_PASSPHRASE` и `exit`. Перед возвратом данных проверьте владельцев и права.

Для локального репозитория Workload `borg key export` можно использовать как дополнительную меру, но сохраняйте экспорт отдельно от серверов и S3. Парольную фразу также храните отдельно.

## Ограничения

- Префиксы `workload` и `edge` должны быть разными и использоваться только этим проектом: `rclone sync` удаляет в целевом префиксе объекты, отсутствующие в локальном репозитории.
- S3-зеркало отражает изменения и удаления исходного репозитория; оно не защищает от всех типов повреждения локальной копии. Периодически проверяйте реальное восстановление.
- Пока тест восстановления не завершён успешно для Workload и Edge, не удаляйте старые резервные копии.
- S3-ключи не хранятся в Git. На серверах они находятся в `/etc/servehub/rclone.conf` с правами `0600`.
