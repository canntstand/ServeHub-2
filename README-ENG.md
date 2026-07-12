![ServeHub-2-logo.svg](ServeHub-2-logo.svg)
![Version](https://img.shields.io/github/v/release/canntstand/ServeHub-2?label=version)
![License](https://img.shields.io/github/license/canntstand/ServeHub-2)
![Last Commit](https://img.shields.io/github/last-commit/canntstand/ServeHub-2)
![Stars](https://img.shields.io/github/stars/canntstand/ServeHub-2?style=social)

- [README Russian version](https://github.com/canntstand/ServeHub-2/blob/main/README.md)

# Table of Contents
- [Brief Overview](#brief-overview)
- [Main dependencies](#main-dependencies)
- [Important Information Before Deployment](#important-information-before-deployment)
- [Testing with Vagrant](#testing-with-vagrant)

This project provides a fast way to recreate an architecture consisting of 2 servers (local + remote) with specific services that solve dedicated tasks. The project was primarily created for my own use, but also for people who want a "ready-to-go" self-hosted server out of the box with a complete maintenance system (see below).

It is worth clarifying that hosting everything on one powerful VPS would be easier; however, that would require significantly higher costs for maintaining such a server. Therefore, I chose a hybrid approach: resource-intensive services run on a local machine, while a remote VPS helps organize secure access to them from the internet. Alternatively, if budget allows, one can easily rent servers instead of hosting them at home.

# Brief Overview
## Main Services
- **Nextcloud:** Cloud storage for files, contacts, and photos.
- **Vaultwarden:** Lightweight password manager (Bitwarden API compatible) for storing encrypted data.
- **Gitea:** Lightweight Git-based version control system.
- **Navidrome:** Personal music collection streaming.
- **Audiobookshelf:** Server for audiobooks and podcasts with progress synchronization.

## Network and Security
- **Webnames:** Domain name registrar. Used for domain ownership and DNS record management (A-records, Wildcard) required for SSL. (Certificates are obtained via Webnames specifically, as my domain was purchased there).
- **WG-easy + AmneziaWG:** Encrypted tunnel between devices and the VPS, hiding the infrastructure. Access to services is strictly possible from within the `10.8.0.0/24` VPN subnet; a web interface for management is included.
- **AdGuard Home:** Local DNS server with ad and tracker blocking functionality.
- **Nginx:** Acts as a reverse proxy, blocking any external requests that do not belong to the internal tunnel network.
- **SSH:** Setup with `ed25519` keys and password authentication disabled.
- **Certbot:** Automation for obtaining SSL (Wildcard) certificates via DNS-01 challenge, integrated with the Webnames API.
- **Fail2ban:** System-level protection against brute-force attacks.

## Architecture
- **Docker + Docker Compose:** Deploying all applications in containers using compose files.

## Monitoring and Control
- **Prometheus & Exporters:** Collection of host metrics (Node Exporter), container metrics (cAdvisor), and web server metrics (Nginx Exporter).
- **Grafana:** Visualization of system state and service performance.
- **Gatus:** Uptime monitoring of the local server via minute-by-minute `healthcheck` endpoint polling.
- **Portainer:** Web interface for Docker container administration.
- **Loki + Grafana Alloy:** Unified log collection from all containers.

## Alerting and Communication
- **Alertmanager:** Alerting for critical events on the local server.
- **Gatus:** Monitoring and alerting for local/remote server health, as well as SSL certificate expiration checks.
- **TG Bot Integration:** Telegram bot for receiving notifications from Alertmanager and Gatus.

## Backup
- **BorgBackup + Borgmatic:** Encrypted automated backups to disk with `lz4` compression.
- **BorgUI:** Interface for easy backup management and viewing.

## Automation
- **Ansible:** Configuration management system for automatic OS deployment, security hardening, and Docker container orchestration.
- **Go and Wails Installer:** A user-friendly installer that automates the deployment process. (It's only in Russian for now)
- **Vagrant:** Rapid deployment of virtual machines for testing Ansible playbooks.
- **Bash scripts:** Automating certain steps using .sh scripts.

## User Experience
- **Homepage:** A beautiful dashboard for easy access to services in the browser.

# Main dependencies
- **[Docker Engine](https://docs.docker.com/compose/install/)/[Docker Desktop](https://docs.docker.com/desktop/)**
- **[Docker Compose](https://docs.docker.com/compose/install/)**
- **SSH**

# Important Information Before Deployment
1. **Supported OS:** The project is designed to run on Ubuntu, Debian, and Arch Linux (7.1.3-arch1-2).
2. **DNS Configuration:** In the registrar's DNS panel (e.g., Webnames), you must point the primary domain A-record and the wildcard record (`*`) to the internal VPN IP address — **`10.8.0.1`**. This is critical: all traffic to your services must travel inside the secure AmneziaWG tunnel. If you point the domain to the public IP of the VPS, Nginx will block external requests due to the `deny all` rule.
3. **VPN for Vagrant:** HashiCorp is currently inaccessible from Russia, so a VPN is required to run tests. (After setting up the machines, you can turn off the VPN or configure split-tunneling to ignore the `192.168.0.0/24` range; otherwise, you will be unable to connect to the test VMs via SSH).
4. **Backup Preservation:** After setting up the local server, save the Borg key (if backups were configured): `borg key export /mnt/backup_storage/server_backup.borg /home/USER/borg_key_backup.txt`. (It is best to move this file elsewhere; do not keep it on the same server).
5. **wg-easy Configuration:** If there is no connection to the VPN, try accessing the wg-easy interface to adjust settings. (Read more in the [AmneziaWG documentation](https://docs.amnezia.org/documentation/amnezia-wg)).
6. **Ansible Playbook Errors:** At the beginning of execution, errors might appear when connecting as `root`. If the script does not terminate and deployment is happening on servers that already contain the users specified in `secrets.yml`, this is normal. In this case, the script should proceed by connecting as the created users.
7. **Reliability Guarantee:** The project's functionality is only guaranteed on completely clean servers, or servers already configured using this project. Otherwise, operation is not guaranteed. Functionality is also not guaranteed in the event of connection issues.

# Testing with Vagrant
1. Install:
    - **[Vagrant](https://developer.hashicorp.com/vagrant/install)**
    - **[VirtualBox](https://www.virtualbox.org/wiki/Downloads)**
2. On the host system, in the project root, run `vagrant up` to create the test servers.
    - When prompted, **"Which interface should the network bridge to?"**, select the interface your host uses to connect to the internet.
3. During deployment, the username and password in `secrets.yml` must be `vagrant`.
4. To stop the virtual machines, use `vagrant halt`.
5. To completely remove the machines, use `vagrant destroy -f`.

# Building the installer from the code
1. Установите:
    - **[Go](https://go.dev/doc/install)**
    - **[Wails](https://wails.io/docs/gettingstarted/installation)**
2. Go to the folder `/gui-installer`
3. Run the command `wails build` (It may vary, it is better to follow the Wails documentation.)
