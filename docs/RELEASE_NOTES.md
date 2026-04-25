# Release Notes

## v0.1.11 - 2026-04-25

### Included

- Settings page cleanup:
  - removed duplicate API/auth/user/storage summary noise from the main settings view
  - made Game Support Modules the primary settings control
  - replaced the wide technical module table with compact module cards
  - moved fixed helper key management into a collapsed `Helper keys` panel
  - moved local `.rsmodule.zip` uploads into a collapsed advanced panel
- Settings visual refresh:
  - tighter dark layout aligned with the cleaner `My Saves` and save detail direction
  - lower information density for non-essential technical fields
  - clearer sync and refresh actions for runtime modules

### Validation Summary

- Full frontend test suite passed locally
- Frontend production build passed locally
- Security gate passed locally

## v0.1.10 - 2026-04-25

### Included

- Runtime game support modules:
  - added GitHub-backed `.rsmodule.zip` bundles under `modules/`
  - modules use sandboxed WASM plus declarative YAML, so new parser-backed details and cheats can be added without rebuilding Docker
  - module sync now validates bundle paths, file sizes, manifests, WASM ABI exports, and cheat-pack schemas before activation
- New module-backed parser/cheat coverage:
  - Game Boy / Game Boy Color: `Donkey Kong Country`, `Pokemon Red/Blue/Yellow`, `Super Mario Bros. Deluxe`, and `Wario Land II`
  - Game Boy Advance: `Wario Land 4`
  - Nintendo 64: `Banjo-Kazooie`, `Banjo-Tooie`, `Wave Race 64`, and `Yoshi's Story`
  - Nintendo DS: `New Super Mario Bros.`
  - Neo Geo: `Double Dragon` and `Metal Slug 5`
  - PlayStation: `Castlevania: Symphony of the Night`
  - PlayStation 2: `Burnout 3: Takedown` and `Mortal Kombat Shaolin Monks`
  - Sega Saturn: `Quake`
  - Super Nintendo: `Super Mario World`
  - Nintendo Wii: `Super Mario Galaxy 2`
- Logical-save module bridges:
  - PlayStation and PlayStation 2 cheat modules can target extracted logical saves instead of full memory-card images
  - Sega Saturn modules can target extracted backup-RAM entries, including YabaSanshiro-style images
  - module-backed cheat applies still create a new current save version and preserve existing history
- Save details and My Saves integration:
  - save summaries now expose module-backed cheat capabilities after module sync/rescan
  - save detail pages can render semantic module fields such as progress, lives, stage, inventory, or verified save facts
- Codebase cleanup:
  - split the runtime module backend into focused files for import, GitHub library sync, WASM runner, inspection, cheat packs, and adapter bridging
  - added contributor-facing comments around the module security model and WASM boundary

### Deploy Notes

- Existing servers should run `POST /api/modules/sync` after updating so the runtime imports the new GitHub module library.
- Run `POST /api/modules/rescan` after sync to enrich existing saves with module semantic fields and cheat capabilities.

### Validation Summary

- Cheat pack validation passed locally
- Full backend test suite passed locally
- Full frontend test suite passed locally
- Frontend production build passed locally
- Security gate passed locally
- Live Docker deploy on the internal Docker host imported `19` active modules with `0` module sync errors

## v0.1.9 - 2026-04-25

### Included

- Helper service mode backend support:
  - added `POST /helpers/heartbeat` for daemon health, sensors, config snapshots, and last-sync counters
  - added `POST /devices/{id}/command` to publish `sync.requested`, `scan.requested`, and `deep_scan.requested` over the existing SSE stream
  - device records now expose service freshness, config hashes, folder counts, and helper sync stats
- Backend-managed helper configuration:
  - Devices can now store backend-managed `configSources` and `configGlobal` policy
  - backend-managed sources survive future helper config syncs and are returned in `policy.sources[]`
  - config edits publish `config.changed` so always-on helpers can reload and write back `config.ini`
- Devices UI upgrade:
  - Devices now shows daemon status, heartbeat timing, reconcile interval, last event, last error, sensors, and quick commands
  - Manage Device can add console/runtime/profile folders before a helper has discovered saves for that console
- Helper contract documentation:
  - added `backend.md` as the canonical helper backend/service/config contract
  - updated `api.md` with heartbeat, command, config source, and global policy fields
- Sega scope alignment:
  - added supported system slugs and strict raw-save validation coverage for `sega-cd` and `sega-32x`

### Validation Summary

- Full backend test suite passed locally
- Full frontend test suite passed locally
- Frontend production build passed locally
- Security gate passed locally

## v0.1.8 - 2026-04-24

