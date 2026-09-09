#!/usr/bin/env bash
# Usage: FORCE=1 ./scripts/restore-db.sh <backup_file.dump>

set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "Usage: FORCE=1 ./scripts/restore-db.sh <backup_file.dump>" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/postgres-tools.sh
source "$SCRIPT_DIR/lib/postgres-tools.sh"

BACKUP_FILE="$1"
DB_NAME="${DB_NAME:-cs_ai_agent}"
DB_USER="${DB_USER:-cs_ai_agent}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_PASSWORD="${DB_PASSWORD:-}"
FORCE="${FORCE:-0}"

if [[ ! -s "$BACKUP_FILE" ]]; then
  echo "ERROR: Backup file is missing or empty: ${BACKUP_FILE}" >&2
  exit 1
fi
if [[ "$FORCE" != "1" ]]; then
  echo "ERROR: Restore replaces database '${DB_NAME}'. Set FORCE=1 after confirming the target." >&2
  exit 1
fi

init_pg_tools
if [[ -f "${BACKUP_FILE}.sha256" ]]; then
  expected_checksum="$(awk 'NR == 1 {print $1}' "${BACKUP_FILE}.sha256")"
  actual_checksum="$(file_sha256 "$BACKUP_FILE")"
  if [[ "$expected_checksum" != "$actual_checksum" ]]; then
    echo "ERROR: Backup checksum mismatch." >&2
    exit 1
  fi
fi

if pg_database_exists "$DB_NAME"; then
  pg_terminate_database_connections "$DB_NAME"
  pg_drop_database "$DB_NAME"
fi
pg_create_database "$DB_NAME"
pg_restore_database "$DB_NAME" "$BACKUP_FILE"

echo "Restore completed: ${DB_NAME} from ${BACKUP_FILE}"
