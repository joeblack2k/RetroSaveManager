# Cheat Intake: The Legend of Zelda - Ocarina of Time

Last updated: 2026-04-23 CEST

Decision: `research-only`

This dossier is based on live RetroSaveManager API data from a local test deployment and format-reference material.
It is intentionally conservative.
No parser-backed cheat editor should be implemented from this evidence alone.

## 1. Identity

- `systemSlug`: `n64`
- `canonicalTitle`: `Legend of Zelda, The - Ocarina of Time`
- `regions observed`: `US`
- `variants observed`:
  - `Legend of Zelda, The - Ocarina of Time`
  - `Legend of Zelda, The - Ocarina of Time (USA) (GameCube)`
- `extensions observed`: `.sra`
- `runtime/source`: live RetroSaveManager API on the local test deployment
- `save model from docs`: 0x8000-byte SRAM with header plus three primary save files and three backups

## 2. Samples

These are the live samples that were actually downloaded:

| file | size | sha256 | source | notes |
|---|---:|---|---|---|
| `Legend of Zelda, The - Ocarina of Time (USA).sra` | 32768 | `52a30829774c64179bea474d54bbe00c5e8b12113ad82e65fd2747c68d8ae55e` | `/api/saves/save-1776776431044301599-52a30829774c/download` | Only 9 non-zero bytes in the entire payload |
| `Legend of Zelda, The - Ocarina of Time (USA) (GameCube).sra` | 32768 | `52a30829774c64179bea474d54bbe00c5e8b12113ad82e65fd2747c68d8ae55e` | `/api/saves/save-1776776431020288235-52a30829774c/download` | Byte-identical to the non-GameCube variant |

## 3. Live API Evidence

Live track ids observed on 2026-04-23:

- `save-1776776431044301599-52a30829774c`
- `save-1776776431020288235-52a30829774c`

The compatibility cheat endpoint currently reports:

- `GET /save/cheats?saveId=save-1776776431044301599-52a30829774c`
- result: `supported: false`

The two live tracks are not just similar.
They are exactly the same payload.

## 4. Structural Format Evidence

### Source references

- CloudModding OoT save format:
  - `https://wiki.cloudmodding.com/oot/Save_Format`
- ZeldaSaveTool README (byte-order aware OoT save conversion):
  - `https://github.com/xoascf/ZeldaSaveTool`

### What the format references say

CloudModding documents OoT save SRAM as:

- total size `0x8000` bytes
- header at `0x0000..0x001F`
- primary save files at:
  - `0x0020`
  - `0x1470`
  - `0x28C0`
- backup save files at:
  - `0x3D10`
  - `0x5160`
  - `0x65B0`
- validation requirements for each save file:
  - string `ZELDAZ` at offset `0x001C` within the save file
  - 16-bit checksum at offset `0x1352`

### What the live payload actually contains

Raw first 16 bytes as downloaded:

```text
98 00 00 00 5A 21 10 09 41 44 4C 45 00 00 00 00
```

This does not match the documented big-endian header directly.
However, if the file is 32-bit word-swapped, the first bytes become:

```text
00 00 00 98 09 10 21 5A 45 4C 44 41 00 00 00 00
```

That word-swapped view matches the documented header signature pattern:

- `00 00 00`
- `98 09 10 21`
- `ZELDA`

So the live sample is consistent with a word-swapped `.sra` container.

### Why the live payload is still not enough

Even after accounting for byte order:

- the payload has only 9 non-zero bytes total
- save-file region at `0x0020` is all zero
- documented save magic at save-file offset `0x001C` is absent
- documented checksum location at save-file offset `0x1352` is zero
- all three primary save blocks and all three backup blocks are effectively empty

Observed offsets from the downloaded payload:

- container offset `0x0000`: `98 00 00 00 5A 21 10 09 41 44 4C 45 ...`
- container offset `0x001C`: all zero
- container offset `0x1352`: `00 00`
- container offsets `0x3D10`, `0x5160`, `0x65B0`: all zero in sampled ranges

## 5. Editable Fields

None can be proven safely from the live dataset yet.

The public format docs describe many possible fields such as:

- name
- hearts
- health
- magic
- rupees

But the live payload does not contain a valid, populated save block that proves:

- slot presence
- region-specific byte order handling
- checksum repair against a real in-use save
- write-back behavior on this specific savedata origin

## 6. Required Backend Logic

- existing editor can be reused: `no`
- likely new editor required: `yes`
- likely requirements:
  - byte-order normalization for `.sra`
  - slot validation for three primary and three backup blocks
  - checksum repair for each edited slot
  - mirrored-write behavior to the corresponding backup block

## 7. Verification

Not yet possible from current live evidence.

Before implementation, verification should include:

- at least 2 real non-empty OoT saves, preferably 3
- before/after slot comparison
- checksum regeneration validation
- emulator or hardware load confirmation
- confirmation that edited primary and backup blocks remain accepted by the game

## 8. Open Questions

- Is the live `.sra` from an emulator that stores word-swapped SRAM by default?
- Are these two API tracks just blank initialized containers rather than real in-game save progress?
- Will the real user dataset provide non-empty primary save blocks for OoT?
- Are there region or GameCube-emulator quirks that change checksum or byte order expectations?

## 9. Decision

`research-only`

Why:

- only one unique payload was found
- that payload is effectively empty apart from header bytes
- no valid populated save slot can be proven from the downloaded data
- implementing cheats from this would require guessing field locations from docs instead of validating against real live saves