### Included

- Cross-ROM duplicate cleanup for visible save tracks:
  - identical payloads under the same canonieke game/region track are now treated as the same save even when helper ROM hashes differ
  - upload idempotency now catches those cross-ROM duplicates before writing a new version
  - `/saves`, save detail history, and the agent summary API now count unique payload hashes per visible track
- Live cleanup support:
  - rescan duplicate cleanup now removes redundant generic save records by visible track + SHA, not only by ROM hash + slot
  - explicit rollback audit versions remain preserved and still count as intentional history

### Validation Summary

- Targeted duplicate/canonical-track tests passed locally
- Full backend test suite passed locally
- Security gate passed locally

## v0.1.7 - 2026-04-24

### Included

- Save upload idempotency:
  - generic save ingest now suppresses exact-latest duplicate uploads on the same canonical save line
  - historical duplicate uploads now return a stale conflict instead of creating a new version
  - duplicate/stale handling is enforced server-side instead of trusting helpers
- PlayStation and N64 logical duplicate suppression:
  - PlayStation logical ingest now checks the full logical revision history, not only the latest revision
  - N64 controller-pak logical ingest now checks the full logical revision history too
  - identical projection payload rebuilds no longer create duplicate projection save records
- Duplicate cleanup during rescan:
  - rescan now collapses redundant generic, PlayStation, and N64 duplicate history
  - oldest duplicates survive by default, while current latest state is preserved when needed to avoid rolling cloud truth backward
  - rollback-created audit versions remain exempt from cleanup
  - rescan reporting now includes duplicate group and removed-version counts

### Validation Summary

- Backend duplicate/idempotency regression tests passed locally
- Full backend test suite passed locally

## v0.1.6 - 2026-04-24

### Included

- Unified runtime-profile handling:
  - projection-capable helper uploads, latest checks, and downloads now resolve through explicit `runtimeProfile`
  - backward-compatible aliases such as `n64Profile` and `saturnFormat` still map into the shared runtime-profile contract
  - placeholder runtime-profile rows are rejected before they can pollute the save dataset
- Runtime-aware web downloads:
  - `My Saves` and save detail downloads now open a runtime/profile selector first
  - backend save responses expose `downloadProfiles[]` so the web UI only offers compatible targets
  - non-converted saves still show `Original file` as a safe fallback choice
- Nintendo 64 controller pak sync:
  - added parser-backed controller pak ingest and projection handling for N64
  - controller pak entries are extracted into logical save tracks instead of being treated as opaque whole-pak blobs
  - N64 projection downloads now stay aligned with the requested runtime profile
- Cheat pack management:
  - added managed cheat-pack storage plus create, enable, disable, delete, and adapter-catalog API endpoints
  - added a web `Cheats` page for publishing YAML packs and reviewing adapter capabilities
  - agent/API consumers can now inspect cheat adapters and managed packs directly
- Save validation tightening:
  - weak raw-save placeholder titles are rejected or cleaned up before listing
  - title alias cleanup preserves real saves while removing noisy placeholder-style rows
  - PlayStation helper `save/latest` runtime-profile enforcement is scoped to the projection flows that actually require it

### Validation Summary

- Backend test suite passed locally
- Frontend test suite passed locally
- Frontend production build passed locally

## v0.1.5 - 2026-04-23

### Included

- Nintendo 64 cross-runtime projection:
  - helpers now upload their real local N64 save format
  - backend normalizes N64 uploads into canonical media truth
  - helper latest/download flows now project back to the explicitly requested target runtime
  - supported target profiles in tranche 1:
    - `n64/mister`
    - `n64/retroarch`
    - `n64/project64`
    - `n64/mupen-family`
    - `n64/everdrive`
- Strict N64 helper contract:
  - N64 helper requests now require `n64Profile`
  - missing `n64Profile` fails clearly for helper upload, latest, and download paths
  - new helper contract documented in `n64update.md`
- Diddy Kong Racing cheat support:
  - added parser-backed `dkr-eeprom` cheat editor
  - curated `Diddy Kong Racing` cheat pack included
  - safe slot-based editing with checksum rebuild
- N64 cheat audit coverage:
  - added repository-backed N64 cheat inventory tests
  - added supporting N64 cheat audit and intake docs

### Validation Summary

- Backend test suite passed locally
- Security gate passed locally
- Live Docker deploy updated and smoke-checked successfully

## v0.1.4 - 2026-04-23

### Included

- Safe cheat editing v1:
  - parser-backed cheat read/apply endpoints at `/save/cheats` and `/save/cheats/apply`
  - every cheat apply creates a new current save version instead of mutating history in place
  - local override support under `SAVE_ROOT/<System>/<Game>/_rsm/cheats.local.yaml`
