#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$ROOT_DIR/backend"
go test ./cmd/server -run 'TestCheat(EditorRegistry|LibraryPacks)' -count=1
