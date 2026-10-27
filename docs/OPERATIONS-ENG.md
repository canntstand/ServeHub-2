> **AI-generated documentation:** Drafted with AI assistance and checked against branch `dev`.

# Operations

Run commands on the relevant host from the project directory (usually `/home/<deploy_user>/ServeHub-2`).

## Status and logs

Edge: `docker compose -f docker-compose.remote.yaml ps`, `docker compose -f docker-compose.remote.yaml logs --tail=200 <service>`.

Workload: `docker compose -f docker-compose.local.yaml ps`, `docker compose -f docker-compose.local.yaml logs --tail=200 <service>`.

Add `-f` to follow logs. Validate Compose without starting it with `docker compose -f docker-compose.local.yaml config --quiet`. Never delete `apps-data/` or volumes as a startup troubleshooting shortcut.

## Updates

Prefer the GUI installer: select a release/tag/branch, verify the pinned commit SHA, then run the update. Preserve custom changes to managed files and confirm a recent backup first. `scripts/manage_deploy.sh` is a CLI alternative, not an equivalent GUI version-selection workflow.

After updating, verify containers, health checks, VPN access, Prometheus targets, and logs on both hosts.

## Firewall and SSH

Check `sudo systemctl status servehub-firewall.service` and `sudo journalctl -u servehub-firewall.service -n 100 --no-pager`. Docker-published ports pass through `DOCKER-USER`, while Fail2ban manages its own chains. Keep provider-console access available during network changes.

## Certificates

Check `sudo systemctl list-timers certbot-renew.timer`, run `sudo systemctl start certbot-renew.service`, and inspect `sudo journalctl -u certbot-renew.service -n 200 --no-pager`. Verify DNS, `server_name`, system time, Webnames API, and `certbot/certs/` on failure.

## Backups

Use `sudo systemctl status borgmatic.timer`, `sudo systemctl start borgmatic.service`, and `sudo journalctl -u borgmatic.service -n 200 --no-pager`. Current schedules are around 03:00 for Edge and 04:00 for Workload, with random delay. Services may be stopped during local archive creation; most Edge VPN/monitoring applications become unavailable.

Monitor `servehub_borgmatic_backup_last_run_success`, `servehub_offsite_backup_last_run_success`, `servehub_backup_pipeline_last_run_success`, and `servehub_offsite_backup_last_success_timestamp_seconds`. See [BACKUPS-ENG.md](BACKUPS-ENG.md). S3 prefixes are mirrors, not independent recovery points; perform actual restore tests periodically.

## Hermes and Actual Budget

Hermes is intended at `https://hermes.<domain>/` over HTTPS/VPN. Its persistent workspace `apps-data/hermes-workspace/` is mounted as `/workspace`; treat all files there as accessible to the agent. Actual Budget uses `apps-data/actual-budget/`; store its E2E encryption password separately.
