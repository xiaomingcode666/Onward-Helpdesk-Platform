import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import vm from "node:vm"
import ts from "typescript"
import MarkdownIt from "markdown-it"

function compile(source, dependencies = {}, globals = {}) {
  const compiled = ts.transpileModule(source, {
    compilerOptions: { target: ts.ScriptTarget.ES2017, module: ts.ModuleKind.CommonJS, esModuleInterop: true },
  })
  const sandbox = { ...globals, exports: {}, require: (name) => {
    assert.ok(name in dependencies, `Unexpected import: ${name}`)
    return dependencies[name]
  } }
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.exports
}

const config = compile(await readFile(new URL("../i18n/config.ts", import.meta.url), "utf8"))
const groups = await Promise.all(["portal", "customer-entry"].map(async (name) => {
  const catalogModule = compile(await readFile(new URL(`../i18n/extracted/${name}.ts`, import.meta.url), "utf8"))
  return Object.values(catalogModule)[0]
}))
const catalogs = Object.fromEntries(await Promise.all(config.SUPPORTED_LOCALES.map(async (locale) => [
  `@/messages/${locale}.json`, JSON.parse(await readFile(new URL(`../messages/${locale}.json`, import.meta.url), "utf8")),
])))
const messages = compile(await readFile(new URL("../i18n/messages.ts", import.meta.url), "utf8"), {
  "@/i18n/config": config,
  "@/i18n/extracted": { getExtractedMessage: (locale, key) => {
    for (const group of groups) {
      const value = key.split(".").reduce((node, part) => node?.[part], group[locale])
      if (value !== undefined) return value
    }
  } },
  ...catalogs,
})
const summaryModule = compile(await readFile(new URL("./customer-conversation-summary-i18n.ts", import.meta.url), "utf8"), {
  "@/i18n/config": config,
  "@/i18n/messages": messages,
})
const { renderCustomerConversationSummary, localizeConversationSummary, getConversationSystemMessageText } = summaryModule
const summaryKey = (name) => `portalExtract.customerMobile.conversation.summaryEvents.${name}`
const eventKey = (name) => `customerEntryExtract.event.${name}`
const t = (locale) => (key, values) => messages.translateMessage(locale, key, values)

test("typed system summaries switch languages without inspecting stored content", () => {
  const text = { key: "conversationSystem.ticketCreatedSummary", params: { ticketNo: "TK42" } }
  assert.equal(renderCustomerConversationSummary("unrelated old snapshot", t("en-US"), "", text), "Service ticket TK42 created")
  assert.equal(renderCustomerConversationSummary("unrelated old snapshot", t("zh-CN"), "", text), "已生成服务工单 TK42")
  const message = { senderType: "system", messageType: "text", payload: JSON.stringify({ systemText: { summary: text } }) }
  assert.equal(getConversationSystemMessageText(message, "summary")?.key, text.key)
  assert.equal(getConversationSystemMessageText({ ...message, senderType: "customer" }, "summary"), undefined)
  assert.equal(getConversationSystemMessageText({ ...message, recalledAt: "2026-10-20" }, "summary"), undefined)
  assert.equal(getConversationSystemMessageText({ ...message, messageType: "image" }, "summary"), undefined)
  assert.equal(getConversationSystemMessageText({ ...message, payload: "{}" }, "summary"), undefined)
})

test("real-time system messages retain keys for language switching and clear them for the next customer message", async () => {
  let locale = "en-US"
  const im = compile(await readFile(new URL("./im-message.ts", import.meta.url), "utf8"), {
    "markdown-it": MarkdownIt,
    "@/i18n/messages": { ...messages, translateCurrentMessage: (key, values) => messages.translateMessage(locale, key, values) },
    "@/lib/customer-conversation-summary-i18n": summaryModule,
  })
  const realtime = compile(await readFile(new URL("./im-realtime-state.ts", import.meta.url), "utf8"), {
    "@/lib/im-message": im,
    "@/lib/im-message-merge": {},
  })
  const summary = { key: "conversationSystem.ticketCreatedSummary", params: { ticketNo: "TK42" } }
  const content = { ...summary, key: "conversationSystem.ticketCreated" }
  const message = { id: 1, conversationId: 42, senderType: "system", messageType: "text", content: "unrelated old snapshot", payload: JSON.stringify({ systemText: { content, summary } }) }
  const conversation = realtime.patchConversationWithMessage({ id: 42 }, message)
  assert.equal(conversation.lastMessageSummary, "Service ticket TK42 created")
  assert.equal(im.renderIMMessageHTML(message), new MarkdownIt().render(t(locale)(content.key, content.params)))
  locale = "zh-CN"
  assert.equal(renderCustomerConversationSummary(conversation.lastMessageSummary, t(locale), "", conversation.lastMessageSummaryText), "已生成服务工单 TK42")
  assert.equal(im.summarizeIMMessage(message), "已生成服务工单 TK42")
  assert.equal(im.renderIMMessageHTML(message), new MarkdownIt().render(t(locale)(content.key, content.params)))
  const next = realtime.patchConversationWithMessage(conversation, { ...message, id: 2, senderType: "customer", content: "保留客户原文" })
  assert.equal(next.lastMessageSummaryText, null)
  assert.equal(next.lastMessageSummary, "保留客户原文")
  assert.equal(getConversationSystemMessageText({ ...message, sendStatus: 6 }, "content"), undefined)
})

