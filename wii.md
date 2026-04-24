# RetroSaveManager Wii Save Support

Last updated: 2026-04-25 00:25 Europe/Amsterdam

## Current Scope

RetroSaveManager supports Wii save ingestion for official SD-card exported `data.bin` saves.

The first verified sample is:

- Game: Super Mario Galaxy 2
- Wii title/game code: `SB4P`
- Region: Europe / PAL
- Expected source paths:
  - `SB4P/data.bin`
  - `private/wii/title/SB4P/data.bin`

## Helper Contract

Helpers should upload the real local Wii save export. Helpers do not need to decode, decrypt, or edit the save.

Required or recommended fields:

- `system=wii`
- `slotName`, preferably the source path or a stable local slot label
- `device_type`
- `fingerprint`
- `rom_sha1`, when the helper knows the exact ROM/disc image hash
- `wiiTitleId`, when uploading a raw `data.bin` without its parent folder path

Recommended upload formats:

1. Zip upload with path evidence:

```text
private/wii/title/SB4P/data.bin
```

or:

```text
SB4P/data.bin
```

2. Raw `data.bin` upload with explicit metadata:

```text
system=wii
wiiTitleId=SB4P
slotName=SB4P/data.bin
```

## Backend Responsibilities

The backend validates and stores the save. The helper does not extract semantic fields.

For Wii `data.bin`, the backend currently verifies:

- `media-verified`: payload is non-empty, non-noise, not an archive/executable, and not blank media
- `structure-verified`: Wii backup header and embedded file header are structurally valid
- `rom-verified`: `rom_sha1` was provided by a trusted helper or uploader
- `semantic-verified`: reserved for future per-game decrypted save decoders

The current Super Mario Galaxy 2 `data.bin` path is structure-verified. It is not semantic-verified yet because official Wii SD exports are encrypted containers.

## Game Title Enrichment

The backend enriches known Wii title/game codes. Current catalog entry:

| Code | Game | Region |
| --- | --- | --- |
| `SB4P` | Super Mario Galaxy 2 | EU / PAL |

Unknown title codes are still valid Wii saves when the `data.bin` structure verifies. They display as `Wii Save <CODE>` until a catalog entry is added.

## Cheats

Wii cheat editing is intentionally disabled until a real semantic decoder exists for the decrypted game save data.

For Super Mario Galaxy 2, agents should provide one of these before cheat editing is enabled:

- verified decrypted save structure documentation, or
- multiple known-before/known-after samples that prove exact fields and checksums, or
- a safe parser implementation that can read and write the save with integrity repair

No raw hex scripts are accepted. Wii cheats must follow the same parser-backed, structured YAML model used by the rest of RetroSaveManager.

## Download Behavior

Wii saves currently download as the stored validated `data.bin` payload. Runtime-specific Wii projections can be added later when we have concrete emulator-specific format differences to support.
