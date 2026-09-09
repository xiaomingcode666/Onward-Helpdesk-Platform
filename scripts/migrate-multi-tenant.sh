#!/bin/bash
# =============================================================================
# RemoteHelpDesk — Single-Tenant to Multi-Tenant Data Migration Script
# =============================================================================
#
# This script migrates a single-tenant RemoteHelpDesk database to a multi-tenant
# RemoteHelpDesk schema.  It performs the following steps:
#
#   1. Backs up the current database (always take a backup first!)
#   2. Creates a default tenant record
#   3. Adds tenant_id columns to all existing tables
#   4. Sets tenant_id on existing data to the default tenant
#   5. Updates foreign key constraints to include tenant_id
#   6. Creates composite indexes (tenant_id + original)
#   7. Validates data integrity
#
# Usage:
#   ./scripts/migrate-multi-tenant.sh [options]
#
# Options:
#   -h, --help              Show this help message
#   -d, --database DB       Database name (default: cs_ai_agent)
#   -u, --user USER         Database user (default: cs_ai_agent)
#   -p, --password PASS     Database password (prompts if not provided)
#   -H, --host HOST         Database host (default: localhost)
#   -P, --port PORT         Database port (default: 5432)
#   -t, --tenant-name NAME  Default tenant name (default: "Default Tenant")
#   -s, --tenant-slug SLUG  Default tenant slug (default: "default")
#   --dry-run               Print SQL statements without executing
#
# Requirements:
#   - PostgreSQL client (psql)
#   - pg_dump (for backup)
#   - Read/write access to the database
#
# IMPORTANT: Run this during a maintenance window.  The script acquires
# exclusive locks and may block other writers.
# =============================================================================

set -euo pipefail

# ─────────────────────────────────────────────────────────────────────────────
# Configuration & Defaults
# ─────────────────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKUP_DIR="${SCRIPT_DIR}/../backups"
TIMESTAMP="$(date +%Y%m%d_%H%M%S)"

DB_NAME="cs_ai_agent"
DB_USER="cs_ai_agent"
DB_PASS=""
DB_HOST="localhost"
DB_PORT="5432"
TENANT_NAME="Default Tenant"
TENANT_SLUG="default"
DRY_RUN=false

# Tables that likely exist in RemoteHelpDesk; extend this array as needed.
TABLES=(
  "users"
  "tickets"
  "conversations"
  "messages"
  "attachments"
  "knowledge_base"
  "knowledge_base_items"
  "sessions"
  "api_keys"
  "settings"
  "audit_logs"
  "agents"
  "agent_configs"
  "workflows"
  "workflow_steps"
  "webhooks"
  "webhook_events"
  "templates"
  "notifications"
  "tags"
  "ticket_tags"
)

# ─────────────────────────────────────────────────────────────────────────────
# Helper Functions
# ─────────────────────────────────────────────────────────────────────────────

print_usage() {
  sed -n '3,30p' "$0" | sed 's/^# //; s/^#$//'
  exit 0
}

log_info()  { echo "[INFO]  $*"; }
log_warn()  { echo "[WARN]  $*"; }
log_error() { echo "[ERROR] $*" >&2; }
log_step()  { echo ""; echo "━━━ $* ━━━"; }

# Run a SQL query, respecting dry-run mode
run_sql() {
  if [ "$DRY_RUN" = true ]; then
    echo "[DRY-RUN] $1"
  else
    if [ -n "$DB_PASS" ]; then
      PGPASSWORD="${DB_PASS}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -t -c "$1"
    else
      psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -t -c "$1"
    fi
  fi
}

# Check if a table exists in the database
table_exists() {
  local table="$1"
  local result
  if [ -n "$DB_PASS" ]; then
    result=$(PGPASSWORD="${DB_PASS}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -t -c \
      "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = '${table}');")
  else
    result=$(psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -t -c \
      "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = '${table}');")
  fi
  result=$(echo "$result" | tr -d '[:space:]')
  [ "$result" = "t" ]
}

