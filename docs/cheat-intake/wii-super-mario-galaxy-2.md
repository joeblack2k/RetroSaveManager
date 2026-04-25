# Cheat Intake: Super Mario Galaxy 2

## 1. Identity
- systemSlug: wii
- canonicalTitle: Super Mario Galaxy 2
- gameId: wii/super-mario-galaxy-2
- regions: SB4E, SB4P, SB4J, SB4K are recognized structurally; embedded public templates are SB4E and SB4P only.
- extensions: bin
- runtimes: official Wii SD `data.bin`; raw decrypted `GameData.bin` for read-only inspection.
- saveModel: Wii `data.bin` export containing encrypted `GameData.bin`; raw `GameData.bin` has a header, index, and binary chunk holders.

## 2. Samples
| sample | size | sha256 | source | use |
|---|---:|---|---|---|
| gamefaqs-20695-sb4e | 75200 | 66866f9d4951ddf8dc428ee326f62a425ec0b844362bc5b301c333f2d46655d3 | GameFAQs mirror via MarioCube | exact same-region full `data.bin` template |
| gamefaqs-20827-sb4e | 75200 | a44c1e29c5365e1870b029f083029ca346570aef2af7aa67a21922d2ea0c49c4 | GameFAQs mirror via MarioCube | exact same-region full `data.bin` template |
| gamefaqs-20813-sb4p | 75200 | a1c8d2023685c4a32506ce98746956d95fa75412728a06044b5291aac77d50b5 | GameFAQs mirror via MarioCube | exact same-region full `data.bin` template |
| gamefaqs-24225-sb4p | 75200 | feca26ceeb4cc65d96acb110c4f156ee6428d5708e4db25e45fa23d5ed628584 | GameFAQs mirror via MarioCube | exact same-region full `data.bin` template |

## 3. Format Evidence
- WiiBrew documents `GameData.bin` as a header plus index, with checksum at `0x0`, version at `0x4`, entry count at `0x8`, file size at `0xC`, and 16-byte index entries.
- WiiBrew documents the checksum as a 16-bit word sum over the file after the checksum plus the inverse sum.
- `galaxy2_save_data` 0.2.0 documents SMG2 raw `GameData.bin` version `3`, max 14 entries, and file size up to `0x7FFF`.
- `galaxy2_save_data` source documents SMG2 entry names as `user*`, `config*`, and `sysconf`, with binary chunk holder version `2`.
- Luma's Workshop documents the binary chunk holder header, chunk header, and binary data content attribute table used by PLAY/GALA/config chunks. Its SMG2 section is incomplete, so SMG2 field semantics are taken from `galaxy2_save_data`, not guessed from SMG1.
- Official Wii `data.bin` exports keep `GameData.bin` encrypted, so granular gameplay fields cannot be patched safely in-place by this module.

Sources:
- https://www.wiibrew.org/wiki/Super_Mario_Galaxy_savefile
- https://www.lumasworkshop.com/wiki/GameData
- https://docs.rs/galaxy2_save_data/latest/galaxy2_save_data/
- https://repo.mariocube.com/Wii/Wii%20Saves/GameFAQs/960551-super-mario-galaxy-2/

## 4. Editable Fields
| fieldId | label | type | scope | safe values | read/write proof |
|---|---|---|---|---|---|
| templateId | Public data.bin Template | enum | whole save | `none`, exact embedded GameFAQs/MarioCube sample IDs | The parser verifies 75200-byte Wii `data.bin` structure, embedded `GameData.bin` file size `0x30A0`, SMG2 title code, and same-region template title code before replacing the full payload. |

## 5. Read-Only Fields
Raw decrypted `GameData.bin` is accepted for parser/verifier inspection only. The module validates version `3`, size `0x30A0`, checksum, index offsets, chunk-holder version `2`, and known chunk signatures before returning:
- per-user PLAY values: lives, stocked Star Bits, stocked coins, last 1-Up coin counter, current Luigi flag.
- per-user GALA aggregates: galaxy count, open galaxies, Power Star count, Bronze Star count, comet medal count, visited scenarios, Luigi ghost/standby scenario flags.
- per-user FLG1/VLE1/STF1/SSWM aggregates: event flag counts, event value counts, Hungry Luma aggregate Star Bits, coin-galaxy count, world number.
- per-config values: created flag, icon id/label, Mii choice flag, misc timestamp presence.
- sysconf values: banked Star Bits, banked Star Bits max, gifted lives.

## 6. Presets
- Public sample install presets exist only for exact full `data.bin` template replacement.
- No granular gameplay/config/Mii presets are exposed.

## 7. Required Backend Logic
- existing editor can be reused: no.
- new parser/editor needed: yes, shipped as `wii-super-mario-galaxy-2.rsmodule.zip`.
- checksum repair required: not for editable `data.bin` template replacement because the full verified export is replaced. Raw `GameData.bin` checksum is verified read-only.
- mirrored-write required: not applicable.

## 8. Verification
- Validate module and cheat YAML with `./scripts/validate-cheat-packs.sh`.
- Import module with `TestRepositoryGameModulesImport`.
- Run `cd backend && go test ./cmd/server -run 'GameModule|Cheat'`.

## 9. Open Questions
- No official Wii decryption support is included, so encrypted inner `GameData.bin` cannot be inspected inside arbitrary `data.bin` exports.
- Individual named event flags, event values, and galaxy-name labels are not exposed until a complete SMG2 label table is bundled and verified.
- No raw `GameData.bin` write support is exposed.

## 10. Decision
- code-ready module.
- Editable support is limited to exact same-region public full-save templates.
- Raw decrypted `GameData.bin` support is read-only semantic parser/verifier support.
