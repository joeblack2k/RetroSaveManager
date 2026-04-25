# Cheat Intake: Banjo-Kazooie

## 1. Identity
- systemSlug: `n64`
- canonicalTitle: `Banjo-Kazooie`
- regions: Europe, USA, Japan samples verified through the Game Tools Collection corpus.
- extensions: `.eep`, `.bin`, `.srm`
- runtimes: native N64 EEPROM windows and RetroArch N64 SRM containers.
- saveModel: 4Kbit EEPROM save data; 0x200 bytes of Banjo-Kazooie data, commonly stored in a 0x800-byte emulator EEPROM window.

## 2. Samples
| file | size | sha256 | runtime | notes |
|---|---:|---|---|---|
| `banjo-kazooie/europe.eep` | 2048 | `bf6750273d42584484fc7ed0bcdf4b8cab49ca51f12c76e972b29ed6207adffe` | Game Tools Collection test corpus | Active File C; validates 0x78-byte slot checksum. |
| `banjo-kazooie/usa.eep` | 2048 | `c16e9d45ba34c208abd9e7957c9c79e19e20fcc513ca8d55a35176902ff7b8f1` | Game Tools Collection test corpus | Active File A; used for module parse/read/apply smoke test. |
| `banjo-kazooie/japan.eep` | 2048 | `3ab9dcd5102559c0c47d4190bc17ebcb03fc13c42ec5897898e0691208885efa` | Game Tools Collection test corpus | Active File B; validates alternate region sample. |
| `banjo-kazooie/usa.srm` | 296960 | `b7ddb241d84c360f6dd56b08aabe09ad45112498b0443982c57d440ed170bd90` | Game Tools Collection test corpus | RetroArch container; first EEPROM window parsed read-only. |

## 3. Format Evidence
- container/header: BKSaveEditor defines `SAVE_DATA_SIZE = 0x200`, four internal `SaveSlot` blocks, and `GlobalData`; Game Tools Collection accepts 0x800-byte EEPROM windows and 0x48800-byte RetroArch SRM containers.
- slot layout: four internal 0x78-byte slots, three user-facing files; internal slot values 2 and 3 are display-remapped.
- checksum/crc: each slot stores a big-endian 32-bit Rare checksum at slot offset `0x74` over `0x00-0x73`; global Stop n Swop data stores its checksum at global offset `0x1c`.
- parser/doc sources:
  - https://github.com/MaikelChan/BKSaveEditor
  - https://github.com/RyudoSynbios/game-tools-collection/tree/main/src/lib/templates/banjo-kazooie
  - https://github.com/RyudoSynbios/game-tools-collection/tree/main/tests/saves/banjo-kazooie
- confidence: high for slot/global structure, checksums, inventory/upgrades, per-level times, note score packing, Jiggy/Mumbo Token/Extra Honeycomb location flags, learned/used ability bitmasks, named progress bits, and Stop n Swop global flags.

