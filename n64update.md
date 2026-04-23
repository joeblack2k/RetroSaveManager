# N64 Sync Contract for Helper Developers

This document describes the current Nintendo 64 backend contract in RetroSaveManager.

It is the single helper-facing source of truth for:

- what helpers upload
- what the backend stores as canonical N64 truth
- how helpers request the correct runtime-specific file back
- which runtime profiles are supported right now

Last updated: 2026-04-23 21:36 CEST

## 1. Short version

RetroSaveManager now handles N64 the same way PlayStation is handled:

- helpers upload the real local N64 save file from their own runtime
- the backend normalizes that upload into one canonical internal N64 save
- helpers must explicitly ask for the target runtime format they need back
- the backend returns a projected runtime-specific file for that requested target

Important:

- helpers do **not** pre-convert N64 saves before upload
- helpers do **not** guess another emulator format client-side
- helpers must always send `n64Profile` for N64 helper sync requests
- `device_type` is helper identity, not N64 runtime truth

## 2. Current supported N64 runtime profiles

The backend currently supports these explicit N64 profiles:

- `n64/mister`
- `n64/retroarch`
- `n64/project64`
- `n64/mupen-family`
- `n64/everdrive`

Notes:

- `n64/mupen-family` is the shared tranche-1 profile for Mupen64Plus and Rosalie's Mupen GUI (RMG).
- Unknown N64 runtimes are rejected.
- There is no fallback profile.

## 3. Canonical backend truth model

The backend does **not** keep runtime-specific container bytes as the sync truth.

Instead, it converts uploads into canonical internal N64 media bytes per media class:

- `eeprom`
- `sram`
- `flashram`
- `controller-pak`

That canonical media becomes the cloud truth.

Downloads are then built from that canonical truth into the requested runtime profile.

This means:

- MiSTer upload -> RetroArch download is supported
- RetroArch upload -> Project64 download is supported
- Project64 upload -> EverDrive download is supported
- equivalent N64 saves from multiple runtimes converge on the same backend save line when the validated game, region, media class and `rom_sha1` match

## 4. Strict N64 ingest rules

The backend is strict on N64.

Current accepted native media forms are:

- `.eep`
- `.sra`
- `.fla`
- `.mpk`

Current native size gates are:

- EEPROM: `512` or `2048` bytes
- SRAM: `32768` bytes
- FlashRAM: `131072` bytes
- Controller Pak: `32768` bytes

Current hard rules:

- blank all-`0x00` payloads are rejected
- blank all-`0xFF` payloads are rejected
- executable/archive-looking payloads are rejected
- mostly-text payloads are rejected
- generic `.sav` fallback is not allowed
- filename-only game trust is not allowed
- if the backend cannot safely normalize the N64 upload, it rejects the upload

## 5. Upload contract

Helpers upload to:

- `POST /saves`
- `POST /v1/saves`

For N64 helper uploads, the multipart form must include:

- `file`
- `app_password`
- `system=n64`
- `rom_sha1`
- `slotName`
- `device_type`
- `fingerprint`
- `n64Profile`

### Field meanings

`file`
- The real local save file as it exists on that helper runtime.
- Do not pre-convert it.

`app_password`
- Helper authentication.

`system`
- Must be `n64`.

`rom_sha1`
- Trusted ROM identity from the helper side.
- This remains required for correct track identity.

`slotName`
- Stable logical slot name used by the helper.
- Keep this stable for the same save line.

`device_type`
- Helper identity and device metadata.
- This is **not** the source of truth for N64 projection format.

`fingerprint`
- Stable device identity.

`n64Profile`
- Required for N64 helper sync.
- This tells the backend what format the uploaded file is currently in.

## 6. Download and latest contract

Helpers must also send `n64Profile` when asking for N64 data back.

### Latest lookup

Helpers call:

- `GET /save/latest`
- `GET /v1/save/latest`

Required N64 query fields:

