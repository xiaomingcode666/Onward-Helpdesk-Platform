import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./_components/detail.tsx", import.meta.url), "utf8")
const localeMessages = await Promise.all(
  ["zh-CN", "en-US", "es-ES"].map(async (locale) => ({
    locale,
    messages: JSON.parse(await readFile(new URL(`../../../messages/${locale}.json`, import.meta.url), "utf8")),
  }))
)

test("conversation monitor detail keeps the dialog structure during loading", () => {
  assert.match(source, /import \{ Skeleton \} from "@\/components\/ui\/skeleton"/)
  assert.match(source, /loading \? \(\s*<ConversationDetailLoading label=\{t\("common\.loadingData"\)\} \/>/)
  assert.match(source, /function ConversationDetailLoading\(\{ label \}: \{ label: string \}\)/)
  assert.match(source, /role="status"/)
  assert.match(source, /aria-busy="true"/)
  assert.match(source, /aria-label=\{label\}/)
  assert.doesNotMatch(source, /conversationMonitor\.loadingDetail/)
  assert.doesNotMatch(source, /flex min-h-0 flex-1 items-center justify-center text-sm text-muted-foreground">\s*\{t\("conversationMonitor\.loadingDetail"\)\}/)
})

test("conversation monitor loading copy is no longer a visible locale key", () => {
  for (const { locale, messages } of localeMessages) {
    assert.equal(Object.hasOwn(messages.conversationMonitor, "loadingDetail"), false, `${locale} should not keep conversationMonitor.loadingDetail`)
  }
})
