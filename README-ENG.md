![ServeHub-2-logo.svg](ServeHub-2-logo.svg)

- [README Russian version](https://github.com/canntstand/ServeHub-2/blob/main/README.md)

The project is a fast way to recreate an architecture consisting of 2 servers (local + remote) with specific services that solve specific tasks. The project is made primarily for me, as well as for people who want their own ready-made self-hosted server out of the box with a full maintenance system (see below).

It is also worth clarifying that hosting everything on a single powerful VPS would be simpler, but it would require too much expense to pay for such a server. Therefore, I chose a hybrid approach: resource-intensive services run on the local machine, and the remote VPS helps organize secure access to them from the internet. Or again, if you have enough money, you can easily rent servers instead of keeping them at home.

- [Important Information](https://github.com/canntstand/ServeHub-2/blob/main/INFO-ENG.md)

## Core Services
- **Nextcloud:** Cloud storage for files, contacts, and photos.
- **Vaultwarden:** Lightweight password manager (compatible with Bitwarden API) for storing encrypted data.
- **Gitea:** Lightweight version control system based on Git.
- **Navidrome:** Streaming of your personal music collection.
- **Audiobookshelf:** Server for audiobooks and podcasts with progress synchronization.

## Network Architecture and Security
- **Webnames:** Domain name registrar. Used for domain ownership and DNS record management (A-records, Wildcard) required for SSL operation. (Certificate issuance happens specifically through Webnames, since my domain was purchased there, more details below.)
- **WG-easy + AmneziaWG:** Encrypted tunnel between devices and VPS, hiding the infrastructure. Access to services is strictly possible from the VPN subnet `10.8.0.0/24`; a web panel for management is also available.
- **AdGuard Home:** Local DNS server with ad and tracker blocking functionality.
- **Nginx:** Acts as a reverse proxy, blocking any external requests not belonging to the tunnel's internal network.
- **SSH:** Access configuration using `ed25519` keys with password login disabled.
- **Certbot:** Automation of SSL certificate (Wildcard) issuance via DNS-01 challenge integrated with the Webnames API.
- **Fail2ban:** Protection against brute-force attacks at the system level.

## Monitoring and Control
- **Prometheus & Exporters:** Collection of host metrics (Node Exporter), containers (cAdvisor), and the web server itself (Nginx Exporter).
- **Grafana:** Visualization of system status and service performance.
- **Gatus:** Uptime monitoring of the local server via minutely polling of a `healthcheck` endpoint.
- **Portainer:** Web interface for administering Docker containers.
- **Loki + Grafana Alloy:** Collection of logs from all containers in one place.

## Alerting and Communication
- **Alertmanager:** Alerting of critical events on the local server.
- **Gatus:** Monitoring and alerting of local and remote server operation, as well as SSL certificate checks.
- **TG Bot Integration:** Using a Telegram bot to receive notifications from Alertmanager and Gatus.

## Backup
- **BorgBackup + Borgmatic:** Encrypted automated backup to disk with lz4 compression.
- **BorgUI:** Interface for convenient backup browsing.

## Automation
- **Ansible:** Configuration management system for automatic OS deployment, security setup, and Docker container orchestration.
- **Go and Wails Installer:** A convenient installer that will deploy the project itself. (Only in Russian for now)
- **Vagrant:** Fast deployment of virtual machines for testing Ansible playbooks.

## Ease of Use
- **Homepage:** A beautiful homepage for convenient browser-based access to services.