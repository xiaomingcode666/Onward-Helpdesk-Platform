import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./page-header.tsx", import.meta.url), "utf8")
const pageHeaderConsumers = await Promise.all(
  [
    "../../app/enterprise/workflow/page.tsx",
    "../../app/enterprise/org/schedules/page.tsx",
    "../../app/enterprise/ai/ai-service-center.tsx",
    "../../app/enterprise/ticket-workbench/page.tsx",
  ].map(async (file) => [file, await readFile(new URL(file, import.meta.url), "utf8")]),
)

test("page header keeps route breadcrumbs as the visible title and avoids explanatory descriptions", () => {
  assert.match(source, /<RouteBreadcrumbs size="page" \/>/)
  assert.match(source, /<h1 className="sr-only">\{title\}<\/h1>/)
  assert.doesNotMatch(source, /HeaderCrumbs/)
  assert.doesNotMatch(source, /breadcrumbs\?:/)
  assert.doesNotMatch(source, /eyebrow\?:/)
  assert.doesNotMatch(source, /titlePrefix\?:/)
  assert.doesNotMatch(source, /icon\?:/)
  assert.doesNotMatch(source, /description\?: ReactNode/)
  assert.doesNotMatch(source, /description\?: string/)
  assert.doesNotMatch(source, /!usesRouteBreadcrumb && description/)
  assert.doesNotMatch(source, /max-w-3xl text-sm leading-5 text-muted-foreground/)

  for (const [file, consumerSource] of pageHeaderConsumers) {
    assert.doesNotMatch(consumerSource, /<PageHeader[\s\S]{0,240}\bicon=/, `${file} should not pass ignored PageHeader icons`)
  }
})
