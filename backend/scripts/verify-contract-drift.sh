#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
project_root="$repo_root/backend"
evidence_dir="$repo_root/tests/evidence"
golden_file="$project_root/cmd/server/testdata/golden/auth_me.json"
backup_file="$golden_file.bak-contract-drift"
mkdir -p "$evidence_dir"

cleanup() {
  if [[ -f "$backup_file" ]]; then
    mv "$backup_file" "$golden_file"
  fi
}
trap cleanup EXIT

cp "$golden_file" "$backup_file"
python3 - <<'PY2' "$golden_file"
from pathlib import Path
import json
import sys
path = Path(sys.argv[1])
data = json.loads(path.read_text())
data["message"] = "INTENTIONAL GOLDEN DRIFT"
path.write_text(json.dumps(data, indent=2) + "\n")
PY2

set +e
(
  cd "$project_root" && make test-golden
) > "$evidence_dir/backend-contract-drift.txt" 2>&1
status=$?
set -e

if [[ $status -eq 0 ]]; then
  echo "expected golden drift failure but tests passed" >> "$evidence_dir/backend-contract-drift.txt"
  exit 1
fi

echo >> "$evidence_dir/backend-contract-drift.txt"
echo "drift-check-status=$status" >> "$evidence_dir/backend-contract-drift.txt"

cat "$evidence_dir/backend-contract-drift.txt"
