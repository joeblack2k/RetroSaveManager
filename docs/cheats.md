# RetroSaveManager Cheat Library Guide

Last updated: 2026-04-24 CEST

This document explains how cheat packs are added, synced, validated, and used by RetroSaveManager.
It is written for AI agents and developers who create safe save-game cheat packs.

RetroSaveManager does not host a generic trainer-code database.
It only supports parser-backed save editing that the backend can validate and repair safely.

## Current Workflow

The default workflow is GitHub-backed:

1. Add a YAML cheat pack to `cheats/packs/<system>/<game>.yaml`.
2. Commit and push the YAML file to the configured GitHub repository.
3. Open the web app at `/app/cheats`.
4. Click `Sync from GitHub`.
5. The server validates every pack before it becomes active.
6. Active packs automatically appear as cheat actions in `My Saves` for matching saves.

A Docker/image rebuild is not required for new YAML packs after this feature has been deployed.
The running server imports valid packs into:

```text
SAVE_ROOT/_rsm/cheats/packs/<packId>/
```

Invalid YAML is never activated. Validation errors are stored and shown on `/app/cheats`.

## GitHub Library Source

Default source:

```text
CHEAT_LIBRARY_REPO=joeblack2k/RetroSaveManager
CHEAT_LIBRARY_REF=main
CHEAT_LIBRARY_PATH=cheats/packs
```

Optional environment overrides:

```text
CHEAT_LIBRARY_REPO=<owner>/<repo>
CHEAT_LIBRARY_REF=<branch-or-ref>
CHEAT_LIBRARY_PATH=<folder-inside-repo>
```

Public GitHub repositories only are supported in this phase.
Do not put tokens or secrets in cheat files, docs, examples, or environment snippets.

## Bundled Fallback Packs

The image still ships bundled fallback packs from:

```text
contracts/cheats/packs/<system>/<game>.yaml
```

Load priority:

1. Runtime-managed GitHub or uploaded packs in `SAVE_ROOT/_rsm/cheats/packs`.
2. Bundled fallback packs in `contracts/cheats/packs`.
3. Optional per-game local override at `SAVE_ROOT/<System>/<Game>/_rsm/cheats.local.yaml`.

GitHub packs override bundled packs by `packId`.
Local enabled, disabled, or deleted status for the same `packId` is preserved during sync.
There is no automatic prune in v1, so removing a YAML file from GitHub does not delete the local runtime copy.

## API Endpoints

Use generic base URLs in scripts and docs:

```bash
export RSM_BASE_URL="https://rsm.example.invalid"
export RSM_API="$RSM_BASE_URL/api"
```

Library endpoints:

```bash
curl -s "$RSM_API/cheats/library" | jq
curl -s -X POST "$RSM_API/cheats/library/sync" | jq
```

Compatibility alias:

```bash
curl -s "$RSM_BASE_URL/api/v1/cheats/library" | jq
curl -s -X POST "$RSM_BASE_URL/api/v1/cheats/library/sync" | jq
```

Existing management endpoints remain available:

```bash
curl -s "$RSM_API/cheats/packs" | jq
curl -s "$RSM_API/cheats/adapters" | jq
```

Existing save editing endpoints remain unchanged:

```bash
curl -s "$RSM_BASE_URL/save/cheats?saveId=save-123" | jq
curl -s -X POST "$RSM_BASE_URL/save/cheats/apply" \
  -H 'Content-Type: application/json' \
  -d '{
    "saveId": "save-123",
    "editorId": "sm64-eeprom",
    "slotId": "A",
    "updates": {
      "haveWingCap": true
    }
  }' | jq
```

Applying a cheat always creates a new save version and makes that version the current sync save.
Old history remains available for rollback.

## Cheat Pack Rules

Every pack must be declarative YAML only.

Allowed:

- title aliases and match metadata
- payload size and format constraints
- safe boolean, integer, enum, and bitmask fields
- safe presets that combine supported fields

