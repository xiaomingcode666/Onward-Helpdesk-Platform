import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const messages = JSON.parse(
  await readFile(new URL("../../messages/zh-CN.json", import.meta.url), "utf8"),
)

const ticketsSource = await readFile(new URL("./tickets/page.tsx", import.meta.url), "utf8")
const ticketWorkbenchSource = await readFile(new URL("./ticket-workbench/page.tsx", import.meta.url), "utf8")
const meetingCenterSource = await readFile(
  new URL("../../components/meeting/enterprise-meeting-center.tsx", import.meta.url),
  "utf8",
)
const legacyLiveSource = await readFile(
  new URL("../../components/remote-helpdesk/enterprise-live-pages.tsx", import.meta.url),
  "utf8",
)
const remoteNavigationSource = await readFile(
  new URL("../../lib/navigation-remote-helpdesk.tsx", import.meta.url),
  "utf8",
)
const dashboardNavigationSource = await readFile(
  new URL("../../lib/navigation.tsx", import.meta.url),
  "utf8",
)

test("enterprise menu copy uses short product labels", () => {
  assert.equal(messages.remoteNav.enterprise.workbench, "运营总览")
  assert.equal(messages.remoteNav.enterprise["ticket-workbench"], "处理工作台")
  assert.equal(messages.remoteNav.enterprise.diagnosis, "诊断")
  assert.equal(messages.remoteNav.enterprise.models, "模型服务配置")
  assert.equal(messages.remoteNav.enterprise.ai, "AI 接待机器人")
  assert.equal(messages.remoteSidebar.aiData, "模型与数据")
  assert.equal(messages.remoteSidebar.workbench, "服务运营")
  assert.equal(messages.nav.workbench, "服务处理")
  assert.equal(messages.nav.meetings, "视频协同")
  assert.equal(messages.enterpriseWorkbench.page.title, "运营总览")
  assert.equal(messages.conversation.workbenchTitle, "会话处理")
  assert.equal(messages.remoteNav.partner.overview, "协作概览")
  assert.equal(messages.remoteNav.partner.conversations, "会话处理")
  assert.equal(messages.auth.portals.partner.title, "供应商协作端")
  assert.equal(messages.workspace.workbench, "服务处理")
  assert.notEqual(messages.remoteNav.enterprise.diagnosis, "AI 诊断")
  assert.notEqual(messages.remoteNav.enterprise.models, "AI 服务配置")
  assert.notEqual(messages.remoteNav.enterprise.ai, "接待机器人")
  assert.notEqual(messages.remoteSidebar.aiData, "AI 与数据")
  assert.doesNotMatch(remoteNavigationSource, /BotIcon/)
  assert.doesNotMatch(dashboardNavigationSource, /BotIcon/)
  assert.doesNotMatch(JSON.stringify(messages), /供应商工作台|客服工作台|工单工作台|服务工作台|会话工作台/)
})

test("enterprise high frequency pages avoid legacy page titles", () => {
  const aftersalesCommandPattern = new RegExp("售后工单" + "指挥台")
  const conversationWorkbenchTitlePattern = new RegExp('title="会话' + '工作台"')
  const videoMeetingTitlePattern = new RegExp('title="视频' + '会议"')
  const serviceWorkbenchPattern = new RegExp('title="服务' + '工作台"|加载服务' + '工作台失败')

  assert.match(ticketsSource, /title="工单中心"/)
  assert.match(ticketWorkbenchSource, /title="处理工作台"/)
  assert.match(meetingCenterSource, /title="视频协同"/)
  assert.doesNotMatch(ticketsSource, aftersalesCommandPattern)
  assert.doesNotMatch(ticketWorkbenchSource, conversationWorkbenchTitlePattern)
  assert.doesNotMatch(ticketWorkbenchSource, /title="工单处理"/)
  assert.doesNotMatch(meetingCenterSource, videoMeetingTitlePattern)
  assert.doesNotMatch(meetingCenterSource, /DialogDescription/)
  assert.doesNotMatch(legacyLiveSource, serviceWorkbenchPattern)
})
