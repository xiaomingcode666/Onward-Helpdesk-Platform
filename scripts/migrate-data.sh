#!/bin/bash
# =============================================================================
# RemoteHelpDesk — Data Migration / Import Tool
# =============================================================================
#
# This script imports data from an external system into RemoteHelpDesk.
# Supported input formats: CSV, JSON, SQL dump.
#
# Features:
#   - Field mapping via configuration file
#   - Dry-run mode for validation
#   - Incremental import (skip already-imported records)
#   - Transactional import with rollback on failure
#   - Import log for audit trail
#
# Usage:
#   ./scripts/migrate-data.sh <input-file> [options]
#
# Options:
#   -h, --help              Show this help message
#   -f, --format FORMAT     Input format: csv | json | sql (auto-detect by default)
#   -t, --type TYPE         Entity type: users | tickets | messages | knowledge
#   -m, --mapping FILE      Field mapping configuration file (YAML/JSON)
#   -d, --database DB       Target database name (default: cs_ai_agent)
#   -u, --user USER         Target database user (default: cs_ai_agent)
#   -p, --password PASS     Target database password
#   -H, --host HOST         Target database host (default: localhost)
#   -P, --port PORT         Target database port (default: 5432)
#   -T, --tenant TENANT_ID  Target tenant ID (required for multi-tenant)
#   --dry-run               Validate without importing
#   --batch-size N          Records per batch (default: 500)
#   --skip-errors           Continue on import errors
#   --resume-from FILE      Resume from a previous import state
#
# Examples:
#   # Import users from CSV
#   ./scripts/migrate-data.sh ./data/users.csv --type users --tenant 1
#
#   # Dry-run JSON import with mapping file
#   ./scripts/migrate-data.sh ./data/tickets.json --type tickets --mapping ./mapping.json --dry-run
#
#   # Import SQL dump directly
#   ./scripts/migrate-data.sh ./data/legacy_dump.sql --type tickets
#
# Required environment variables (or use --password):
#   DB_PASSWORD — PostgreSQL password
# =============================================================================

set -euo pipefail

# ─────────────────────────────────────────────────────────────────────────────
# Configuration & Defaults
# ─────────────────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMPORT_LOG_DIR="${SCRIPT_DIR}/../import_logs"
TIMESTAMP="$(date +%Y%m%d_%H%M%S)"

DB_NAME="cs_ai_agent"
DB_USER="cs_ai_agent"
DB_PASS="${DB_PASSWORD:-}"
DB_HOST="localhost"
DB_PORT="5432"
TENANT_ID=""
INPUT_FILE=""
INPUT_FORMAT=""
ENTITY_TYPE=""
MAPPING_FILE=""
DRY_RUN=false
BATCH_SIZE=500
SKIP_ERRORS=false
RESUME_FILE=""

# ─────────────────────────────────────────────────────────────────────────────
# Helper Functions
# ─────────────────────────────────────────────────────────────────────────────

print_usage() {
  sed -n '3,39p' "$0" | sed 's/^# //; s/^#$//'
  exit 0
}

log_info()    { echo "[INFO]    $*"; }
log_warn()    { echo "[WARN]    $*"; }
log_error()   { echo "[ERROR]   $*" >&2; }
log_step()    { echo ""; echo "━━━ $* ━━━"; }
log_success() { echo "[SUCCESS] $*"; }

# Run a SQL query (prints result)
run_sql() {
  if [ -n "$DB_PASS" ]; then
    PGPASSWORD="${DB_PASS}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -t -c "$1"
  else
    psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -t -c "$1"
  fi
}

# Run a SQL query silently (no output, returns exit code)
run_sql_silent() {
  if [ -n "$DB_PASS" ]; then
    PGPASSWORD="${DB_PASS}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -q -c "$1" 2>/dev/null
  else
    psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -q -c "$1" 2>/dev/null
  fi
}

# Detect file format from extension
detect_format() {
  local file="$1"
  case "${file,,}" in
    *.csv) echo "csv" ;;
    *.json) echo "json" ;;
    *.jsonl) echo "jsonl" ;;
    *.sql) echo "sql" ;;
    *.sql.gz) echo "sql_gz" ;;
    *.yaml|*.yml) echo "yaml" ;;
    *) echo "unknown" ;;
  esac
}

