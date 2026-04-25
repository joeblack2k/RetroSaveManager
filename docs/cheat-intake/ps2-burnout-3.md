# Cheat Intake: Burnout 3: Takedown

## 1. Identity
- systemSlug: `ps2`
- canonicalTitle: `Burnout 3: Takedown`
- regions: NTSC-U confirmed from `BASLUS-21050` save directory and `SLUS-21050` disc serial references
- extensions: `.ps2` memory card image for source extraction; `.zip` RSM logical archive and `.bin`/`.dat` raw primary payload for the read-only module
- runtimes: PCSX2 memory card source sample
- saveModel: PS2 memory card directory containing `icon.sys`, `view.ico`, and primary game payload `BASLUS-21050`

## 2. Samples
| file | size | sha256 | runtime | notes |
|---|---:|---|---|---|
| PCSX2 Save File Collection Memory Card 7.zip | 863116 | `01dbcfeb863bf589698f07a8970f5998d4fb6f44788a77cc47d775b96faf805b` | PCSX2 | Public HippyJ3/Google Drive archive; README lists Burnout 3 as "Everything 100% Completed and Unlocked - NTSC UC". |
| PCSX2 Save File Collection Memory Card 7.ps2 | 8650752 | `00c6eb317cd0cb61d16fd9ff450cf81042ad66c8cb122c8df1cee2ae2f930610` | PCSX2 | Full memory card image extracted from the public archive. |
| BASLUS-21050/icon.sys | 964 | `d9adcf628851464ae6e6df0a811dd6500909d5284c5794b965cd3e7207acd638` | PS2 logical save | `PS2D` icon metadata; Shift-JIS title bytes decode to full-width "Burnout 3". |
| BASLUS-21050/view.ico | 15234 | `4904fb2c7195a2319b1c2040bc4c0ea7900169f362237f6babd3d82790542868` | PS2 logical save | Icon payload referenced by `icon.sys`. |
| BASLUS-21050/BASLUS-21050 | 61772 | `1c0e3cd716f9eb347271bda34dd1c7c4f0c2f0235da5cde49cdb0df622d6a1d6` | PS2 logical save | Primary Burnout 3 payload; no obvious ASCII magic string. CRC32 `f6a0358b`. |
| BASLUS-21050 logical ZIP | 8333 | `0f894a8782daa9b5836acf0448002ef4e5a8aa10678283e1b0617325098849bd` | derived RSM-style logical archive | Contains the three files above; used for module parser verification. |

## 3. Format Evidence
- container/header: Full card begins with `Sony PS2 Memory Card Format 1.1.0.0`, matching the PS2 memory-card image format documented by PS2 homebrew references and already parsed by RetroSaveManager's PS2 projection reader.
- slot layout: Root directory contains a `BASLUS-21050` directory. Its children are exactly `icon.sys` (964 bytes), `view.ico` (15234 bytes), and `BASLUS-21050` (61772 bytes).
- checksum/crc: The module validates ZIP central-directory CRC32 values for the derived logical archive and CRC32 for the raw primary payload. This proves sample identity only; it is not a proven in-game checksum repair algorithm.
- mirrored data: Not proven. The primary payload has repeated-looking binary structures, but no safe mirror or slot model was established.
- parser/doc sources:
  - HippyJ3 PCSX2 Save File Collection Memory Card 7 page and linked public Google Drive sample.
  - GameFAQs Burnout 3: Takedown save listing metadata for public PS2 saves.
  - PS2 memory-card/icon.sys format references, plus RetroSaveManager's existing PS2 card extractor behavior.
- confidence: High for NTSC-U container identity and exact public sample recognition. Low for gameplay field semantics and write safety.

## 4. Editable Fields
| fieldId | label | type | slot scope | safe values | read/write location | proof |
|---|---|---|---|---|---|---|
| none | none | none | none | none | none | No Burnout 3 field map or checksum repair routine was proven. |

## 5. Presets
- none

## 6. Required Backend Logic
- existing editor can be reused: no
- new parser/editor needed: yes; implemented as `modules/burnout-3.rsmodule.zip`
- checksum repair required: unknown, so no writes are exposed
- mirrored-write required: unknown
- module behavior: read-only structural parser for the verified NTSC-U public sample as an RSM logical ZIP or exact raw primary payload; bundled cheat pack has an empty details section and no editable fields.

## 7. Verification
- Extracted the Burnout 3 logical directory from the public PCSX2 card image and verified file sizes/hashes.
- Confirmed module import via `TestRepositoryGameModulesImport`.
- Ran a temporary local parser sanity check against the derived logical ZIP and raw primary payload before removing the temporary test.
- No in-game write verification was performed because no writes are supported.

## 8. Open Questions
- PAL/Japanese directory names and file sizes were not verified.
- The primary `BASLUS-21050` payload has no obvious text magic; internal gameplay fields such as cars, tracks, medals, crashes, money, trophies, or completion percentage remain unmapped.
- Any in-game checksum, compression, encryption, or integrity footer remains unproven.

## 9. Decision
- read-only module-backed support
- no cheat writes
- safe details only for exact NTSC-U public sample forms
