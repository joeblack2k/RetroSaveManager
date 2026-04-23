# RetroSaveManager Cheats Intake and Authoring Guide

Last updated: 2026-04-23 CEST

This document is the handoff guide for anyone collecting cheat support for RetroSaveManager.

Use this file when:

- a separate agent is researching cheats for games
- someone wants to add a new cheat-enabled game
- someone wants to hand Codex enough material to implement a safe cheat editor
- someone wants to understand what is acceptable evidence and what is still too speculative

This is not a generic cheat-code database.
RetroSaveManager only supports safe, parser-backed save editing.

## Core rules

The cheat system is intentionally strict:

- parser-backed only
- safe structured editing only
- no raw hex editor in the web UI
- no executable user scripts inside cheat packs
- no RAM-only trainer codes
- no filename guessing as proof
- no speculative fields
- every cheat apply creates a new save version
- the new version becomes the current sync save
- existing history, rollback, download, delete, and helper flows remain unchanged

If a save cannot be proven structurally, the correct output is a research dossier, not a cheat implementation.

## Current backend status

Curated packs currently ship for:

- `n64/super-mario-64` via `sm64-eeprom`
- `n64/mario-kart-64` via `mk64-eeprom`

Current curated pack files:

```text
contracts/cheats/packs/n64/super-mario-64.yaml
contracts/cheats/packs/n64/mario-kart-64.yaml
```

Current backend cheat endpoints:

- `GET /save/cheats?saveId=...`
- `GET /v1/save/cheats?saveId=...`
- `POST /save/cheats/apply`
- `POST /v1/save/cheats/apply`

Current load order:

1. Curated pack from the repo/image.
2. Optional local override from the save tree.
3. Backend parser/editor validates the real payload.
4. Only then does the UI expose cheat editing.

Local override path:

```text
SAVE_ROOT/<System>/<Game>/_rsm/cheats.local.yaml
```

Example:

```text
SAVE_ROOT/Nintendo 64/Super Mario 64/_rsm/cheats.local.yaml
```

There is no runtime auto-download of cheat packs in the current phase.
Deployments stay deterministic and offline-friendly.

## How the cheat agent should access saves via API

The cheat agent should use the API, not the web UI.

Current phase defaults:

- auth disabled
- primary machine API: `/api` and `/api/v1`
- helper compatibility API: `/` and `/v1`

Use a sanitized base URL in scripts and examples:

```bash
export RSM_BASE_URL="https://rsm.example.invalid"
export RSM_API="$RSM_BASE_URL/api"
```

Recommended flow for the cheat agent:

1. List candidate save tracks with `GET /api/saves`
2. Inspect one save track with `GET /api/saves/{id}`
3. Download the actual payload with `GET /api/saves/{id}/download`
4. If cheat support already exists, inspect capability with `GET /save/cheats?saveId=...`
5. Apply structured cheats with `POST /save/cheats/apply`

Important detail:

- save discovery and payload download should use the agent API under `/api`
- cheat read/apply endpoints currently live on the compatibility layer under `/save/...`, not `/api/...`

### Save list

Use `GET /api/saves` to find candidate save tracks.

Useful query params:

- `systemSlug`
- `q`
- `limit`
- `offset`
- `gameId`
- `romSha1`

Example:

```bash
curl -s "$RSM_API/saves?systemSlug=n64&limit=50&offset=0&q=mario" | jq
```

The response includes machine-usable action URLs per save track, so the cheat agent can follow those directly instead of rebuilding URLs manually.

### Save detail and history

Use `GET /api/saves/{id}` to inspect one save track and its version history.

Example:

```bash
curl -s "$RSM_API/saves/save-123" | jq
```

For PlayStation logical saves, the API may also require `psLogicalKey` on detail-oriented operations.
If the list or detail response includes a PlayStation logical key, preserve it.

### Save download

Use `GET /api/saves/{id}/download` to fetch the actual save payload for offline analysis.

Example:

```bash
curl -L "$RSM_API/saves/save-123/download" -o save.bin
```

