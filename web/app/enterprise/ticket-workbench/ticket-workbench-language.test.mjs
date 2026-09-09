import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./page.tsx", import.meta.url), "utf8")

test("conversation messages keep source text for language-aware translation", () => {
  assert.match(source, /type WorkbenchMessage = \{[\s\S]*content: string/)
  assert.match(source, /content: message\.content/)
  assert.match(source, /html: renderIMMessageHTML\(\{ \.\.\.message, content: displayContent \}\)/)
  assert.match(source, /resolveMessageTranslationTarget\(message\.content, translationTargetLanguage\)/)
})

test("translation avoids same-language no-op and labels the actual target", () => {
  assert.match(source, /sourceLanguage === "zh-CN"[\s\S]*return "en"/)
  assert.match(source, /sourceLanguage === "en"[\s\S]*return "zh-CN"/)
  assert.match(source, /translationLanguageLabel\(translation\.target_language\).*译文/)
  assert.match(source, /原文已经是.*已自动翻译为/)
})
