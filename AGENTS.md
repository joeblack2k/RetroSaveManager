# Repo Instructions

This repository is public-facing. Treat every tracked file as publishable.

## Documentation Safety

- Never commit real private IP addresses, SSH targets, usernames, passwords, tokens, API keys, or internal hostnames.
- Always use placeholders such as `<deploy-host>`, `<deploy-user>`, `<deploy-password>`, `<test-host>`, and `<workspace-root>`.
- If an example must include an IP address, prefer RFC 5737 documentation ranges such as `192.0.2.10`, `198.51.100.10`, or `203.0.113.10`.
- Use relative repository paths in docs. Do not commit personal absolute filesystem paths.

## Operational Notes

- Keep real environment details in a local untracked file such as `LOCAL_SECRETS.md` or in an external secret manager.
- Do not copy live infrastructure credentials into `README.md`, `DEPLOY.md`, `onboarding.md`, workflows, scripts, or tests.
- Before committing doc, workflow, or deploy changes, run `./scripts/security-gate.sh`.

## Product Constraints

- Keep RetroSaveManager branding only. Do not reintroduce `1Retro` branding, analytics, or upstream service references except in necessary compatibility notes.
- Preserve single-container runtime assumptions unless the task explicitly changes deployment architecture.
- Maintain helper/API backward compatibility unless a change is explicitly approved.
