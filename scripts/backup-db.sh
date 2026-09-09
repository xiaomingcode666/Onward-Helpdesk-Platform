#!/usr/bin/env bash
# Usage: ./scripts/backup-db.sh [backup_dir]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/postgres-tools.sh
source "$SCRIPT_DIR/lib/postgres-tools.sh"

BACKUP_DIR="${1:-./backups}"
DB_NAME="${DB_NAME:-cs_ai_agent}"
DB_USER="${DB_USER:-cs_ai_agent}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_PASSWORD="${DB_PASSWORD:-}"
BACKUP_RETENTION_COUNT="${BACKUP_RETENTION_COUNT:-30}"
BACKUP_METRICS_FILE="${BACKUP_METRICS_FILE:-}"

if [[ ! "$BACKUP_RETENTION_COUNT" =~ ^[1-9][0-9]*$ ]]; then
  echo "ERROR: BACKUP_RETENTION_COUNT must be a positive integer." >&2
  exit 1
fi

init_pg_tools
mkdir -p "$BACKUP_DIR"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
backup_file="${BACKUP_DIR}/${DB_NAME}_${timestamp}.dump"
partial_file="${backup_file}.partial"
checksum_file="${backup_file}.sha256"
metadata_file="${backup_file}.meta"

cleanup_partial() {
  rm -f "$partial_file"
}
trap cleanup_partial EXIT

echo "Backing up ${DB_NAME} with ${PG_TOOL_MODE} PostgreSQL tools..."
pg_dump_database "$DB_NAME" "$partial_file"
if [[ ! -s "$partial_file" ]]; then
  echo "ERROR: Backup file is empty." >&2
  exit 1
fi
mv "$partial_file" "$backup_file"

checksum="$(file_sha256 "$backup_file")"
printf '%s  %s\n' "$checksum" "$(basename "$backup_file")" >"$checksum_file"
{
  printf 'database=%s\n' "$DB_NAME"
  printf 'created_at=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'tool_mode=%s\n' "$PG_TOOL_MODE"
  printf 'sha256=%s\n' "$checksum"
  printf 'size_bytes=%s\n' "$(wc -c <"$backup_file" | tr -d ' ')"
} >"$metadata_file"

shopt -s nullglob
backup_files=("${BACKUP_DIR}/${DB_NAME}_"*.dump)
if (( ${#backup_files[@]} > BACKUP_RETENTION_COUNT )); then
  ls -1t "${backup_files[@]}" | tail -n "+$((BACKUP_RETENTION_COUNT + 1))" | while IFS= read -r old_file; do
    rm -f "$old_file" "${old_file}.sha256" "${old_file}.meta"
  done
fi

echo "Backup completed: ${backup_file}"
echo "SHA-256: ${checksum}"

if [[ -n "$BACKUP_METRICS_FILE" ]]; then
  mkdir -p "$(dirname "$BACKUP_METRICS_FILE")"
  metrics_partial="${BACKUP_METRICS_FILE}.partial.$$"
  {
    printf '# HELP remotehelpdesk_database_backup_last_success_unixtime Unix timestamp of the latest successful database backup.\n'
    printf '# TYPE remotehelpdesk_database_backup_last_success_unixtime gauge\n'
    printf 'remotehelpdesk_database_backup_last_success_unixtime %s\n' "$(date +%s)"
    printf '# HELP remotehelpdesk_database_backup_size_bytes Size of the latest successful database backup.\n'
    printf '# TYPE remotehelpdesk_database_backup_size_bytes gauge\n'
    printf 'remotehelpdesk_database_backup_size_bytes %s\n' "$(wc -c <"$backup_file" | tr -d ' ')"
  } >"$metrics_partial"
  mv "$metrics_partial" "$BACKUP_METRICS_FILE"
fi
