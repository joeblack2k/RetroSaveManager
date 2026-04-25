# Cheat Intake: Yoshi's Story

## 1. Identity
- systemSlug: `n64`
- canonicalTitle: `Yoshi's Story`
- regions: USA sample verified; title aliases include USA/Europe/Japan for matching, but no region-specific gameplay fields are exposed.
- extensions: canonical `.eep`
- runtimes: Project64 raw EEPROM; RetroArch `.srm` sample starts with the same canonical 2048-byte EEPROM payload.
- saveModel: 16Kbit EEPROM, 2048 bytes total, stored as two 0x400-byte `EepBuffer` copies.

## 2. Samples
| file | size | sha256 | runtime | notes |
|---|---:|---|---|---|
| `Yoshi's Story (U) (M2) [!].eep` | 2048 | `711940b90005adc9724c246fe92608238ddcd880546679d2dbb98436ec64640b` | Project64 | Real EEPROM sample from EmuNations; both 0x400-byte copies are byte-identical. |
| `Yoshi's Story.srm` | 296960 | `c987a59f457bd415dcf061cb267d9bc8e261225f4fa619c325c267111549d905` | RetroArch | Real RetroArch N64 SRM sample from EmuNations; first 2048 bytes match the Project64 EEPROM sample exactly. |
| Project64 ZIP | 229 | `c501d8a15d2f446056ab76f098760652cae675918eaae3f2a23ce18b73751a98` | Project64 | Container downloaded from the EmuNations Yoshi's Story save page. |
| RetroArch ZIP | 2152 | `ad7cc70fdd744f2e79b6d4ba24fd5c076977cb51882e791f7ab9ece98ec4bb76` | RetroArch | Container downloaded from the EmuNations Yoshi's Story save page. |

## 3. Format Evidence
- container/header: public decomp source defines `EepBuffer` as `data[0x3FA]`, `u16 unk3FA` at `0x3FA`, and magic `0x81317531` at `0x3FC`.
- slot layout: two mirrored 0x400-byte `EepBuffer` copies fill the 2048-byte EEPROM.
- checksum/crc: decomp source proves a 16-bit check at `0x3FA` via `func_800699BC`, but that function is not decompiled in the reviewed source. The sample stores `0x9EB0` in both copies.
- mirrored data: save routine writes copy 1, assigns checksum and magic, then clones it to copy 2; load routine checks copy 1 first and falls back to copy 2.
- parser/doc sources:
  - https://github.com/decompals/yoshis-story/blob/main/include/eepmgr.h
  - https://github.com/decompals/yoshis-story/blob/main/src/main/O2/65520.c
  - https://www.emunations.com/gamesaves/nintendo/n64/yoshi%27s-story
- confidence: high for container layout and mirrored-copy behavior; insufficient for checksum rebuild or gameplay semantics.

## 4. Editable Fields
| fieldId | label | type | slot scope | safe values | read/write location | proof |
|---|---|---|---|---|---|---|
| installVerified100Percent | Install Verified 100% Save | boolean command | whole EEPROM | `true` installs the verified template | replaces the full 2048-byte EEPROM payload with the verified Project64 sample | EmuNations describes the sample as 100% complete, black and white Yoshis unlocked for Story Mode, and all stages available for Trial Mode; the sample has valid mirrored 0x400-byte EepBuffer copies, stored checksum `0x9EB0`, and magic `0x81317531`. |

## 5. Presets
- preset id: `installVerified100Percent`
- label: `Install verified 100% save`
- updates: `installVerified100Percent: true`

## 6. Required Backend Logic
- existing editor can be reused: no
- new parser/editor needed: yes, delivered as `modules/n64-yoshis-story.rsmodule.zip`
- checksum repair required: avoided for the supported write by replacing the full payload with the verified checksummed EEPROM sample
- mirrored-write required: avoided for the supported write by replacing the full payload with the verified mirrored EEPROM sample

## 7. Verification
- before/after checks: module import, parse, readCheats, and applyCheats tested through the backend module loader with the real Project64 EEPROM sample.
- in-game confirmation: not performed.
- parser/tool confirmation: module reports structural fields plus whether the current payload already matches the verified 100% template.

## 8. Open Questions
- Decompile or otherwise verify `func_800699BC`.
- Map the 0x3FA gameplay data bytes to real game progress, options, scores, trial-mode unlocks, melon/fruit state, or high-score state before exposing any editable fields.
- Confirm whether all regional ROMs use the same EEPROM structure and checksum.
- Public RAM cheat lists identify live-memory trial/fruit/current-level addresses, but those are not direct EEPROM offsets and are not used as raw save edits.

## 9. Decision
- module-backed parser and cheat pack
- safe whole-save 100% template write supported
- granular per-field gameplay edits deferred until the transform/checksum and individual EEPROM semantics are proven
