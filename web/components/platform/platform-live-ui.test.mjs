import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./platform-live-ui.tsx", import.meta.url), "utf8")

test("platform shared cards use design tokens", () => {
  assert.match(source, /border-border/)
  assert.match(source, /bg-card/)
  assert.match(source, /text-muted-foreground/)
  assert.match(source, /text-foreground/)
  assert.doesNotMatch(source, /border-slate-200 bg-white|text-slate-950|text-slate-500/)
})
