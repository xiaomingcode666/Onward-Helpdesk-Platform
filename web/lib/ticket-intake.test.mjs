import assert from "node:assert/strict"
import test from "node:test"
import { EMPTY_INTAKE, phoneIntakePayload, missingPhoneContext } from "./ticket-intake.ts"

test("phone intake preserves real values and never invents caller context", () => {
  const result = phoneIntakePayload("phone", EMPTY_INTAKE)
  assert.equal(result.caller_name, "")
  assert.equal(result.caller_phone, "")
  assert.equal(result.source_record_id, "")
  assert.equal(result.received_at, undefined)
  assert.deepEqual(phoneIntakePayload("enterprise", { ...EMPTY_INTAKE, caller_phone: "hidden" }), {})
})

test("phone payload trims inputs and normalizes the supplied instant", () => {
  const result = phoneIntakePayload("phone", { ...EMPTY_INTAKE, caller_phone: " +1 0000 ", source_record_id: " ext-123 ", received_at: "2026-09-08T09:00:00+08:00" })
  assert.equal(result.caller_phone, "+1 0000")
  assert.equal(result.source_record_id, "ext-123")
  assert.equal(result.received_at, "2026-09-08T01:00:00.000Z")
  assert.throws(() => phoneIntakePayload("phone", { ...EMPTY_INTAKE, received_at: "bad date" }))
})

test("unconfigured project remains context incomplete", () => {
  assert.deepEqual(missingPhoneContext(EMPTY_INTAKE, { rules: [] }, 0), ["project_key", "ticket_type", "projectPolicy"])
})

test("context follows matching project channel and ticket type", () => {
  const rule = { project_key: "p", channel: "phone", ticket_type: "incident", required_fields: ["caller_phone", "customer_id", "device_id"] }
  const draft = { ...EMPTY_INTAKE, project_key: "p", ticket_type: "incident", caller_phone: "  " }
  assert.deepEqual(missingPhoneContext(draft, { rules: [rule] }, 0), ["caller_phone", "customer_id", "device_id"])
  assert.deepEqual(missingPhoneContext({ ...draft, caller_phone: "123", device_id: 5 }, { rules: [rule] }, 8), [])
  for (const mismatch of [{ channel: "email" }, { project_key: "other" }, { ticket_type: "request" }]) {
    assert.deepEqual(missingPhoneContext(draft, { rules: [{ ...rule, ...mismatch }] }, 0), ["projectPolicy"])
  }
})
