#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
project_root="$repo_root/backend"
evidence_dir="$repo_root/tests/evidence"
mkdir -p "$evidence_dir"

{
  echo "== go build ./... =="
  (cd "$project_root" && go build ./...)
  echo
  echo "== make test-contract =="
  (cd "$project_root" && make test-contract)
  echo
  echo "== make test-golden =="
  (cd "$project_root" && make test-golden)
} > "$evidence_dir/backend-contract-happy.txt" 2>&1

cat "$evidence_dir/backend-contract-happy.txt"
