> **AI-generated documentation:** Drafted with AI assistance and checked against branch `dev`.

# ServeHub-2 documentation

- [Architecture](ARCHITECTURE-ENG.md) — Edge/Workload, VPN, and network boundaries.
- [Installation](INSTALLATION-ENG.md) — prerequisites, GUI installer, versions, and secrets.
- [Components](COMPONENTS-ENG.md) — Ansible, Compose, Nginx, Certbot, monitoring, and Hermes.
- [Backup and recovery](BACKUPS-ENG.md) — Borg/Borgmatic and S3.

Target servers: Ubuntu Server 24.04 LTS. The project uses an Edge/VPS and a Workload/home server. Branch `dev` is for development; successful CI does not replace end-to-end installation and recovery tests on clean VMs.
