# RetroSaveManager Agent API

This document describes the no-auth automation API for AI agents, shell scripts, and local tools.

Current phase defaults:

- Authentication: disabled
- Primary API bases: `/api` and `/api/v1`
- Existing helper compatibility APIs remain available at `/` and `/v1`
- Response format: JSON unless a download endpoint returns binary data

Use a sanitized base URL in scripts and examples, for example:

```bash
export RSM_BASE_URL="https://rsm.example.invalid"
export RSM_API="$RSM_BASE_URL/api"
```

## Design Goals

- Easy to consume from `curl`, shell scripts, and AI agents
- Rich read access for saves, ROMs, devices, sync state, conflicts, and helper enrollment
- No frontend scraping required
- No auth required during the current internal test phase

## Base Paths

Both of these are equivalent:

- `/api`
- `/api/v1`

Examples:

```bash
curl -s "$RSM_BASE_URL/api"
curl -s "$RSM_BASE_URL/api/v1/overview"
```

## Endpoint Index

### Discovery and status

- `GET /api`
- `GET /api/overview`
- `GET /api/sync/status`
- `GET /api/systems`
- `GET /api/logs`
- `GET /api/events`

### Devices

- `GET /api/devices`
- `GET /api/devices/{id}`
- `PATCH /api/devices/{id}`
- `POST /api/devices/{id}/command`
- `POST /api/devices/config/report`
- `DELETE /api/devices/{id}`

### Saves

- `GET /api/save/latest`
- `GET /api/saves`
- `POST /api/saves`
- `DELETE /api/saves`
- `POST /api/saves/rescan`
- `GET /api/saves/download-many`
- `GET /api/saves/{id}`
- `DELETE /api/saves/{id}`
- `POST /api/saves/{id}/rollback`
- `GET /api/saves/{id}/download`

### ROM data

- `GET /api/roms`
- `GET /api/roms/{hash}`
- `GET /api/roms/lookup`
- `POST /api/roms/lookup`
- `POST /api/roms/lookup/batch`

### Conflicts

- `GET /api/conflicts`
- `GET /api/conflicts/count`
- `GET /api/conflicts/check`
- `POST /api/conflicts/check`
- `POST /api/conflicts/report`
- `GET /api/conflicts/{id}`
- `POST /api/conflicts/{id}/resolve`

### Helper config and enrollment

- `POST /api/helpers/config/sync`
- `POST /api/helpers/heartbeat`
- `GET /api/helpers/auto-enroll`
- `POST /api/helpers/auto-enroll`

## Discovery

`GET /api` returns the active base path, alias, and the most important endpoint URLs.

```bash
curl -s "$RSM_API" | jq
```

Typical response:

```json
{
  "success": true,
  "api": {
    "name": "RetroSaveManager Agent API",
    "version": "v1",
    "authMode": "disabled",
    "basePath": "/api",
    "basePathAlias": "/api/v1",
    "docsFile": "api.md",
    "compatBases": ["/", "/v1"]
  }
}
```

## Overview

`GET /api/overview` gives one machine-friendly snapshot for dashboards and automation.

It includes:

- auth mode
- auto-enroll window status
- counts for games, save files, save tracks, save versions, storage, devices, systems, ROMs, conflicts
- latest save/device timestamps
- online vs stale device counts
- per-system summary blocks

```bash
curl -s "$RSM_API/overview" | jq
```

## Sync Status

`GET /api/sync/status` is the best single endpoint for “what is happening right now?”

It includes:

- device online or stale status
- last seen and last synced ages in seconds
- helper binding info
- recent saves with machine-usable action URLs
- conflict count
- auto-enroll status

Optional query params:

- `recentLimit` default `10`
- `staleAfterMinutes` default `30`

```bash
curl -s "$RSM_API/sync/status?recentLimit=25&staleAfterMinutes=45" | jq
```

## Systems

`GET /api/systems` returns per-system save and device aggregates.

Each item includes:

- normalized system object
- save track count
- save version count
- game count
- total size in bytes
- allowed device count
- reported device count
- latest created-at timestamp

```bash
curl -s "$RSM_API/systems" | jq
```

## Logs

`GET /api/logs` returns paginated sync activity for automation, dashboards, and admin tooling.

Defaults:

- `hours=72`
- `page=1`
- `limit=50`

Each log row includes:

- `createdAt`
- `deviceName`
- `action`
- `game`
- `error`
- `errorMessage`
- `systemSlug`
- `saveId`
- `conflictId`

