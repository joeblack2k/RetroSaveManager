# 1Retro self-host compatibility route matrix

Frozen from `COMPAT_SPEC_V0.md` (primary) and `reverse/selfhost_endpoint_contract.md` (inventory widening) on 2026-04-20.

## Alias policy

- Serve every surfaced compatibility route at both the root path and a `/v1` alias.
- Treat the root path as canonical for native/helper clients.
- Treat `/v1` as the canonical alias for web clients that hardcode `/v1`.

## Scope counts

- required: 19
- stub-required: 35
- out-of-scope: 71
- total: 125

## Notes

- `COMPAT_SPEC_V0.md` wins when payload or method conflicts with reverse evidence.
- Billing/Stripe routes stay out-of-scope for self-hosted v1.
- If a surfaced route lacks contract detail, classify it as `stub-required` rather than `required`.

## Route inventory

| Method | Path | Scope | Alias policy | Expected statuses | Source | Notes |
|---|---|---|---|---|---|---|
| POST | `/admin/catalog` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin/catalog surface is beyond single-user helper-first scope. |
| DELETE | `/admin/catalog/{id}` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin/catalog surface is beyond single-user helper-first scope. |
| GET | `/admin/plans` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin surface excluded from v1 compatibility target. |
| POST | `/admin/plans` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin surface excluded from v1 compatibility target. |
| DELETE | `/admin/plans/{id}` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin surface excluded from v1 compatibility target. |
| PUT | `/admin/plans/{id}` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin surface excluded from v1 compatibility target. |
| GET | `/admin/releases` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin surface excluded from v1 compatibility target. |
| DELETE | `/admin/releases/version` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin surface excluded from v1 compatibility target. |
| PATCH | `/admin/releases/version` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin surface excluded from v1 compatibility target. |
| GET | `/admin/reports/signups` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin surface excluded from v1 compatibility target. |
| POST | `/admin/roadmap/items` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Roadmap/admin feature set is outside helper-first compatibility. |
| DELETE | `/admin/roadmap/items/{id}` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Roadmap/admin feature set is outside helper-first compatibility. |
| PUT | `/admin/roadmap/items/{id}` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Roadmap/admin feature set is outside helper-first compatibility. |
| GET | `/admin/roadmap/suggestions` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Roadmap/admin feature set is outside helper-first compatibility. |
| PUT | `/admin/roadmap/suggestions/{id}` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Roadmap/admin feature set is outside helper-first compatibility. |
| POST | `/admin/roadmap/suggestions/{id}/promote` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Roadmap/admin feature set is outside helper-first compatibility. |
| GET | `/admin/roles` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin surface excluded from v1 compatibility target. |
| POST | `/admin/roles/grant` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin surface excluded from v1 compatibility target. |
| POST | `/admin/roles/revoke` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin surface excluded from v1 compatibility target. |
| GET | `/admin/settings` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin surface excluded from v1 compatibility target. |
| PUT | `/admin/settings` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin surface excluded from v1 compatibility target. |
| GET | `/admin/stripe/price-mappings` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Billing admin surface is excluded with all stripe work. |
| POST | `/admin/stripe/price-mappings` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Billing admin surface is excluded with all stripe work. |
| DELETE | `/admin/stripe/price-mappings/{id}` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Billing admin surface is excluded with all stripe work. |
| PATCH | `/admin/stripe/price-mappings/{id}` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Billing admin surface is excluded with all stripe work. |
| GET | `/admin/users` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin surface excluded from v1 compatibility target. |
| POST | `/admin/users/{id}/cancel-deletion` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin surface excluded from v1 compatibility target. |
| POST | `/admin/users/{id}/delete` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin surface excluded from v1 compatibility target. |
| POST | `/admin/users/{id}/disable` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin surface excluded from v1 compatibility target. |
| POST | `/admin/users/{id}/enable` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin surface excluded from v1 compatibility target. |
| GET | `/admin/users/{id}/plan` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin surface excluded from v1 compatibility target. |
| POST | `/admin/users/{id}/plan` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin surface excluded from v1 compatibility target. |
| POST | `/admin/users/{id}/storage-bonus` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Admin surface excluded from v1 compatibility target. |
| POST | `/auth/2fa/backup-codes/regenerate` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Stub to keep web settings flows from breaking. |
| POST | `/auth/2fa/disable` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Surfaced account-security workflow. |
| POST | `/auth/2fa/setup/totp` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Out of primary helper flow but part of surfaced account UI. |
| GET | `/auth/2fa/status` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Can return disabled/default values in no-auth mode. |
| GET | `/auth/2fa/trusted-devices` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Web account-security surface. |
| DELETE | `/auth/2fa/trusted-devices/{id}` | stub-required | serve-both-root-and-v1 | 200, 404 | selfhost_endpoint_contract | Delete surface for trusted device rows. |
| POST | `/auth/2fa/verify` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Required by surfaced clients but can remain a permissive no-op in single-user no-auth mode. |
| POST | `/auth/2fa/verify-setup` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Surfaced account-security workflow. |
| GET | `/auth/app-passwords` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Surfaced settings route adjacent to required token/app-password exchange. |
| POST | `/auth/app-passwords` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Settings-management surface; exact shape can be stubbed first. |
| DELETE | `/auth/app-passwords/{id}` | stub-required | serve-both-root-and-v1 | 200, 404 | selfhost_endpoint_contract | Delete surface for app-password rows. |
| POST | `/auth/cancel-deletion` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Companion lifecycle action for web settings flow. |
| POST | `/auth/delete-account` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Surfaced account lifecycle action; can no-op for single-user LAN mode. |
| POST | `/auth/device` | required | serve-both-root-and-v1 | 200 | COMPAT_SPEC_V0, cmd/server/main.go | Compat-spec device flow stub for helper/no-auth onboarding. |
| POST | `/auth/device/confirm` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract, cmd/server/main.go | Surfaced in reverse contract for browser device confirmation flow. |
| POST | `/auth/device/token` | required | serve-both-root-and-v1 | 202, 200 | COMPAT_SPEC_V0, selfhost_endpoint_contract, cmd/server/main.go | Canon from compat spec: request includes deviceCode; no-auth mode should remain permissive. |
| POST | `/auth/device/verify` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract, cmd/server/main.go | Surfaced in reverse contract for browser device flow; no compat-spec payload yet. |
| POST | `/auth/forgot-password` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Single-user no-auth deployments can no-op while preserving stable JSON. |
| POST | `/auth/login` | required | serve-both-root-and-v1 | 200 | COMPAT_SPEC_V0, selfhost_endpoint_contract, cmd/server/main.go | Primary helper/session bootstrap; accept Authorization header and ignore CSRF marker. |
| POST | `/auth/logout` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Surfaced by web and reverse notes, but not contract-detailed in compat spec. |
| GET | `/auth/me` | required | serve-both-root-and-v1 | 200 | COMPAT_SPEC_V0, selfhost_endpoint_contract, cmd/server/main.go | Must never 401 in AUTH_MODE=disabled; map all requests to one internal principal. |
| POST | `/auth/resend-verification` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Reverse/web surfaced route without compat-spec payload details. |
| POST | `/auth/reset-password` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Stub for surfaced password-reset forms. |
| POST | `/auth/signup` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Keep stub-compatible for helper/web flows; no strict auth semantics in LAN mode. |
| POST | `/auth/token` | required | serve-both-root-and-v1 | 200 | COMPAT_SPEC_V0, selfhost_endpoint_contract, cmd/server/main.go | Allow empty JSON body; issue dummy or internal token in no-auth mode. |
| POST | `/auth/token/app-password` | required | serve-both-root-and-v1 | 200 | COMPAT_SPEC_V0, selfhost_endpoint_contract, cmd/server/main.go | Stub-compatible app-password token exchange for helper clients. |
| POST | `/auth/verify-email` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Account-extra surface from web contract. |
| GET | `/catalog` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Optional free-ROM catalog surface, not part of helper-first compatibility scope. |
| GET | `/catalog/{id}` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Optional free-ROM catalog surface, not part of helper-first compatibility scope. |
| GET | `/catalog/{id}/download` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Optional free-ROM catalog surface, not part of helper-first compatibility scope. |
| GET | `/conflicts` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Web-core conflicts page surface without compat-spec payload detail. |
| GET | `/conflicts/check` | required | serve-both-root-and-v1 | 200 | COMPAT_SPEC_V0, cmd/server/main.go | Treat GET with romSha1 and slotName as canonical because compat spec is primary. |
| POST | `/conflicts/check` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Reverse contract infers POST, but compat spec canonizes GET; retain as conservative compatibility stub. |
| GET | `/conflicts/count` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract, cmd/server/main.go | Surfaced by web bundle for conflict badges; current server already exposes it. |
| POST | `/conflicts/report` | required | serve-both-root-and-v1 | 200 | COMPAT_SPEC_V0, selfhost_endpoint_contract, cmd/server/main.go | Compat spec expects multipart with file and hash fields; missing file should fail with client-readable error. |
| GET | `/conflicts/{id}` | stub-required | serve-both-root-and-v1 | 200, 404 | selfhost_endpoint_contract | Needed for web conflict detail pages, but payload still reverse-derived. |
| POST | `/conflicts/{id}/resolve` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Conflict resolution route surfaced in web bundle. |
| POST | `/dev/signup` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Inferred binary route; keep as stub until runtime capture confirms payload. |
| GET | `/devices` | required | serve-both-root-and-v1 | 200 | COMPAT_SPEC_V0, selfhost_endpoint_contract, cmd/server/main.go | Web-core device management is explicitly called out in compat spec section 6. |
| DELETE | `/devices/{id}` | required | serve-both-root-and-v1 | 200 | COMPAT_SPEC_V0, selfhost_endpoint_contract, cmd/server/main.go | Compat spec requires delete support for device cleanup. |
| PATCH | `/devices/{id}` | required | serve-both-root-and-v1 | 200, 404 | COMPAT_SPEC_V0, selfhost_endpoint_contract, cmd/server/main.go | Alias update is part of required web parity surface. |
| GET | `/events` | required | serve-both-root-and-v1 | 200 | COMPAT_SPEC_V0, selfhost_endpoint_contract, cmd/server/main.go | Required for watch compatibility; /v1/events should be served via alias policy too. |
| GET | `/forge/games` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Forge/editor surface is outside v1 compatibility scope. |
| GET | `/forge/parsers` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Forge/editor surface is outside v1 compatibility scope. |
| GET | `/forge/parsers/{id}` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Forge/editor surface is outside v1 compatibility scope. |
| PATCH | `/forge/parsers/{id}` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Forge/editor surface is outside v1 compatibility scope. |
| POST | `/forge/parsers/{id}/games` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Forge/editor surface is outside v1 compatibility scope. |
| DELETE | `/forge/parsers/{id}/games/{gameId}` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Forge/editor surface is outside v1 compatibility scope. |
| GET | `/forge/systems` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Forge/editor surface is outside v1 compatibility scope. |
| DELETE | `/game/saves` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Web bulk-delete convenience route for multiple game ids. |
| GET | `/games/library` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Optional library management surface outside v1 helper-sync goal. |
| POST | `/games/library` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Optional library management surface outside v1 helper-sync goal. |
| DELETE | `/games/library/{id}` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Optional library management surface outside v1 helper-sync goal. |
| POST | `/games/lookup` | required | serve-both-root-and-v1 | 200 | COMPAT_SPEC_V0, selfhost_endpoint_contract, cmd/server/main.go | Web helper lookup route; stable query echoing matters more than deep metadata in v1. |
| GET | `/manager/games` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Manager surface is outside helper-first compatibility scope. |
| POST | `/manager/games` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Manager surface is outside helper-first compatibility scope. |
| DELETE | `/manager/games/{id}` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Manager surface is outside helper-first compatibility scope. |
| GET | `/manager/games/{id}` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Manager surface is outside helper-first compatibility scope. |
| PUT | `/manager/games/{id}` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Manager surface is outside helper-first compatibility scope. |
| POST | `/manager/games/{id}/boxart` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Manager surface is outside helper-first compatibility scope. |
| POST | `/manager/games/{id}/links` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Manager surface is outside helper-first compatibility scope. |
| DELETE | `/manager/games/{id}/links/{linkId}` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Manager surface is outside helper-first compatibility scope. |
| DELETE | `/manager/games/{id}/metadata/{key}` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Manager surface is outside helper-first compatibility scope. |
| PUT | `/manager/games/{id}/metadata/{key}` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Manager surface is outside helper-first compatibility scope. |
| GET | `/manager/games/{id}/roms` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Manager surface is outside helper-first compatibility scope. |
| POST | `/manager/games/{id}/roms` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Manager surface is outside helper-first compatibility scope. |
| GET | `/manager/publishers` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Manager surface is outside helper-first compatibility scope. |
| POST | `/manager/publishers` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Manager surface is outside helper-first compatibility scope. |
| DELETE | `/manager/roms/{id}` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Manager surface is outside helper-first compatibility scope. |
| PUT | `/manager/roms/{id}` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Manager surface is outside helper-first compatibility scope. |
| GET | `/manager/systems` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Manager surface is outside helper-first compatibility scope. |
| GET | `/parser/wasm` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Surfaced parser-editor dependency for save detail pages. |
| GET | `/referral` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Surfaced non-admin route without v1 single-user priority, so preserve as stub only. |
| GET | `/roadmap/items` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Roadmap/community feature set is outside helper-first compatibility. |
| POST | `/roadmap/items/{id}/vote` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Roadmap/community feature set is outside helper-first compatibility. |
| POST | `/roadmap/suggestions` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Roadmap/community feature set is outside helper-first compatibility. |
| GET | `/roadmap/suggestions/mine` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Roadmap/community feature set is outside helper-first compatibility. |
| GET | `/rom/lookup` | required | serve-both-root-and-v1 | 200 | COMPAT_SPEC_V0, cmd/server/main.go | Treat GET with filenameStem query as canonical because compat spec is the primary contract. |
| POST | `/rom/lookup` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract | Reverse contract infers POST, but compat spec canonizes GET; keep POST as compatibility stub if later capture confirms it. |
| DELETE | `/save` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract, cmd/server/main.go | Surfaced delete-by-id route for web save management. |
| GET | `/save` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract, cmd/server/main.go | Used by web save detail; not specified in compat spec, so freeze as stub-required. |
| GET | `/save/latest` | required | serve-both-root-and-v1 | 200 | COMPAT_SPEC_V0, selfhost_endpoint_contract, cmd/server/main.go | Canonical method is GET with romSha1 and optional slotName query parameters. |
| DELETE | `/saves` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract, cmd/server/main.go | Bulk-delete route surfaced by web bundle; exact response body can stay minimal in v1. |
| GET | `/saves` | required | serve-both-root-and-v1 | 200 | COMPAT_SPEC_V0, selfhost_endpoint_contract, cmd/server/main.go | List endpoint for web parity and batch clients; filters may be accepted permissively at first. |
| POST | `/saves` | required | serve-both-root-and-v1 | 200 | COMPAT_SPEC_V0, selfhost_endpoint_contract, cmd/server/main.go | Single route must absorb helper multipart uploads and web batch JSON uploads under root and /v1 aliases. |
| GET | `/saves/download` | required | serve-both-root-and-v1 | 200 | COMPAT_SPEC_V0, selfhost_endpoint_contract, cmd/server/main.go | Download by save id; keep attachment semantics stable. |
| GET | `/saves/download_many` | stub-required | serve-both-root-and-v1 | 200 | selfhost_endpoint_contract, cmd/server/main.go | Web convenience route outside compat spec but needed for save-management parity. |
| GET | `/saves/systems` | required | serve-both-root-and-v1 | 200 | COMPAT_SPEC_V0, selfhost_endpoint_contract, cmd/server/main.go | Compat spec calls for simple system list used by web save filtering. |
| POST | `/stripe/checkout` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Billing explicitly excluded for this self-hosted v1. |
| POST | `/stripe/portal` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Billing explicitly excluded for this self-hosted v1. |
| GET | `/stripe/status` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Billing explicitly excluded for this self-hosted v1. |
| POST | `/stripe/unlink` | out-of-scope | serve-both-root-and-v1 | 404, 501 | selfhost_endpoint_contract | Billing explicitly excluded for this self-hosted v1. |
