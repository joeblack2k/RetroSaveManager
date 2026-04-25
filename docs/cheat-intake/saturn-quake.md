# Cheat Intake: Quake

## 1. Identity
- systemSlug: saturn
- canonicalTitle: Quake
- regions: USA fixture verified; PAL/Japanese saves not independently sampled
- extensions: `.sav`, `.srm`, `.ram`, `.bkr`, `.bcr`
- runtimes: Saturn internal/cartridge backup RAM, raw or byte-expanded; MiSTer combined interleaved fixture verified
- saveModel: Saturn backup RAM archive entry named `LOBOQUAKE__`

## 2. Samples
| file | size | sha256 | runtime | notes |
|---|---:|---|---|---|
| `backend/cmd/server/testdata/saturn_quake_usa.sav` | 1114112 | `647d1dce8084fa694dcd0e6a0e49cba64f32746e8162af8079fbf8894a8183c9` | MiSTer combined interleaved Saturn backup RAM | Contains one internal archive entry `LOBOQUAKE__` with comment `save games`; entry payload SHA-256 `873e40fe1e7d027c04c2f258ac046964e38c32cadd79e4b38aa4519ee53de896`. |

## 3. Format Evidence
- container/header: Saturn backup RAM image with repeated `BackUpRam Format` header, matching the existing backend Saturn parser.
- archive entry: internal volume, filename `LOBOQUAKE__`, comment `save games`, English language byte, date present, entry payload length `1408` bytes.
- entry layout: `1408 = 12 * 116 + 16`; the verified sample has twelve non-empty 116-byte records followed by sixteen zero padding bytes.
- record integrity: each 116-byte record ends with a big-endian 16-bit checksum at offset `0x72`; the checksum is the byte sum of record bytes `0x00..0x71`. In the fixture every record stores and computes `0x031e` (`798`).
- validity marker: record offset `0x6e` is `0x0001` in all non-empty fixture records. This matches the Lobotomy `SaveRec { SaveState state; short valid; }` backup pattern, with Quake adding a checksum word after the record state/marker area.
- mirrors/copies: all twelve fixture records are byte-identical. The module reports that fact, but does not use it as a repair mirror because Quake's UI exposes multiple save slots and more independent samples are needed.
- external corroboration: SaturnQuakePC extracted text includes `LOBOQUAKE__`, `save games`, save-file creation/overwrite prompts, level names, and difficulty messages. Lobotomy's SlaveDriver source shows the same Saturn BUP filename/comment/raw-`SaveRec` write pattern for sibling Saturn titles. id Software's Quake source proves initial new-level values such as health `100`, minimum/current shells/ammo `25`, zero armor, weapon/item bitmasks, and skill values.
- confidence: high for container, entry identity, 12-record geometry, checksum, and valid marker; medium for read-only player stat offsets in the sampled record; low for named map/weapon labels and write safety.

## 4. Supported Fields
All fields are read-only in `modules/saturn-quake.rsmodule.zip`; `applyCheats` returns the original payload unchanged.

