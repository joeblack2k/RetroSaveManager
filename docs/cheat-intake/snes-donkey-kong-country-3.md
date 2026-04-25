# Cheat Intake: Donkey Kong Country 3 - Dixie Kong's Double Trouble!

## 1. Identity
- systemSlug: snes
- canonicalTitle: Donkey Kong Country 3 - Dixie Kong's Double Trouble!
- regions: USA/EUR-compatible SRAM layout expected
- extensions: .srm, .sav, .sram, .sa1
- runtimes: Snes9x/bsnes raw cartridge SRAM
- saveModel: 2 KiB SRAM with three checksummed save slots

## 2. Samples
| file | size | sha256 | runtime | notes |
|---|---:|---|---|---|
| Zophar DKC3.SRM | 2048 | ce2f1f227f7fafbf4dda0f27fc079ee25bc95b72227037c914ea188ec27429de | ZSNES/SNES SRAM | Slot 1, 103% note |
| Zophar DKC3.srm | 2048 | 15f1a73cc385d183839a79edfeae88e9b2b7b5147c18233bcb81c3c5ec8212c7 | ZSNES/SNES SRAM | Slot 1, first two worlds note |
| Zophar DKC3 USA SRM | 2048 | 8a5744906d967b6bd6569bd63dd9f95fca1db3e1a0dacba7e24d06671cf3ad81 | SNES SRAM | Slot 1, 103% note |

## 3. Format Evidence
- container/header: raw 2048-byte cartridge SRAM.
- slot layout: slot bases are 0x0062, 0x02ec, and 0x0576; each slot is 0x028a bytes.
- checksum/crc: each slot stores a little-endian sum at +0x00 and XOR at +0x02 over words +0x06 through +0x288. A zero sum is stored as 0x0001.
- mirrored data: slot 3 can select an alternate data area at +0x148 when marker bit 0 is set; normal data starts at +0x06.
- parser/doc sources: Yoshifanatic1 DKC3 disassembly, `Routine_Macros_DKC3.asm` routines `CODE_809141`, `CODE_809177`, `CODE_80920D`, and `CODE_8092F2`; monorail/Archipelago DKC3 `item_rom_data` for collectible RAM counters.
- confidence: high for counters and checksum repair; no stage/progression flags promoted yet.

## 4. Editable Fields
| fieldId | label | type | slot scope | safe values | read/write location | proof |
|---|---|---|---|---|---|---|
| bearCoins | Bear Coins | integer | selected save slot | 0-99 | data + 0x13, WRAM 0x05C9 | DKC3 save routine stores WRAM 0x05C9 at data + 0x13; monorail/Archipelago labels 0x05C9 as Bear Coin |
| bonusCoins | Bonus Coins | integer | selected save slot | 0-85 | data + 0x15, WRAM 0x05CB | DKC3 save routine stores WRAM 0x05CB at data + 0x15; monorail/Archipelago labels 0x05CB as Bonus Coin |
| bananaBirds | Banana Birds | integer | selected save slot | 0-15 | data + 0x17, WRAM 0x05CD | DKC3 save routine stores WRAM 0x05CD at data + 0x17; monorail/Archipelago labels 0x05CD as Banana Bird |
| dkCoins | DK Coins | integer | selected save slot | 0-41 | data + 0x19, WRAM 0x05CF | DKC3 save routine stores WRAM 0x05CF at data + 0x19; monorail/Archipelago labels 0x05CF as DK Coin |

## 5. Presets
- maxCurrencies: sets Bear Coins to 99, Bonus Coins to 85, and DK Coins to 41.
- freeBananaBirdQueen: sets Banana Birds to 15.
- maxCollectibles: applies all supported collectible counters.

## 6. Required Backend Logic
- existing editor can be reused: no
- new parser/editor needed: yes, `dkc3-sram`
- checksum repair required: yes
- mirrored-write required: no for normal slots; editor respects the active slot 3 data-area bit

## 7. Verification
- before/after checks: parse slot marker and checksum before edits.
- after edits: recompute per-slot sum/XOR and reparse.
- parser/tool confirmation: Go unit tests cover read/apply, checksum repair, endpoint exposure, and new-save versioning.

## 8. Open Questions
- Stage clear flags, world unlock flags, vehicle unlocks, cogs, and Brothers Bear trade-state flags need separate proof before becoming live fields.
- Completion percentage is derived by the game from other state, so it is intentionally not exposed as an editable counter.

## 9. Decision
- code-ready and implemented as a limited parser-backed pack.
