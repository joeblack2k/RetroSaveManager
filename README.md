# RetroSaveManager

RetroSaveManager is a self-hosted save sync manager for retro gaming setups.

It is built for homelab users who want to run one Docker container, keep all save data on their own storage, and use compatible helper apps with MiSTer, RetroArch, and other supported systems.

## What You Get

- One runtime container
- One web UI
- One API for helper apps
- One save root you can back up
- Docker-first self-hosting

## Quick Start

This is the smallest useful Docker Compose setup.

```yaml
services:
  retrosavemanager:
    image: ghcr.io/joeblack2k/retrosavemanager:latest
    container_name: retrosavemanager
    restart: unless-stopped
    ports:
      - "80:80"
    environment:
      AUTH_MODE: disabled
      SAVE_ROOT: /saves
      STATE_ROOT: /config
      PORT: 80
    volumes:
      - /srv/retrosavemanager/config:/config
      - /srv/retrosavemanager/saves:/saves
```

Start it:

```bash
docker compose up -d
```

Open it:

```text
http://YOUR-DOCKER-HOST-IP/
```

## What Each Option Does

`image: ghcr.io/joeblack2k/retrosavemanager:latest`

Uses the published container image from GitHub Container Registry.

`container_name: retrosavemanager`

Gives the container a fixed and easy-to-recognize name.

`restart: unless-stopped`

Starts the container again after a reboot or Docker restart.

`ports:`

Publishes the web UI and API to your Docker host.

`- "80:80"`

Maps host port `80` to container port `80`.

`AUTH_MODE: disabled`

Disables login requirements. This is the default and is intended for trusted internal networks.

`SAVE_ROOT: /saves`

The container stores all managed save data under `/saves`.

`STATE_ROOT: /config`

The container stores app state, indexes, and internal metadata under `/config`.

`PORT: 80`

Tells the app to listen on port `80` inside the container.

`/srv/retrosavemanager/config:/config`

Persistent config and app-state storage on your Docker host.

`/srv/retrosavemanager/saves:/saves`

Persistent save storage on your Docker host. This is the most important volume for backups.

## Volume Mappings

These are the two important host folders:

- `/srv/retrosavemanager/config`
- `/srv/retrosavemanager/saves`

What they contain:

- `config`
  - internal app state
  - save index metadata
  - helper-related state
  - settings used by the service
- `saves`
  - the actual save files
  - per-system folders
  - per-game folders
  - metadata for each stored save version

If you only care about protecting save history, back up both folders.

If you want the shortest answer to “where are my saves?”, it is this:

```text
/srv/retrosavemanager/saves
```

## Save Storage Layout

RetroSaveManager keeps save data under one backup-friendly root:

```text
SAVE_ROOT/<System>/<Game>/<save-id>/
```

Example:

```text
/srv/retrosavemanager/saves/Nintendo Super Nintendo Entertainment System/Super Mario World/save-123456/
```

Inside each save folder you will typically see:

- `payload.*`
- `metadata.json`

## Ports Used

By default, RetroSaveManager uses:

- Container port `80`
- Host port `80` in the example above

That means:

- Web UI: `http://YOUR-HOST-IP/`
- API: `http://YOUR-HOST-IP/`

There is no required separate frontend port.

There is no required separate API port.

There is no built-in HTTPS requirement inside the container.

If you want to use a different host port, change only the left side:

```yaml
ports:
  - "8080:80"
```

Then the service will be available at:

```text
http://YOUR-HOST-IP:8080/
```

## Optional Environment Variables

You only need the minimal compose above to get started.

These extra variables are optional:

`BASE_URL`

Set this if you want a fixed public or internal base URL.

`PUBLIC_HOST`

Useful if you use an internal DNS name such as `retrosavemanager.lan`.

`IGDB_CLIENT_ID`

Optional metadata and cover-art enrichment.

`IGDB_CLIENT_SECRET`

Optional metadata and cover-art enrichment.

`RAWG_API_KEY`

Optional metadata and cover-art fallback enrichment.

`BOOTSTRAP_DEMO_DATA`

Set to `true` only if you want demo data in a fresh environment.

## Recommended Example With Named Host Paths

```yaml
services:
  retrosavemanager:
    image: ghcr.io/joeblack2k/retrosavemanager:latest
    container_name: retrosavemanager
    restart: unless-stopped
    ports:
      - "80:80"
    environment:
      AUTH_MODE: disabled
      PORT: 80
      SAVE_ROOT: /saves
      STATE_ROOT: /config
      PUBLIC_HOST: retrosavemanager.lan
    volumes:
      - /srv/retrosavemanager/config:/config
      - /srv/retrosavemanager/saves:/saves
```

## Updating

Pull the latest image and recreate the container:

```bash
docker compose pull
docker compose up -d
```

## Helper App Configuration

Point helper apps to your RetroSaveManager host:

```text
http://YOUR-HOST-IP/
```

If your helper supports an API URL variable, use:

```text
RSM_API_URL=http://YOUR-HOST-IP/
```

If you use an internal DNS name:

```text
RSM_API_URL=http://retrosavemanager.lan/
```

## Notes

- This project is designed for internal and self-hosted use.
- Default auth mode is disabled for trusted LAN setups.
- The standard deployment is exactly one container.
- The web UI and helper API are served by the same container.

## Releases

Container images are published here:

- [ghcr.io/joeblack2k/retrosavemanager](https://ghcr.io/joeblack2k/retrosavemanager)
