#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RUNTIME_DIR="$REPO_ROOT/.local/jitsi"
RELEASE="stable-11146-2"
RELEASE_DIR="$RUNTIME_DIR/docker-jitsi-meet-$RELEASE"
COMPOSE_FILE="$RELEASE_DIR/docker-compose.yml"
TRANSCRIBER_COMPOSE_FILE="$RELEASE_DIR/transcriber.yml"
TRANSCRIBER_OVERRIDE_FILE="$REPO_ROOT/docker-compose.jigasi.override.yml"
JITSI_ENV_FILE="$RUNTIME_DIR/.env"
APP_ENV_FILE="$RUNTIME_DIR/remotehelpdesk.env"
PROJECT_NAME="remotehelpdesk-jitsi"

log() {
  printf '[jitsi-local] %s\n' "$*"
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf '[jitsi-local] required command not found: %s\n' "$1" >&2
    exit 1
  fi
}

generate_secret() {
  openssl rand -hex 24
}

set_env_value_in_file() {
  local file="$1" key="$2" value="$3" temp_file
  temp_file="$(mktemp)"
  awk -F= -v key="$key" -v value="$value" '
    BEGIN { written = 0 }
    $1 == key {
      if (!written) {
        print key "=" value
        written = 1
      }
      next
    }
    { print }
    END {
      if (!written) print key "=" value
    }
  ' "$file" >"$temp_file"
  mv "$temp_file" "$file"
}

set_env_value() {
  set_env_value_in_file "$JITSI_ENV_FILE" "$1" "$2"
}

ensure_env_value_in_file() {
  local file="$1" key="$2" value="$3"
  if ! grep -q "^${key}=" "$file"; then
    printf '%s=%s\n' "$key" "$value" >>"$file"
  fi
}

read_env_value() {
  local file="$1" key="$2"
  awk -F= -v key="$key" '$1 == key { value = substr($0, index($0, "=") + 1) } END { print value }' "$file"
}

configure_local_http_transport() {
  # stable-11031 assumes PUBLIC_URL uses HTTPS when constructing XMPP URLs.
  # Relative BOSH keeps local HTTP development same-origin and avoids the
  # invalid wss://http://localhost URL produced by that template.
  set_env_value "BOSH_RELATIVE" "1"
  set_env_value "ENABLE_XMPP_WEBSOCKET" "0"
}

configure_media_transport() {
  set_env_value "ENABLE_CODEC_AV1" "0"
  set_env_value "ENABLE_CODEC_VP8" "1"
  set_env_value "ENABLE_CODEC_VP9" "1"
  set_env_value "ENABLE_CODEC_H264" "1"
  set_env_value "CODEC_ORDER_JVB" '["VP8","H264","VP9"]'
  set_env_value "CODEC_ORDER_JVB_MOBILE" '["VP8","H264"]'
  set_env_value "CODEC_ORDER_P2P" '["VP8","H264","VP9"]'
  set_env_value "CODEC_ORDER_P2P_MOBILE" '["VP8","H264"]'
}

configure_local_app() {
  set_env_value_in_file "$APP_ENV_FILE" "JITSI_TOKEN_TTL_MINUTES" "${JITSI_LOCAL_TOKEN_TTL_MINUTES:-15}"
}