## 4. Editable Fields
| fieldId | label | type | slot scope | safe values | read/write location | proof |
|---|---|---|---|---|---|---|
| `jiggiesHeld` | Jiggies Held | integer | selected file | `0..100` | slot `0x69` | BKSaveEditor `Items`/UI and Game Tools Collection template. |
| `mumboTokens` | Mumbo Tokens | integer | selected file | `0..116` | slot `0x65` | BKSaveEditor held item enum and Game Tools Collection template. |
| `eggs` | Eggs | integer | selected file | `0..200` | slot `0x66` | Game Tools Collection dynamic max logic and BKSaveEditor item offset. |
| `redFeathers` | Red Feathers | integer | selected file | `0..100` | slot `0x67` | Game Tools Collection dynamic max logic and BKSaveEditor item offset. |
| `goldFeathers` | Gold Feathers | integer | selected file | `0..20` | slot `0x68` | Game Tools Collection dynamic max logic and BKSaveEditor item offset. |
| `doubleHealth` | Double Health | boolean | selected file | true/false | slot `0x57`, bit 1 | Game Tools Collection health flag. |
| `maxEggsUpgrade` | 200 Egg Capacity | boolean | selected file | true/false | slot `0x57`, bit 6 | Game Tools Collection max egg flag. |
| `maxRedFeathersUpgrade` | 100 Red Feather Capacity | boolean | selected file | true/false | slot `0x57`, bit 7 | Game Tools Collection max red feather flag. |
| `maxGoldFeathersUpgrade` | 20 Gold Feather Capacity | boolean | selected file | true/false | slot `0x58`, bit 0 | Game Tools Collection max gold feather flag. |
| `time*` | Per-level playtime seconds | integer | selected file | `0..65535` | slot `0x2a..0x3f`, u16 BE with BKSaveEditor level index order | BKSaveEditor `Times[11]` and Game Tools Collection playtime groups. |
| `notes*` | Per-level note scores | integer | selected file | `0..100` | slot `0x22..0x29`, packed 7-bit scores in a BE u64 | BKSaveEditor `GetNotes`/`SetNotes` bit offsets. |
| `jiggyFlags` | Individual Jiggy flags | bitmask | selected file | 100 named flags | slot `0x02..0x0e` | Game Tools Collection level Jiggy bitflags, cross-checked with BKSaveEditor Jiggy storage. |
| `mumboTokenFlags` | Individual Mumbo Token flags | bitmask | selected file | named flags from template | slot `0x12..0x21` | Game Tools Collection and BKSaveEditor `MumboTokens[16]`. |
| `extraHoneycombFlags` | Individual Extra Honeycomb flags | bitmask | selected file | 24 named flags | slot `0x0f..0x11` | Game Tools Collection and BKSaveEditor `Honeycombs[3]`. |
| `learnedAbilities` | Learned abilities | bitmask | selected file | 20 named flags | slot `0x6a..0x6d`, u32 BE bitmask | BKSaveEditor `LearnableAbilities`. |
| `usedAbilities` | Used ability/tutorial flags | bitmask | selected file | 13 named flags | slot `0x6e..0x71`, u32 BE bitmask | BKSaveEditor `UsableAbilities`. |
| `progressFlags` | Named progress flags | bitmask | selected file | 86 named non-hidden flags | slot flags area `0x40..0x64` | Game Tools Collection Events/Note Doors/Magic Cauldrons and BKSaveEditor `Flags[0x25]`. |
| `snsUnlocked` / `snsObtained` | Stop n Swop flags | bitmask | global | 7 unlocked + 7 obtained flags | global `0x1e0..0x1e1`; checksum `0x1fc` | Game Tools Collection Stop n Swop appendSubinstance and BKSaveEditor `GlobalData`. |

## 5. Presets
- none. No max-all, unlock-all, or progress presets are shipped.

## 6. Required Backend Logic
- existing editor can be reused: no.
- new parser/editor needed: yes, delivered as `modules/n64-banjo-kazooie.rsmodule.zip`.
- checksum repair required: yes, rebuild selected slot's 32-bit Rare checksum.
- mirrored-write required: no mirrored slot copy is used for the supported fields.

## 7. Verification
- module import: `TestRepositoryGameModulesImport` passed.
- parser/tool confirmation: direct WASM ABI smoke test parsed `usa.eep`, read File A values, applied note score, learned ability, and Stop n Swop updates, then reparsed the output successfully with rebuilt slot/global checksums. `usa.srm` parsed as read-only.
- in-game confirmation: not performed.

## 8. Open Questions
- Native N64 Banjo-Kazooie stores per-level note scores, not individual loose note pickups; individual note-location editing is only present in BKSaveEditor for the Banjo Recompiled extension and is not exposed as N64 EEPROM support here.
- Hidden/internal GTC flags and `???` placeholders are not editable.
- RetroArch SRM containers are parsed/read-only; editing is limited to native 512/2048-byte EEPROM windows to avoid returning a full 0x48800-byte container from the current WASM ABI.

## 9. Decision
- module-backed parser with RW support for proven native EEPROM fields and read-only support for RetroArch SRM containers.