```bash
curl -s "$RSM_API/logs?hours=72&page=1&limit=50" | jq
```


## Helper Config Sync

Helpers should report parsed `config.ini` snapshots to the backend before each sync/watch cycle. This lets the backend return the effective source policy that helpers apply in memory for the current run.

```bash
curl -s -X POST "$RSM_API/helpers/config/sync" \
  -H 'Content-Type: application/json' \
  -H 'X-RSM-App-Password: ABC-123' \
  -d @helper-config-sync.json | jq
```

The response includes `accepted: true`, `policy.sources[]`, and when available `policy.global`. Global policy can include `url`, `port`, `baseUrl`, `email`, `root`, `stateDir`, `watch`, `watchInterval`, `forceUpload`, `dryRun`, and `routePrefix`. Each source policy contains the backend-approved `systems` list plus source runtime fields such as `kind`, `profile`, `savePaths`, `romPaths`, `recursive`, and `createMissingSystemDirs`.

### Helper heartbeat

Always-on helpers should report service health and sensors:

```bash
curl -s -X POST "$RSM_API/helpers/heartbeat" \
  -H 'Content-Type: application/json' \
  -H 'X-RSM-App-Password: ABC-123' \
  -d @helper-heartbeat.json | jq
```

The heartbeat payload contains `helper`, `service`, `sensors`, `config`, and `capabilities`. The backend stores daemon status, freshness, config hash, folder counts, last sync counters, and the reported structured config snapshot.

The simpler `POST /api/devices/config/report` route remains available for scripts and compatibility, but helpers should prefer `/api/helpers/config/sync`.

## Devices

### List devices

```bash
curl -s "$RSM_API/devices" | jq
```

Device records expose helper-facing metadata when available, including:

- `deviceType`
- `fingerprint`
- `displayName`
- `hostname`
- `helperName`
- `helperVersion`
- `platform`
- `syncPaths`
- `reportedSystemSlugs`
- `configGlobal`
- `service`
- `sensors`
- `lastSeenIp`
- `lastSeenUserAgent`
- `lastSeenAt`
- `lastSyncedAt`
- `allowedSystemSlugs`
- `configSources`
- `configCapabilities`
- `effectivePolicy`
- bound app-password metadata

### Get one device

```bash
curl -s "$RSM_API/devices/1" | jq
```

### Update one device

Supported patch fields:

- `alias`
- `syncAll`
- `allowedSystemSlugs`
- `configGlobal`
- `configSources`

```bash
curl -s -X PATCH "$RSM_API/devices/1" \
  -H 'Content-Type: application/json' \
  -d '{
    "alias": "Living Room MiSTer",
    "syncAll": false,
    "allowedSystemSlugs": ["snes", "n64", "psx"],
    "configSources": [
      {
        "id": "backend-snes-snes9x",
        "label": "Super Nintendo Snes9x",
        "kind": "custom",
        "profile": "snes9x",
        "savePaths": ["/media/snes9x/saves"],
        "romPaths": ["/media/snes9x/roms"],
        "recursive": true,
        "systems": ["snes"],
        "createMissingSystemDirs": false,
        "managed": true,
        "origin": "backend"
      }
    ]
  }' | jq
```

When `configSources` changes, the backend publishes `config.changed` on `GET /events`. Service helpers should reload/apply the new policy and write it back to `config.ini`.

### Send a helper command

```bash
curl -s -X POST "$RSM_API/devices/1/command" \
  -H 'Content-Type: application/json' \
  -d '{"command":"sync","reason":"user_requested"}' | jq
```

Supported commands:

- `sync` publishes `sync.requested`
- `scan` publishes `scan.requested`
- `deep_scan` publishes `deep_scan.requested`

Helpers receive these events over `GET /events`.


### Report helper config

Helpers can report their parsed local config so the backend can calculate the effective per-device sync policy. Send structured JSON; do not send raw INI text.

```bash
curl -s -X POST "$RSM_API/devices/config/report" \
  -H 'Content-Type: application/json' \
  -H 'X-RSM-Device-Type: mister' \
  -H 'X-RSM-Fingerprint: example-device-id' \
  -H 'X-RSM-App-Password: ABC-123' \
  -d '{
    "configRevision": "sha256:example",
    "sources": [
      {
        "id": "mister_default",
        "label": "MiSTer Default",
        "kind": "mister-fpga",
        "profile": "mister",
        "savePath": "/media/fat/saves",
        "romPath": "/media/fat/games",
        "recursive": true,
        "systems": ["nes", "snes", "n64", "psx"],
        "managed": false,
        "origin": "manual"
      }
    ]
  }' | jq
```

