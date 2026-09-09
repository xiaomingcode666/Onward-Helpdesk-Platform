#!/usr/bin/env bash

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

DEPLOY_HOST="${RHD_DEPLOY_HOST:-118.195.255.60}"
DEPLOY_USER="${RHD_DEPLOY_USER:-ubuntu}"
DEPLOY_PORT="${RHD_DEPLOY_PORT:-22}"
PUBLIC_URL="${RHD_PUBLIC_URL:-https://remotehelpdesk.digintelspace.com:8443}"
REMOTE_ROOT="${RHD_REMOTE_ROOT:-/data/remotehelpdesk}"
IMAGE_NAME="${RHD_IMAGE_NAME:-remotehelpdesk}"
RELEASE_LABEL="manual"
RUN_TESTS=1
RUN_BACKUP=1
RUN_RESTORE_DRILL=1
PRUNE_BUILD_CACHE=1
DRY_RUN=0

SOURCE_LIST=""
CURRENT_SOURCE_LIST=""
ARCHIVE=""

usage() {
  cat <<'EOF'
Usage: ./scripts/deploy-production.sh [options]

Build and deploy the current tracked and unignored source snapshot to the
RemoteHelpDesk 1Panel production server.

Options:
  --label NAME          Add a short release label (default: manual)
  --skip-tests          Skip local frontend and Go verification
  --skip-backup         Skip the production database backup and restore drill
  --skip-restore-drill  Create a database backup without restoring it to a
                        temporary verification database
  --keep-build-cache    Keep remote Docker BuildKit cache after success
  --dry-run             Validate and package locally without uploading
  -h, --help            Show this help

Environment overrides:
  RHD_DEPLOY_HOST       SSH host (default: 118.195.255.60)
  RHD_DEPLOY_USER       SSH user (default: ubuntu)
  RHD_DEPLOY_PORT       SSH port (default: 22)
  RHD_DEPLOY_PASSWORD   SSH password; omit to receive a hidden prompt
  RHD_PUBLIC_URL        Public application URL
  RHD_REMOTE_ROOT       Remote release/data root
  RHD_IMAGE_NAME        Docker image prefix

SSH keys are preferred. When key authentication is unavailable, install
sshpass locally and the script will ask for the password without echoing it.
EOF
}

log() {
  printf '[%s] %s\n' "$(date '+%H:%M:%S')" "$*"
}

die() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

cleanup() {
  [[ -z "$SOURCE_LIST" ]] || rm -f "$SOURCE_LIST"
  [[ -z "$CURRENT_SOURCE_LIST" ]] || rm -f "$CURRENT_SOURCE_LIST"
  [[ -z "$ARCHIVE" ]] || rm -f "$ARCHIVE"
  unset SSHPASS || true
}
trap cleanup EXIT

while [[ $# -gt 0 ]]; do
  case "$1" in
    --label)
      [[ $# -ge 2 ]] || die "--label requires a value"
      RELEASE_LABEL="$2"
      shift 2
      ;;
    --skip-tests)
      RUN_TESTS=0
      shift
      ;;
    --skip-backup)
      RUN_BACKUP=0
      RUN_RESTORE_DRILL=0
      shift
      ;;
    --skip-restore-drill)
      RUN_RESTORE_DRILL=0
      shift
      ;;
    --keep-build-cache)
      PRUNE_BUILD_CACHE=0
      shift
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown option: $1"
      ;;
  esac
done

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

require_command() {
  command_exists "$1" || die "required command is missing: $1"
}

sha256_file() {
  local file="$1"
  if command_exists shasum; then
    shasum -a 256 "$file" | awk '{print $1}'
  else
    sha256sum "$file" | awk '{print $1}'
  fi
}

