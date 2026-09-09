import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./page.tsx", import.meta.url), "utf8")
const locales = ["zh-CN", "en-US", "es-ES"]
const messagesByLocale = await Promise.all(
  locales.map(async (locale) => [
    locale,
    JSON.parse(await readFile(new URL(`../../../messages/${locale}.json`, import.meta.url), "utf8")),
  ]),
)

test("device product tree uses shared product display labels", () => {
  assert.match(source, /getProductTreeDisplayName/)
  assert.match(source, /const label = getProductTreeDisplayName\(product\)/)
  assert.match(source, /\$\{label\} \$\{product\.code\} \$\{product\.product_line\} \$\{product\.category\}/)
  assert.doesNotMatch(source, /selectedProduct\.name\} 下导入设备/)
})

test("device product panel uses the shared product directory structure", () => {
  assert.match(source, /ProductTree/)
  assert.match(source, /buildProductTreeGroups\(filteredProducts\)/)
  assert.match(source, /className="rhd-railops-product-panel rhd-railops-product-directory-panel rhd-railops-device-product-panel"/)
  assert.match(source, /mode="directory"/)
  assert.match(source, /rootLabel=\{t\("enterpriseDevices\.allProducts"\)\}/)
  assert.match(source, /search=\{productSearch\}/)
  assert.match(source, /onSelectRoot=\{\(\) => handleSelectProduct\(""\)\}/)
  assert.match(source, /onSelectProduct=\{\(product\) => handleSelectProduct\(String\(product\.id\)\)\}/)
  assert.match(source, /renderProductMeta=\{\(product\) => <small className="kb-product-code">/)
  assert.match(source, /className=\{cn\("ent-product-list-badge"/)
  assert.doesNotMatch(source, /rhd-railops-device-product-filter/)
})

test("device product panel keeps the shared compact presentation", () => {
  assert.match(source, /rhd-railops-product-panel/)
  assert.doesNotMatch(source, /className="ent-product-list-meta"/)
  assert.doesNotMatch(source, /product_id=\$\{product\.id\}/)
  assert.doesNotMatch(source, /product_id=\{device\.product_id/)
  assert.doesNotMatch(source, /className="text-\[11px\] text-muted-foreground"/)
})

test("device page omits product tree device filters and top metric cards", () => {
  assert.doesNotMatch(source, /DeviceMetric/)
  assert.doesNotMatch(source, /treeFilterOptions/)
  assert.doesNotMatch(source, /filterHasDevices/)
  assert.doesNotMatch(source, /filterNoDevices/)
  assert.doesNotMatch(source, /enterpriseDevices\.metrics/)
})

test("device page uses the RailOps table workspace with drawer details", () => {
  assert.match(source, /@railops\/ui/)
  assert.match(source, /<section className="rhd-railops-device-workspace">/)
  assert.match(source, /<ContentModule[\s\S]*className="rhd-railops-device-list-module"/)
  assert.match(source, /<AntTable<DeviceListItem>/)
  assert.match(source, /<DetailDrawer[\s\S]*open=\{Boolean\(selectedDeviceId\)\}/)
  assert.doesNotMatch(source, /ent-product-domain-layout-detail/)
  assert.doesNotMatch(source, /<aside className="space-y-4">/)
})

test("device list keeps stale rows visible while refreshing", () => {
  assert.match(source, /const \[devicesLoaded, setDevicesLoaded\] = useState\(false\)/)
  assert.match(source, /setDevicesLoaded\(true\)/)
  assert.match(source, /const hasDevices = devices\.length > 0/)
  assert.match(source, /const initialDeviceListLoading = loading && !devicesLoaded/)
  assert.match(source, /const deviceListRefreshing = loading && devicesLoaded/)
  assert.match(source, /const blockingDeviceError = Boolean\(error\) && !loading && !hasDevices/)
  assert.match(source, /const deviceRefreshError = Boolean\(error\) && !loading && hasDevices/)
  assert.match(source, /deviceListRefreshing \? \([\s\S]*role="status" aria-busy="true"/)
  assert.match(source, /initialDeviceListLoading \? \(/)
  assert.match(source, /aria-busy=\{deviceListRefreshing\}/)
  assert.doesNotMatch(source, /\{loading \? \(\s*<div className="space-y-3 p-4">/)
  assert.doesNotMatch(source, /setDevices\(\[\]\)\s*setSelectedDeviceId\(""\)/)
})

test("device empty states stay concise", () => {
  const removedKeys = [
    "noDevicesDescription",
    "noDevicesDescriptionWithProduct",
    "panelDescriptionAll",
    "panelDescriptionWithProduct",
    "selectDeviceDescription",
  ]
  const removedDetailKeys = [
    "customerBindingsEmptyDescription",
    "recentTicketsEmptyDescription",
    "repairHistoryEmptyDescription",
    "softwareVersionsEmptyDescription",
  ]

  assert.doesNotMatch(source, /enterpriseDevices\.(noDevicesDescription|noDevicesDescriptionWithProduct|selectDeviceDescription)/)
  assert.doesNotMatch(source, /enterpriseDevices\.(panelDescriptionAll|panelDescriptionWithProduct)/)
  assert.doesNotMatch(source, /enterpriseDevices\.detail\.(customerBindingsEmptyDescription|recentTicketsEmptyDescription|repairHistoryEmptyDescription|softwareVersionsEmptyDescription)/)
  assert.doesNotMatch(source, /<EmptyState[^>]+description=\{t\("enterpriseDevices/)
  assert.doesNotMatch(source, /DialogDescription/)
  assert.doesNotMatch(source, /enterpriseDevices\.dialog\.createDescription/)
  assert.doesNotMatch(source, /enterpriseDevices\.batchDialog\.description/)

  for (const [locale, messages] of messagesByLocale) {
    const enterpriseDevices = messages.enterpriseDevices
    assert.ok(enterpriseDevices, `${locale} should define enterpriseDevices copy`)
    assert.equal(Object.hasOwn(enterpriseDevices.dialog, "createDescription"), false, `${locale} should not keep dialog create description`)
    assert.equal(Object.hasOwn(enterpriseDevices.batchDialog, "description"), false, `${locale} should not keep batch dialog description`)
    for (const key of removedKeys) {
      assert.equal(Object.hasOwn(enterpriseDevices, key), false, `${locale} should not keep enterpriseDevices.${key}`)
    }
    for (const key of removedDetailKeys) {
      assert.equal(Object.hasOwn(enterpriseDevices.detail, key), false, `${locale} should not keep enterpriseDevices.detail.${key}`)
    }
  }
})
