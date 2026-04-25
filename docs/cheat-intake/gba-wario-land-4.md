# GBA - Wario Land 4

## Decision

Wario Land 4 needs a runtime game support module. The existing backend cheat editors are game-specific for N64/SNES titles and do not include a reusable GBA save-block editor with Wario Land 4 checksum repair.

Delivered as:

- `modules/gba-wario-land-4.rsmodule.zip`

No standalone YAML-only pack was added under `cheats/packs/`.

## Sources Used

- WarioWiki: [Wario Land 4 Save File Structure](https://wario.fandom.com/wiki/Wario_Land_4_Save_File_Structure)
- TASVideos: [User file 638204818075722537](https://tasvideos.org/UserFiles/Info/638204818075722537), a BizHawk movie for `Wario Land 4 (USA, Europe)` that includes `MovieSaveRam.bin`.

The TASVideos sample header identifies `Wario Land 4 (USA, Europe)` and ROM SHA1 `B9FE05A8080E124B67BCE6A623234EE3B518A2C1`. The sample was used only as public save-byte evidence for magic markers, mirrored save-file blocks, and checksum behavior.

## Structural Evidence

The module requires:

- Raw GBA save size of 32768, 65536, or 131072 bytes.
- Wario Land 4 global header marker `AGBWarioLand-USver00`.
- Selected-save marker `WAR4SAVEAWARABGSELECTSAVE`.
- At least one valid save-file block for File A or File B.
- Save-file block marker `AutoSAVEWar4key1`, end marker `SAVE_END`, secondary marker `key2AutoSAVEWar4`, and a valid checksum pair.

Checksum handling was verified against the TASVideos sample. Header and save-file blocks use little-endian 32-bit word sums excluding the checksum pair; the checksum is the body sum minus one, and the companion word is its bitwise complement. The module rebuilds this pair after edits and mirrors the edited slot to its primary and backup save blocks.

## Safely Supported Fields

- Save selector: File A and File B.
- Hard Mode beaten global unlock state.
- Stage collectible bits for Hall of Hieroglyphs, all passage stages, and Golden Passage:
  - Top Right Gem Piece
  - Bottom Right Gem Piece
  - Bottom Left Gem Piece
  - Top Left Gem Piece
  - CD
  - Keezer
- Boss beaten state for Spoiled Rotten, Cractus, Cuckoo Condor, Aerodent, and Catbat.
- Boss treasure chest count for Cractus, Cuckoo Condor, Aerodent, and Catbat.
- Last-visited-stage passage ID, stage number, and map step counter.
- Documented high-score fields for Hall of Hieroglyphs, all passage stages, and Golden Passage.

High scores are exposed as displayed scores. The save stores these values divided by 10, so the module only accepts multiples of 10.

Additional decomp/sanity evidence after the initial module:

- The TASVideos SaveRAM has repeated ordinary-stage values of `0x2f` and `0x3f` at the documented stage flag offsets. Those decode exactly as the documented Hall of Hieroglyphs bits: four gem pieces, optional CD, and Keezer. This supports applying the same six-bit collectible schema to all normal stages.
- The boss chest table is treated as a low two-bit treasure count (`0..3`), not independent chest bits. That resolves the public table's `0x03` third-chest entry while preserving any non-chest boss bits.
- Last-visited blocks use their own 32-byte checksum pair: the checksum is the little-endian 32-bit body sum excluding the checksum pair, and the companion word is its bitwise complement. This differs from the main save-file block checksum, which stores body sum minus one.

## Intentionally Not Supported

- Unknown or padding fields.
- Raw offset writes or arbitrary bitmask writes.
- Golden Diva boss state/chests; the real SaveRAM sample uses a divergent `0x10` value at the documented field, so the safe bit semantics are not proven yet.
- Last-visited reload-point text rewriting; the module preserves the existing text token because the complete token table is not documented.
- Filename-only matching.
