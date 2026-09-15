#!/usr/bin/env bash
# Every Docker call is an inherited Bash stub. No database is contacted.
set -Eeuo pipefail
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
test_dir="$(mktemp -d "${TMPDIR:-/tmp}/rhd-backup-tests.XXXXXX")"
cleanup() {
  case "$test_dir" in /*/rhd-backup-tests.*) rm -rf -- "$test_dir" ;; esac
}
trap cleanup EXIT
export RHD_INSTANCE_ID=onward-acme-staging RHD_DEPLOYMENT_PROJECT_ID=acme
export RHD_PROJECT_ENVIRONMENT=staging RHD_PROJECT_CONFIG_TENANT_ID=11
export PG_TOOL_MODE=docker DB_NAME=cs_ai_agent DB_USER=cs_ai_agent
export STUB_TOOL_LOG="$test_dir/tools.log"
export SYNTHETIC_DRILL_COLLISION=0
export STUB_ACTIVE_VERSION=1
export STUB_DIGEST="$(printf 'a%.0s' {1..64})"
export RHD_PROJECT_CONFIG_FILE="$test_dir/instance/project-config/current.json"
export RHD_ENV_FILE="$test_dir/instance/.env"
export RHD_PROJECT_SECRET_DIR="$test_dir/instance/project-secrets"
export RHD_PROJECT_CONFIG_CHECKER="$test_dir/checker"
export RHD_FAKE_SECRET_FILE="$test_dir/secret"
mkdir -p "$test_dir/instance/project-config" "$RHD_PROJECT_SECRET_DIR"
printf 'fixture-secret\n' >"$RHD_FAKE_SECRET_FILE"
printf '#!/usr/bin/env bash\n[[ -r "$RHD_FAKE_SECRET_FILE" ]]\n' >"$RHD_PROJECT_CONFIG_CHECKER"
chmod 755 "$RHD_PROJECT_CONFIG_CHECKER"
printf '{"version_id":1,"digest":"%s","document":{"schema_version":1,"tenant_id":11,"environment":"staging","projects":[],"intake":{"rules":[]},"secret_refs":[]}}\n' "$STUB_DIGEST" >"$test_dir/config.json"
export STUB_CONFIG_FILE="$test_dir/config.json"
cp "$STUB_CONFIG_FILE" "$RHD_PROJECT_CONFIG_FILE"
for key in RHD_INSTANCE_ID RHD_DEPLOYMENT_PROJECT_ID RHD_PROJECT_ENVIRONMENT RHD_PROJECT_CONFIG_TENANT_ID; do
  printf '%s=%s\n' "$key" "${!key}"
done >"$RHD_ENV_FILE"
printf 'RHD_PROJECT_CONFIG_DIGEST=%s\n' "$STUB_DIGEST" >>"$RHD_ENV_FILE"

# Git Bash fixtures use native Windows Python; production commands run on Linux.
if ! command -v python3 >/dev/null 2>&1 && command -v cygpath >/dev/null 2>&1; then
  python3() {
    RHD_ENV_FILE="$(cygpath -m "$RHD_ENV_FILE")" \
    RHD_PROJECT_CONFIG_FILE="$(cygpath -m "$RHD_PROJECT_CONFIG_FILE")" \
    RHD_PROJECT_SECRET_DIR="$(cygpath -m "$RHD_PROJECT_SECRET_DIR")" python "$@"
  }
  export -f python3
fi

docker() {
  printf '%s\n' "$*" >>"$STUB_TOOL_LOG"
  case " $* " in
    *' ps '*) printf 'synthetic-postgres\n' ;;
    *' pg_dump '*)
      printf 'synthetic-database-content\n'
      if [[ "${STUB_CHANGE_DURING_DUMP:-0}" == 1 ]]; then touch "$STUB_CONFIG_FILE.changed"; fi
      ;;
    *' psql '*json_build_object*) cat "$STUB_CONFIG_FILE" ;;
    *' psql '*"v.id::text"*)
      if [[ -e "$STUB_CONFIG_FILE.changed" ]]; then printf '2|%s\n' "$STUB_DIGEST";
      else printf '%s|%s\n' "$STUB_ACTIVE_VERSION" "$STUB_DIGEST"; fi
      ;;
    *' psql '*information_schema*) printf '1\n' ;;
    *' psql '*pg_database*cs_ai_agent*) printf '1\n' ;;
    *' psql '*pg_database*) printf '%s\n' "$SYNTHETIC_DRILL_COLLISION" ;;
    *' psql '*) printf '1\n' ;;
  esac
}
export -f docker

bash "$repo/scripts/backup-db.sh" "$test_dir/managed" >"$test_dir/backup.log"
backups=("$test_dir/managed/"*.dump)
backup="${backups[0]}"
[[ -s "$backup" && -s "$backup.meta" ]]
for field in instance_id=onward-acme-staging deployment_project_id=acme environment=staging tenant_id=11; do
  grep -Fxq "$field" "$backup.meta"
done

expect_rejected() {
  local file="$1" label="$2" script
  for script in restore-db.sh drill-db-restore.sh; do
    : >"$STUB_TOOL_LOG"
    if FORCE=1 bash "$repo/scripts/$script" "$file" >"$test_dir/rejected.log" 2>&1; then
      printf 'FAIL: %s accepted %s\n' "$script" "$label" >&2
      exit 1
    fi
    [[ ! -s "$STUB_TOOL_LOG" ]] || {
      printf 'FAIL: %s accessed the database before rejecting %s\n' "$script" "$label" >&2
      exit 1
    }
  done
}

cp "$backup" "$test_dir/missing.dump"
expect_rejected "$test_dir/missing.dump" 'missing identity metadata'
cp "$backup" "$test_dir/invalid.dump"
cp "$backup.config.json" "$test_dir/invalid.dump.config.json"
for field in instance_id deployment_project_id environment tenant_id database sha256 config_version_id config_digest config_sha256; do
  cp "$backup" "$test_dir/invalid.dump"
  sed "/^${field}=/d" "$backup.meta" >"$test_dir/invalid.dump.meta"
  expect_rejected "$test_dir/invalid.dump" "missing $field"
done
cp "$backup.meta" "$test_dir/missing.dump.meta"
expect_rejected "$test_dir/missing.dump" 'missing configuration sidecar'
rm "$test_dir/missing.dump.meta"
cp "$backup.meta" "$test_dir/invalid.dump.meta"
printf 'broken json' >"$test_dir/invalid.dump.config.json"
expect_rejected "$test_dir/invalid.dump" 'damaged configuration sidecar'
cp "$backup.config.json" "$test_dir/invalid.dump.config.json"
mv "$RHD_FAKE_SECRET_FILE" "$RHD_FAKE_SECRET_FILE.saved"
expect_rejected "$backup" 'missing historical configuration secret'
mv "$RHD_FAKE_SECRET_FILE.saved" "$RHD_FAKE_SECRET_FILE"
for field in instance_id deployment_project_id environment tenant_id database; do
  cp "$backup.meta" "$test_dir/invalid.dump.meta"
  printf '%s=foreign-value\n' "$field" >>"$test_dir/invalid.dump.meta"
  expect_rejected "$test_dir/invalid.dump" "duplicate $field"
  sed "s/^${field}=.*/${field}=foreign-value/" "$backup.meta" >"$test_dir/invalid.dump.meta"
  expect_rejected "$test_dir/invalid.dump" "foreign $field"
done
cp "$backup.meta" "$test_dir/invalid.dump.meta"
printf 'different content\n' >>"$test_dir/invalid.dump"
expect_rejected "$test_dir/invalid.dump" 'content inconsistent with metadata'

# Compatible restore executes only the stub after validating the matching metadata.
# A pending V2 file must not replace the active V1 configuration archived by backup.
sed 's/"version_id":1/"version_id":2/' "$STUB_CONFIG_FILE" >"$RHD_PROJECT_CONFIG_FILE"
sed -i "s/RHD_PROJECT_CONFIG_DIGEST=.*/RHD_PROJECT_CONFIG_DIGEST=$(printf 'b%.0s' {1..64})/" "$RHD_ENV_FILE"
: >"$STUB_TOOL_LOG"
FORCE=1 bash "$repo/scripts/restore-db.sh" "$backup" >"$test_dir/restore.log"
grep -q 'dropdb.*cs_ai_agent' "$STUB_TOOL_LOG"
grep -q 'pg_restore.*cs_ai_agent' "$STUB_TOOL_LOG"
cmp "$backup.config.json" "$RHD_PROJECT_CONFIG_FILE"
grep -Fxq "RHD_PROJECT_CONFIG_DIGEST=$STUB_DIGEST" "$RHD_ENV_FILE"

# Concurrent activation cannot publish a dump paired with the wrong configuration.
if STUB_CHANGE_DURING_DUMP=1 bash "$repo/scripts/backup-db.sh" "$test_dir/changed" >"$test_dir/changed.log" 2>&1; then
  echo 'FAIL: backup accepted an active configuration change during pg_dump' >&2
  exit 1
fi
[[ -z "$(find "$test_dir/changed" -type f -print)" ]]
rm "$STUB_CONFIG_FILE.changed"

# Retention deletes a complete old archive, including its config sidecar.
BACKUP_RETENTION_COUNT=1 bash "$repo/scripts/backup-db.sh" "$test_dir/managed" >"$test_dir/retention.log"
[[ ! -e "$backup" && ! -e "$backup.meta" && ! -e "$backup.config.json" ]]
backups=("$test_dir/managed/"*.dump)
backup="${backups[0]}"

# A successful drill creates and removes only its own temporary database.
: >"$STUB_TOOL_LOG"
DRILL_DB_NAME=rhd_test_drill bash "$repo/scripts/drill-db-restore.sh" "$backup" >"$test_dir/drill.log"
grep -q 'createdb.*rhd_test_drill' "$STUB_TOOL_LOG"
grep -q 'pg_restore.*rhd_test_drill' "$STUB_TOOL_LOG"
if grep -q 'dropdb.*cs_ai_agent' "$STUB_TOOL_LOG"; then
  echo 'FAIL: drill tried to drop the source database' >&2
  exit 1
fi

# If the chosen temporary name already exists, cleanup must not remove it.
: >"$STUB_TOOL_LOG"
if SYNTHETIC_DRILL_COLLISION=1 DRILL_DB_NAME=rhd_test_drill bash "$repo/scripts/drill-db-restore.sh" "$backup" >"$test_dir/collision.log" 2>&1; then
  echo 'FAIL: colliding drill database was accepted' >&2
  exit 1
fi
if grep -Eq 'dropdb|createdb|pg_terminate_backend' "$STUB_TOOL_LOG"; then
  echo 'FAIL: colliding drill database was changed' >&2
  exit 1
fi

# Selecting production while holding a staging dump is rejected without a query.
(
  export RHD_INSTANCE_ID=onward-acme-production RHD_PROJECT_ENVIRONMENT=production
  expect_rejected "$backup" 'staging backup used for production'
)
(
  # FND-002 legacy deployments still need archived configuration even before
  # adopting a FND-003 instance ID. Only pure database installations are exempt.
  unset RHD_INSTANCE_ID RHD_DEPLOYMENT_PROJECT_ID
  bash "$repo/scripts/backup-db.sh" "$test_dir/config-only" >"$test_dir/config-only.log"
  archives=("$test_dir/config-only/"*.dump)
  archive="${archives[0]}"
  [[ -s "$archive.config.json" ]]
  ! grep -q '^instance_id=' "$archive.meta"
  grep -Fxq 'environment=staging' "$archive.meta"
  FORCE=1 bash "$repo/scripts/restore-db.sh" "$archive" >"$test_dir/config-only-restore.log"
  cmp "$archive.config.json" "$RHD_PROJECT_CONFIG_FILE"
)
(
  unset RHD_INSTANCE_ID RHD_DEPLOYMENT_PROJECT_ID RHD_PROJECT_ENVIRONMENT RHD_PROJECT_CONFIG_TENANT_ID
  unset RHD_PROJECT_CONFIG_FILE RHD_PROJECT_CONFIG_REQUIRED
  expect_rejected "$backup" 'managed backup used without instance selection'
  : >"$STUB_TOOL_LOG"
  FORCE=1 bash "$repo/scripts/restore-db.sh" "$test_dir/missing.dump" >"$test_dir/legacy.log"
  grep -q 'pg_restore.*cs_ai_agent' "$STUB_TOOL_LOG"
  bash "$repo/scripts/backup-db.sh" "$test_dir/legacy" >"$test_dir/legacy-backup.log"
  if grep -q '^instance_id=' "$test_dir/legacy/"*.meta; then
    echo 'FAIL: a legacy backup acquired a fabricated identity' >&2
    exit 1
  fi
)
echo 'PASS: backup/restore instance metadata, mismatch rejection and legacy compatibility'
