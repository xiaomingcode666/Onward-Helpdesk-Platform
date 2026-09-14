#!/usr/bin/env bash
# Usage: ./scripts/backup-db.sh [backup_dir]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/postgres-tools.sh
source "$SCRIPT_DIR/lib/postgres-tools.sh"
# shellcheck source=lib/deployment-backup-identity.sh
source "$SCRIPT_DIR/lib/deployment-backup-identity.sh"

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

rhd_backup_check_target_identity
init_pg_tools
mkdir -p "$BACKUP_DIR"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)_$$_${RANDOM}"
backup_file="${BACKUP_DIR}/${DB_NAME}_${timestamp}.dump"
partial_file="${backup_file}.partial"
checksum_file="${backup_file}.sha256"
metadata_file="${backup_file}.meta"
config_file="${backup_file}.config.json"
config_partial="${config_file}.partial"
published=0

cleanup_partial() {
  rm -f "$partial_file"
  rm -f "$config_partial"
  if [[ "$published" != 1 ]]; then
    rm -f "$backup_file" "$checksum_file" "$metadata_file" "$config_file"
  fi
}
trap cleanup_partial EXIT

echo "Backing up ${DB_NAME} with ${PG_TOOL_MODE} PostgreSQL tools..."
if rhd_backup_config_managed; then
  # The mounted file may already contain the NEXT deployment's draft. Archive
  # the database's actual active version instead, then prove it stayed unchanged
  # throughout pg_dump's snapshot. Published versions cannot be reused/rewritten.
  rhd_backup_database_config "$DB_NAME" >"$config_partial"
  config_summary="$(rhd_backup_config_summary "$config_partial")"
  rhd_backup_check_config "$config_partial" "${config_summary#*|}"
  rhd_backup_verify_database_config "$DB_NAME" "$config_summary"
fi
pg_dump_database "$DB_NAME" "$partial_file"
if [[ ! -s "$partial_file" ]]; then
  echo "ERROR: Backup file is empty." >&2
  exit 1
fi
if rhd_backup_config_managed; then
  rhd_backup_verify_database_config "$DB_NAME" "$config_summary"
  chmod 600 "$config_partial"
  mv "$config_partial" "$config_file"
fi

checksum="$(file_sha256 "$partial_file")"
printf '%s  %s\n' "$checksum" "$(basename "$backup_file")" >"$checksum_file"
{
  printf 'database=%s\n' "$DB_NAME"
  printf 'created_at=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'tool_mode=%s\n' "$PG_TOOL_MODE"
  printf 'sha256=%s\n' "$checksum"
  printf 'size_bytes=%s\n' "$(wc -c <"$partial_file" | tr -d ' ')"
  rhd_backup_write_identity
  if rhd_backup_config_managed; then
    printf 'config_version_id=%s\n' "${config_summary%%|*}"
    printf 'config_digest=%s\n' "${config_summary#*|}"
    printf 'config_sha256=%s\n' "$(file_sha256 "$config_file")"
  fi
} >"$metadata_file"
# Publish the dump last: latest_backup must never select an incomplete archive.
mv "$partial_file" "$backup_file"
published=1
if [[ -n "${BACKUP_RESULT_FILE:-}" ]]; then
  printf '%s\n' "$backup_file" >"$BACKUP_RESULT_FILE"
fi

shopt -s nullglob
backup_files=("${BACKUP_DIR}/${DB_NAME}_"*.dump)
if (( ${#backup_files[@]} > BACKUP_RETENTION_COUNT )); then
  ls -1t "${backup_files[@]}" | tail -n "+$((BACKUP_RETENTION_COUNT + 1))" | while IFS= read -r old_file; do
    # A restore may target the oldest archive while taking its safety backup.
    [[ "$old_file" != "${BACKUP_PRESERVE_FILE:-}" ]] || continue
    rm -f "$old_file" "${old_file}.sha256" "${old_file}.meta" "${old_file}.config.json"
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
