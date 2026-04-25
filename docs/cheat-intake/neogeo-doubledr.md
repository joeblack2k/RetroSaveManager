# Cheat Intake: Double Dragon (Neo-Geo)

## 1. Identity
- systemSlug: `neogeo`
- canonicalTitle: `Double Dragon (Neo-Geo)`
- regions: MVS/AES `doubledr`, identified by MAME as `Double Dragon (Neo-Geo)` with `NGM-082` and `NGH-082`
- extensions: `.sav`, `.srm`, `.ram`, backend-normalized `sram`
- runtimes: raw MVS backup RAM, raw Neo Geo memory-card data, RetroSaveManager NeoGeo compound save
- saveModel: 64KiB MVS backup RAM; 8KiB Neo Geo memory-card data; compound saves preserve an additional 8KiB memory-card tail

## 2. Samples
| file | size | sha256 | runtime | notes |
|---|---:|---|---|---|
| none | n/a | n/a | n/a | No public Double Dragon payload was used. A local and web search did not find a usable real save sample, so support is based on primary emulator/source documentation and requires structural evidence inside the uploaded save. |

## 3. Format Evidence
- container/header: NeoGeoDev documents MVS backup RAM as a 64KiB region at `$D00000-$D0FFFF`; the BIOS marker starts at `$D00010` with `BACKUP RAM OK!`.
- slot layout: NeoGeoDev documents eight slot table entries starting at `$D00124`; each entry stores the slot NGH number and backup-RAM game block id.
- title proof: MAME identifies `doubledr` as `Double Dragon (Neo-Geo)` and marks it save-capable; the source comment lists ID `0082`, `NGM-082`, and `NGH-082`. The module requires a slot table NGH word of `0x0082`.
- memory-card proof: NeoGeoDev documents memory-card directory entries as save sub-number, NGH word, and FAT index. The module accepts card-only payloads only when Neo Geo memory-card magic is present and at least one directory entry contains NGH `0x0082`.
- byte order: MiSTer/NeoGeo compound saves may store the 64KiB backup RAM window as 16-bit byte-swapped words. The module accepts both direct and byte-swapped marker/table layouts and preserves the original layout on writes.
- marker variant: the module accepts `BACKUP RAM OK!` and the common spaced marker `BACKUP RAM OK !`, matching the existing NeoGeo Metal Slug 5 module behavior for real compound saves.
- same-console evidence: `modules/neogeo-mslug5.rsmodule.zip` already validates the same MVS backup-RAM container, slot table, word-swapped layout, and cabinet byte for Metal Slug 5 by changing only the game NGH requirement.
- game data: NeoGeoDev documents per-game soft DIP bytes at `$D00220-$D0029F`, game-name blocks at `$D002A0-$D0031F`, and 0x1000-byte game data blocks after `$D00320`.
- checksum/crc: no global checksum is documented for the MVS backup RAM cabinet setting edited here.
- mirrored data: none for the supported cabinet byte. Compound saves preserve the trailing memory-card area unchanged.
- parser/doc sources:
  - https://wiki.neogeodev.org/index.php?title=Backup_RAM
  - https://wiki.neogeodev.org/index.php?title=Software_DIPs
  - https://wiki.neogeodev.org/index.php?title=68k_program_header
  - https://wiki.neogeodev.org/index.php?title=Memory_card
  - https://github.com/mamedev/mame/blob/master/src/mame/neogeo/neogeo.cpp
  - https://gamefaqs.gamespot.com/neo/565675-double-dragon/faqs/42496
- confidence: high for the MVS backup RAM container, Double Dragon NGH validation, and the cabinet free-play byte; insufficient for Double Dragon-specific soft-DIP labels/bounds or high-score semantics.

## 4. Editable Fields
| fieldId | label | type | slot scope | safe values | read/write location | proof |
|---|---|---|---|---|---|---|
| `freePlay` | Free Play | boolean | whole backup RAM | `false` writes `0x00`, `true` writes `0x01` | relative offset `0x42` in the 64KiB backup RAM window | NeoGeoDev documents `$D00042` as game select: `1=Free`, `0=Only when credited`. |

Read-only parser details exposed in save inspection:

| fieldId | meaning | source |
|---|---|---|
| `softDipBytesHex` / `softDipBytesNonPadding` | 16-byte soft-DIP storage block for the validated MVS slot, read-only | NeoGeoDev backup RAM `$D00220-$D0029F`; exact Double Dragon byte-to-label mapping is not proven. |
| `gameDataBlockIndex` / `gameDataBlockOffset` / `gameDataBlockNonPaddingBytes` | selected 0x1000-byte backup-RAM game-data block occupancy, read-only | slot table block id plus NeoGeoDev game-data block layout. |
| `bookkeepingSlotIndex` / `slotBookDateBcd` | cabinet bookkeeping slot/date bytes, read-only | NeoGeoDev backup RAM table. |
| `memoryCardUsedEntries` / `doubleDragonCardEntries` / `cardFirstDoubleDragonSubNumber` / `cardFirstDoubleDragonFatIndex` | memory-card directory summary, read-only for card-only and compound card tails | NeoGeoDev memory-card directory layout. |

## 5. Presets
- preset id: `enableFreePlay`; label: `Enable Free Play`; updates: `freePlay: true`
- preset id: `disableFreePlay`; label: `Disable Free Play`; updates: `freePlay: false`

## 6. Required Backend Logic
- existing editor can be reused: no
- new parser/editor needed: yes, delivered as `modules/neogeo-doubledr.rsmodule.zip`
- checksum repair required: no documented global checksum for the supported byte
- mirrored-write required: no; compound save memory-card data is preserved byte-for-byte
- card-only write support: no; card-only payloads are read-only details because memory-card data/FAT edits and Double Dragon-specific card contents are not proven safe

## 7. Verification
- before/after checks: module import test validates the zip, manifest, WASM ABI, and bundled YAML.
- in-game confirmation: not performed.
- parser/tool confirmation: parser accepts payload size `65536` or `73728` for backup-RAM support, requiring the MVS backup RAM marker and a slot table entry for NGH `0x0082`; parser accepts payload size `8192` for read-only card-only support, requiring Neo Geo memory-card magic and a Double Dragon NGH directory entry.

## 8. Open Questions
- Double Dragon-specific soft-DIP labels and writable bounds remain unsupported until derived from the real program header or confirmed documentation. A GameFAQs arcade-menu reference lists user-facing settings such as rounds/time/difficulty/continue-related options, but it does not prove the backup-RAM byte mapping.
- High-score, ranking, and bookkeeping edits remain unsupported; block occupancy and documented bookkeeping bytes are read-only.
- Memory-card game data edits remain unsupported; directory details and card-only identification are read-only.

## 9. Decision
- module-backed parser and cheat pack
- safe write support limited to documented MVS backup RAM `Free Play`
- safe read-only support added for soft-DIP storage bytes, game-data block occupancy, bookkeeping bytes, memory-card directory details, and card-only identification
- no standalone YAML-only pack because no existing backend editor can validate or edit NeoGeo backup RAM semantically
