#!/usr/bin/env bash
# Go backend hot-reload watcher (no external deps; polling-based).
# Rebuilds and restarts the backend whenever a watched .go/.yaml file changes.
# Usage: ./scripts/dev-backend.sh
set -uo pipefail

cd "$(dirname "$0")/.."   # repo root
ROOT="$(pwd)"

load_dev_env() {
  local env_file
  for env_file in ".env" ".env.local" ".local/jitsi/remotehelpdesk.env"; do
    if [[ -f "$env_file" ]]; then
      set -a
      # shellcheck disable=SC1090
      source "$env_file"
      set +a
    fi
  done
  if [[ -z "${RHD_REDIS_PASSWORD:-}" && -n "${REDIS_PASSWORD:-}" ]]; then
    export RHD_REDIS_PASSWORD="$REDIS_PASSWORD"
  fi
}

load_dev_env

BIN="./tmp/main"
CONFIG="config/config.yaml"
PIDFILE="./tmp/backend.pid"
LOCKDIR="./tmp/dev-backend.lock"
POLL_INTERVAL="${POLL_INTERVAL:-2}"

# Directories/patterns to watch for changes
WATCH_DIRS=(internal cmd)
WATCH_EXTRA=(config/config.yaml .env .env.local .local/jitsi/remotehelpdesk.env web/embed.go web/embed_dev.go)

mkdir -p tmp
if ! mkdir "$LOCKDIR" 2>/dev/null; then
  lock_pid="$(cat "$LOCKDIR/pid" 2>/dev/null || true)"
  if [[ -n "$lock_pid" ]] && kill -0 "$lock_pid" 2>/dev/null; then
    echo "[dev-backend] watcher is already running (pid $lock_pid)" >&2
    exit 1
  fi
  rm -f "$LOCKDIR/pid"
  rmdir "$LOCKDIR" 2>/dev/null || {
    echo "[dev-backend] cannot recover stale watcher lock: $LOCKDIR" >&2
    exit 1
  }
  mkdir "$LOCKDIR"
fi
echo $$ > "$LOCKDIR/pid"

c_green="\033[0;32m"; c_yellow="\033[0;33m"; c_red="\033[0;31m"; c_cyan="\033[0;36m"; c_reset="\033[0m"
log() { echo -e "${c_cyan}[dev-backend]${c_reset} $*"; }

build() {
  go build -tags dev -o "$BIN" ./cmd/server
}

start() {
  load_dev_env
  "$BIN" -config "$CONFIG" &
  echo $! > "$PIDFILE"
  log "started backend (pid $(cat "$PIDFILE")) at http://127.0.0.1:${RHD_SERVER_PORT:-8083}"
}

stop() {
  if [[ -f "$PIDFILE" ]]; then
    local pid; pid="$(cat "$PIDFILE")"
    if kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null
      # give it a moment, then force
      for _ in 1 2 3 4 5; do kill -0 "$pid" 2>/dev/null || break; sleep 0.3; done
      kill -0 "$pid" 2>/dev/null && kill -9 "$pid" 2>/dev/null
    fi
    rm -f "$PIDFILE"
  fi
}

backend_running() {
  [[ -f "$PIDFILE" ]] || return 1
  local pid state
  pid="$(cat "$PIDFILE" 2>/dev/null || true)"
  [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null || return 1
  state="$(ps -p "$pid" -o stat= 2>/dev/null | tr -d "[:space:]")"
  [[ -n "$state" && "$state" != Z* ]]
}

# Snapshot mtimes of all watched files (macOS stat -f)
snapshot() {
  {
    for d in "${WATCH_DIRS[@]}"; do
      find "$d" -type f \( -name '*.go' -o -name '*.yaml' -o -name '*.yml' \) 2>/dev/null
    done
    for f in "${WATCH_EXTRA[@]}"; do
      [[ -f "$f" ]] && echo "$f"
    done
  } | while read -r f; do
      stat -f '%m %N' "$f" 2>/dev/null
    done | sort
}

cleanup() {
  log "shutting down..."
  stop
  if [[ "$(cat "$LOCKDIR/pid" 2>/dev/null || true)" = "$$" ]]; then
    rm -f "$LOCKDIR/pid"
    rmdir "$LOCKDIR" 2>/dev/null || true
  fi
  exit 0
}
trap cleanup INT TERM

log "initial build..."
if build; then
  log "build OK"
  start
else
  log "${c_red}initial build FAILED — waiting for changes to retry${c_reset}"
fi

prev="$(snapshot)"
while true; do
  sleep "$POLL_INTERVAL"
  cur="$(snapshot)"
  if [[ "$cur" != "$prev" ]]; then
    log "${c_yellow}change detected — rebuilding...${c_reset}"
    stop
    if build; then
      log "${c_green}build OK — restarting${c_reset}"
      start
    else
      log "${c_red}build FAILED — fix errors; watcher keeps running${c_reset}"
    fi
    prev="$(snapshot)"
  elif ! backend_running; then
    log "${c_yellow}backend exited — restarting${c_reset}"
    if [[ -x "$BIN" ]]; then
      start
    elif build; then
      start
    else
      log "${c_red}restart build FAILED — watcher keeps running${c_reset}"
    fi
  fi
done
