#!/usr/bin/env bash
# Full restore orchestration with command stubs; never contacts Docker or a DB.
set -Eeuo pipefail
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
test_dir="$(mktemp -d "${TMPDIR:-/tmp}/rhd-restore-tests.XXXXXX")"
cleanup() { case "$test_dir" in /*/rhd-restore-tests.*) rm -rf -- "$test_dir" ;; esac; }
trap cleanup EXIT
export STUB_ROOT="$test_dir" STUB_LOG="$test_dir/tools.log"
export STUB_DIGEST="$(printf 'a%.0s' {1..64})"
export STUB_DIGEST_V2="$(printf 'b%.0s' {1..64})"
export STUB_FAIL_RESTORE=0 STUB_FAIL_RUNTIME=0
export RHD_INSTANCE_ID=onward-acme-staging RHD_DEPLOYMENT_PROJECT_ID=acme
export RHD_PROJECT_ENVIRONMENT=staging RHD_PROJECT_CONFIG_TENANT_ID=11
export DB_NAME=cs_ai_agent DB_USER=cs_ai_agent PG_TOOL_MODE=docker
export RHD_PROJECT_CONFIG_FILE="$test_dir/instance/project-config/current.json"
export RHD_PROJECT_SECRET_DIR="$test_dir/instance/project-secrets"
export RHD_PROJECT_CONFIG_CHECKER="$test_dir/checker" RHD_ENV_FILE="$test_dir/instance/.env"
mkdir -p "$test_dir/instance/project-config" "$RHD_PROJECT_SECRET_DIR"
printf '#!/usr/bin/env bash\nexit 0\n' >"$RHD_PROJECT_CONFIG_CHECKER"
chmod 755 "$RHD_PROJECT_CONFIG_CHECKER"
for version in 1 2; do
  digest="$STUB_DIGEST"
  if [[ "$version" == 2 ]]; then digest="$STUB_DIGEST_V2"; fi
  printf '{"version_id":%s,"digest":"%s","document":{"schema_version":1,"tenant_id":11,"environment":"staging","projects":[],"intake":{"rules":[]},"secret_refs":[]}}\n' "$version" "$digest" >"$test_dir/v$version.json"
done
printf '1' >"$test_dir/active"
if ! command -v python3 >/dev/null 2>&1 && command -v cygpath >/dev/null 2>&1; then
  python3() {
    RHD_ENV_FILE="$(cygpath -m "$RHD_ENV_FILE")" RHD_PROJECT_CONFIG_FILE="$(cygpath -m "$RHD_PROJECT_CONFIG_FILE")" \
      RHD_PROJECT_SECRET_DIR="$(cygpath -m "$RHD_PROJECT_SECRET_DIR")" python "$@"
  }
  export -f python3
fi
docker() {
  printf '%s\n' "$*" >>"$STUB_LOG"
  case " $* " in
    *' info '*'--format '*) printf '/\n' ;;
    *' inspect '*minio-init*) printf 'exited/0\n' ;;
    *' inspect '*) printf 'running/healthy\n' ;;
    *' ps -aq '*) printf '%s\n' "${@: -1}" ;;
    *' ps '*) printf 'synthetic-postgres\n' ;;
    *' pg_dump '*) printf 'synthetic-dump\n' ;;
    *' pg_restore '*)
      [[ "$STUB_FAIL_RESTORE" == 0 ]] || return 1
      printf '1' >"$STUB_ROOT/active"
      ;;
    *' psql '*json_build_object*) cat "$STUB_ROOT/v$(cat "$STUB_ROOT/active").json" ;;
    *' psql '*"v.id::text"*)
      local version="$(cat "$STUB_ROOT/active")" digest="$STUB_DIGEST"
      if [[ "$version" == 2 ]]; then digest="$STUB_DIGEST_V2"; fi
      printf '%s|%s\n' "$version" "$digest"
      ;;
    *' psql '*) printf '1\n' ;;
    *' -check-project-config '*) [[ "$STUB_FAIL_RUNTIME" == 0 ]] || return 1 ;;
    *' up -d api web '*)
      [[ "$RHD_PROJECT_CONFIG_DIGEST" == "$STUB_DIGEST" ]] || {
        echo 'FAIL: API restarted with old inherited deployment digest' >&2
        return 1
      }
      cmp "$STUB_ROOT/v1.json" "$RHD_PROJECT_CONFIG_FILE"
      ;;
  esac
}
df() { printf 'Filesystem 1024-blocks Used Available Capacity Mounted\nsynthetic 999999999 1 999999998 1%% /\n'; }
curl() { return 0; }
export -f docker df curl

# Deliberately use mode 0644: the checked-out scripts do not have execute bits.
# Every operator subcommand must invoke its shell explicitly.
bash "$repo/scripts/backup-db.sh" "$test_dir/instance/backups" >"$test_dir/backup.log"
backups=("$test_dir/instance/backups/"*.dump)
backup="${backups[0]}"

prepare_current() {
  cp "$test_dir/v2.json" "$RHD_PROJECT_CONFIG_FILE"
  printf '2' >"$test_dir/active"
  cp "$repo/deploy/1panel/.env.example" "$test_dir/example.env"
  # Replace duplicate keys exactly as the renderer does.
  sed '/^RHD_/d; /^POSTGRES_PASSWORD=/d; /^REDIS_PASSWORD=/d; /^MINIO_ROOT_PASSWORD=/d; /^CUSTOMER_SESSION_SECRET=/d; /^MCP_SERVER_TOKEN=/d; /^ENCRYPTION_KEY=/d; /^IMAGE_TAG=/d; /^MINIO_PUBLIC_ENDPOINT=/d; /^BACKUP_RETENTION_COUNT=/d' "$test_dir/example.env" >"$RHD_ENV_FILE"
  {
    for key in RHD_INSTANCE_ID RHD_DEPLOYMENT_PROJECT_ID RHD_PROJECT_ENVIRONMENT RHD_PROJECT_CONFIG_TENANT_ID RHD_PROJECT_CONFIG_FILE RHD_PROJECT_SECRET_DIR RHD_PROJECT_CONFIG_CHECKER; do
      printf '%s=%s\n' "$key" "${!key}"
    done
    printf 'RHD_PROJECT_NAME=onward-acme-staging\nRHD_PROJECT_CONFIG_REQUIRED=1\n'
    printf 'RHD_PROJECT_CONFIG_DIGEST=%s\n' "$STUB_DIGEST_V2"
    printf 'RHD_PUBLIC_URL=https://support.test.invalid\nMINIO_PUBLIC_ENDPOINT=https://files.test.invalid\n'
    printf 'IMAGE_TAG=release-test\nRHD_MIN_FREE_GB=1\nRHD_DOCKER_MIN_FREE_GB=1\nBACKUP_RETENTION_COUNT=1\n'
    printf 'RHD_CONFIG_FILE=%s/deploy/1panel/remotehelpdesk.yaml\n' "$repo"
    for pair in DATA:data BACKUP:backups STATE:state METRICS:metrics PROJECT_CONFIG:project-config; do
      printf 'RHD_%s_DIR=%s/instance/%s\n' "${pair%:*}" "$test_dir" "${pair#*:}"
    done
    printf 'RHD_RESTORE_DATA_DIR=%s/instance/backups\n' "$test_dir"
    for key in POSTGRES_PASSWORD REDIS_PASSWORD RHD_BOOTSTRAP_ADMIN_PASSWORD MINIO_ROOT_PASSWORD CUSTOMER_SESSION_SECRET MCP_SERVER_TOKEN ENCRYPTION_KEY; do
      printf '%s=synthetic-test-value-abcdefghijklmnopqrstuvwxyz0123456789\n' "$key"
    done
  } >>"$RHD_ENV_FILE"
  chmod 600 "$RHD_ENV_FILE"
  : >"$STUB_LOG"
}

prepare_current
if STUB_FAIL_RUNTIME=1 bash "$repo/deploy/1panel/rhdctl" --env-file "$RHD_ENV_FILE" restore "$backup" --confirm cs_ai_agent >"$test_dir/runtime.log" 2>&1; then
  echo 'FAIL: inaccessible historical secrets were restored' >&2; exit 1
fi
! grep -Eq 'stop web api|dropdb|pg_restore' "$STUB_LOG"

prepare_current
if STUB_FAIL_RESTORE=1 bash "$repo/deploy/1panel/rhdctl" --env-file "$RHD_ENV_FILE" restore "$backup" --confirm cs_ai_agent >"$test_dir/failed.log" 2>&1; then
  echo 'FAIL: failed database restore reported success' >&2; exit 1
fi
grep -q 'stop web api' "$STUB_LOG"
! grep -q 'up -d api web' "$STUB_LOG"
grep -q 'application remains stopped' "$test_dir/failed.log"
[[ -s "$backup" && -s "$backup.config.json" ]]
[[ "$(find "$test_dir/instance/backups" -name '*.dump' | wc -l)" -ge 2 ]]

prepare_current
bash "$repo/deploy/1panel/rhdctl" --env-file "$RHD_ENV_FILE" restore "$backup" --confirm cs_ai_agent >"$test_dir/restored.log" 2>&1
cmp "$backup.config.json" "$RHD_PROJECT_CONFIG_FILE"
grep -q 'up -d api web' "$STUB_LOG"
[[ -s "$backup" ]]
echo 'PASS: ordinary API secret precheck, preserved recovery archives, stopped-on-failure and matching-config restart'
