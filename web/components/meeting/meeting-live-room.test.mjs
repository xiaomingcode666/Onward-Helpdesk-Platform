import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./meeting-live-room.tsx", import.meta.url), "utf8")

test("meeting live room automatically starts ready transcription after join", () => {
  assert.match(source, /autoTranscriptionRequestedRef/)
  assert.match(source, /apiRef\.current\.executeCommand\("setSubtitles", true, false, transcriptionLanguage\)/)
  assert.match(source, /setShowTranscriptPanel\(true\)/)
  assert.match(source, /实时字幕启动超时，请检查 Jigasi 和转写服务配置/)
})

test("meeting live room explains muted microphone transcription state", () => {
  assert.match(source, /const transcriptionNeedsAudio =/)
  assert.match(source, /当前麦克风静音/)
  assert.match(source, /打开麦克风后才会生成文字记录/)
})
