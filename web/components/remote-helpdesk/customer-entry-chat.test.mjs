import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import test from "node:test"

const source = readFileSync(new URL("./customer-entry-chat.tsx", import.meta.url), "utf8")

test("customer entry chat keeps the chat shell during initial loading", () => {
  assert.match(source, /cec\(t, "serviceSession"\)/)
  assert.match(source, /cec\(t, "conversationLoading"\)/)
  assert.match(source, /placeholder=\{cec\(t, "inputPlaceholder"\)\}/)
  assert.doesNotMatch(source, /产品服务助手/)
  assert.doesNotMatch(source, /正在建立安全会话/)
})

test("customer entry chat avoids explanatory AI copy", () => {
  assert.match(source, /cec\(t, "stageAi"\)/)
  assert.match(source, /SupportStep label=\{cec\(t, "stepOnline"\)\}/)
  assert.match(source, /SELF_SERVICE_QUICK_PROMPTS/)
  assert.doesNotMatch(source, /AI_ONLY_QUICK_PROMPTS/)
  assert.doesNotMatch(source, /服务助手在线/)
  assert.doesNotMatch(source, /需要更多诊断建议/)
  assert.doesNotMatch(source, /description: "结果已归档。"/)
  assert.doesNotMatch(source, /AI 助手正在接待/)
  assert.doesNotMatch(source, /AI 接待/)
  assert.doesNotMatch(source, /系统会把当前会话/)
  assert.doesNotMatch(source, /转接后仍可继续/)
  assert.doesNotMatch(source, /实时分析中/)
  assert.doesNotMatch(source, /快速描述问题/)
})

test("customer entry chat limits guest questions", () => {
  assert.match(source, /CUSTOMER_GUEST_QUESTION_LIMIT/)
  assert.match(source, /guestQuestionCount >= CUSTOMER_GUEST_QUESTION_LIMIT/)
  assert.match(source, /cec\(t, "guestQuestionLimit"/)
})
