# ServeHub-2 3.0.0 — Release Notes

Version 3.0.0 is a major revision of the infrastructure architecture and deployment workflow. These notes summarize the changes from the previous project baseline in the `main` branch.

## Highlights

- **New Edge / Workload two-node architecture.** Edge/VPS provides VPN access, DNS filtering, monitoring, and alerting. Workload hosts user applications and persistent data, with web access routed through Nginx and TLS.
- **Ubuntu Server 24.04 LTS only.** Debian and Arch Linux are no longer supported deployment targets.
- **Hermes Agent is integrated into Edge and is now required.** Its dashboard is intended for VPN/HTTPS access. State is stored under `apps-data/hermes/`, and `apps-data/hermes-workspace/` is mounted as `/workspace`. Setup requires a compatible API endpoint, API key, model identifier, and dashboard secret.
- **Actual Budget added** for self-hosted budgeting, with persistent data in `apps-data/actual-budget/`.
- **Availability monitoring redesigned.** Gatus was removed; HTTP/TLS probes now use Prometheus and Blackbox Exporter, with a Grafana uptime dashboard.
- **Backup workflow redesigned.** Borg/Borgmatic creates encrypted local repositories, and rclone mirrors them to separate S3 prefixes for Edge and Workload. The pipeline includes destination-safety checks, object-size verification, end-to-end metrics, alerts, and a Grafana backup-status dashboard.
- **Web architecture updated.** Nginx configuration is consolidated into a shared template for service subdomains; Certbot uses Webnames DNS-01 on Workload.
- **Deployment and security improved.** Bootstrap and preflight checks were revised, firewall rules account for Docker DNAT through `DOCKER-USER`, and Fail2ban integration is preserved. Secret inputs, Compose/Ansible configuration, and monitoring rules are validated.
- **GUI installer updated.** Select a published release, Git tag, or `dev`; the selected source is pinned to a commit SHA. Installation directory, downloaded project integrity, and secret configuration checks have been improved.
- **Automated CI checks added** for secrets, YAML/Ansible, Compose, firewall syntax, Prometheus rules, source dependencies, and container images.

## Removed or replaced components

- **Gatus** was replaced by Prometheus + Blackbox Exporter and a Grafana dashboard.
- **BorgUI** was replaced by the Grafana backup-status dashboard and Borgmatic metrics.
- **Portainer and Portainer Agent** are no longer included in the Compose stacks.
- The separate Compose setup for an optional Hermes VPN client is no longer the deployment path: Hermes now runs as a required Edge service.

## Compatibility and operational notes

- **Migration from older versions is not officially supported.** Manual migration is technically possible and has been tried by the author, but it is not automated and does not guarantee compatibility of configuration, application data, or backups. A fresh installation on clean Ubuntu Server 24.04 LTS hosts, followed by selective and verified data migration, is recommended.
- **Backups cause temporary service downtime.** During local Borg archive creation, Compose services are stopped and subsequently started again, except for services explicitly kept running. On Edge, this affects VPN and monitoring as well.
- **The S3 copy is a mirror, not an independent immutable archive.** `rclone sync` reflects deletions from the local repository. `--size-only` verifies object presence and sizes, but is not a substitute for a restore test. Test recovery and store the Borg passphrase separately from the servers and S3.
- **Before production use, verify** VPN, DNS/TLS, application access, Prometheus targets, alert delivery, and restoration of both backup repositories.

For installation, operations, and recovery instructions, see the [documentation index](docs/README.md). Detailed operational guides are currently in Russian; the English project overview is in [README-ENG.md](README-ENG.md).
