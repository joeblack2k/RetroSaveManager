# Cheat Intake: Wave Race 64

## 1. Identity
- systemSlug: n64
- canonicalTitle: Wave Race 64
- regions: JP/US/EU (`NWRJ`, `NWRE`, `NWRP`)
- extensions: `.eep`, `.bin`, `.cpk`, `.mpk`, canonical 32768-byte Controller Pak `.srm`
- runtimes: N64 EEPROM and Controller Pak projections
- saveModel: 512-byte on-cartridge EEPROM for options/progress/settings/records; Controller Pak PFS note for 0x200-byte Wave Race payload/ghost data.

## 2. Samples
| file | size | sha256 | runtime | notes |
|---|---:|---|---|---|
| Wave Race 64 (USA) (Rev A).eep | 512 | 4fbb09a048d743d1122b52c27a24cfb2ac35ca14bd0a594d968917fd26b9b533 | ED64M save archive | Starts `54 45 a2 3d`; checksum over `0x04-0x1FF` is `0xa23d`. |
| Wave Race 64.eep | 512 | 8c50df0b776cd95938de1f5a0b012ce13562ae03740bef65918b81628a057e5e | ED64M save archive | Starts `54 45 c8 9b`; checksum over `0x04-0x1FF` is `0xc89b`. |

Controller Pak write support is verified against a generated valid N64 PFS fixture containing a Wave Race note. I did not find an archival Wave Race ghost `.cpk` sample in this pass, so the module supports the proven 0x200-byte PFS payload structurally and does not invent ghost-stream semantics.

## 3. Format Evidence
- EEPROM header: bytes `0x00-0x01` are `TE`; save read rejects mismatched magic.
- EEPROM checksum: decomp `Save_GenCheckSum` sums bytes `0x04-0x1FF`; write paths store the big-endian 16-bit result at `0x02-0x03`.
- progress/options: save code persists progress bytes at `0x08-0x0B` and options byte at `0x0C`.
- rider names: defaults and editor copy routines prove four editable rider names; samples show four 10-byte ASCII slots at `0x10-0x37`.
- condition matrix: decomp writes `D_801CB308[3][3]` to EEPROM `0x50-0x58`; gameplay reads wave mode, buoy miss limit, and lap count by difficulty.
- records: decomp record packers use 6-byte, 5-byte, and 24-bit encodings; samples show stable packed ranges at `0x60-0x11F`, `0x120-0x1A6`, `0x1A8-0x1D7`, and `0x1D8-0x1FF`.
- Controller Pak: decomp calls `osPfsFindFile`/delete with company code `1` and game code `0x4E57524A` (`NWRJ`), and requires `0x200` free bytes for the file. The module accepts `NWRJ`, `NWRE`, and `NWRP` notes with Nintendo publisher code.
- sources:
  - Data Crystal: https://datacrystal.tcrf.net/wiki/Wave_Race_64
  - Nintendo PAL manual PDF: https://www.nintendo.com/eu/media/downloads/games_8/emanuals/nintendo_8/Manual_Nintendo64_WaveRace64_EN.pdf
  - LLONSIT decomp commit `3c86d610f974bd0b3f0af1003cc16a74bf54928e`, especially `wr64_save.c`, `code_4C750.c`, `structs.h`, and rider/options overlays.

## 4. Editable Fields
| field group | type | safe values | read/write location | proof |
|---|---|---|---|---|
| audio | enum/boolean | stereo, mono, headphones; music on/off | `0x0C`, bits `0xC0` and `0x20` | option load/write code maps these bits. |
| championship/progress bytes | integer | `0..255` | `0x08-0x0B` | save code copies four persisted progress bytes. |
| rider names | enum chars | end, space, A-Z, dash, dot | four 10-byte slots at `0x10-0x37` | defaults, editor copy length, and sample layout. |
| course/condition settings | enum/integer | wave `0..3`, buoy misses `0..5`, laps `1..9` | `0x50-0x58` | decomp reads/writes `D_801CB308[3][3]`. |
| time-trial packed records | integer bytes | `0..255` | `0x60-0x11F`, 32 x 6-byte blocks | record packer and sample block layout. |
| course packed records | integer bytes | `0..255` | `0x120-0x1A6`, 27 x 5-byte blocks | `func_8007C9D4` writes packed 5-byte groups. |
| stunt scores / 24-bit values | integer | `-1` empty, otherwise `0..16777214` | `0x1A8-0x1D7`, 16 x 3-byte values | 24-bit encode/decode helpers treat `0xFFFFFF` as empty. |
| extra packed records | integer bytes | `0..255` | `0x1D8-0x1FF`, 8 x 5-byte blocks | sample/decomp packed record tail. |
| Controller Pak ghost payload | integer bytes | `0..255` | Wave Race PFS note, first `0x200` bytes | PFS identity and allocation size from decomp; PFS structure from backend `pakfs`. |

## 5. Presets
- none. No "max all" or invented gameplay presets are shipped.

## 6. Required Backend Logic
- existing editor can be reused: no.
- new parser/editor needed: yes, supplied as `modules/wave-race-64.rsmodule.zip`.
- module version: `1.1.1`, including the live catalog title alias `Wave Race 64 - Kawasaki Jet Ski`.
- checksum repair required: EEPROM only, rebuild `0x02-0x03`.
- Controller Pak repair required: no metadata rewrite for byte edits; the module preserves PFS notes, inode chains, and unrelated entries.

## 7. Verification
- module import/read/apply smoke test passed for:
  - real ED64M EEPROM sample: read audio/progress/conditions/records and apply representative edits with checksum repair.
  - generated valid Controller Pak PFS fixture: detect Wave Race note, read ghost bytes, apply one ghost byte while preserving pak size.
- required validation commands are listed in the handoff/final response.

## 8. Open Questions
- Packed record bytes are editable, but course/rider/initial/time labels are not decoded into friendly record rows yet.
- Controller Pak ghost bytes are editable as the proven 0x200-byte payload; the ghost stream is not decoded into route, input, or replay-frame semantics without a real ghost corpus.
- Rider watercraft tuning bytes adjacent to the name area are preserved but not exposed because the exact EEPROM packing from RAM struct to save bytes is not fully proven.

## 9. Decision
- code-ready / module-backed. Ship the expanded module with EEPROM and Controller Pak packs, no backend Go changes.
