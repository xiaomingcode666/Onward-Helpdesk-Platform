#!/usr/bin/env bash
# Test an OpenAI-compatible gateway: /models + /responses (non-stream & stream).
# Usage:
#   ./scripts/test-openai-endpoint.sh [model]
# Env overrides: BASE_URL, API_KEY, MODEL
set -u

BASE_URL="${BASE_URL:-http://43.160.245.179:8080}"
API_KEY="${API_KEY:-sk-38eea01324763b417fbbc50febe45b70502c284b9f9171e767e8e51ba8ab2467}"
MODEL="${1:-${MODEL:-gpt-5.4-mini}}"
TIMEOUT=60
AUTH="Authorization: Bearer $API_KEY"

PASS=0; FAIL=0; FAILED_NAMES=()

section() { printf '\n\033[1;36m== %s ==\033[0m\n' "$1"; }
ok()   { PASS=$((PASS+1)); printf '\033[32m[PASS]\033[0m %s\n' "$1"; }
fail() { FAIL=$((FAIL+1)); FAILED_NAMES+=("$1"); printf '\033[31m[FAIL]\033[0m %s\n' "$1"; }
ms()   { awk -v d="$1" 'BEGIN{printf "%.0f ms", d*1000}'; }

section "1. GET /models — gateway 与鉴权"
code=$(curl -s -m 10 -o /tmp/models.json -w "%{http_code}" "$BASE_URL/models" -H "$AUTH")
if [ "$code" = "200" ]; then
  n=$(jq '.data | length' /tmp/models.json 2>/dev/null || echo 0)
  ok "GET /models → 200, $n 个模型"
else
  fail "GET /models → HTTP $code"
fi

section "2. POST /responses — 非流式对话 (model=$MODEL)"
t0=$(date +%s.%N)
code=$(curl -s -m $TIMEOUT -o /tmp/resp.json -w "%{http_code}" "$BASE_URL/responses" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"model\":\"$MODEL\",\"input\":\"用一句话介绍你自己\"}")
t1=$(date +%s.%N)
lat=$(awk -v a="$t0" -v b="$t1" 'BEGIN{printf "%.3f", b-a}')
if [ "$code" = "200" ]; then
  text=$(jq -r '[.output[]? | select(.type=="message") | .content[]? | select(.type=="output_text") | .text] | join("")' /tmp/resp.json)
  in_tok=$(jq -r '.usage.input_tokens // "?"' /tmp/resp.json)
  out_tok=$(jq -r '.usage.output_tokens // "?"' /tmp/resp.json)
  ok "HTTP 200 · 延迟 ${lat}s · in=${in_tok} out=${out_tok}"
  printf '\033[2m回复: %s\033[0m\n' "$(printf '%s' "$text" | head -c 300)"
else
  msg=$(jq -r '.error.message // "no body"' /tmp/resp.json 2>/dev/null)
  fail "HTTP $code · ${lat}s · 错误: $msg"
fi

section "3. POST /responses — 流式 (stream=true)"
t0=$(date +%s.%N)
code=$(curl -s -m $TIMEOUT -N -o /tmp/resp_stream.txt -w "%{http_code}" "$BASE_URL/responses" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"model\":\"$MODEL\",\"input\":\"Count to 3\",\"stream\":true}")
t1=$(date +%s.%N)
lat=$(awk -v a="$t0" -v b="$t1" 'BEGIN{printf "%.3f", b-a}')
if [ "$code" = "200" ]; then
  n_chunks=$(wc -l < /tmp/resp_stream.txt)
  has_done=$(grep -c '"type":"response.completed"' /tmp/resp_stream.txt || true)
  [ "$has_done" -gt 0 ] && ok "HTTP 200 · ${lat}s · $n_chunks 行 SSE, 含 response.completed" \
                       || fail "HTTP 200 但未见 response.completed 事件"
  printf '\033[2m末尾 3 行:\033[0m\n'; tail -3 /tmp/resp_stream.txt | sed 's/^/  /' | head -c 600; echo
else
  msg=$(head -c 200 /tmp/resp_stream.txt)
  fail "HTTP $code · ${lat}s · $msg"
fi

section "4. 鉴权校验 — 错误 key 应被拒绝"
code=$(curl -s -m 10 -o /dev/null -w "%{http_code}" "$BASE_URL/models" -H "Authorization: Bearer sk-invalid-key")
if [ "$code" = "401" ]; then
  ok "错误 key → HTTP 401"
else
  fail "错误 key → HTTP $code (期望 401)"
fi

printf '\n\033[1;37m结果: %d 通过, %d 失败\033[0m\n' "$PASS" "$FAIL"
[ $FAIL -gt 0 ] && printf '\033[31m失败项: %s\033[0m\n' "${FAILED_NAMES[*]}"
exit $((FAIL > 0 ? 1 : 0))
