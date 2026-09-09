import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./customer-meeting-privacy-consent.ts", import.meta.url), "utf8")

test("remembered meeting privacy consent is account scoped and versioned", () => {
  assert.match(source, /CUSTOMER_MEETING_PRIVACY_NOTICE_VERSION = "2026-08-12-meeting-v1"/)
  assert.match(source, /scope\.tenantId <= 0 \|\| scope\.userId <= 0/)
  assert.match(source, /\$\{CUSTOMER_MEETING_PRIVACY_STORAGE_PREFIX\}:\$\{scope\.tenantId\}:\$\{scope\.userId\}/)
  assert.match(source, /consent\.policyVersion === CUSTOMER_MEETING_PRIVACY_NOTICE_VERSION/)
})

test("customers can revoke a remembered meeting privacy choice", () => {
  assert.match(source, /if \(!accepted\) \{[\s\S]*window\.localStorage\.removeItem\(storageKey\)/)
  assert.match(source, /acceptedAt: new Date\(\)\.toISOString\(\)/)
})
