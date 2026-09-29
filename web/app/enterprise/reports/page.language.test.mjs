import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./page.tsx", import.meta.url), "utf8")

test("report date range refreshes when the active locale changes", () => {
  assert.match(source, /const \{ locale \} = useAppLocale\(\)/)
  assert.match(source, /useMemo\(\(\) => getDateRange\(periodKey, locale\), \[locale, periodKey\]\)/)
})
