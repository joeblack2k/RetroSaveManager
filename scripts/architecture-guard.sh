#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

if ! command -v rg >/dev/null 2>&1; then
  echo "error: ripgrep (rg) is required" >&2
  exit 1
fi

failed=0

fail_if_found() {
  local label="$1"
  local pattern="$2"
  shift 2
  local output

  output="$(rg -n "$pattern" "$@" || true)"
  if [[ -n "$output" ]]; then
    echo "[FAIL] ${label}"
    echo "$output"
    echo
    failed=1
  fi
}

fail_if_found "Use ConfirmDialog for destructive browser actions" 'window\.confirm' frontend/src
fail_if_found "Do not leave uncontrolled frontend console logging" 'console\.log' frontend/src
fail_if_found "Do not suppress TypeScript errors in app code" '@ts-ignore|@ts-expect-error' frontend/src

if [[ "$failed" -ne 0 ]]; then
  echo "architecture guard failed"
  exit 1
fi

echo "architecture guard passed"
