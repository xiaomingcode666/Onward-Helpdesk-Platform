import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./ticket-detail-content.tsx", import.meta.url), "utf8")

test("ticket detail tabs are translated during render", () => {
  assert.match(source, /function buildDetailTabs\(\): RailopsTabItem\[\]/)
  assert.match(source, /const detailTabs = buildDetailTabs\(\)/)
  assert.doesNotMatch(source, /const detailTabs: RailopsTabItem\[\] =/)
})

test("ticket assignment history is rendered and translated during render", () => {
  assert.match(source, /aggregate\.assignment_history \?\? \[\]/)
  assert.match(source, /data-testid="ticket-assignment-history"/)
  assert.match(source, /assignmentOutcomeLabel\(item\.outcome\)/)
  assert.match(source, /assignmentSourceLabel\(item\.source\)/)
  assert.match(source, /ee\("ticketDetail\.text118", \{ value0: item\.reason \}\)/)
  assert.match(source, /ee\("ticketDetail\.text106"\)/)
})

test("ticket knowledge access is explained for the current viewer and assignee", () => {
  assert.match(source, /useAuth\(\)/)
  assert.match(source, /data-testid="ticket-support-access-banner"/)
  assert.match(source, /viewerIsAssignee/)
  assert.match(source, /ticketSupportAccessLabel\(accessStatus\)/)
  assert.match(source, /"ticketDetail\.text124"/)
  assert.match(source, /"ticketDetail\.text125"/)
  assert.match(source, /ticket\.support_reason/)
  assert.match(source, /ticket\.support_checked_at/)
  assert.doesNotMatch(source, /ticketSupportStatusLabel/)
  assert.doesNotMatch(source, /\[ee\("ticketDetail\.text119"\),/)
  assert.doesNotMatch(source, /label=\{ee\("ticketDetail\.text119"\)\}/)
})

test("ticket service profile and target are rendered from the configuration snapshot", () => {
  assert.match(source, /ticketServiceProfileLabel\(ticket\.service_profile\)/)
  assert.match(source, /ticketServiceTargetLabel\(ticket\.service_target\)/)
  assert.match(source, /ticket\.service_metrics\?\.length/)
  assert.match(source, /ticketServiceMetricLabel\(metric\.metric_type\)/)
  assert.match(source, /ticketServiceMetricStatusLabel\(metric\.status\)/)
  assert.match(source, /ee\("tickets\.text103"\)/)
  assert.match(source, /ee\("tickets\.text108"\)/)
})
