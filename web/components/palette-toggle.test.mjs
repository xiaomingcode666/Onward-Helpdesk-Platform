import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const paletteSource = await readFile(
  new URL("./palette-toggle.tsx", import.meta.url),
  "utf8",
)
const layoutSource = await readFile(new URL("../app/layout.tsx", import.meta.url), "utf8")
const zhMessages = JSON.parse(
  await readFile(new URL("../messages/zh-CN.json", import.meta.url), "utf8"),
)
const enMessages = JSON.parse(
  await readFile(new URL("../messages/en-US.json", import.meta.url), "utf8"),
)

test("plain palette is the default RemoteHelpDesk palette", () => {
  assert.match(paletteSource, /type PaletteMode = "plain"/)
  assert.match(paletteSource, /const DEFAULT_PALETTE: PaletteMode = "plain"/)
  assert.match(paletteSource, /const PALETTE_STORAGE_KEY = "remote_helpdesk_palette"/)
  assert.match(paletteSource, /const LEGACY_PALETTE_STORAGE_KEY = "dashboard_palette"/)
  assert.match(layoutSource, /remote_helpdesk_palette/)
  assert.match(layoutSource, /dashboard_palette/)
  assert.match(layoutSource, /dataset\.palette = "plain"/)
})

test("plain palette is available in the palette menu and messages", () => {
  assert.match(paletteSource, /value: "plain"[\s\S]*labelKey: "palette\.plain"/)
  assert.doesNotMatch(paletteSource, /value: "green"/)
  assert.doesNotMatch(paletteSource, /value: "gray"/)
  assert.doesNotMatch(paletteSource, /value: "blue"/)
  assert.equal(zhMessages.palette.plain, "默认")
  assert.equal(enMessages.palette.plain, "Default")
})
