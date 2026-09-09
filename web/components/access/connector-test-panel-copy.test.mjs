import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./connector-test-panel.tsx", import.meta.url), "utf8")
const messages = await Promise.all(
  ["zh-CN", "en-US", "es-ES"].map(async (locale) => [
    locale,
    JSON.parse(await readFile(new URL(`../../messages/${locale}.json`, import.meta.url), "utf8")),
  ]),
)

test("connector test panel uses localized compact copy", () => {
  const leftoverPatterns = [
    /Test Now/,
    /No test has been run/,
    /Unknown test error/,
    /Testing connection/,
    /Connection successful/,
    /Connection failed/,
    /Show detail/,
    /Hide detail/,
    /"Fast"/,
    /"Moderate"/,
    /"Slow"/,
    /"Timeout"/,
    /testInProgressDesc/,
    /testInfo/,
  ]

  for (const pattern of leftoverPatterns) {
    assert.doesNotMatch(source, pattern)
  }
  assert.match(source, /access\.latencyFast/)
  assert.match(source, /access\.latencyNormal/)
  assert.match(source, /access\.latencySlow/)
  assert.match(source, /access\.latencyTimeout/)

  for (const [locale, catalog] of messages) {
    assert.equal(Object.hasOwn(catalog.access, "testInProgressDesc"), false, `${locale} should not keep connector explanation copy`)
    assert.equal(Object.hasOwn(catalog.access, "testInfo"), false, `${locale} should not keep connector explanation copy`)
  }
})
