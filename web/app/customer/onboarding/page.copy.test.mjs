import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const pageSource = await readFile(new URL("./page.tsx", import.meta.url), "utf8")
const portalSource = await readFile(new URL("../../../components/customer-portal/customer-portal-pages.tsx", import.meta.url), "utf8")
const locales = ["zh-CN", "en-US", "es-ES"]

function getMessage(source, key) {
  return key.split(".").reduce((current, part) => current?.[part], source)
}

test("customer onboarding avoids explanatory page and dialog copy", async () => {
  assert.doesNotMatch(pageSource, /customerOnboarding\.pageDescription/)
  assert.doesNotMatch(portalSource, /customerOnboarding\.dialogDescription/)

  for (const locale of locales) {
    const messages = JSON.parse(
      await readFile(new URL(`../../../messages/${locale}.json`, import.meta.url), "utf8")
    )

    assert.equal(getMessage(messages, "customerOnboarding.pageDescription"), undefined, `${locale} should not keep pageDescription`)
    assert.equal(getMessage(messages, "customerOnboarding.dialogDescription"), undefined, `${locale} should not keep dialogDescription`)
  }
})
