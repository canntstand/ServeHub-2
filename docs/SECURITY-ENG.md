> **AI-generated documentation:** Drafted with AI assistance and checked against branch `dev`.

# Security and trust boundaries

## Secrets

Never commit `ansible/vars/secrets.yml`, private SSH keys, `.env`, Webnames API keys, Telegram tokens, S3 credentials, or the Borg passphrase. Set `secrets.yml` permissions to `0600`. Atomic installer writes do not encrypt the file. The GUI edits plain YAML; Ansible Vault is not integrated end-to-end.

On hosts, S3 configuration lives in `/etc/servehub/rclone.conf` and must be root-only. Do not paste secrets into logs, issues, or public files.

## SSH and Docker privileges

The deployment user belongs to the `docker` group; Docker access on Linux is effectively root-equivalent. Treat this account as administrative and use a dedicated SSH deployment key. Ansible accepts unknown host keys on first contact via `StrictHostKeyChecking accept-new`; investigate changes to known keys through a trusted channel.

## Firewall and VPN

Edge is intended to accept SSH administration and inbound UDP 51820 for VPN. Nginx and firewall rules restrict service access to the VPN subnet. Never expose TCP 51821 (wg-easy UI) publicly: wg-easy is configured to permit HTTP, so its administrative UI should be behind the HTTPS reverse proxy over VPN. Do not remove Fail2ban chains or change `DOCKER-USER` without understanding routing. Keep VPS console access available.

## Hermes

Hermes can access its configuration, tools, and `/workspace`. Every file in `apps-data/hermes-workspace/` is accessible to the agent; avoid placing secrets there unless intentional. The Basic Auth Dashboard should be available over VPN/HTTPS; do not publish port 9119.

## Backups

Borg encrypts archives, but S3 keys can read, write, and delete objects. Use a dedicated least-privilege key. `rclone sync` mirrors deletions, so the S3 mirror does not protect against every data-loss scenario. Store the Borg passphrase separately from the hosts and S3. Object-size checks do not replace restore tests. See [BACKUPS-ENG.md](BACKUPS-ENG.md).
