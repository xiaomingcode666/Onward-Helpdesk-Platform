import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./models-tab.tsx", import.meta.url), "utf8")

test("product models tab uses shared productCenter copy", () => {
  assert.match(source, /useI18n\(\)/)
  assert.match(source, /productCenter\.model\.createTitle/)
  assert.match(source, /productCenter\.model\.tableTitle/)
  assert.match(source, /productCenter\.model\.referencesSummary/)
  assert.match(source, /statusLabel\(t, model\.status\)/)
  assert.match(source, /t\("productCenter\.retry"\)/)
  assert.match(source, /const \[modelsLoaded, setModelsLoaded\] = useState\(false\)/)
  assert.match(source, /setModelsLoaded\(true\)/)
  assert.match(source, /const initialLoading = loading && !modelsLoaded/)
  assert.doesNotMatch(source, /if \(loading\)\s*\{\s*return <LoadingSkeleton/)
  assert.doesNotMatch(source, /DialogDescription/)
  assert.doesNotMatch(source, /新增型号|型号主数据|版本策略|适用区域|编辑型号|停用|启用/)
})
