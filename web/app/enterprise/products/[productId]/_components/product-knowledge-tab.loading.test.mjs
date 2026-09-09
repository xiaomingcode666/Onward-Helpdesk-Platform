import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./product-knowledge-tab.tsx", import.meta.url), "utf8")

test("product knowledge tab keeps the module structure while data is loading", () => {
  assert.doesNotMatch(source, /if \(!data\) return/)
  assert.match(source, /function KnowledgeMetricLoading\(\)/)
  assert.match(source, /function KnowledgeSectionLoading\(\{ title, columns \}/)
  assert.match(source, /<KnowledgeMetricLoading \/>/)
  assert.match(source, /<KnowledgeSectionLoading title=\{t\("productArchive\.knowledgeBindings"\)\} columns=\{5\} \/>/)
  assert.match(source, /<KnowledgeSectionLoading title=\{t\("productArchive\.knowledgeDocuments"\)\} columns=\{4\} \/>/)
})

test("product knowledge tab preserves existing data when a refresh fails", () => {
  assert.doesNotMatch(source, /if \(error && !data\) return <ErrorState/)
  assert.match(source, /const blockingError = Boolean\(error && !data\)/)
  assert.match(source, /\{blockingError \? \(/)
  assert.match(source, /<ErrorState\s+title=\{t\("productArchive\.loadFailed"\)\}/)
  assert.match(source, /border-destructive\/20 bg-destructive\/5/)
  assert.doesNotMatch(source, /if \(error\) return <ErrorState/)
})
