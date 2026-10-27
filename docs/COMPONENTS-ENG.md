> **AI-generated documentation:** Drafted with AI assistance and checked against branch `dev`.

# Components

## Ansible and installer

`ansible/deploy.yml` coordinates bootstrap, source synchronization, Edge/Workload roles, and postflight. `bootstrap_os.yml` prepares Ubuntu, SSH, the deployment user, Docker, and AmneziaWG. `setup_remote.yml` configures Edge, `setup_local.yml` configures Workload, and `setup_backup.yml` configures Borgmatic/S3. The Go/Wails GUI selects a release/tag/`dev`, pins a commit SHA, validates inputs, and runs Ansible in Docker.

## VPN and reverse proxy

Edge runs wg-easy with AmneziaWG support; Workload uses the `awg0` interface. The profile `apps-data/amnezia/awg0.conf` contains private key material. Changing VPN subnet `10.8.0.0/24` requires coordinated updates to Compose, firewall, Nginx, monitoring, and client profiles.

Nginx on Workload routes requests to applications. Certbot uses the maintenance profile and Webnames DNS-01 hook for certificates. Validate Nginx with `nginx -t` before reloading.

## Workload applications

| Service | Purpose | Persistent data |
|---|---|---|
| Homepage | Service directory | `configs/homepage/config/` |
| Nextcloud | File storage | `apps-data/nextcloud/`, `PersonalData/NextcloudData/` |
| Vaultwarden | Password manager | `apps-data/vaultwarden/data/`, PostgreSQL |
| Gitea | Git hosting | `apps-data/gitea/` |
| Navidrome | Music streaming | `apps-data/navidrome/data/`, `PersonalData/Music/` |
| Audiobookshelf | Audiobooks/podcasts | `apps-data/audiobookshelf/`, `PersonalData/` |
| Habitica | Habit tracking | `apps-data/habitica-db/` |
| Homebox | Inventory | `apps-data/homebox/`, PostgreSQL |
| Actual Budget | Personal budgeting | `apps-data/actual-budget/` |

PostgreSQL data is stored under `apps-data/postgres/`. Do not delete persistent directories during image updates.

## Edge monitoring

Prometheus reads `configs/monitoring/prometheus.generated.yml` and `alert_rules.yml`. Blackbox Exporter probes HTTP/TLS endpoints; Node Exporter and cAdvisor expose host/container metrics. Grafana loads dashboards from `configs/grafana/`; Alloy forwards logs to Loki; Alertmanager delivers Telegram notifications.

## Hermes and persistent state

Hermes is an Edge component. Ansible creates `apps-data/hermes/.env` and `config.yaml` on initial setup; subsequent runs preserve them to avoid overwriting Dashboard settings. `apps-data/hermes-workspace/` is mounted into the agent as `/workspace`. Treat this folder as accessible to Hermes and avoid placing secrets there. The Dashboard should be accessed over VPN/HTTPS, not through a publicly exposed port.

Generated/runtime configuration, secrets, and persistent service state are not fully recreated by cloning Git. Back up `apps-data/`, `configs/`, `.env`, certificates, and `PersonalData/` according to [BACKUPS-ENG.md](BACKUPS-ENG.md).
