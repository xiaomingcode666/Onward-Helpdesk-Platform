import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./loading-states.tsx", import.meta.url), "utf8")

test("shared loading states use design tokens", () => {
  assert.match(source, /border-border/)
  assert.match(source, /bg-card/)
  assert.match(source, /bg-muted/)
  assert.match(source, /text-muted-foreground/)
  assert.doesNotMatch(source, /border-slate|bg-white|text-slate|divide-slate/)
})