# Check if a column exists in a table
column_exists() {
  local table="$1"
  local column="$2"
  local result
  if [ -n "$DB_PASS" ]; then
    result=$(PGPASSWORD="${DB_PASS}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -t -c \
      "SELECT EXISTS (SELECT FROM information_schema.columns WHERE table_name = '${table}' AND column_name = '${column}');")
  else
    result=$(psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -t -c \
      "SELECT EXISTS (SELECT FROM information_schema.columns WHERE table_name = '${table}' AND column_name = '${column}');")
  fi
  result=$(echo "$result" | tr -d '[:space:]')
  [ "$result" = "t" ]
}

# ─────────────────────────────────────────────────────────────────────────────
# Parse Arguments
# ─────────────────────────────────────────────────────────────────────────────

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help) print_usage ;;
    -d|--database) DB_NAME="$2"; shift 2 ;;
    -u|--user) DB_USER="$2"; shift 2 ;;
    -p|--password) DB_PASS="$2"; shift 2 ;;
    -H|--host) DB_HOST="$2"; shift 2 ;;
    -P|--port) DB_PORT="$2"; shift 2 ;;
    -t|--tenant-name) TENANT_NAME="$2"; shift 2 ;;
    -s|--tenant-slug) TENANT_SLUG="$2"; shift 2 ;;
    --dry-run) DRY_RUN=true; shift ;;
    *) log_error "Unknown argument: $1"; print_usage ;;
  esac
done

# Prompt for password if not provided
if [ -z "$DB_PASS" ] && [ "$DRY_RUN" = false ]; then
  read -rsp "Enter PostgreSQL password for user '${DB_USER}': " DB_PASS
  echo ""
fi

# ─────────────────────────────────────────────────────────────────────────────
# Validation
# ─────────────────────────────────────────────────────────────────────────────

if ! command -v psql &>/dev/null; then
  log_error "psql is not installed.  Please install the PostgreSQL client."
  exit 1
fi

if ! command -v pg_dump &>/dev/null; then
  log_error "pg_dump is not installed.  Please install the PostgreSQL client."
  exit 1
fi

# Test database connection
log_info "Testing database connection..."
if ! run_sql "SELECT 1;" >/dev/null 2>&1; then
  log_error "Cannot connect to PostgreSQL at ${DB_HOST}:${DB_PORT}/${DB_NAME} as ${DB_USER}."
  exit 1
fi
log_info "Connection successful."

# =============================================================================
# MIGRATION STEPS
# =============================================================================

# ─────────────────────────────────────────────────────────────────────────────
# Step 1: Backup the current database
# ─────────────────────────────────────────────────────────────────────────────
log_step "Step 1/7: Backing up current database"

mkdir -p "${BACKUP_DIR}"
BACKUP_FILE="${BACKUP_DIR}/pre_migration_backup_${TIMESTAMP}.sql"
LOG_FILE="${BACKUP_DIR}/migration_${TIMESTAMP}.log"

if [ "$DRY_RUN" = false ]; then
  log_info "Backing up to: ${BACKUP_FILE}"
  if [ -n "$DB_PASS" ]; then
    PGPASSWORD="${DB_PASS}" pg_dump -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" \
      --format=custom \
      --file="${BACKUP_FILE}" \
      --verbose 2>&1 | tee -a "${LOG_FILE}"
  else
    pg_dump -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" \
      --format=custom \
      --file="${BACKUP_FILE}" \
      --verbose 2>&1 | tee -a "${LOG_FILE}"
  fi
  log_info "Backup saved to: ${BACKUP_FILE}"
else
  log_info "[DRY-RUN] Would backup to: ${BACKUP_FILE}"
fi

# ─────────────────────────────────────────────────────────────────────────────
# Step 2: Create the default tenant
# ─────────────────────────────────────────────────────────────────────────────
log_step "Step 2/7: Creating default tenant"

# Create tenants table if it doesn't exist
run_sql "
  CREATE TABLE IF NOT EXISTS tenants (
    id            BIGSERIAL PRIMARY KEY,
    name          VARCHAR(255) NOT NULL,
    slug          VARCHAR(255) NOT NULL UNIQUE,
    domain        VARCHAR(255),
    logo_url      TEXT,
    settings      JSONB DEFAULT '{}',
    is_active     BOOLEAN DEFAULT true,
    created_at    TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at    TIMESTAMP WITH TIME ZONE DEFAULT NOW()
  );
"

# Insert default tenant (if not already present)
run_sql "
  INSERT INTO tenants (slug, name, domain, settings)
  SELECT '${TENANT_SLUG}', '${TENANT_NAME}', '', '{}'
  WHERE NOT EXISTS (SELECT 1 FROM tenants WHERE slug = '${TENANT_SLUG}');
"

# Get the default tenant ID
DEFAULT_TENANT_ID=$(run_sql "SELECT id FROM tenants WHERE slug = '${TENANT_SLUG}';" | tr -d '[:space:]')
log_info "Default tenant ID: ${DEFAULT_TENANT_ID}"

