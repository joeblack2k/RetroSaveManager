#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
project_root="$repo_root/backend"
default_save_root="$project_root/data/saves"

usage() {
  cat <<'EOF'
Usage:
  ./scripts/save-root-backup.sh backup <archive.tar.gz> [save-root]
  ./scripts/save-root-backup.sh restore <archive.tar.gz> [save-root] --force

Commands:
  backup   Create a gzipped tar archive from the save root.
  restore  Restore an archive into the save root after validating it.

Notes:
  - Default save root is backend/data/saves.
  - Restore requires --force and replaces the target only after archive validation.
  - Archives are expected to contain the save-root contents, not the parent folder.
EOF
}

die() {
  printf 'error: %s
' "$*" >&2
  exit 1
}

require_parent_dir() {
  local target="$1"
  local parent
  parent="$(dirname "$target")"
  [ -d "$parent" ] || die "parent directory does not exist: $parent"
}

ensure_tar() {
  command -v tar >/dev/null 2>&1 || die 'tar is required'
}

backup_cmd() {
  local archive_path="$1"
  local save_root="${2:-$default_save_root}"
  local abs_save_root
  abs_save_root="$(python3 -c 'import os,sys; print(os.path.abspath(sys.argv[1]))' "$save_root")"

  [ -d "$abs_save_root" ] || die "save root does not exist: $abs_save_root"
  require_parent_dir "$archive_path"
  ensure_tar

  tar -C "$abs_save_root" -czf "$archive_path" .
  printf 'backup created: %s
' "$archive_path"
  printf 'save root: %s
' "$abs_save_root"
}

restore_cmd() {
  local archive_path="$1"
  local save_root="$2"
  local force_flag="$3"
  local abs_save_root parent_dir stage_dir extracted_root backup_target

  [ -f "$archive_path" ] || die "archive does not exist: $archive_path"
  [ "$force_flag" = '--force' ] || die 'restore requires --force'
  ensure_tar

  abs_save_root="$(python3 -c 'import os,sys; print(os.path.abspath(sys.argv[1]))' "$save_root")"
  parent_dir="$(dirname "$abs_save_root")"
  [ -d "$parent_dir" ] || die "save-root parent directory does not exist: $parent_dir"

  tar -tzf "$archive_path" >/dev/null 2>&1 || die "archive validation failed: $archive_path"

  stage_dir="$(mktemp -d "$parent_dir/restore-stage.XXXXXX")"
  backup_target="$parent_dir/.restore-backup-$(basename "$abs_save_root").$$"
  trap 'rm -rf -- '"'"'$stage_dir'"'"' '"'"'$backup_target'"'"'' EXIT

  tar -C "$stage_dir" -xzf "$archive_path"

  if [ -e "$abs_save_root" ]; then
    mv "$abs_save_root" "$backup_target"
  fi

  mkdir -p "$abs_save_root"
  extracted_root="$stage_dir"
  if ! cp -a "$extracted_root"/. "$abs_save_root"/; then
    rm -rf "$abs_save_root"
    if [ -e "$backup_target" ]; then
      mv "$backup_target" "$abs_save_root"
    fi
    die 'restore copy failed; original save root was restored'
  fi

  rm -rf "$backup_target"
  printf 'restore completed: %s
' "$archive_path"
  printf 'save root: %s
' "$abs_save_root"
}

main() {
  local cmd="${1:-}"
  case "$cmd" in
    backup)
      [ "$#" -ge 2 ] && [ "$#" -le 3 ] || { usage >&2; exit 1; }
      backup_cmd "$2" "${3:-$default_save_root}"
      ;;
    restore)
      [ "$#" -ge 2 ] || { usage >&2; exit 1; }
      local archive_path="$2"
      local save_root="$default_save_root"
      local force_flag=''
      shift 2
      for arg in "$@"; do
        case "$arg" in
          --force)
            force_flag='--force'
            ;;
          *)
            if [ "$save_root" = "$default_save_root" ]; then
              save_root="$arg"
            else
              usage >&2
              exit 1
            fi
            ;;
        esac
      done
      restore_cmd "$archive_path" "$save_root" "$force_flag"
      ;;
    -h|--help|help|'')
      usage
      ;;
    *)
      usage >&2
      exit 1
      ;;
  esac
}

main "$@"
