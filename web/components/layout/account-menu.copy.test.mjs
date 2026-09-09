import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const accountMenuSource = await readFile(new URL("./account-menu.tsx", import.meta.url), "utf8")
const changePasswordSource = await readFile(new URL("../change-password-dialog.tsx", import.meta.url), "utf8")
const locales = ["zh-CN", "en-US", "es-ES"]
const messagesByLocale = await Promise.all(
  locales.map(async (locale) => [
    locale,
    JSON.parse(await readFile(new URL(`../../messages/${locale}.json`, import.meta.url), "utf8")),
  ]),
)

test("account dialogs avoid explanatory description copy", () => {
  assert.doesNotMatch(accountMenuSource, /DialogDescription/)
  assert.doesNotMatch(accountMenuSource, /account\.(profileDescription|languageDescription)/)
  assert.doesNotMatch(changePasswordSource, /changePasswordDescription/)
  assert.doesNotMatch(changePasswordSource, /description=\{t\("account\.changePasswordDescription"\)\}/)

  for (const [locale, messages] of messagesByLocale) {
    assert.equal(Object.hasOwn(messages.account, "profileDescription"), false, `${locale} should not keep profile description copy`)
    assert.equal(Object.hasOwn(messages.account, "languageDescription"), false, `${locale} should not keep language description copy`)
    assert.equal(Object.hasOwn(messages.account, "changePasswordDescription"), false, `${locale} should not keep password description copy`)
  }
})
