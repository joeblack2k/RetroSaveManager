#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

if ! command -v rg >/dev/null 2>&1; then
  echo "error: ripgrep (rg) is required" >&2
  exit 1
fi

failed=0

scan_forbidden() {
  local label="$1"
  local pattern="$2"
  local ignore_file="${3:-}"
  local output

  if [[ -n "$ignore_file" ]]; then
    output="$(rg -n --hidden --glob '!.git' --glob '!frontend/node_modules/**' --glob '!frontend/dist/**' --glob '!deploy/data/**' --glob '!scripts/security-gate.sh' --glob "!${ignore_file}" -e "$pattern" . || true)"
  else
    output="$(rg -n --hidden --glob '!.git' --glob '!frontend/node_modules/**' --glob '!frontend/dist/**' --glob '!deploy/data/**' --glob '!scripts/security-gate.sh' -e "$pattern" . || true)"
  fi

  if [[ -n "$output" ]]; then
    echo "[FAIL] ${label}"
    echo "$output"
    echo
    failed=1
  fi
}

scan_forbidden "Upstream API domain reference" 'api\.1retro\.com'
scan_forbidden "Upstream 1retro.com domain reference" 'https?://[^[:space:]"'"'"']*1retro\.com'
scan_forbidden "Upstream analytics remnants (Plausible/Sentry)" '(plausible\.io|plausible-script|sentry\.io|sentry\.)'
scan_forbidden "Private key material" '-----BEGIN (RSA|DSA|EC|OPENSSH|PGP) PRIVATE KEY-----'
scan_forbidden "GitHub token pattern" 'ghp_[A-Za-z0-9]{36}'
scan_forbidden "AWS key pattern" 'AKIA[0-9A-Z]{16}'
scan_forbidden "Slack token pattern" 'xox[baprs]-[A-Za-z0-9-]{10,}'
scan_forbidden "Hardcoded private network IP" '\b(10\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}|172\.(1[6-9]|2[0-9]|3[01])\.[0-9]{1,3}\.[0-9]{1,3}|192\.168\.[0-9]{1,3}\.[0-9]{1,3})\b' 'deploy/.env.example'

if [[ "$failed" -ne 0 ]]; then
  echo "security gate failed"
  exit 1
fi

echo "security gate passed"
