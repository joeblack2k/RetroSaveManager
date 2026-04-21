#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
./backend/scripts/verify-contract-drift.sh
