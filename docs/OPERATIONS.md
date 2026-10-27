> **AI-generated documentation:** Документ подготовлен с помощью ИИ и сверяется с реализацией ветки `dev`.

# Эксплуатация

Команды выполняйте на соответствующем сервере из каталога проекта (обычно `/home/<deploy_user>/ServeHub-2`).

## Состояние и журналы

Edge: `docker compose -f docker-compose.remote.yaml ps`, `docker compose -f docker-compose.remote.yaml logs --tail=200 <service>`.

Workload: `docker compose -f docker-compose.local.yaml ps`, `docker compose -f docker-compose.local.yaml logs --tail=200 <service>`.

Добавьте `-f` к `logs` для потокового просмотра. Проверить Compose без запуска можно командой `docker compose -f docker-compose.local.yaml config --quiet`. Не удаляйте `apps-data/` или volumes для исправления ошибок запуска.

## Обновление

Предпочтительный путь — GUI-установщик: выберите релиз/тег/ветку, проверьте закреплённый commit SHA и запустите обновление. Перед обновлением сохраните пользовательские изменения в управляемых файлах и убедитесь, что есть свежая резервная копия. CLI-скрипт `scripts/manage_deploy.sh` — альтернативный путь, но не следует считать его интерфейсом выбора версии, эквивалентным GUI.

После обновления проверьте контейнеры, healthcheck, доступ к приложениям через VPN, Prometheus targets и журналы на обоих узлах.

## Firewall и SSH

Проверьте службу восстановления firewall: `sudo systemctl status servehub-firewall.service`, журнал: `sudo journalctl -u servehub-firewall.service -n 100 --no-pager`. Не редактируйте правила вслепую: Docker-публикации проходят через `DOCKER-USER`, а Fail2ban использует собственные цепочки. Перед сетевыми изменениями обеспечьте доступ к консоли провайдера.

## Сертификаты

Проверьте расписание `sudo systemctl list-timers certbot-renew.timer`, запустите проверку `sudo systemctl start certbot-renew.service`, изучите `sudo journalctl -u certbot-renew.service -n 200 --no-pager`. При ошибках проверьте DNS, `server_name`, время на сервере, Webnames API и каталог `certbot/certs/`.

## Резервное копирование

Используйте `sudo systemctl status borgmatic.timer`, `sudo systemctl start borgmatic.service`, `sudo journalctl -u borgmatic.service -n 200 --no-pager`. По текущей конфигурации Edge планируется около 03:00, Workload — около 04:00, с дополнительным случайным сдвигом. Во время создания локального архива сервисы могут быть остановлены; на Edge недоступность затрагивает большинство приложений VPN/мониторинга.

Проверяйте метрики `servehub_borgmatic_backup_last_run_success`, `servehub_offsite_backup_last_run_success`, `servehub_backup_pipeline_last_run_success` и `servehub_offsite_backup_last_success_timestamp_seconds`. Детали и восстановление — [BACKUPS.md](BACKUPS.md). S3-префиксы являются зеркалами, а не независимыми точками восстановления; периодически выполняйте фактическое тестовое восстановление.

## Hermes и Actual Budget

Hermes доступен через HTTPS/VPN по `https://hermes.<домен>/`; его постоянная рабочая папка `apps-data/hermes-workspace/` смонтирована агенту как `/workspace`. Считайте все файлы этой папки доступными агенту. Actual Budget использует `apps-data/actual-budget/`; пароль E2E-шифрования бюджета следует хранить отдельно.
