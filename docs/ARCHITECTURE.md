> **AI-generated documentation:** Документ подготовлен с помощью ИИ; содержание сверено с кодом ветки `dev`.

# Архитектура

ServeHub-2 разделяет инфраструктуру на Edge/VPS и Workload. Edge предоставляет VPN, DNS-фильтрацию, мониторинг, оповещения и Hermes; Workload запускает пользовательские приложения и Nginx.

~~~mermaid
flowchart TB
  Internet((Интернет)) -->|UDP 51820| Edge
  Admin[Администратор] -->|VPN / AmneziaWG| Edge
  Edge[Edge / VPS<br/>10.8.0.1] <-->|Туннель 10.8.0.0/24| Workload[Workload / home server<br/>10.8.0.2]
  Workload --> Nginx[Nginx + TLS] --> Apps[Self-hosted приложения]
  Edge --> Monitoring[Prometheus / Grafana / Loki / Alertmanager]
  Edge --> Hermes[Hermes Agent]
  Edge --> BackupE[Borgmatic → S3: edge]
  Workload --> BackupW[Borgmatic → S3: workload]
~~~

## Адреса и интерфейсы

| Компонент | Значение |
|---|---|
| Edge VPN | `10.8.0.1`, интерфейс `wg0` |
| Workload VPN | `10.8.0.2`, интерфейс `awg0` |
| VPN-подсеть | `10.8.0.0/24` |
| Внутренняя сеть Nginx/приложений | `10.254.240.0/24` |

Клиентский профиль хранится в `apps-data/amnezia/awg0.conf`. Nginx на Workload является входной точкой к приложениям; Prometheus собирает метрики с обоих узлов, Alloy отправляет логи в Loki. Borgmatic создаёт локальные зашифрованные репозитории, rclone зеркалирует их в разные S3-префиксы.

## Сетевые границы

Docker host networking используется VPN и отдельными мониторинговыми компонентами. Защита Docker-публикаций реализована не только через `INPUT`, но и через `DOCKER-USER`, поскольку публикация портов использует DNAT. `INSECURE=true` разрешает HTTP для wg-easy: порт TCP 51821 нельзя открывать публично; административный доступ предназначен для HTTPS reverse proxy через VPN. Edge принимает SSH для управления и UDP 51820 для VPN.

## Структура

- `ansible/deploy.yml` — общий порядок развёртывания и postflight.
- `ansible/tasks/bootstrap_os.yml` — первичная настройка ОС и SSH.
- `ansible/tasks/setup_remote.yml`, `setup_local.yml` — Edge и Workload.
- `ansible/tasks/setup_backup.yml` — резервное копирование.
- `docker-compose.remote.yaml`, `docker-compose.local.yaml` — сервисные стеки.
- `ansible/templates/` — генерируемые конфигурации; `configs/` — конфиги сервисов.
- `apps-data/`, `PersonalData/` — постоянные данные, требуемые для восстановления.