test("screenshot summaries use the selected language and keep company names and ticket numbers", () => {
  const cases = [
    ["已邀请供应商协作：Onward_Rag", "Supplier collaboration invited: Onward_Rag"],
    ["已生成服务工单 TK2026101400001，会话摘要和已上传资料已同步至工单。", "Service ticket TK2026101400001 created"],
  ]
  for (const [input, expected] of cases) {
    assert.equal(renderCustomerConversationSummary(input, t("en-US")), expected)
  }
  assert.equal(renderCustomerConversationSummary(cases[0][1], t("zh-CN")), cases[0][0])
  assert.equal(renderCustomerConversationSummary("维修结论已提交，等待客户确认服务结果。", t("en-US")), t("en-US")(summaryKey("repairSubmittedWaiting")))
})

test("stored system templates switch in both directions for every supported locale", () => {
  const cases = [
    [eventKey("ticketCreatedDescriptionWithTicketNoGeneral"), summaryKey("ticketCreatedWithTicketNo"), { ticketNo: "TK2026101400001" }],
    [eventKey("ticketCreatedDescriptionWithTicketNo"), summaryKey("ticketCreatedWithTicketNo"), { ticketNo: "TK2026101600005" }],
    [eventKey("ticketCreatedDescriptionWithTicketNoGeneral"), summaryKey("ticketCreatedWithTicketNo"), { ticketNo: "#42" }],
    [summaryKey("supplierInvitedWithModule"), summaryKey("supplierInvitedWithModule"), { company: "Onward_Rag", module: "模块 A" }],
    [eventKey("supplierJoinedDescriptionWithCompany"), summaryKey("supplierJoinedWithCompany"), { company: "Onward_Rag" }],
    [eventKey("feedbackSubmittedDescriptionWithComment"), eventKey("feedbackSubmittedDescriptionWithComment"), { rating: 5, comment: "服务很好" }],
    [summaryKey("repairSubmittedWithSummary"), summaryKey("repairSubmittedWithSummary"), { summary: "保留工程师原文" }],
  ]
  for (const [sourceKey, targetKey, values] of cases) {
    for (const sourceLocale of config.SUPPORTED_LOCALES) {
      const stored = t(sourceLocale)(sourceKey, values)
      for (const targetLocale of config.SUPPORTED_LOCALES) {
        assert.equal(renderCustomerConversationSummary(stored, t(targetLocale)), t(targetLocale)(targetKey, values), `${sourceKey}: ${sourceLocale} -> ${targetLocale}`)
      }
    }
  }
})

test("media labels and video durations switch language without translating filenames or room names", () => {
  assert.equal(renderCustomerConversationSummary("[Image] 现场照片.png", t("zh-CN")), "[图片] 现场照片.png")
  const values = { roomName: "Room A", duration: t("en-US")(eventKey("durationMinutesSeconds"), { minutes: 2, seconds: 30 }) }
  const input = t("en-US")(summaryKey("videoEndedWithRoomDuration"), values)
  const expected = t("zh-CN")(summaryKey("videoEndedWithRoomDuration"), {
    ...values, duration: t("zh-CN")("portalExtract.customerMobile.time.minutesSeconds", { minutes: 2, seconds: 30 }),
  })
  assert.equal(renderCustomerConversationSummary(input, t("zh-CN")), expected)
  assert.equal(renderCustomerConversationSummary("视频协作已结束，时长：2分30秒", t("en-US")), t("en-US")(summaryKey("videoEndedWithDuration"), {
    duration: t("en-US")("portalExtract.customerMobile.time.minutesSeconds", { minutes: 2, seconds: 30 }),
  }))
})

test("custom messages, custom additions and empty summaries retain their content", () => {
  for (const value of ["请帮我检查设备", "已生成服务工单 TK2026101400001 是什么意思？", "Custom summary", "  原始消息  "]) {
    assert.equal(renderCustomerConversationSummary(value, t("en-US")), value)
  }
  assert.equal(renderCustomerConversationSummary(undefined, t("en-US"), "No messages"), "No messages")
})

test("workbench queue cards display localized summaries and keep the source record intact", async () => {
  const source = await readFile(new URL("../app/enterprise/ticket-workbench/page.tsx", import.meta.url), "utf8")
  const ast = ts.createSourceFile("page.tsx", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX)
  const functions = ast.statements.filter((node) => ts.isFunctionDeclaration(node) && ["toAgentConversation", "buildConversationQueueItem"].includes(node.name?.text))
  assert.equal(functions.length, 2)
  const builder = compile(`${functions.map((node) => node.getText(ast)).join("\n")}\nexports.build = buildConversationQueueItem`, {}, {
    localizeConversationSummary,
    statusMeta: () => ({ label: "Completed", tone: "neutral", isDone: true }),
    conversationDisplayName: (item) => item.customerName,
    conversationPriorityLabel: () => "P4",
    formatShortDateTime: (value) => value,
    ticketDispatchAgingLabel: () => "",
    ee: (key) => key,
  }).build
  const stored = { id: 57, customer_name: "client", last_message_summary: "已邀请供应商协作：Onward_Rag", status: 4 }
  const english = builder(stored, undefined, "en-US")
  assert.equal(english.title, "Supplier collaboration invited: Onward_Rag")
  assert.equal(english.summary, english.title)
  const chinese = builder(stored, undefined, "zh-CN")
  assert.equal(chinese.title, stored.last_message_summary)
  assert.equal(english.conversation.lastMessageSummary, stored.last_message_summary)
  assert.equal(stored.last_message_summary, "已邀请供应商协作：Onward_Rag")
  const typed = { ...stored, last_message_summary: "snapshot without a recognizable template", last_message_summary_text: { key: "conversationSystem.ticketCreatedSummary", params: { ticketNo: "TK42" } } }
  assert.equal(builder(typed, undefined, "en-US").summary, "Service ticket TK42 created")
  assert.equal(builder(typed, undefined, "zh-CN").summary, "已生成服务工单 TK42")
})
