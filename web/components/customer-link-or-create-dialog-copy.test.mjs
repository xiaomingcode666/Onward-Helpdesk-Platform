import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./customer-link-or-create-dialog.tsx", import.meta.url), "utf8")
const localeMessages = await Promise.all(
  ["zh-CN", "en-US", "es-ES"].map(async (locale) => ({
    locale,
    messages: JSON.parse(await readFile(new URL(`../messages/${locale}.json`, import.meta.url), "utf8")),
  })),
)

test("customer link dialog does not render explanatory description copy", () => {
  assert.doesNotMatch(source, /const description = \(/)
  assert.doesNotMatch(source, /description=\{description\}/)
  assert.doesNotMatch(source, /customerLink\.description/)

  for (const { locale, messages } of localeMessages) {
    const keys = Object.keys(messages.customerLink ?? {})
    for (const key of keys) {
      assert.equal(key.startsWith("description"), false, `${locale} should not keep customerLink.${key}`)
    }
  }
})
