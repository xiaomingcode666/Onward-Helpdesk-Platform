import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const resetSource = await readFile(new URL("./_components/reset-password.tsx", import.meta.url), "utf8")
const initialSource = await readFile(new URL("./_components/initial-password-dialog.tsx", import.meta.url), "utf8")

test("user password dialogs avoid explanatory dialog descriptions", async () => {
  for (const source of [resetSource, initialSource]) {
    assert.doesNotMatch(source, /DialogDescription/)
  }
  assert.doesNotMatch(resetSource, /confirmResetDescription|resetSuccessDescription/)
  assert.doesNotMatch(initialSource, /initialPasswordDescription/)

  for (const locale of ["zh-CN", "en-US", "es-ES"]) {
    const messages = JSON.parse(await readFile(new URL(`../../../messages/${locale}.json`, import.meta.url), "utf8"))
    assert.equal(Object.hasOwn(messages.user, "initialPasswordDescription"), false, `${locale} should not keep initial password description`)
    assert.equal(Object.hasOwn(messages.user, "confirmResetDescription"), false, `${locale} should not keep reset confirmation description`)
    assert.equal(Object.hasOwn(messages.user, "resetSuccessDescription"), false, `${locale} should not keep reset result description`)
  }
})
