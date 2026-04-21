# Release Notes

## 2026-04-22

### Included

- Single-container runtime for self-hosted deploys:
  - one `app` container serves both API and frontend SPA
  - one GHCR image: `ghcr.io/joeblack2k/retrosavemanager`
- PlayStation save-domain improvements:
  - real PS1 memory-card detection only
  - real PS2 memory-card detection only
  - unsupported PS1/PS2 save-state noise is rejected during rescan
- Memory-card detail enrichment:
  - PS1 entry titles and icon previews
  - PS2 entry titles from `icon.sys`
  - PS2 entry previews, product codes, block counts, and size stats
- Live rescan behavior improved:
  - noisy or false-positive PlayStation records are pruned
  - valid PS memory cards remain grouped as `Memory Card N`

### Deploy Notes

- Default runtime is direct HTTP on port `80`
- Docker Compose default is a single `app` service
- Macvlan stays available as an optional override

### Validation Summary

- Backend tests passed on the test bench VM
- Frontend tests and production build passed locally
- Live deploy validated on `192.168.2.10`

### Not Included In This Release

- In-progress frontend `My Games` grouping refinements still in the local worktree and were not part of this release
