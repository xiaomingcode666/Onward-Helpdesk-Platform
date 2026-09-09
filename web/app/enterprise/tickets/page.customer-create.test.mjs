import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const pageSource = await readFile(new URL("./page.tsx", import.meta.url), "utf8")

test("ticket creation always collects and persists customer context", () => {
  assert.match(pageSource, /if \(!createOpen\) return/)
  assert.doesNotMatch(pageSource, /if \(!createOpen \|\| hasDeviceConcept\) return/)
  assert.match(pageSource, /<FormField label=\{ee\("tickets\.text078"\)\} required>/)
  assert.match(pageSource, /ticketDraft\.customerMode === "existing"/)
  assert.match(pageSource, /ticketDraft\.customerMode === "invite"/)
  assert.match(pageSource, /ticketDraft\.customerId > 0 \? \{ customer_id: ticketDraft\.customerId \}/)
  assert.doesNotMatch(pageSource, /!hasDeviceConcept && ticketDraft\.customerMode/)
})
