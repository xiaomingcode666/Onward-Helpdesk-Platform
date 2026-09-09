import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const sourceFiles = await Promise.all([
  "_components/summary-cards.tsx",
  "_components/trend-panel.tsx",
  "_components/alert-list.tsx",
  "_components/team-load-panel.tsx",
  "help/page.tsx",
  "settings/page.tsx",
].map(async (file) => readFile(new URL(file, import.meta.url), "utf8")))
const helpPageSource = await readFile(new URL("./help/page.tsx", import.meta.url), "utf8")
const settingsPageSource = await readFile(new URL("./settings/page.tsx", import.meta.url), "utf8")
const mcpPageSource = await readFile(new URL("./mcp/page.tsx", import.meta.url), "utf8")
const dialogCopySources = await Promise.all([
  "skill-definition/_components/debug-dialog.tsx",
  "knowledge/_components/knowledge-bulk-move-dialog.tsx",
  "knowledge/_components/faq-import-dialog.tsx",
].map(async (file) => readFile(new URL(file, import.meta.url), "utf8")))

const localeFiles = await Promise.all([
  "../../messages/zh-CN.json",
  "../../messages/en-US.json",
  "../../messages/es-ES.json",
].map(async (file) => readFile(new URL(file, import.meta.url), "utf8")))