The response includes `effectivePolicy.allowedSystemSlugs` and `effectivePolicy.blocked`. Backend policy always wins over helper config.

### Delete one device

```bash
curl -s -X DELETE "$RSM_API/devices/1" | jq
```

## Saves

### List saves

`GET /api/saves` returns aggregated save tracks with action URLs.

Supported query params:

- `limit`
- `offset`
- `gameId`
- `systemSlug`
- `romSha1`
- `romMd5`
- `q`

```bash
curl -s "$RSM_API/saves?systemSlug=snes&limit=50&offset=0&q=zelda" | jq
```

Each list item looks like:

```json
{
  "save": {
    "id": "save-...",
    "displayTitle": "The Legend of Zelda - A Link to the Past",
    "systemSlug": "snes",
    "regionCode": "US",
    "saveCount": 3,
    "latestSizeBytes": 8192,
    "totalSizeBytes": 24576,
    "latestVersion": 3
  },
  "actions": {
    "detail": "/api/saves/save-123",
    "download": "/api/saves/save-123/download",
    "delete": "/api/saves/save-123",
    "rollback": "/api/saves/save-123/rollback"
  }
}
```

### Save history and details

`GET /api/saves/{id}` returns the full version history for that save track.

Important notes:

- Non-PlayStation tracks return normal version history
- PlayStation logical saves can include `psLogicalKey` in the query string
- History remains the canonical place for version inspection

```bash
curl -s "$RSM_API/saves/save-123" | jq
curl -s "$RSM_API/saves/save-123?psLogicalKey=psx%3ASLUS-00000%3AExample" | jq
```

### Save latest

`GET /api/save/latest` answers the helper-style “does the latest save already exist?” question.

Useful query params:

- `romSha1`
- `slotName`
- `saturnFormat`
- `saturnEntry`
- helper identity fields when you want helper semantics

```bash
curl -s "$RSM_API/save/latest?romSha1=abc123&slotName=default" | jq
```

### Upload a save

`POST /api/saves` accepts the same multipart upload flow as the helper API.

Useful multipart fields:

- `file`
- `system`
- `rom_sha1`
- `rom_md5`
- `slotName`
- helper metadata fields such as `device_type`, `fingerprint`, `hostname`, `helper_name`, `helper_version`, `platform`, `sync_paths`, `systems`

```bash
curl -s -X POST "$RSM_API/saves" \
  -F "file=@./Super Mario World (USA).srm" \
  -F "system=snes" \
  -F "rom_sha1=0123456789abcdef0123456789abcdef01234567" \
  -F "slotName=default" | jq
```

### Delete many saves

```bash
curl -s -X DELETE "$RSM_API/saves?ids=save-1,save-2,save-3" | jq
```

### Rescan save storage

`POST /api/saves/rescan` reimports metadata from disk and applies current detection rules.

JSON body fields:

- `dryRun` boolean
- `pruneUnsupported` boolean

```bash
curl -s -X POST "$RSM_API/saves/rescan" \
  -H 'Content-Type: application/json' \
  -d '{"dryRun":false,"pruneUnsupported":true}' | jq
```

### Download one or many saves

```bash
curl -L "$RSM_API/saves/save-123/download" -o save.bin
curl -L "$RSM_API/saves/download-many?ids=save-1,save-2" -o saves.zip
```

PlayStation logical downloads can include:

- `psLogicalKey`
- `revisionId`

Saturn downloads can include:

- `saturnFormat`
- `saturnEntry`

### Roll back a save

`POST /api/saves/{id}/rollback`

JSON body:

- `revisionId`
- `psLogicalKey` when rolling back a PlayStation logical save

```bash
curl -s -X POST "$RSM_API/saves/save-123/rollback" \
  -H 'Content-Type: application/json' \
  -d '{"revisionId":"save-older"}' | jq
```

### Delete one save

```bash
curl -s -X DELETE "$RSM_API/saves/save-123" | jq
curl -s -X DELETE "$RSM_API/saves/save-123?psLogicalKey=psx%3ASLUS-00000%3AExample" | jq
```

## ROM Data

### List ROMs

`GET /api/roms` derives ROM-oriented information from stored save records.

Supported query params:

- `includeMissing` boolean
- `systemSlug`
- `q`
- `limit`
- `offset`

Each ROM item includes:

