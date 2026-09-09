import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

test("auth session storage is scoped per portal while retaining legacy compatibility", async () => {
  const source = await readFile(new URL("./auth.ts", import.meta.url), "utf8")

  assert.match(source, /PORTAL_SESSION_STORAGE_PREFIX\s*=\s*`\$\{SESSION_STORAGE_KEY\}:`/)
  assert.match(source, /currentLocationPortal\(\)/)
  assert.match(source, /readSessionFromStorage\(portalSessionStorageKey\(portal\), portal\)/)
  assert.match(source, /sessionStoragePortal\(session\)\s*!==\s*expectedPortal/)
  assert.match(source, /readSessionFromStorage\(portalSessionStorageKey\(portal, true\), portal\)/)
  assert.match(source, /window\.localStorage\.setItem\(LEGACY_SESSION_STORAGE_KEY, serialized\)/)
  assert.match(source, /window\.localStorage\.setItem\(portalSessionStorageKey\(portal, true\), serialized\)/)
  assert.match(source, /window\.sessionStorage\.getItem\(LEGACY_PLATFORM_RETURN_SESSION_KEY\)/)
  assert.match(source, /window\.sessionStorage\.setItem\(LEGACY_PLATFORM_RETURN_SESSION_KEY, serialized\)/)
  assert.match(source, /for \(const portal of LOGIN_PORTALS\)/)
})
