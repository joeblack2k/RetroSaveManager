# RetroSaveManager

RetroSaveManager is a self-hosted, LAN-first save synchronization service with compatibility-focused API behavior for existing 1Retro helper clients.

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

- `SAVE_ROOT/<system>/<game>/<save-id>/payload.*`
- `SAVE_ROOT/<system>/<game>/<save-id>/metadata.json`

Back up one root folder to capture all saves.

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

- `ONE_RETRO_API_URL=http://<internal-hostname-or-ip>`
- `ONE_RETRO_APP_PASSWORD=<optional-app-password>`

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

## Image Publishing

GitHub Actions publishes:

- `ghcr.io/<owner>/retrosavemanager`
- `ghcr.io/<owner>/retrosavemanager-frontend`

on pushes to `main` and version tags.
