#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ -n "$(gofmt -l backend/cmd/server/*.go)" ]]; then
  echo "error: gofmt is required for backend/cmd/server/*.go" >&2
  gofmt -l backend/cmd/server/*.go >&2
  exit 1
fi

./scripts/architecture-guard.sh

(
  cd backend
  go test ./...
  go build ./...
)

./scripts/verify-contract.sh

(
  cd frontend
  npm test
  npm run build
  npm audit --audit-level=high
)

./scripts/security-gate.sh

echo "pre-live check passed"
