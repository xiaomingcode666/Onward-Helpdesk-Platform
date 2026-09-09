import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./_components/speech-runtime-panel.tsx", import.meta.url), "utf8")
const messages = await Promise.all(
  ["zh-CN", "en-US", "es-ES"].map(async (locale) => [
    locale,
    JSON.parse(await readFile(new URL(`../../../messages/${locale}.json`, import.meta.url), "utf8")),
  ]),
)

test("speech runtime provider keeps test mode productized", () => {
  assert.match(source, /speechProviderMock/)
  for (const [locale, catalog] of messages) {
    assert.notEqual(catalog.aiConfig.speechProviderMock, "Mock transcription", `${locale} should avoid mock copy`)
    assert.notEqual(catalog.aiConfig.speechProviderMock, "Test transcription", `${locale} should avoid test copy`)
    assert.notEqual(catalog.aiConfig.speechProviderMock, "测试转写", `${locale} should avoid test copy`)
    assert.notEqual(catalog.aiConfig.speechMockSegmentDuration, "Mock caption segment duration (ms)", `${locale} should avoid mock copy`)
    assert.notEqual(catalog.aiConfig.speechMockSegmentDuration, "Test caption segment duration (ms)", `${locale} should avoid test copy`)
    assert.notEqual(catalog.aiConfig.speechMockSegmentDuration, "测试字幕分段时长 (ms)", `${locale} should avoid test copy`)
    assert.equal(Object.hasOwn(catalog.aiConfig, "speechTranslationDescription"), false, `${locale} should not keep explanation-only speech copy`)
  }
  assert.doesNotMatch(source, /speechTranslationDescription/)
})
