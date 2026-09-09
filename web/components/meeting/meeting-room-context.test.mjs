import assert from "node:assert/strict"
import test from "node:test"
import { readFile } from "node:fs/promises"
import vm from "node:vm"
import ts from "typescript"

async function loadModule() {
  const source = await readFile(new URL("./meeting-room-context.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS },
    fileName: "meeting-room-context.ts",
  })
  const sandbox = { exports: {}, module: { exports: {} } }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

test("keeps the ticket context through the meeting room journey", async () => {
  const { buildEnterpriseMeetingRoomPath, getMeetingReturnPath, getMeetingTicketId } = await loadModule()

  assert.equal(buildEnterpriseMeetingRoomPath("meeting 1", 42), "/enterprise/meeting-room?meeting_id=meeting%201&ticket_id=42")
  assert.equal(getMeetingTicketId({ ticketId: 42 }), 42)
  assert.equal(getMeetingTicketId({ ticket_id: 43 }), 43)
  assert.equal(getMeetingReturnPath({ ticketId: 42 }), "/enterprise/ticket-workbench?ticket_id=42")
  assert.equal(getMeetingReturnPath(null), "/enterprise/video")
})

test("uses a business-facing meeting title", async () => {
  const { getMeetingDisplaySubject } = await loadModule()

  assert.equal(getMeetingDisplaySubject({ subject: "TK-42 · 电源故障" }, "room-opaque"), "TK-42 · 电源故障")
  assert.equal(getMeetingDisplaySubject({ ticketNo: "TK-42" }, "room-opaque"), "TK-42 · 远程视频协作")
  assert.equal(getMeetingDisplaySubject(null, "room-opaque"), "room-opaque")
})