- `romSha1`
- `slotName`
- `device_type`
- `fingerprint`
- `n64Profile`

For N64, missing `n64Profile` is a request error in helper mode.

### Download

Helpers call:

- `GET /saves/download?id=...`
- `GET /v1/saves/download?id=...`

Required N64 query fields in helper mode:

- `id`
- `device_type`
- `fingerprint`
- `n64Profile`

For downloads, `n64Profile` means:

- the local runtime that must consume the returned file
- not necessarily the runtime that originally uploaded the cloud version

That distinction is important.

Example:

- MiSTer uploads `.sra`
- RetroArch helper asks for latest/download with `n64Profile=n64/retroarch`
- backend returns RetroArch-style `.srm`

## 7. Profile behavior in tranche 1

### `n64/mister`

Expected upload forms:

- `.eep`
- `.sra`
- `.fla`
- `.mpk`

Download behavior:

- returns native canonical media bytes
- filename extensions remain native: `.eep`, `.sra`, `.fla`, `.mpk`

### `n64/retroarch`

Expected upload forms:

- combined N64 `.srm`

RetroArch `.srm` handling:

- backend splits the combined `.srm` into one canonical N64 media payload
- upload is rejected if the `.srm` cannot be safely normalized

Download behavior:

- backend builds a full RetroArch combined `.srm`
- output filename uses `.srm`

### `n64/project64`

Expected upload forms in tranche 1:

- `.eep`
- `.sra`
- `.fla`
- `.mpk`

Download behavior:

- returns Project64-compatible projected bytes
- SRAM and FlashRAM are projected with the required 32-bit word swap policy
- filename extensions remain `.eep`, `.sra`, `.fla`, `.mpk`

### `n64/mupen-family`

Expected upload forms in tranche 1:

- `.eep`
- `.sra`
- `.fla`
- `.mpk`

Special handling:

- controller pak downloads can be projected as merged Mupen-family pak layout where needed
- SRAM and FlashRAM are projected with the required 32-bit word swap policy

Download behavior:

- filename extensions remain native for media-class downloads
- controller pak projection is Mupen-family compatible

### `n64/everdrive`

Expected upload forms in tranche 1:

- `.eep`
- `.sra`
- `.fla`
- `.mpk`
- `.srm` for EverDrive SRAM-only cases already handled by the backend

Download behavior:

- native media layout
- SRAM downloads use `.srm` when required by the profile

## 8. Concrete helper examples

Use sanitized example hosts in helper code and docs.

```bash
export RSM_BASE_URL="https://rsm.example.invalid"
export RSM_APP_PASSWORD="XXX-XXX"
```

### Example A: MiSTer uploads its real local SRAM file

```bash
curl -X POST "$RSM_BASE_URL/saves" \
  -F "app_password=$RSM_APP_PASSWORD" \
  -F "system=n64" \
  -F "rom_sha1=0123456789abcdef0123456789abcdef01234567" \
  -F "slotName=default" \
  -F "device_type=mister" \
  -F "fingerprint=mister-living-room" \
  -F "n64Profile=n64/mister" \
  -F "file=@/media/fat/saves/N64/Star Fox 64.sra"
```

### Example B: RetroArch uploads its real local `.srm`

```bash
curl -X POST "$RSM_BASE_URL/saves" \
  -F "app_password=$RSM_APP_PASSWORD" \
  -F "system=n64" \
  -F "rom_sha1=0123456789abcdef0123456789abcdef01234567" \
  -F "slotName=default" \
  -F "device_type=retroarch" \
  -F "fingerprint=retroarch-handheld" \
  -F "n64Profile=n64/retroarch" \
  -F "file=@/userdata/saves/n64/Star Fox 64.srm"
```

### Example C: Project64 helper asks whether a newer save exists

```bash
curl -s "$RSM_BASE_URL/save/latest?romSha1=0123456789abcdef0123456789abcdef01234567&slotName=default&device_type=windows-helper&fingerprint=desktop-project64&n64Profile=n64/project64" \
  -H "Authorization: Bearer $RSM_APP_PASSWORD"
```

