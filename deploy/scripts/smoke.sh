#!/usr/bin/env bash
set -euo pipefail

base_url="${1:-http://localhost}"

echo "== health =="
curl -fsS "$base_url/healthz" | sed -e 's/{/{\n/'

echo
echo "== auth/me =="
curl -fsS "$base_url/auth/me" | sed -e 's/{/{\n/'

echo
echo "== saves =="
curl -fsS "$base_url/saves?limit=1&offset=0" | sed -e 's/{/{\n/'
