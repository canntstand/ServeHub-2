# ServeHub-2 Backup and Recovery

## Design

Borg/Borgmatic creates an encrypted local repository on each host. After Borg has completed and the Compose stack has been restored, `scripts/borgmatic_with_metrics.sh` mirrors the repository to S3-compatible object storage with `rclone` and verifies object presence and sizes.

- Workload local repository: `/mnt/backup_storage/server_backup.borg` when `backup_disk_uuid` is configured; otherwise `/var/lib/servehub-backup/workload-repo`.
- Edge local repository: `/var/lib/servehub-backup/edge-repo`.
- Off-site destinations: one private bucket with separate prefixes, e.g. `offsite-s3:<bucket>/workload` and `offsite-s3:<bucket>/edge`.
- Both repositories use the same `borgmatic_encryption_passphrase` but have independent Borg repository IDs.
- Retention: 7 daily, 4 weekly, 6 monthly; compression: `lz4`. Edge runs at 03:00 and Workload at 04:00, each with up to 15 minutes of jitter.

`rclone sync` creates a mirror of the local Borg repository, not an independent Borg repository. Deletes after `prune` are reflected remotely. Before syncing, the script compares the Borg repository ID: a non-empty prefix without a readable `config` or with a different ID is rejected to avoid deleting unrelated data. If the first sync was interrupted before `config` reached S3, inspect and manually clear only that incomplete prefix before retrying. Do not put unrelated files in either prefix, and never run Borg simultaneously against the local repository and its S3 mirror. After sync, `rclone check --one-way --size-only` checks object presence and sizes, not the cryptographic integrity of every byte.

## Initial setup

1. Choose an S3-compatible provider and create a private bucket. Set `offsite_s3_endpoint` to the base S3 API URL, without a bucket name or path. HTTPS is preferred.
2. Set `offsite_s3_region` to the provider's region if required; otherwise leave it empty. The project uses rclone's generic `provider = Other` mode, so the service must support the S3 API compatible with that mode. See the [rclone S3 documentation](https://rclone.org/s3/).
3. Create a dedicated Access Key and Secret Key with permission to list the bucket and read, write, and delete objects. Do not use the account's primary key; scope permissions to the required bucket or prefixes.
4. Set `offsite_s3_endpoint`, `offsite_s3_region`, `offsite_s3_bucket`, `offsite_s3_access_key_id`, `offsite_s3_secret_access_key`, and distinct `workload`/`edge` prefixes in `ansible/vars/secrets.yml`.
5. Use the same `borgmatic_encryption_passphrase` on both hosts (at least 20 characters). Store it separately; without it, encrypted backups cannot be recovered.
6. Re-run the Ansible deployment from `dev`. Ansible verifies read/write/delete access with a temporary object, creates a root-only rclone config, and enables the backup timers.
7. After deployment, trigger the first backup on each host with `sudo systemctl start borgmatic.service`.
8. Review `systemctl status borgmatic.service`, `systemctl list-timers borgmatic.timer`, and `sudo journalctl -u borgmatic.service --since today`.
9. Check Prometheus metrics `servehub_borgmatic_backup_last_run_success`, `servehub_offsite_backup_last_run_success`, `servehub_backup_pipeline_last_run_success`, and `servehub_offsite_backup_last_success_timestamp_seconds`. Failed uploads raise `OffsiteBackupFailed`; a stale off-site copy raises `OffsiteBackupTooOld`.
## Data included

Workload: `apps-data/`, `configs/`, `.env`, `certbot/`, and `PersonalData/`. Edge: `apps-data/`, `configs/`, and `.env`. This includes generated configuration and service state that cannot be rebuilt from Git alone.

Before creating an archive, the script records which Compose services are running, stops the remaining services with a longer graceful-shutdown timeout, and then starts the previously running services again while waiting for readiness. Workload keeps AmneziaWG, Node Exporter and cAdvisor running. Edge keeps only Node Exporter and cAdvisor running; all other Edge Compose services, including wg-easy, AdGuard Home, Prometheus, Grafana, Loki, Alertmanager and Nginx, are stopped and unavailable while the local archive is created. Borg must release its repository lock before remote synchronization starts. Services will be unavailable during the backup window. If Compose restoration fails, a metric and critical alert are emitted.

S3 stores the encrypted Borg repository. rclone receives the S3 access keys but does not get the Borg passphrase or decrypt data. The initial sync can be large; consider storage, operation, and cold-object retrieval pricing.

## Restore from S3

Restore into a separate directory on a trusted recovery host with a compatible Borg version and rclone installed. Do not restore over an active repository.

1. Configure a private rclone remote with the same S3 endpoint, region, Access Key, and Secret Key.
2. Copy the appropriate prefix into an empty directory. For Workload:
   `sudo mkdir -p /srv/restore/workload-repo`
   `sudo rclone --config /etc/servehub/rclone.conf copy offsite-s3:<bucket>/workload /srv/restore/workload-repo --progress`
   For Edge, use `edge` instead of `workload`.
3. Enter a root shell and read the passphrase into the environment rather than placing it in command arguments:
   `sudo -i`
   `read -rsp 'Borg passphrase: ' BORG_PASSPHRASE; export BORG_PASSPHRASE; echo`
4. List archives with `borg list /srv/restore/workload-repo`.
5. Extract the selected archive into a clean directory:
   `mkdir -p /srv/restore/extracted && cd /srv/restore/extracted`
   `borg extract /srv/restore/workload-repo::ARCHIVE_NAME`
6. Afterwards run `unset BORG_PASSPHRASE` and `exit`. Check ownership and permissions before restoring data into production.

A `borg key export` is an optional safeguard for a local repository, but store any key export and the passphrase separately from both servers and S3.

## Limitations and safety

- Keep `workload` and `edge` prefixes separate and dedicated to ServeHub: `rclone sync` deletes destination objects missing from the local repository.
- The S3 mirror reflects source changes and deletions; it does not protect against every form of local repository corruption. Periodically perform a real restore test.
- Do not remove older backups until restore tests have succeeded for both hosts.
- S3 keys are not committed to Git. On both hosts they are stored in `/etc/servehub/rclone.conf` with mode `0600`.
