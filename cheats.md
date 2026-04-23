# RetroSaveManager Cheat Editing

Last updated: 2026-04-23 00:00 CEST

This document explains how the RetroSaveManager cheat system works, how curated cheat packs are loaded, how local overrides work, and what helper and frontend developers can expect from the backend.

## Goals

The cheat system is intentionally strict:

- parser-backed only
- safe structured editing only
- no raw hex editor in the web UI
- no executable user scripts inside cheat packs
- every cheat apply creates a new save version
- the new version becomes the current sync save
- existing history, rollback, download, delete, and helper flows remain unchanged

## Current support

Curated packs currently ship for:

- `n64/super-mario-64` via `sm64-eeprom`
- `n64/mario-kart-64` via `mk64-eeprom`

If a save does not match a curated pack plus a parser/editor, the save still works normally in RetroSaveManager, but no cheat action is exposed.

## Source of truth

RetroSaveManager loads cheats from two layers:

1. Curated pack shipped inside the repo/image.
2. Optional local override file stored under the save root for one game.

Load order:

1. Curated pack loads first.
2. Local override is merged on top.
3. Backend parser/editor validates the real payload before the cheat action is exposed.

There is no runtime auto-download of cheat packs in the current phase. Deployments stay deterministic and offline-friendly.

## Curated pack locations

Curated packs live in:

```text
contracts/cheats/packs/<system>/<game>.yaml
```

Examples:

```text
contracts/cheats/packs/n64/super-mario-64.yaml
contracts/cheats/packs/n64/mario-kart-64.yaml
```

## Local override location

A game-specific local override lives inside the backed-up save tree:

```text
SAVE_ROOT/<System>/<Game>/_rsm/cheats.local.yaml
```

Example:

```text
SAVE_ROOT/Nintendo 64/Super Mario 64/_rsm/cheats.local.yaml
```

That keeps cheat customization inside the same backup root as the save data.

## Pack schema

Cheat packs use YAML.

Top-level fields:

- `gameId`: stable game identity, for example `n64/super-mario-64`
- `systemSlug`: backend system slug, for example `n64`
- `editorId`: parser/editor implementation id, for example `sm64-eeprom`
- `title`: primary display title
- `match.titleAliases[]`: allowed title aliases used during matching
- `payload.exactSizes[]`: exact payload sizes in bytes
- `payload.formats[]`: accepted save formats or extensions
- `selector`: optional extra selector such as SM64 file `A/B/C/D`
- `sections[]`: visible field groups for the cheat modal
- `presets[]`: prebuilt structured changes

### Field types

Supported field types today:

- `boolean`
- `integer`
- `enum`
- `bitmask`

### Operation model

The YAML pack stays declarative. It does not contain executable code.

Each field declares an `op.kind` that the backend editor understands. The parser/editor maps that safe field into real save bytes.

Examples of current parser-backed operation kinds:

- SM64:
  - `flag`
  - `secretStars`
  - `courseStars`
  - `courseCannon`
  - `courseCoinScore`
- MK64:
  - `soundMode`
  - `gpCupPoints`

## Minimal pack example

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

## Local override example

A local override can add or replace fields or presets for one game folder.

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

- matching aliases are merged
- payload constraints can be replaced
- sections with the same `id` are merged by field `id`
- presets with the same `id` are replaced
- new sections and presets are appended

## Backend API

The cheat system is available on both root and `/v1` compatibility bases.

### Read resolved cheat schema and current values

- `GET /save/cheats?saveId=...`
- `GET /v1/save/cheats?saveId=...`

Response shape highlights:

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

Example request:

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

SM64 example with a selector:

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

## Apply semantics

When cheats are applied:

1. The backend loads the original save payload.
2. The parser/editor validates that the payload matches the expected game format.
3. Structured fields and presets are applied.
4. The parser/editor repairs any required checksum or mirrored data.
5. RetroSaveManager writes a new save version.
6. That new version becomes the current sync save.
7. Existing history remains available for details and rollback.

Cheat apply never mutates an old version in place.

## Frontend behavior

`My Saves` can show a cheat action only when:

- `cheats.supported = true`
- `cheats.availableCount > 0`

If no safe editor is available, the save still shows normal actions like:

- details
- download
- delete
- sync-save selection
- rollback/history

## Helper compatibility

Helpers do not need any cheat-specific upload changes.

Important points:

- helpers continue uploading raw save payloads the normal way
- cheat editing is backend-driven
- helper compatibility endpoints stay unchanged
- cheat-applied saves appear as ordinary newer save versions to helpers

## Authoring guidance

Use a curated pack when:

- the save format is structurally verified
- the byte layout is well understood
- checksum or mirrored-block behavior is known
- the fields can be modeled safely as booleans, integers, enums, bitmasks, or presets

Do not add a cheat pack when:

- the format still depends on filename guessing
- the field meanings are speculative
- the editor would need arbitrary hex writes
- checksum repair is not understood

That keeps the system stable and prevents destructive edits.
