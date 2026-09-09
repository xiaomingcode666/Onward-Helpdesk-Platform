#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cache_dir="${REMOTE_HELPDESK_GOCACHE:-$repo_root/tmp/go-build-cache}"
max_mb="${REMOTE_HELPDESK_GOCACHE_MAX_MB:-2048}"

mkdir -p "$cache_dir" "$repo_root/tmp"

cache_kb="$(du -sk "$cache_dir" 2>/dev/null | awk '{print $1}')"
max_kb="$((max_mb * 1024))"
if [ "${cache_kb:-0}" -gt "$max_kb" ]; then
  echo "go build cache exceeded ${max_mb}MB; clearing $cache_dir"
  GOCACHE="$cache_dir" go clean -cache -testcache
fi

cd "$repo_root"
GOCACHE="$cache_dir" go build -tags dev -o ./tmp/main ./cmd/server
