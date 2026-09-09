import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./history-list.tsx", import.meta.url), "utf8")

test("customer service history avoids robot visual cues", () => {
  assert.doesNotMatch(source, /BotIcon/)
  assert.match(source, /MessageSquareIcon/)
})

test("customer service history keeps empty state terse", () => {
  assert.doesNotMatch(source, /history_list\.empty_desc/)
  assert.doesNotMatch(source, /开始新的咨询后将显示在此处/)
  assert.doesNotMatch(source, /您还没有创建过服务工单/)
  assert.match(source, /暂无记录/)
})
