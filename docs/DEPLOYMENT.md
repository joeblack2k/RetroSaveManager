# Deployment Notes

## Profiles

- `direct`: single container on host port 80
- `macvlan`: single container on LAN with its own IP

## Internal DNS (AdGuard)

Create an internal DNS rewrite for your chosen hostname:

- Hostname: `retrosavemanager.lan` (example)
- Target: `<docker-host-ip>`

Then set `PUBLIC_HOST` in `deploy/.env` to that hostname.

## Update Flow

Use pull-based update on your Docker host:

```bash
cd deploy
./scripts/pull-up.sh direct
```

Switch `direct` to `macvlan` when needed.

Notes:

- `pull-up.sh` is the canonical production path
- `up.sh` now only restarts from the locally cached image
- `build-local.sh` is the explicit opt-in path for local image builds

## Persistence

- App state/config volume: `CONFIG_HOST_PATH` (maps to container `/config`)
- Save data volume: `SAVES_HOST_PATH` (maps to container `/saves`)
- Recommended host paths:
  - `CONFIG_HOST_PATH=/config/retrosavemanager/config`
  - `SAVES_HOST_PATH=/config/retrosavemanager/saves`

## Save Layout Migration

After upgrading, migrate older slug folders to display folders:

```bash
./scripts/migrate-save-layout.sh --dry-run
./scripts/migrate-save-layout.sh --manifest ./deploy/data/config/save-layout-manifest.json
```

Rollback if needed:

```bash
./scripts/migrate-save-layout.sh --rollback --manifest ./deploy/data/config/save-layout-manifest.json
```

## Save Rescan And Noise Prune

After deploy, run a deep rescan to improve system detection and prune unsupported/noise saves:

```bash
./scripts/rescan-saves.sh --dry-run
./scripts/rescan-saves.sh --prune-unsupported=true
```

Docker deploy (without local Go toolchain):

```bash
docker compose exec app /usr/local/bin/retrosave-api rescan-saves --prune-unsupported=true
```

## Optional Metadata Enrichment

Set these backend env vars in `deploy/.env` to enable live cover lookup:

- `IGDB_CLIENT_ID`
- `IGDB_CLIENT_SECRET`
- `RAWG_API_KEY`

## Backup Retention

If you keep host-side deploy backups outside the repo checkout, prune them with:

```bash
cd deploy
./scripts/prune-backups.sh --root /srv/retrosavemanager/backups --keep-recent 4 --keep-days 7 --dry-run
./scripts/prune-backups.sh --root /srv/retrosavemanager/backups --keep-recent 4 --keep-days 7
```

Behavior:

- keeps the newest `N` backup directories regardless of age
- also keeps any backup directory newer than `KEEP_DAYS`
- only prunes immediate child directories below the chosen backup root
- never touches `deploy/data/config` or `deploy/data/saves`
- respects an optional `.keep` marker inside a backup directory

For host automation, run the same script from a systemd timer or cron job with explicit `--root`, `--keep-recent`, and `--keep-days` values.

Example systemd install:

```bash
sudo install -D -m 0644 deploy/systemd/retrosavemanager-backup-retention.service /etc/systemd/system/retrosavemanager-backup-retention.service
sudo install -D -m 0644 deploy/systemd/retrosavemanager-backup-retention.timer /etc/systemd/system/retrosavemanager-backup-retention.timer
sudo tee /etc/default/retrosavemanager-backup-retention >/dev/null <<'EOF'
RSM_REPO_DIR=/srv/retrosavemanager/app
RSM_BACKUP_ROOT=/srv/retrosavemanager/backups
RSM_KEEP_RECENT=4
RSM_KEEP_DAYS=7
EOF
sudo systemctl daemon-reload
sudo systemctl enable --now retrosavemanager-backup-retention.timer
sudo systemctl start retrosavemanager-backup-retention.service
```
