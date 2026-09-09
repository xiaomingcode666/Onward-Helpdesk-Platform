#!/usr/bin/env bash

# Shared helpers for the operator-facing private deployment commands.

RHD_DEPLOY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RHD_REPO_ROOT="$(cd "${RHD_DEPLOY_DIR}/../.." && pwd)"
RHD_ENV_FILE="${RHD_ENV_FILE:-${RHD_DEPLOY_DIR}/.env}"
RHD_COMPOSE_FILE="${RHD_COMPOSE_FILE:-${RHD_DEPLOY_DIR}/docker-compose.yml}"
RHD_MONITORING_COMPOSE_FILE="${RHD_MONITORING_COMPOSE_FILE:-${RHD_DEPLOY_DIR}/monitoring-compose.yml}"
RHD_PROJECT_NAME="${RHD_PROJECT_NAME:-remotehelpdesk}"

rhd_log() {
  printf '[RemoteHelpDesk] %s\n' "$*"
}

rhd_warn() {
  printf '[RemoteHelpDesk] WARN: %s\n' "$*" >&2
}

rhd_die() {
  printf '[RemoteHelpDesk] ERROR: %s\n' "$*" >&2
  exit 1
}

rhd_require_command() {
  command -v "$1" >/dev/null 2>&1 || rhd_die "required command is missing: $1"
}

rhd_env_value() {
  local key="$1"
  local default_value="${2:-}"
  local value

  if [[ ! -f "$RHD_ENV_FILE" ]]; then
    printf '%s' "$default_value"
    return
  fi

  value="$(awk -v key="$key" '
    $0 ~ "^[[:space:]]*" key "[[:space:]]*=" {
      line = $0
      sub("^[[:space:]]*" key "[[:space:]]*=[[:space:]]*", "", line)
      value = line
    }
    END { printf "%s", value }
  ' "$RHD_ENV_FILE")"
  value="${value%$'\r'}"
  case "$value" in
    \"*\") value="${value#\"}"; value="${value%\"}" ;;
    \'*\') value="${value#\'}"; value="${value%\'}" ;;
  esac
  if [[ -z "$value" ]]; then
    value="$default_value"
  fi
  printf '%s' "$value"
}

rhd_env_is_set() {
  [[ -f "$RHD_ENV_FILE" ]] && awk -v key="$1" '
    $0 ~ "^[[:space:]]*" key "[[:space:]]*=" { found = 1 }
    END { exit(found ? 0 : 1) }
  ' "$RHD_ENV_FILE"
}

rhd_compose() {
  docker compose \
    --env-file "$RHD_ENV_FILE" \
    -f "$RHD_COMPOSE_FILE" \
    -p "$RHD_PROJECT_NAME" \
    "$@"
}

rhd_compose_with_override() {
  local override_file="$1"
  shift
  docker compose \
    --env-file "$RHD_ENV_FILE" \
    -f "$RHD_COMPOSE_FILE" \
    -f "$override_file" \
    -p "$RHD_PROJECT_NAME" \
    "$@"
}

rhd_monitoring_compose() {
  docker compose \
    --env-file "$RHD_ENV_FILE" \
    -f "$RHD_COMPOSE_FILE" \
    -f "$RHD_MONITORING_COMPOSE_FILE" \
    -p "$RHD_PROJECT_NAME" \
    --profile monitoring \
    "$@"
}

rhd_absolute_path() {
  local value="$1"
  case "$value" in
    /*) printf '%s' "$value" ;;
    *) printf '%s/%s' "$RHD_DEPLOY_DIR" "${value#./}" ;;
  esac
}

rhd_service_container_id() {
  rhd_compose ps -aq "$1" 2>/dev/null | head -n 1
}

rhd_service_state() {
  local container_id="$1"
  docker inspect --format '{{.State.Status}}{{if .State.Health}}/{{.State.Health.Status}}{{end}}' "$container_id" 2>/dev/null || true
}

rhd_wait_for_service() {
  local service="$1"
  local timeout_seconds="${2:-180}"
  local started_at="$SECONDS"
  local container_id state

  while (( SECONDS - started_at < timeout_seconds )); do
    container_id="$(rhd_service_container_id "$service")"
    if [[ -n "$container_id" ]]; then
      state="$(rhd_service_state "$container_id")"
      case "$state" in
        running/healthy|running)
          rhd_log "service is ready: ${service} (${state})"
          return 0
          ;;
        exited*|dead*|removing*)
          rhd_warn "service stopped before becoming ready: ${service} (${state})"
          return 1
          ;;
      esac
    fi
    sleep 3
  done

  state="unknown"
  if [[ -n "${container_id:-}" ]]; then
    state="$(rhd_service_state "$container_id")"
  fi
  rhd_warn "service did not become ready within ${timeout_seconds}s: ${service} (${state})"
  return 1
}

rhd_wait_for_job() {
  local service="$1"
  local timeout_seconds="${2:-120}"
  local started_at="$SECONDS"
  local container_id status exit_code

  while (( SECONDS - started_at < timeout_seconds )); do
    container_id="$(rhd_service_container_id "$service")"
    if [[ -n "$container_id" ]]; then
      status="$(docker inspect --format '{{.State.Status}}' "$container_id" 2>/dev/null || true)"
      if [[ "$status" == "exited" ]]; then
        exit_code="$(docker inspect --format '{{.State.ExitCode}}' "$container_id" 2>/dev/null || true)"
        if [[ "$exit_code" == "0" ]]; then
          rhd_log "job completed: ${service}"
          return 0
        fi
        rhd_warn "job failed: ${service} (exit ${exit_code:-unknown})"
        return 1
      fi
    fi
    sleep 2
  done
  rhd_warn "job did not complete within ${timeout_seconds}s: ${service}"
  return 1
}

rhd_http_check() {
  local url="$1"
  if command -v curl >/dev/null 2>&1; then
    curl --connect-timeout 5 --max-time 15 --retry 2 --retry-delay 1 -fsS "$url" >/dev/null
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -q -T 15 -O /dev/null "$url"
    return
  fi
  rhd_warn "curl or wget is required for HTTP health checks"
  return 1
}

rhd_set_env_value() {
  local key="$1"
  local value="$2"
  local target_file="$3"
  local temporary_file

  temporary_file="$(mktemp "${target_file}.tmp.XXXXXX")"
  awk -v key="$key" -v value="$value" '
    BEGIN { replaced = 0 }
    $0 ~ "^[[:space:]]*" key "[[:space:]]*=" {
      if (!replaced) {
        print key "=" value
        replaced = 1
      }
      next
    }
    { print }
    END {
      if (!replaced) {
        print key "=" value
      }
    }
  ' "$target_file" >"$temporary_file"
  chmod --reference="$target_file" "$temporary_file" 2>/dev/null || chmod 600 "$temporary_file"
  mv "$temporary_file" "$target_file"
}

rhd_postgres_tool_environment() {
  export PG_TOOL_MODE=docker
  export PG_DOCKER_SERVICE=postgres
  export PG_DOCKER_COMPOSE_ENV_FILE="$RHD_ENV_FILE"
  export PG_DOCKER_COMPOSE_FILE="$RHD_COMPOSE_FILE"
  export PG_DOCKER_COMPOSE_PROJECT_NAME="$RHD_PROJECT_NAME"
  export DB_NAME="$(rhd_env_value POSTGRES_DB cs_ai_agent)"
  export DB_USER="$(rhd_env_value POSTGRES_USER cs_ai_agent)"
  export BACKUP_RETENTION_COUNT="$(rhd_env_value BACKUP_RETENTION_COUNT 30)"
}