Not allowed:

- executable scripts
- raw hex-edit expressions
- RAM-only trainer codes
- Game Genie or Action Replay codes as direct backend edits
- filename-only proof
- speculative fields
- values that the backend editor cannot validate

If a save format is not understood well enough, create a research dossier first instead of a live pack.

## YAML Location

New GitHub-managed packs go here:

```text
cheats/packs/<system>/<game>.yaml
```

Examples:

```text
cheats/packs/n64/super-mario-64.yaml
cheats/packs/snes/donkey-kong-country.yaml
```

Bundled fallback copies may also exist under `contracts/cheats/packs`, but agents should edit the GitHub library path for new runtime-syncable packs.

## Minimal Pack Shape

```yaml
packId: n64--super-mario-64
schemaVersion: 1
adapterId: sm64-eeprom
gameId: n64/super-mario-64
systemSlug: n64
editorId: sm64-eeprom
title: Super Mario 64
match:
  titleAliases:
    - Super Mario 64
payload:
  exactSizes:
    - 512
  formats:
    - eep
sections:
  - id: abilities
    title: Abilities
    fields:
      - id: haveWingCap
        label: Wing Cap
        type: boolean
presets:
  - id: unlockCaps
    label: Unlock caps
    updates:
      haveWingCap: true
```

The backend adapter decides whether each field is valid.
A YAML pack can only expose fields already known by that adapter.

## Current Active Editors

Current parser-backed editors include:

- `sm64-eeprom` for Nintendo 64 Super Mario 64 EEPROM saves
- `mk64-eeprom` for Nintendo 64 Mario Kart 64 EEPROM saves
- `dkr-eeprom` for Nintendo 64 Diddy Kong Racing EEPROM saves
- `dkc-sram` for SNES Donkey Kong Country SRAM saves

If a game does not fit an existing editor, the agent must provide a code-ready dossier and a backend editor must be implemented first.

## Agent Workflow For New Games

For each game, produce one of these handoffs:

1. Research-only dossier: format is partially understood, but write safety is not proven.
2. Pack-ready dossier: an existing backend editor supports the game and only YAML is needed.
3. Code-ready dossier: a new parser/editor is required and enough evidence exists to implement it safely.

Recommended research flow:

1. Query saves with `GET /api/saves`.
2. Inspect a save with `GET /api/saves/{id}`.
3. Download payloads with `GET /api/saves/{id}/download`.
4. Compare multiple real samples.
5. Confirm size, format, checksum, mirrors, and field meanings.
6. Add or propose YAML only after the backend editor can validate every field.

## Dossier Template

```md
# Cheat Intake: <Game Title>

## 1. Identity
- systemSlug:
- canonicalTitle:
- regions:
- extensions:
- runtimes:
- saveModel:

## 2. Samples
| file | size | sha256 | runtime | notes |
|---|---:|---|---|---|
| ... | ... | ... | ... | ... |

## 3. Format Evidence
- container/header:
- slot layout:
- checksum/crc:
- mirrored data:
- parser/doc sources:
- confidence:

## 4. Editable Fields
| fieldId | label | type | slot scope | safe values | read/write location | proof |
|---|---|---|---|---|---|---|
| ... | ... | ... | ... | ... | ... | ... |

## 5. Presets
- preset id:
- label:
- updates:

## 6. Required Backend Logic
- existing editor can be reused:
- new parser/editor needed:
- checksum repair required:
- mirrored-write required:

## 7. Verification
- before/after checks:
- in-game confirmation:
- parser/tool confirmation:

## 8. Open Questions
- ...

## 9. Decision
- research-only / pack-ready / code-ready
```

## Sync Result Semantics

`POST /api/cheats/library/sync` returns:

- configured GitHub repo/ref/path
- `lastSyncedAt`
- imported pack count
- validation error count
- imported pack paths and statuses
- validation errors by source path

A sync can partially succeed.
Valid packs become available immediately; invalid packs are skipped and reported.
