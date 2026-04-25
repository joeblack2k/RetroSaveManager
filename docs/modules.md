# Game Support Modules

Last updated: 2026-04-25 CEST

Game Support Modules let contributors add parser-backed save details and cheat editing without rebuilding the Docker image. A module is a reviewed `.rsmodule.zip` bundle containing sandboxed WASM plus declarative YAML.

The backend never compiles or executes raw Go from a module zip. Optional source files are audit material only.

## Default Library

```text
MODULE_LIBRARY_REPO=joeblack2k/RetroSaveManager
MODULE_LIBRARY_REF=main
MODULE_LIBRARY_PATH=modules
```

A running server can import modules from the GitHub library with the Settings page or the API:

```bash
export RSM_API="https://rsm.example.invalid/api"
curl -s -X POST "$RSM_API/modules/sync" | jq
curl -s "$RSM_API/modules" | jq
```

Imported modules are stored under:

```text
SAVE_ROOT/_rsm/modules/installed/<moduleId>/
```

## Bundle Layout

```text
<game>.rsmodule.zip
  rsm-module.yaml
  parser.wasm
  cheats/*.yaml
  README.md
  src/* optional, ignored by runtime
```

Rules:

- The zip filename must end in `.rsmodule.zip`.
- Paths must be relative and must not contain `..` segments.
- Symlinks are rejected.
- Oversized zips, oversized files, bad YAML, and invalid WASM are rejected before activation.
- `src/*` is allowed for review only. The server does not compile or run it.

## Manifest v1

`rsm-module.yaml`:

```yaml
moduleId: n64-example-game
schemaVersion: 1
version: 1.0.0
systemSlug: n64
gameId: n64/example-game
title: Example Game
parserId: example-game-wasm
wasmFile: parser.wasm
abiVersion: rsm-wasm-json-v1
titleAliases:
  - Example Game
romHashes:
  - 0123456789abcdef0123456789abcdef01234567
payload:
  exactSizes:
    - 32768
  formats:
    - sra
cheatPacks:
  - path: cheats/example-game.yaml
```

Required fields:

- `moduleId`: stable lowercase id, unique across modules.
- `schemaVersion`: currently `1`.
- `version`: module version string.
- `systemSlug`: supported RetroSaveManager system slug.
- `gameId`: stable game id, usually `<system>/<game>`.
- `title`: display title.
- `parserId`: parser/editor id exposed by the WASM module.
- `wasmFile`: path to the compiled WASM file inside the zip.
- `abiVersion`: currently `rsm-wasm-json-v1`.
- `payload.exactSizes`: accepted payload sizes in bytes.
- `payload.formats`: accepted save formats/extensions.

Optional fields:

- `titleAliases`: additional strict title matches.
- `romHashes`: trusted ROM hashes if the game needs stronger matching.
- `cheatPacks`: YAML packs bundled with the module. If omitted, all `cheats/*.yaml` files are loaded.

## WASM ABI

The backend calls four commands using JSON input and JSON output:

- `capabilities`: prove the module exports the ABI and describe supported operations.
- `parse`: inspect a save payload and return parser evidence plus semantic fields.
- `readCheats`: return current editable values for a bundled cheat pack.
- `applyCheats`: patch the payload from structured updates and return repaired bytes.

The module must export:

```text
memory
rsm_alloc(len: i32) -> i32
rsm_call(commandPtr: i32, commandLen: i32, inputPtr: i32, inputLen: i32) -> i64
```

`rsm_call` returns a packed pointer/length value:

```text
return (outputPtr << 32) | outputLen
```

The backend writes UTF-8 command bytes and JSON input bytes into module memory. The module returns UTF-8 JSON output bytes.

## Parse Command

Input shape:

```json
{
  "payload": "base64 bytes from JSON encoding",
  "filename": "Example Game.sav",
  "format": "sav",
  "systemSlug": "n64",
  "displayTitle": "Example Game",
  "romSha1": "optional",
  "romMd5": "optional",
  "slotName": "optional",
  "metadata": {}
}
```

Output shape:

```json
{
  "supported": true,
  "parserLevel": "semantic",
  "parserId": "example-game-wasm",
  "validatedSystem": "n64",
  "validatedGameId": "n64/example-game",
  "validatedGameTitle": "Example Game",
  "trustLevel": "module-semantic-verified",
  "evidence": ["checksum valid", "header matched"],
  "warnings": [],
  "payloadSizeBytes": 32768,
  "slotCount": 3,
  "activeSlotIndexes": [0],
  "checksumValid": true,
  "semanticFields": {
    "lives": 7,
    "stage": "World 1"
  }
}
```

Only return `supported: true` after structural checks pass. Do not guess from filename alone.

## Cheat YAML

Module cheat packs use the same declarative YAML shape as built-in packs, but `adapterId` and `editorId` point at the module `parserId`.

```yaml
packId: n64--example-game
schemaVersion: 1
adapterId: example-game-wasm
editorId: example-game-wasm
gameId: n64/example-game
systemSlug: n64
title: Example Game
match:
  titleAliases:
    - Example Game
payload:
  exactSizes:
    - 32768
  formats:
    - sav
sections:
  - id: stats
    title: Stats
    fields:
      - id: lives
        label: Lives
        type: integer
        min: 0
        max: 99
        op:
          kind: moduleNumber
          field: lives
```

YAML is declarative only. No scripts, raw hex expressions, or trainer-code execution are allowed.

## Cheat Commands

`readCheats` receives the payload, resolved pack, save summary, and current inspection. It returns a normal `SaveCheatEditorState` response with values, slot values, sections, selector, and presets.

`applyCheats` receives structured updates and must return:

```json
{
  "payload": "patched payload bytes via JSON base64",
  "changed": {
    "lives": 99
  },
  "integritySteps": ["checksum rebuilt"]
}
```

Applying cheats always creates a new current save version and preserves rollback history.

## Security Model

Runtime modules are sandboxed with a pure-Go WASM runtime:

- no host filesystem mounts
- no network access
- memory limit
- execution timeout
- JSON-only ABI
- zip path and symlink validation
- module enable, disable, delete, rescan, and sync controls in Settings

A broken module fails validation or stays inactive. Existing saves and built-in cheat editors continue to work.

## Author Workflow

1. Build a parser in TinyGo, Rust, AssemblyScript, or another WASM target that can export the ABI above.
2. Create `rsm-module.yaml` and one or more `cheats/*.yaml` packs.
3. Zip the files as `<game>.rsmodule.zip`.
4. Add the zip to `modules/` in GitHub.
5. Run `POST /api/modules/sync` or click `Sync from GitHub` in Settings.
6. Run `POST /api/modules/rescan` or click `Refresh saves` so existing saves gain semantic details.

Before handing off a module, test with a real save sample and include a short `README.md` in the zip explaining evidence, supported payloads, and known limitations.
