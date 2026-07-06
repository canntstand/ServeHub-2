# Infrastructure Setup Guide

## ⚠️ Important
1. **Supported OS:** The project is designed to run on Ubuntu, Debian, and Arch Linux.
2. **DNS Configuration:** In your registrar's DNS panel (e.g., Webnames), you must point the main domain's A-record and the wildcard record (`*`) to the internal VPN IP address — **`10.8.0.1`**. This is critical: all traffic to your services must route through the secure AmneziaWG tunnel. If you point the domain to the public IP of the VPS, Nginx will block external requests due to the `deny all` rule.
3. **VPN for Vagrant:** HashiCorp services are currently unavailable in Russia, so you will need a VPN to spin up the test environment. (Turn off the VPN after the machines are provisioned, or enable split-tunneling to ignore `192.168.0.0/24`, otherwise you won't be able to connect to the test VMs via SSH).
4. **Backup Security:** After setting up the local server, remember to safeguard your Borg key (if the backup system was configured): `borg key export /mnt/backup_storage/server_backup.borg /home/USER/borg_key_backup.txt`. (It is best to move this file to another secure location; do not store it on the same server).
5. **wg-easy Configuration:** If you cannot connect to the VPN, try accessing the wg-easy interface to tweak parameters. (For more details, refer to the [AmneziaWG documentation](https://docs.amnezia.org/documentation/amnezia-wg)).

## Installation
### Linux
1. Install:
    - **Docker** + **Docker Compose**
    - **Git**
    - **SSH**
2. Prepare 2 Linux servers with SSH enabled (Ubuntu/Debian/Arch).
3. Clone the repository: `git clone https://github.com/canntstand/ServeHub-2 && cd ServeHub-2`
4. Create the `ansible/vars/secrets.yml` file based on the template `ansible/vars/secrets.yml.example`.
5. Generate an SSH key: `ssh-keygen -t ed25519 -C "your_email@example.com"`
6. Run the deployment script: `chmod +x manage_deploy.sh && ./manage_deploy.sh` (for a clean install on fresh servers, select option 1).
7. The script will guide you through the remaining steps.

### Windows
1. Install:
    - **Docker** + **Docker Compose**
    - **Git**
    - **SSH**
    - **WSL + a Distribution**
    - **A terminal with Bash support**
2. Prepare 2 Linux servers with SSH enabled (Ubuntu/Debian/Arch).
3. Open your WSL distribution terminal (all subsequent steps must be executed strictly inside it).
4. Clone the repository (anywhere inside `/mnt/c/`): `git clone https://github.com/canntstand/ServeHub-2 && cd ServeHub-2`
5. Create the `ansible/vars/secrets.yml` file based on the template `ansible/vars/secrets.yml.example`.
6. Generate an SSH key and set the correct permissions: `ssh-keygen -t ed25519 -C "your_email@example.com" && chmod 600 ~/.ssh/id_ed25519`
7. Run the deployment script: `chmod +x manage_deploy.sh && ./manage_deploy.sh` (for a clean install on fresh servers, select option 1).
8. The script will guide you through the remaining steps.

### Backup Configuration (Optional Add-on to Standard Installation)
1. Ensure the backup drive is connected to the server.
2. Specify the drive's UUID in `secrets.yml`. (You can find it using `sudo blkid`).
3. Once the local architecture is deployed, log into Borg UI (the username is always `admin`) and follow these steps:
    1. Select repository import (specify the path `/mnt/backup_storage/server_backup.borg`).
    2. Select **Observability Only** and **Read-only storage access** (Borgmatic handles the backups; Borg UI is strictly for viewing).
    3. Enter the passphrase you defined in `secrets.yml`.
    4. Select `lz4` as the compression type.
    5. Import the repository.
4. **Reminder:** Do not, under any circumstances, configure backups directly through the Borg UI!
5. To enable metrics, navigate to Settings -> System -> Metrics Access and toggle off "Enable /metrics endpoint".

### Additional Settings (Optional Add-on to Standard Installation)
- Adding remote server containers to Portainer:
    1. Select the **Environments** section.
    2. Click **Add environment** (Docker Standalone).
    3. Connection method: **Agent**.
    4. Environment URL: `10.8.0.1:9001`.

### Local Server Autonomy Settings
1. To ensure the server boots back up automatically after a power outage, locate the following settings in your BIOS (listed as named on my system; titles may vary):
    1. **State After G3:** set to `S0 State`
    2. **Wake system from S5:** `Disabled`

### Testing with Vagrant
1. Install (additional prerequisites):
    - **Vagrant**
    - **VirtualBox**
2. In the root of the project on your host machine, run `vagrant up` to provision the test servers.
    - When prompted with **Which interface should the network bridge to?**, select the network interface your host uses to access the internet.
3. Follow the standard installation guide for the rest of the steps.
4. To stop the virtual machines, use `vagrant halt`. (You can view other available commands with `vagrant --help`).

## Using the Services
1. **Accessing the Portal:** Once successfully launched, turn on the AmneziaWG VPN client on your device and navigate to your domain to view the main dashboard (web portal).
2. **Initial App Configuration:** For Navidrome, Audiobookshelf, Vaultwarden, and Portainer, you will need to complete a quick admin account creation wizard upon your first login. To access the system administration panel for Vaultwarden, use the token defined in the `secret_vaultwarden_password` variable.
3. **Automatic Deployment:** Nextcloud, Grafana, and other core services are provisioned automatically. Your credentials for the first login correspond to the global `admin_user` and `admin_password` variables set in your `secrets.yml`.
4. **Recommended Apps for Daily Use:**
    - Cloud Storage — *Nextcloud*
    - Password Manager — *Bitwarden*
    - DNS Server Management — *AdGuard Home Manager*
    - Music Collection — *Tempo*
    - Audiobooks & Podcasts — *Audiobookshelf*
    - VPN Client — *AmneziaWG/WG Tunnel*
5. **Direct SSH Access Inside the Secure VPN Tunnel:**
    - To the remote server (VPS): `ssh user@10.8.0.1`
    - To the local server: `ssh user@10.8.0.2`
    - *(Note: These private IP addresses are only accessible when the AmneziaWG VPN client is actively running on your local workstation or smartphone).*
6. **Directory Management:** (When migrating the project, copying these folders is essential)
    - The `~/ServeHub-2/apps-data` directory (present on both local and remote servers) contains application data—it is vital to preserve this folder during migrations.
    - The `~/PersonalData` directory holds all user files: documents, photos, videos, music, etc.