That download endpoint is the correct source for:

- reverse engineering
- checksum verification
- sample comparison
- parser development

### Existing cheat capability

If a game may already have cheat support, inspect it with:

```bash
curl -s "$RSM_BASE_URL/save/cheats?saveId=save-123" | jq
```

That returns:

- whether cheats are supported
- the `editorId`
- visible sections
- presets
- current values
- slot values when the save contains multiple internal save files

### Apply cheats

To apply cheats, call:

```bash
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

Cheat apply semantics:

- creates a new save version
- makes that new version the current sync save
- does not mutate old history in place

### Practical guidance for the separate agent

When researching a new game, the agent should usually do this:

1. Query `/api/saves` for the target system and title candidates
2. Download one or more real save payloads from `/api/saves/{id}/download`
3. Build the research dossier from those real payloads
4. Only use `/save/cheats` if the goal is to inspect already implemented cheat support
5. Only use `/save/cheats/apply` for validating an existing editor, not for guessing fields

## The most important rule for the cheat agent

Do not hand over "cheats" as game genie codes, emulator patches, RAM pokes, or forum guesses.

Hand over one of these three things:

1. Research-only dossier
2. Pack-ready dossier
3. Code-ready dossier

If you are not sure which one you have, it is probably research-only.

## Three contribution levels

### 1. Research-only dossier

Use this when you can identify the save format and some likely fields, but you cannot yet prove safe write behavior.

This is still useful.
It gives Codex a clean starting point without pretending the game is ready.

Research-only means:

- save format is partially understood
- field meanings may still be incomplete
- checksum or mirrored-copy behavior may still be unclear
- editor implementation should not be written yet

### 2. Pack-ready dossier

Use this only when the game already matches an existing backend editor and only needs:

- title aliases
- payload sizes
- payload formats
- safe field exposure that the existing editor already understands
- presets built from already supported fields

This is uncommon.
Most new games will not be pack-only.

### 3. Code-ready dossier

Use this when the game needs a new parser/editor or new operation kinds.

Code-ready means the agent can provide:

- structurally verified save format information
- safe read model
- safe write model
- checksum or mirrored-block repair logic
- exact editable fields
- safe ranges or enum options
- verification steps for before/after behavior

If the dossier is code-ready, Codex can usually turn it into:

- a backend editor
- one curated YAML pack
- tests
- frontend exposure through the existing cheat modal

## What the cheat agent must hand over for every game

Every game handoff should be a small, self-contained package.
Do one game at a time unless multiple titles are proven byte-identical and use the same exact editor behavior.

### 1. Game identity

Provide:

- `systemSlug`
- canonical game title
- known region variants
- known save extensions
- known runtime/emulator origins
- whether the save is per-slot, per-profile, per-file, or one monolithic blob

Example:

```text
systemSlug: n64
title: Super Mario 64
regions: USA, Europe, Japan
extensions: .eep
runtimes: MiSTer, RetroArch
save model: 4 internal save files inside one EEPROM blob
```

### 2. Real sample saves

At minimum provide:

- 2 real saves
- preferably 3 or more distinct progression states
- original filename
- exact byte size
- SHA256
- runtime or helper source
- short note describing what changed between samples

Good sample set:

- early-game save
- mid-game save
- late-game or completed save

If the format contains multiple internal slots, try to vary those too.

### 3. Structural format evidence

This is the most important part.

Provide any verified information you have about:

- header layout
- slot table layout
- block layout
- mirrored copies
- checksums
- CRCs
- magic values
- record boundaries
- active/inactive markers
- region markers if present
- compression or encoding

Accepted evidence sources:

- real sample comparison
- open source parser code
- emulator source
- format documentation
- reverse engineering notes
- decompilation or disassembly notes

Best case: you can point to both docs and real sample validation.

### 4. Editable fields

For every field you want exposed in the web UI, provide:

- stable field id
- human label
- field type
- where it lives in the save
- how to read it
- how to write it
- allowed values
- what makes it safe
- proof that the field meaning is correct

Valid field types today:

- `boolean`
- `integer`
- `enum`
- `bitmask`

If the field does not fit one of those safely, it probably should not ship yet.

### 5. Presets

Presets are optional.

A preset is only a named collection of safe field changes, for example:

- unlock all cups
- unlock all caps
- give both Bowser keys

Do not propose presets that depend on hidden, unverified, or game-breaking values.

### 6. Safety and validation rules

For every game, provide:

- exact payload size rules
- exact allowed formats or extensions
- whether there are duplicate/mirrored data blocks
- checksum or CRC repair behavior
- invalid values that must be rejected
- max/min values that should be clamped or refused
- any fields that look editable but are too risky

### 7. Verification steps

Tell Codex how to verify the edit after apply.

Examples:

- start the game and check whether File A has Wing Cap unlocked
- confirm all 150cc cups are Gold
- verify checksum passes in a known parser
- compare before/after bytes at specific offsets

Without verification steps, the implementation is not done.

## Exact handoff format to give Codex

When the separate agent finishes one game, hand over a single Markdown dossier using this template.

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
- existing editor can be reused? yes/no
- if yes: which editorId?
- if no: what new parser/editor is needed?
- checksum repair required:
- mirrored-write required:

## 7. Verification
- before/after checks:
- in-game confirmation:
- parser/tool confirmation:

## 8. Open Questions
- ...

## 9. Decision
- research-only
- pack-ready
- code-ready
```

