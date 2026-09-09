import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./manuals-tab.tsx", import.meta.url), "utf8")

test("product manuals tab uses shared productCenter copy", () => {
  assert.match(source, /useI18n\(\)/)
  assert.match(source, /productCenter\.manuals\.uploadTitle/)
  assert.match(source, /productCenter\.manuals\.tableTitle/)
  assert.match(source, /manualVisibilityLabel\(t, item\.visibility\)/)
  assert.match(source, /manualSyncLabel\(t, item\)/)
  assert.match(source, /productCenter\.manuals\.previewTitle/)
  assert.match(source, /productCenter\.manuals\.editTitle/)
  assert.match(source, /const \[itemsLoaded, setItemsLoaded\] = useState\(false\)/)
  assert.match(source, /setItemsLoaded\(true\)/)
  assert.match(source, /const initialLoading = loading && !itemsLoaded/)
  assert.doesNotMatch(source, /if \(loading\)\s*\{\s*return <LoadingSkeleton/)
  assert.doesNotMatch(source, /上传产品手册|暂无手册|产品手册文件|预览产品手册|编辑产品手册/)
})
