import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./project-runtime-fields.tsx", import.meta.url), "utf8")

test("selecting a profile copies times from the stored target configuration", () => {
  assert.match(source, /r\.targets\.find\(\(item, j\) => j !== i && item\.profile === profile\)/)
  assert.match(source, /response_minutes: preset\.response_minutes/)
  assert.match(source, /assignment_minutes: preset\.assignment_minutes/)
  assert.match(source, /resolution_minutes: preset\.resolution_minutes/)
})
