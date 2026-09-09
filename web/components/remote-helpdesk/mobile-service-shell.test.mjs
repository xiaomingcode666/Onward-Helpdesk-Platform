import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./mobile-service-shell.tsx", import.meta.url), "utf8")

test("mobile service shell uses a shell-like initial loading surface", () => {
  assert.match(source, /import \{ ModuleLoading \} from "@\/components\/shared\/loading-states"/)
  assert.match(source, /const hasEntryState = Boolean\(state\)/)
  assert.match(source, /const initialLoading = loading && !hasEntryState/)
  assert.match(source, /initialLoading \? <LoadingState \/>/)
  assert.doesNotMatch(source, /loading \? <LoadingState \/>/)
  assert.doesNotMatch(source, /loading && !state/)
  assert.doesNotMatch(source, /!loading && state\?\.kind/)
  assert.match(source, /function LoadingState\(\)[\s\S]*aria-label="服务入口加载中"/)
  assert.match(source, /Skeleton className="mt-3 h-20 w-full"/)
  assert.match(source, /Skeleton className="h-28 rounded-lg"/)
})

test("mobile service shell keeps customer copy terse", () => {
  assert.doesNotMatch(source, /BotIcon/)
  for (const text of [
    "正在读取产品、设备和服务状态",
    "正在建立服务会话",
    "正在连接产品客服",
    "开始 AI 咨询",
    "AI 诊断",
    "产品服务上下文已加载",
    "系统已识别",
    "未登录用户只能识别设备",
    "确认设备后开始服务，会话和工单将关联到这台设备。",
    "该服务码已停用，请扫描设备上的新二维码或联系服务人员。",
    "请检查二维码，或联系服务人员获取新的服务入口。",
    "该设备的服务会话已就绪",
    "将相机对准设备上的二维码，开始售后服务。",
    "功能正在建设中",
    "当前服务入口还没有工单记录",
    "当前服务入口尚未关联远程会议",
    "补充本次服务反馈，例如响应速度、处理效果或建议",
    "问题未解决，重新打开工单",
  ]) {
    assert.doesNotMatch(source, new RegExp(text))
  }
})
