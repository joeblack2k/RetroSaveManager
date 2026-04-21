# Compatibility

RetroSaveManager keeps compatibility-first routing behavior:

- Every surfaced route is available at both root and `/v1` alias.
- Helper-critical endpoints remain shape-compatible:
  - `/auth/*`
  - `/saves*`
  - `/save/latest`
  - `/rom/lookup`
  - `/conflicts*`
  - `/events`

Additional user-web endpoints are included:

- `/catalog*`
- `/games/library*`
- `/roadmap/items*`
- `/roadmap/suggestions*`

Excluded parity domains:

- admin
- billing/stripe management
- manager
- forge
