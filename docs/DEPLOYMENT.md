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