# ─────────────────────────────────────────────────────────────────────────────
# Step 3: Add tenant_id columns to all existing tables
# ─────────────────────────────────────────────────────────────────────────────
log_step "Step 3/7: Adding tenant_id columns"

for table in "${TABLES[@]}"; do
  if table_exists "$table"; then
    if ! column_exists "$table" "tenant_id"; then
      log_info "  Adding tenant_id to: ${table}"
      run_sql "ALTER TABLE \"${table}\" ADD COLUMN tenant_id BIGINT REFERENCES tenants(id) ON DELETE CASCADE;"
    else
      log_info "  tenant_id already exists in: ${table} (skipping)"
    fi
  else
    log_info "  Table does not exist: ${table} (skipping)"
  fi
done

# ─────────────────────────────────────────────────────────────────────────────
# Step 4: Set tenant_id on existing data to the default tenant
# ─────────────────────────────────────────────────────────────────────────────
log_step "Step 4/7: Setting tenant_id on existing data"

for table in "${TABLES[@]}"; do
  if table_exists "$table" && column_exists "$table" "tenant_id"; then
    log_info "  Setting tenant_id = ${DEFAULT_TENANT_ID} for: ${table}"
    run_sql "UPDATE \"${table}\" SET tenant_id = ${DEFAULT_TENANT_ID} WHERE tenant_id IS NULL;"
  fi
done

# Add NOT NULL constraints where appropriate
for table in "${TABLES[@]}"; do
  if table_exists "$table" && column_exists "$table" "tenant_id"; then
    log_info "  Adding NOT NULL constraint to: ${table}.tenant_id"
    run_sql "ALTER TABLE \"${table}\" ALTER COLUMN tenant_id SET NOT NULL;"
  fi
done

# ─────────────────────────────────────────────────────────────────────────────
# Step 5: Update foreign key constraints (if any reference other tenant-scoped tables)
# ─────────────────────────────────────────────────────────────────────────────
log_step "Step 5/7: Updating foreign key constraints"

# Drop existing foreign key constraints that need tenant awareness
# This is table-specific — adjust based on your schema
log_info "Dropping and recreating foreign keys with tenant_id..."

