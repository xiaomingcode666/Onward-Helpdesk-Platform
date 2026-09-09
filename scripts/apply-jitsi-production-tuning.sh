#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RUNTIME_ENV="$REPO_ROOT/deploy/jitsi/runtime.env"
SYSCTL_CONFIG="$REPO_ROOT/deploy/jitsi/99-jitsi-streaming.conf"
JITSI_HOST="${JITSI_DEPLOY_HOST:-43.165.7.131}"
JITSI_USER="${JITSI_DEPLOY_USER:-ubuntu}"
JITSI_PORT="${JITSI_DEPLOY_PORT:-22}"
JITSI_DIR="${JITSI_DEPLOY_DIR:-/opt/jitsi/current}"
PROJECT_NAME="${JITSI_PROJECT_NAME:-remotehelpdesk-jitsi}"

die() {
  printf '[jitsi-tuning] %s\n' "$*" >&2
  exit 1
}

for command_name in ssh sshpass; do
  command -v "$command_name" >/dev/null 2>&1 || die "required command not found: $command_name"
done
[[ -f "$RUNTIME_ENV" ]] || die "missing $RUNTIME_ENV"
[[ -f "$SYSCTL_CONFIG" ]] || die "missing $SYSCTL_CONFIG"

deploy_password="${JITSI_DEPLOY_PASSWORD:-}"
if [[ -z "$deploy_password" ]]; then
  [[ -t 0 ]] || die "set JITSI_DEPLOY_PASSWORD for non-interactive use"
  read -r -s -p 'Jitsi SSH password: ' deploy_password
  printf '\n'
fi

export SSHPASS="$deploy_password"
ssh_options=(-o StrictHostKeyChecking=accept-new -o ConnectTimeout=10 -p "$JITSI_PORT")
remote_target="$JITSI_USER@$JITSI_HOST"

sshpass -e ssh "${ssh_options[@]}" "$remote_target" \
  'sudo install -m 0644 /dev/stdin /etc/sysctl.d/99-jitsi-streaming.conf && sudo sysctl --system >/dev/null' \
  <"$SYSCTL_CONFIG"

# runtime.env contains only reviewed, non-secret values.
set -a
source "$RUNTIME_ENV"
set +a

sshpass -e ssh "${ssh_options[@]}" "$remote_target" \
  bash -s -- \
  "$JITSI_DIR" "$PROJECT_NAME" \
  "$JITSI_IMAGE_VERSION" "$ENABLE_CODEC_AV1" "$ENABLE_CODEC_VP8" "$ENABLE_CODEC_VP9" "$ENABLE_CODEC_H264" \
  "$CODEC_ORDER_JVB" "$CODEC_ORDER_JVB_MOBILE" "$CODEC_ORDER_P2P" "$CODEC_ORDER_P2P_MOBILE" <<'REMOTE_SCRIPT'
set -euo pipefail
jitsi_dir="$1"
project_name="$2"
shift 2
cd "$jitsi_dir"
cp .env ".env.bak-$(date -u +%Y%m%dT%H%M%SZ)"
keys=(
  JITSI_IMAGE_VERSION ENABLE_CODEC_AV1 ENABLE_CODEC_VP8 ENABLE_CODEC_VP9 ENABLE_CODEC_H264
  CODEC_ORDER_JVB CODEC_ORDER_JVB_MOBILE CODEC_ORDER_P2P CODEC_ORDER_P2P_MOBILE
)
for key in "${keys[@]}"; do
  value="$1"
  shift
  if grep -q "^${key}=" .env; then
    sed -i "s#^${key}=.*#${key}=${value}#" .env
  else
    printf '%s=%s\n' "$key" "$value" >>.env
  fi
done

compose=(sudo docker compose --env-file .env -f docker-compose.yml -f transcriber.yml -p "$project_name")
"${compose[@]}" pull web prosody jicofo jvb transcriber
"${compose[@]}" up -d --force-recreate web prosody jicofo jvb transcriber

prosody_container=""
for _ in $(seq 1 60); do
  prosody_container="$("${compose[@]}" ps -q prosody)"
  if [[ -n "$prosody_container" ]] && [[ "$(sudo docker inspect --format '{{.State.Running}}' "$prosody_container")" == "true" ]]; then
    break
  fi
  sleep 1
done
[[ -n "$prosody_container" ]] || { printf 'Prosody container is unavailable\n' >&2; exit 1; }

prosody_config_root="$(sudo docker inspect --format '{{range .Mounts}}{{if eq .Destination "/config"}}{{.Source}}{{end}}{{end}}' "$prosody_container")"
prosody_config="$prosody_config_root/conf.d/jitsi-meet.cfg.lua"
for _ in $(seq 1 60); do
  [[ -f "$prosody_config" ]] && grep -q '"token_verification";' "$prosody_config" && break
  sleep 1
done
[[ -f "$prosody_config" ]] && grep -q '"token_verification";' "$prosody_config" || {
  printf 'Prosody JWT configuration is unavailable: %s\n' "$prosody_config" >&2
  exit 1
}
if ! grep -q '"token_affiliation";' "$prosody_config"; then
  patched_config="$(mktemp)"
  awk '{ print } /"token_verification";/ { print "        \"token_affiliation\";" }' "$prosody_config" >"$patched_config"
  sudo cp "$patched_config" "$prosody_config"
  rm -f "$patched_config"
  "${compose[@]}" restart prosody
fi
"${compose[@]}" ps
REMOTE_SCRIPT

unset SSHPASS deploy_password
