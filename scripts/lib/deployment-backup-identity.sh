#!/usr/bin/env bash
# Backup identity is plain data, never shell code. This guard runs before any
# database connection is terminated or any restore/drill database is created.
RHD_BACKUP_HELPER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

rhd_backup_managed() {
  [[ -n "${RHD_INSTANCE_ID:-}${RHD_DEPLOYMENT_PROJECT_ID:-}" ]]
}

rhd_backup_config_managed() {
  rhd_backup_managed || [[ -n "${RHD_PROJECT_CONFIG_FILE:-}" || "${RHD_PROJECT_CONFIG_REQUIRED:-0}" == 1 ]]
}

rhd_backup_check_target_identity() {
  if ! rhd_backup_managed; then
    if rhd_backup_config_managed; then
      [[ "${RHD_PROJECT_CONFIG_TENANT_ID:-}" =~ ^[1-9][0-9]*$ &&
         "${RHD_PROJECT_ENVIRONMENT:-}" =~ ^(development|integration|staging|production)$ ]] || {
        echo 'ERROR: configuration-managed backup requires company and environment scope.' >&2
        return 1
      }
    fi
    return 0
  fi
  local expected
  local project="${RHD_DEPLOYMENT_PROJECT_ID:-}"
  if [[ ! "$project" =~ ^[a-z][a-z0-9]*(-[a-z0-9]+)*$ || ${#project} -gt 48 || "$project" == shared || ! "${RHD_PROJECT_CONFIG_TENANT_ID:-}" =~ ^[1-9][0-9]*$ ]]; then
    echo 'ERROR: managed backup operations require complete project, environment, instance and company identity.' >&2
    return 1
  fi
  if [[ "$RHD_DEPLOYMENT_PROJECT_ID" == daypop-shared && "${RHD_PROJECT_ENVIRONMENT:-}" == integration ]]; then
    expected=onward-shared-integration
  elif [[ "$RHD_DEPLOYMENT_PROJECT_ID" != daypop-shared && ( "${RHD_PROJECT_ENVIRONMENT:-}" == staging || "${RHD_PROJECT_ENVIRONMENT:-}" == production ) ]]; then
    expected="onward-${RHD_DEPLOYMENT_PROJECT_ID}-${RHD_PROJECT_ENVIRONMENT}"
  else
    echo 'ERROR: invalid managed backup project/environment identity.' >&2
    return 1
  fi
  if [[ "${RHD_INSTANCE_ID:-}" != "$expected" ]]; then
    echo 'ERROR: managed backup instance does not match the project and environment.' >&2
    return 1
  fi
}

rhd_backup_write_identity() {
  if rhd_backup_managed; then
    printf 'instance_id=%s\n' "$RHD_INSTANCE_ID"
    printf 'deployment_project_id=%s\n' "$RHD_DEPLOYMENT_PROJECT_ID"
  fi
  if rhd_backup_config_managed; then
    printf 'environment=%s\n' "$RHD_PROJECT_ENVIRONMENT"
    printf 'tenant_id=%s\n' "$RHD_PROJECT_CONFIG_TENANT_ID"
  fi
}

rhd_backup_config_summary() {
  python3 "$RHD_BACKUP_HELPER_DIR/deployment-backup-config.py" summary "$1"
}

rhd_backup_check_config() {
  local bundle="$1" digest="$2" checker="${RHD_PROJECT_CONFIG_CHECKER:-}"
  [[ -n "$checker" && -x "$checker" ]] || {
    echo 'ERROR: managed backup/restore requires the release configuration checker in RHD_PROJECT_CONFIG_CHECKER.' >&2
    return 1
  }
  "$checker" -check-project-config "$bundle" \
    -project-tenant "$RHD_PROJECT_CONFIG_TENANT_ID" \
    -project-environment "$RHD_PROJECT_ENVIRONMENT" \
    -project-digest "$digest" -project-secret-dir "${RHD_PROJECT_SECRET_DIR:-}" || return 1
}

rhd_backup_database_config() {
  pg_query "$1" "SELECT json_build_object('version_id', v.id, 'digest', v.digest, 'document', v.document_json::json)::text FROM t_project_configuration_state s JOIN t_project_configuration_version v ON v.id = s.active_version_id AND v.tenant_id = s.tenant_id AND v.environment = s.environment WHERE s.tenant_id = ${RHD_PROJECT_CONFIG_TENANT_ID} AND s.environment = '${RHD_PROJECT_ENVIRONMENT}'"
}

rhd_backup_verify_database_config() {
  local database="$1" summary="$2" actual
  actual="$(pg_query "$database" "SELECT v.id::text || '|' || v.digest FROM t_project_configuration_state s JOIN t_project_configuration_version v ON v.id = s.active_version_id AND v.tenant_id = s.tenant_id AND v.environment = s.environment WHERE s.tenant_id = ${RHD_PROJECT_CONFIG_TENANT_ID} AND s.environment = '${RHD_PROJECT_ENVIRONMENT}'")" || return 1
  [[ "$actual" == "$summary" ]] || {
    echo 'ERROR: database active configuration does not match the backup configuration; keep application stopped after restore.' >&2
    return 1
  }
}

rhd_backup_verify_config() {
  local backup="$1" summary version digest expected
  if ! rhd_backup_config_managed; then return 0; fi
  [[ -f "${backup}.config.json" && ! -L "${backup}.config.json" ]] || {
    echo 'ERROR: managed backup requires its matching .config.json sidecar.' >&2
    return 1
  }
  summary="$(rhd_backup_config_summary "${backup}.config.json")" || return 1
  version="${summary%%|*}"
  digest="${summary#*|}"
  [[ "$(rhd_backup_metadata_value "${backup}.meta" config_version_id)" == "$version" &&
     "$(rhd_backup_metadata_value "${backup}.meta" config_digest)" == "$digest" ]] || {
    echo 'ERROR: archived configuration version/digest does not match backup metadata.' >&2
    return 1
  }
  expected="$(rhd_backup_metadata_value "${backup}.meta" config_sha256)" || return 1
  [[ "$expected" =~ ^[a-f0-9]{64}$ && "$expected" == "$(file_sha256 "${backup}.config.json")" ]] || {
    echo 'ERROR: archived configuration content failed its checksum.' >&2
    return 1
  }
  rhd_backup_check_config "${backup}.config.json" "$digest"
}

rhd_backup_metadata_value() {
  local metadata="$1" key="$2"
  awk -F= -v key="$key" '
    $1 == key { count++; value = substr($0, index($0, "=") + 1); sub(/\r$/, "", value) }
    END { if (count != 1 || value == "") exit 1; printf "%s", value }
  ' "$metadata"
}

rhd_backup_verify_identity() {
  local backup="$1" metadata="${1}.meta" pair key expected actual checksum
  local -a pairs=("database:$DB_NAME")
  rhd_backup_check_target_identity || return 1
  if ! rhd_backup_managed; then
    if [[ -f "$metadata" ]] && grep -q '^instance_id=' "$metadata"; then
      echo 'ERROR: this backup belongs to a managed instance; select its environment before restore.' >&2
      return 1
    fi
    if ! rhd_backup_config_managed; then
      if [[ -f "$metadata" ]] && grep -q '^config_digest=' "$metadata"; then
        echo 'ERROR: this backup has managed configuration; select its configuration scope before restore.' >&2
        return 1
      fi
      return 0
    fi
  fi
  if [[ ! -f "$metadata" ]]; then
    echo 'ERROR: managed restore requires the backup .meta identity file.' >&2
    return 1
  fi
  if rhd_backup_managed; then
    pairs+=("instance_id:$RHD_INSTANCE_ID" "deployment_project_id:$RHD_DEPLOYMENT_PROJECT_ID")
  fi
  pairs+=("environment:$RHD_PROJECT_ENVIRONMENT" "tenant_id:$RHD_PROJECT_CONFIG_TENANT_ID")
  for pair in "${pairs[@]}"; do
    key="${pair%%:*}"
    expected="${pair#*:}"
    actual="$(rhd_backup_metadata_value "$metadata" "$key")" || {
      echo "ERROR: backup identity metadata is missing or ambiguous: ${key}." >&2
      return 1
    }
    if [[ "$actual" != "$expected" ]]; then
      echo "ERROR: backup ${key} does not match the selected restore environment." >&2
      return 1
    fi
  done
  checksum="$(rhd_backup_metadata_value "$metadata" sha256)" || {
    echo 'ERROR: managed backup metadata is missing an unambiguous SHA-256.' >&2
    return 1
  }
  actual="$(file_sha256 "$backup")" || return 1
  if [[ ! "$checksum" =~ ^[a-f0-9]{64}$ || "$checksum" != "$actual" ]]; then
    echo 'ERROR: backup content does not match its identity metadata checksum.' >&2
    return 1
  fi
  rhd_backup_verify_config "$backup"
}
