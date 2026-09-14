#!/usr/bin/env bash
# Isolated contract checks. No containers are started and no real .env is read.
set -Eeuo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
test_dir="$(mktemp -d "${TMPDIR:-/tmp}/rhd-instance-tests.XXXXXX")"
cleanup() {
  case "$test_dir" in
    /*/rhd-instance-tests.*) rm -rf -- "$test_dir" ;;
  esac
}
trap cleanup EXIT
unset RHD_PROJECT_NAME RHD_INSTANCE_ID RHD_DEPLOYMENT_PROJECT_ID RHD_NETWORK_NAME

fixture() {
  local target="$1" project="$2" environment="$3" instance="$4"
  cp "$repo/deploy/1panel/.env.example" "$target"
  {
    printf 'RHD_PROJECT_NAME=%s\nRHD_INSTANCE_ID=%s\n' "$instance" "$instance"
    printf 'RHD_DEPLOYMENT_PROJECT_ID=%s\nRHD_PROJECT_ENVIRONMENT=%s\n' "$project" "$environment"
    printf 'RHD_PROJECT_CONFIG_REQUIRED=1\nRHD_PROJECT_CONFIG_TENANT_ID=11\n'
    for pair in DATA:data BACKUP:backups STATE:state METRICS:metrics PROJECT_CONFIG:project-config PROJECT_SECRET:project-secrets; do
      printf 'RHD_%s_DIR=/srv/onward/%s/%s\n' "${pair%:*}" "$instance" "${pair#*:}"
    done
    printf 'RHD_PROJECT_CONFIG_FILE=/srv/onward/%s/project-config/current.json\n' "$instance"
    printf 'RHD_RESTORE_DATA_DIR=/srv/onward/%s/backups\n' "$instance"
  } >>"$target"
}

expect_rejected() {
  local env_file="$1" reason="$2"
  if RHD_ENV_FILE="$env_file" bash -c 'source "$1"; echo UNEXPECTED_COMMAND' _ "$repo/deploy/1panel/lib.sh" >"$test_dir/rejected.log" 2>&1; then
    printf 'FAIL: %s was accepted\n' "$reason" >&2
    exit 1
  fi
  if grep -q UNEXPECTED_COMMAND "$test_dir/rejected.log"; then
    printf 'FAIL: %s reached a deployment command\n' "$reason" >&2
    exit 1
  fi
}

fixture "$test_dir/staging.env" acme staging onward-acme-staging
fixture "$test_dir/production.env" acme production onward-acme-production
fixture "$test_dir/shared.env" daypop-shared integration onward-shared-integration
for item in staging:acme:staging production:acme:production shared:daypop-shared:integration; do
  entry="${item%%:*}"
  selected="$test_dir/${entry}.env"
  # Poison inherited variables to prove --env-file is authoritative for managed identity.
  RHD_ENV_FILE="$selected" RHD_PROJECT_NAME=unrelated RHD_DATA_DIR=/wrong/data \
    RHD_NETWORK_NAME=wrong RHD_PROJECT_ENVIRONMENT=production \
    bash -c '
      source "$1"
      docker() { printf "%s\n" "$@"; }
      rhd_compose ps -q api
      rhd_compose_with_override /tmp/rollback.yml config -q
      rhd_monitoring_compose config -q
      rhd_postgres_tool_environment
      [[ "$PG_DOCKER_COMPOSE_PROJECT_NAME" == "$RHD_INSTANCE_ID" ]]
      [[ "$PG_DOCKER_COMPOSE_ENV_FILE" == "$RHD_ENV_FILE" ]]
      [[ "$RHD_DATA_DIR" == "/srv/onward/$RHD_INSTANCE_ID/data" ]]
      [[ "$RHD_NETWORK_NAME" == "${RHD_INSTANCE_ID}_1panel" ]]
    ' _ "$repo/deploy/1panel/lib.sh" >"$test_dir/${entry}.args"
  grep -Fxq "$selected" "$test_dir/${entry}.args"
done
grep -Fxq onward-acme-staging "$test_dir/staging.args"
grep -Fxq onward-acme-production "$test_dir/production.args"
grep -Fxq onward-shared-integration "$test_dir/shared.args"

for key in RHD_PROJECT_NAME RHD_INSTANCE_ID RHD_DEPLOYMENT_PROJECT_ID RHD_DATA_DIR RHD_BACKUP_DIR RHD_STATE_DIR RHD_METRICS_DIR RHD_PROJECT_CONFIG_DIR RHD_PROJECT_SECRET_DIR RHD_PROJECT_CONFIG_FILE RHD_PROJECT_CONFIG_TENANT_ID RHD_PROJECT_CONFIG_REQUIRED; do
  sed "/^${key}=/d" "$test_dir/staging.env" >"$test_dir/missing.env"
  expect_rejected "$test_dir/missing.env" "missing ${key}"
done
for invalid in \
  'RHD_DATA_DIR=./volumes' \
  'RHD_DATA_DIR=/' \
  'RHD_DATA_DIR=/srv/../shared' \
  'RHD_DATA_DIR=/srv//shared' \
  'RHD_DATA_DIR=/srv/onward/onward-acme-staging/backups' \
  'RHD_DATA_DIR=/srv/onward/onward-acme-staging' \
  'RHD_NETWORK_NAME=remotehelpdesk_1panel' \
  'RHD_DEPLOYMENT_PROJECT_ID=shared' \
  'RHD_DEPLOYMENT_PROJECT_ID=123company' \
  'RHD_DEPLOYMENT_PROJECT_ID=acme--north' \
  'RHD_DEPLOYMENT_PROJECT_ID=acme-' \
  'RHD_PROJECT_ENVIRONMENT=integration' \
  'RHD_INSTANCE_ID=onward-acme-production' \
  'RHD_PROJECT_CONFIG_FILE=/srv/other.json' \
  'RHD_PROJECT_CONFIG_TENANT_ID=0' \
  'RHD_PROJECT_CONFIG_REQUIRED=0' \
  'RHD_RESTORE_DATA_DIR=./backups'; do
  cp "$test_dir/staging.env" "$test_dir/invalid.env"
  printf '%s\n' "$invalid" >>"$test_dir/invalid.env"
  expect_rejected "$test_dir/invalid.env" "$invalid"
done

printf 'RHD_DATA_DIR=./volumes\n' >"$test_dir/legacy.env"
RHD_ENV_FILE="$test_dir/legacy.env" bash -c '
  source "$1"
  [[ "$RHD_PROJECT_NAME" == remotehelpdesk ]]
  [[ "$(rhd_absolute_path ./volumes)" == "$RHD_DEPLOY_DIR/volumes" ]]
' _ "$repo/deploy/1panel/lib.sh"
printf 'RHD_PROJECT_NAME=legacy-custom\n' >>"$test_dir/legacy.env"
RHD_ENV_FILE="$test_dir/legacy.env" bash -c '
  source "$1"
  [[ "$RHD_PROJECT_NAME" == legacy-custom ]]
' _ "$repo/deploy/1panel/lib.sh"

# A foreign rollback state must fail before docker inspection or cutover.
sed "s|^RHD_STATE_DIR=.*|RHD_STATE_DIR=$test_dir/rollback-state|" "$test_dir/staging.env" >"$test_dir/rollback.env"
mkdir "$test_dir/rollback-state"
printf 'ROLLBACK_INSTANCE_ID=onward-acme-production\n' >"$test_dir/rollback-state/rollback.env"
if bash "$repo/deploy/1panel/rhdctl" --env-file "$test_dir/rollback.env" rollback >"$test_dir/rollback.log" 2>&1; then
  echo 'FAIL: foreign rollback state was accepted' >&2
  exit 1
fi
grep -q 'rollback state does not belong' "$test_dir/rollback.log"

if [[ "${1:-}" == "--compose" ]]; then
  mkdir "$test_dir/docker-config"
  export DOCKER_CONFIG="$test_dir/docker-config"
  docker compose version >/dev/null
  for entry in staging production shared; do
    docker compose --env-file "$test_dir/${entry}.env" \
      -f "$repo/deploy/1panel/docker-compose.yml" \
      -f "$repo/deploy/1panel/monitoring-compose.yml" \
      --profile monitoring config --format json >"$test_dir/${entry}.json"
  done
  node - "$test_dir" <<'NODE'
const fs = require('fs');
const path = require('path');
const assert = require('assert/strict');
const root = process.argv[2];
const entries = ['staging', 'production', 'shared'].map(name =>
  JSON.parse(fs.readFileSync(path.join(root, `${name}.json`), 'utf8')));
for (const config of entries) {
  assert.equal(config.services.api.environment.RHD_INSTANCE_ID, config.name);
  assert.equal(config.networks.default.name, `${config.name}_1panel`);
  assert.equal(config.services.api.environment.RHD_DEPLOYMENT_PROJECT_ID,
    config.name.includes('shared') ? 'daypop-shared' : 'acme');
  assert.ok(!config.services['node-exporter'].network_mode);
  for (const service of ['postgres', 'redis', 'qdrant', 'minio', 'api', 'prometheus', 'alertmanager']) {
    const mounts = config.services[service].volumes.filter(v => v.type === 'bind' && v.source.includes('/srv/onward/'));
    assert.ok(mounts.length > 0, `${service} lacks an instance-specific mount`);
    for (const mount of mounts) assert.ok(mount.source.includes(`/${config.name}/`));
  }
}
assert.equal(new Set(entries.map(c => c.networks.default.name)).size, 3);
assert.equal(new Set(entries.map(c => c.services.postgres.volumes[0].source)).size, 3);
NODE
fi
echo 'PASS: 1Panel instance isolation, legacy compatibility and rollback identity'
