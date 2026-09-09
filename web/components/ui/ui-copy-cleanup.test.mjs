import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const sidebarSource = await readFile(new URL("./sidebar.tsx", import.meta.url), "utf8")
const commandSource = await readFile(new URL("./command.tsx", import.meta.url), "utf8")

test("base UI components avoid scaffold descriptions", () => {
  assert.doesNotMatch(sidebarSource, /Displays the mobile sidebar/)
  assert.doesNotMatch(sidebarSource, /<SheetDescription>/)
  assert.match(sidebarSource, /<SheetTitle>导航<\/SheetTitle>/)

  assert.doesNotMatch(commandSource, /Command Palette/)
  assert.doesNotMatch(commandSource, /Search for a command to run/)
  assert.match(commandSource, /title = "搜索"/)
  assert.match(commandSource, /description \? <DialogDescription>\{description\}<\/DialogDescription> : null/)
})
