# PlayStation Sync Contract for Helper Developers

This document describes how RetroSaveManager currently handles PlayStation save sync in the backend, how PS1/PS2 memory cards are stored, and what helper applications must send to stay compatible.

The short version:

- Helpers do **not** pre-extract PS save entries.
- Helpers upload the **full memory card image**.
- The backend parses the card, extracts logical saves, stores those as the sync truth, and rebuilds full cards per supported runtime profile.
- For helper compatibility, helper download/latest endpoints still behave like **memory-card sync**, not loose file sync.
- The web UI can now show PS saves **per individual game entry**, but helper sync remains **card upload / card projection download**.

## 1. Current backend truth model

RetroSaveManager no longer treats the uploaded PS memory card blob as the long-term source of truth.

The backend now keeps four layers:

1. `ps_import_artifact`
- Raw uploaded card blob.
- Kept for audit/debug/forensics.
- Not used as the primary sync truth after ingest.

2. `ps_logical_save`
- Canonical logical save extracted from a PS card entry/directory.
- This is the real cross-runtime sync truth.
- Version history lives here.

3. `ps_projection`
- Rebuilt full memory card for a concrete runtime profile and card slot.
- This is what helpers download.

4. `ps_tombstone`
- Logical delete marker.
- Removes a logical save from future rebuilt projections without needing to purge its full history immediately.

## 2. What helpers should upload

### PS1
Upload the full PS1 memory card image.

Supported card formats currently recognized by the backend:

- `.mcr`
- `.mcd`
- `.mc`
- `.gme`
- `.vmp`
- `.psv`

Important:

- Upload the **entire card file**, not a single extracted save.
- The backend will parse directory entries, title, product code, region, icon and block chain.
- If the payload is not a real PS1 card image, the backend rejects it.

### PS2
Upload the full PS2 memory card image.

Supported runtime/profile today:

- `PCSX2` with `.ps2` cards

Important:

- Upload the **entire `.ps2` card**, not an extracted save directory.
- The backend parses root directories, `icon.sys`, title lines, directory product code, files and icons.
- System/config directories like `BADATA-SYSTEM` are not treated as user savegames.

## 3. What helpers should NOT do

Helpers should **not**:

- extract PS1 entries themselves before upload
- extract PS2 save directories themselves before upload
- merge card contents client-side
- try to do fuzzy title matching
- try to decide cross-runtime conflict resolution client-side
- upload save states/config files as if they were memory cards

That work is server-side now.

## 4. Supported runtime profiles

Current supported PlayStation runtime profiles are hard-coded and explicit.

### PS1
- `mister` -> `psx/mister`
- `retroarch` -> `psx/retroarch`

### PS2
- `pcsx2` -> `ps2/pcsx2`

Unknown PlayStation runtimes are rejected.

There is deliberately no generic fallback profile.

## 5. What the backend does on upload

When a helper uploads a PlayStation memory card to `/saves`, the backend does this:

1. Authenticates the helper/device.
2. Detects whether the payload is a real PS1 or PS2 memory card image.
3. Maps the helper runtime to a supported backend runtime profile.
4. Derives the card slot (`Memory Card 1` or `Memory Card 2`).
5. Parses the card into logical saves.
6. Upserts those logical saves into the PlayStation sync store.
7. Detects tracked deletes/conflicts.
8. Rebuilds full runtime-specific card projections.
9. Stores those projections as normal save records so helper endpoints stay compatible.

## 6. Slot naming requirements

Helpers must send an explicit slot identity.

Accepted slot patterns are things that clearly resolve to:

- `Memory Card 1`
- `Memory Card 2`

Examples that are accepted today:

- `slotName=Memory Card 1`
- filename `memory_card_1.mcr`
- filename `Mcd001.ps2`

If the backend cannot confidently derive `Memory Card 1` or `Memory Card 2`, the upload is rejected for PlayStation sync.

## 7. Helper request fields

For PlayStation helper uploads, the important multipart fields are:

- `file`
- `device_type`
- `fingerprint`
- `app_password`
- `slotName`
- optional `system`
- optional `rom_sha1`
- optional `rom_md5`

### Required in practice

`file`
- The full PS memory card image.

`device_type`
- Used to map the helper to a supported runtime profile.
- Current valid values for PlayStation sync:
- `mister`
- `retroarch`
- `pcsx2`

`fingerprint`
- Stable device identity.

`app_password`
- Helper authentication.

`slotName`
- Must resolve to `Memory Card 1` or `Memory Card 2`.

### Optional

`system`
- Not the primary source of truth for PlayStation cards.
- Real payload/card detection is the deciding factor.

`rom_sha1`
- Not the primary identity for PlayStation card sync.
- PlayStation sync identity is card/profile/slot based, then logical-save based after extraction.

## 8. Example helper upload

```bash
curl -X POST "http://YOUR-RSM-HOST/saves" \
  -H "Authorization: Bearer YOUR-APP-PASSWORD" \
  -F "device_type=mister" \
  -F "fingerprint=mister-psx-living-room" \
  -F "slotName=Memory Card 1" \
  -F "system=psx" \
  -F "file=@memory_card_1.mcr"
```

Example for PS2 / PCSX2:

```bash
curl -X POST "http://YOUR-RSM-HOST/saves" \
  -H "Authorization: Bearer YOUR-APP-PASSWORD" \
  -F "device_type=pcsx2" \
  -F "fingerprint=pcsx2-desktop" \
  -F "slotName=Memory Card 1" \
  -F "system=ps2" \
  -F "file=@Mcd001.ps2"
```

