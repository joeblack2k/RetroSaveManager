# Cheat Intake: Donkey Kong Country (Game Boy Color)

## 1. Identity
- systemSlug: gameboy
- canonicalTitle: Donkey Kong Country (Game Boy Color)
- regions: USA/EUR/JPN cartridge docs reviewed
- extensions: sav, srm, ram, sram
- runtimes: raw Game Boy SRAM exports
- saveModel: 8 KiB SRAM, semantic layout not verified

## 2. Samples
| file | size | sha256 | runtime | notes |
|---|---:|---|---|---|
| none accepted | - | - | - | no verified raw GBC SRAM sample with semantic field proof found in this pass |

## 3. Format Evidence
- container/header: Data Crystal documents 8 KiB SRAM for Donkey Kong Country on Game Boy Color.
- slot layout: not verified for the GBC version.
- checksum/crc: not verified for the GBC version.
- mirrored data: not verified for the GBC version.
- parser/doc sources: Data Crystal cartridge metadata; Game Boy hardware database cartridge record confirms battery-backed SRAM hardware for CGB-BDDE-0.
- confidence: structural-only for non-blank 8 KiB SRAM; no semantic write confidence.

Sources:
- https://datacrystal.tcrf.net/wiki/Donkey_Kong_Country_(Game_Boy_Color)
- https://gbhwdb.gekkio.fi/cartridges/CGB-BDDE-0

## 4. Editable Fields
| fieldId | label | type | slot scope | safe values | read/write location | proof |
|---|---|---|---|---|---|---|
| detailsOnly | Read-Only Details | enum | whole payload | keep | no mutation | inert field only, used to expose structural details without writes |

## 5. Presets
- none

## 6. Required Backend Logic
- existing editor can be reused: no; the existing DKC editor is SNES SRAM only
- new parser/editor needed: yes, supplied as `gameboy-donkey-kong-country-gbc.rsmodule.zip`
- checksum repair required: no, because no writes are performed
- mirrored-write required: no, because no writes are performed

## 7. Verification
- before/after checks: module import and read-only unchanged-payload apply path
- in-game confirmation: not performed
- parser/tool confirmation: structural size based on cartridge documentation only

## 8. Open Questions
- Need raw SRAM samples or reverse-engineered GBC save code before exposing progress, completion, lives, bananas, or other fields.

## 9. Decision
- research-only / module-backed read-only structural support
