#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: prune-backups.sh [options]

Prune old backup directories below one backup root while keeping a recent safety window.

Options:
  --root PATH         Backup root directory (default: $BACKUP_ROOT)
  --keep-recent N     Always keep the newest N backup directories (default: $KEEP_RECENT)
  --keep-days N       Keep any backup directory newer than N days (default: $KEEP_DAYS)
  --dry-run           Show what would be removed without deleting anything
  -h, --help          Show this help text

Environment:
  BACKUP_ROOT         Backup root directory
  KEEP_RECENT         Newest directories to keep regardless of age
  KEEP_DAYS           Age threshold in whole days before a directory becomes pruneable
EOF
}

BACKUP_ROOT="${BACKUP_ROOT:-}"
KEEP_RECENT="${KEEP_RECENT:-5}"
KEEP_DAYS="${KEEP_DAYS:-14}"
DRY_RUN=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --root)
      BACKUP_ROOT="${2:-}"
      shift 2
      ;;
    --keep-recent)
      KEEP_RECENT="${2:-}"
      shift 2
      ;;
    --keep-days)
      KEEP_DAYS="${2:-}"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "$BACKUP_ROOT" ]]; then
  echo "BACKUP_ROOT is required. Pass --root or set BACKUP_ROOT." >&2
  exit 1
fi

if ! [[ "$KEEP_RECENT" =~ ^[0-9]+$ ]]; then
  echo "KEEP_RECENT must be a non-negative integer." >&2
  exit 1
fi

if ! [[ "$KEEP_DAYS" =~ ^[0-9]+$ ]]; then
  echo "KEEP_DAYS must be a non-negative integer." >&2
  exit 1
fi

if [[ ! -d "$BACKUP_ROOT" ]]; then
  echo "Backup root does not exist: $BACKUP_ROOT" >&2
  exit 1
fi

now_epoch="$(date +%s)"
keep_days_seconds="$((KEEP_DAYS * 86400))"
kept_count=0
deleted_count=0
skipped_count=0

echo "Backup root: $BACKUP_ROOT"
echo "Keep recent: $KEEP_RECENT"
echo "Keep days: $KEEP_DAYS"
if [[ "$DRY_RUN" -eq 1 ]]; then
  echo "Mode: dry-run"
else
  echo "Mode: delete"
fi
echo

mapfile -t entries < <(
  python3 - "$BACKUP_ROOT" <<'PY'
import os
import sys

root = sys.argv[1]
entries = []
for name in os.listdir(root):
    path = os.path.join(root, name)
    if not os.path.isdir(path):
        continue
    entries.append((os.path.getmtime(path), name))

for mtime, name in sorted(entries, reverse=True):
    print(f"{mtime} {name}")
PY
)

if [[ "${#entries[@]}" -eq 0 ]]; then
  echo "No backup directories found."
  exit 0
fi

index=0
for entry in "${entries[@]}"; do
  index=$((index + 1))
  mtime_raw="${entry%% *}"
  name="${entry#* }"
  dir="$BACKUP_ROOT/$name"

  if [[ ! -d "$dir" ]]; then
    skipped_count=$((skipped_count + 1))
    echo "SKIP   $name (directory disappeared)"
    continue
  fi

  if [[ -e "$dir/.keep" ]]; then
    kept_count=$((kept_count + 1))
    echo "KEEP   $name (.keep marker)"
    continue
  fi

  mtime_epoch="${mtime_raw%.*}"
  age_seconds=$((now_epoch - mtime_epoch))
  age_days=$((age_seconds / 86400))

  if [[ "$index" -le "$KEEP_RECENT" ]]; then
    kept_count=$((kept_count + 1))
    echo "KEEP   $name (recent slot $index/$KEEP_RECENT, age=${age_days}d)"
    continue
  fi

  if [[ "$age_seconds" -lt "$keep_days_seconds" ]]; then
    kept_count=$((kept_count + 1))
    echo "KEEP   $name (younger than ${KEEP_DAYS}d, age=${age_days}d)"
    continue
  fi

  if [[ "$DRY_RUN" -eq 1 ]]; then
    deleted_count=$((deleted_count + 1))
    echo "DELETE $name (dry-run, age=${age_days}d)"
    continue
  fi

  rm -rf -- "$dir"
  deleted_count=$((deleted_count + 1))
  echo "DELETE $name (age=${age_days}d)"
done

echo
echo "Summary:"
echo "  kept=$kept_count"
echo "  deleted=$deleted_count"
echo "  skipped=$skipped_count"
