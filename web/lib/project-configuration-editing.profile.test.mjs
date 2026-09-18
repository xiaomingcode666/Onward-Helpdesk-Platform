import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const editingSource = await readFile(new URL("./project-configuration-editing.ts", import.meta.url), "utf8")
const runtimeExample = JSON.parse(await readFile(new URL("../../config/project-configuration.runtime.example.json", import.meta.url), "utf8"))

test("service target minutes are configuration data, not product defaults", () => {
  assert.doesNotMatch(editingSource, /SERVICE_SLA_DEFAULTS/)
  assert.doesNotMatch(editingSource, /response_minutes: 15/)
  const profiles = new Set(runtimeExample.runtime.targets.map((target) => target.profile))
  assert.deepEqual([...profiles].sort(), ["enhanced", "mission_critical", "standard"])
})
