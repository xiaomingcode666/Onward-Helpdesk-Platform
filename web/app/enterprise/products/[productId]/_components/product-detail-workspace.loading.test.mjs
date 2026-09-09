import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./product-detail-workspace.tsx", import.meta.url), "utf8")

test("product detail renders the workspace shell before the profile API resolves", () => {
  assert.doesNotMatch(source, /if \(!ready \|\| !session \|\| !profile\) return/)
  assert.doesNotMatch(source, /if \(error && !profile\) return <ErrorState/)
  assert.match(source, /const profileError = Boolean\(error && !profile\)/)
  assert.match(source, /const profilePending = !profileError && \(!ready \|\| !session \|\| !profile\)/)
  assert.match(source, /<ErrorState\s+title=\{t\("productArchive\.loadFailed"\)\}/)
  assert.match(source, /RouteBreadcrumbs/)
  assert.match(source, /<RouteBreadcrumbs size="page" \/>/)
  assert.match(source, /<header className="border-b border-border bg-card px-4 py-4 lg:px-6">/)
  assert.match(source, /showTabPlaceholders/)
  assert.match(source, /<ProductDetailWorkspaceLoading \/>/)
  assert.match(source, /function ProductDetailWorkspaceLoading\(\)/)
  assert.match(source, /aria-label=\{t\("enterpriseExtract\.products\.archiveLoadingAria"\)\}/)
  assert.doesNotMatch(source, /<h1 className="truncate text-xl font-semibold text-foreground">\{product\.name\}<\/h1>/)
})
