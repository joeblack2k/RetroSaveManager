# Game Support Modules

This folder is the default GitHub library for runtime-loadable RetroSaveManager game support modules.

Add reviewed `.rsmodule.zip` bundles here, then use the web app Settings page or `POST /api/modules/sync` to import them into a running server without rebuilding Docker.

Module source code may be stored elsewhere for review, but the runtime bundle must contain sandboxed WASM plus declarative YAML only. Optional `src/` files inside a bundle are audit/reference material and are never compiled or executed by the backend.

## Included Game Modules

| System | Game | Module | Version | Cheat packs | File |
| --- | --- | --- | --- | ---: | --- |
| `gameboy` | Donkey Kong Country (Game Boy Color) | `gameboy-donkey-kong-country-gbc` | `1.0.0` | 1 | `gameboy-donkey-kong-country-gbc.rsmodule.zip` |
| `gameboy` | Pokemon Red/Blue/Yellow | `gameboy-pokemon-red-blue-yellow` | `1.0.0` | 1 | `gameboy-pokemon-red-blue-yellow.rsmodule.zip` |
| `gameboy` | Super Mario Bros. Deluxe | `gameboy-super-mario-bros-deluxe` | `1.0.0` | 1 | `gameboy-super-mario-bros-deluxe.rsmodule.zip` |
| `gameboy` | Wario Land II | `gameboy-wario-land-ii` | `1.0.0` | 1 | `gameboy-wario-land-ii.rsmodule.zip` |
| `gba` | Wario Land 4 | `gba-wario-land-4` | `1.0.0` | 1 | `gba-wario-land-4.rsmodule.zip` |
| `n64` | Banjo-Kazooie | `n64-banjo-kazooie` | `1.0.0` | 1 | `n64-banjo-kazooie.rsmodule.zip` |
| `n64` | Banjo-Tooie | `n64-banjo-tooie` | `1.0.0` | 1 | `n64-banjo-tooie.rsmodule.zip` |
| `n64` | Wave Race 64 | `n64-wave-race-64` | `1.1.1` | 2 | `wave-race-64.rsmodule.zip` |
| `n64` | Yoshi's Story | `n64-yoshis-story` | `1.1.0` | 1 | `n64-yoshis-story.rsmodule.zip` |
| `nds` | New Super Mario Bros. | `nds-new-super-mario-bros` | `1.0.0` | 1 | `nds-new-super-mario-bros.rsmodule.zip` |
| `neogeo` | Double Dragon (Neo-Geo) | `neogeo-doubledr` | `1.0.0` | 1 | `neogeo-doubledr.rsmodule.zip` |
| `neogeo` | Metal Slug 5 | `neogeo-mslug5` | `1.0.1` | 1 | `neogeo-mslug5.rsmodule.zip` |
| `ps2` | Burnout 3: Takedown | `ps2-burnout-3` | `1.0.0` | 1 | `burnout-3.rsmodule.zip` |
| `ps2` | Mortal Kombat Shaolin Monks | `ps2-mk-shaolin-monks` | `1.0.0` | 1 | `mortal-kombat-shaolin-monks.rsmodule.zip` |
| `psx` | Castlevania: Symphony of the Night | `psx-castlevania-symphony-of-the-night` | `1.0.1` | 1 | `playstation-castlevania-symphony-of-the-night.rsmodule.zip` |
| `saturn` | Quake | `saturn-quake` | `1.0.0` | 1 | `saturn-quake.rsmodule.zip` |
| `snes` | Super Mario World | `snes-super-mario-world` | `1.1.0` | 1 | `snes-super-mario-world.rsmodule.zip` |
| `wii` | Super Mario Galaxy 2 | `wii-super-mario-galaxy-2` | `1.0.0` | 1 | `wii-super-mario-galaxy-2.rsmodule.zip` |

## Included Sample

`sample-semantic-module.rsmodule.zip` is a safe loader smoke-test for a fictive 4-byte Game Boy save named `Module Demo Cartridge`.
It proves GitHub sync, WASM ABI validation, parser output, and module-backed cheat catalog wiring without matching or overriding real saves.
