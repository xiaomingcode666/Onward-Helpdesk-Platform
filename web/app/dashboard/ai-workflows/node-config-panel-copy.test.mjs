import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./_components/node-config-panel.tsx", import.meta.url), "utf8")

test("workflow node config keeps reception wording and terse helper copy", () => {
  assert.match(source, /跟随接待配置服务模式/)
  assert.match(source, /接待配置默认知识/)
  assert.doesNotMatch(source, /跟随机器人服务模式/)
  assert.doesNotMatch(source, /机器人默认知识/)
  assert.doesNotMatch(source, /服务码和已绑定设备会自动优先使用/)
})
