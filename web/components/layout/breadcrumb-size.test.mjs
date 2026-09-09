import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const routeBreadcrumbsSource = await readFile(new URL("./route-breadcrumbs.tsx", import.meta.url), "utf8")
const pageHeaderSource = await readFile(new URL("./page-header.tsx", import.meta.url), "utf8")
const headerCrumbsSource = await readFile(new URL("./header-crumbs.tsx", import.meta.url), "utf8")

test("route breadcrumbs use readable sizes across shell and page headers", () => {
  assert.match(routeBreadcrumbsSource, /size = "sm"/)
  assert.doesNotMatch(routeBreadcrumbsSource, /"xs"/)

  assert.match(pageHeaderSource, /<RouteBreadcrumbs size="page" \/>/)
  assert.doesNotMatch(pageHeaderSource, /HeaderCrumbs/)
  assert.match(headerCrumbsSource, /size === "page" \? "text-2xl leading-8" : "text-base leading-6"/)
  assert.match(headerCrumbsSource, /flex-nowrap/)
  assert.match(headerCrumbsSource, /whitespace-nowrap/)
  assert.match(headerCrumbsSource, /visibleSegments = segments\.length > 2/)
  assert.match(headerCrumbsSource, /"block truncate"/)
  assert.doesNotMatch(headerCrumbsSource, /"xs"/)
  assert.doesNotMatch(headerCrumbsSource, /flex min-w-0 flex-wrap items-center/)
})
