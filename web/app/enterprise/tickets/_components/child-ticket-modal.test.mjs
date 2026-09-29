import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const governanceSource = await readFile(new URL("./ticket-governance.tsx", import.meta.url), "utf8")
const createModalSource = await readFile(new URL("./ticket-create-modal.tsx", import.meta.url), "utf8")
const workbenchSource = await readFile(new URL("../../ticket-workbench/page.tsx", import.meta.url), "utf8")

test("child ticket actions open the shared modal instead of navigating", () => {
  assert.match(governanceSource, /onClick=\{\(\) => onCreateChild\(id\)\}/)
  assert.doesNotMatch(governanceSource, /\/enterprise\/tickets\?parent_ticket_id=/)
  assert.match(workbenchSource, /onCreateChild=\{setChildCreateParentTicketId\}/)
  assert.match(workbenchSource, /<TicketCreateModal[\s\S]*parentTicketId=\{childCreateParentTicketId\}/)
})

test("the shared modal preserves parent relation fields for both customer modes", () => {
  assert.equal(createModalSource.match(/parent_ticket_id: draft\.parentId/g)?.length, 2)
  assert.equal(createModalSource.match(/relation_reason: draft\.relationReason/g)?.length, 2)
})
