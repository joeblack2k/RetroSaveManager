# Auto Helper Enrollment

Last updated: 2026-04-23 09:15 Europe/Amsterdam

This document explains how RetroSaveManager auto-enrolls helpers, how a helper should authenticate, and which extra metadata a helper should send so the `Devices` screen becomes genuinely useful.

All examples below use reserved example hostnames and TEST-NET IP ranges on purpose.
Do not copy internal/private addresses into docs or fixtures.

## What the sidebar button does

The sidebar button `Add helper` opens a temporary auto-enrollment window for 15 minutes.

During that 15 minute window, a helper may do one of these two flows:

1. Upload its first save without an app password.
2. Request an app password first through the token endpoint and then start syncing.

Once the helper has been enrolled, the server binds the generated app password to that helper identity.
After that first enrollment, the helper must keep using the same app password for future sync requests.

## Identity model

A helper is identified by these two required values:

- `device_type`
- `fingerprint`

These two values are the backend identity key.
They must be stable across runs for the same helper installation.

Recommended examples:

- `device_type=mister`
- `device_type=retroarch`
- `device_type=pcsx2`
- `fingerprint=mister-living-room`
- `fingerprint=retroarch-steamdeck`

If a helper presents only one of the two values, the backend rejects the request.

## Enrollment flow A: first sync upload without a key

This is the simplest flow.
The user clicks `Add helper` in the UI, then the helper performs its first normal multipart upload to `/saves` without `app_password`.

If the window is still open and the helper sends a valid identity, the backend will:

1. create or bind the device record
2. generate an app password
3. return that password in the response header `X-RSM-Auto-App-Password`
4. accept the save upload

The helper must store that returned password locally and reuse it on later requests.

### Required fields for this flow

- `device_type`
- `fingerprint`
- the normal save upload fields already used by the helper

### Recommended extra fields

- `hostname`
- `helper_name`
- `helper_version`
- `platform`
- `sync_paths`
- `systems`

### Multipart example

```bash
curl -X POST "https://rsm.example.invalid/saves" \
  -F "device_type=mister" \
  -F "fingerprint=mister-living-room" \
  -F "hostname=mister-01.example.invalid" \
  -F "helper_name=RSM Helper" \
  -F "helper_version=2.1.0" \
  -F "platform=MiSTer" \
  -F "sync_paths=/media/fat/saves/SNES;/media/fat/saves/PSX" \
  -F "systems=snes,psx" \
  -F "system=snes" \
  -F "rom_sha1=abc123..." \
  -F "slotName=default" \
  -F "file=@/path/to/save.srm"
```

On success, read and persist:

- response header `X-RSM-Auto-App-Password`

## Enrollment flow B: ask for a key first

If a helper wants to provision first and sync after, it can call:

- `POST /auth/token/app-password`

This endpoint only auto-provisions while the 15 minute window is open.

### JSON example

```json
{
  "name": "Living Room MiSTer",
  "deviceType": "mister",
  "fingerprint": "mister-living-room",
  "hostname": "mister-01.example.invalid",
  "helperName": "RSM Helper",
  "helperVersion": "2.1.0",
  "platform": "MiSTer",
  "syncPaths": [
    "/media/fat/saves/SNES",
    "/media/fat/saves/PSX"
  ],
  "systems": ["snes", "psx"]
}
```

The response includes the generated key as:

- `token`
- `plainTextKey`

The helper should persist that value and then use it for all future sync requests.

## Normal authenticated helper requests

After enrollment, the helper should always send:

- the app password
- the same stable `device_type`
- the same stable `fingerprint`

Supported app password transport:

1. `Authorization: Bearer XXX-XXX`
2. `X-RSM-App-Password: XXX-XXX`
3. multipart field `app_password=XXX-XXX`

Recommended request metadata should still be sent regularly so the server can keep the `Devices` page fresh.
That means hostname, helper version, sync folders, and reported systems should not be one-time-only values.

## Recommended metadata contract

These fields are optional from a strict compatibility point of view, but helpers should send them.

### Identity and client

- `hostname`
  - example: `mister-01.example.invalid`
- `helper_name`
  - example: `RSM Helper`
- `helper_version`
  - example: `2.1.0`
- `platform`
  - example: `MiSTer`, `RetroArch`, `Linux`, `Windows`, `macOS`

### Sync scope

- `sync_paths`
  - the concrete filesystem folders this helper watches
  - may be sent as repeated values, JSON array, or delimited string
- `systems`
  - the systems this helper currently reports it can sync
  - send canonical slugs such as `snes`, `psx`, `ps2`, `nds`, `dreamcast`

### What the backend infers itself

Helpers do **not** need to send these fields.
The backend stores them automatically from the request:

- `lastSeenIp`
- `lastSeenUserAgent`
- `lastSeenAt`

## Accepted field names

The backend currently accepts multiple naming styles for compatibility.
Preferred names are below.

### Preferred form/query names

- `device_type`
- `fingerprint`
- `hostname`
- `helper_name`
- `helper_version`
- `platform`
- `sync_paths`
- `systems`

### Accepted headers

- `X-RSM-Device-Type`
- `X-RSM-Fingerprint`
- `X-RSM-Hostname`
- `X-RSM-Helper-Name`
- `X-RSM-Helper-Version`
- `X-RSM-Platform`
- `X-RSM-Sync-Paths`
- `X-RSM-Systems`
- `X-RSM-App-Password`

### Accepted JSON names for `POST /auth/token/app-password`

- `deviceType`
- `fingerprint`
- `hostname`
- `helperName`
- `helperVersion`
- `platform`
- `syncPaths`
- `systems`

## Value precedence

If the same metadata is supplied more than once, the backend resolves fields in this order:

1. form/body field
2. query parameter
3. request header

For `lastSeenIp`, the backend uses request networking data:

1. `X-Forwarded-For` first value, if present
2. `X-Real-IP`
3. socket remote address

## Devices page expectations

When helpers send the metadata above, the `Devices` page can show:

- display name
- device type and fingerprint
- hostname
- helper name and version
- platform
- last seen IP
- last seen user agent
- last seen timestamp
- last synced timestamp
- reported sync folders
- reported systems
- app password binding
- server-side console policy

## Helper implementation checklist

1. Generate a stable `fingerprint` per installation, not per run.
2. Keep `device_type` stable for the runtime profile.
3. During onboarding, let the user press `Add helper` in the UI.
4. Use either first-upload auto-enroll or `POST /auth/token/app-password`.
5. Persist the returned app password securely.
6. Send the same identity on every request after enrollment.
7. Keep sending metadata like version, hostname, folders, and systems so the server can display current device state.
8. On PlayStation helpers, continue sending explicit `slotName` and runtime-specific identity exactly as already documented in `playstation.md`.

## Minimal recommended helper payload

For a normal multipart `/saves` upload, this is the minimum recommended shape:

- `app_password`
- `device_type`
- `fingerprint`
- `hostname`
- `helper_name`
- `helper_version`
- `platform`
- `sync_paths`
- `systems`
- `system`
- `rom_sha1`
- `slotName`
- `file`

## Notes for helper developers

- The backend is the source of truth for device records.
- The helper should report facts, not derived guesses.
- `systems` should describe what the helper is actually syncing right now, not every system it could theoretically support.
- `sync_paths` should be real folders used by that device.
- If helper metadata changes over time, resend the new values. The backend stores the latest seen values.
