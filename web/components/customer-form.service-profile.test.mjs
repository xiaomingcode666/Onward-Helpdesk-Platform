import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./customer-form.tsx", import.meta.url), "utf8")

test("customer form persists the selected service profile", () => {
  assert.match(source, /serviceProfileValueOptions = \["standard", "enhanced", "mission_critical"\]/)
  assert.match(source, /serviceProfile: "standard"/)
  assert.match(source, /serviceProfile: \(item\.serviceProfile \|\| "standard"\)/)
  assert.match(source, /serviceProfile: values\.serviceProfile/)
  assert.match(source, /t\("customerForm\.serviceProfile"\)/)
})
