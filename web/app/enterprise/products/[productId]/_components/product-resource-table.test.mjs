import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./product-resource-table.tsx", import.meta.url), "utf8")

test("product resource table keeps toolbar visible during loading", () => {
  assert.match(source, /const \[itemsLoaded, setItemsLoaded\] = useState\(false\)/)
  assert.match(source, /setItemsLoaded\(true\)/)
  assert.match(source, /const initialLoading = loading && !itemsLoaded/)
  assert.match(source, /disabled=\{loading\}/)
  assert.doesNotMatch(source, /if \(loading\)\s*\{/)
  assert.doesNotMatch(source, /if \(error\)\s*\{/)
})
