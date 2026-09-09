#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${ANTHROPIC_BASE_URL:-https://token-plan.cn-beijing.maas.aliyuncs.com/apps/anthropic}"
MODEL="${ANTHROPIC_MODEL:-qwen3.8-max-preview}"
PROMPT="${1:-hi}"
TOKEN="${ANTHROPIC_AUTH_TOKEN:-${ANTHROPIC_API_KEY:-}}"

if [[ -z "$TOKEN" ]]; then
  echo "Missing ANTHROPIC_AUTH_TOKEN or ANTHROPIC_API_KEY" >&2
  exit 2
fi

if ! command -v node >/dev/null 2>&1; then
  echo "node is required to build the JSON payload" >&2
  exit 2
fi

payload="$(
  MODEL="$MODEL" PROMPT="$PROMPT" node -e '
    const payload = {
      model: process.env.MODEL,
      max_tokens: 64,
      messages: [{ role: "user", content: process.env.PROMPT }],
    };
    process.stdout.write(JSON.stringify(payload));
  '
)"

call_once() {
  local label="$1"
  local auth_mode="$2"
  local url="${BASE_URL%/}/v1/messages"
  local tmp_body
  tmp_body="$(mktemp)"

  local -a headers=(
    -H "content-type: application/json"
    -H "anthropic-version: 2023-06-01"
  )

  case "$auth_mode" in
    x-api-key)
      headers+=(-H "x-api-key: $TOKEN")
      ;;
    bearer)
      headers+=(-H "authorization: Bearer $TOKEN")
      ;;
    *)
      echo "Unknown auth mode: $auth_mode" >&2
      return 2
      ;;
  esac

  echo "== $label =="
  echo "POST $url"
  echo "model=$MODEL"

  status="$(
    curl -sS \
      -o "$tmp_body" \
      -w "%{http_code}" \
      -X POST "$url" \
      "${headers[@]}" \
      --data "$payload"
  )"

  echo "status=$status"
  if command -v jq >/dev/null 2>&1; then
    jq . "$tmp_body" 2>/dev/null || cat "$tmp_body"
  else
    cat "$tmp_body"
  fi
  echo
  rm -f "$tmp_body"
}

call_once "Anthropic x-api-key header" "x-api-key"
call_once "Bearer authorization header" "bearer"
