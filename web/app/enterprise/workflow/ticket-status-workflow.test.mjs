import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./ticket-status-workflow.tsx", import.meta.url), "utf8")

test("workflow loading keeps a stable localized copy dependency", () => {
  assert.match(source, /useMemo\(\(\) => ticketWorkflowCopy\(locale\), \[locale\]\)/)
  assert.match(source, /\}, \[copy, project\]\)/)
})
