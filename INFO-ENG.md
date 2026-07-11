# Additional Project Information
1.  **Supported Operating Systems:** The project is designed to work on Ubuntu, Debian, Arch Linux (7.1.3-arch1-2).
2.  **DNS Configuration:** In your registrar's DNS panel (e.g., Webnames), you must point the A-record of your main domain and the wildcard record (`*`) to the internal VPN IP address — **`10.8.0.1`**. This is critically important: all traffic to your services must go through the secured AmneziaWG tunnel. If you point the domain to the public IP address of the VPS server, Nginx will block external requests due to the `deny all` rule.
3.  **VPN for Vagrant:** HashiCorp currently does not work in Russia, so you will need a VPN to run the tests. (After setting up the machines, the VPN should be turned off, or you can enable split-tunneling to ignore the `192.168.0.0/24` range, as otherwise you won't be able to connect to the test virtual machines via SSH.)
4.  **Backup Preservation:** After setting up the local server, it is worth saving the Borg key (if the backup system was configured): `borg key export /mnt/backup_storage/server_backup.borg /home/USER/borg_key_backup.txt`. (It is better to move this file to another location; do not store it on the same server.)
5.  **wg-easy Configuration:** If there is no VPN connection, you can try accessing the wg-easy interface and adjusting the settings. (More details can be found in the [AmneziaWG documentation](https://docs.amnezia.org/documentation/amnezia-wg).)
6.  **Errors During Ansible Playbook Execution:** Errors may appear at the very beginning when connecting as root. If the script does not terminate and deployment is happening on servers where the users specified in `secrets.yml` already exist — this is normal. In that case, the script should connect using the created users.
7.  **Warranty of Operation:** The project's operation is guaranteed only on completely clean servers, or if the server has already been configured using this project. Otherwise, there is no guarantee of operation. There is also no guarantee of operation if you have internet connectivity issues.

### Setting Up Convenient Backup Browsing
1.  Ensure that the backup disk is connected to the server.
2.  Specify the disk UUID in `secrets.yml`. (You can find it using `sudo blkid`.)
3.  After deploying the local architecture, log into Borg UI (username is always `admin`) and follow these steps:
    1.  Choose to import a repository (specify the name `/mnt/backup_storage/server_backup.borg`).
    2.  Select **Observability Only** and **Read-only storage access** (backups are managed by borgmatic; Borg UI is for viewing only).
    3.  Enter the passphrase that was set in `secrets.yml`.
    4.  Select the compression type `lz4`.
    5.  Import the repository.
4.  **Reminder:** Backups must never be configured through Borg UI!
5.  To enable metrics, go to **Settings** -> **System** -> **Metrics Access** and disable the "Enable /metrics endpoint" option.

### Portainer Configuration
-   **Adding Remote Server Containers to Portainer:**
    1.  Select the **Environments** option.
    2.  Click **Add environment** (Docker Standalone).
    3.  Choose the connection method: **Agent**.
    4.  In the **Environment URL** field, enter: `10.8.0.1:9001`.

### Local Server Autonomy Settings
1.  In order for the server to turn back on after a power outage, you need to find the following settings in the BIOS (I'm writing them as they were named on my system; names may vary):
    1.  **State After G3:** select `S0 State`.
    2.  **Wake system from S5:** `Disabled`.

### Testing with Vagrant
1.  Install:
    -   **Vagrant**
    -   **VirtualBox**
2.  On the host system, run `vagrant up` from the project root to create the servers for testing.
    -   When asked **Which interface should the network bridge to?**, select the interface through which the host connects to the internet.
3.  Otherwise, everything follows the standard guide.
4.  To stop the virtual machines, you can use `vagrant halt`. (Other commands can be viewed using `vagrant --help`.)

## Using the Services
1.  **Portal Login:** After a successful launch, navigate to your domain's main page (web portal), having first enabled the AmneziaWG VPN client on your device.
2.  **Initial Application Setup:** For Navidrome, Audiobookshelf, Vaultwarden, Portainer, and Gitea, you must complete a quick admin account creation procedure upon first opening. To access the Vaultwarden admin panel, use the token set in the `secret_vaultwarden_password` variable.
3.  **Automatic Deployment:** Services like Nextcloud, Grafana, and others are configured automatically. The initial login credentials for them correspond to the global variables `admin_user` and `admin_password` from your `secrets.yml`.
4.  **Recommended Applications for Daily Use:**
    -   Cloud storage — *Nextcloud*
    -   Password manager — *Bitwarden*
    -   DNS server management — *AdGuard Home Manager*
    -   Music collection — *Tempo*
    -   Audiobooks and podcasts — *Audiobookshelf*
    -   VPN client — *AmneziaWG / WG Tunnel*
5.  **Direct SSH Connection Within the Secured VPN Tunnel:**
    -   To the remote server (VPS): `ssh user@10.8.0.1`
    -   To the local server: `ssh user@10.8.0.2`
    -   *(Note: these private IP addresses will only be accessible for connection when the AmneziaWG VPN client is running and active on your current working device — laptop or smartphone.)*
6.  **Folder Usage** (when migrating the project, the most important thing is to copy these folders):
    -   The `~/ServeHub-2/apps-data` folder (exists on both the local and remote servers) stores application data. It is important to preserve this folder when migrating the project.
    -   The `~/PersonalData` folder stores all user data: documents, photos, videos, music, etc.