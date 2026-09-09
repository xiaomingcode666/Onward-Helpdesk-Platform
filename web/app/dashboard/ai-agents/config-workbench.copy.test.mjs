import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./_components/config-workbench.tsx", import.meta.url), "utf8")
const locales = ["zh-CN", "en-US", "es-ES"]
const messagesByLocale = await Promise.all(
  locales.map(async (locale) => [
    locale,
    JSON.parse(await readFile(new URL(`../../../messages/${locale}.json`, import.meta.url), "utf8")),
  ]),
)

test("reception bot config workbench uses module-level loading", () => {
  assert.match(source, /ModuleLoading/)
  assert.match(source, /ac\(t, "header\.loadingExisting"\)/)
  assert.doesNotMatch(source, /加载中\.\.\./)
})

test("reception bot handoff mode stays concise", () => {
  assert.match(source, /handoffModeLabels/)
  assert.doesNotMatch(source, /handoffModeDetails/)
  assert.doesNotMatch(source, /生产环境推荐/)
  assert.doesNotMatch(source, /（推荐）/)
  assert.doesNotMatch(source, /建议补充信息/)
  assert.doesNotMatch(source, /适合通用咨询/)
  assert.doesNotMatch(source, /会创建工单并通知人工/)
  assert.doesNotMatch(source, /仅适合非高危/)
  assert.doesNotMatch(source, /description:\s*"[^"]+"/)
  assert.doesNotMatch(source, /handoffModeDetail/)
})

test("reception bot config workbench avoids instructional helper copy", () => {
  assert.match(source, /ac\(t, "aiConfig\.platform"\)/)
  assert.match(source, /ac\(t, "serviceMode\.aiOnly"\)/)
  assert.match(source, /ac\(t, "serviceMode\.aiFirst"\)/)
  assert.match(source, /ac\(t, "skills\.none"\)/)
  assert.match(source, /ac\(t, "overview\.inNodeCapability"\)/)
  assert.doesNotMatch(source, /节点内智能能力/)
  assert.doesNotMatch(source, /Eino/)
  assert.doesNotMatch(source, /仅 AI/)
  assert.doesNotMatch(source, /AI 优先/)
  assert.doesNotMatch(source, /AI 保持接待/)
  assert.doesNotMatch(source, /新建 AI 机器人/)
  assert.doesNotMatch(source, /接待机器人|机器人配置|机器人已创建/)
  assert.doesNotMatch(source, /平台统一模型能力/)
  assert.doesNotMatch(source, /平台统一 AI 能力/)
  assert.doesNotMatch(source, /暂无已部署版本，用户端未启用/)
  assert.doesNotMatch(source, /当前不可变工作流版本/)
  assert.doesNotMatch(source, /由工作流固定/)
  assert.doesNotMatch(source, /运行时不会启用/)
  assert.doesNotMatch(source, /建议配置/)
  assert.doesNotMatch(source, /用于维护条件/)
  assert.doesNotMatch(source, /无法按值班、技能和并发量派单/)
  assert.doesNotMatch(source, /能力由当前版本实际节点决定/)
  assert.doesNotMatch(source, /当前版本仅允许节点执行/)
  assert.doesNotMatch(source, /未编排的动作不可由提示词/)
  assert.doesNotMatch(source, /当前版本没有外部动作节点/)
  assert.doesNotMatch(source, /未填写职责边界/)
  assert.match(source, /ac\(t, "overview\.noExternalAction"\)/)

  for (const [locale, messages] of messagesByLocale) {
    assert.notEqual(messages.aiAgent.fallbackSuggestRetry, "建议补充信息", `${locale} should keep fallback copy concise`)
    assert.notEqual(messages.aiAgent.fallbackSuggestRetry, "Ask for more details", `${locale} should keep fallback copy concise`)
    assert.doesNotMatch(messages.aiAgent.replyTimeoutHelp, /异步执行超时时间|async automated replies/)
    assert.doesNotMatch(messages.aiAgent.knowledgeHint, /优先级|priority order/)
    assert.doesNotMatch(messages.aiAgent.fallbackMessageHint, /策略和文案|strategy and message/)
    assert.doesNotMatch(messages.aiAgent.skillsHint, /用于|Use skills for/)
    assert.doesNotMatch(messages.aiAgent.noSkillsHint, /自动路由|routing only uses/)
    assert.doesNotMatch(messages.aiAgent.directToolsHint, /低风险、原子化|low-risk atomic/)
    assert.doesNotMatch(messages.aiAgent.noDirectToolsHint, /normal replies/)
    assert.doesNotMatch(messages.aiAgent.graphToolsHint, /不再混放|kept separate/)
    assert.doesNotMatch(messages.aiAgent.noGraphToolsHint, /will not expose/)
  }
})
