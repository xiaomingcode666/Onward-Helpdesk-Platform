import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./provider.tsx", import.meta.url), "utf8")

test("tenant default locale is isolated from another tenant's browser locale", () => {
  assert.match(source, /function localeStorageKey\(session: AuthSession \| null\)/)
  assert.match(
    source,
    /`\$\{LOCALE_STORAGE_KEY\}:\$\{getSessionPortal\(session\.domainType\)\}:\$\{session\.tenantId\}`/
  )

  const sessionIndex = source.indexOf("const session = readSession()")
  const storedLocaleIndex = source.indexOf(
    "window.localStorage.getItem(localeStorageKey(session))"
  )
  const customerLocaleIndex = source.indexOf("session.customerDefaultLocale")
  const tenantLocaleIndex = source.indexOf("session?.tenantDefaultLocale")
  const accountLocaleIndex = source.indexOf("session?.user?.locale || session?.locale")

  assert.ok(sessionIndex >= 0)
  assert.ok(storedLocaleIndex > sessionIndex)
  assert.ok(customerLocaleIndex > storedLocaleIndex)
  assert.ok(tenantLocaleIndex > customerLocaleIndex)
  assert.ok(accountLocaleIndex > tenantLocaleIndex)
  assert.match(source, /writeBrowserLocale\(configuredLocale\)/)
  assert.doesNotMatch(
    source,
    /window\.localStorage\.setItem\(LOCALE_STORAGE_KEY, configuredLocale\)/
  )
  assert.match(source, /applyLocale\(storedLocale\)\n/)
  assert.doesNotMatch(source, /applyLocale\(storedLocale, true\)/)
})