- ROM hashes
- system slug and system name
- game ID and titles
- region code
- save count
- latest save ID
- latest version
- latest created-at
- latest and total size in bytes
- slot names
- all related save IDs

```bash
curl -s "$RSM_API/roms?systemSlug=n64&includeMissing=true" | jq
```

### Get one ROM entry

You can query by:

- internal ROM key
- SHA1
- MD5

```bash
curl -s "$RSM_API/roms/0123456789abcdef0123456789abcdef01234567" | jq
```

### ROM lookup

This is a lightweight helper-style lookup endpoint.

```bash
curl -s "$RSM_API/roms/lookup?filenameStem=Wario%20Land%20II" | jq

curl -s -X POST "$RSM_API/roms/lookup" \
  -H 'Content-Type: application/json' \
  -d '{"filenameStem":"Wario Land II"}' | jq
```

### Batch ROM lookup

```bash
curl -s -X POST "$RSM_API/roms/lookup/batch" \
  -H 'Content-Type: application/json' \
  -d '{
    "items": [
      {"id":"1","value":{"name":"Super Mario World"}},
      {"id":"2","value":{"name":"Star Fox 64"}}
    ]
  }' | jq
```

## Conflicts

Use the conflicts API to inspect or resolve sync conflicts without scraping the web UI.

```bash
curl -s "$RSM_API/conflicts" | jq
curl -s "$RSM_API/conflicts/count" | jq
curl -s "$RSM_API/conflicts/check" | jq
curl -s -X POST "$RSM_API/conflicts/report" \
  -H 'Content-Type: application/json' \
  -d '{"conflictKey":"example","localSha256":"a","cloudSha256":"b"}' | jq
curl -s "$RSM_API/conflicts/conflict-123" | jq
curl -s -X POST "$RSM_API/conflicts/conflict-123/resolve" \
  -H 'Content-Type: application/json' \
  -d '{"winner":"cloud"}' | jq
```

## Helper Auto-Enroll

This API exposes the helper auto-enrollment window used when a helper should be allowed to fetch a temporary app password automatically.

### Read status

```bash
curl -s "$RSM_API/helpers/auto-enroll" | jq
```

### Enable the window

```bash
curl -s -X POST "$RSM_API/helpers/auto-enroll" \
  -H 'Content-Type: application/json' \
  -d '{"minutes":15}' | jq
```

When the window is active, helper flows can request temporary credentials through the existing helper token route.

## Events

`GET /api/events` exposes the same event stream endpoint used by the current app/backend stack.

```bash
curl -N "$RSM_API/events"
```

## Helper Semantics vs Agent Semantics

Most `/api` endpoints work fine as plain no-auth HTTP calls.

Some routes can switch into helper-aware behavior when you provide helper identity or app-password context:

- `device_type`
- `fingerprint`
- `hostname`
- `helper_name`
- `helper_version`
- `platform`
- `sync_paths`
- `systems`
- `Authorization: Bearer <app-password>`
- `X-RSM-App-Password: <app-password>`

This matters especially for:

- `/api/save/latest`
- `/api/saves`
- `/api/saves/{id}/download`
- `/api/saves/download-many`

Without helper fields, these endpoints behave as a plain internal management API.

## Error Handling

The API uses JSON error envelopes for API routes:

```json
{
  "error": "Bad Request",
  "message": "id is required",
  "statusCode": 400
}
```

Reserved `/api/...` paths never fall back to the SPA HTML shell. Unknown API paths return JSON `404`.

## Recommended Automation Patterns

### Save dashboard snapshot

```bash
curl -s "$RSM_API/overview" | jq '{stats: .overview.stats, latest: .overview.latest}'
```

### List stale devices

```bash
curl -s "$RSM_API/sync/status?staleAfterMinutes=30" \
  | jq '.devices[] | select(.status == "stale") | .device.displayName'
```

### Export all N64 save download URLs

```bash
curl -s "$RSM_API/saves?systemSlug=n64&limit=500&offset=0" \
  | jq -r '.saves[] | .actions.download'
```

### Find all saves tied to one ROM hash

```bash
curl -s "$RSM_API/roms/0123456789abcdef0123456789abcdef01234567" | jq
```

## Stability Notes

- `/api` is intended for automation and internal tooling
- `/api/v1` is the alias if you prefer an explicit versioned base
- `/` and `/v1` remain the legacy/helper compatibility surface
- During this phase there is intentionally no auth gate on `/api`
- If auth is added later, the path layout can remain stable while only access control changes
