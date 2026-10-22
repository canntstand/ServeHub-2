![ServeHub-2-logo-violet-banner-dark.svg](icons/ServeHub-2-logo-violet-banner-dark.svg)
![Version](https://img.shields.io/github/v/release/canntstand/ServeHub-2?label=version)
![License](https://img.shields.io/github/license/canntstand/ServeHub-2)
![Last Commit](https://img.shields.io/github/last-commit/canntstand/ServeHub-2)
![Stars](https://img.shields.io/github/stars/canntstand/ServeHub-2?style=social)

- [README Russian version](README.md)

# Table of Contents
- [Brief Overview](#brief-overview)
- [Hermes Agent (optional)](#hermes-agent-optional)
- [Main dependencies](#main-dependencies)
- [Important Information Before Deployment](#important-information-before-deployment)
- [Testing with Vagrant](#testing-with-vagrant)

This project provides a fast way to recreate an architecture consisting of 2 servers (local + remote) with specific services that solve dedicated tasks. The project was primarily created for my own use, but also for people who want a "ready-to-go" self-hosted server out of the box with a complete maintenance system (see below).

It is worth clarifying that hosting everything on one powerful VPS would be easier; however, that would require significantly higher costs for maintaining such a server. Therefore, I chose a hybrid approach: resource-intensive services run on a local machine, while a remote VPS helps organize secure access to them from the internet. Alternatively, if budget allows, one can easily rent servers instead of hosting them at home.

# Brief Overview
## Main Services
- **Nextcloud:** Cloud storage for files, contacts, and photos.
- **Vaultwarden:** Lightweight password manager (Bitwarden API compatible) for storing encrypted data.
- **Homebox:** Inventory of Items and Equipment
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
- **Loki + Grafana Alloy:** Unified log collection from all containers.

## Alerting and Communication
- **Alertmanager:** Alerting for critical events on the local server.
- **Gatus:** Monitoring and alerting for local/remote server health, as well as SSL certificate expiration checks.
- **TG Bot Integration:** Telegram bot for receiving notifications from Alertmanager and Gatus.

## Backup
- **BorgBackup + Borgmatic:** Encrypted automated backups to disk with `lz4` compression.

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

# Hermes Agent (Optional)
Hermes Agent can be deployed as an optional third server in ServeHub-2. It is an AI agent designed to automate everyday tasks and interact with infrastructure.

Possible use cases:
- **Monitoring and diagnostics:** analyzing server metrics and logs, generating reports, and sending notifications to Telegram.
- **Gitea integration:** interacting with repositories (for example, my Obsidian vault), reviewing changes, and assisting with development.
- **Automation:** running scheduled tasks and interacting with external service APIs, such as Habitica.

Hermes is deployed separately from the core ServeHub-2 components and is not required for the project to function. It can run on a separate VPS connected to the existing VPN network as an independent client. This allows it to interact with Gitea and other internal services without running the agent on the main server.

Hermes's VPN access should be restricted to the IP addresses and ports it actually needs. Connecting to the VPN should not grant the agent unrestricted access to all devices and services.

## Server Setup for Hermes Agent and Installation

1. Install Docker + Docker Compose
2. Install amneziawg-dkms: https://github.com/amnezia-vpn/amneziawg-linux-kernel-module
3. git clone https://github.com/canntstand/ServeHub-2
4. Download the config and place it at ./apps-data/amnezia/awg0.conf on the server where Hermes will be installed.
5. Run the commands below:
    ```bash
    cat > /etc/sysctl.d/99-servehub-vpn.conf <<'EOF'
    net.ipv4.ip_forward=1
    net.ipv4.conf.all.src_valid_mark=1
    net.ipv6.conf.all.disable_ipv6=0
    net.ipv6.conf.all.forwarding=1
    net.ipv6.conf.default.forwarding=1
    EOF

    sysctl --system
    ```

6. Start the VPN client:
    ```bash
    docker compose -f docker-compose.hermes.yaml up -d
    ```

7. Configure DNS for the awg0 interface:
    ```bash
    cat << 'EOF' | sudo tee /etc/systemd/system/amnezia-dns.service > /dev/null && sudo systemctl daemon-reload && sudo systemctl enable --now amnezia-dns.service
    [Unit]
    Description=Fix Amnezia DNS for awg0
    After=docker.service
    BindsTo=docker.service

    [Service]
    Type=oneshot
    RemainAfterExit=yes
    ExecStartPre=/bin/sh -c 'until ip link show awg0; do sleep 1; done'
    ExecStart=/usr/bin/resolvectl dns awg0 10.8.0.1
    ExecStartPost=/usr/bin/resolvectl domain awg0 ~.

    [Install]
    WantedBy=multi-user.target
    EOF
    ```
8. Install Hermes Agent using the standard method (official website: https://hermes-agent.nousresearch.com):

# Important Information Before Deployment
1. **Supported OS:** The project is designed to run on Ubuntu, Debian, and Arch Linux (7.1.3-arch1-2).
2. **DNS Configuration:** In the registrar's DNS panel (e.g., Webnames), you must point the primary domain A-record and the wildcard record (`*`) to the internal VPN IP address — **`10.8.0.1`**. This is critical: all traffic to your services must travel inside the secure AmneziaWG tunnel. If you point the domain to the public IP of the VPS, Nginx will block external requests due to the `deny all` rule.
3. **VPN for Vagrant:** HashiCorp is currently inaccessible from Russia, so a VPN is required to run tests. (After setting up the machines, you can turn off the VPN or configure split-tunneling to ignore the `192.168.0.0/24` range; otherwise, you will be unable to connect to the test VMs via SSH).
4. **Backup Preservation:** After setting up the local server, save the Borg key (if backups were configured): `borg key export /mnt/backup_storage/server_backup.borg /home/USER/borg_key_backup.txt`. (It is best to move this file elsewhere; do not keep it on the same server).
5. **wg-easy Configuration:** If there is no connection to the VPN, try accessing the wg-easy interface to adjust settings. (Read more in the [AmneziaWG documentation](https://docs.amnezia.org/documentation/amnezia-wg)).
6. **Ansible Playbook Errors:** At the beginning of execution, errors might appear when connecting as `root`. If the script does not terminate and deployment is happening on servers that already contain the users specified in `secrets.yml`, this is normal. In this case, the script should proceed by connecting as the created users.
7. **Reliability Guarantee:** The project's functionality is only guaranteed on completely clean servers, or servers already configured using this project. Otherwise, operation is not guaranteed. Functionality is also not guaranteed in the event of connection issues.
8. **WG-easy main network interface name:** If the main network interface name on the remote server is not eth0, then in the WG-easy panel you need to change the Device value to the actual interface name of the server.

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

# Building the installer from code
## 1. Environmental requirements
Install dependencies before starting the build:
- **[Go 1.21+](https://go.dev/doc/install)**
- **[NPM (PNPM) / Node.js (15+)](https://docs.npmjs.com/downloading-and-installing-node-js-and-npm)**
- **[Wails v2](https://wails.io/docs/gettingstarted/installation)**
- **libwebkit2gtk-4.1-dev** — *Linux only* (required for Wails rendering engine)

## 2. Building a standard binary
1. Go to the `/gui-installer` directory.
2. Run the build command for your OS:
   - **Windows:** `wails build`
   - **Linux:** `wails build -tags webkit2_41`

The executable `gui-installer` (or `gui-installer.exe`) will be saved in `gui-installer/build/bin/`.

## 3. Building a .deb package (Linux)
1. From the project root, run the build script:
```bash
./scripts/build_deb.sh
```

2. Install the compiled package:
```bash
sudo apt install ./gui-installer/build/bin/servehub-installer.deb
```

The finished package servehub-installer.deb will be saved in gui-installer/build/bin/.