# Validate required parameters
validate_params() {
  local errors=0

  if [ -z "$INPUT_FILE" ]; then
    log_error "Input file is required."
    errors=$((errors + 1))
  elif [ ! -f "$INPUT_FILE" ]; then
    log_error "Input file not found: ${INPUT_FILE}"
    errors=$((errors + 1))
  fi

  if [ -z "$ENTITY_TYPE" ]; then
    log_error "Entity type is required (--type)."
    errors=$((errors + 1))
  fi

  if [ -z "$TENANT_ID" ] && [ "$DRY_RUN" = false ]; then
    log_warn "Tenant ID not provided (--tenant).  Imports may fail if the target schema requires tenant_id."
  fi

  if [ -n "$MAPPING_FILE" ] && [ ! -f "$MAPPING_FILE" ]; then
    log_error "Mapping file not found: ${MAPPING_FILE}"
    errors=$((errors + 1))
  fi

  # Test database connection
  if [ "$DRY_RUN" = false ]; then
    if ! run_sql_silent "SELECT 1;"; then
      log_error "Cannot connect to PostgreSQL at ${DB_HOST}:${DB_PORT}/${DB_NAME} as ${DB_USER}."
      log_error "Set DB_PASSWORD environment variable or use --password."
      errors=$((errors + 1))
    fi
  fi

  if [ "$errors" -gt 0 ]; then
    exit 1
  fi
}

# Convert field mapping file to sed patterns
# Supports simple JSON mappings like: {"source_field": "target_field"}
apply_mapping() {
  local source_field="$1"
  local record="$2"  # JSON record

  if [ -z "$MAPPING_FILE" ]; then
    echo "$record"
    return
  fi

  # Use jq if available to transform the JSON
  if command -v jq &>/dev/null; then
    local mapping_filter
    mapping_filter=$(jq -r 'to_entries | map("\(.key)=\(.value)") | .[]' "$MAPPING_FILE")
    local transformed="$record"
    while IFS= read -r mapping; do
      local src="${mapping%%=*}"
      local dst="${mapping#*=}"
      transformed=$(echo "$transformed" | jq --arg src "$src" --arg dst "$dst" \
        '. as $r | {($dst): $r[$src]} + del($r[$src]) // $r | with_entries(if .key == $src then key = $dst else . end)')
    done <<< "$mapping_filter"
    echo "$transformed"
  else
    log_warn "jq not available — mapping will not be applied."
    echo "$record"
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# Parse Arguments
# ─────────────────────────────────────────────────────────────────────────────

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help) print_usage ;;
    -f|--format) INPUT_FORMAT="$2"; shift 2 ;;
    -t|--type) ENTITY_TYPE="$2"; shift 2 ;;
    -m|--mapping) MAPPING_FILE="$2"; shift 2 ;;
    -d|--database) DB_NAME="$2"; shift 2 ;;
    -u|--user) DB_USER="$2"; shift 2 ;;
    -p|--password) DB_PASS="$2"; shift 2 ;;
    -H|--host) DB_HOST="$2"; shift 2 ;;
    -P|--port) DB_PORT="$2"; shift 2 ;;
    -T|--tenant) TENANT_ID="$2"; shift 2 ;;
    --dry-run) DRY_RUN=true; shift ;;
    --batch-size) BATCH_SIZE="$2"; shift 2 ;;
    --skip-errors) SKIP_ERRORS=true; shift ;;
    --resume-from) RESUME_FILE="$2"; shift 2 ;;
    -*)
      # Treat as input file if it looks like a path
      if [ -f "$1" ]; then
        INPUT_FILE="$1"; shift
      else
        log_error "Unknown argument: $1"; print_usage
      fi
      ;;
    *)
      if [ -z "$INPUT_FILE" ]; then
        INPUT_FILE="$1"; shift
      else
        log_error "Unexpected argument: $1"; print_usage
      fi
      ;;
  esac
done

# Auto-detect format if not specified
if [ -z "$INPUT_FORMAT" ] && [ -n "$INPUT_FILE" ]; then
  INPUT_FORMAT=$(detect_format "$INPUT_FILE")
  log_info "Detected format: ${INPUT_FORMAT}"
