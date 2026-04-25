# RetroSaveManager Helper Backend Contract

This document is the backend-side contract for SGM helpers that report local `config.ini`, run as an always-on service, and receive backend-managed sync policy.

Aligned helper version: `v0.4.14`.

## Goals

- The backend Devices page is the source of truth for what a helper may sync.
- Helpers report structured config and service health; they do not send raw `config.ini` text.
- Backend policy wins at runtime and is written back by helpers to `config.ini` with a timestamped local backup.
- The backend can add new console/runtime/profile sources even when no save exists yet.
- MiSTer and other constrained helpers must not sync unsupported consoles such as Wii to a MiSTer source.

## Helper Endpoints

Helpers should use these root endpoints. `/v1`, `/api`, and `/api/v1` aliases are also available.

| Endpoint | Purpose |
| --- | --- |
| `POST /helpers/config/sync` | Report parsed `config.ini`; receive backend-approved policy. |
| `POST /helpers/heartbeat` | Report daemon/service status, sensors, config snapshot, and capabilities. |
| `GET /events` | Server-sent events channel for backend push control. |
| `POST /saves` | Upload saves after applying backend policy. |
| `GET /save/latest` | Check current cloud truth before upload/download decisions. |
| `GET /saves/download` | Download backend-projected save payloads. |

Compatibility/admin routes remain available for scripts:

- `POST /devices/config/report`
- `GET /devices`
- `GET /devices/{id}`
- `PATCH /devices/{id}`
- `POST /devices/{id}/command`

## Authentication

Helpers should send the app password header when configured:

```http
X-RSM-App-Password: ABC-123
```

Use example credentials only in docs and tests. Do not commit local LAN addresses, real app passwords, or device secrets.

The helper reports only whether an app password is configured:

```json
"appPasswordConfigured": true
```

The backend derives device identity from:

- `helper.deviceType`
- `helper.fingerprint` when present
- otherwise `helper.hostname` or `helper.configPath`

## Service Mode

Helper `v0.4.14` can run continuously:

```bash
sgm-helper service run --heartbeat-interval 30 --reconcile-interval 1800
```

Default behavior:

- heartbeat every `30` seconds
- reconcile sync every `1800` seconds
- SSE control channel at `GET /events`
- startup sync
- normal sync lock usage through `STATE_DIR`

Backend freshness rules:

| State | Rule |
| --- | --- |
| `online` | Last heartbeat age is within `max(heartbeatInterval * 3, 90s)`. |
| `degraded` | Online but helper reports `status=backoff`, `lastSyncOk=false`, or `lastError`. |
| `stale` | Last heartbeat older than stale threshold but within offline threshold. |
| `offline` | Last heartbeat older than `max(heartbeatInterval * 6, 180s)` or helper reports stopping. |

## Heartbeat Request

`POST /helpers/heartbeat`

