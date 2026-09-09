import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const lobbySource = await readFile(new URL("./meeting-lobby.tsx", import.meta.url), "utf8")
const liveRoomSource = await readFile(new URL("./meeting-live-room.tsx", import.meta.url), "utf8")

test("camera and microphone checks fail independently", () => {
  assert.match(lobbySource, /Promise\.allSettled\(\[/)
  assert.match(lobbySource, /getUserMedia\(\{ video: true, audio: false \}\)/)
  assert.match(lobbySource, /getUserMedia\(\{ video: false, audio: true \}\)/)
  assert.match(lobbySource, /setCameraIssue\(nextCameraIssue\)/)
  assert.match(lobbySource, /setMicIssue\(nextMicIssue\)/)
})

test("missing media devices never become an admission requirement", () => {
  assert.doesNotMatch(lobbySource, /CardDescription/)
  assert.doesNotMatch(lobbySource, /检查正在打开此页面的手机或电脑/)
  assert.doesNotMatch(lobbySource, /与会议发起人的设备无关/)
  assert.match(lobbySource, /disabled=\{isJoining\}/)
  assert.doesNotMatch(lobbySource, /joinDisabled/)
  assert.match(lobbySource, /这不是会议故障，也不会阻止入会/)
  assert.match(lobbySource, /加入协作（音视频默认关闭）/)
})

test("media errors explain the current customer device and recovery action", () => {
  assert.match(lobbySource, /kind: "permission-denied"/)
  assert.match(lobbySource, /kind: "not-found"/)
  assert.match(lobbySource, /kind: "in-use"/)
  assert.match(lobbySource, /当前设备未检测到摄像头/)
  assert.match(lobbySource, /请在浏览器地址栏的网站权限中允许摄像头或麦克风/)
  assert.match(lobbySource, /请确认设备已连接并在系统设置中启用/)
})

test("Jitsi joins muted without acquiring media until the user enables it", () => {
  assert.match(liveRoomSource, /startWithAudioMuted: true/)
  assert.match(liveRoomSource, /startWithVideoMuted: true/)
  assert.match(liveRoomSource, /disableInitialGUM: true/)
})
