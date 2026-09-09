import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./global-search.tsx", import.meta.url), "utf8")
const localeFiles = [
  "../messages/zh-CN.json",
  "../messages/en-US.json",
  "../messages/es-ES.json",
]

test("global search results avoid long description snippets", () => {
  assert.doesNotMatch(source, /result\.description/)
  assert.doesNotMatch(source, /globalSearch\.noResultsDesc/)
  assert.doesNotMatch(source, /Try a different search term|请尝试不同的搜索词|Prueba con otro término/)
  assert.match(source, /function formatSearchMetadata/)
  assert.match(source, /METADATA_KEYS/)
})

test("global search locale copy does not keep no-results descriptions", async () => {
  for (const file of localeFiles) {
    const messages = JSON.parse(await readFile(new URL(file, import.meta.url), "utf8"))
    assert.equal(Object.hasOwn(messages.globalSearch, "noResultsDesc"), false, `${file} should not keep noResultsDesc`)
  }
})
