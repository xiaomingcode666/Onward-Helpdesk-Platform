import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const centerSource = await readFile(new URL("./enterprise-meeting-center.tsx", import.meta.url), "utf8")
const liveRoomSource = await readFile(new URL("./meeting-live-room.tsx", import.meta.url), "utf8")
const emptyStateSource = await readFile(new URL("../shared/error-states.tsx", import.meta.url), "utf8")

test("meeting pages avoid leftover explanatory empty-state copy", () => {
  assert.doesNotMatch(centerSource, /BotIcon/)
  assert.doesNotMatch(centerSource, /没有找到匹配的会议记录/)
  assert.doesNotMatch(centerSource, /该设备关联工单还没有产生会议记录/)
  assert.doesNotMatch(centerSource, /从工单详情发起视频协作后/)
  assert.doesNotMatch(centerSource, /本场会议没有归档/)
  assert.doesNotMatch(centerSource, /无补充说明/)
  assert.doesNotMatch(centerSource, /当前没有协作/)
  assert.doesNotMatch(centerSource, /当前没有可发起视频的工单/)
  assert.doesNotMatch(centerSource, /工单完成受理并分配给你后会显示在这里/)
  assert.doesNotMatch(centerSource, /全部会议|立即会议|预定会议|进入会议|开始会议/)
  assert.doesNotMatch(centerSource, /会议加载失败|会议记录加载失败|部分会议记录加载失败/)
  assert.doesNotMatch(centerSource, /会议概述|会议时长|会议资料|会议状态|会议冻结帧/)
  assert.match(centerSource, /全部协作/)
  assert.match(centerSource, /协作记录/)
  assert.match(centerSource, /协作概述/)
  assert.match(centerSource, /暂无协作/)
  assert.match(centerSource, /暂无可发起协作的工单/)

  assert.doesNotMatch(liveRoomSource, /DialogDescription/)
  assert.doesNotMatch(liveRoomSource, /会议结束后，当前参会者将离开房间/)
  assert.doesNotMatch(liveRoomSource, /准备加入会议|进入会议|正在进入会议|参会身份|入会设置/)
  assert.match(liveRoomSource, /准备加入协作/)
  assert.match(liveRoomSource, /进入协作/)

  assert.doesNotMatch(emptyStateSource, /description=\{description \|\| t\("errorStates\.emptyDescription"\)\}/)
  assert.doesNotMatch(emptyStateSource, /description=\{description \|\| t\("errorStates\.(loadFailed|forbidden|quotaExceeded|connectorDown|videoPermissionDenied)Description"\)\}/)
  assert.match(emptyStateSource, /description=\{description\}/)
})
