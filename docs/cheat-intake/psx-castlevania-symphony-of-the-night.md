# Cheat Intake: Castlevania: Symphony of the Night

## 1. Identity
- systemSlug: `psx`
- canonicalTitle: `Castlevania: Symphony of the Night`
- requestedTitle: `Playstation - CASTLEVANIA-1 JB 14%`
- regions: US/NTSC PlayStation release only for write support; memory-card id `BASLUS-00067DRAXnn`
- extensions: `mcr`, `mcd`, `mc`, `bin`, `sav`
- runtimes: raw PlayStation memory card images; standalone one-block logical save payloads
- saveModel: one 8192-byte PS1 memory-card block per game save

The requested `CASTLEVANIA-1 JB 14%` text matches the game's memory-card title pattern: title, save slot, player name, and castle completion percentage. It is not used as the only proof of identity.

## 2. Samples
| file | size | sha256 | runtime | notes |
|---|---:|---|---|---|
| `csotnsave01-Alch_Laboratory.zip` | 4461 | `307c78e95f972fff54594c172775c27acb62ac73264c8992b162750f9b3bcbdf` | ePSXe `.mcr` | FantasyAnime NTSC sample archive |
| `csotnsave01` `epsxe000.mcr` | 131072 | `2bf297ff159ba035125d6fb81bc29de2643831119a69a3f8a32e7fedb7349e28` | ePSXe `.mcr` | one `BASLUS-00067DRAX00` entry; level 2; 3% |
| `csotnsave02` `epsxe000.mcr` | 131072 | `ede5b5e3dd03527d11d49d8359fd910335da93b3d9c324a0b60480a6495c9e3a` | ePSXe `.mcr` | one `BASLUS-00067DRAX00` entry; level 11; 15% |
| `csotnsave14` `epsxe000.mcr` | 131072 | `cf080ae1d6b246677de6bd93bb9d11af5e70cb0ac93207d7e557f72fe6cad3f7` | ePSXe `.mcr` | one `BASLUS-00067DRAX00` entry; inverse-castle transition; 97% |
| `csotnsave18` `epsxe000.mcr` | 131072 | `bade3584a81392a40495d12e64ffd0f500db666bf5cca2bf9f926f075955c726` | ePSXe `.mcr` | one `BASLUS-00067DRAX00` entry; endgame; 188% |

The sample page states these are NTSC PS1 `.mcr` saves and each card contains only the specified SOTN save with other slots empty.

## 3. Format Evidence
- container/header: raw PS1 memory card starts with `MC`; used directory frames are 128 bytes with XOR checksum.
- slot layout: directory state `0x51`, file size `0x2000`, next block `0xFFFF`, filename prefix `BASLUS-00067DRAX`; the final two filename digits encode the game save slot.
- payload header: selected block starts `53 43 13 01` (`SC`, icon type `0x13`, one block).
- save structure: decomp `SaveData` places `MemcardHeader` at `0x000`, `SaveInfo` at `0x200`, `PlayerStatus` at `0x238`, `MenuNavigation` at `0x56C`, `GameSettings` at `0x5B8`, castle flags at `0x6C8`, castle map at the layout-implied `0x9C8`, and `rng` at `0x11C8`; total struct size is `0x11CC`.
- checksum/crc: no SOTN gameplay checksum was found in `StoreSaveData` or `ApplySaveData`; PS1 directory/header checksums are container metadata and are preserved by gold edits.
- parser/doc sources: `sotn-decomp` `include/game.h`, `src/save_mgr.h`, and FantasyAnime NTSC save samples.
- confidence: high for US/NTSC raw cards and one-block payloads; unsupported for PAL/Japan until matching ids and samples are verified.

## 4. Editable Fields
| fieldId | label | type | slot scope | safe values | read/write location | proof |
|---|---|---|---|---|---|---|
| `gold` | Gold | integer | selected SOTN save slot | `0..999999` | block `+0x210` (`SaveInfo.gold`) and block `+0x4C4` (`PlayerStatus.gold`) | `MAX_GOLD` is `999999`; `StoreSaveData` copies `g_Status.gold` to both structures; samples confirm duplicated values; `ApplySaveData` restores `PlayerStatus` after checking `SaveInfo.saveSize`. |

Read-only parser details exposed in save inspection:
- memory-card directory slot and SOTN save slot
- player name
- level
- gold
- explored rooms and computed percentage using the game's `942` room-count denominator
- play time
- stage id and character id
- HP/MP/hearts/current maxima
- exp and kill count
- spells learnt mask and raw spell bytes
- relic bytes and non-zero relic byte count
- inventory hand/body count arrays and inventory order arrays
- raw equipment slots, attack hands, and defense equip
- castle flags bytes and non-zero byte count
- castle map bytes and non-zero byte count

## 5. Presets
- preset id: `maxGold`
- label: `Max gold`
- updates: `gold: 999999`

## 6. Required Backend Logic
- existing editor can be reused: no
- new parser/editor needed: yes, delivered as `modules/playstation-castlevania-symphony-of-the-night.rsmodule.zip`
- checksum repair required: no SOTN gameplay checksum; PS1 directory frames are preserved for this edit
- mirrored-write required: yes, gold is written to both `SaveInfo.gold` and `PlayerStatus.gold`

## 7. Verification
- before/after checks: module parser validates `BASLUS-00067DRAXnn`, `SC 13 01`, US title prefix, `SaveInfo.saveSize == 0x11CC`, matching level/gold copies, and PS1 frame checksums when parsing full memory cards.
- parser/tool confirmation: FantasyAnime samples were decoded and compared against decomp offsets for level, gold, time, rooms, HP/MP/hearts, exp, kill count, relics, inventory arrays, equipment slots, castle flags, and castle map.
- automated verification: run repository cheat-pack validation and backend module import tests.

## 8. Open Questions
- PAL (`BESLES`/`BESCES`) and Japanese (`BISLPM`) variants are intentionally unsupported until real samples are compared.
- HP, MP, hearts, rooms, level, stage, relics, inventory, equipment, map, and castle flags are read-only for now; static safe ranges and all secondary side effects were not fully proven for writes.
- The module supports raw cards and standalone 8192-byte blocks; DexDrive `.gme` and PSP `.vmp` wrappers are not written until wrapper-preserving tests are added.

## 9. Decision
- code-ready / module-backed