- Curated Nintendo 64 cheat packs:
  - `Super Mario 64` via `sm64-eeprom`
  - `Mario Kart 64` via `mk64-eeprom`
- Mario Kart 64 editor coverage:
  - Grand Prix cup progress for `50cc`, `100cc`, `150cc`, and `Extra`
  - sound mode editing with checksum repair on main and backup save-info blocks
- Cheat developer documentation:
  - added `cheats.md` with pack schema, override rules, API examples, and authoring guidance

### Validation Summary

- Backend cheat tests passed locally for SM64 and Mario Kart 64
- Frontend tests and production build passed locally
- Security gate and contract checks passed locally

## v0.1.3 - 2026-04-23

### Included

- Security gate sanitization:
  - removed private-network fixture references from Saturn testdata notes
  - retained fixture hashes, sizes, and source-path examples without leaking internal host details
- Agent and ops API expansion:
  - added no-auth `/api` and `/api/v1` automation surface for saves, ROMs, devices, sync status, conflicts, helper auto-enroll, and logs
  - added paginated sync-log endpoint for the last 72 hours of helper and web save activity
- Logs UI:
  - added `Logs` to the sidebar
  - added a paginated web table for recent sync activity with device, action, game, and error state

### Validation Summary

- Repository security gate passed locally after sanitization

## v0.1.1 - 2026-04-23

### Included

- Strict Sega save-domain backend:
  - parser-led Dreamcast VMU/VMS/DCI detection and validation
  - parser-led Sega Saturn backup RAM detection and validation
  - strict raw SRAM validation for Genesis, Master System, and Game Gear
  - empty Dreamcast VMUs and empty Saturn backup RAM images are rejected as noise
- Saturn metadata extraction:
  - volume summaries for internal and cartridge backup RAM
  - per-entry filename, comment, language, timestamp, block list, and payload size
  - helper-compatible `saturnFormat` export support for `mister`, `internal-raw`, `cartridge-raw`, `mednafen`, `yabause`, `yabasanshiro`, `bup`, `ymir`, and `ymbp`
- Backend inspection metadata:
  - explicit parser level, parser id, validated system, evidence, warnings, and slot metadata on supported Sega saves
- My Saves frontend redesign:
  - compact English TreeGrid-style layout
  - text-based sidebar navigation
  - obsolete public links removed from the promoted shell
  - denser high-contrast dark styling for save management

### Validation Summary

- Backend test suite passed locally with Saturn and Dreamcast fixture coverage
- Frontend tests passed locally
- Frontend production build passed locally

## v0.1.0 - 2026-04-22

### Included

- Single-container runtime for self-hosted deploys:
  - one `app` container serves both API and frontend SPA
  - one GHCR image: `ghcr.io/joeblack2k/retrosavemanager`
- Persistent storage hardening:
  - save and config mounts must now be explicit persistent host paths
  - demo bootstrap save is disabled by default for production-style deploys
  - startup continues to rehydrate save state from disk-backed metadata under `SAVE_ROOT`
- Save manager UI refresh:
  - compact dark `My Saves` tree/grid layout
  - text-based sidebar navigation
  - tighter typography and higher-contrast styling
- PlayStation save-domain improvements:
  - extracted logical saves are the sync truth
  - PS1/PS2 projections remain helper-compatible downloads
  - real PS1 memory-card detection only
  - real PS2 memory-card detection only
  - unsupported PS1/PS2 save-state noise is rejected during rescan
- Memory-card detail enrichment:
  - PS1 entry titles and icon previews
  - PS2 entry titles from `icon.sys`
  - PS2 entry previews, product codes, block counts, and size stats
- Live rescan behavior improved:
  - noisy or false-positive PlayStation records are pruned
  - valid PS memory cards remain grouped as `Memory Card N`
- PS1 projection integrity fix:
  - generated PS1 downloads now emit valid raw card header frames
  - frame `0` and frame `63` now contain canonical `MC` markers plus valid XOR checksums
  - helper-facing `/saves/download` output is validated by backend integration tests

### Deploy Notes

- Default runtime is direct HTTP on port `80`
- Docker Compose default is a single `app` service
- Default deploy is GHCR-only; it no longer builds locally during `up`
- Optional local image builds now use `deploy/docker-compose.build.yml` via `deploy/scripts/build-local.sh`
- Macvlan stays available as an optional override

### Validation Summary

- Backend tests passed on the test bench VM
- Frontend tests and production build passed locally
- Live deploy validated on the internal Docker host
- GitHub release tagging and release publication are part of this release