| fieldId | label | type | slot scope | safe values | read/write location | proof |
|---|---|---|---|---|---|---|
| structureStatus | Structure Status | enum | save entry | `verified` | read-only | Saturn archive filename/comment plus 1408-byte entry geometry. |
| quakeRecordCount | Quake Records | integer | save entry | `12` | read-only | Entry length decomposes into twelve 116-byte records plus padding. |
| validSlotCount | Valid Slot Markers | integer | save entry | `0..12` | read-only | Counts records with marker `0x0001` at record offset `0x6e`; fixture has `12`. |
| checksumValidSlotCount | Valid Checksums | integer | save entry | `0..12` | read-only | Counts records whose stored checksum equals the computed byte sum; fixture has `12`. |
| identicalRecordCopies | Identical Records | boolean | save entry | true/false | read-only | Direct byte comparison across the 12 records. |
| slotValidMarker | Valid Marker | integer | per slot | `0..65535` | record `+0x6e`, BE16 | Fixture marker is `1`; Lobotomy sibling source uses a trailing `short valid` in saved records. |
| checksumStatus | Checksum | enum | per slot | `valid`, `invalid`, `empty` | record `+0x72`, BE16 | Stored checksum equals sum of bytes `0x00..0x71`; fixture stores `798`. |
| checksumStored / checksumComputed | Stored/Computed Checksum | integer | per slot | `0..65535` | record `+0x72`, BE16 / computed | Proven by fixture equality and byte-sum formula. |
| levelIndexRaw | Level Index | integer | per slot | raw `0..65535` | record `+0x00`, BE16 | Exposed raw only; SaturnQuakePC confirms level-name table exists, but one sample is not enough to name every index safely. |
| difficultyRaw | Difficulty Raw | integer | per slot | raw `0..65535` | record `+0x70`, BE16 | Fixture value `0`; exposed raw only because one sample is not enough to prove named difficulty labels. |
| health | Health | integer | per slot | read observed/ranged value | record `+0x10`, BE32 | Fixture value `100`, matching Quake `SetNewParms` initial health. |
| armor | Armor | integer | per slot | read observed/ranged value | record `+0x14`, BE32 | Fixture value `0`, matching Quake new-game armor. Armor type and writes require a nonzero armor sample. |
| currentAmmo | Current Ammo | integer | per slot | read observed/ranged value | record `+0x24`, BE32 | Fixture value `25`, matching Quake's initial/minimum shell/current ammo. |
| inventoryWeaponMask | Inventory/Weapon Mask | integer | per slot | raw `0..65535` | record `+0x02`, BE16 | Fixture mask `0x0300`; Quake starts with two weapons, but bit names are intentionally left numbered. |
| weaponBits | Weapon Bits | bitmask | per slot | numbered bits | derived from `inventoryWeaponMask` | Avoids unproven weapon-name labels while still exposing weapon/inventory state. |
| monsterTotal / secretTotal | Monster/Secret Total | integer | per slot | raw `0..65535` | record `+0x40` / `+0x42`, BE16 | Fixture values `46` and `4`; SaturnQuakePC level data/text and Quake intermission stats corroborate these as progress/display counters. |
| progressByteCount / progressNonDefaultBytes / progressChecksum | Progress Bytes | integer | per slot | byte counts/sum | record `+0x4c..+0x6d` | Exposes the per-slot progress block structurally without guessing individual bit meanings. |
| safeWritableFields | Safe Writable Fields | integer | save entry | `0` | read-only | Gameplay writes are disabled until before/after saves prove every edited offset and the game accepts repaired records. |

## 5. Presets
- none

## 6. Required Backend Logic
- existing editor can be reused: no
- new parser/editor needed: yes, delivered as WASM module `modules/saturn-quake.rsmodule.zip`
- checksum repair required: known formula, but not used for writes yet
- mirrored-write required: unknown; records are reported as identical when they are identical, not repaired as mirrors
- backend module runtime note: real MiSTer Saturn backup RAM payloads exceed the old 1 MiB WASM memory cap once JSON/base64 encoded, so the sandbox memory limit was raised while retaining zip/file/output limits.

## 7. Verification
- module import test validates the repository module and inspects the real Quake fixture.
- `readCheats` exposes twelve slot groups with health, ammo, armor, inventory/weapon mask, raw level/difficulty, progress counters, validity marker, and checksum status.
- `applyCheats` returns the original payload unchanged; no gameplay edit is available.

## 8. Not Supported Yet
- write support for health, armor, ammo, weapons, inventory, level, difficulty, or progress.
- named weapon labels; only numbered weapon bits are shown.
- ammo pool breakdown by shells/nails/rockets/cells beyond the verified current ammo word.
- armor type/class.
- named level mapping or level switching.
- repair from mirrored records.
- PAL/Japanese layout claims.

## 9. Decision
- code-ready / module-backed, read-only semantic details. No cheat writes are safe yet.

## Sources
- Repository fixture: `backend/cmd/server/testdata/saturn_quake_usa.sav`
- Existing Saturn container parser/tests: `backend/cmd/server/saturn_backup_ram.go`, `backend/cmd/server/saturn_backup_ram_test.go`
- GameFAQs Saturn FAQ memory-manager example listing `Loboquake` / `Data`: https://gamefaqs.gamespot.com/saturn/916393-sega-saturn/faqs/16467
- SaturnQuakePC extracted Saturn Quake strings/assets: https://github.com/vfig/SaturnQuakePC
- id Software Quake source for baseline player stats and skill/item constants: https://github.com/id-Software/Quake
- Lobotomy SlaveDriver source for Saturn BUP save-record pattern: https://github.com/Lobotomy-Software/SlaveDriver-Engine
- Retro Reversing Saturn save-data overview and BUP notes: https://www.retroreversing.com/sega-saturn-save-data
