import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./customer-portal-client.ts", import.meta.url), "utf8")

test("customer portal requests participate in shared authentication expiry handling", () => {
  assert.match(source, /skipAuth: false/)
  assert.doesNotMatch(source, /skipAuth: true/)
})
