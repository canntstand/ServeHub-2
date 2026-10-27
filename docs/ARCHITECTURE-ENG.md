> **AI-generated documentation:** Drafted with AI assistance and checked against branch `dev`.

# Architecture

ServeHub-2 separates infrastructure into an Edge/VPS node and a Workload/home-server node. Edge provides VPN access, DNS filtering, monitoring, alerting, and Hermes. Workload hosts user applications and the Nginx entry point.

~~~mermaid
flowchart TB
  Internet((Internet)) -->|UDP 51820| Edge
  Admin[Administrator] -->|AmneziaWG VPN| Edge
  Edge[Edge / VPS<br/>10.8.0.1] <-->|10.8.0.0/24 tunnel| Workload[Workload / home server<br/>10.8.0.2]
  Workload --> Nginx[Nginx + TLS] --> Apps[Self-hosted applications]
  Edge --> Monitoring[Prometheus / Grafana / Loki / Alertmanager]
  Edge --> Hermes[Hermes Agent]
  Edge --> BackupE[Borgmatic → S3: edge]
  Workload --> BackupW[Borgmatic → S3: workload]
~~~

## Network and repository layout

| Item | Value |
|---|---|
| Edge VPN address | `10.8.0.1` |
| Workload VPN address | `10.8.0.2` |
| VPN subnet | `10.8.0.0/24` |
| Application Docker network | `10.254.240.0/24` |

Workload's client profile is stored at `apps-data/amnezia/awg0.conf`; it contains sensitive key material. Nginx on Workload is the entry point for web applications. Prometheus scrapes metrics from both hosts; Alloy forwards logs to Loki. Borgmatic creates encrypted local repositories and rclone mirrors them to separate S3 prefixes.

Key files: `ansible/deploy.yml` orchestrates deployment; `ansible/tasks/bootstrap_os.yml` configures the OS and access; `setup_remote.yml` and `setup_local.yml` configure Edge and Workload; `setup_backup.yml` configures backups. Compose definitions are `docker-compose.remote.yaml` and `docker-compose.local.yaml`. `ansible/templates/` contains templates; `configs/` holds service configuration; `apps-data/` and `PersonalData/` hold persistent data.

## Security boundaries

Docker-published ports require the `DOCKER-USER` chain in addition to ordinary host INPUT rules. Do not expose wg-easy TCP 51821 publicly. The configuration permits HTTP inside wg-easy, so use the HTTPS reverse proxy over VPN for the UI. Edge needs SSH for administration and UDP 51820 for VPN.
