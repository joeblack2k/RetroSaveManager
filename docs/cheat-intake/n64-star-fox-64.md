# Cheat Intake: Star Fox 64

## 1. Identity
- systemSlug: n64
- canonicalTitle: Star Fox 64
- alternateTitle: Lylat Wars
- extensions: .eep
- runtimes: N64 raw/canonical EEPROM, including 512-byte and 2048-byte normalized payloads
- saveModel: 512-byte EEPROM with one primary 0x100-byte save block and one backup 0x100-byte save block

## 2. Samples
| file | size | sha256 | runtime | notes |
|---|---:|---|---|---|
| synthetic Star Fox 64 fixture | 512 | generated in Go tests | parser fixture | Primary and backup blocks contain valid Star Fox 64 checksums |

## 3. Format Evidence
- container/header: raw EEPROM, 0x200 bytes total.
- mirrored data: `SaveFile` contains `save` at 0x000 and `backup` at 0x100; each `Save` is 0x100 bytes.
- save data: `SaveData` is 0xFE bytes followed by a big-endian `u16 checksum` at 0xFE.
- checksum: iterate bytes 0x00 through 0xFD, XOR the byte into a 16-bit accumulator, shift left once, fold bit 8 into bit 0 with `(checksum & 0xFE) | ((checksum >> 8) & 1)`, then store `(checksum & 0xFF) | 0x9500`.
- parser/doc sources: `sf64` decomp `include/sf64save.h` for `SaveData`, `Save`, `SaveFile`, and `PlanetData`; `src/engine/fox_save.c` for `Save_Checksum`, `Save_Read`, and `Save_Write`.
- confidence: high for checksum repair, primary/backup mirroring, audio settings, and planet clear/medal flags.

## 4. Editable Fields
| fieldId | label | type | safe values | read/write location | proof |
|---|---|---|---|---|---|
| soundMode | Sound Mode | enum | stereo, mono, headphones | SaveData + 0x14 | `fox_option.c` reads/writes `gSaveFile.save.data.soundMode`; option enum has 0 stereo, 1 mono, 2 headset |
| musicVolume | Music Volume | integer | 0-99 | SaveData + 0x15 | `fox_option.c` clamps and writes music volume to save data |
| voiceVolume | Voice Volume | integer | 0-99 | SaveData + 0x16 | `fox_option.c` clamps and writes voice volume to save data |
| sfxVolume | SFX Volume | integer | 0-99 | SaveData + 0x17 | `fox_option.c` clamps and writes SFX volume to save data |
| playedPlanets | Played Planets | bitmask | 16 known planet slots | SaveData + 0x00..0x0F, mask 0x04 | `PlanetData.played`; map/HUD code sets it when a route node is reached |
| normalClearPlanets | Normal Clear Planets | bitmask | 16 known planet slots | SaveData + 0x00..0x0F, mask 0x01 | `PlanetData.normalClear`; map/HUD and Venom ending code set it on normal clear |
| normalMedalPlanets | Normal Medal Planets | bitmask | 16 known planet slots | SaveData + 0x00..0x0F, mask 0x02 | `PlanetData.normalMedal`; option menu enables Expert mode when normal medals are present except Venom 1 |
| expertClearPlanets | Expert Clear Planets | bitmask | 16 known planet slots | SaveData + 0x00..0x0F, mask 0x08 | `PlanetData.expertClear`; versus code uses Venom 2 expert clear for on-foot unlock |
| expertMedalPlanets | Expert Medal Planets | bitmask | 16 known planet slots | SaveData + 0x00..0x0F, mask 0x10 | `PlanetData.expertMedal`; title/menu code reads expert medal state |
| venom2NormalClear | Venom 2 Normal Clear | boolean | true/false | SaveData + 0x0F, mask 0x01 | `fox_versus.c` unlocks Landmaster when `SAVE_SLOT_VENOM_2.normalClear == 1` |
| venom2ExpertClear | Venom 2 Expert Clear | boolean | true/false | SaveData + 0x0F, mask 0x08 | `fox_versus.c` unlocks on-foot when `SAVE_SLOT_VENOM_2.expertClear == 1` |

PlanetData bitfield note: the decomp comments use big-endian bit offsets. The raw byte masks are therefore `expertMedal=0x10`, `expertClear=0x08`, `played=0x04`, `normalMedal=0x02`, and `normalClear=0x01`. The `PLANET_STATS` raw-byte macro in `fox_save.c` confirms this bit numbering style.

## 5. Presets
- unlockVersusVehicles: sets Venom 2 played, normal clear, and expert clear without replacing other planet selections.
- unlockExpertMode: marks every planet played, normal-cleared, and normal-medaled.
- allNormalMedals: marks every planet played, normal-cleared, and normal-medaled.
- allExpertMedals: marks every planet played, expert-cleared, and expert-medaled.
- allMedals: applies all supported normal and expert clear/medal flags.

## 6. Required Backend Logic
- existing editor can be reused: no
- new parser/editor needed: yes, `sf64-eeprom`
- checksum repair required: yes
- mirrored-write required: yes, primary and backup blocks must be written together
- payload normalization: accept 512-byte EEPROM or the first 512 bytes of a 2048-byte canonical EEPROM

## 7. Verification
- before edits: parse primary block when checksum-valid, otherwise parse checksum-valid backup.
- after edits: recompute Star Fox 64 checksum and write identical primary and backup blocks.
- parser/tool confirmation: Go unit tests cover read/apply, backup recovery, endpoint exposure, new-save versioning, registry validation, and N64 inventory behavior.

## 8. Open Questions
- Ranking names, route records, hit counts, team status, and language bytes are intentionally not exposed yet.
- Real end-user sample hashes should be added when available, but current parser behavior is based on decomp-level format evidence and checksum verification.

## 9. Decision
- code-ready and implemented as a parser-backed EEPROM pack.
