#!/usr/bin/env bash
set -euo pipefail

profile="${1:-direct}"

cd "$(dirname "$0")/.."
docker compose --profile "$profile" pull
docker compose --profile "$profile" up -d
