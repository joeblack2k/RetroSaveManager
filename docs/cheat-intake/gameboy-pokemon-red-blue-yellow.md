# Cheat Intake: Pokemon Red/Blue/Yellow

## 1. Identity
- systemSlug: gameboy
- canonicalTitle: Pokemon Red/Blue/Yellow
- regions: international Red, Blue, Yellow write support; Japanese shifted layout researched but not exposed
- extensions: sav, srm, ram, sram
- runtimes: raw Game Boy SRAM exports
- saveModel: 32 KiB cartridge SRAM, main save block plus PC box banks

## 2. Samples
| file | size | sha256 | runtime | notes |
|---|---:|---|---|---|
| game-tools-collection red-europeusa.sav | 32768 | 5aabc0042a55a0e5beb40d122db7a0ae5eca092ef320543d324c5a7d8e29c1f6 | raw SRAM | international Red sample, checksum/field sanity |
| game-tools-collection blue-europeusa.sav | 32768 | d3b43cac0dde1a8adc5542fd6c9d7963aec3e6d84eed5ccc0f349fa41bbae78f | raw SRAM | international Blue sample |
| game-tools-collection yellow-europeusa.sav | 32768 | a0653c78838bedd5010535d0c84e3bc0bee093fcd166c3a4d0749c4fb1288871 | raw SRAM | international Yellow sample |

## 3. Format Evidence
- container/header: 32 KiB Game Boy SRAM. pret `pokered` and `pokeyellow` define `sGameData` after `0x2598` bytes in SRAM.
- slot layout: one active main save; PC boxes live in separate SRAM bank ranges and are not edited.
- checksum/crc: main checksum byte at `0x3523`, computed as `0xff - (sum(0x2598..0x3522) & 0xff)`.
- mirrored data: no mirrored main block is used for these exposed fields.
- parser/doc sources: pret `pokered`/`pokeyellow` SRAM and save code; RyudoSynbios game-tools-collection Pokemon Red/Blue/Yellow template and public test saves.
- confidence: high for international money, casino coins, badges, and checksum repair.

Sources:
- https://github.com/pret/pokered/blob/master/ram/sram.asm
- https://github.com/pret/pokered/blob/master/engine/menus/save.asm
- https://github.com/pret/pokeyellow/blob/master/ram/sram.asm
- https://github.com/pret/pokeyellow/blob/master/engine/menus/save.asm
- https://github.com/RyudoSynbios/game-tools-collection/blob/master/src/lib/templates/pokemon-red-blue-and-yellow/saveEditor/template.ts

## 4. Editable Fields
| fieldId | label | type | slot scope | safe values | read/write location | proof |
|---|---|---|---|---|---|---|
| money | Money | integer | main save | 0-999999 | `0x25f3`, 3-byte big-endian BCD | game-tools template offset; pret main save checksum range |
| casinoCoins | Casino Coins | integer | main save | 0-9999 | `0x2850`, 2-byte big-endian BCD | game-tools template offset |
| badges | Badges | bitmask | main save | 8 documented Kanto badge bits | `0x2602`, bits 0-7 | game-tools template bit labels |

## 5. Presets
- maxMoney: `money=999999`
- maxCasinoCoins: `casinoCoins=9999`
- allBadges: sets all eight documented badge bits

## 6. Required Backend Logic
- existing editor can be reused: no
- new parser/editor needed: yes, supplied as `gameboy-pokemon-red-blue-yellow.rsmodule.zip`
- checksum repair required: yes, main checksum byte rebuilt after writes
- mirrored-write required: no

## 7. Verification
- before/after checks: module import, checksum validation, BCD validation, and checksum rebuild through WASM parser logic
- in-game confirmation: not performed in this pass
- parser/tool confirmation: offsets cross-checked with pret and game-tools-collection samples

## 8. Open Questions
- Japanese Red/Green/Blue/Yellow uses shifted layout and is intentionally not writable here.
- Pokedex, bag, party, PC boxes, names, and playtime are not exposed until mutation side effects are reviewed.

## 9. Decision
- code-ready / module-backed write support for international Pokemon Red/Blue/Yellow currency and badges only