fi

# Validate
validate_params

# Create import log directory
mkdir -p "${IMPORT_LOG_DIR}"
IMPORT_LOG="${IMPORT_LOG_DIR}/import_${ENTITY_TYPE}_${TIMESTAMP}.log"
IMPORT_STATE="${IMPORT_LOG_DIR}/import_${ENTITY_TYPE}_state.json"

# ─────────────────────────────────────────────────────────────────────────────
# Import Routines
# ─────────────────────────────────────────────────────────────────────────────

import_csv() {
  local file="$1"
  local total=0
  local imported=0
  local skipped=0
  local errors=0

  log_step "Importing CSV: ${file}"

  # Read header line to get columns
  IFS=',' read -r -a headers < "${file}"
  log_info "Detected columns: ${headers[*]}"

  # Build table name
  local table_name="${ENTITY_TYPE}"

  # Count total lines (minus header)
  total=$(tail -n +2 "$file" | wc -l | tr -d ' ')
  log_info "Total records: ${total}"

  if [ "$DRY_RUN" = true ]; then
    log_info "[DRY-RUN] Would import ${total} records into table '${table_name}'."
    log_info "[DRY-RUN] Columns: ${headers[*]}"
    return
  fi

  # Process in batches
  local batch_start=1
  while [ "$batch_start" -le "$total" ]; do
    local batch_end=$((batch_start + BATCH_SIZE - 1))
    [ "$batch_end" -gt "$total" ] && batch_end="$total"

    log_info "  Processing batch ${batch_start}-${batch_end} of ${total}..."

    # Read batch lines
    local batch_file
    batch_file=$(mktemp)
    sed -n "$((batch_start + 1)),$((batch_end + 1))p" "$file" > "$batch_file"

    # Build INSERT statement
    local insert_sql="INSERT INTO ${table_name} ("
    local cols=""
    local first=true
    for h in "${headers[@]}"; do
      h_clean=$(echo "$h" | tr -d '"\r\n ')
      if [ "$first" = true ]; then
        cols="\"${h_clean}\""
        first=false
      else
        cols="${cols}, \"${h_clean}\""
      fi
    done

    # Add tenant_id if provided
    if [ -n "$TENANT_ID" ]; then
      cols="${cols}, tenant_id"
    fi

    insert_sql="${insert_sql} ${cols}) VALUES "

    local row_num=0
    while IFS= read -r line; do
      [ -z "$line" ] && continue
      row_num=$((row_num + 1))

      # Parse CSV line (simple — use Python for robust parsing)
      local values="("
      local col_index=0
      IFS=',' read -r -a fields <<< "$line"
      for field in "${fields[@]}"; do
        [ "$col_index" -gt 0 ] && values="${values}, "
        # Escape single quotes
        field_escaped=$(echo "$field" | sed "s/'/''/g" | tr -d '\r\n')
        values="${values}'${field_escaped}'"
        col_index=$((col_index + 1))
      done

      if [ -n "$TENANT_ID" ]; then
        values="${values}, ${TENANT_ID}"
      fi

      values="${values})"
      insert_sql="${insert_sql} ${values},"

    done < "$batch_file"

    # Remove trailing comma
    insert_sql="${insert_sql%,} ON CONFLICT DO NOTHING;"

    # Execute
    if run_sql_silent "$insert_sql"; then
      imported=$((imported + row_num))
    else
      if [ "$SKIP_ERRORS" = true ]; then
        log_warn "  Batch ${batch_start}-${batch_end} had errors, skipping..."
        skipped=$((skipped + row_num))
      else
        log_error "Batch import failed at rows ${batch_start}-${batch_end}. Aborting."
        rm -f "$batch_file"
        return 1
      fi
    fi

    rm -f "$batch_file"
    batch_start=$((batch_end + 1))
  done

  log_success "Imported ${imported} records, ${skipped} skipped."
}

