import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./fault-stats-tab.tsx", import.meta.url), "utf8")

test("fault stats uses shared product archive i18n labels", () => {
  assert.match(source, /useI18n\(\)/)
  assert.match(source, /productArchive\.faultStats\.loadFailed/)
  assert.match(source, /productArchive\.faultStats\.summary\.faults/)
  assert.match(source, /productArchive\.faultStats\.columns\.severity/)
  assert.match(source, /rangeLabel\(t, item\)/)
  assert.match(source, /severityLabel\(t, s\.severity\)/)
  assert.match(source, /action=\{\{\s*label: t\("productArchive\.retry"\), onClick: load\s*\}\}/)
  assert.match(source, /const initialLoading = stats === null && loading/)
  assert.doesNotMatch(source, /if \(loading\)\s*\{\s*return <LoadingSkeleton/)
  assert.doesNotMatch(source, /\u6545\u969c\u603b\u6570|\u6545\u969c\u5206\u5e03|\u8fd190\u5929|\u91cd\u5efa\u7edf\u8ba1|\u4e0a\u5347|\u4e25\u91cd/)
})
