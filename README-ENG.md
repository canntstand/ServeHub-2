![ServeHub-2-logo.svg](ServeHub-2-logo.svg)

- [README Russian version](https://github.com/canntstand/ServeHub-2/blob/main/README.md)

This project provides a fast way to deploy a two-server architecture (local + remote) pre-configured with specific services to handle specialized tasks. It was primarily created for my personal use, as well as for anyone looking for a ready-to-use, out-of-the-box self-hosted server complete with a full maintenance system.

While hosting everything on a single high-performance VPS would be simpler, it would also incur significant recurring costs. To balance performance and budget, this project adopts a hybrid approach: resource-heavy services run on a local machine, while a lightweight remote VPS provides secure internet access to them. Alternatively, if your budget permits, you can easily lease cloud servers instead of running hardware at home.

- [Setup Guide](https://github.com/canntstand/ServeHub-2/blob/main/SETUP_GUIDE-ENG.md)

## Core Services
- **Nextcloud:** Cloud storage for files, contacts, and photos.
- **Vaultwarden:** A lightweight password manager compatible with the Bitwarden API for secure, encrypted data storage.
- **Gitea:** A lightweight system of managing Git versions.
- **Navidrome:** A personal music collection streaming server.
- **Audiobookshelf:** A server for audiobooks and podcasts with cross-device progress synchronization.

## Network Architecture & Security
- **Webnames:** A domain name registrar used for domain ownership and managing DNS records (A-records, Wildcard) required for SSL. (SSL certificates are obtained specifically via Webnames because the domain was purchased there).
- **WG-easy + AmneziaWG:** An encrypted tunnel between your devices and the VPS that hides your internal infrastructure. Access to services is strictly restricted to the VPN subnet `10.8.0.0/24`; a web panel is also included for easy management.
- **AdGuard Home:** A local DNS server featuring built-in ad and tracker blocking.
- **Nginx (Proxy):** Acts as a reverse proxy, blocking any external requests that do not originate from within the internal tunnel network.
- **SSH:** Configured for secure access using `ed25519` keys, with password authentication completely disabled.
- **Certbot:** Automation tool for obtaining wildcard SSL certificates via DNS-01 challenges integrated with the Webnames API.
- **Fail2ban:** System-level protection against brute-force attacks.

## Monitoring & Control
- **Prometheus & Exporters:** Gathers metrics from the host (Node Exporter), containers (cAdvisor), and the web server itself (Nginx Exporter).
- **Grafana:** Visualizes system health, performance metrics, and service status.
- **Gatus:** Monitors local server uptime by querying a `healthcheck` endpoint every minute.
- **Portainer:** A web-based interface for streamlined Docker container administration.
- **Loki + Grafana Alloy:** Centralizes log collection from all running containers into a single location.

## Alerting & Communication
- **Alertmanager:** Handles alerting for critical events occurring on the local server.
- **Gatus:** Monitors and sends alerts regarding local and remote server operations, as well as SSL certificate expiration checks.
- **TG Bot Integration:** Utilizes a Telegram bot to deliver instant notifications from both Alertmanager and Gatus.

## Backup System
- **BorgBackup + Borgmatic:** Secure, automated, and encrypted backups written to disk utilizing `lz4` compression.
- **BorgUI:** A user-friendly web interface for easily browsing and managing backups.

## Automation
- **Ansible:** A configuration management system used for automated OS deployment, security hardening, and Docker container orchestration.
- **Vagrant:** Facilitates rapid deployment of virtual machines to test Ansible playbooks locally.

## User Experience
- **Homepage:** A sleek, customizable dashboard for convenient browser-based access to all hosted services.