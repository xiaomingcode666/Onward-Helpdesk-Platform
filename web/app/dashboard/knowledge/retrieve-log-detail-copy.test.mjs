import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./_components/retrieve-log-detail.tsx", import.meta.url), "utf8")

test("knowledge retrieve detail drawer avoids header description component", () => {
  assert.doesNotMatch(source, /DrawerDescription/)
  assert.match(source, /detail\?\.log\.question/)
})