configure_local_transcription() {
  local shared_secret websocket_token event_token app_url gateway_url callback_url
  shared_secret="$(read_env_value "$APP_ENV_FILE" "SPEECH_JIGASI_SHARED_SECRET")"
  if [[ -z "$shared_secret" ]]; then
    shared_secret="$(generate_secret)"
  fi
  websocket_token="$(printf '%s' 'remotehelpdesk-jigasi-transcription-websocket-v1' | openssl dgst -sha256 -hmac "$shared_secret" | awk '{print $NF}')"
  event_token="$(printf '%s' 'remotehelpdesk-jigasi-transcription-event-v1' | openssl dgst -sha256 -hmac "$shared_secret" | awk '{print $NF}')"
  if [[ ${#websocket_token} -ne 64 || ${#event_token} -ne 64 || "$websocket_token" == "$event_token" ]]; then
    printf 'failed to derive isolated Jigasi transcription access tokens\n' >&2
    exit 1
  fi

  app_url="${JITSI_LOCAL_APP_URL:-http://host.docker.internal:8083}"
  gateway_url="${app_url/#http:/ws:}/api/third/jitsi/transcription/ws/$websocket_token"
  gateway_url="${gateway_url/#https:/wss:}"
  callback_url="$app_url/api/third/jitsi/transcription/events/$event_token"

  set_env_value "ENABLE_TRANSCRIPTIONS" "1"
  set_env_value "JIGASI_TRANSCRIBER_CUSTOM_SERVICE" "org.jitsi.jigasi.transcription.WhisperTranscriptionService"
  set_env_value "JIGASI_TRANSCRIBER_WHISPER_URL" "$gateway_url"
  set_env_value "JIGASI_TRANSCRIBER_ENABLE_SAVING" "0"
  set_env_value "JIGASI_TRANSCRIBER_FILTER_SILENCE" "0"
  set_env_value "JIGASI_CONFIGURATION" "org.jitsi.jigasi.transcription.SEND_JSON_REMOTE_URLS=$callback_url"

  set_env_value_in_file "$APP_ENV_FILE" "SPEECH_JIGASI_ENABLED" "true"
  set_env_value_in_file "$APP_ENV_FILE" "SPEECH_JIGASI_SHARED_SECRET" "$shared_secret"
  set_env_value_in_file "$APP_ENV_FILE" "SPEECH_JIGASI_MAX_PARTICIPANT_STREAMS" "8"
  set_env_value_in_file "$APP_ENV_FILE" "SPEECH_JIGASI_MAX_CONCURRENT_STREAMS" "32"
  if [[ "$(read_env_value "$APP_ENV_FILE" "SPEECH_PROVIDER")" == "mock" ]]; then
    set_env_value_in_file "$APP_ENV_FILE" "SPEECH_PROVIDER" "disabled"
  else
    ensure_env_value_in_file "$APP_ENV_FILE" "SPEECH_PROVIDER" "disabled"
  fi
  ensure_env_value_in_file "$APP_ENV_FILE" "XFYUN_RTASR_APP_ID" ""
  ensure_env_value_in_file "$APP_ENV_FILE" "XFYUN_RTASR_API_KEY" ""
  ensure_env_value_in_file "$APP_ENV_FILE" "XFYUN_RTASR_ENDPOINT" "wss://rtasr.xfyun.cn/v1/ws"
  ensure_env_value_in_file "$APP_ENV_FILE" "XFYUN_RTASR_DOMAIN" ""
  ensure_env_value_in_file "$APP_ENV_FILE" "XFYUN_TRANSLATION_ENABLED" "false"
  ensure_env_value_in_file "$APP_ENV_FILE" "XFYUN_TRANSLATION_API_SECRET" ""
  ensure_env_value_in_file "$APP_ENV_FILE" "XFYUN_TRANSLATION_ENDPOINT" "https://itrans.xfyun.cn/v2/its"
  ensure_env_value_in_file "$APP_ENV_FILE" "XFYUN_TRANSLATION_TARGET_LANGUAGE" "en"
  ensure_env_value_in_file "$APP_ENV_FILE" "MEETING_AR_PROVIDER" "disabled"
}

download_release() {
  if [[ -f "$COMPOSE_FILE" ]]; then
    return
  fi

  require_command curl
  require_command tar
  local download_dir archive source_dir
  download_dir="$(mktemp -d)"
  archive="$download_dir/docker-jitsi-meet.tar.gz"
  source_dir="$download_dir/docker-jitsi-meet-$RELEASE"
  trap 'rm -rf "$download_dir"' RETURN

  log "downloading docker-jitsi-meet $RELEASE"
  curl --fail --location --silent --show-error \
    --connect-timeout 15 --max-time 180 --retry 3 \
    "https://codeload.github.com/jitsi/docker-jitsi-meet/tar.gz/refs/tags/$RELEASE" \
    --output "$archive"
  tar -xzf "$archive" -C "$download_dir"
  mv "$source_dir" "$RELEASE_DIR"
  trap - RETURN
  rm -rf "$download_dir"
}

initialize() {
  require_command docker
  require_command openssl
  mkdir -p "$RUNTIME_DIR"
  download_release

  if [[ -f "$JITSI_ENV_FILE" && -f "$APP_ENV_FILE" ]]; then
    configure_local_http_transport
    configure_local_app
    configure_local_transcription
    return
  fi

  local jwt_secret webhook_secret public_url advertise_ips
  jwt_secret="$(generate_secret)"
  webhook_secret="$(generate_secret)"
  public_url="${JITSI_LOCAL_PUBLIC_URL:-http://localhost:8000}"
  advertise_ips="${JITSI_LOCAL_ADVERTISE_IPS:-127.0.0.1}"

  cp "$RELEASE_DIR/env.example" "$JITSI_ENV_FILE"
  {
    printf '\n# RemoteHelpDesk local overrides\n'
    printf 'CONFIG=%s\n' "$RUNTIME_DIR/config"
    printf 'HTTP_PORT=8000\n'
    printf 'HTTPS_PORT=8443\n'
    printf 'PUBLIC_URL=%s\n' "$public_url"
    printf 'JVB_ADVERTISE_IPS=%s\n' "$advertise_ips"
    printf 'JVB_PORT=10000\n'
    printf 'JVB_COLIBRI_PORT=8080\n'
    printf 'TZ=Asia/Shanghai\n'
    printf 'DISABLE_HTTPS=1\n'
    printf 'ENABLE_HTTP_REDIRECT=0\n'
    printf 'ENABLE_AUTH=1\n'
    printf 'ENABLE_GUESTS=0\n'
    printf 'AUTH_TYPE=jwt\n'
    printf 'JWT_APP_ID=remotehelpdesk\n'
    printf 'JWT_APP_SECRET=%s\n' "$jwt_secret"
    printf 'JWT_ENABLE_DOMAIN_VERIFICATION=0\n'
    printf 'BOSH_RELATIVE=1\n'
    printf 'ENABLE_XMPP_WEBSOCKET=0\n'
    printf 'ENABLE_PREJOIN_PAGE=0\n'
    printf 'ENABLE_WELCOME_PAGE=0\n'
    printf 'ENABLE_CLOSE_PAGE=0\n'
    printf 'ENABLE_TRANSCRIPTIONS=1\n'
    printf 'JICOFO_AUTH_PASSWORD=%s\n' "$(generate_secret)"
    printf 'JVB_AUTH_PASSWORD=%s\n' "$(generate_secret)"
    printf 'JIGASI_XMPP_PASSWORD=%s\n' "$(generate_secret)"
    printf 'JIGASI_TRANSCRIBER_PASSWORD=%s\n' "$(generate_secret)"
    printf 'JIBRI_RECORDER_PASSWORD=%s\n' "$(generate_secret)"
    printf 'JIBRI_XMPP_PASSWORD=%s\n' "$(generate_secret)"
    printf 'RESTART_POLICY=unless-stopped\n'
    printf 'JITSI_IMAGE_VERSION=%s\n' "$RELEASE"
  } >>"$JITSI_ENV_FILE"

  {
    printf '# Source this file before starting the RemoteHelpDesk backend.\n'
    printf 'JITSI_URL=%s\n' "$public_url"
    printf 'JITSI_DOMAIN=%s\n' "${public_url#*://}"
    printf 'JITSI_APP_ID=remotehelpdesk\n'
    printf 'JITSI_APP_SECRET=%s\n' "$jwt_secret"
    printf 'JITSI_WEBHOOK_SECRET=%s\n' "$webhook_secret"
    printf 'JITSI_REQUIRE_AUTH=true\n'
    printf 'JITSI_TOKEN_TTL_MINUTES=15\n'
    printf 'SPEECH_PROVIDER=disabled\n'
    printf 'XFYUN_RTASR_APP_ID=\n'
    printf 'XFYUN_RTASR_API_KEY=\n'
    printf 'XFYUN_RTASR_ENDPOINT=wss://rtasr.xfyun.cn/v1/ws\n'
    printf 'XFYUN_RTASR_DOMAIN=\n'
    printf 'XFYUN_TRANSLATION_ENABLED=false\n'
    printf 'XFYUN_TRANSLATION_API_SECRET=\n'
    printf 'XFYUN_TRANSLATION_ENDPOINT=https://itrans.xfyun.cn/v2/its\n'
    printf 'XFYUN_TRANSLATION_TARGET_LANGUAGE=en\n'
    printf 'MEETING_AR_PROVIDER=disabled\n'
  } >"$APP_ENV_FILE"
  chmod 600 "$JITSI_ENV_FILE" "$APP_ENV_FILE"
  configure_local_http_transport
  configure_media_transport
  configure_local_app
  configure_local_transcription
  log "initialized local configuration in $RUNTIME_DIR"
}

compose() {
  docker compose \
    --project-name "$PROJECT_NAME" \
    --env-file "$JITSI_ENV_FILE" \
    --file "$COMPOSE_FILE" \
    --file "$TRANSCRIBER_COMPOSE_FILE" \
    --file "$TRANSCRIBER_OVERRIDE_FILE" \
    "$@"
}

wait_until_prosody_ready() {
  local attempt
  for attempt in $(seq 1 60); do
    if compose exec --no-TTY prosody bash -lc 'exec 3<>/dev/tcp/127.0.0.1/5222' >/dev/null 2>&1; then
      log "Prosody XMPP is ready"
      return
    fi
    sleep 1
  done
  log "Prosody XMPP startup timed out; inspect logs with: $0 logs"
  return 1
}

wait_until_transcriber_ready() {
  local attempt state
  for attempt in $(seq 1 60); do
    state="$({ compose logs --no-color --since 2m transcriber 2>&1 || true; } | awk '
      /Failed to connect to XMPP service|was not authenticated/ { state = "failed" }
      /Joined call control room:/ { state = "ready" }
      END { print state }
    ')"
    if [[ "$state" == "ready" ]]; then
      log "Jigasi transcriber is registered"
      return
    fi
    sleep 1
  done
  log "Jigasi transcriber registration timed out; inspect logs with: $0 logs"
  return 1
}

wait_until_ready() {
  local attempt
  for attempt in $(seq 1 60); do
    if curl --fail --silent "http://localhost:8000/external_api.js" >/dev/null 2>&1; then
      log "ready at http://localhost:8000"
      return
    fi
    sleep 2
  done
  log "startup timed out; inspect logs with: $0 logs"
  return 1
}

patch_external_api_for_http() {
  local public_url web_container external_api_file
  public_url="$(read_env_value "$JITSI_ENV_FILE" "PUBLIC_URL")"
  if [[ "$public_url" != http://localhost:* && "$public_url" != http://127.0.0.1:* ]]; then
    return
  fi

  web_container="$(compose ps --quiet web)"
  external_api_file="/usr/share/jitsi-meet/libs/external_api.min.js"
  if [[ -z "$web_container" ]]; then
    log "web container is unavailable; cannot enable HTTP iframe development mode"
    return 1
  fi
  if docker exec "$web_container" grep -q 'url:`https://${t}/' "$external_api_file"; then
    docker exec "$web_container" sed -i 's#url:`https://${t}/#url:`http://${t}/#' "$external_api_file"
    log "enabled loopback HTTP support for the Jitsi iframe API"
  fi
}

patch_token_affiliation() {
  local attempt prosody_config temp_file
  prosody_config="$RUNTIME_DIR/config/prosody/config/conf.d/jitsi-meet.cfg.lua"

  # The upstream JWT template validates tokens but does not map the
  # context.user.moderator claim to the Prosody room owner affiliation.
  for attempt in $(seq 1 60); do
    if compose logs --no-color prosody 2>&1 | grep -q '\[register-setup\] Prosody is ready!'; then
      break
    fi
    sleep 1
  done
  if [[ ! -f "$prosody_config" ]] || ! grep -q '"token_verification";' "$prosody_config"; then
    log "Prosody configuration is unavailable; cannot enable JWT moderator roles"
    return 1
  fi
  if grep -q '"token_affiliation";' "$prosody_config"; then
    return
  fi

  temp_file="$(mktemp)"
  awk '
    { print }
    /"token_verification";/ { print "        \"token_affiliation\";" }
  ' "$prosody_config" >"$temp_file"
  mv "$temp_file" "$prosody_config"
  compose exec --no-TTY prosody s6-svc -r /var/run/s6/services/50-prosody
  log "enabled JWT moderator-to-room-owner mapping"
}

start_stack() {
  # Jigasi does not reliably recover when its first XMPP connection races
  # Prosody startup, so register it only after the core XMPP service is live.
  compose up --detach prosody web jicofo jvb
  wait_until_prosody_ready
  patch_token_affiliation
  sleep 1
  wait_until_prosody_ready
  compose up --detach --no-deps transcriber
  wait_until_transcriber_ready
  wait_until_ready
  patch_external_api_for_http
}

usage() {
  printf 'Usage: %s {init|start|stop|restart|status|logs|app-env}\n' "$0"
}

command="${1:-start}"
case "$command" in
  init)
    initialize
    ;;
  start)
    initialize
    start_stack
    ;;
  stop)
    initialize
    compose down
    ;;
  restart)
    initialize
    compose down
    start_stack
    ;;
  status)
    initialize
    compose ps
    ;;
  logs)
    initialize
    compose logs --follow --tail 200
    ;;
  app-env)
    initialize
    printf '%s\n' "$APP_ENV_FILE"
    ;;
  *)
    usage >&2
    exit 1
    ;;
esac
