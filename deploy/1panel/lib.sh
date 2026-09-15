#!/usr/bin/env bash

# Shared helpers for the operator-facing private deployment commands.

RHD_DEPLOY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RHD_REPO_ROOT="$(cd "${RHD_DEPLOY_DIR}/../.." && pwd)"
RHD_ENV_FILE="${RHD_ENV_FILE:-${RHD_DEPLOY_DIR}/.env}"
RHD_COMPOSE_FILE="${RHD_COMPOSE_FILE:-${RHD_DEPLOY_DIR}/docker-compose.yml}"
RHD_MONITORING_COMPOSE_FILE="${RHD_MONITORING_COMPOSE_FILE:-${RHD_DEPLOY_DIR}/monitoring-compose.yml}"

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

# A managed instance must never inherit another instance's Compose project or
# storage defaults. Read the selected file without executing shell statements.
rhd_load_instance_settings() {
  local configured_project instance deployment_project environment expected key value other path_part path_cursor
  local -a directory_values=()
  configured_project="$(rhd_env_value RHD_PROJECT_NAME)"
  instance="$(rhd_env_value RHD_INSTANCE_ID)"
  deployment_project="$(rhd_env_value RHD_DEPLOYMENT_PROJECT_ID)"
  if [[ -z "$instance$deployment_project${RHD_INSTANCE_ID:-}${RHD_DEPLOYMENT_PROJECT_ID:-}" ]]; then
    RHD_PROJECT_NAME="${configured_project:-${RHD_PROJECT_NAME:-remotehelpdesk}}"
    export RHD_PROJECT_NAME
    return 0
  fi

  [[ -n "$instance" && -n "$deployment_project" && -n "$configured_project" ]] || {
    rhd_warn "managed instances require RHD_INSTANCE_ID, RHD_DEPLOYMENT_PROJECT_ID and RHD_PROJECT_NAME in the selected env file"
    return 1
  }
  [[ "$deployment_project" =~ ^[a-z][a-z0-9]*(-[a-z0-9]+)*$ && ${#deployment_project} -le 48 && "$deployment_project" != shared ]] || {
    rhd_warn "RHD_DEPLOYMENT_PROJECT_ID must start with a lowercase letter, use single hyphens between letters/digits, and be at most 48 characters; shared is reserved"
    return 1
  }
  environment="$(rhd_env_value RHD_PROJECT_ENVIRONMENT)"
  if [[ "$deployment_project" == "daypop-shared" && "$environment" == "integration" ]]; then
    expected="onward-shared-integration"
  elif [[ "$deployment_project" != "daypop-shared" && ( "$environment" == "staging" || "$environment" == "production" ) ]]; then
    expected="onward-${deployment_project}-${environment}"
  else
    rhd_warn "use daypop-shared/integration for the shared environment, and staging or production for each deployment project"
    return 1
  fi
  [[ "$instance" == "$expected" && "$configured_project" == "$instance" ]] || {
    rhd_warn "RHD_INSTANCE_ID and RHD_PROJECT_NAME must match the deployment project and environment (${expected})"
    return 1
  }
  value="$(rhd_env_value RHD_NETWORK_NAME "${instance}_1panel")"
  [[ "$value" == "${instance}_1panel" ]] || {
    rhd_warn "RHD_NETWORK_NAME must be ${instance}_1panel for this managed instance"
    return 1
  }
  export RHD_NETWORK_NAME="$value"

  for key in RHD_DATA_DIR RHD_BACKUP_DIR RHD_STATE_DIR RHD_METRICS_DIR RHD_PROJECT_CONFIG_DIR RHD_PROJECT_SECRET_DIR; do
    value="$(rhd_env_value "$key")"
    if [[ "$value" != /* || "$value" == "/" || "$value" == */ || "$value" == *//* || "/${value#/}/" == */../* || "/${value#/}/" == */./* || "$value" == *'$'* || "$value" == *'`'* || "$value" == *:* || "$value" == *'\'* || "$value" == *$'\n'* || "$value" == *$'\r'* ]]; then
      rhd_warn "${key} must be an explicit normalized absolute directory for this instance"
      return 1
    fi
    for other in "${directory_values[@]}"; do
      if [[ "$value" == "$other" || "$value" == "$other/"* || "$other" == "$value/"* ]]; then
        rhd_warn "managed data, backup, state, metrics, configuration and secret directories must not overlap"
        return 1
      fi
    done
    path_cursor=""
    local -a path_parts=()
    IFS=/ read -r -a path_parts <<<"${value#/}"
    for path_part in "${path_parts[@]}"; do
      path_cursor="${path_cursor}/${path_part}"
      if [[ -L "$path_cursor" ]]; then
        rhd_warn "${key} must not pass through a symlink to another storage location"
        return 1
      fi
    done
    directory_values+=("$value")
    # Compose gives shell variables precedence over --env-file. Explicitly pin
    # these paths to the selected file for both Compose and child commands.
    export "$key=$value"
  done
  value="$(rhd_env_value RHD_PROJECT_CONFIG_FILE)"
  [[ "$value" == "$RHD_PROJECT_CONFIG_DIR/current.json" ]] || {
    rhd_warn "RHD_PROJECT_CONFIG_FILE must be RHD_PROJECT_CONFIG_DIR/current.json, matching the container mount"
    return 1
  }
  [[ "$(rhd_env_value RHD_PROJECT_CONFIG_TENANT_ID)" =~ ^[1-9][0-9]*$ ]] || {
    rhd_warn "RHD_PROJECT_CONFIG_TENANT_ID must identify the company used by this environment"
    return 1
  }
  [[ "$(rhd_env_value RHD_PROJECT_CONFIG_REQUIRED)" == "1" ]] || {
    rhd_warn "managed instances require RHD_PROJECT_CONFIG_REQUIRED=1"
    return 1
  }
  export RHD_PROJECT_NAME="$configured_project"
  export RHD_INSTANCE_ID="$instance"
  export RHD_DEPLOYMENT_PROJECT_ID="$deployment_project"
  export RHD_PROJECT_ENVIRONMENT="$environment"
  export RHD_PROJECT_CONFIG_FILE="$value"
  export RHD_PROJECT_CONFIG_REQUIRED=1
  export RHD_PROJECT_CONFIG_TENANT_ID="$(rhd_env_value RHD_PROJECT_CONFIG_TENANT_ID)"
  export RHD_PROJECT_CONFIG_DIGEST="$(rhd_env_value RHD_PROJECT_CONFIG_DIGEST)"
  # Keep host-side preflight and container paths in agreement even when a shell
  # was previously used to operate another environment.
  export RHD_CONFIG_FILE="$(rhd_env_value RHD_CONFIG_FILE ./remotehelpdesk.yaml)"
  value="$(rhd_env_value RHD_RESTORE_DATA_DIR "$RHD_BACKUP_DIR")"
  [[ "$value" == "$RHD_BACKUP_DIR" ]] || {
    rhd_warn "RHD_RESTORE_DATA_DIR must use this instance's RHD_BACKUP_DIR"
    return 1
  }
  export RHD_RESTORE_DATA_DIR="$value"
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
  local value key
  export PG_TOOL_MODE=docker
  export PG_DOCKER_SERVICE=postgres
  export PG_DOCKER_COMPOSE_ENV_FILE="$RHD_ENV_FILE"
  export PG_DOCKER_COMPOSE_FILE="$RHD_COMPOSE_FILE"
  export PG_DOCKER_COMPOSE_PROJECT_NAME="$RHD_PROJECT_NAME"
  export DB_NAME="$(rhd_env_value POSTGRES_DB cs_ai_agent)"
  export DB_USER="$(rhd_env_value POSTGRES_USER cs_ai_agent)"
  export BACKUP_RETENTION_COUNT="$(rhd_env_value BACKUP_RETENTION_COUNT 30)"
  export RHD_ENV_FILE
  export RHD_PROJECT_CONFIG_CHECKER="$(rhd_absolute_path "$(rhd_env_value RHD_PROJECT_CONFIG_CHECKER ../../dist/remotehelpdesk)")"
  # FND-002 installations without FND-003 instance identity still bind their
  # active configuration and need it archived/restored with the database.
  for key in RHD_PROJECT_ENVIRONMENT RHD_PROJECT_CONFIG_TENANT_ID RHD_PROJECT_CONFIG_DIGEST RHD_PROJECT_CONFIG_REQUIRED; do
    export "$key=$(rhd_env_value "$key")"
  done
  for key in RHD_PROJECT_CONFIG_FILE RHD_PROJECT_CONFIG_DIR RHD_PROJECT_SECRET_DIR; do
    value="$(rhd_env_value "$key")"
    if [[ -n "$value" ]]; then value="$(rhd_absolute_path "$value")"; fi
    export "$key=$value"
  done
  if [[ -z "$RHD_PROJECT_CONFIG_DIR" && -n "$RHD_PROJECT_CONFIG_FILE" ]]; then
    export RHD_PROJECT_CONFIG_DIR="$(dirname "$RHD_PROJECT_CONFIG_FILE")"
  fi
}

rhd_load_instance_settings || rhd_die "invalid instance configuration; no deployment command was run"
