# N64 Cheat Audit

Last updated: 2026-04-23 CEST

This audit was derived from repository evidence inside `RetroSaveManager`.
It does not claim that real end-user N64 saves are present in this workspace.

## What was actually found

- `cheats.md` confirms only two curated N64 cheat packs currently ship:
  - `n64/super-mario-64` via `sm64-eeprom`
  - `n64/mario-kart-64` via `mk64-eeprom`
- No real N64 save tree such as `SAVE_ROOT/Nintendo 64/...` was found in this workspace.
- N64 save titles do appear in backend tests and API contract coverage.
- Matching is strict: title, payload size, payload format, and successful parser read must all pass before cheats are exposed.

## Observed N64 titles in repo tests

These titles were found in backend tests as uploaded N64 save filenames:

- `Super Mario 64`
- `Mario Kart 64`
- `Star Fox 64`
- `Wave Race 64`
- `F-Zero X`
- `Yoshi's Story`

These generic files also exist in tests, but they are not useful for cheat authoring because they do not prove game identity:

- `slot1.eep`
- `policy-n64.eep`
- `policy-n64-2.eep`

## Status by title

### Supported now

- `Super Mario 64`
  - Status: supported
  - Editor: `sm64-eeprom`
  - Evidence: curated pack exists, parser/editor exists, automated cheat endpoint tests exist
  - Safe scope today: flags, castle secrets, course stars, cannons, 100-coin scores, presets

- `Mario Kart 64`
  - Status: supported
  - Editor: `mk64-eeprom`
  - Evidence: curated pack exists, parser/editor exists, automated cheat endpoint tests exist
  - Safe scope today: sound mode, Grand Prix cup progress, presets

### Not supported from current repo evidence

- `Star Fox 64`
  - Status: not supported
  - Why: no curated pack, no parser/editor, no verified editable-field dossier
  - Required next step: real samples plus a research dossier, likely followed by a new backend editor

- `Wave Race 64`
  - Status: not supported
  - Why: no curated pack, no parser/editor, no verified editable-field dossier
  - Required next step: real samples plus a research dossier, likely followed by a new backend editor

- `F-Zero X`
  - Status: not supported
  - Why: no curated pack, no parser/editor, no verified editable-field dossier
  - Required next step: real samples plus a research dossier, likely followed by a new backend editor

- `Yoshi's Story`
  - Status: not supported
  - Why: no curated pack, no parser/editor, no verified editable-field dossier
  - Required next step: real samples plus a research dossier, likely followed by a new backend editor

## Important non-hallucination boundary

Having an `.eep` filename or an N64 title is not enough.
RetroSaveManager only exposes cheats when all of this is true:

- a curated pack matches the cleaned title
- the payload size and format match
- the backend parser can successfully read the actual save payload

That means even `Super Mario 64` and `Mario Kart 64` do not get cheat support for fake or malformed 512-byte payloads.

## Verification added in code

The backend test `backend/cmd/server/cheat_n64_inventory_test.go` now checks:

- `Super Mario 64` only reports cheats for a valid parser-backed payload
- `Mario Kart 64` only reports cheats for a valid parser-backed payload
- invalid payloads do not fake support for those titles
- `Star Fox 64`, `Wave Race 64`, `F-Zero X`, and `Yoshi's Story` remain unsupported with current repo evidence
