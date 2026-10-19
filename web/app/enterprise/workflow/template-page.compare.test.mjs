import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./[workflowId]/template-page.tsx", import.meta.url), "utf8")

test("version compare actions reveal and select the comparison panel", () => {
  assert.match(source, /const compareSectionRef = useRef<HTMLElement \| null>\(null\)/)
  assert.match(source, /const handleCompare = \(versionId: number\) => \{/)
  assert.match(source, /setCompareVersionId\(versionId\)/)
  assert.match(source, /compareSectionRef\.current\?\.scrollIntoView\(/)
  assert.equal((source.match(/onClick=\{\(\) => handleCompare\(version\.id\)\}/g) ?? []).length, 2)
  assert.equal((source.match(/aria-pressed=\{version\.id === effectiveCompareVersionId\}/g) ?? []).length, 2)
  assert.equal((source.match(/variant=\{version\.id === effectiveCompareVersionId \? "default" : "outline"\}/g) ?? []).length, 2)
})
