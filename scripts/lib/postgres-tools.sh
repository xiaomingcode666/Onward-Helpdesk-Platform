#!/usr/bin/env bash

# Shared PostgreSQL command runner. PG_TOOL_MODE=auto uses local client tools
# when available and otherwise executes them in the Docker Compose postgres service.

PG_TOOL_MODE="${PG_TOOL_MODE:-auto}"
PG_DOCKER_SERVICE="${PG_DOCKER_SERVICE:-postgres}"

pg_docker_compose() {
  local args=(docker compose)
  if [[ -n "${PG_DOCKER_COMPOSE_ENV_FILE:-}" ]]; then
    args+=(--env-file "$PG_DOCKER_COMPOSE_ENV_FILE")
  fi
  if [[ -n "${PG_DOCKER_COMPOSE_FILE:-}" ]]; then
    args+=(-f "$PG_DOCKER_COMPOSE_FILE")
  fi
  if [[ -n "${PG_DOCKER_COMPOSE_PROJECT_NAME:-}" ]]; then
    args+=(-p "$PG_DOCKER_COMPOSE_PROJECT_NAME")
  fi
  "${args[@]}" "$@"
}

validate_pg_identifier() {
  local value="$1"
  local label="$2"
  if [[ ! "$value" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
    echo "ERROR: ${label} must be a PostgreSQL identifier: ${value}" >&2
    return 1
  fi
}

init_pg_tools() {
  validate_pg_identifier "$DB_NAME" "DB_NAME"
  validate_pg_identifier "$DB_USER" "DB_USER"
  validate_pg_identifier "$PG_DOCKER_SERVICE" "PG_DOCKER_SERVICE"

  case "$PG_TOOL_MODE" in
    auto)
      if command -v pg_dump >/dev/null 2>&1 && command -v pg_restore >/dev/null 2>&1 && command -v psql >/dev/null 2>&1; then
        PG_TOOL_MODE="host"
      elif command -v docker >/dev/null 2>&1 && pg_docker_compose ps "$PG_DOCKER_SERVICE" --status running --quiet 2>/dev/null | grep -q .; then
        PG_TOOL_MODE="docker"
      else
        echo "ERROR: PostgreSQL client tools are unavailable and Docker Compose service '${PG_DOCKER_SERVICE}' is not running." >&2
        return 1
      fi
      ;;
    host)
      for tool in pg_dump pg_restore psql createdb dropdb; do
        if ! command -v "$tool" >/dev/null 2>&1; then
          echo "ERROR: Required PostgreSQL client tool is missing: ${tool}" >&2
          return 1
        fi
      done
      ;;
    docker)
      if ! command -v docker >/dev/null 2>&1 || ! pg_docker_compose ps "$PG_DOCKER_SERVICE" --status running --quiet 2>/dev/null | grep -q .; then
        echo "ERROR: Docker Compose service '${PG_DOCKER_SERVICE}' is not running." >&2
        return 1
      fi
      ;;
    *)
      echo "ERROR: PG_TOOL_MODE must be auto, host, or docker." >&2
      return 1
      ;;
  esac
}

pg_tool() {
  local tool="$1"
  shift
  if [[ "$PG_TOOL_MODE" == "host" ]]; then
    PGPASSWORD="$DB_PASSWORD" "$tool" -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" "$@"
    return
  fi
  pg_docker_compose exec -T "$PG_DOCKER_SERVICE" "$tool" -U "$DB_USER" "$@"
}

pg_query() {
  local database="$1"
  local sql="$2"
  validate_pg_identifier "$database" "database"
  pg_tool psql -d "$database" -v ON_ERROR_STOP=1 -Atqc "$sql"
}

pg_database_exists() {
  local database="$1"
  validate_pg_identifier "$database" "database"
  [[ "$(pg_query postgres "SELECT count(*) FROM pg_database WHERE datname = '${database}'")" == "1" ]]
}

pg_create_database() {
  local database="$1"
  validate_pg_identifier "$database" "database"
  pg_tool createdb "$database"
}

pg_drop_database() {
  local database="$1"
  validate_pg_identifier "$database" "database"
  pg_tool dropdb --if-exists "$database"
}

pg_terminate_database_connections() {
  local database="$1"
  validate_pg_identifier "$database" "database"
  pg_query postgres "SELECT count(pg_terminate_backend(pid)) FROM pg_stat_activity WHERE datname = '${database}' AND pid <> pg_backend_pid()" >/dev/null
}

pg_dump_database() {
  local database="$1"
  local output_file="$2"
  validate_pg_identifier "$database" "database"
  pg_tool pg_dump -d "$database" -Fc --no-owner --no-acl >"$output_file"
}

pg_restore_database() {
  local database="$1"
  local backup_file="$2"
  validate_pg_identifier "$database" "database"
  pg_tool pg_restore -d "$database" -Fc --no-owner --no-acl --exit-on-error <"$backup_file"
}

file_sha256() {
  local file="$1"
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
  elif command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  else
    echo "ERROR: shasum or sha256sum is required." >&2
    return 1
  fi
}