```json
{
  "schemaVersion": 1,
  "helper": {
    "name": "sgm-mister-helper",
    "version": "0.4.14",
    "deviceType": "mister",
    "defaultKind": "mister-fpga",
    "hostname": "mister.example.invalid",
    "platform": "linux",
    "arch": "arm",
    "pid": 1234,
    "startedAt": "2026-04-25T12:00:00Z",
    "uptimeSeconds": 61,
    "binaryPath": "/media/fat/1retro/sgm-mister-helper",
    "binaryDir": "/media/fat/1retro",
    "configPath": "/media/fat/1retro/config.ini",
    "stateDir": "/media/fat/1retro/state"
  },
  "service": {
    "mode": "daemon",
    "status": "idle",
    "loop": "sse-plus-periodic-reconcile",
    "heartbeatInterval": 30,
    "reconcileInterval": 1800,
    "controlChannel": "GET /events",
    "lastSyncStartedAt": "2026-04-25T12:00:01Z",
    "lastSyncFinishedAt": "2026-04-25T12:00:04Z",
    "lastSyncOk": true,
    "lastError": null,
    "lastEvent": "startup",
    "syncCycles": 1
  },
  "sensors": {
    "online": true,
    "authenticated": true,
    "configHash": "sha256-of-redacted-structured-config",
    "configReadable": true,
    "configError": null,
    "sourceCount": 1,
    "savePathCount": 1,
    "romPathCount": 1,
    "configuredSystems": ["n64", "psx", "snes"],
    "supportedSystems": ["nes", "snes", "gameboy", "gba", "n64", "genesis", "master-system", "game-gear", "sega-cd", "sega-32x", "saturn", "neogeo", "psx"],
    "syncLockPresent": false,
    "lastSync": {
      "scanned": 24,
      "uploaded": 1,
      "downloaded": 0,
      "inSync": 23,
      "conflicts": 0,
      "skipped": 0,
      "errors": 0
    }
  },
  "config": {
    "url": "rsm.example.invalid",
    "port": 80,
    "baseUrl": "https://rsm.example.invalid",
    "email": "",
    "appPasswordConfigured": true,
    "root": "/media/fat",
    "stateDir": "./state",
    "watch": false,
    "watchInterval": 30,
    "forceUpload": false,
    "dryRun": false,
    "routePrefix": "",
    "sources": [
      {
        "id": "mister_default",
        "label": "MiSTer Default",
        "kind": "mister-fpga",
        "profile": "mister",
        "savePaths": ["/media/fat/saves"],
        "romPaths": ["/media/fat/games"],
        "recursive": true,
        "systems": ["nes", "snes", "gameboy", "gba", "n64", "genesis", "master-system", "game-gear", "sega-cd", "sega-32x", "saturn", "neogeo", "psx"],
        "createMissingSystemDirs": false,
        "managed": false,
        "origin": "manual"
      }
    ]
  },
  "capabilities": {
    "service": {
      "supportsDaemonMode": true,
      "heartbeatEndpoint": "POST /helpers/heartbeat",
      "controlChannel": "GET /events",
      "controlEvents": ["sync.requested", "scan.requested", "deep_scan.requested", "config.changed"]
    }
  }
}
```

Recommended response:

```json
{
  "success": true,
  "accepted": true,
  "serverTime": "2026-04-25T12:00:05Z",
  "helperId": "1"
}
```

The backend may include the public device object in the response. Helpers do not need it yet.

## Config Sync Request

`POST /helpers/config/sync`

The request uses the same `helper`, `config`, and `capabilities` objects shown above. The helper should call it during startup, periodic reconcile cycles, backend-triggered sync, and normal CLI sync/watch cycles.

A helper config section like this:

```ini
[source.mister_default]
LABEL="MiSTer Default"
KIND="mister-fpga"
PROFILE="mister"
SAVE_PATH="/media/fat/saves"
ROM_PATH="/media/fat/games"
RECURSIVE="true"
SYSTEMS="nes,snes,gameboy,gba,n64,genesis,master-system,game-gear,sega-cd,sega-32x,saturn,neogeo,psx"
CREATE_MISSING_SYSTEM_DIRS="false"
MANAGED="false"
ORIGIN="manual"
```

Should be reported as:

```json
{
  "id": "mister_default",
  "label": "MiSTer Default",
  "kind": "mister-fpga",
  "profile": "mister",
  "savePaths": ["/media/fat/saves"],
  "romPaths": ["/media/fat/games"],
  "recursive": true,
  "systems": ["nes", "snes", "gameboy", "gba", "n64", "genesis", "master-system", "game-gear", "sega-cd", "sega-32x", "saturn", "neogeo", "psx"],
  "createMissingSystemDirs": false,
  "managed": false,
  "origin": "manual"
}
```

The backend accepts legacy singular `savePath` and `romPath`, but helpers should send `savePaths` and `romPaths`.

## Backend Policy Response

The backend returns the stored device and runtime policy:

