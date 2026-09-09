#!/usr/bin/env bash
# Restores a backup into an isolated temporary database, validates core tables,
# and removes the drill database. It never modifies DB_NAME.

set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "Usage: ./scripts/drill-db-restore.sh <backup_file.dump>" >&2
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
KEEP_DRILL_DB="${KEEP_DRILL_DB:-0}"
RESTORE_DRILL_METRICS_FILE="${RESTORE_DRILL_METRICS_FILE:-}"
DRILL_DB_NAME="${DRILL_DB_NAME:-rhd_restore_drill_$(date -u +%Y%m%dT%H%M%S)_${RANDOM}}"

if [[ ! -s "$BACKUP_FILE" ]]; then
  echo "ERROR: Backup file is missing or empty: ${BACKUP_FILE}" >&2
  exit 1
fi
if [[ "$DRILL_DB_NAME" == "$DB_NAME" ]]; then
  echo "ERROR: DRILL_DB_NAME must not equal DB_NAME." >&2
  exit 1
fi

init_pg_tools
validate_pg_identifier "$DRILL_DB_NAME" "DRILL_DB_NAME"

if [[ -f "${BACKUP_FILE}.sha256" ]]; then
  expected_checksum="$(awk 'NR == 1 {print $1}' "${BACKUP_FILE}.sha256")"
  actual_checksum="$(file_sha256 "$BACKUP_FILE")"
  if [[ "$expected_checksum" != "$actual_checksum" ]]; then
    echo "ERROR: Backup checksum mismatch." >&2
    exit 1
  fi
fi

cleanup() {
  if [[ "$KEEP_DRILL_DB" == "1" ]]; then
    echo "Keeping drill database: ${DRILL_DB_NAME}"
    return
  fi
  if pg_database_exists "$DRILL_DB_NAME"; then
    pg_terminate_database_connections "$DRILL_DB_NAME"
    pg_drop_database "$DRILL_DB_NAME"
  fi
}
trap cleanup EXIT

if pg_database_exists "$DRILL_DB_NAME"; then
  echo "ERROR: Drill database already exists: ${DRILL_DB_NAME}" >&2
  exit 1
fi

started_at="$(date +%s)"
pg_create_database "$DRILL_DB_NAME"
pg_restore_database "$DRILL_DB_NAME" "$BACKUP_FILE"

required_tables=(t_tenant t_user t_conversation t_ticket t_knowledge_base t_ai_agent t_ai_workflow)
for table in "${required_tables[@]}"; do
  count="$(pg_query "$DRILL_DB_NAME" "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = '${table}'")"
  if [[ "$count" != "1" ]]; then
    echo "ERROR: Restored database is missing required table: ${table}" >&2
    exit 1
  fi
done

tenant_count="$(pg_query "$DRILL_DB_NAME" "SELECT count(*) FROM t_tenant")"
ticket_count="$(pg_query "$DRILL_DB_NAME" "SELECT count(*) FROM t_ticket")"
duration_seconds="$(( $(date +%s) - started_at ))"

echo "Restore drill passed."
echo "  temporary_database=${DRILL_DB_NAME}"
echo "  tenant_rows=${tenant_count}"
echo "  ticket_rows=${ticket_count}"
echo "  duration_seconds=${duration_seconds}"

if [[ -n "$RESTORE_DRILL_METRICS_FILE" ]]; then
  mkdir -p "$(dirname "$RESTORE_DRILL_METRICS_FILE")"
  metrics_partial="${RESTORE_DRILL_METRICS_FILE}.partial.$$"
  {
    printf '# HELP remotehelpdesk_restore_drill_last_success_unixtime Unix timestamp of the latest successful isolated restore drill.\n'
    printf '# TYPE remotehelpdesk_restore_drill_last_success_unixtime gauge\n'
    printf 'remotehelpdesk_restore_drill_last_success_unixtime %s\n' "$(date +%s)"
    printf '# HELP remotehelpdesk_restore_drill_duration_seconds Duration of the latest successful isolated restore drill.\n'
    printf '# TYPE remotehelpdesk_restore_drill_duration_seconds gauge\n'
    printf 'remotehelpdesk_restore_drill_duration_seconds %s\n' "$duration_seconds"
  } >"$metrics_partial"
  mv "$metrics_partial" "$RESTORE_DRILL_METRICS_FILE"
fi
