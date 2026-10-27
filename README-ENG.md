![ServeHub-2](icons/ServeHub-2-logo-matte-neutral.svg)

![Version](https://img.shields.io/github/v/release/canntstand/ServeHub-2?label=version)
![License](https://img.shields.io/github/license/canntstand/ServeHub-2)
![Last commit](https://img.shields.io/github/last-commit/canntstand/ServeHub-2)
![Stars](https://img.shields.io/github/stars/canntstand/ServeHub-2?style=social)

[Русская версия](README.md)

# ServeHub-2

ServeHub-2 automates a self-hosted infrastructure split across two nodes: a remote Edge/VPS and a local Workload server. Edge provides VPN access, network restrictions, monitoring, and infrastructure services; Workload runs applications and stores persistent data.

The project targets clean **Ubuntu Server 24.04 LTS** hosts. Deployment is automated with Ansible, and a Go/Wails GUI installer helps select a project version and run deployment.

## Main features

- **Applications:** Nextcloud, Vaultwarden, Gitea, Navidrome, Audiobookshelf, Homebox, Habitica, and Actual Budget.
- **Networking and security:** AmneziaWG/wg-easy, AdGuard Home, Nginx, Certbot with Webnames DNS-01, SSH keys, and Fail2ban.
- **Monitoring:** Prometheus, Grafana, exporters, Blackbox Exporter, Loki, Grafana Alloy, and Alertmanager with Telegram notifications.
- **Backups:** Borg/Borgmatic and mirroring of encrypted repositories to S3-compatible storage.
- **Hermes Agent:** runs on Edge and has a persistent workspace for files the agent is allowed to access.

## Before deployment

- Prepare two Ubuntu Server 24.04 LTS hosts: a public Edge/VPS and a Workload host reachable over SSH.
- The GUI installer requires Docker Engine and Docker Compose on the computer running it.
- Prepare a domain, Webnames API key, Telegram notification credentials, and a pre-created private S3 bucket.
- For VPN-only access, domain and wildcard DNS records should point to the Workload VPN address `10.8.0.2`, not the public Edge IP.
- Deployment changes SSH, firewall, networking, and Docker configuration. Test on clean VMs first and retain provider-console access to the VPS.

## Installation

1. Launch the GUI installer from `gui-installer/`, or use `scripts/manage_deploy.sh`.
2. Select the project version and verify its pinned commit SHA.
3. Configure `ansible/vars/secrets.yml`, restrict it to mode `0600`, and never commit secrets.
4. After deployment, verify VPN-only service access, monitoring, notifications, and an actual backup restore.

## Documentation

See the [ServeHub-2 documentation index](docs/README.md) for all guides. **All project documentation is currently available in Russian only.** It is marked as AI-generated and covers architecture, components, installation, operations, security, troubleshooting, and backup/recovery.

## Testing

Vagrant and VirtualBox are used for Ansible testing:

```bash
vagrant up
vagrant halt
vagrant destroy -f
```

See the [installation guide](docs/INSTALLATION.md) for VM prerequisites and GUI installer build instructions.
