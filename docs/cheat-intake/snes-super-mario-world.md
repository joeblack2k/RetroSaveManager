# Cheat Intake: Super Mario World

## 1. Identity
- systemSlug: snes
- canonicalTitle: Super Mario World
- regions: USA/EUR/JPN retail layout expected for standard cartridge SRAM
- extensions: .srm, .sav, .sram, .sa1
- runtimes: Snes9x, RetroArch Snes9x, bsnes/higan raw SNES SRAM
- saveModel: 2048-byte raw cartridge SRAM; the game uses 858 bytes for three save files and three backup copies

## 2. Samples
| file | size | sha256 | runtime | notes |
|---|---:|---|---|---|
| EmuNations Super Mario World.srm | 2048 | 6e04630e4800b14bfa7fa2a6921b7c515c2f1751a1d4ea65945b1d0b20696dc1 | SNES SRAM | File A has 96 events, File B has 95 events, File C is blank |
| RetroMaggedon Vanilla Dome SRM | 2048 | 6709d6041d187f79d3e5a75e5921a6fee72840da9a38a35afc9e1f5b55580fd8 | SNES9X / RetroArch Linux | File A valid, event count 30 |
| RetroMaggedon Chocolate Island SRM | 2048 | 9ddadf26fcc4a0fa4acf30f054e6bf73349df2cc79adeee059405444bee9a491 | SNES9X / RetroArch Linux | File A valid, event count 49 |
| RetroMaggedon Valley of Bowser SRM | 2048 | 68c5d7e84feab60337657df9154261f494979b06e4e2946c903a017ad793158d | SNES9X / RetroArch Linux | File A valid, event count 66 |

## 3. Format Evidence
- container/header: raw 2048-byte SNES cartridge SRAM. There is no game magic string, so matching must combine title/ROM metadata with structural checks.
- slot layout: File A is 0x0000-0x008e, File B is 0x008f-0x011d, File C is 0x011e-0x01ac. Backup copies are at 0x01ad, 0x023c, and 0x02cb.
- checksum/crc: each 143-byte file stores 141 data bytes followed by a little-endian checksum complement. The save routine sums the 141 data bytes and stores `0x5A5A - sum`.
- mirrored data: the save routine duplicates the edited file to the corresponding backup block by adding 0x01ad to the primary SRAM index.
- parser/doc sources: SMWCentral SRAM map by Lui37; SnesLab SMW save routine by p4plus2; SMWCentral RAM map entries for `$7E:1EA2`, `$7E:1F02`, `$7E:1F11`-`$7E:1F27`, and `$7E:1F2E`; SMWDisX `bank_00.asm` and `constants.asm`; EmuNations and RetroMaggedon SRM samples.
- confidence: high for slot boundaries, checksum repair, mirrors, event-count writes, switch-block bytes, numbered event bits, documented level-state bit categories, submap IDs, player position/pointer words, blank-file initialization, and the verified 96-exit template.

