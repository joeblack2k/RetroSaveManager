# RetroSaveManager Deploy

Dit document beschrijft de canonieke GHCR-only deployflow voor RetroSaveManager.

## Canonieke releaseflow

RetroSaveManager heeft exact 1 runtime image:

- `ghcr.io/joeblack2k/retrosavemanager`

De standaard productiestroom is:

1. push naar `main`
2. GitHub Actions bouwt en publiceert de image naar GHCR
3. de deploy-host pulled exact die image
4. Docker Compose start exact 1 runtime container: `app`

De deploy-host hoort dus niet meer lokaal te bouwen voor productie-updates.

## Live omgeving

- Docker host: `<deploy-host>`
- User: `<deploy-user>`
- Password: keep this outside the public repo
- Repo pad: `/home/debian/RetroSaveManager`
- Live URL: `http://<deploy-host>`

## Vereiste GitHub workflow

Publicatie naar GHCR loopt via:

- [`publish-ghcr.yml`](.github/workflows/publish-ghcr.yml)

Belangrijk:

- `main` publiceert `latest`
- `main` publiceert ook een immutable `sha-<full-commit-sha>` tag
- de workflow verifieert dat GHCR `latest` echt de revision van `main` bevat
- echte omgevingsgegevens horen niet in de repo; gebruik lokale ongetrackte notities zoals `LOCAL_SECRETS.md`

## Standaard deploy op je Docker-host

1. Check dat de publish-workflow op GitHub groen is.
2. SSH naar de deploy-host.
3. Fast-forward de repo.
4. Pull de GHCR image.
5. Restart de enkele `app` service.
6. Run smoke checks.

Exacte commando’s:

```bash
sshpass -p '<deploy-password>' ssh -o StrictHostKeyChecking=no <deploy-user>@<deploy-host> '
  set -euo pipefail
  cd /home/debian/RetroSaveManager
  git fetch origin main
  git pull --ff-only origin main
  cd deploy
  ./scripts/pull-up.sh direct
'
```

## Smoke checks

```bash
curl -fsS http://<deploy-host>/healthz
curl -fsS http://<deploy-host>/auth/me
curl -fsS 'http://<deploy-host>/saves?limit=5&offset=0'
curl -fsS http://<deploy-host>/app/my-games > /dev/null
```

Container check:

```bash
sshpass -p '<deploy-password>' ssh -o StrictHostKeyChecking=no <deploy-user>@<deploy-host> '
  cd /home/debian/RetroSaveManager/deploy
  docker compose ps
'
```

Verwacht:

- exact 1 RetroSaveManager runtime container
- service name `app`
- container name meestal `deploy-app-1`

## Deploy modes

Beschikbare runtime modes:

- `direct`: bindt op host `:80`
- `macvlan`: zelfde single `app` service, maar met eigen LAN-IP via compose override

Voor macvlan:

```bash
cd /home/debian/RetroSaveManager/deploy
./scripts/pull-up.sh macvlan
```

## Wat de scripts nu doen

- `deploy/scripts/pull-up.sh`
  - canonieke productie-update
  - doet `docker compose pull`
  - doet daarna `docker compose up -d --remove-orphans`
- `deploy/scripts/up.sh`
  - herstart met de huidige lokaal aanwezige image
  - doet geen `pull`
  - doet geen `build`
  - nuttig voor snelle lokale restarts, niet de standaard productieflow
- `deploy/scripts/build-local.sh`
  - expliciete lokale buildflow
  - gebruikt `deploy/docker-compose.build.yml`
  - alleen gebruiken als je bewust een niet-GHCR lokale image wilt draaien

## Rollback

Standaard draait de host op `IMAGE_TAG=latest`.

Voor een rollback:

1. kies een eerdere immutable GHCR tag in de vorm `sha-<full-commit-sha>`
2. zet die in `deploy/.env`
3. run opnieuw `./scripts/pull-up.sh direct`

Voorbeeld:

```bash
cd /home/debian/RetroSaveManager/deploy
printf 'IMAGE=ghcr.io/joeblack2k/retrosavemanager\nIMAGE_TAG=sha-31002518feb4d9360f9a207f240e279d7a396b85\n' > .env.rollback
cp .env.rollback .env
./scripts/pull-up.sh direct
```

Na rollback kun je terug naar `latest` door `deploy/.env` weer terug te zetten.

## Config en persistence

Belangrijkste env vars:

- `IMAGE`
- `IMAGE_TAG`
- `AUTH_MODE`
- `HTTP_PORT`
- `PUBLIC_HOST`
- `CONFIG_HOST_PATH`
- `SAVES_HOST_PATH`

Standaard image:

- `IMAGE=ghcr.io/joeblack2k/retrosavemanager`

Data blijft persistent in:

- host config map -> container `/config`
- host saves map -> container `/saves`

## Niet meer doen

Deze dingen zijn niet meer de standaard releaseflow:

- productie-updates via `docker compose up --build`
- lokale builds op de deploy-host als vervanging van GHCR publish
- aparte frontend/gateway containers voor de standaard runtime

## Relevante bestanden

- [`deploy/docker-compose.yml`](deploy/docker-compose.yml)
- [`deploy/docker-compose.build.yml`](deploy/docker-compose.build.yml)
- [`deploy/scripts/pull-up.sh`](deploy/scripts/pull-up.sh)
- [`deploy/scripts/up.sh`](deploy/scripts/up.sh)
- [`deploy/scripts/build-local.sh`](deploy/scripts/build-local.sh)
- [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md)
- [`onboarding.md`](onboarding.md)
