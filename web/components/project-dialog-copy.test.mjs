import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./project-dialog.tsx", import.meta.url), "utf8")

test("project dialog keeps form headers concise", () => {
  assert.doesNotMatch(source, /DialogDescription/)
  assert.doesNotMatch(source, /<DialogDescription>/)
})
