import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const panel = await readFile(new URL("./ticket-quality-panel.tsx", import.meta.url), "utf8")
const page = await readFile(new URL("../../ticket-quality/page.tsx", import.meta.url), "utf8")
const api = await readFile(new URL("../../../../lib/api/enterprise-tickets.ts", import.meta.url), "utf8")

test("quality review loads an explainable automatic assessment", () => {
  assert.match(panel, /fetchTicketQualityAutoAssessment\(ticketID\)/)
  assert.match(panel, /assessmentResponse\.data\.answers/)
  assert.match(panel, /review_required_count/)
  assert.match(panel, /evidence_key/)
})

test("manual scoring and advanced fields stay collapsed by default", () => {
  assert.match(panel, /scoreDetailsOpen/)
  assert.match(panel, /advancedOpen/)
  assert.doesNotMatch(panel, /<option value=\{0\}>0<\/option>/)
  assert.match(panel, /\[0, 1, 2\]\.map/)
})

test("sample list exposes saved results and uses equal-size analysis controls", () => {
  assert.match(page, /columns\.reviewResult/)
  assert.match(page, /item\.latest_review/)
  assert.match(page, /actions\.viewResult/)
  assert.match(page, /item\.status !== "pending"/)
  assert.match(page, /grid items-end gap-3 md:grid-cols-2 xl:grid-cols-4 2xl:grid-cols-8/)
  assert.match(page, /DatePicker className="ticket-quality-date-picker h-9 w-full"/)
  assert.match(api, /quality-reviews\/auto-assessment/)
})

test("saved review history shows the complete multiline evidence", () => {
  assert.match(panel, /whitespace-pre-wrap break-words leading-5/)
  assert.doesNotMatch(panel, /line-clamp-2 text-muted-foreground">\{review\.evidence\}/)
})
