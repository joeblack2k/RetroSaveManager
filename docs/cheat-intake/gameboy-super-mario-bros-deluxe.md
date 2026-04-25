# Cheat Intake: Super Mario Bros. Deluxe

## 1. Identity
- systemSlug: gameboy
- canonicalTitle: Super Mario Bros. Deluxe
- regions: USA/EUR/JPN cartridge docs reviewed
- extensions: sav, srm, ram, sram
- runtimes: raw Game Boy SRAM exports
- saveModel: 8 KiB SRAM, semantic layout not verified

## 2. Samples
| file | size | sha256 | runtime | notes |
|---|---:|---|---|---|
| none accepted | - | - | - | Zophar files inspected were emulator savestates, not raw SRAM, so they were not used as field proof |

## 3. Format Evidence
- container/header: Data Crystal documents 8 KiB SRAM for Super Mario Bros. Deluxe.
- slot layout: not verified.
- checksum/crc: not verified.
- mirrored data: not verified.
- parser/doc sources: Data Crystal cartridge metadata.
- confidence: structural-only for non-blank 8 KiB SRAM; no semantic write confidence.

Source:
- https://datacrystal.tcrf.net/wiki/Super_Mario_Bros._Deluxe

## 4. Editable Fields
| fieldId | label | type | slot scope | safe values | read/write location | proof |
|---|---|---|---|---|---|---|
| detailsOnly | Read-Only Details | enum | whole payload | keep | no mutation | inert field only, used to expose structural details without writes |

## 5. Presets
- none

## 6. Required Backend Logic
- existing editor can be reused: no
- new parser/editor needed: yes, supplied as `gameboy-super-mario-bros-deluxe.rsmodule.zip`
- checksum repair required: no, because no writes are performed
- mirrored-write required: no, because no writes are performed

## 7. Verification
- before/after checks: module import and read-only unchanged-payload apply path
- in-game confirmation: not performed
- parser/tool confirmation: structural size based on cartridge documentation only

## 8. Open Questions
- Need real raw SRAM samples or disassembly-backed save map before exposing progress, unlocks, high scores, or checksums.

## 9. Decision
- research-only / module-backed read-only structural support
