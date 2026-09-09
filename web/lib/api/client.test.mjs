import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./client.ts", import.meta.url), "utf8")
const authSource = await readFile(new URL("../auth.ts", import.meta.url), "utf8")

test("both API clients expire sessions for envelope and HTTP auth failures", () => {
  assert.match(source, /const AUTH_ERROR_CODES = new Set\(\[3000, 3002\]\)/)
  assert.match(source, /expireStoredAuth && response\.status === 401/)
  assert.match(source, /isAuthenticationFailure\(response\.status, payload\.errorCode\)/)
  assert.match(source, /isAuthenticationFailure\(response\.status, json\.errorCode\)/)
  assert.doesNotMatch(source, /session\?\.accessToken && isAuthenticationFailure/)
  assert.doesNotMatch(source, /!skipAuth && authHeaders\.has\("Authorization"\)/)
})

test("concurrent auth failures redirect once until the next login", () => {
  assert.match(authSource, /const authExpiryDispatchedPortals = new Set<LoginPortal>\(\)/)
  assert.match(authSource, /authExpiryDispatchedPortals\.has\(portal\)/)
  assert.match(authSource, /authExpiryDispatchedPortals\.add\(portal\)/)
  assert.match(authSource, /authExpiryDispatchedPortals\.delete\(portal\)/)
  assert.match(authSource, /getPortalFromPath\(currentPath\)/)
  assert.match(authSource, /const portal = currentLocationPortal\(\)/)
  assert.match(authSource, /clearPortalSession\(portal\)/)
  assert.match(authSource, /window\.location\.replace\(loginPath\)/)
})
