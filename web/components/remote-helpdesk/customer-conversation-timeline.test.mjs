import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./customer-conversation-timeline.tsx", import.meta.url), "utf8")

test("customer timeline accepts optional message translation controls", () => {
  assert.match(source, /translatedMessages\?: Record<number, ConversationMessageTranslationDTO>/)
  assert.match(source, /translatingMessageId\?: number \| null/)
  assert.match(source, /onTranslateMessage\?: \(message: ImMessage\) => Promise<void> \| void/)
  assert.match(source, /data-testid="customer-translate-message-button"/)
  assert.match(source, /data-testid="customer-message-translation"/)
})