## 4. Editable Fields
| fieldId | label | type | slot scope | safe values | read/write location | proof |
|---|---|---|---|---|---|---|
| greenSwitchBlocks | Green Switch Blocks | boolean | selected save file | false/true stored as 0x00/0x01 | slot + 0x85 and mirrored backup | SMWCentral maps the four bytes at +0x85 as switch block flags in Green, Yellow, Blue, Red order; samples use 0/1 values |
| yellowSwitchBlocks | Yellow Switch Blocks | boolean | selected save file | false/true stored as 0x00/0x01 | slot + 0x86 and mirrored backup | same source and sample confirmation |
| blueSwitchBlocks | Blue Switch Blocks | boolean | selected save file | false/true stored as 0x00/0x01 | slot + 0x87 and mirrored backup | same source and sample confirmation |
| redSwitchBlocks | Red Switch Blocks | boolean | selected save file | false/true stored as 0x00/0x01 | slot + 0x88 and mirrored backup | same source and sample confirmation |
| eventCount | Event Count | integer | selected save file | 0-96 | slot + 0x8c and mirrored backup | SRAM map labels event count; SMWDisX defines `!TotalExitCount = 96`; samples match 30, 49, 66, 95, and 96 |
| overworldEvents | Overworld Events | bitmask | selected save file | `event000`-`event119` | slot + 0x60..0x6e and mirrored backup | SRAM/RAM maps document the 15-byte event table and event-bit formula |
| levelBeatenFlags | Level Beaten Flags | bitmask | selected save file | `level00`-`level95` | bit 7 of slot + 0x00..0x5f and mirrored backup | RAM map documents level-state format as `bmesudlr` |
| levelMidwayFlags | Level Midway Flags | bitmask | selected save file | `level00`-`level95` | bit 5 of slot + 0x00..0x5f and mirrored backup | RAM map documents level-state format as `bmesudlr` |
| levelMoveUpFlags | Level Move Up Flags | bitmask | selected save file | `level00`-`level95` | bit 3 of slot + 0x00..0x5f and mirrored backup | RAM map documents level-state format as `bmesudlr` |
| levelMoveDownFlags | Level Move Down Flags | bitmask | selected save file | `level00`-`level95` | bit 2 of slot + 0x00..0x5f and mirrored backup | RAM map documents level-state format as `bmesudlr` |
| levelMoveLeftFlags | Level Move Left Flags | bitmask | selected save file | `level00`-`level95` | bit 1 of slot + 0x00..0x5f and mirrored backup | RAM map documents level-state format as `bmesudlr` |
| levelMoveRightFlags | Level Move Right Flags | bitmask | selected save file | `level00`-`level95` | bit 0 of slot + 0x00..0x5f and mirrored backup | RAM map documents level-state format as `bmesudlr` |
| marioSubmap / luigiSubmap | Player Submap | enum | selected save file | main, yoshisIsland, vanillaDome, forestOfIllusion, valleyOfBowser, specialWorld, starWorld | slot + 0x6f / +0x70 and mirrored backup | SRAM map labels current submap; SMWDisX constants define IDs 0-6 |
| marioOverworldAnimation / luigiOverworldAnimation | Overworld Animation | integer | selected save file | 0-65535 | slot + 0x71 / +0x73 and mirrored backup | SRAM map labels the 4-byte player animation area |
| marioOverworldX/Y, luigiOverworldX/Y | Overworld Position | integer | selected save file | 0-65535 | slot + 0x75..0x7c and mirrored backup | SRAM map labels player X/Y words; SMWDisX init routine writes starting values |
| marioOverworldX/YPointer, luigiOverworldX/YPointer | Overworld Position Pointers | integer | selected save file | 0-65535 | slot + 0x7d..0x84 and mirrored backup | SRAM map labels player position pointer words; SMWDisX init routine writes position/16 values |
| createBlankFile | Create Blank File | boolean action | selected save file, including blank slot | true | full 143-byte slot and mirrored backup | SMWDisX `InitSaveData` and constants define Yoshi's Island starting state |
| install96ExitTemplate | Install 96-Exit File | boolean action | selected save file, including blank slot | true | full 143-byte slot and mirrored backup | verified EmuNations 96-exit sample block with checksum repair |

Read-only parser details:

- active save-file indexes
- per-slot primary/backup validity
- per-slot backup match status
- per-slot event count at slot + 0x8c
- per-slot active switch-block color list
- blank-SRAM support status
- used-byte count 858

## 5. Presets
- createBlankFile: creates a valid Yoshi's Island starting file in the selected slot.
- activateSwitchBlocks: sets Green, Yellow, Blue, and Red switch-block flags to true in the selected save file.
- install96ExitTemplate: installs the verified 96-exit sample block into the selected slot.

## 6. Required Backend Logic
- existing editor can be reused: no
- new parser/editor needed: yes, implemented as runtime module `snes-super-mario-world`
- checksum repair required: yes, rebuild the 0x5A5A checksum complement after edits
- mirrored-write required: yes, write edited slot to primary and backup blocks

## 7. Verification
- before/after checks: validate 2048-byte size, at least one checksummed non-empty save-file block, and primary/backup copy status.
- sample confirmation: all four sample SRMs match the documented slot size and checksum formula; active slots have mirrored valid backup copies.
- module confirmation: local WASM ABI smoke tests parsed a real sample, read File A bitfields/map fields, created File C from a blank template, installed the 96-exit template into an all-zero SRAM, rebuilt checksums, and mirrored edited slots.

## 8. Open Questions
- Numbered event bits are exposed, but not translated into human-named events/stages.
- Numbered level slots are exposed only for documented `bmesudlr` categories; unknown/unused level-state bits are not writable.
- Player map/position/pointer fields are advanced structural edits. The module repairs integrity, but it cannot prove every coordinate combination is playable.
- Switch-block flags are labeled as block activation only unless paired with event/level edits.
- Raw Game Genie, Action Replay, Pro Action Replay, and other trainer-code writes are not supported because they are RAM/runtime patch formats, not SRAM structure.

## 9. Decision
- code-ready and implemented as a limited runtime module.
