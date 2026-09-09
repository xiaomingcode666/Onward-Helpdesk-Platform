import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const editDialog = await readFile(new URL("./_components/edit.tsx", import.meta.url), "utf8")
const conversationDialog = await readFile(new URL("./_components/create-ticket-from-conversation-dialog.tsx", import.meta.url), "utf8")
const messages = await Promise.all(
  ["zh-CN", "en-US", "es-ES"].map(async (locale) => [
    locale,
    JSON.parse(await readFile(new URL(`../../../messages/${locale}.json`, import.meta.url), "utf8")),
  ]),
)

test("ticket edit dialogs avoid redundant header descriptions", () => {
  assert.doesNotMatch(editDialog, /descriptionOverride/)
  assert.doesNotMatch(editDialog, /ticket\.dialogDescription/)
  assert.doesNotMatch(conversationDialog, /conversationToTicketDescription/)

  for (const [locale, catalog] of messages) {
    assert.equal(Object.hasOwn(catalog.ticket, "dialogDescription"), false, `${locale} should not keep ticket.dialogDescription`)
    assert.equal(Object.hasOwn(catalog.ticket, "conversationToTicketDescription"), false, `${locale} should not keep ticket.conversationToTicketDescription`)
  }
})
