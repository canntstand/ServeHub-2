# Компоненты

> **Документация создана с помощью ИИ.** Содержание сверено с кодом ветки `dev`; перед применением команд проверяйте актуальные файлы проекта.

## Ansible и GUI

`ansible/deploy.yml` выполняет bootstrap, синхронизацию кода, роли Edge/Workload и postflight. `bootstrap_os.yml` готовит Ubuntu, SSH, пользователя развёртывания, Docker и AmneziaWG. `setup_remote.yml` настраивает Edge, `setup_local.yml` — Workload, `setup_backup.yml` — Borgmatic/S3. GUI на Go/Wails выбирает релиз/тег/`dev`, закрепляет SHA, проверяет целостность файлов и запускает Ansible в Docker.

## VPN и веб

Edge использует `ghcr.io/wg-easy/wg-easy:15.5.0`, `EXPERIMENTAL_AWG=true`, `OVERRIDE_AUTO_AWG=awg`; Workload использует интерфейс `awg0`. Профиль `apps-data/amnezia/awg0.conf` содержит приватные ключи. Изменение подсети `10.8.0.0/24` требует согласования Compose, firewall, Nginx, Prometheus и клиентских профилей.

Nginx на Workload — входная точка к веб-приложениям. Certbot работает через профиль `maintenance`, используя Webnames DNS-01 hook. Перед reload Nginx проверяйте конфигурацию через `nginx -t`.

## Workload приложения

| Сервис | Назначение | Данные |
|---|---|---|
| Homepage | Каталог сервисов | `configs/homepage/config/` |
| Nextcloud | Файлы | `apps-data/nextcloud/`, `PersonalData/NextcloudData/` |
| Vaultwarden | Менеджер паролей | `apps-data/vaultwarden/data/`, PostgreSQL |
| Gitea | Git | `apps-data/gitea/` |
| Navidrome | Музыка | `apps-data/navidrome/data/`, `PersonalData/Music/` |
| Audiobookshelf | Аудиокниги | `apps-data/audiobookshelf/`, `PersonalData/` |
| Habitica | Привычки | `apps-data/habitica-db/` |
| Homebox | Учёт имущества | `apps-data/homebox/`, PostgreSQL |
| Actual Budget | Бюджет | `apps-data/actual-budget/` |

PostgreSQL хранится в отдельных каталогах `apps-data/postgres/`. Не удаляйте каталоги данных при обновлении Compose.

## Мониторинг Edge

Prometheus читает `configs/monitoring/prometheus.generated.yml` и `alert_rules.yml`; Blackbox проверяет HTTP/TLS и сертификаты; Node Exporter/cAdvisor собирают метрики; Grafana загружает дашборды из `configs/grafana/`; Alloy пересылает логи в Loki; Alertmanager отправляет уведомления в Telegram.

### Prometheus-метрики wg-easy

Метрики wg-easy по умолчанию выключены. В панели wg-easy откройте **Admin Panel → General** и включите **Prometheus**. Endpoint экспортёра: `http://10.8.0.1:51821/metrics/prometheus`.

Scrape job `wg-easy` уже задан в `ansible/templates/prometheus.yml.j2`. После включения метрик в панели проверьте в интерфейсе Prometheus **Status → Targets**: цель `wg-easy` должна иметь состояние `UP`.

Не публикуйте TCP-порт `51821` в интернете. В текущем scrape job авторизация не настроена. Если в wg-easy задан **Bearer Password**, настройте соответствующую авторизацию и в Prometheus; не сохраняйте пароль или токен в отслеживаемых Git-файлах. См. [официальную документацию wg-easy по Prometheus](https://wg-easy.github.io/wg-easy/latest/advanced/metrics/prometheus/) и выберите в ней версию, соответствующую используемому образу.

## Hermes

Hermes обязателен на Edge. Ansible создаёт `apps-data/hermes/.env` и `config.yaml` только при первом развёртывании, чтобы не перезаписывать Dashboard settings. `apps-data/hermes-workspace/` монтируется как `/workspace` и доступна агенту: не размещайте там секреты без необходимости. Dashboard защищён Basic Auth и предназначен для HTTPS/VPN на `https://hermes.<домен>/`; не публикуйте порт 9119.

## Постоянные данные

`ansible/templates/` содержит шаблоны runtime-конфигураций; `.env` и `ansible/vars/secrets.yml` содержат секреты; `apps-data/` и `PersonalData/` необходимы для восстановления и не восстанавливаются клонированием Git.