# Get all foreign key constraints
FKS=$(run_sql "
  SELECT
    con.conname AS constraint_name,
    conrelid::regclass AS source_table,
    (SELECT array_agg(col.attname) FROM pg_attribute col
      WHERE col.attrelid = con.conrelid AND col.attnum = ANY(con.conkey)) AS source_columns,
    confrelid::regclass AS target_table,
    (SELECT array_agg(col.attname) FROM pg_attribute col
      WHERE col.attrelid = con.confrelid AND col.attnum = ANY(con.confkey)) AS target_columns
  FROM pg_constraint con
  WHERE con.contype = 'f'
    AND con.connamespace = (SELECT nsp.oid FROM pg_namespace nsp WHERE nsp.nspname = 'public')
  ORDER BY con.conname;
")

log_info "Foreign keys found:"
log_info "${FKS}"

# Note: Automated FK migration is schema-dependent.  This script logs the
# current FKs for review.  Manual adjustment may be needed for composite FKs.
log_warn "Review foreign keys above.  If tables reference each other across tenants,"
log_warn "you need to add tenant_id to the composite foreign key.  Example:"
log_warn "  ALTER TABLE ticket_tags DROP CONSTRAINT fk_ticket_tags_ticket;"
log_warn "  ALTER TABLE ticket_tags ADD CONSTRAINT fk_ticket_tags_ticket"
log_warn "    FOREIGN KEY (ticket_id, tenant_id) REFERENCES tickets(id, tenant_id);"

# ─────────────────────────────────────────────────────────────────────────────
# Step 6: Create composite indexes (include tenant_id)
# ─────────────────────────────────────────────────────────────────────────────
log_step "Step 6/7: Creating composite indexes"

# Common indexing patterns for multi-tenant tables
declare -A INDEX_COLUMNS
INDEX_COLUMNS=(
  ["users"]="(tenant_id, email)"
  ["users"]="(tenant_id, id)"
  ["tickets"]="(tenant_id, id)"
  ["tickets"]="(tenant_id, status)"
  ["tickets"]="(tenant_id, created_at DESC)"
  ["conversations"]="(tenant_id, id)"
  ["conversations"]="(tenant_id, ticket_id)"
  ["messages"]="(tenant_id, conversation_id, created_at)"
  ["knowledge_base"]="(tenant_id, id)"
  ["knowledge_base_items"]="(tenant_id, kb_id)"
  ["sessions"]="(tenant_id, token)"
  ["api_keys"]="(tenant_id, key_hash)"
)

for table in "${!INDEX_COLUMNS[@]}"; do
  columns="${INDEX_COLUMNS[$table]}"
  # Create a safe index name from the columns
  index_name_suffix=$(echo "$columns" | sed 's/[^a-zA-Z0-9_,]/_/g' | tr -d '()')
  idx_name="idx_${table}_tenant_${index_name_suffix}"

  if table_exists "$table" && column_exists "$table" "tenant_id"; then
    log_info "  Creating index: ${idx_name}"
    # Use a CONCURRENTLY approach for production (requires outside transaction)
    run_sql "CREATE INDEX IF NOT EXISTS \"${idx_name}\" ON \"${table}\" ${columns};"
  fi
done

# ─────────────────────────────────────────────────────────────────────────────
# Step 7: Validate Data Integrity
# ─────────────────────────────────────────────────────────────────────────────
log_step "Step 7/7: Validating data integrity"

if [ "$DRY_RUN" = false ]; then
  errors=0

  # Check that all tables with tenant_id have it set
  for table in "${TABLES[@]}"; do
    if table_exists "$table" && column_exists "$table" "tenant_id"; then
      null_count=$(run_sql "SELECT COUNT(*) FROM \"${table}\" WHERE tenant_id IS NULL;" | tr -d '[:space:]')
      if [ "$null_count" != "0" ] && [ -n "$null_count" ]; then
        log_error "  Table '${table}' has ${null_count} rows with NULL tenant_id!"
        errors=$((errors + 1))
      else
        row_count=$(run_sql "SELECT COUNT(*) FROM \"${table}\";" | tr -d '[:space:]')
        log_info "  Table '${table}': ${row_count} rows, all with valid tenant_id."
      fi
    fi
  done

  # Verify default tenant exists
  tenant_check=$(run_sql "SELECT COUNT(*) FROM tenants WHERE id = ${DEFAULT_TENANT_ID};" | tr -d '[:space:]')
  if [ "$tenant_check" != "1" ] && [ -n "$tenant_check" ]; then
    log_error "Default tenant (ID: ${DEFAULT_TENANT_ID}) not found!"
    errors=$((errors + 1))
  else
    log_info "Default tenant '${TENANT_NAME}' (ID: ${DEFAULT_TENANT_ID}) exists and is valid."
  fi

  # Verify tenant_id foreign key integrity
  for table in "${TABLES[@]}"; do
    if table_exists "$table" && column_exists "$table" "tenant_id"; then
      orphaned=$(run_sql "SELECT COUNT(*) FROM \"${table}\" t LEFT JOIN tenants ten ON t.tenant_id = ten.id WHERE ten.id IS NULL;" | tr -d '[:space:]')
      if [ "$orphaned" != "0" ] && [ -n "$orphaned" ]; then
        log_error "  Table '${table}' has ${orphaned} orphaned tenant_id references!"
        errors=$((errors + 1))
      fi
    fi
  done

  if [ "$errors" -eq 0 ]; then
    log_info "All integrity checks passed!"
  else
    log_warn "${errors} integrity issue(s) found.  Review the log above."
  fi
else
  log_info "[DRY-RUN] Validation skipped in dry-run mode."
fi

# ─────────────────────────────────────────────────────────────────────────────
# Summary
# ─────────────────────────────────────────────────────────────────────────────
echo ""
echo "══════════════════════════════════════════════════════════════════════════"
echo "  Migration Summary"
echo "══════════════════════════════════════════════════════════════════════════"
if [ "$DRY_RUN" = true ]; then
  echo "  Mode:     DRY RUN (no changes made)"
else
  echo "  Mode:     LIVE"
  echo "  Backup:   ${BACKUP_FILE}"
fi
echo "  Database: ${DB_HOST}:${DB_PORT}/${DB_NAME}"
echo "  Tenant:   ${TENANT_NAME} (slug: ${TENANT_SLUG}, ID: ${DEFAULT_TENANT_ID})"
echo "══════════════════════════════════════════════════════════════════════════"
echo ""
log_info "Migration script completed."
log_info "To roll back, run: pg_restore -h ${DB_HOST} -p ${DB_PORT} -U ${DB_USER} -d ${DB_NAME} --clean ${BACKUP_FILE}"
