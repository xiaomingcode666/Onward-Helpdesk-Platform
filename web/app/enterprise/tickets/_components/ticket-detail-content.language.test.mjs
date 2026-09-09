import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./ticket-detail-content.tsx", import.meta.url), "utf8")

test("ticket detail tabs are translated during render", () => {
  assert.match(source, /function buildDetailTabs\(\): RailopsTabItem\[\]/)
  assert.match(source, /const detailTabs = buildDetailTabs\(\)/)
  assert.doesNotMatch(source, /const detailTabs: RailopsTabItem\[\] =/)
})
