# RetroSaveManager

RetroSaveManager is a self-hosted, LAN-first save synchronization service with compatibility-focused API behavior for existing helper clients.

## Scope

- In scope: user web flows + helper API compatibility
- Out of scope: admin, billing, manager, forge parity
- Auth mode default: `AUTH_MODE=disabled` (trusted internal environment)

## Repository Structure

- `backend/` Go API (compat routes, save store, contracts, tests)
- `frontend/` Modular React + TypeScript web app
- `deploy/` Docker Compose profiles, Caddy gateway configs, deployment scripts
- `contracts/` Route matrix and compatibility contract snapshots
- `scripts/` Contract verification, backup/restore wrappers, security gate
- `tests/` Evidence output folder for local verification runs

## Save Data Layout

All save files live below a single backup-friendly root:

- `SAVE_ROOT/<System Display>/<Game Title or Memory Card>/<save-id>/payload.*`
- `SAVE_ROOT/<System Display>/<Game Title or Memory Card>/<save-id>/metadata.json`

Back up one root folder to capture all saves.

### Layout Migration

Migrate existing slug-based paths to the display layout:

```bash
./scripts/migrate-save-layout.sh --dry-run
./scripts/migrate-save-layout.sh --manifest ./deploy/data/config/save-layout-manifest.json
```

Rollback with the same manifest:

```bash
./scripts/migrate-save-layout.sh --rollback --manifest ./deploy/data/config/save-layout-manifest.json
```

### Save Rescan And Noise Prune

Run a deep rescan to normalize console detection, clean title noise, and optionally remove unsupported/unknown save entries:

```bash
./scripts/rescan-saves.sh --dry-run
./scripts/rescan-saves.sh --prune-unsupported=true
```

Docker-only deploys can run the built-in command directly:

```bash
docker compose exec backend /usr/local/bin/retrosave-api rescan-saves --prune-unsupported=true
```

## Quick Start (Docker)

1. Copy environment template:

```bash
cp deploy/.env.example deploy/.env
```

2. Start direct HTTP profile (`:80`):

```bash
cd deploy
./scripts/up.sh direct
```

3. Optional TLS profile (`:443`, internal certs via Caddy):

```bash
cd deploy
./scripts/up.sh tls
```

4. Optional macvlan profile (container with own LAN IP):

```bash
cd deploy
./scripts/up.sh macvlan
```

## Helper Compatibility

Use your helper client with:

- `RSM_API_URL=http://<internal-hostname-or-ip>`
- `RSM_APP_PASSWORD=<optional-app-password>`

The service exposes compatibility routes at both root and `/v1` aliases.

## Development

Backend:

```bash
cd backend
go test ./...
```

Frontend:

```bash
cd frontend
npm ci
npm run test
npm run build
```

Contract + security checks:

```bash
./scripts/verify-contract.sh
./scripts/security-gate.sh
```

Optional live enrichment (cover art + metadata fallback chain):

- `IGDB_CLIENT_ID`
- `IGDB_CLIENT_SECRET`
- `RAWG_API_KEY`

## Image Publishing

GitHub Actions publishes:

- `ghcr.io/<owner>/retrosavemanager`
- `ghcr.io/<owner>/retrosavemanager-frontend`

on pushes to `main` and version tags.
