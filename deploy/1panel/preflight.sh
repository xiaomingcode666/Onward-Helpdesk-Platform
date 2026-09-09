#!/usr/bin/env bash

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STRICT=0
SKIP_DOCKER=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env-file)
      [[ $# -ge 2 ]] || { echo "ERROR: --env-file requires a path" >&2; exit 2; }
      export RHD_ENV_FILE="$2"
      shift 2
      ;;
    --strict)
      STRICT=1
      shift
      ;;
    --skip-docker)
      SKIP_DOCKER=1
      shift
      ;;
    -h|--help)
      echo "Usage: ./preflight.sh [--env-file PATH] [--strict] [--skip-docker]"
      exit 0
      ;;
    *)
      echo "ERROR: unknown option: $1" >&2
      exit 2
      ;;
  esac
done

# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

errors=0
warnings=0

pass() {
  printf 'PASS  %s\n' "$*"
}

warn() {
  warnings=$((warnings + 1))
  printf 'WARN  %s\n' "$*" >&2
}

fail() {
  errors=$((errors + 1))
  printf 'FAIL  %s\n' "$*" >&2
}

check_command() {
  if command -v "$1" >/dev/null 2>&1; then
    pass "command available: $1"
  else
    fail "required command is missing: $1"
  fi
}

check_secret() {
  local key="$1"
  local min_length="$2"
  local value
  value="$(rhd_env_value "$key")"
  if [[ -z "$value" ]]; then
    fail "${key} is empty"
  elif [[ "$value" == *CHANGE_ME* || "$value" == *replace-me* ]]; then
    fail "${key} still contains a placeholder"
  elif (( ${#value} < min_length )); then
    fail "${key} must contain at least ${min_length} characters"
  else
    pass "${key} is configured"
  fi
}

check_url() {
  local key="$1"
  local require_https="$2"
  local value
  value="$(rhd_env_value "$key")"
  if [[ ! "$value" =~ ^https?://[^[:space:]]+$ ]]; then
    fail "${key} must be an absolute http(s) URL"
    return
  fi
  if [[ "$value" == *example.com* || "$value" == *.example || "$value" == *.example/* ]]; then
    fail "${key} still uses the example domain"
    return
  fi
  if [[ "$require_https" == "1" && "$value" != https://* && "$value" != http://127.0.0.1* && "$value" != http://localhost* ]]; then
    fail "${key} must use HTTPS outside localhost"
    return
  fi
  pass "${key} is configured"
}

check_command awk
check_command sed
check_command df

if [[ ! -f "$RHD_ENV_FILE" ]]; then
  fail "environment file is missing: ${RHD_ENV_FILE}; run ./rhdctl init first"
else
  pass "environment file exists: ${RHD_ENV_FILE}"
  env_mode="$(stat -f '%Lp' "$RHD_ENV_FILE" 2>/dev/null || stat -c '%a' "$RHD_ENV_FILE" 2>/dev/null || true)"
  case "$env_mode" in
    400|600) pass "environment file permissions are restricted (${env_mode})" ;;
    *) warn "environment file permissions should be 600 (current: ${env_mode:-unknown})" ;;
  esac
fi

if [[ ! -f "$RHD_COMPOSE_FILE" ]]; then
  fail "Compose file is missing: ${RHD_COMPOSE_FILE}"
fi

config_file="$(rhd_absolute_path "$(rhd_env_value RHD_CONFIG_FILE ./remotehelpdesk.yaml)")"
if [[ -f "$config_file" ]]; then
  pass "runtime config exists: ${config_file}"
else
  fail "runtime config is missing: ${config_file}"
fi

if [[ "$SKIP_DOCKER" == "0" ]]; then
  check_command docker
  if command -v docker >/dev/null 2>&1; then
    if docker info >/dev/null 2>&1; then
      pass "Docker daemon is reachable"
      docker_root="$(docker info --format '{{.DockerRootDir}}' 2>/dev/null || true)"
      docker_min_free_gb="$(rhd_env_value RHD_DOCKER_MIN_FREE_GB 8)"
      if [[ -n "$docker_root" && "$docker_min_free_gb" =~ ^[1-9][0-9]*$ ]]; then
        docker_free_kb="$(df -Pk "$docker_root" 2>/dev/null | awk 'NR == 2 {print $4}')"
        if [[ "$docker_free_kb" =~ ^[0-9]+$ && "$docker_free_kb" -ge $((docker_min_free_gb * 1024 * 1024)) ]]; then
          pass "Docker filesystem has at least ${docker_min_free_gb} GiB free"
        else
          fail "Docker filesystem needs at least ${docker_min_free_gb} GiB free (${docker_root})"
        fi
      else
        warn "could not validate Docker filesystem free space"
      fi
    else
      fail "Docker daemon is not reachable"
    fi
    if docker compose version >/dev/null 2>&1; then
      pass "Docker Compose v2 is available"
    else
      fail "Docker Compose v2 is unavailable"
    fi
  fi
fi

if [[ -f "$RHD_ENV_FILE" ]]; then
  check_secret POSTGRES_PASSWORD 24
  check_secret REDIS_PASSWORD 24
  check_secret RHD_BOOTSTRAP_ADMIN_PASSWORD 12
  check_secret MINIO_ROOT_PASSWORD 24
  check_secret CUSTOMER_SESSION_SECRET 32
  check_secret MCP_SERVER_TOKEN 32
  check_secret ENCRYPTION_KEY 32
  check_url RHD_PUBLIC_URL "$STRICT"
  check_url MINIO_PUBLIC_ENDPOINT "$STRICT"

  image_tag="$(rhd_env_value IMAGE_TAG)"
  if [[ -z "$image_tag" || "$image_tag" == *CHANGE_ME* ]]; then
    fail "IMAGE_TAG must identify this release"
  elif [[ "$image_tag" == "latest" || "$image_tag" == "1panel" || "$image_tag" == "local" ]]; then
    warn "IMAGE_TAG is mutable (${image_tag}); use a unique release tag for reliable rollback"
  else
    pass "release image tag is explicit: ${image_tag}"
  fi

  for key in \
    NEXT_PUBLIC_LOCAL_LOGIN_PLATFORM_PASSWORD \
    NEXT_PUBLIC_LOCAL_LOGIN_ENTERPRISE_PASSWORD \
    NEXT_PUBLIC_LOCAL_LOGIN_PARTNER_PASSWORD \
    NEXT_PUBLIC_LOCAL_LOGIN_CUSTOMER_PASSWORD; do
    if [[ -n "$(rhd_env_value "$key")" ]]; then
      fail "${key} must stay blank because it is embedded in the public frontend"
    fi
  done

  encryption_fallbacks="$(rhd_env_value ENCRYPTION_KEY_FALLBACKS)"
  if [[ -n "$encryption_fallbacks" ]]; then
    if [[ "$(rhd_env_value ALLOW_LEGACY_ENCRYPTION_FALLBACK 0)" == "1" ]]; then
      warn "legacy encryption fallbacks are enabled for a controlled migration"
    else
      fail "ENCRYPTION_KEY_FALLBACKS requires ALLOW_LEGACY_ENCRYPTION_FALLBACK=1"
    fi
  fi

  if [[ "$(rhd_env_value JITSI_REQUIRE_AUTH false)" == "true" ]]; then
    for key in JITSI_URL JITSI_DOMAIN JITSI_APP_ID JITSI_APP_SECRET; do
      if [[ -z "$(rhd_env_value "$key")" ]]; then
        fail "${key} is required when JITSI_REQUIRE_AUTH=true"
      fi
    done
  fi

  for key in POSTGRES_BIND_IP REDIS_BIND_IP QDRANT_BIND_IP MINIO_CONSOLE_BIND_IP; do
    bind_ip="$(rhd_env_value "$key" 127.0.0.1)"
    if [[ "$bind_ip" != "127.0.0.1" && "$bind_ip" != "::1" ]]; then
      warn "${key} exposes an internal service beyond loopback (${bind_ip})"
    fi
  done

  for key in QDRANT_IMAGE MINIO_IMAGE MINIO_CLIENT_IMAGE; do
    image_ref="$(rhd_env_value "$key")"
    if [[ "$image_ref" == *:latest || "$image_ref" != *@sha256:* && "$image_ref" != *:* ]]; then
      warn "${key} is not immutable (${image_ref:-unset}); pin a version or digest before release"
    fi
  done

  if [[ "$(rhd_env_value RHD_MONITORING_ENABLED 0)" == "1" ]]; then
    for key in POSTGRES_EXPORTER_IMAGE REDIS_EXPORTER_IMAGE NODE_EXPORTER_IMAGE PROMETHEUS_IMAGE ALERTMANAGER_IMAGE; do
      image_ref="$(rhd_env_value "$key")"
      if [[ -z "$image_ref" ]]; then
        fail "${key} is required when RHD_MONITORING_ENABLED=1"
      elif [[ "$image_ref" == *:latest ]]; then
        warn "${key} uses latest; pin a version or digest before release"
      fi
    done
    alertmanager_config="$(rhd_absolute_path "$(rhd_env_value RHD_ALERTMANAGER_CONFIG_FILE ./alertmanager.yml)")"
    if [[ ! -f "$alertmanager_config" ]]; then
      fail "Alertmanager config is missing: ${alertmanager_config}"
    elif grep -q 'local-log-only' "$alertmanager_config"; then
      warn "Alertmanager has no external notification receiver configured"
    fi
  fi

  min_free_gb="$(rhd_env_value RHD_MIN_FREE_GB 10)"
  if [[ "$min_free_gb" =~ ^[1-9][0-9]*$ ]]; then
    data_dir="$(rhd_absolute_path "$(rhd_env_value RHD_DATA_DIR ./volumes)")"
    disk_path="$data_dir"
    while [[ ! -e "$disk_path" && "$disk_path" != "/" ]]; do
      disk_path="$(dirname "$disk_path")"
    done
    free_kb="$(df -Pk "$disk_path" 2>/dev/null | awk 'NR == 2 {print $4}')"
    if [[ "$free_kb" =~ ^[0-9]+$ ]]; then
      required_kb=$((min_free_gb * 1024 * 1024))
      if (( free_kb < required_kb )); then
        fail "data filesystem has less than ${min_free_gb} GiB free (${disk_path})"
      else
        pass "data filesystem has at least ${min_free_gb} GiB free"
      fi
    else
      warn "could not determine free space for ${disk_path}"
    fi
  else
    fail "RHD_MIN_FREE_GB must be a positive integer"
  fi

  for key in RHD_DATA_DIR RHD_BACKUP_DIR RHD_STATE_DIR RHD_METRICS_DIR; do
    target_path="$(rhd_absolute_path "$(rhd_env_value "$key")")"
    writable_path="$target_path"
    while [[ ! -e "$writable_path" && "$writable_path" != "/" ]]; do
      writable_path="$(dirname "$writable_path")"
    done
    if [[ -d "$writable_path" && -w "$writable_path" ]]; then
      pass "${key} can be created or updated"
    else
      fail "${key} is not writable through ${writable_path}"
    fi
  done

  if [[ "$SKIP_DOCKER" == "0" && -f "$RHD_COMPOSE_FILE" ]] && command -v docker >/dev/null 2>&1; then
    if rhd_compose config -q >/dev/null 2>&1; then
      pass "Docker Compose configuration is valid"
    else
      fail "Docker Compose configuration is invalid; run docker compose config for details"
    fi
    if [[ "$(rhd_env_value RHD_MONITORING_ENABLED 0)" == "1" ]]; then
      if rhd_monitoring_compose config -q >/dev/null 2>&1; then
        pass "monitoring Compose configuration is valid"
      else
        fail "monitoring Compose configuration is invalid"
      fi
    fi
  fi
fi

printf '\nPreflight summary: %d error(s), %d warning(s).\n' "$errors" "$warnings"
if (( errors > 0 )); then
  exit 1
fi
