> **AI-generated documentation:** Drafted with AI assistance and checked against branch `dev`.

# Troubleshooting

## GUI does not start Ansible

Check `docker info` and `docker compose version`: the installer runs Ansible in a container, so Docker Engine must be running. Verify GitHub access and the selected source version.

## SSH or bootstrap fails

Check the IP, username, selected scenario, private key matching the configured public key, and sudo privileges. Scenarios without bootstrap require working SSH and sudo beforehand. Do not disable host-key checking globally. If a host key changes, investigate through the provider console.

## AmneziaWG/DKMS does not load

Confirm Ubuntu Server 24.04 LTS, headers for the running kernel, and whether a reboot is required. Diagnose with `uname -r`, `modinfo amneziawg`, and `sudo modprobe amneziawg`.

## Workload preflight cannot verify Edge

On Edge, check `docker compose -f docker-compose.remote.yaml ps wg-easy` and `docker compose -f docker-compose.remote.yaml logs --tail=200 wg-easy`. Dependent scenarios require a successful health check, not just `running`. On Workload, verify `apps-data/amnezia/awg0.conf`, address `10.8.0.2`, interface `awg0`, and VPN reachability to Edge.

## Nginx or TLS fails

Check DNS, `server_name`, system time, Webnames API key, and certificate presence under `certbot/certs/live/<domain>/`. Run `docker compose -f docker-compose.local.yaml config --quiet`, then `docker compose -f docker-compose.local.yaml exec -T nginx nginx -t`. Certbot logs: `sudo journalctl -u certbot-renew.service -n 200 --no-pager`.

## S3 or Borgmatic fails

Verify the base S3 endpoint without bucket/path, bucket existence, region, keys, and object read/write/delete permissions. Inspect `sudo journalctl -u borgmatic.service -n 300 --no-pager` and `sudo systemctl status borgmatic.timer`. Never publish `/etc/servehub/rclone.conf`. If an existing repository ID or configuration differs, do not delete it to bypass validation.

## Containers, metrics, or alerts

Check logs, directory ownership, free disk space, environment values, health checks, and Prometheus targets. Do not delete `apps-data/` or volumes. For backups, monitor `servehub_borgmatic_backup_last_run_success`, `servehub_offsite_backup_last_run_success`, and `servehub_backup_pipeline_last_run_success`. Test actual recovery separately.
