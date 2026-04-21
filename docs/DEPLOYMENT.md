# Deployment Notes

## Profiles

- `direct`: expose HTTP on host port 80
- `tls`: expose HTTP/HTTPS on ports 80/443 with Caddy internal TLS
- `macvlan`: place gateway container on LAN with its own IP

## Internal DNS (AdGuard)

Create an internal DNS rewrite for your chosen hostname:

- Hostname: `retrosavemanager.lan` (example)
- Target: `<docker-host-ip>`

Then set `PUBLIC_HOST` in `deploy/.env` to that hostname.

## Update Flow

Use pull-based update on your Docker host:

```bash
cd deploy
./scripts/pull-up.sh direct
```

Switch `direct` to `tls` or `macvlan` when needed.

## Persistence

- Save data volume: `SAVE_ROOT_HOST_PATH` (back up this root)
- App state volume: `STATE_ROOT_HOST_PATH`
