# Cheat Intake: New Super Mario Bros.

## 1. Identity
- systemSlug: `nds`
- canonicalTitle: `New Super Mario Bros.`
- regions: USA, Europe, Japan aliases accepted by title only; format evidence is not region-specific.
- extensions: `.sav`, `.dsv` when the payload is a raw 8192-byte backup memory image.
- runtimes: Nintendo DS raw backup saves; DeSmuME exported backup memory is compatible with this raw shape.
- saveModel: 8192-byte save split into two 0x1000-byte banks. Each bank has a header block, three save-file blocks, and a footer block.

## 2. Samples
| file | size | sha256 | runtime | notes |
|---|---:|---|---|---|
| not redistributed | 8192 | not recorded | public editor/source evidence | No private or bundled user save sample was added. The module is based on public save-editor source and the existing project fixture shape for `Mario2d` NSMBDS saves. |

## 3. Format Evidence
- container/header: raw 8192-byte NDS backup. Known save-block signature is ASCII `Mario2d`, stored at block offset `+0x02`.
- slot layout: two 0x1000-byte banks. In each bank, blocks are at `0x000` header, `0x100` File 1, `0x380` File 2, `0x600` File 3, and `0x880` footer.
- checksum/crc: public editor code initializes a 16-bit checksum to `654`, folds each byte from block offset `+0x0A` for block-specific lengths, and stores the result little-endian at block offset `+0x00`.
- mirrored data: the module edits the selected save-file block in both 0x1000-byte banks, then recalculates all ten block checksums.
- parser/doc sources:
  - https://github.com/newluigidev/New-Super-Mario-Bros-Save-Editor
  - https://github.com/newluigidev/New-Super-Mario-Bros-Save-Editor/blob/master/NewSuperMarioBrosSaveEditor/Form1.cs
  - https://nsmbhd.net/thread/4220-save-editor-tool/
- confidence: high for size, signatures, banks, checksum algorithm, slot block offsets, and the supported field offsets/bounds below. Medium for treating both banks as mirrors, but this is the safest write policy because both banks are checksummed copies of the same block layout.

## 4. Editable Fields
| fieldId | label | type | slot scope | safe values | read/write location | proof |
|---|---|---|---|---|---|---|
| `lives` | Lives | integer | File 1/2/3 | `0..99` | file block `+0x16`, little-endian u32 | public editor reads/writes this offset and UI max is 99 |
| `coins` | Coins | integer | File 1/2/3 | `0..99` | file block `+0x1A`, little-endian u32 | public editor reads/writes this offset and UI max is 99 |
| `starCoins` | Star Coins | integer | File 1/2/3 | `0..240` | file block `+0x22`, little-endian u32 | public editor reads/writes this offset and UI max is 240 |
| `score` | Score | integer | File 1/2/3 | `0..9999950` | file block `+0x1E`, little-endian u32 | public editor reads/writes this offset and UI max is 9999950 |
| `currentPowerup` | Current Power-Up | enum | File 1/2/3 | small, super, fire, mini, blueShell | file block `+0x3A`, byte | public editor maps stored values `0,1,2,4,5` to labels |
| `reserveItem` | Reserve Item | enum | File 1/2/3 | none, superMushroom, fireFlower, blueShell, miniMushroom, megaMushroom | file block `+0x66`, byte | public editor maps stored values `0..5` to labels |
| `bottomScreenBackground` | Bottom Screen Background | enum | File 1/2/3 | bg1..bg5 | file block `+0x42`, byte | public editor maps stored values `0..4` to UI values 1..5 |

## 5. Presets
- `maxCounters`: lives 99, coins 99, star coins 240, score 9999950.
- `fireAndMegaReserve`: current power-up Fire, reserve item Mega Mushroom.

## 6. Required Backend Logic
- existing editor can be reused: no. The backend already has structural NSMBDS detection, but no DS cheat editor.
- new parser/editor needed: yes, delivered as `modules/nds-new-super-mario-bros.rsmodule.zip`.
- checksum repair required: yes, all header/save/footer block checksums are recalculated after edits.
- mirrored-write required: yes, selected save-file values are written in both 0x1000-byte banks.

## 7. Verification
- before/after checks: module parser requires raw size 8192, repeated `Mario2d` signatures, and valid checksums in both banks before writes.
- in-game confirmation: not performed in this pass; no private ROM/runtime sample was used.
- parser/tool confirmation: public editor source confirms checksum routine, block offsets, field offsets, UI labels, and UI bounds.

## 8. Open Questions
- The public editor has bulk "Unlock all Worlds" and "Unlock all Levels" buttons, but the individual bit semantics and per-level mapping were not documented enough for safe field support.
- DeSmuME native `.dsv` files with emulator metadata/footer are not supported unless the uploaded payload is exactly the raw 8192-byte backup memory.
- Region-specific differences were not found in the public editor source; the module therefore relies on structural validation instead of region-specific ROM hashes.

## 9. Decision
- code-ready / module delivered. No standalone YAML-only pack was added because DS/NSMB requires parser-backed checksum validation and repair.