## 9. What `/save/latest` and `/saves/download` mean for helpers

For PlayStation helpers, these endpoints remain card-oriented.

### `/save/latest`
For a helper/runtime/card-slot line, this resolves to the latest rebuilt card projection for that runtime profile.

That means:

- PS1 MiSTer asks for PS1 MiSTer card line
- PS1 RetroArch asks for PS1 RetroArch card line
- PS2 PCSX2 asks for PS2 PCSX2 card line

### `/saves/download`
For helper PlayStation sync, download returns the rebuilt **full memory card projection**, not a single logical save.

This is important:

- helper sync stays card-based
- backend truth is logical-save based
- backend projection is how compatibility is preserved

## 10. What changed for the web UI

The web UI is now allowed to work per logical PS save entry.

That means the frontend can now:

- show PS1/PS2 games individually on the dashboard
- open details per individual PS save entry
- download a single logical PS save for user inspection/export
- delete a single logical PS save entry from active projections
- rollback a single logical PS save entry to an older revision

This is **not** the same as helper sync behavior.

Helpers still upload/download full cards.

## 11. PS1 logical save extraction details

For PS1, the backend extracts logical saves from confirmed directory start entries and their block chains.

Per logical save, the backend extracts/stores:

- display title
- product code
- region code
- icon preview
- slot index
- block count
- raw directory entries
- raw block payloads

Portable PS1 saves are keyed by:

- `system=psx`
- `productCode`
- normalized title
- region

If product code is missing, the entry is treated as non-portable and stays scoped to the projection/card line.

## 12. PS2 logical save extraction details

For PS2, the backend scans root directories and only treats real save directories as user saves.

A directory is considered a real save when the backend can read a valid `icon.sys` and parse save metadata.

Per logical save, the backend extracts/stores:

- directory name
- title from `icon.sys`
- product code from directory naming
- region code
- icon/preview when available
- full file tree for that logical save

Portable PS2 saves are keyed by:

- `system=ps2`
- directory product code
- normalized title
- region

If product code cannot be confirmed, the save remains non-portable and card-line scoped.

## 13. Delete behavior

Delete in the PS domain is no longer “delete the whole card” when triggered from the per-game UI path.

Per-game delete now means:

- backend writes a tombstone for that logical save
- affected card projections are rebuilt
- the game disappears from active card projections
- helper downloads after that receive the rebuilt card without that save

The old raw import artifact stays available internally for audit/debug.

## 14. Rollback behavior

Per-game rollback now means:

- choose a historical logical-save revision
- backend creates a new latest logical revision from that old revision
- affected projections are rebuilt
- helper downloads receive new full cards containing the rolled-back logical save content

This is not a raw old-card restore.
It is a logical-save promote/copy into a new latest state.

## 15. Download behavior for web vs helper

### Helper download
- returns full card projection
- format stays runtime/card compatible

### Web per-game download
- PS1: a single-save card image is generated
- PS2: a zip archive of the logical save directory is generated

This split is intentional.

## 16. Storage layout under `SAVE_ROOT`

All PlayStation state still lives under the same main save root so one backup root remains enough.

PlayStation internals live in:

```text
SAVE_ROOT/_rsm/playstation/
```

Inside that, the backend stores state for:

- imports
- logical saves
- projections
- tombstones
- device line state

User-visible save records still live under the main save layout alongside other systems.

## 17. Compatibility rules for helper developers

If you are maintaining a helper, assume these rules:

1. Upload full PS cards, never extracted entries.
2. Always send stable `device_type` and `fingerprint`.
3. Always send explicit `slotName` resolving to `Memory Card 1` or `Memory Card 2`.
4. Expect the backend to return full rebuilt cards for helper sync endpoints.
5. Do not assume the uploaded blob is what comes back later; the backend may rebuild it from logical saves.
6. Do not assume unknown PlayStation runtimes will be accepted.
7. Do not send save states/config/data blobs as PS cards.

## 18. Current supported helper behavior summary

### MiSTer PS1 helper
- Upload full PS1 card
- Backend extracts logical saves
- Backend also builds RetroArch-compatible PS1 projection

### RetroArch PS1 helper
- Upload full PS1 card
- Backend extracts logical saves
- Backend also builds MiSTer-compatible PS1 projection

### PCSX2 PS2 helper
- Upload full PS2 card
- Backend extracts logical saves
- Backend rebuilds PS2 `.ps2` card projection for PCSX2

## 19. Answer to the main helper question

### Should the helper already extract the saves before upload?
No.

### Should the helper upload the full MiSTer memory card to the backend?
Yes.

### Does the backend do the extraction?
Yes.

### Should helper sync stay memory-card based?
Yes.

### Can the web UI now work per individual game on a card?
Yes.

That split is the current contract.

## 20. Recommended next helper updates

For helper programmers, the safe next step is:

1. Ensure PS uploads always send authenticated helper identity.
2. Ensure `device_type` is exactly one of the supported values.
3. Ensure `slotName` is explicit and stable.
4. Stop trying to pre-merge or pre-extract PlayStation saves client-side.
5. Treat backend downloads as authoritative rebuilt projections.

If we later broaden runtime support, that should happen by adding explicit runtime profiles in the backend, not by making helper logic fuzzy.
