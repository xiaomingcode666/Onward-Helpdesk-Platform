import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import { test } from "node:test"

const source = await readFile(new URL("./product-overview-tab.tsx", import.meta.url), "utf8")

test("product overview tab translates owner and status labels", () => {
  assert.match(source, /useI18n\(\)/)
  assert.match(source, /productArchive\.fields\.owner/)
  assert.match(source, /productArchive\.status\.active/)
  assert.match(source, /productArchive\.status\.discontinued/)
  assert.doesNotMatch(source, /BotIcon/)
  assert.match(source, /BookOpenIcon/)
  assert.doesNotMatch(source, /产品负责人/)
})
