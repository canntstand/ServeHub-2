![ServeHub-2](icons/ServeHub-2-logo-matte-neutral.svg)

![Version](https://img.shields.io/github/v/release/canntstand/ServeHub-2?label=version)
![License](https://img.shields.io/github/license/canntstand/ServeHub-2)
![Last commit](https://img.shields.io/github/last-commit/canntstand/ServeHub-2)
![Stars](https://img.shields.io/github/stars/canntstand/ServeHub-2?style=social)

[Русская версия](README.md)

# ServeHub-2

ServeHub-2 automates a self-hosted infrastructure split across two nodes: a remote Edge/VPS and a local Workload server. Edge provides VPN access, DNS filtering, monitoring, and infrastructure services; Workload runs user-facing applications and stores persistent data.

The project targets **Ubuntu Server 24.04 LTS**. Ansible handles deployment, and the Go/Wails GUI installer lets you select a release, Git tag, or the `dev` branch.

## Features

- **Applications:** Nextcloud, Vaultwarden, Gitea, Navidrome, Audiobookshelf, Homebox, Habitica, and Actual Budget.
- **Networking and security:** AmneziaWG/wg-easy, AdGuard Home, Nginx, Certbot with Webnames DNS-01, SSH keys, and Fail2ban.
- **Monitoring:** Prometheus, Grafana, exporters, Blackbox Exporter, Loki, Grafana Alloy, and Alertmanager with Telegram notifications.
- **Backups:** Borg/Borgmatic and encrypted repository mirrors in S3-compatible object storage.
- **Hermes Agent:** an Edge component with a persistent workspace for files the agent is allowed to access.

## Enabling wg-easy Prometheus metrics

Prometheus metrics are disabled by default. In wg-easy, open **Admin Panel → General** and enable **Prometheus**. The scrape job is already configured in the project; see the [components guide](docs/COMPONENTS.md) for target verification and authentication details. Do not expose TCP port `51821` to the public internet.

## Prerequisites

Prepare the following before deployment:

- **Two Ubuntu Server 24.04 LTS hosts:** a public Edge/VPS and a Workload host reachable over SSH. Clean hosts are recommended.
- **Domain and DNS:** a domain managed through Webnames, a Webnames API key for DNS-01, and both the apex A record and wildcard `*` record pointing to the Workload VPN address `10.8.0.2`. Do not point VPN-only services at the public Edge IP.
- **Telegram:** a bot token and chat ID if Alertmanager notifications are required.
- **S3:** a pre-created private S3-compatible bucket, API endpoint, region if required, and Access/Secret Keys with object read, write, and delete permissions. Restrict the credentials to the required bucket/prefixes. The project does not create the bucket automatically.
- **Access and secrets:** SSH access and required root/sudo privileges, plus application and host values for `ansible/vars/secrets.yml`. Never commit this file; set mode `0600`. Store `borgmatic_encryption_passphrase` separately from the servers and S3.
- **GUI host:** Docker Engine and Docker Compose must be installed and running on the computer launching the GUI installer; Ansible runs in a container.
- **Recommended test environment:** Vagrant, VirtualBox, and a network interface with internet access. Choose the correct bridged interface and account for VPN routes so SSH access to test VMs remains available.

Deployment changes SSH, firewall, routing, and Docker. Test on clean VMs first and retain provider-console access to the VPS.

## Quick start

1. Launch the GUI installer from `gui-installer/`, or use `scripts/manage_deploy.sh`.
2. Select a project version and verify the pinned commit SHA.
3. Configure and validate `ansible/vars/secrets.yml`, keeping its permissions restrictive.
4. Verify VPN access, TLS, applications, monitoring, and alert delivery after deployment.
5. Perform an actual restore test before relying on backups.

## Documentation

See the [ServeHub-2 documentation index](docs/README.md) for all guides. **All project documentation is currently available in Russian only.** Documents are marked as AI-generated; verify commands against the current `dev` branch before use.

## Testing

Test VMs can be managed from the project root:

```bash
vagrant up
vagrant halt
vagrant destroy -f
```

See the [installation guide](docs/INSTALLATION.md) for VM, DNS/VPN, `secrets.yml`, S3, and GUI installer build instructions.