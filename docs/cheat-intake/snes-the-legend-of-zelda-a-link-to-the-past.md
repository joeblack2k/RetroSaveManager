# Cheat Intake: The Legend of Zelda: A Link to the Past

## 1. Identity
- systemSlug: snes
- canonicalTitle: The Legend of Zelda: A Link to the Past
- regions: USA/EUR and Japan markers supported
- extensions: .srm, .sav, .sram
- runtimes: raw SNES cartridge SRAM
- saveModel: 8 KiB SRAM with three primary 0x500-byte slots and three mirrored 0x500-byte slots

## 2. Samples
| file | size | sha256 | runtime | notes |
|---|---:|---|---|---|
| synthetic fixture | 8192 | generated in `cheat_alttp_test.go` | unit test | Slot 2 primary and mirror with valid marker/checksum |

## 3. Format Evidence
- container/header: raw 8192-byte SRAM.
- slot layout: primary slots start at 0x0000, 0x0500, and 0x0a00; mirrors start at 0x0f00, 0x1400, and 0x1900.
- checksum/crc: a valid 0x500-byte slot sums all little-endian 16-bit words to 0x5a5a. The repair word is stored at slot + 0x04fe.
- markers: USA/EUR slots carry bytes aa 55 at slot + 0x03e5; Japan slots carry bytes aa 55 at slot + 0x03e1.
- parser/doc sources: snesrev `zelda3` `select_file.c` checksum/mirror validation and `variables.h` WRAM labels; mysterypaint `ALTTPSRAMEditor` `Constants.cs`, `SRAM.cs`, `SaveSlot.cs`, and `Link.cs` item offsets and edit behavior.
- confidence: high for item/resource fields, slot mirrors, region markers, and checksum repair.

## 4. Editable Fields
| fieldId | label | type | slot scope | safe values | read/write location | proof |
|---|---|---|---|---|---|---|
| rupees | Rupees | integer | selected save slot | 0-999 | item data + 0x20 and +0x22 | WRAM labels expose rupee goal/current; ALTTPSRAMEditor writes current rupees at item data + 0x22 |
| bombs/arrows | Bombs/Arrows | integer | selected save slot | 0-50 / 0-70 | item data + 0x03 and +0x37 | ALTTPSRAMEditor constants and capacity tables |
| heartContainers | Heart Containers | integer | selected save slot | 3-20 | item data + 0x2c and +0x2d, stored as hearts * 8 | ALTTPSRAMEditor sets max/current health together |
| magicPower | Magic Power | integer | selected save slot | 0-128 | item data + 0x2e, clears refill byte +0x33 | ALTTPSRAMEditor magic setter |
| equipment enums | Bow, swords, shields, mail, gloves, flute/shovel, boomerang, mushroom/powder | enum | selected save slot | documented item values only | item data + 0x00..0x1b | ALTTPSRAMEditor constants |
| inventory booleans | Hookshot, rods, medallions, lamp, hammer, net, book, canes, cape, mirror, boots, flippers, moon pearl | boolean | selected save slot | off/on only | item data + specific byte, boots/flippers also update ability flags | `zelda3` item receipt sets boots/flippers ability masks 0x04/0x02 |
| pendants/crystals | Quest rewards | bitmask | selected save slot | known reward bits only | item data + 0x34 and +0x3a | ALTTPSRAMEditor pendant/crystal constants |

## 5. Presets
- maxResources: sets rupees, bombs, arrows, upgrades, and magic to safe maxima.
- maxHealthMagic: sets 20 hearts, clears partial heart pieces, refills magic, and enables quarter magic.
- fullAdventureKit: combines major equipment, inventory items, pendants, crystals, bottles, and resources.

## 6. Required Backend Logic
- existing editor can be reused: no
- new parser/editor needed: yes, `alttp-sram`
- checksum repair required: yes, per edited slot
- mirrored-write required: yes, primary and mirror slot are both written after each edit

## 7. Verification
- before edits: require a valid region marker and checksum in the primary slot or mirror.
- after edits: recompute checksum, write primary and mirror slot, and reparse.
- parser/tool confirmation: Go unit tests cover read/apply, mirror repair, endpoint exposure, and new-save versioning.

## 8. Open Questions
- Dungeon maps, compasses, big keys, per-room events, overworld events, and starting-point story state are intentionally not exposed yet.
- Randomizer-expanded SRAM is not supported in this pack.

## 9. Decision
- code-ready and implemented as a parser-backed pack.