### Example D: Mupen/RMG helper downloads the right file for its runtime

```bash
curl -L "$RSM_BASE_URL/saves/download?id=save-123&device_type=linux-helper&fingerprint=steamdeck-rmg&n64Profile=n64/mupen-family" \
  -H "Authorization: Bearer $RSM_APP_PASSWORD" \
  -o projected-save.sra
```

### Example E: EverDrive helper requests a projected download

```bash
curl -L "$RSM_BASE_URL/saves/download?id=save-123&device_type=everdrive-sync&fingerprint=cart-everdrive-64&n64Profile=n64/everdrive" \
  -H "Authorization: Bearer $RSM_APP_PASSWORD" \
  -o projected-save.srm
```

## 9. What helpers must not do

Helpers must **not**:

- infer N64 format from filename alone when sending requests
- omit `n64Profile`
- use `device_type` as a substitute for `n64Profile`
- convert a MiSTer save into RetroArch format before upload
- convert a RetroArch `.srm` into another format before upload
- upload generic `.sav` files as N64 saves
- upload blank initialized media as if it were a real save

The backend is responsible for normalization and projection.

## 10. Runtime-specific response expectations

For N64 helper downloads:

- the backend may return a different filename extension than the uploaded artifact
- the returned bytes are the projected target-format bytes
- the download payload is authoritative for that requested runtime profile

For N64 latest checks:

- the returned `sha256` is the checksum of the projected target-runtime payload when `n64Profile` is present
- helpers should compare against the bytes that exist locally for that same runtime profile

## 11. Metadata now exposed by the backend

N64 save records can now carry projection-related metadata such as:

- `runtimeProfile`
- `mediaType`
- `projectionCapable`
- `sourceArtifactProfile`

These fields help explain:

- what the canonical media class is
- whether projection is available
- what profile originally uploaded the source artifact

## 12. Future direction

This tranche solves runtime projection first.

What it does **not** guarantee yet:

- full per-game semantic validation for every N64 title
- deep game-state extraction for every save
- universal support for unknown emulator wrappers

The long-term trust model remains:

- strict media validation now
- game-specific validators layered on top over time

Until a title has parser-backed game identity, the backend can still store normalized N64 media safely, but “fully trusted game identity” remains separate from raw media acceptance.

## 13. Validation status and roadmap

Projection support does not replace validation.

The backend still treats N64 trust as a layered problem:

1. prove the payload is real N64 save media
2. prove which game it belongs to
3. prove the internal structure is coherent for that game

### What is already enforced

Current backend validation already rejects:

- empty payloads
- all-`0x00` payloads
- all-`0xFF` payloads
- executable or archive-like noise
- mostly-text payloads
- unsupported media extensions or sizes

This is the current strict media gate.

### What is still intentionally separate

Some N64 payloads can be structurally real but still too weak to trust as a fully validated game save.

Examples of reasons:

- sparse payload with too little in-game data
- correct media class but no parser-backed game identity yet
- title only known from filename instead of payload structure

That means:

- projection-capable does not automatically mean fully game-validated
- helper uploads can still be normalized safely while deeper game trust remains pending

### Long-term validator direction

For stronger trust, N64 titles should keep moving toward parser-backed validators that can:

- confirm exact game identity
- validate slot structure
- validate checksums or mirrors where the format supports that
- expose parser-backed metadata into `inspection`

Preferred order of trust:

1. parser-backed game validator
2. trusted helper `rom_sha1` aligned to that validator
3. only then promotion to a fully trusted canonical game line

Not enough on their own:

- filename
- folder name
- extension only
- size only

### Helper-side filtering remains useful

Helpers should continue doing cheap prefiltering for obvious blank native N64 media before upload.

But the backend remains authoritative for:

- final acceptance or rejection
- canonical normalization
- projection building
- validator-backed trust decisions