That is the preferred handoff format.

## What Codex needs to implement a new game

There are two different implementation paths.

### Path A: existing editor + new pack

This only works if the game already fits an existing backend editor.

Today, existing editors are:

- `sm64-eeprom`
- `mk64-eeprom`

In that case, the agent should provide:

- the dossier
- the final YAML pack
- any new aliases
- payload sizes
- payload formats
- presets if needed

### Path B: new editor + new pack

This is the normal path for a new game.

In that case, the agent should provide:

- the dossier
- parser/read model
- write model
- checksum or CRC repair rules
- mirrored-write rules if applicable
- proposed field schema
- proposed preset schema

Codex will then usually add:

- `backend/cmd/server/cheat_<game>.go`
- `backend/cmd/server/cheat_<game>_test.go`
- an editor registration entry in `backend/cmd/server/cheat_service.go`
- `contracts/cheats/packs/<system>/<game>.yaml`

## YAML cheat pack schema

Cheat packs live here:

```text
contracts/cheats/packs/<system>/<game>.yaml
```

Examples:

```text
contracts/cheats/packs/n64/super-mario-64.yaml
contracts/cheats/packs/n64/mario-kart-64.yaml
```

### Top-level fields

- `gameId`: stable identity, example `n64/super-mario-64`
- `systemSlug`: backend system slug, example `n64`
- `editorId`: backend editor id, example `sm64-eeprom`
- `title`: primary display title
- `match.titleAliases[]`: accepted title aliases for matching
- `payload.exactSizes[]`: exact payload sizes in bytes
- `payload.formats[]`: accepted formats/extensions as seen by the backend
- `selector`: optional selector for internal save slots or profiles
- `sections[]`: visible field groups for the cheat modal
- `presets[]`: prebuilt safe update groups

### Minimal example

```yaml
gameId: n64/example-game
systemSlug: n64
editorId: example-editor
title: Example Game
match:
  titleAliases:
    - Example Game
payload:
  exactSizes: [512]
  formats: [eep]
sections:
  - id: progress
    title: Progress
    fields:
      - id: unlockedThing
        label: Unlock Thing
        type: boolean
        op:
          kind: flag
          flag: unlockedThing
presets:
  - id: unlockAll
    label: Unlock All
    updates:
      unlockedThing: true
```

### Selector example

Use a selector only when one uploaded save blob contains multiple internal save files.

```yaml
selector:
  id: file
  label: Save File
  type: save-file
  options:
    - id: A
      label: File A
    - id: B
      label: File B
```

### Local override example

