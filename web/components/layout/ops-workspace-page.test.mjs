import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./ops-workspace-page.tsx", import.meta.url), "utf8")

test("ops workspace page uses semantic tokens for cards and controls", () => {
  assert.match(source, /border-primary\/20 bg-primary\/10 text-primary/)
  assert.match(source, /border-border bg-card text-foreground/)
  assert.match(source, /border-border bg-muted text-muted-foreground/)
  assert.match(source, /focus:border-ring focus:ring-2 focus:ring-ring\/20/)
  assert.doesNotMatch(source, /border-slate-200 bg-white text-slate-700/)
  assert.doesNotMatch(source, /border-blue-200 bg-blue-50 text-blue-700/)
})
