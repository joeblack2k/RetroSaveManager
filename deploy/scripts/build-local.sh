#!/usr/bin/env bash
set -euo pipefail

mode="${1:-direct}"

cd "$(dirname "$0")/.."
case "$mode" in
  direct)
    docker compose -f docker-compose.yml -f docker-compose.build.yml up -d --build --remove-orphans
    ;;
  macvlan)
    docker compose -f docker-compose.yml -f docker-compose.build.yml -f docker-compose.macvlan.yml up -d --build --remove-orphans
    ;;
  *)
    echo "Unsupported mode: $mode" >&2
    echo "Use: direct | macvlan" >&2
    exit 1
    ;;
esac