Local overrides are for one game folder inside `SAVE_ROOT`.

```yaml
gameId: n64/super-mario-64
systemSlug: n64
editorId: sm64-eeprom
presets:
  - id: localCastleBoost
    label: Local Castle Boost
    description: Example local preset for this one save folder.
    updates:
      haveWingCap: true
      haveMetalCap: true
```

Override behavior:

- title aliases are merged
- payload constraints can be replaced
- sections with the same `id` are merged by field `id`
- presets with the same `id` are replaced
- new sections and presets are appended

## Current operation model

The YAML file is declarative.
It does not execute arbitrary code.

Each field declares `op.kind`, and the backend editor must understand that operation.

Current operation kinds already implemented:

- `flag`
- `secretStars`
- `courseStars`
- `courseCannon`
- `courseCoinScore`
- `soundMode`
- `gpCupPoints`

Important:

- You cannot invent a new `op.kind` in YAML and expect it to work.
- If a new game needs new semantics, that means new backend code.
- If all you have is "write byte X at offset Y", that is not enough by itself.

## Matching rules

Pack matching is currently based on:

- `systemSlug`
- title alias matching against cleaned display title
- payload size
- payload format
- successful backend parser read

That means the agent should always provide:

- the exact cleaned game title
- region/title variants that may appear in filenames or metadata
- exact size expectations
- exact format expectations

## API behavior

### Read resolved cheat schema and current values

- `GET /save/cheats?saveId=...`
- `GET /v1/save/cheats?saveId=...`

Response highlights:

- `supported`
- `gameId`
- `systemSlug`
- `editorId`
- `title`
- `availableCount`
- `selector`
- `sections`
- `presets`
- `values`
- `slotValues`

### Apply structured cheat updates

- `POST /save/cheats/apply`
- `POST /v1/save/cheats/apply`

Example:

```json
{
  "saveId": "save_123",
  "editorId": "mk64-eeprom",
  "presetIds": ["unlockExtraMode"],
  "updates": {
    "soundMode": "headphones"
  }
}
```

Selector example:

```json
{
  "saveId": "save_456",
  "editorId": "sm64-eeprom",
  "slotId": "A",
  "updates": {
    "haveWingCap": true,
    "bob100Coin": 120
  }
}
```

Apply semantics:

1. The backend loads the current save payload.
2. The parser/editor validates the payload.
3. Presets and structured fields are resolved.
4. The backend repairs checksum, CRC, or mirrored blocks if needed.
5. RetroSaveManager writes a new save version.
6. The new version becomes the current sync save.
7. Older history remains available.

Cheat apply never mutates an old version in place.

## What not to give Codex

Do not hand over only:

- Action Replay codes
- GameShark codes
- Cheat Engine RAM addresses
- emulator memory watch values
- save filenames without payload proof
- forum posts without confirmation
- one lonely sample save
- guesses based on title alone
- values that require arbitrary raw hex writes
- fields with unknown checksum impact

Those can still be kept in the research dossier, but they are not enough for implementation.

## Definition of done for a cheat-enabled game

A game is not done when we have "found cheats online".

A game is done when all of these are true:

- the save format is structurally verified
- the backend can parse the save safely
- the backend can write the save safely
- checksum or mirrored-block repair is handled
- the YAML pack loads cleanly
- `GET /save/cheats` returns `supported: true`
- `POST /save/cheats/apply` creates a new current save version
- tests cover read and apply behavior
- normal save-management behavior still works

## Recommended workflow for the separate cheat agent

For each game:

1. Identify the save format and runtime variants.
2. Collect at least 2 to 3 real save samples.
3. Compare samples and map stable structure.
4. Prove checksum, CRC, mirror, or backup-copy behavior.
5. List only the fields that are structurally verified.
6. Build the Markdown dossier.
7. Decide: research-only, pack-ready, or code-ready.
8. Hand exactly that dossier to Codex.

If a field or format is uncertain, call it out clearly and stop short of implementation.

That keeps the repo clean and keeps cheat support safe.
