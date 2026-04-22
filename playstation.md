# PlayStation Sync Contract for Helper Developers

This document describes how RetroSaveManager currently handles PlayStation save sync in the backend, how PS1/PS2 memory cards are stored, and what helper applications must send to stay compatible.

Last updated: 2026-04-22 15:21:30 CEST

## Latest update: 2026-04-22 15:21:30 CEST

The current PlayStation contract was clarified and tightened on April 22, 2026 at 15:21:30 CEST.

What changed in practice:

- The web UI no longer treats `Memory Card 1` or `Memory Card 2` as the primary user-facing object.
- PS1 and PS2 are now presented in the GUI primarily as individual logical game saves.
- A single PlayStation projection `saveId` can still back multiple logical game rows internally.
- Because of that, any per-game PlayStation action must be scoped by the pair `saveId` + `psLogicalKey`.
- `saveId` alone identifies the rebuilt projection/card artifact line.
- `psLogicalKey` identifies the specific logical game save inside that projection line.

This matters for all PlayStation per-game operations:

- details
- rollback
- delete
- per-game web download

It does **not** change helper upload/download semantics:

- helpers still upload full cards
- helpers still receive full rebuilt cards from helper sync endpoints
- the backend still does extraction, logical-save storage and projection rebuilds

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

Important implementation detail for tool authors:

- the same PlayStation projection `saveId` may appear on more than one logical game row
- this is expected and does not mean there are duplicate cards in storage
- the real per-game key is `psLogicalKey`
- any PlayStation UI/tool route that targets one specific game must preserve `psLogicalKey`

In other words:

- `saveId` = projection/card lineage
- `psLogicalKey` = individual logical save lineage

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

### Web routing rule
- when the web app opens a PlayStation save detail page, it must include `psLogicalKey`
- when the web app triggers PlayStation rollback/delete/download for one game, it must include `psLogicalKey`
- otherwise the request falls back to projection/card context instead of individual game context

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

### Does a PlayStation `saveId` always mean one game?
No.

For PlayStation, a `saveId` is primarily the rebuilt projection/card artifact identity.
One projection can contain multiple logical game saves.
The per-game identity is `psLogicalKey`.

## 20. Recommended next helper updates

For helper programmers, the safe next step is:

1. Ensure PS uploads always send authenticated helper identity.
2. Ensure `device_type` is exactly one of the supported values.
3. Ensure `slotName` is explicit and stable.
4. Stop trying to pre-merge or pre-extract PlayStation saves client-side.
5. Treat backend downloads as authoritative rebuilt projections.

If we later broaden runtime support, that should happen by adding explicit runtime profiles in the backend, not by making helper logic fuzzy.

## 21. Coordination Note (Session 019dafb9-4ddc-7fc3-b0bd-d341c69f1f11)

Date: 2026-04-22

Purpose: keep backend/web/helper work on one contract line without drift.

Leadership split:

- Contract lead: session `019dafb9-4ddc-7fc3-b0bd-d341c69f1f11` (this file + backend behavior).
- Helper execution lead: SGM helper implementation thread (MiSTer/SteamDeck/Windows clients).
- Tie-breaker: this `playstation.md` document. If behavior and helper code differ, helper code must be aligned to this file.

Current alignment status from helper side:

- Done: helper now sends stable helper identity and auth fields for PlayStation flows (`device_type`, `fingerprint`, `app_password`).
- Done: helper uses explicit PS slot (`Memory Card 1`) when slot is default.
- Open mismatch: helper currently performs client-side PS1 card injection/merge on download to preserve local card continuity.

Decision needed from contract lead (please confirm in this section):

1. Is any client-side merge/injection allowed for helper sync?
   - Current contract text says no (backend projection is authoritative).
2. If preserving local-only entries is desired, should this be backend-driven via projection logic only?
3. Idempotency target for helper runs:
   - expected steady state is `in_sync` with no repeated PS1 download/upload loops.

Confirmed by contract lead: 2026-04-22 15:30:41 CEST

Confirmed decisions:

1. Client-side merge/injection for PlayStation helper sync is not allowed.
   - Helpers must not inject, merge, or preserve card contents client-side after download.
   - The backend-generated projection is authoritative.
2. If preserving local-only entries is ever desired, it must be implemented backend-side only.
   - That logic belongs in projection/import policy on the server, not in helper code.
   - Helpers should stay dumb and deterministic for PlayStation card state.
3. Idempotency target is confirmed as steady-state `in_sync`.
   - A clean helper run should converge without repeated PS1 download/upload churn.
   - Repeated loops indicate contract drift, stale local state handling, or a backend conflict path that must be fixed centrally.

Implementation rule after confirmation:

- Once decision is confirmed here, helper code will be normalized to exactly match it across all helper targets.

## 22. Backend Incident Note: Invalid PS1 projection returned by `/saves/download`

Date: 2026-04-22 (CEST)

Status: confirmed backend-side projection integrity bug (not a helper-side merge/injection issue).

### What was observed

