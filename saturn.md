# Sega Saturn Backend + Helper Contract

Last updated: 2026-04-23 14:18 Europe/Amsterdam

## Goal

This document defines the current Saturn save contract between RetroSaveManager and the SGM helpers.

The important correction is this:

- the backend is already Saturn-aware
- the remaining instability was helper-side Saturn scanning
- empty Saturn backup RAM images are not valid sync saves

## Current Reality

As of the current backend implementation, Saturn support is parser-led and strict.

Backend behavior today:

- valid Saturn backup RAM images are accepted
- empty Saturn backup RAM images are rejected
- Saturn downloads can be projected to supported target formats
- generic fallback classification is not the intended Saturn path

Concrete examples from the current fixture set:

- `Quake (USA).sav`
  - valid Saturn backup RAM image
  - accepted by backend
  - exposes Saturn metadata
  - supports `saturnFormat` download conversion
- `Fighting Vipers (USA) (6S).sav`
  - structurally recognizable as Saturn backup RAM
  - contains no active save entries
  - must be rejected as noise / empty image
- `Alien Trilogy (USA).sav`
  - not decision-complete yet
  - requires a real fixture and parser validation before it can be treated as supported

## Backend Contract

### Upload Acceptance

For `system=saturn`, the backend accepts only validated Saturn backup RAM payloads.

Required conditions:

1. payload matches a supported Saturn backup RAM container size/layout
2. header/volume structure validates
3. at least one active save entry exists

If those checks pass, backend stores the file as a normal save track and adds Saturn metadata.

### Saturn-specific Errors

Saturn validation itself already produces specific reject reasons:

- `saturn requires a validated backup RAM image`
- `saturn backup RAM image has no active save entries`

Current API behavior:

- upload routes keep the stable generic `422` message  
  `unsupported or unrecognized save format; only known consoles/arcade are allowed`
- when available, backend now also returns a specific `reason` field in the same error response
  (for example `saturn backup RAM image has no active save entries`)

### Download Conversions

The backend supports these Saturn download targets through `saturnFormat`:

- `original`
- `mister`
- `internal-raw`
- `cartridge-raw`
- `mednafen`
- `mednafen-internal`
- `mednafen-cartridge`
- `yabause`
- `yabasanshiro`
- `bup`
- `ymir`
- `ymbp`

Without `saturnFormat`, the original stored payload is returned.

For `bup`, `ymir`, and `ymbp`:

- if the Saturn image contains multiple entries, caller must provide `saturnEntry=<entry filename>`
- without `saturnEntry`, backend returns an explicit bad request error

### Helper-facing Endpoints

The Saturn helper flow is expected to work through:

- `POST /saves`
- `GET /save/latest`
- `GET /saves/download`

API aliases:

- primary agent API bases are `/api` and `/api/v1`
- helper compatibility routes at `/` and `/v1` remain valid for current helper builds

With helper-auth semantics (when auth mode is enabled):

- app-password required outside auto-enroll window
- key bound to `device_type + fingerprint`
- no silent rebinding to a different helper identity

When auth mode is disabled, requests without app-password are accepted, but helper identity metadata should still be sent.

For MiSTer Saturn, runtime identity remains:

- `device_type=mister`

## SGM-Helper Scanner Contract

This is the rule the helpers must now follow.

### Scanner Decision

A local file may be classified as `saturn` only when both are true:

1. the path/ROM/source context indicates Saturn
2. the payload itself validates as a Saturn backup RAM image with active entries

Path hints alone are not enough.

### Accepted Local Container Shapes

The helper scanner should only accept Saturn payloads matching known backup RAM shapes:

- internal raw: `32768`
- internal interleaved: `65536`
- cartridge raw: `524288`
- cartridge interleaved: `1048576`
- combined raw: `557056`
- combined interleaved: `1114112`
- YabaSanshiro raw: `4194304`
- YabaSanshiro interleaved: `8388608`

Accepted extensions remain:

- `.sav`
- `.srm`
- `.ram`
- `.bkr`

But extension alone never makes a file a Saturn save.

### Structural Validation

Before upload, the helper must confirm:

1. recognized Saturn backup RAM size/layout
2. valid backup RAM header
3. valid archive entry structure
4. at least one active save entry

If the image is only header-valid but contains no active saves, it must be skipped.

### Required Upload Shape

For accepted Saturn uploads, helper sends:

- `file=<payload>`
- `system=saturn`
- `rom_sha1=<sha1>`
- `slotName=default` unless a stronger slot identity exists
- `device_type=mister` on MiSTer
- stable `fingerprint`

### Helper Skip Policy

Helpers should not upload these cases:

- empty Saturn backup RAM images
- structurally invalid Saturn images
- files that look Saturn-like only by path/name/extension

Recommended helper-side skip reasons:

- `skip_invalid_saturn_backup_ram`
- `skip_empty_saturn_backup_ram`
- `skip_saturn_without_structural_evidence`

Recommended helper log lines:

- `Skipping Saturn save <path>: skip_invalid_saturn_backup_ram`
- `Skipping Saturn save <path>: skip_empty_saturn_backup_ram`
- `Skipping Saturn save <path>: skip_saturn_without_structural_evidence(size=<bytes>)`

## Implementation Decision

The sync model remains:

- helpers upload Saturn backup RAM containers
- backend is the authority for accept/reject and conversion
- helpers do not split/merge Saturn sub-entries client-side

This is intentionally different from the PlayStation extracted-save model.

## Acceptance Criteria

Saturn is considered stable when all of these hold:

1. valid Saturn fixture uploads succeed
2. empty Saturn fixture uploads are skipped by helper or rejected by backend with Saturn-specific reason
3. `GET /save/latest` returns `exists=true` for accepted Saturn saves
4. `GET /saves/download?id=...&saturnFormat=mister` returns a valid Saturn payload
5. helper logs clearly show why empty/invalid Saturn images were not uploaded

## Open Items

- Add and validate a real `Alien Trilogy (USA).sav` fixture before marking that title/path as supported
- Add helper ingest support for Mednafen cartridge source files (`.bcr` and optionally `.bcr.gz`) if we want parity for that local-save shape
