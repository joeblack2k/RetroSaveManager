# RetroSaveManager Onboarding

This file is the sanitized handover for new sessions. It is safe to keep in the public repo.

## Project Goal

RetroSaveManager is a self-hosted save-sync service for MiSTer, RetroArch, and related helpers with:

- helper compatibility at both root and `/v1` API routes
- no-auth internal default via `AUTH_MODE=disabled`
- single-container Docker deployment for internal LAN use

Key product decisions:

- no admin, billing, manager, or forge scope
- no `1Retro` branding or upstream analytics remnants
- one backup-friendly save root
- modular frontend and backend structure

## Repository Layout

- workspace root: `<workspace-root>/RetroSaveManager`
- GitHub repo: `https://github.com/joeblack2k/RetroSaveManager`
- container image: `ghcr.io/joeblack2k/retrosavemanager`
- reverse-engineering notes: keep these outside the public repo when needed

Important paths:

- `backend/`
- `frontend/`
- `deploy/`
- `scripts/`
- `docs/`
- `contracts/`

## Environment Policy

Do not store real infrastructure data in this repo.

- deploy host: `<deploy-host>`
- deploy user: `<deploy-user>`
- test/build host: `<test-host>`
- test/build user: `<test-user>`
- passwords and secrets: store in local untracked notes or a secret manager

Recommended local untracked file:

- `LOCAL_SECRETS.md`

## Current Architecture

Runtime is exactly one container:

- service name: `app`
- image: `ghcr.io/joeblack2k/retrosavemanager`
- default ingress: HTTP on port `80`
- the Go server serves both API routes and frontend assets

Supported deploy modes:

- `direct`
- `macvlan`

## Save Domain Notes

- save layout root: `SAVE_ROOT/<System Display>/<Game Title or Memory Card>/<save-id>/`
- helper-critical routes remain available at both root and `/v1`
- PlayStation support is moving toward logical-save handling instead of raw card blobs as the source of truth
- save detection should be parser-driven or helper-evidence-driven, not title-guessing

## Common Commands

Local frontend validation:

```bash
cd <workspace-root>/RetroSaveManager/frontend
npm test
npm run build
```

Backend validation:

```bash
cd <workspace-root>/RetroSaveManager/backend
go test ./...
```

Deploy via GHCR pull:

```bash
sshpass -p '<deploy-password>' ssh -o StrictHostKeyChecking=no <deploy-user>@<deploy-host> '
  set -euo pipefail
  cd /home/<deploy-user>/RetroSaveManager
  git fetch origin main
  git pull --ff-only origin main
  cd deploy
  ./scripts/pull-up.sh direct
'
```

Smoke checks:

```bash
curl -fsS http://<deploy-host>/healthz
curl -fsS http://<deploy-host>/auth/me
curl -fsS 'http://<deploy-host>/saves?limit=5&offset=0'
curl -fsS http://<deploy-host>/app/my-games > /dev/null
```

Backup retention cleanup:

```bash
cd <workspace-root>/RetroSaveManager/deploy
./scripts/prune-backups.sh --root /srv/retrosavemanager/backups --keep-recent 4 --keep-days 7 --dry-run
./scripts/prune-backups.sh --root /srv/retrosavemanager/backups --keep-recent 4 --keep-days 7
```

Optional host timer install:

```bash
sudo install -D -m 0644 deploy/systemd/retrosavemanager-backup-retention.service /etc/systemd/system/retrosavemanager-backup-retention.service
sudo install -D -m 0644 deploy/systemd/retrosavemanager-backup-retention.timer /etc/systemd/system/retrosavemanager-backup-retention.timer
sudo systemctl daemon-reload
sudo systemctl enable --now retrosavemanager-backup-retention.timer
sudo systemctl start retrosavemanager-backup-retention.service
```

## Session Checklist

1. Read `AGENTS.md`, this file, and `README.md`.
2. Check `git status -sb`.
3. Keep docs and examples sanitized.
4. Run relevant tests.
5. Run `./scripts/security-gate.sh` before committing doc, workflow, or deploy changes.
6. Prefer GHCR pull-based deploys for production-like environments.

## Known Constraints

- helper/API compatibility should remain backward compatible
- public docs must use placeholders or documentation-only example values
- keep real environment-specific data outside tracked files