write_source_list() {
  local output="$1"
  : >"$output"
  while IFS= read -r -d '' path; do
    [[ "$path" != "tmp/build-errors.log" ]] || continue
    case "$path" in
      .claude|.claude/*)
        continue
        ;;
      web/artifacts/*|web/artifacts)
        continue
        ;;
    esac
    if [[ -e "$path" || -L "$path" ]]; then
      printf '%s\0' "$path" >>"$output"
    fi
  done < <(
    {
      git ls-files -z
      git ls-files --others --exclude-standard -z
    } | sort -zu
  )
}

source_fingerprint() {
  local list_file="$1"
  local digest_file
  digest_file="$(mktemp "${TMPDIR:-/tmp}/rhd-source-digests.XXXXXX")"
  while IFS= read -r -d '' path; do
    if [[ -f "$path" || -L "$path" ]]; then
      if command_exists shasum; then
        shasum -a 256 "$path" >>"$digest_file"
      else
        sha256sum "$path" >>"$digest_file"
      fi
    fi
  done <"$list_file"
  sha256_file "$digest_file"
  rm -f "$digest_file"
}

run_local_verification() {
  log "Running local frontend and backend verification"

  local unformatted
  unformatted="$(
    while IFS= read -r -d '' path; do
      gofmt -l "$path"
    done < <(
      {
        git diff --name-only --diff-filter=ACMR -z HEAD -- '*.go'
        git ls-files --others --exclude-standard -z -- '*.go'
      } | sort -zu
    )
  )"
  [[ -z "$unformatted" ]] || die "gofmt is required for:\n$unformatted"

  git diff --check
  pnpm -C web build:sdk
  pnpm -C web typecheck
  pnpm -C web build:static
  go test -count=1 ./...
}

prepare_archive() {
  local release_id="$1"

  SOURCE_LIST="$(mktemp "${TMPDIR:-/tmp}/rhd-source-list.XXXXXX")"
  write_source_list "$SOURCE_LIST"
  [[ -s "$SOURCE_LIST" ]] || die "source snapshot is empty"

  ARCHIVE="${TMPDIR:-/tmp}/${release_id}.tar.gz"
  rm -f "$ARCHIVE"

  local tar_options=()
  if [[ "$(uname -s)" == "Darwin" ]]; then
    tar_options+=(--no-xattrs --no-mac-metadata)
  fi
  COPYFILE_DISABLE=1 tar "${tar_options[@]}" -czf "$ARCHIVE" --null -T "$SOURCE_LIST"
  [[ -s "$ARCHIVE" ]] || die "release archive is empty"
}

configure_ssh() {
  local target="$1"
  local common_options=(
    -o ConnectTimeout=15
    -o ServerAliveInterval=30
    -o ServerAliveCountMax=6
    -o StrictHostKeyChecking=accept-new
  )

  SSH_COMMAND=(ssh "${common_options[@]}" -p "$DEPLOY_PORT")
  SCP_COMMAND=(scp "${common_options[@]}" -P "$DEPLOY_PORT")

  if ssh "${common_options[@]}" -p "$DEPLOY_PORT" -o BatchMode=yes "$target" true </dev/null >/dev/null 2>&1; then
    log "Using SSH key authentication"
    return
  fi

  require_command sshpass
  local deploy_password="${RHD_DEPLOY_PASSWORD:-}"
  if [[ -z "$deploy_password" ]]; then
    [[ -t 0 ]] || die "SSH password is required; set RHD_DEPLOY_PASSWORD for non-interactive use"
    read -r -s -p "SSH password for ${target}: " deploy_password
    printf '\n'
  fi
  [[ -n "$deploy_password" ]] || die "SSH password cannot be empty"
  export SSHPASS="$deploy_password"
  SSH_COMMAND=(sshpass -e ssh "${common_options[@]}" -p "$DEPLOY_PORT")
  SCP_COMMAND=(sshpass -e scp "${common_options[@]}" -P "$DEPLOY_PORT")
}

remote_preflight() {
  local target="$1"
  "${SSH_COMMAND[@]}" "$target" bash -s -- "$REMOTE_ROOT" "$PUBLIC_URL" <<'REMOTE'
set -Eeuo pipefail

remote_root="$1"
public_url="${2%/}"
current="${remote_root}/current"

sudo -n true
command -v docker >/dev/null
command -v curl >/dev/null
sudo docker compose version >/dev/null
[[ -f "${current}/deploy/1panel/.env" ]]
[[ -f "${current}/deploy/1panel/remotehelpdesk.yaml" || -f "${current}/deploy/1panel/agent-desk.yaml" ]]

root_free_kb="$(df -Pk / | awk 'NR == 2 {print $4}')"
data_free_kb="$(df -Pk "$remote_root" | awk 'NR == 2 {print $4}')"
if (( root_free_kb < 8 * 1024 * 1024 )); then
  echo "ERROR: root filesystem needs at least 8 GiB free for image builds" >&2
  exit 1
fi
if (( data_free_kb < 5 * 1024 * 1024 )); then
  echo "ERROR: release filesystem needs at least 5 GiB free" >&2
  exit 1
fi

container_health() {
  local name
  for name in "$@"; do
    if sudo docker inspect "$name" >/dev/null 2>&1; then
      sudo docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$name"
      return
    fi
  done
}

api_health="$(container_health remotehelpdesk-api-1 remotehelpdesk-agent-desk-api-1)"
web_health="$(container_health remotehelpdesk-web-1 remotehelpdesk-agent-desk-web-1)"
[[ "$api_health" == "healthy" ]]
[[ "$web_health" == "healthy" ]]
curl -kfsS "${public_url}/api/health/ready" >/dev/null

echo "Remote preflight passed: root_free_kb=${root_free_kb} data_free_kb=${data_free_kb}"
REMOTE
}

remote_prepare_build() {
  local target="$1"
  local release_id="$2"
  local image_tag="$3"
  local archive_sha="$4"
  local source_sha="$5"
  local git_commit="$6"
  local build_time="$7"
  local archive_name
  archive_name="$(basename "$ARCHIVE")"

  log "Uploading source snapshot"
  "${SCP_COMMAND[@]}" "$ARCHIVE" "${target}:/tmp/${archive_name}"

  log "Preparing release, building images, and backing up PostgreSQL"
  "${SSH_COMMAND[@]}" "$target" bash -s -- \
    "$REMOTE_ROOT" "$release_id" "$image_tag" "$archive_name" "$archive_sha" \
    "$source_sha" "$IMAGE_NAME" "$git_commit" "$build_time" "$RUN_BACKUP" "$RUN_RESTORE_DRILL" "$PUBLIC_URL" <<'REMOTE'
set -Eeuo pipefail

remote_root="$1"
release_id="$2"
image_tag="$3"
archive_name="$4"
expected_sha="$5"
source_sha="$6"
image_name="$7"
git_commit="$8"
build_time="$9"
run_backup="${10}"
run_restore_drill="${11}"
public_url="${12%/}"

archive="/tmp/${archive_name}"
release="${remote_root}/releases/${release_id}"
current="${remote_root}/current"
deploy_lock="${remote_root}/.deploy-lock"
prepared=0
lock_acquired=0

cleanup_prepare() {
  local status="$?"
  set +e
  rm -f "$archive"
  if [[ "$status" != "0" && "$release" == "${remote_root}/releases/"* ]]; then
    sudo rm -rf -- "$release"
  fi
  if [[ "$status" != "0" && "$lock_acquired" == "1" ]]; then
    sudo rm -rf -- "$deploy_lock"
  fi
  exit "$status"
}
trap cleanup_prepare EXIT

actual_sha="$(sha256sum "$archive" | awk '{print $1}')"
[[ "$actual_sha" == "$expected_sha" ]] || {
  echo "ERROR: uploaded archive checksum mismatch" >&2
  exit 1
}

sudo mkdir -p "${remote_root}/releases" "${remote_root}/backups" "${remote_root}/metrics"
if [[ -d "$deploy_lock" ]]; then
  lock_mtime="$(sudo stat -c %Y "$deploy_lock" 2>/dev/null || printf '0')"
  lock_age="$(( $(date +%s) - lock_mtime ))"
  if (( lock_age > 12 * 60 * 60 )); then
    echo "Removing stale deployment lock older than 12 hours" >&2
    sudo rm -rf -- "$deploy_lock"
  fi
fi
if ! sudo mkdir "$deploy_lock" 2>/dev/null; then
  lock_owner="$(sudo cat "${deploy_lock}/owner" 2>/dev/null || printf 'unknown')"
  echo "ERROR: another deployment is active: ${lock_owner}" >&2
  exit 1
fi
lock_acquired=1
sudo chown "$(id -u):$(id -g)" "$deploy_lock"
printf '%s|%s|%s\n' "$release_id" "$(id -un)" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >"${deploy_lock}/owner"

[[ ! -e "$release" ]] || {
  echo "ERROR: release already exists: ${release}" >&2
  exit 1
}
sudo mkdir -p "$release"
sudo chown "$(id -u):$(id -g)" "$release"
tar -xzf "$archive" -C "$release"

if find "$release" -name '._*' -print -quit | grep -q .; then
  echo "ERROR: AppleDouble metadata files were found in the extracted release" >&2
  exit 1
fi

for required in \
  Dockerfile.prod \
  go.mod \
  web/package.json \
  web/components/ui/input.tsx \
  deploy/1panel/docker-compose.yml \
  deploy/1panel/nginx.conf; do
  [[ -f "${release}/${required}" ]] || {
    echo "ERROR: release is missing ${required}" >&2
    exit 1
  }
done

current_config="${current}/deploy/1panel/remotehelpdesk.yaml"
if [[ ! -f "$current_config" ]]; then
  current_config="${current}/deploy/1panel/agent-desk.yaml"
fi
sudo install -m 600 "${current}/deploy/1panel/.env" "${release}/deploy/1panel/.env"
sudo install -m 644 "$current_config" "${release}/deploy/1panel/remotehelpdesk.yaml"
sudo chown "$(id -u):$(id -g)" "${release}/deploy/1panel/.env" "${release}/deploy/1panel/remotehelpdesk.yaml"
tmp_env="$(mktemp)"
awk -F= '$1 != "RHD_PUBLIC_URL" {print}' "${release}/deploy/1panel/.env" >"$tmp_env"
printf 'RHD_PUBLIC_URL=%s\n' "$public_url" >>"$tmp_env"
install -m 600 "$tmp_env" "${release}/deploy/1panel/.env"
rm -f "$tmp_env"

cd "${release}/deploy/1panel"
sudo env IMAGE_NAME="$image_name" IMAGE_TAG="$image_tag" docker compose config -q

sudo env \
  IMAGE_NAME="$image_name" \
  IMAGE_TAG="$image_tag" \
  GIT_COMMIT="$git_commit" \
  BUILD_TIME="$build_time" \
  docker compose build web

sudo env \
  IMAGE_NAME="$image_name" \
  IMAGE_TAG="$image_tag" \
  GIT_COMMIT="$git_commit" \
  BUILD_TIME="$build_time" \
  docker compose build api

sudo docker image inspect "${image_name}-web:${image_tag}" >/dev/null
sudo docker image inspect "${image_name}-api:${image_tag}" >/dev/null

if [[ "$run_backup" == "1" ]]; then
  postgres_container="$(sudo docker compose ps -q postgres)"
  [[ -n "$postgres_container" ]] || {
    echo "ERROR: postgres container is unavailable" >&2
    exit 1
  }
  db_user="$(sudo docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$postgres_container" | awk -F= '$1 == "POSTGRES_USER" {print $2}')"
  db_name="$(sudo docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$postgres_container" | awk -F= '$1 == "POSTGRES_DB" {print $2}')"
  [[ -n "$db_user" && -n "$db_name" ]]

  sudo env \
    PG_TOOL_MODE=docker \
    DB_USER="$db_user" \
    DB_NAME="$db_name" \
    BACKUP_RETENTION_COUNT=30 \
    BACKUP_METRICS_FILE="${remote_root}/metrics/remotehelpdesk_backup.prom" \
    "${release}/scripts/backup-db.sh" "${remote_root}/backups"

  backup_file="$(find "${remote_root}/backups" -maxdepth 1 -type f -name "${db_name}_*.dump" -print | sort | tail -n 1)"
  [[ -n "$backup_file" && -s "$backup_file" ]]

  if [[ "$run_restore_drill" == "1" ]]; then
    sudo env \
      PG_TOOL_MODE=docker \
      DB_USER="$db_user" \
      DB_NAME="$db_name" \
      RESTORE_DRILL_METRICS_FILE="${remote_root}/metrics/remotehelpdesk_restore_drill.prom" \
      "${release}/scripts/drill-db-restore.sh" "$backup_file"
  fi
fi

printf '%s\n' "$expected_sha" >"${release}/SOURCE_ARCHIVE.sha256"
printf '%s\n' "$source_sha" >"${release}/SOURCE_MANIFEST.sha256"
printf '%s\n' "$git_commit" >"${release}/SOURCE_COMMIT"
prepared=1
echo "Release prepared: ${release_id}"
REMOTE
}

remote_discard_prepared_release() {
  local target="$1"
  local release_id="$2"
  local image_tag="$3"
  "${SSH_COMMAND[@]}" "$target" bash -s -- "$REMOTE_ROOT" "$release_id" "$IMAGE_NAME" "$image_tag" <<'REMOTE'
set -Eeuo pipefail
remote_root="$1"
release_id="$2"
image_name="$3"
image_tag="$4"
release="${remote_root}/releases/${release_id}"
deploy_lock="${remote_root}/.deploy-lock"
if [[ "$release" == "${remote_root}/releases/"* ]]; then
  sudo rm -rf -- "$release"
fi
sudo docker image rm "${image_name}-api:${image_tag}" "${image_name}-web:${image_tag}" >/dev/null 2>&1 || true
if sudo grep -q "^${release_id}|" "${deploy_lock}/owner" 2>/dev/null; then
  sudo rm -rf -- "$deploy_lock"
fi
REMOTE
}

remote_cutover() {
  local target="$1"
  local release_id="$2"
  local image_tag="$3"

  log "Switching API and web containers"
  "${SSH_COMMAND[@]}" "$target" bash -s -- \
    "$REMOTE_ROOT" "$release_id" "$image_tag" "$IMAGE_NAME" "$PUBLIC_URL" "$PRUNE_BUILD_CACHE" <<'REMOTE'
set -Eeuo pipefail

remote_root="$1"
release_id="$2"
image_tag="$3"
image_name="$4"
public_url="${5%/}"
prune_build_cache="$6"
release="${remote_root}/releases/${release_id}"
compose_dir="${release}/deploy/1panel"
current="${remote_root}/current"
deploy_lock="${remote_root}/.deploy-lock"

[[ -d "$compose_dir" ]] || {
  echo "ERROR: prepared release compose directory is missing: ${compose_dir}" >&2
  exit 1
}
[[ -f "${compose_dir}/.env" ]] || {
  echo "ERROR: prepared release is missing .env: ${compose_dir}/.env" >&2
  exit 1
}
sudo grep -q "^${release_id}|" "${deploy_lock}/owner" || {
  lock_owner="$(sudo cat "${deploy_lock}/owner" 2>/dev/null || printf 'missing')"
  echo "ERROR: deployment lock owner mismatch for ${release_id}: ${lock_owner}" >&2
  exit 1
}
cd "$compose_dir"

resolve_existing_container() {
  local name
  for name in "$@"; do
    if sudo docker inspect "$name" >/dev/null 2>&1; then
      printf '%s\n' "$name"
      return
    fi
  done
  return 1
}

old_api_container="$(resolve_existing_container remotehelpdesk-api-1 remotehelpdesk-agent-desk-api-1)"
old_web_container="$(resolve_existing_container remotehelpdesk-web-1 remotehelpdesk-agent-desk-web-1)"
old_api_image="$(sudo docker inspect --format '{{.Config.Image}}' "$old_api_container")"
old_web_image="$(sudo docker inspect --format '{{.Config.Image}}' "$old_web_container")"
[[ -n "$old_api_image" && -n "$old_web_image" ]]
legacy_containers=()
[[ "$old_api_container" == "remotehelpdesk-api-1" ]] || legacy_containers+=("$old_api_container")
[[ "$old_web_container" == "remotehelpdesk-web-1" ]] || legacy_containers+=("$old_web_container")

rollback_override="${compose_dir}/.rollback-${release_id}.yml"
cat >"$rollback_override" <<EOF
services:
  api:
    image: ${old_api_image}
  web:
    image: ${old_web_image}
EOF

wait_healthy() {
  local service="$1"
  local container=""
  local state=""
  local attempt
  for attempt in $(seq 1 48); do
    container="$(sudo env IMAGE_NAME="$image_name" IMAGE_TAG="$image_tag" docker compose ps -q "$service")"
    if [[ -n "$container" ]]; then
      state="$(sudo docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container" 2>/dev/null || true)"
      if [[ "$state" == "healthy" ]]; then
        return 0
      fi
      if [[ "$state" == "exited" || "$state" == "dead" ]]; then
        return 1
      fi
    fi
    sleep 5
  done
  return 1
}

wait_public() {
  local path="$1"
  local attempt
  for attempt in $(seq 1 36); do
    if curl -kfsS "${public_url}${path}" >/dev/null; then
      return 0
    fi
    sleep 5
  done
  return 1
}

rollback() {
  local failure_code="$?"
  local rollback_failed=0
  trap - ERR
  set +e
  echo "Deployment failed; restoring ${old_api_image} and ${old_web_image}" >&2
  sudo docker compose -f docker-compose.yml -f "$rollback_override" up -d --no-deps api || rollback_failed=1
  sudo docker compose -f docker-compose.yml -f "$rollback_override" up -d --no-deps web || rollback_failed=1
  for service in api web; do
    container="$(sudo docker compose -f docker-compose.yml -f "$rollback_override" ps -q "$service")"
    state=""
    for attempt in $(seq 1 36); do
      state="$(sudo docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container" 2>/dev/null || true)"
      [[ "$state" == "healthy" ]] && break
      sleep 5
    done
    if [[ "$state" != "healthy" ]]; then
      echo "Rollback health check failed for ${service}: ${state:-unknown}" >&2
      rollback_failed=1
    fi
  done
  rm -f "$rollback_override"
  if [[ "$rollback_failed" == "0" ]]; then
    echo "Rollback completed successfully" >&2
  else
    echo "CRITICAL: rollback did not restore every service to healthy" >&2
  fi
  sudo rm -rf -- "$deploy_lock"
  exit "$failure_code"
}
trap rollback ERR

# The legacy web service owns the same host port as the renamed service. Remove
# legacy-named containers only after their rollback images have been captured.
if (( ${#legacy_containers[@]} > 0 )); then
  sudo docker rm -f "${legacy_containers[@]}"
fi

api_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
sudo env IMAGE_NAME="$image_name" IMAGE_TAG="$image_tag" docker compose up -d --no-deps api
wait_healthy api
wait_public /api/health/ready

sudo env IMAGE_NAME="$image_name" IMAGE_TAG="$image_tag" docker compose up -d --no-deps web
wait_healthy web
wait_public /health
wait_public /
wait_public /platform/models
wait_public /customer
wait_public /enterprise
wait_public /partner

api_container="$(sudo docker compose ps -q api)"
if sudo docker logs --since "$api_started_at" "$api_container" 2>&1 \
  | grep -Eqi 'panic|fatal|migration.*(fail|error)|error.*migration|failed to (migrate|start)|schema.*error'; then
  echo "ERROR: API startup log risk scan failed" >&2
  false
fi

sudo ln -sfn "$release" "$current"
rm -f "$rollback_override"
trap - ERR
sudo rm -rf -- "$deploy_lock"

if [[ "$prune_build_cache" == "1" ]]; then
  sudo docker builder prune -af >/dev/null || true
fi
sudo docker image prune -f >/dev/null || true

echo "current=$(readlink -f "$current")"
for service in api web; do
  container="$(sudo docker compose ps -q "$service")"
  sudo docker inspect --format '{{.Config.Image}}|{{.State.Health.Status}}|restarts={{.RestartCount}}' "$container"
done
curl -kfsS "${public_url}/api/health/ready"
echo
REMOTE
}

cd "$REPO_ROOT"

for command_name in git tar awk sort mktemp go gofmt pnpm ssh scp curl; do
  require_command "$command_name"
done
if ! command_exists shasum && ! command_exists sha256sum; then
  die "shasum or sha256sum is required"
fi
[[ -f deploy/1panel/docker-compose.yml ]] || die "run this script from the RemoteHelpDesk repository"
[[ -f Dockerfile.prod ]] || die "Dockerfile.prod is missing"

RELEASE_LABEL="$(printf '%s' "$RELEASE_LABEL" | tr -cs 'A-Za-z0-9._-' '-' | sed 's/^-*//; s/-*$//')"
[[ -n "$RELEASE_LABEL" ]] || RELEASE_LABEL="manual"
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
release_id="remotehelpdesk-1panel-${timestamp}-${RELEASE_LABEL}"
image_tag="1panel-${timestamp}-${RELEASE_LABEL}"
git_commit="$(git rev-parse --short=12 HEAD 2>/dev/null || printf 'unknown')"
build_time="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

if [[ "$RUN_TESTS" == "1" ]]; then
  run_local_verification
else
  log "Skipping local tests by request"
fi

log "Creating portable source archive"
prepare_archive "$release_id"
source_sha="$(source_fingerprint "$SOURCE_LIST")"
archive_sha="$(sha256_file "$ARCHIVE")"
archive_size="$(du -h "$ARCHIVE" | awk '{print $1}')"
log "Snapshot ${source_sha} (${archive_size})"

if [[ "$DRY_RUN" == "1" ]]; then
  log "Dry run passed; no files were uploaded and production was not changed"
  exit 0
fi

target="${DEPLOY_USER}@${DEPLOY_HOST}"
configure_ssh "$target"
remote_preflight "$target"
remote_prepare_build "$target" "$release_id" "$image_tag" "$archive_sha" "$source_sha" "$git_commit" "$build_time"

CURRENT_SOURCE_LIST="$(mktemp "${TMPDIR:-/tmp}/rhd-current-source-list.XXXXXX")"
write_source_list "$CURRENT_SOURCE_LIST"
current_source_sha="$(source_fingerprint "$CURRENT_SOURCE_LIST")"
if [[ "$current_source_sha" != "$source_sha" ]]; then
  remote_discard_prepared_release "$target" "$release_id" "$image_tag" || true
  die "local source changed while images were building; rerun to deploy the latest snapshot"
fi

remote_cutover "$target" "$release_id" "$image_tag"

log "Deployment completed"
log "Release: ${release_id}"
log "URL: ${PUBLIC_URL}"
