import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const workspace = await readFile(new URL("./_components/workspace.tsx", import.meta.url), "utf8")
const messages = await Promise.all(
  ["zh-CN", "en-US", "es-ES"].map(async (locale) => [
    locale,
    JSON.parse(await readFile(new URL(`../../../messages/${locale}.json`, import.meta.url), "utf8")),
  ]),
)

test("workflow run detail dialog avoids redundant header descriptions", () => {
  assert.match(workspace, /workflowRun\.detailTitle/)
  assert.doesNotMatch(workspace, /description=\{run \? `Run #\$\{run\.id\}` : t\("workflowRun\.detailDescription"\)\}/)
  for (const [locale, catalog] of messages) {
    assert.equal(Object.hasOwn(catalog.workflowRun, "detailDescription"), false, `${locale} should not keep workflowRun.detailDescription`)
  }
})
