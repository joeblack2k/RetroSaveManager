# Release Notes

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
