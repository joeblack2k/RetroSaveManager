# Cheat Intake: Banjo-Tooie

## 1. Identity
- systemSlug: `n64`
- canonicalTitle: `Banjo-Tooie`
- regions: Europe, USA, Japan, Australia samples verified through the Game Tools Collection corpus.
- extensions: `.eep`, `.bin`, `.srm`
- runtimes: native N64 EEPROM windows and RetroArch N64 SRM containers.
- saveModel: 16Kbit EEPROM with four 0x1c0-byte save slots at fixed offsets.

## 2. Samples
| file | size | sha256 | runtime | notes |
|---|---:|---|---|---|
| `banjo-tooie/europe.eep` | 2048 | `8b57429231e850ad54b75db2de9744d8fc753f33b5bfccdc537cd702ca5f5327` | Game Tools Collection test corpus | Active Files B/C; validates `KHJC` slots and 64-bit checksums. |
| `banjo-tooie/usa.eep` | 2048 | `bec0c4010859a1552c14d530535f9e252938acfb096cac70beb8070dd62d1678` | Game Tools Collection test corpus | Active File C; used for module parse/read/apply smoke test. |
| `banjo-tooie/japan.eep` | 2048 | `85fbb9a6ee9a82f6560a7b307ccd4340b0dd55c2bb43a0babd84895e100cf234` | Game Tools Collection test corpus | Active Files A/C; validates alternate region sample. |
| `banjo-tooie/australia.eep` | 2048 | `6d02f49f8fe27630dfaa5a5d7da0ba69fd36b3a3d97f8220f01f129bc79e33f0` | Game Tools Collection test corpus | Active File B; validates Australia sample. |
| `banjo-tooie/usa.srm` | 296960 | `5e1f0ca5c3d9645987512123faa7060b86d811bf6ac87d08091f35fa43fbfd1b` | Game Tools Collection test corpus | RetroArch container; first EEPROM window parsed read-only. |

## 3. Format Evidence
- container/header: Game Tools Collection validates `KHJC` magic at slot starts `0x100`, `0x2c0`, `0x480`, and `0x640`.
- slot layout: each slot is 0x1c0 bytes; display slot id is stored at slot offset `0x0a`.
- checksum/crc: each active slot stores a big-endian 64-bit Rare checksum at slot offset `0x1b8` over bytes `0x00-0x1b7`.
- region shifting: Game Tools Collection applies the USA bit shift to slot bit/bitfield fields at/after logical offset `0xa6`; this module implements that transform when RetroSaveManager metadata marks the save region as US, otherwise it uses the non-USA logical offsets used for Europe/Japan/Australia.
- replay/settings: Game Tools Collection stores replay/settings in an extra 0x80-byte block at `0x000` or `0x080`, with a 64-bit Rare checksum at block offset `0x78`.
- parser/doc sources:
  - https://github.com/RyudoSynbios/game-tools-collection/tree/main/src/lib/templates/banjo-tooie
  - https://github.com/RyudoSynbios/game-tools-collection/tree/main/tests/saves/banjo-tooie
- confidence: high for `KHJC` slot structure, checksums, counters, per-level times, collection bitflags, moves, named progress flags, USA-shift transform, and replay/settings checksum. Region selection is metadata-driven because the EEPROM bytes do not uniquely identify USA vs non-USA layout.