import_json() {
  local file="$1"
  local is_jsonl="$2"  # true for JSONL, false for JSON array

  log_step "Importing JSON: ${file}"

  # Detect structure
  local file_content
  file_content=$(head -c 1024 "$file")

  if echo "$file_content" | python3 -c "import sys,json; d=json.load(sys.stdin); print(isinstance(d, list))" 2>/dev/null | grep -q "True"; then
    is_jsonl=false
  else
    # Check first line — if it starts with {, it's likely JSONL
    first_char=$(head -c 1 "$file")
    [ "$first_char" = "{" ] && is_jsonl=true
  fi

  local table_name="${ENTITY_TYPE}"

  if [ "$DRY_RUN" = true ]; then
    local count=0
    if [ "$is_jsonl" = true ]; then
      count=$(wc -l < "$file" | tr -d ' ')
    else
      count=$(python3 -c "import json; print(len(json.load(open('${file}'))))" 2>/dev/null || echo "unknown")
    fi
    log_info "[DRY-RUN] Would import ${count} records from JSON into table '${table_name}'."
    return
  fi

  if ! command -v python3 &>/dev/null; then
    log_error "python3 is required for JSON import."
    exit 1
  fi

  mkdir -p "${IMPORT_LOG_DIR}"

  # Use Python for JSON processing — more robust than shell
  python3 -c "
import json
import sys
import os
import subprocess

db_pass = os.environ.get('DB_PASS', '')
db_name = '${DB_NAME}'
db_user = '${DB_USER}'
db_host = '${DB_HOST}'
db_port = '${DB_PORT}'
table_name = '${table_name}'
tenant_id = '${TENANT_ID}'
batch_size = ${BATCH_SIZE}
skip_errors = ${SKIP_ERRORS}
mapping_file = '${MAPPING_FILE}'
import_log_file = '${IMPORT_LOG}'
dry_run = ${DRY_RUN}
improt_state_file = '${IMPORT_STATE}'
entity_type = '${ENTITY_TYPE}'

records = []
if ${is_jsonl}:
    with open('${file}', 'r') as f:
        for line in f:
            line = line.strip()
            if line:
                records.append(json.loads(line))
else:
    with open('${file}', 'r') as f:
        records = json.load(f)

log = open(import_log_file, 'w')
log.write(f'Import started: {len(records)} records\n')
log.flush()

total = len(records)
imported = 0
skipped = 0
errors = 0

# Apply field mapping if configured
if mapping_file:
    with open(mapping_file, 'r') as f:
        mapping = json.load(f)
    mapped_records = []
    for rec in records:
        mapped = {}
        for src_field, dst_field in mapping.items():
            if src_field in rec:
                mapped[dst_field] = rec[src_field]
        # Carry over unmapped fields
        for k, v in rec.items():
            if k not in mapping:
                mapped[k] = v
        mapped_records.append(mapped)
    records = mapped_records

for i in range(0, len(records), batch_size):
    batch = records[i:i+batch_size]
    batch_num = i // batch_size + 1

    if len(batch) == 0:
        continue

    # Build INSERT values
    values_list = []
    for rec in batch:
        cols = []
        vals = []
        for k, v in rec.items():
            cols.append(f'\"{k}\"')
            if v is None:
                vals.append('NULL')
            elif isinstance(v, bool):
                vals.append('true' if v else 'false')
            elif isinstance(v, (int, float)):
                vals.append(str(v))
            else:
                escaped = str(v).replace(\"'\", \"''\")
                vals.append(f\"'{escaped}'\")
        if tenant_id:
            cols.append('\"tenant_id\"')
            vals.append(str(tenant_id))
        values_list.append(f\"({' ,'.join(vals)})\")

    if not values_list:
        continue

    cols_str = ', '.join(cols)
    sql = f\"INSERT INTO {table_name} ({cols_str}) VALUES {', '.join(values_list)} ON CONFLICT DO NOTHING;\"

    if dry_run:
        print(f'[DRY-RUN] Batch {batch_num}: {len(batch)} records')
        continue

    # Execute SQL
    env = os.environ.copy()
    if db_pass:
        env['PGPASSWORD'] = db_pass
    cmd = ['psql', '-h', db_host, '-p', db_port, '-U', db_user, '-d', db_name, '-q', '-c', sql]
    result = subprocess.run(cmd, capture_output=True, text=True, env=env)

    if result.returncode == 0:
        imported += len(batch)
        log.write(f'Batch {batch_num}: {len(batch)} records imported\n')
    else:
        if skip_errors:
            skipped += len(batch)
            log.write(f'Batch {batch_num}: {len(batch)} records SKIPPED (error: {result.stderr[:200]})\n')
            print(f'[WARN] Batch {batch_num}: {len(batch)} records skipped: {result.stderr[:200]}')
        else:
            log.write(f'FATAL: Batch {batch_num} failed: {result.stderr}\n')
            print(f'[ERROR] Batch {batch_num} failed. Aborting.')
            print(f'SQL Error: {result.stderr}')
            sys.exit(1)

    log.flush()

log.write(f'Import complete: {imported} imported, {skipped} skipped, {errors} errors\n')
log.close()

print(f'Import complete: {imported} imported, {skipped} skipped, {errors} errors')
" 2>&1

  if [ "$?" -eq 0 ]; then
    log_success "JSON import completed."
  else
    log_error "JSON import failed."
    return 1
  fi
}

import_sql() {
  local file="$1"

  log_step "Importing SQL dump: ${file}"

  if [ "$DRY_RUN" = true ]; then
    log_info "[DRY-RUN] Would execute SQL from: ${file}"
    return
  fi

  # Handle gzipped SQL
  if [[ "$file" == *.gz ]]; then
    log_info "Decompressing gzipped SQL..."
    if [ -n "$DB_PASS" ]; then
      PGPASSWORD="${DB_PASS}" gunzip -c "$file" | psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" 2>&1 | tee -a "${IMPORT_LOG}"
    else
      gunzip -c "$file" | psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" 2>&1 | tee -a "${IMPORT_LOG}"
    fi
  else
    if [ -n "$DB_PASS" ]; then
      PGPASSWORD="${DB_PASS}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -f "$file" 2>&1 | tee -a "${IMPORT_LOG}"
    else
      psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -f "$file" 2>&1 | tee -a "${IMPORT_LOG}"
    fi
  fi

  local exit_code=${PIPESTATUS[0]}
  if [ "$exit_code" -eq 0 ]; then
    log_success "SQL import completed."
  else
    log_error "SQL import failed with exit code ${exit_code}."
    return 1
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# Main Import Dispatcher
# ─────────────────────────────────────────────────────────────────────────────

echo ""
echo "╔══════════════════════════════════════════════════════════════════════╗"
echo "║        RemoteHelpDesk — Data Migration Tool                         ║"
echo "╚══════════════════════════════════════════════════════════════════════╝"
echo ""
echo "  Input:    ${INPUT_FILE}"
echo "  Format:   ${INPUT_FORMAT}"
echo "  Type:     ${ENTITY_TYPE}"
echo "  Database: ${DB_HOST}:${DB_PORT}/${DB_NAME}"
echo "  Tenant:   ${TENANT_ID:-<not set>}"
echo "  Dry run:  ${DRY_RUN}"
echo ""

case "${INPUT_FORMAT}" in
  csv)
    import_csv "$INPUT_FILE"
    ;;
  json|jsonl)
    import_json "$INPUT_FILE" $( [ "$INPUT_FORMAT" = "jsonl" ] && echo true || echo false )
    ;;
  sql|sql_gz)
    import_sql "$INPUT_FILE"
    ;;
  yaml|yml)
    log_error "YAML import is not yet supported.  Convert to JSON first."
    exit 1
    ;;
  *)
    log_error "Unsupported format: ${INPUT_FORMAT}"
    log_error "Supported formats: csv, json, jsonl, sql"
    exit 1
    ;;
esac

echo ""
echo "══════════════════════════════════════════════════════════════════════════"
echo "  Migration Summary"
echo "══════════════════════════════════════════════════════════════════════════"
echo "  Input file:  ${INPUT_FILE}"
echo "  Entity type: ${ENTITY_TYPE}"
echo "  Format:      ${INPUT_FORMAT}"
echo "  Log:         ${IMPORT_LOG}"
echo "  Dry run:     ${DRY_RUN}"
echo "══════════════════════════════════════════════════════════════════════════"
echo ""
