#!/usr/bin/env bash

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SHOW_LOGS=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env-file)
      [[ $# -ge 2 ]] || { echo "ERROR: --env-file requires a path" >&2; exit 2; }
      export RHD_ENV_FILE="$2"
      shift 2
      ;;
    --logs)
      SHOW_LOGS=1
      shift
      ;;
    -h|--help)
      echo "Usage: ./doctor.sh [--env-file PATH] [--logs]"
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
failed_services=""

check_service() {
  local service="$1"
  local container_id state
  container_id="$(rhd_service_container_id "$service")"
  if [[ -z "$container_id" ]]; then
    printf 'FAIL  %-20s missing\n' "$service" >&2
    errors=$((errors + 1))
    failed_services="${failed_services} ${service}"
    return
  fi
  state="$(rhd_service_state "$container_id")"
  case "$state" in
    running/healthy|running)
      printf 'PASS  %-20s %s\n' "$service" "$state"
      ;;
    *)
      printf 'FAIL  %-20s %s\n' "$service" "${state:-unknown}" >&2
      errors=$((errors + 1))
      failed_services="${failed_services} ${service}"
      ;;
  esac
}

if ! "$SCRIPT_DIR/preflight.sh" --env-file "$RHD_ENV_FILE"; then
  rhd_die "preflight failed; fix configuration before runtime diagnosis"
fi

printf '\nService health:\n'
for service in postgres redis qdrant minio api web; do
  check_service "$service"
done

minio_init_id="$(rhd_service_container_id minio-init)"
if [[ -z "$minio_init_id" ]]; then
  printf 'FAIL  %-20s missing\n' "minio-init" >&2
  errors=$((errors + 1))
  failed_services="${failed_services} minio-init"
else
  minio_init_state="$(docker inspect --format '{{.State.Status}}/{{.State.ExitCode}}' "$minio_init_id" 2>/dev/null || true)"
  if [[ "$minio_init_state" == "exited/0" ]]; then
    printf 'PASS  %-20s %s\n' "minio-init" "$minio_init_state"
  else
    printf 'FAIL  %-20s %s\n' "minio-init" "${minio_init_state:-unknown}" >&2
    errors=$((errors + 1))
    failed_services="${failed_services} minio-init"
  fi
fi

app_port="$(rhd_env_value APP_PORT 8083)"
local_base_url="http://127.0.0.1:${app_port}"
printf '\nHTTP health:\n'
for path in /health /api/health/ready; do
  if rhd_http_check "${local_base_url}${path}"; then
    printf 'PASS  %s%s\n' "$local_base_url" "$path"
  else
    printf 'FAIL  %s%s\n' "$local_base_url" "$path" >&2
    errors=$((errors + 1))
  fi
done

if [[ "$(rhd_env_value RHD_DOCTOR_CHECK_PUBLIC 0)" == "1" ]]; then
  public_url="$(rhd_env_value RHD_PUBLIC_URL)"
  if rhd_http_check "${public_url%/}/api/health/ready"; then
    printf 'PASS  %s/api/health/ready\n' "${public_url%/}"
  else
    printf 'FAIL  %s/api/health/ready\n' "${public_url%/}" >&2
    errors=$((errors + 1))
  fi
fi

if [[ "$(rhd_env_value RHD_MONITORING_ENABLED 0)" == "1" ]]; then
  printf '\nMonitoring health:\n'
  for service in postgres-exporter redis-exporter node-exporter prometheus alertmanager; do
    monitoring_id="$(rhd_monitoring_compose ps -aq "$service" 2>/dev/null | head -n 1)"
    monitoring_state=""
    if [[ -n "$monitoring_id" ]]; then
      monitoring_state="$(rhd_service_state "$monitoring_id")"
    fi
    case "$monitoring_state" in
      running/healthy|running) printf 'PASS  %-20s %s\n' "$service" "$monitoring_state" ;;
      *) printf 'FAIL  %-20s %s\n' "$service" "${monitoring_state:-missing}" >&2; errors=$((errors + 1)) ;;
    esac
  done
  if rhd_http_check "http://127.0.0.1:$(rhd_env_value PROMETHEUS_PORT 9090)/-/ready"; then
    printf 'PASS  Prometheus readiness\n'
  else
    printf 'FAIL  Prometheus readiness\n' >&2
    errors=$((errors + 1))
  fi
  if rhd_http_check "http://127.0.0.1:$(rhd_env_value ALERTMANAGER_PORT 9093)/-/ready"; then
    printf 'PASS  Alertmanager readiness\n'
  else
    printf 'FAIL  Alertmanager readiness\n' >&2
    errors=$((errors + 1))
  fi
fi

if (( SHOW_LOGS == 1 && errors > 0 )); then
  printf '\nRecent logs for failed services:\n' >&2
  for service in $failed_services; do
    rhd_compose logs --tail 80 "$service" >&2 || true
  done
fi

printf '\nDoctor summary: %d error(s).\n' "$errors"
if (( errors > 0 )); then
  exit 1
fi
