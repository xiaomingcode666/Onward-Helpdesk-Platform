#!/usr/bin/env bash
# Exercise provisioning order with synthetic files and stubbed Docker/checker.
set -Eeuo pipefail
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
test_dir="$(mktemp -d "${TMPDIR:-/tmp}/rhd-initialize-tests.XXXXXX")"
cleanup() {
  case "$test_dir" in /*/rhd-initialize-tests.*) rm -rf -- "$test_dir" ;; esac
}
trap cleanup EXIT
unset RHD_PROJECT_NAME RHD_INSTANCE_ID RHD_DEPLOYMENT_PROJECT_ID RHD_NETWORK_NAME
export STUB_TOOL_LOG="$test_dir/tools.log"
export STUB_DIGEST=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
export STUB_FAIL_RAW=0 STUB_FAIL_RUNTIME=0
if ! command -v python3 >/dev/null 2>&1 && command -v cygpath >/dev/null 2>&1; then
  python3() { python "$@"; }
  export -f python3
fi

docker() {
  printf 'docker %s\n' "$*" >>"$STUB_TOOL_LOG"
  case " $* " in
    *' info '*'--format '*) printf '/\n' ;;
    *' inspect '*) printf 'running/healthy\n' ;;
    *' ps -aq postgres '*) printf 'synthetic-postgres\n' ;;
    *' -initialize-project-config '*)
      if [[ ! -e "$RHD_PROJECT_CONFIG_FILE" ]]; then
        printf '{"version_id":1,"digest":"%s","document":{"schema_version":1,"tenant_id":27,"environment":"staging","projects":[],"intake":{"rules":[]},"secret_refs":[]}}\n' "$STUB_DIGEST" >"$RHD_PROJECT_CONFIG_FILE"
      fi
      printf 'Project configuration digest: %s\n' "$STUB_DIGEST"
      ;;
    *' -check-project-config '*)
      [[ "$STUB_FAIL_RUNTIME" == 0 ]] || return 1
      ;;
  esac
}
df() {
  printf 'Filesystem 1024-blocks Used Available Capacity Mounted\n'
  printf 'synthetic 999999999 1 999999998 1%% /\n'
}
export -f docker df

fixture() {
  local package="$1" pair
  mkdir -p "$package/project-config" "$package/project-secrets"
  printf '{}\n' >"$package/project-config/initial.json"
  cp "$repo/deploy/1panel/.env.example" "$package/env"
  {
    printf 'RHD_PROJECT_NAME=onward-acme-staging\nRHD_INSTANCE_ID=onward-acme-staging\n'
    printf 'RHD_DEPLOYMENT_PROJECT_ID=acme\nRHD_PROJECT_ENVIRONMENT=staging\n'
    printf 'RHD_PROJECT_CONFIG_TENANT_ID=27\nRHD_PROJECT_CONFIG_REQUIRED=1\n'
    printf 'RHD_PUBLIC_URL=https://support.test.invalid\nMINIO_PUBLIC_ENDPOINT=https://files.test.invalid\n'
    printf 'IMAGE_TAG=release-test\nRHD_MIN_FREE_GB=1\nRHD_DOCKER_MIN_FREE_GB=1\n'
    printf 'RHD_CONFIG_FILE=%s/deploy/1panel/remotehelpdesk.yaml\n' "$repo"
    for pair in DATA:data BACKUP:backups STATE:state METRICS:metrics PROJECT_CONFIG:project-config PROJECT_SECRET:project-secrets; do
      printf 'RHD_%s_DIR=%s/%s\n' "${pair%:*}" "$package" "${pair#*:}"
    done
    printf 'RHD_PROJECT_CONFIG_FILE=%s/project-config/current.json\n' "$package"
    printf 'RHD_RESTORE_DATA_DIR=%s/backups\nRHD_PROJECT_CONFIG_CHECKER=%s/checker\n' "$package" "$package"
    for key in POSTGRES_PASSWORD REDIS_PASSWORD RHD_BOOTSTRAP_ADMIN_PASSWORD MINIO_ROOT_PASSWORD CUSTOMER_SESSION_SECRET MCP_SERVER_TOKEN ENCRYPTION_KEY; do
      printf '%s=synthetic-test-value-abcdefghijklmnopqrstuvwxyz0123456789\n' "$key"
    done
  } >>"$package/env"
  printf '#!/usr/bin/env bash\nprintf "checker %%s\\n" "$*" >>"$STUB_TOOL_LOG"\n[[ "$STUB_FAIL_RAW" == 0 ]]\n' >"$package/checker"
  chmod 600 "$package/env"
  chmod 755 "$package/checker"
}

fixture "$test_dir/first"
: >"$STUB_TOOL_LOG"
bash "$repo/deploy/1panel/rhdctl" --env-file "$test_dir/first/env" initialize-config \
  --document "$test_dir/first/project-config/initial.json" --tenant-name 'Synthetic Company' >"$test_dir/success.log" 2>&1
[[ -s "$test_dir/first/project-config/current.json" ]]
grep -Fxq "RHD_PROJECT_CONFIG_DIGEST=$STUB_DIGEST" "$test_dir/first/env"
grep -q -- '-check-project-config-document' "$STUB_TOOL_LOG"
grep -q -- '--user .*--volume .*project-config:/app/project-config:rw' "$STUB_TOOL_LOG"
grep -q -- '-project-tenant 27.*-project-environment staging.*-project-tenant-name Synthetic Company' "$STUB_TOOL_LOG"
grep -q -- '-check-project-config /app/project-config/current.json' "$STUB_TOOL_LOG"
if grep -Eq 'up -d (api|web)' "$STUB_TOOL_LOG"; then
  echo 'FAIL: initialize-config started application services before install' >&2
  exit 1
fi
# Runtime verification must not acquire the temporary writable mount or host UID.
if grep -- '-check-project-config /app/project-config/current.json' "$STUB_TOOL_LOG" | grep -Eq -- '--volume|--user'; then
  echo 'FAIL: normal API verification used initialization privileges' >&2
  exit 1
fi
: >"$STUB_TOOL_LOG"
cp "$test_dir/first/project-config/current.json" "$test_dir/original.json"
bash "$repo/deploy/1panel/rhdctl" --env-file "$test_dir/first/env" initialize-config \
    --document "$test_dir/first/project-config/initial.json" --tenant-name 'Synthetic Company' \
    --admin-user-id 42 >"$test_dir/existing.log" 2>&1
cmp "$test_dir/original.json" "$test_dir/first/project-config/current.json"
grep -q -- '-project-admin-user-id 42' "$STUB_TOOL_LOG"

# Host-side schema/secret failure must stop before build, pull or database start.
fixture "$test_dir/invalid"
: >"$STUB_TOOL_LOG"
if STUB_FAIL_RAW=1 bash "$repo/deploy/1panel/rhdctl" --env-file "$test_dir/invalid/env" initialize-config \
    --document "$test_dir/invalid/project-config/initial.json" --tenant-name 'Synthetic Company' >"$test_dir/invalid.log" 2>&1; then
  echo 'FAIL: invalid raw configuration was provisioned' >&2
  exit 1
fi
if grep -Eq ' pull | build | up -d | -initialize-project-config ' "$STUB_TOOL_LOG"; then
  echo 'FAIL: deployment mutations ran despite failed preflight' >&2
  exit 1
fi
[[ ! -e "$test_dir/invalid/project-config/current.json" ]]

# Runtime file-permission failure preserves the bundle and permits explicit repair.
fixture "$test_dir/recover"
: >"$STUB_TOOL_LOG"
if STUB_FAIL_RUNTIME=1 bash "$repo/deploy/1panel/rhdctl" --env-file "$test_dir/recover/env" initialize-config \
    --document "$test_dir/recover/project-config/initial.json" --tenant-name 'Synthetic Company' >"$test_dir/recover.log" 2>&1; then
  echo 'FAIL: inaccessible runtime configuration was marked ready' >&2
  exit 1
fi
[[ -s "$test_dir/recover/project-config/current.json" ]]
if grep -Fxq "RHD_PROJECT_CONFIG_DIGEST=$STUB_DIGEST" "$test_dir/recover/env"; then
  echo 'FAIL: failed runtime check still updated the pinned digest' >&2
  exit 1
fi
bash "$repo/deploy/1panel/rhdctl" --env-file "$test_dir/recover/env" sync-config-digest >"$test_dir/sync.log" 2>&1
grep -Fxq "RHD_PROJECT_CONFIG_DIGEST=$STUB_DIGEST" "$test_dir/recover/env"

echo 'PASS: explicit initialization, raw preflight, read-only runtime validation and recovery'
