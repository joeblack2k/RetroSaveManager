# Cheat Intake: Metal Slug 5

## 1. Identity
- systemSlug: `neogeo`
- canonicalTitle: `Metal Slug 5`
- regions: MVS `mslug5`, identified by MAME as `Metal Slug 5 (NGM-2680)`
- extensions: `.sav`, `.srm`, `.ram`, backend-normalized `sram`
- runtimes: raw MVS backup RAM, RetroSaveManager NeoGeo compound save
- saveModel: 64KiB MVS backup RAM; compound saves preserve an additional 8KiB memory-card tail

## 2. Samples
| file | size | sha256 | runtime | notes |
|---|---:|---|---|---|
| none | n/a | n/a | n/a | No public Metal Slug 5 save payload was used. Support is based on primary emulator/source documentation and requires structural evidence inside the uploaded save. |

## 3. Format Evidence
- container/header: NeoGeoDev documents MVS backup RAM as a 64KiB region at `$D00000-$D0FFFF`; the BIOS marker starts at `$D00010` with `BACKUP RAM OK!`.
- slot layout: NeoGeoDev documents eight slot table entries starting at `$D00124`; each entry stores the slot NGH number and backup-RAM game block id.
- title proof: MAME identifies `mslug5` as `Metal Slug 5 (NGM-2680)` and marks it save-capable; the module requires a slot table NGH word of `0x0268`, which is how the MVS backup RAM table stores this title.
- byte order: MiSTer/NeoGeo compound saves may store the 64KiB backup RAM window as 16-bit byte-swapped words. The module accepts both direct and byte-swapped marker/table layouts and preserves the original layout on writes.
- marker variant: live MiSTer/compound samples use the common spaced marker `BACKUP RAM OK !`; the module accepts that and `BACKUP RAM OK!`.
- game data: NeoGeoDev documents per-game soft DIP bytes at `$D00220-$D0029F`, game-name blocks at `$D002A0-$D0031F`, and 0x1000-byte game data blocks after `$D00320`.
- checksum/crc: no global checksum is documented for the MVS backup RAM cabinet setting edited here.
- mirrored data: none for the supported cabinet byte. Compound saves preserve the trailing memory-card area unchanged.
- parser/doc sources:
  - https://wiki.neogeodev.org/index.php?title=Backup_RAM
  - https://wiki.neogeodev.org/index.php?title=Software_DIPs
  - https://wiki.neogeodev.org/index.php?title=68k_program_header
  - https://wiki.neogeodev.org/index.php?title=Memory_card
  - https://github.com/mamedev/mame/blob/master/src/mame/neogeo/neogeo.cpp
- confidence: high for the MVS backup RAM container, Metal Slug 5 NGH validation, and the cabinet free-play byte; insufficient for game-specific soft-DIP labels/bounds or high-score semantics.

## 4. Editable Fields
| fieldId | label | type | slot scope | safe values | read/write location | proof |
|---|---|---|---|---|---|---|
| `freePlay` | Free Play | boolean | whole backup RAM | `false` writes `0x00`, `true` writes `0x01` | relative offset `0x42` in the 64KiB backup RAM window | NeoGeoDev documents `$D00042` as game select: `1=Free`, `0=Only when credited`. |

## 5. Presets
- preset id: `enableFreePlay`; label: `Enable Free Play`; updates: `freePlay: true`
- preset id: `disableFreePlay`; label: `Disable Free Play`; updates: `freePlay: false`

## 6. Required Backend Logic
- existing editor can be reused: no
- new parser/editor needed: yes, delivered as `modules/neogeo-mslug5.rsmodule.zip`
- checksum repair required: no documented global checksum for the supported byte
- mirrored-write required: no; compound save memory-card data is preserved byte-for-byte

## 7. Verification
- before/after checks: module import test validates the zip, manifest, WASM ABI, and bundled YAML.
- in-game confirmation: not performed.
- parser/tool confirmation: parser requires payload size `65536` or `73728`, the MVS backup RAM marker, and a slot table entry for NGH `0x0268`.

## 8. Open Questions
- Metal Slug 5-specific soft-DIP labels and bounds remain unsupported until derived from the real program header or confirmed documentation.
- High-score, ranking, and bookkeeping fields remain unsupported.
- Memory-card game data remains unsupported.
- Card-only 8KiB saves are not supported because they do not contain the MVS backup RAM slot table needed to prove `mslug5`.

## 9. Decision
- module-backed parser and cheat pack
- safe support limited to documented MVS backup RAM `Free Play`
- no standalone YAML-only pack because no existing backend editor can validate or edit NeoGeo backup RAM semantically
