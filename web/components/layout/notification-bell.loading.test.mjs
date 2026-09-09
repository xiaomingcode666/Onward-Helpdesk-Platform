import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./notification-bell.tsx", import.meta.url), "utf8")

test("platform notification bell loads alerts from the events slice api", () => {
  assert.match(source, /fetchPlatformOverviewEvents/)
  assert.match(source, /fetchPlatformOverviewEvents\(8\)/)
  assert.doesNotMatch(source, /fetchPlatformOverview\(/)
  assert.doesNotMatch(source, /\/api\/platform\/overview["'`]/)
})
