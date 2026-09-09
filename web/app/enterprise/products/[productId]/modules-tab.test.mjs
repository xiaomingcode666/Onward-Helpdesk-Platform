import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import { test } from "node:test"

const source = await readFile(new URL("./modules-tab.tsx", import.meta.url), "utf8")

test("modules tab uses i18n labels and status helpers", () => {
  assert.match(source, /useI18n\(\)/)
  assert.match(source, /productCenter\.modules\.createTitle/)
  assert.match(source, /productCenter\.modules\.tableTitle/)
  assert.match(source, /productCenter\.modules\.selectedScopeTitle/)
  assert.match(source, /statusLabel\(t, module\.status\)/)
  assert.match(source, /moduleTypeLabel\(t, module\.is_safety_critical\)/)
  assert.match(source, /const \[modulesLoaded, setModulesLoaded\] = useState\(false\)/)
  assert.match(source, /setModulesLoaded\(true\)/)
  assert.match(source, /const initialLoading = loading && !modulesLoaded/)
  assert.doesNotMatch(source, /if \(loading\)\s*\{\s*return <LoadingSkeleton/)
  assert.doesNotMatch(source, /DialogDescription/)
  assert.doesNotMatch(
    source,
    /新增模块|模块主数据|适用型号维护|安全关键模块|暂不绑定供应商|先维护供应商账号|编辑模块|维护适用型号|停用|启用/
  )
})
