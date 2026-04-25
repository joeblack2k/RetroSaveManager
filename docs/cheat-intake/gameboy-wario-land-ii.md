# Cheat Intake: Wario Land II

## 1. Identity
- systemSlug: gameboy
- canonicalTitle: Wario Land II
- regions: Europe, USA, Japan samples in source dataset
- extensions: sav, srm, ram, sram
- runtimes: raw Game Boy SRAM exports
- saveModel: 32 KiB SRAM with rotating 0x200-byte save sections and header checksums

## 2. Samples
| file | size | sha256 | runtime | notes |
|---|---:|---|---|---|
| game-tools-collection gb-europeusa.sav | 32768 | 5ca21ec2e48c859974a9864891d9ac94f7633591d0629dd3bef2476f9f4795f0 | raw SRAM | GB Europe/USA sample |
| game-tools-collection gbc-gbc-europeusa.sav | 32768 | f76c6017c2f82723003a934fc2d79f5c4a66df6486b7a68de6e5957f344e22d4 | raw SRAM | GBC Europe/USA sample |
| game-tools-collection gbc-gbc-japan.sav | 32768 | 8be4f22f15a6c99e51f57a7659126dde06932f42ca1d7c54a66c270e6b0d2d5c | raw SRAM | GBC Japan sample |

## 3. Format Evidence
- container/header: Data Crystal documents Wario Land II as 32 KiB SRAM.
- slot layout: six candidate save sections at `0x400 + section*0x200`; active section is the highest non-erased save count at section offset `+0x04`.
- checksum/crc: game-tools utility sums bytes from active section `+0x04` through `+0x36` or `+0x15c` depending on section, stored as big-endian uint16 at header offsets `0x01,0x05,0x09,0x03,0x07,0x0b`.
- mirrored data: writes are mirrored to every valid section with the same active save count.
- parser/doc sources: Data Crystal Wario Land II; RyudoSynbios game-tools-collection Wario Land II template, checksum utility, and public test saves.
- confidence: high for BCD counters and checksum repair; lower for progression/flag mutation, so those are not exposed.

Sources:
- https://datacrystal.tcrf.net/wiki/Wario_Land_II
- https://github.com/RyudoSynbios/game-tools-collection/blob/master/src/lib/templates/wario-land-ii/saveEditor/template.ts
- https://github.com/RyudoSynbios/game-tools-collection/blob/master/src/lib/templates/wario-land-ii/saveEditor/utils.ts

## 4. Editable Fields
| fieldId | label | type | slot scope | safe values | read/write location | proof |
|---|---|---|---|---|---|---|
| stageCoins | Stage Coins | integer | active mirrored section | 0-999 | active section `+0x12`, 2-byte big-endian BCD | game-tools template |
| totalCoins | Total Coins | integer | active mirrored section | 0-99999 | active section `+0x0f`, 3-byte big-endian BCD | game-tools template |
| flagmanDDHighScore | Flagman D.D Hi-Score | integer | active mirrored section | 0-9999 | active section `+0x35`, 2-byte big-endian BCD | game-tools template |

## 5. Presets
- maxCoins: maxes stage and total coin counters
- maxFlagmanScore: maxes the Flagman D.D high score

## 6. Required Backend Logic
- existing editor can be reused: no
- new parser/editor needed: yes, supplied as `gameboy-wario-land-ii.rsmodule.zip`
- checksum repair required: yes, per patched section
- mirrored-write required: yes, to valid sections with matching save count

## 7. Verification
- before/after checks: module import and checksum validation through repository module import test
- in-game confirmation: not performed in this pass
- parser/tool confirmation: offsets and checksums cross-checked against game-tools-collection samples

## 8. Open Questions
- Progression flags, treasures, picture panels, hidden events, and clear percentages are deliberately not writable yet.

## 9. Decision
- code-ready / module-backed write support for documented BCD counters only