test("dashboard shared panels avoid explanatory CardDescription copy", () => {
  for (const source of sourceFiles) {
    assert.doesNotMatch(source, /CardDescription/)
    assert.doesNotMatch(source, /description=\{t\("(?:help|settings|dashboardHome|scaffold)\./)
  }
})

test("dashboard placeholder pages do not expose development scaffold language", () => {
  const combinedLocales = localeFiles.join("\n")
  const combinedSources = sourceFiles.join("\n")

  assert.doesNotMatch(combinedSources, /nextPhase|placeholder\.status|statusDescription|scaffold\./)
  assert.doesNotMatch(combinedSources, /eyebrow=|"Help"|"Settings"/)
  assert.doesNotMatch(combinedSources, /DashboardPlaceholder|dashboard-placeholder/)
  assert.doesNotMatch(combinedLocales, /"scaffold"\s*:/)
  assert.doesNotMatch(combinedLocales, /帮助中心骨架|后台一期|真实 API|演示数据|后续可替换/)
  assert.doesNotMatch(combinedLocales, /Help Center Scaffold|first-phase admin scaffold|Demo data showing|UI scaffold is ready/)
  assert.doesNotMatch(combinedLocales, /Connect Markdown or a documentation center later/)

  for (const localeSource of localeFiles) {
    const messages = JSON.parse(localeSource)
    assert.equal(Object.hasOwn(messages, "scaffold"), false)
    assert.equal(Object.hasOwn(messages.settings, "description"), false)
    for (const section of ["general", "notifications", "security", "sla", "members", "usage"]) {
      assert.equal(Object.hasOwn(messages.settings[section], "description"), false)
    }
    assert.equal(Object.hasOwn(messages.enterpriseDiagnosis, "nodeDescriptionEmpty"), false)
    assert.equal(Object.hasOwn(messages.mcp, "noDescription"), false)
    assert.equal(Object.hasOwn(messages.agentProfile, "noDescription"), false)
    assert.equal(Object.hasOwn(messages.ticket, "noDescription"), false)
    assert.equal(Object.hasOwn(messages.skillDefinition, "noDescription"), false)
    assert.equal(Object.hasOwn(messages.workflowRun, "skillDescriptionUnavailable"), false)
  }
})

test("dashboard help and settings are real entry pages instead of placeholder scaffolds", () => {
  assert.match(helpPageSource, /<DashboardPage>/)
  assert.match(settingsPageSource, /<DashboardPage>/)
  assert.match(helpPageSource, /nav\.productCenter/)
  assert.match(helpPageSource, /help\.checkTitle/)
  assert.match(settingsPageSource, /settings\.openConfig/)
  assert.match(settingsPageSource, /settings\.commonEntries/)
  assert.doesNotMatch(helpPageSource, /产品与设备|处理检查|后台帮助/)
  assert.doesNotMatch(settingsPageSource, /打开配置|常用入口|角色权限/)
})

test("dashboard MCP tool drawer avoids repeated helper copy", () => {
  assert.doesNotMatch(mcpPageSource, /DrawerDescription/)
  assert.doesNotMatch(mcpPageSource, /mcp\.detailDescription/)
  for (const localeSource of localeFiles) {
    const messages = JSON.parse(localeSource)
    assert.equal(Object.hasOwn(messages.mcp, "detailDescription"), false)
  }
})

test("dashboard dialogs avoid explanatory description copy", () => {
  for (const source of dialogCopySources) {
    assert.doesNotMatch(source, /FieldDescription/)
    assert.doesNotMatch(source, /description=\{t\("skillDefinition\.debugDescription"/)
    assert.doesNotMatch(source, /description=\{t\("knowledge\.batchMoveDescription"/)
    assert.doesNotMatch(source, /description=\{t\("knowledge\.importFAQDescription"/)
    assert.doesNotMatch(source, /knowledge\.importModeOverwriteDescription/)
    assert.doesNotMatch(source, /knowledge\.importModeAppendDescription/)
    assert.doesNotMatch(source, /knowledge\.importFileDescription/)
  }

  for (const localeSource of localeFiles) {
    const messages = JSON.parse(localeSource)
    assert.equal(Object.hasOwn(messages.dashboardHome, "description"), false)
    assert.equal(Object.hasOwn(messages.skillDefinition, "debugDescription"), false)
    assert.equal(Object.hasOwn(messages.knowledge, "batchMoveDescription"), false)
    assert.equal(Object.hasOwn(messages.knowledge, "importFAQDescription"), false)
    assert.equal(Object.hasOwn(messages.knowledge, "importModeOverwriteDescription"), false)
    assert.equal(Object.hasOwn(messages.knowledge, "importModeAppendDescription"), false)
    assert.equal(Object.hasOwn(messages.knowledge, "importFileDescription"), false)
  }
})

test("dashboard visible surfaces avoid robot and magic visual cues", async () => {
  const scannedFiles = [
    "channels/page.tsx",
    "channels/_components/edit.tsx",
    "ai-workflow-runs/_components/workspace.tsx",
    "conversations/_components/chat-panel.tsx",
    "conversations/_components/conversation-info-panel.tsx",
    "help/page.tsx",
    "_components/summary-cards.tsx",
    "ai-workflows/_components/workflow-editor.tsx",
    "knowledge/_components/debug-panel.tsx",
  ]
  for (const file of scannedFiles) {
    const source = await readFile(new URL(file, import.meta.url), "utf8")
    assert.doesNotMatch(source, /Bot(?:Icon|MessageSquareIcon)/, `${file} should not use robot visuals`)
    assert.doesNotMatch(source, /SparklesIcon/, `${file} should not use magic visuals`)
    assert.doesNotMatch(source, />Agent</, `${file} should not expose Agent as a label`)
    assert.doesNotMatch(source, /该 Agent/, `${file} should not expose Agent in toast copy`)
  }
})

test("dashboard homepage keeps service automation labels neutral", () => {
  for (const localeSource of localeFiles) {
    const messages = JSON.parse(localeSource)
    const values = [
      messages.dashboardHome.enabledAiAgents,
      messages.dashboardHome.todayAiHandoffCount,
      messages.dashboardHome.summaryAiServiceRate,
      messages.dashboardHome.statusAiServing,
      messages.conversation.filterAiServing,
      messages.conversation.aiServingNotice,
      messages.conversationMonitor.serviceAi,
      messages.conversationMonitor.serviceAiFirst,
    ]

    for (const value of values) {
      assert.equal(/AI|IA/.test(value), false, `${value} should avoid visible AI branding`)
    }
  }
})