## 4. Editable Fields
| fieldId | label | type | slot scope | safe values | read/write location | proof |
|---|---|---|---|---|---|---|
| `eggs` | Eggs | integer | selected file | `0..999` | slot `0x2b`, u16 BE | Game Tools Collection template. |
| `fireEggs` | Fire Eggs | integer | selected file | `0..999` | slot `0x2d`, u16 BE | Game Tools Collection template. |
| `iceEggs` | Ice Eggs | integer | selected file | `0..999` | slot `0x2f`, u16 BE | Game Tools Collection template. |
| `grenadeEggs` | Grenade Eggs | integer | selected file | `0..999` | slot `0x31`, u16 BE | Game Tools Collection template. |
| `clockworkEggs` | Clockwork Kazooie Eggs | integer | selected file | `0..999` | slot `0x33`, u16 BE | Game Tools Collection template. |
| `redFeathers` | Red Feathers | integer | selected file | `0..999` | slot `0x37`, u16 BE | Game Tools Collection template. |
| `goldFeathers` | Gold Feathers | integer | selected file | `0..999` | slot `0x39`, u16 BE | Game Tools Collection template. |
| `extraHoneycombs` | Extra Honeycombs | integer | selected file | `0..25` | slot `0x3d`, u16 BE | Game Tools Collection template. |
| `cheatoPages` | Cheato Pages | integer | selected file | `0..25` | slot `0x3f`, u16 BE | Game Tools Collection template. |
| `glowbos` | Glowbos | integer | selected file | `0..17` | slot `0x3b`, u16 BE | Game Tools Collection template. |
| `megaGlowbos` | Mega Glowbos | integer | selected file | `0..1` | slot `0x53`, u16 BE | Game Tools Collection template. |
| `doubloons` | Doubloons | integer | selected file | `0..30` | slot `0x47`, u16 BE | Game Tools Collection template. |
| `iceKey` | Ice Key | integer | selected file | `0..1` | slot `0x51`, u16 BE | Game Tools Collection template. |
| `blueSecretEgg` | Blue Secret Egg | boolean | selected file | true/false | slot `0xfa`, bit 5 | Game Tools Collection template. |
| `pinkSecretEgg` | Pink Secret Egg | boolean | selected file | true/false | slot `0xfa`, bit 7 | Game Tools Collection template. |
| `time*` | Per-level playtime seconds | integer | selected file | `0..65535` | slot `0x0f..0x25` named level timers | Game Tools Collection playtime groups; total readback sums `0x0d..0x27`. |
| `extraEnergy`, `dragonKazooie`, `trainLocationBits`, `jinjoSeed`, `klungoPotion`, `jiggywiggyChallenge`, `towerOfTragedyRound` | General progress values | integer/boolean | selected file | template ranges | slot bitfields at `0xe0`, `0xe9`, `0xed`, `0xfb`, `0x106`, `0x11b` | Game Tools Collection variables/resources. |
| `jiggyFlags` | Individual Jiggy flags | bitmask | selected file | 90 named flags | logical flags starting at `0xc8` | Game Tools Collection total/jiggy bitflags, USA shift applied when region metadata is US. |
| `noteFlags` | Individual Note flags | bitmask | selected file | 153 named flags | logical flags starting at `0x108` | Game Tools Collection total/note bitflags; Treble Clefs are included as 20-note flags. |
| `jinjoFlags`, `cheatoPageFlags`, `extraHoneycombFlags`, `glowboFlags` | Individual collectible flags | bitmask | selected file | named template flags | logical offsets in the GTC level tabs | Game Tools Collection bitflags, USA shift applied when needed. |
| `specialMoves` | Learned/special moves | bitmask | selected file | named Basic/Advanced move flags | logical offsets `0x9f..0xb6` | Game Tools Collection Basic Moves and Advanced Moves groups. |
| `progressFlags`, `warpPads`, `silos`, `cheats*`, `jukeboxTracks` | Progress/jukebox/cheat flags | bitmask | selected file | named non-hidden flags | logical offsets through late slot region | Game Tools Collection Events/Warp Pads/Silos/Cheats/Jukebox groups, with USA shift. |
| `wideScreen`, `speakerMode`, `language` | Settings/language | boolean/integer | global extra block | true/false, `0..3`, `0..3` | extra block offsets `0x01`, `0x02`, `0x0b` | Game Tools Collection Settings appendSubinstance; language is Europe-labeled in GTC. |
| `replayMiniGames`, `replayBosses`, `replayCinema`, `replayMisc`, `replay*` high scores | Replay unlocks and scores | bitmask/integer | global extra block | template ranges | extra block offsets `0x03..0x39`; checksum `0x78` | Game Tools Collection Replay appendSubinstance; USA extra score offsets `0x0f..0x3a` subtract 2 per `utils.ts`. |

## 5. Presets
- none. No max-all, unlock-all, or progress presets are shipped.

## 6. Required Backend Logic
- existing editor can be reused: no.
- new parser/editor needed: yes, delivered as `modules/n64-banjo-tooie.rsmodule.zip`.
- checksum repair required: yes, rebuild selected slot's 64-bit Rare checksum.
- mirrored-write required: no mirrored slot copy is used for the supported fields.

## 7. Verification
- module import: `TestRepositoryGameModulesImport` passed.
- parser/tool confirmation: direct WASM ABI smoke test parsed `usa.eep`, read File C values, applied counter, move, replay, setting, and USA-shifted note flag updates, then reparsed the output successfully with rebuilt slot and replay/settings checksums. `usa.srm` parsed as read-only.
- in-game confirmation: not performed.

## 8. Open Questions
- USA vs non-USA semantic layout is not inferable from the EEPROM bytes alone; the module uses RetroSaveManager `regionCode`/`regionFlag` metadata for the USA shift and reports `usaRegionShiftActive` in read values.
- Hidden/internal GTC flags and `???` placeholders are not editable.
- RetroArch SRM containers are parsed/read-only; editing is limited to native 2048-byte EEPROM windows to avoid returning a full 0x48800-byte container from the current WASM ABI.

## 9. Decision
- module-backed parser with RW support for proven native EEPROM fields and read-only support for RetroArch SRM containers.
