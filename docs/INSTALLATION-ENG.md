> **AI-generated documentation:** Drafted with AI assistance and checked against branch `dev`.

# Installation

## Prerequisites

- Two Ubuntu Server 24.04 LTS hosts: a public Edge/VPS and a Workload host reachable over SSH.
- Docker Engine and Docker Compose on the computer running the GUI installer; Ansible runs in a container.
- Root access for bootstrap or an existing sudo-capable deployment user for non-bootstrap scenarios.
- A domain, Webnames API key, Telegram bot credentials, and a pre-created private S3 bucket.
- Outbound access to package repositories, GitHub, Webnames API, and S3.

Use clean test VMs first. Deployment changes SSH, firewall, routing, and Docker configuration.

## GUI installer

1. Choose a parent directory; the project is placed in a `ServeHub-2` subdirectory.
2. Select a stable release, Git tag, or `dev`. The installer pins the source to a full commit SHA.
3. Enter and validate `ansible/vars/secrets.yml`.
4. Select the deployment scenario and complete preflight checks.
5. Review the result of Ansible and postflight checks.

Scenarios cover full Edge+Workload, service-only deployment without bootstrap, Workload with an already configured Edge, and Edge-only deployment. Workload scenarios that depend on Edge require a healthy wg-easy service there.

## Secrets

For CLI setup, start from the example if available:

~~~bash
cp ansible/vars/secrets.yml.example ansible/vars/secrets.yml
chmod 600 ansible/vars/secrets.yml
~~~

The configuration covers host IPs/users, bootstrap credentials, domain/email, application admin credentials, Telegram, Webnames, database passwords, SSH deployment key, Borg passphrase, S3 endpoint/bucket/keys/prefixes, and Hermes API/model/dashboard settings. Keep the file out of Git and backups that are not encrypted. The GUI expects regular YAML; Ansible Vault is not an end-to-end supported workflow.

Use at least 20 characters for the Borg passphrase. Ensure database passwords are safe for their configured URL format. S3 endpoint should be the base API URL without bucket/path, and the bucket must exist before deployment.

## VPN client profile

During full deployment, create a Workload client profile in wg-easy through an SSH tunnel rather than exposing the admin port publicly:

~~~bash
ssh -L 8000:127.0.0.1:51821 <vps_user>@<vps_public_ip>
~~~

Open `http://127.0.0.1:8000`, create a client with address `10.8.0.2`, export the profile, and save it on Workload as `apps-data/amnezia/awg0.conf` with restrictive permissions. Do not expose TCP 51821 to the Internet.

## DNS and deployment verification

Configure the intended domain and wildcard DNS records to resolve to Workload's VPN address `10.8.0.2` for VPN-only access. Workload Certbot uses Webnames DNS-01 for certificate issuance. After installation verify Compose health checks, HTTPS through VPN, monitoring targets, alert delivery, and a successful backup. Do not treat a successful installer exit as a substitute for testing service functionality and restore.
