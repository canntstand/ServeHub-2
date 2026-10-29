# Эксплуатация

Команды выполняйте на соответствующем сервере из каталога проекта (обычно `/home/<deploy_user>/ServeHub-2`).

## Состояние и журналы

Edge: `docker compose -f docker-compose.remote.yaml ps`, `docker compose -f docker-compose.remote.yaml logs --tail=200 <service>`.

Workload: `docker compose -f docker-compose.local.yaml ps`, `docker compose -f docker-compose.local.yaml logs --tail=200 <service>`.

Добавьте `-f` к `logs` для потокового просмотра. Проверить конфигурацию Compose без запуска контейнеров можно отдельно на каждом сервере: Edge — `docker compose -f docker-compose.remote.yaml config --quiet`; Workload — `docker compose -f docker-compose.local.yaml config --quiet`. Не удаляйте `apps-data/` или volumes для исправления ошибок запуска.

## Обновление

Для обновления внутри поддерживаемого поколения 3.x используйте GUI-установщик: выберите опубликованный релиз или Git-тег, проверьте закреплённый commit SHA и запустите обновление. Перед обновлением сохраните пользовательские изменения в управляемых файлах и убедитесь, что есть свежая резервная копия. Резервная копия каталога проекта не откатывает изменения в БД и данных приложений.

Переход со старых поколений ServeHub-2 на 3.0.0 не поддерживается. Ручной перенос возможен, но не гарантирован; см. [правила совместимости](INSTALLATION.md#совместимость-и-переход-с-предыдущих-версий).

После обновления проверьте контейнеры, healthcheck, доступ к приложениям через VPN, Prometheus targets и журналы на обоих узлах.

Если target `wg-easy` не имеет состояния `UP`, проверьте, включены ли метрики в **Admin Panel → General → Prometheus** и не задан ли в wg-easy Bearer Password без соответствующей настройки авторизации в scrape job. Не публикуйте TCP-порт `51821` в интернет. Подробности приведены в [документации компонентов](COMPONENTS.md).

## Firewall и SSH

Проверьте службу восстановления firewall: `sudo systemctl status servehub-firewall.service`, журнал: `sudo journalctl -u servehub-firewall.service -n 100 --no-pager`. Не редактируйте правила вслепую: Docker-публикации проходят через `DOCKER-USER`, а Fail2ban использует собственные цепочки. Перед сетевыми изменениями обеспечьте доступ к консоли провайдера.

## Сертификаты

Проверьте расписание `sudo systemctl list-timers certbot-renew.timer`, запустите проверку `sudo systemctl start certbot-renew.service`, изучите `sudo journalctl -u certbot-renew.service -n 200 --no-pager`. При ошибках проверьте DNS, `server_name`, время на сервере, Webnames API и каталог `certbot/certs/`.

## Резервное копирование

Используйте `sudo systemctl status borgmatic.timer`, `sudo systemctl start borgmatic.service`, `sudo journalctl -u borgmatic.service -n 200 --no-pager`. По текущей конфигурации Edge планируется около 03:00, Workload — около 04:00, с дополнительным случайным сдвигом. Во время создания локального архива сервисы могут быть остановлены; на Edge недоступность затрагивает большинство приложений VPN/мониторинга.

Проверяйте метрики `servehub_borgmatic_backup_last_run_success`, `servehub_offsite_backup_last_run_success`, `servehub_backup_pipeline_last_run_success` и `servehub_offsite_backup_last_success_timestamp_seconds`. Детали и восстановление — [BACKUPS.md](BACKUPS.md). S3-префиксы являются зеркалами, а не независимыми точками восстановления; периодически выполняйте фактическое тестовое восстановление.

## Hermes и Actual Budget

Hermes доступен через HTTPS/VPN по `https://hermes.<домен>/`; его постоянная рабочая папка `apps-data/hermes-workspace/` смонтирована агенту как `/workspace`. Считайте все файлы этой папки доступными агенту. Actual Budget использует `apps-data/actual-budget/`; пароль E2E-шифрования бюджета следует хранить отдельно.

