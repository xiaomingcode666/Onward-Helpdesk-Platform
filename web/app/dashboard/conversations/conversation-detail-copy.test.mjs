import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const chatPanel = await readFile(new URL("./_components/chat-panel.tsx", import.meta.url), "utf8")
const infoPanel = await readFile(new URL("./_components/conversation-info-panel.tsx", import.meta.url), "utf8")
const locales = ["zh-CN", "en-US", "es-ES"]
const messagesByLocale = await Promise.all(
  locales.map(async (locale) => [
    locale,
    JSON.parse(await readFile(new URL(`../../../messages/${locale}.json`, import.meta.url), "utf8")),
  ]),
)

test("workflow run detail dialogs avoid redundant header descriptions", () => {
  for (const source of [chatPanel, infoPanel]) {
    assert.match(source, /workflowRunDetail\.labels\.title/)
    assert.doesNotMatch(source, /description=\{run \? `Run #\$\{run\.id\}` : "Workflow 执行链路"\}/)
    assert.doesNotMatch(source, /Workflow 执行链路/)
  }
})

test("image forwarding dialog avoids redundant header description copy", () => {
  assert.doesNotMatch(chatPanel, /DialogDescription/)
  assert.doesNotMatch(chatPanel, /forwardImageDescription/)
  for (const [locale, messages] of messagesByLocale) {
    assert.equal(Object.hasOwn(messages.conversation, "forwardImageDescription"), false, `${locale} should not keep conversation.forwardImageDescription`)
  }
})