```json
{
  "success": true,
  "accepted": true,
  "effectivePolicy": {
    "mode": "source-scoped-all",
    "allowedSystemSlugs": ["n64", "nes", "psx", "sega-32x", "sega-cd", "snes"],
    "blocked": [
      {
        "system": "wii",
        "reason": "not supported by this helper kind/profile",
        "sourceId": "mister_default",
        "sourceLabel": "MiSTer Default"
      }
    ]
  },
  "policy": {
    "global": {
      "url": "rsm.example.invalid",
      "port": 80,
      "baseUrl": "https://rsm.example.invalid",
      "root": "/media/fat",
      "stateDir": "./state",
      "watchInterval": 30,
      "forceUpload": false,
      "dryRun": false
    },
    "sources": [
      {
        "id": "mister_default",
        "sourceId": "mister_default",
        "name": "MiSTer Default",
        "label": "MiSTer Default",
        "enabled": true,
        "kind": "mister-fpga",
        "profile": "mister",
        "savePaths": ["/media/fat/saves"],
        "romPaths": ["/media/fat/games"],
        "recursive": true,
        "systems": ["n64", "nes", "psx", "sega-32x", "sega-cd", "snes"],
        "createMissingSystemDirs": false
      }
    ]
  }
}
```

Helpers should apply `policy.global` and `policy.sources` immediately and write them back to `config.ini` with a timestamped backup.

## Backend-Managed Sources

The Devices UI can add a source before a helper has seen any saves for that console. Example user action:

1. Open Devices.
2. Open Manage for a helper.
3. Add console `Super Nintendo`.
4. Select profile `snes9x` or `retroarch`.
5. Enter a save folder such as `/media/snes9x/saves`.
6. Optionally enter a ROM folder such as `/media/snes9x/roms`.
7. Save.

The backend stores this as a backend-managed source:

```json
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
```

When the helper later calls `/helpers/config/sync`, backend-managed sources are merged into `policy.sources[]` and survive helper-reported config snapshots. This lets the backend create profiles for systems that were not auto-discovered yet.

## Source Capability Rules

The backend validates source capabilities before returning runtime policy.

MiSTer `mister-fpga/mister` allows only:

- `game-gear`
- `gameboy`
- `gba`
- `genesis`
- `master-system`
- `n64`
- `neogeo`
- `nes`
- `psx`
- `saturn`
- `sega-32x`
- `sega-cd`
- `snes`

Examples of systems that must be blocked for MiSTer sources:

- `wii`
- `ps2`
- `psp`
- `ps3`
- `ps4`
- `ps5`

Aliases such as `gbc` normalize to `gameboy` where the backend supports that canonical slug.

## Push Control Events

The backend publishes control events on `GET /events`.

| Backend command | SSE event | Helper behavior |
| --- | --- | --- |
| `sync` | `sync.requested` | Run normal reconcile/sync now. |
| `scan` | `scan.requested` | Scan configured sources. |
| `deep_scan` | `deep_scan.requested` | Run deeper scan where supported. |
| config edit | `config.changed` | Reload/apply backend policy. |

Manual command endpoint:

```http
POST /devices/1/command
Content-Type: application/json

{"command":"sync","reason":"user_requested"}
```

Supported commands are `sync`, `scan`, and `deep_scan`.

## Device Object Fields

The backend Devices API exposes:

- identity: `deviceType`, `fingerprint`, `displayName`, `hostname`, `platform`
- helper metadata: `helperName`, `helperVersion`
- service state: `service.mode`, `service.status`, `service.freshness`, `service.lastHeartbeatAt`, `service.lastError`, `service.syncCycles`
- sensors: `sensors.configHash`, `sensors.sourceCount`, `sensors.savePathCount`, `sensors.romPathCount`, `sensors.configuredSystems`, `sensors.lastSync`
- structured config: `configGlobal`, `configSources`, `configCapabilities`
- effective policy: `effectivePolicy.allowedSystemSlugs`, `effectivePolicy.blocked`, `effectivePolicy.sources`

## Helper Rules

- Helpers upload raw local saves; backend parsers/projections remain the source of format truth.
- Helpers must call latest checks before overwriting local saves.
- Helpers must apply backend `policy.sources[]` before scanning/uploading/downloading.
- Helpers should not sync a local source for a console omitted from backend policy.
- Helpers should not treat `MANAGED=false` as a block on backend writeback. It only means local autoscan does not own the source.
- Helpers should preserve a local backup before writing backend policy to `config.ini`.
