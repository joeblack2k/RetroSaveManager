#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
./backend/scripts/save-root-backup.sh "$@"
