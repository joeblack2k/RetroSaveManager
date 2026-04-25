# Game Support Modules

This folder is the default GitHub library for runtime-loadable RetroSaveManager game support modules.

Add reviewed `.rsmodule.zip` bundles here, then use the web app Settings page or `POST /api/modules/sync` to import them into a running server without rebuilding Docker.

Module source code may be stored elsewhere for review, but the runtime bundle must contain sandboxed WASM plus declarative YAML only.

## Included sample

`sample-semantic-module.rsmodule.zip` is a safe loader smoke-test for a fictive 4-byte Game Boy save named `Module Demo Cartridge`.
It proves GitHub sync, WASM ABI validation, parser output, and module-backed cheat catalog wiring without matching or overriding real saves.
