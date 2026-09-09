import assert from "node:assert/strict"
import test from "node:test"

import {
  isProcessingTicketStatus,
  isTerminalTicketStatus,
} from "./ticket-lifecycle.ts"

test("classifies after-sales terminal states without hiding quality review", () => {
  assert.equal(isTerminalTicketStatus("closed"), true)
  assert.equal(isTerminalTicketStatus("done"), true)
  assert.equal(isTerminalTicketStatus("cancelled"), true)
  assert.equal(isTerminalTicketStatus("resolved"), false)
  assert.equal(isTerminalTicketStatus("pending_customer_confirm"), false)
  assert.equal(isTerminalTicketStatus("quality_review"), false)
})

test("classifies all active repair collaboration states as processing", () => {
  for (const status of ["in_progress", "processing", "video_support", "supplier_support"]) {
    assert.equal(isProcessingTicketStatus(status), true, status)
  }
  assert.equal(isProcessingTicketStatus("closed"), false)
})