1. KNULLI local PS1 card is valid raw card (`131072` bytes).
2. The helper downloads a PS1 projection from backend for:
   - `romSha1=ps-line:psx:retroarch:memory-card-1`
   - `slotName=Memory Card 1`
   - `device_type=retroarch`
3. Downloaded payload is also `131072` bytes, but fails strict PS1 raw validation:
   - Frame 0 starts with `MC` but checksum byte does not match XOR.
   - Frame 63 does not contain trailing `MC` marker.
4. Helper therefore rejects restore with:
   - `canonieke PS1 save is ongeldig en kan niet worden teruggezet naar lokaal formaat`

### Hard evidence captured

1. Local KNULLI card:
   - SHA256: `ec80805978cf36a23db52b1678e86e06bce427b48cd61907c73002e69111f0ef`
   - Frame 0 checksum: valid (`calc=14`, `stored=14`)
   - Frame 63 marker+checksum: valid (`MC`, checksum ok)
2. Backend downloaded projection:
   - SHA256: `a377a18653868397721e830d4f8ede9bc06ed00e33cf4dc81a7ab7f3e0dc622a`
   - Frame 0 checksum: invalid (`calc=14`, `stored=0`)
   - Frame 63 marker: invalid (`00 00`, not `MC`)
3. Re-uploading the valid local card still yields the same invalid projection SHA on download.
   - This confirms corruption happens in backend projection/build path (or post-import normalization), not in helper upload.

### Reproduction (minimal)

1. Upload valid PS1 raw card with:
   - `rom_sha1=ps-line:psx:retroarch:memory-card-1`
   - `slotName=Memory Card 1`
   - `device_type=retroarch`
   - `fingerprint=<device>`
2. Fetch latest via `GET /save/latest`.
3. Download via `GET /saves/download`.
4. Validate PS1 raw:
   - size must be `131072`
   - frame 0 must start with `MC` and checksum must match
   - directory frames `1..15` checksums must match
   - frame 63 must start with `MC` and checksum must match
5. Current backend output fails step 4.

### Required backend fix

1. In PS1 projection builder, always emit canonical raw card with:
   - frame 0 `MC` signature + valid checksum
   - frame 63 `MC` signature + valid checksum
   - valid checksums for all written directory frames
2. Ensure checksum recalculation happens after every projection rewrite step, not before final serialization.
3. Add backend contract test:
   - input: valid PS1 card upload
   - output from `/saves/download` must pass raw validator and remain restorable by helper.
4. Backfill existing broken PS1 projections by reprojecting affected records.

### Acceptance criteria for backend developer

1. Two consecutive helper `sync` runs on the same device converge to `in_sync`.
2. No PS1 download/upload loop for unchanged cards.
3. Helper no longer emits invalid canonical PS1 error for valid PS1 line records.

## 23. Backend Incident Note: Dreamcast uploads rejected with `422` (helper-side validation already active)

Date: 2026-04-22 (CEST)

Status: confirmed backend-side format acceptance gap for Dreamcast payloads.

### What was observed

1. Updated helper build recognizes Dreamcast saves on KNULLI:
   - `*.A1.bin`, `*.B1.bin`, `*.C1.bin`, `*.D1.bin` (128KB VMU images)
2. Helper strict validation succeeds and classifies these as Dreamcast:
   - evidence example: `path hint sega + .bin (131072 bytes) [vmu-bin entries=0 icons=0 title=- app=-]`
3. `dc_nvmem.bin` is rejected client-side as intended (NVRAM blob, not a VMU save container).
4. Upload request then fails server-side with:
   - `422 Unprocessable Entity`
   - message: `unsupported or unrecognized save format; only known consoles/arcade are allowed`

### Reproduction (minimal)

1. On KNULLI, keep source path that includes `/userdata/saves/dreamcast/*.A1.bin`.
2. Run helper sync:
   - `./sgm-mister-helper --config ./config.ini sync --verbose`
3. Observe:
   - Dreamcast files are detected by helper and attempted for upload.
   - Backend responds `422` with unsupported-format message.

### Required backend fix

1. Extend backend save classifier/validator to accept Dreamcast VMU-family payloads:
   - raw VMU image (`.bin`, 128KB)
   - VMS package (`.vms`)
   - DCI dump (`.dci`)
2. Add Dreamcast as a supported console slug in backend normalization path (`dreamcast`).
3. Preserve helper-provided Dreamcast line identity (`dc-line:<system>:<device_type>:<slot>`) in latest/conflict flow.
4. Add contract tests:
   - upload valid VMU `.bin` -> accepted
   - upload known-invalid `dc_nvmem.bin` style blob -> rejected
   - `/save/latest` + `/saves/download` roundtrip for Dreamcast converges to `in_sync`

### Acceptance criteria for backend developer

1. Dreamcast VMU uploads from helper return success (no `422` unsupported format).
2. `dc_nvmem.bin` remains rejected.
3. Two consecutive helper sync runs on unchanged Dreamcast cards converge to `in_sync`.
4. `/saves/download` payload for PS1 passes structural validator every time.
