import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./conversation-service-event.tsx", import.meta.url), "utf8")

test("conversation service events avoid robot visual cues", () => {
  assert.doesNotMatch(source, /BotIcon/)
  assert.match(source, /icon: MessageSquareTextIcon/)
})
