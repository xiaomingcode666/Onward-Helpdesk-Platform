import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import vm from "node:vm"
import ts from "typescript"

function compile(source, dependencies = {}) {
  const compiled = ts.transpileModule(source, {
    compilerOptions: { target: ts.ScriptTarget.ES2017, module: ts.ModuleKind.CommonJS, esModuleInterop: true },
  })
  const sandbox = { exports: {}, require: (name) => {
    assert.ok(name in dependencies, `Unexpected import: ${name}`)
    return dependencies[name]
  } }
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.exports
}

const config = compile(await readFile(new URL("../i18n/config.ts", import.meta.url), "utf8"))
const catalogs = Object.fromEntries(await Promise.all(config.SUPPORTED_LOCALES.map(async (locale) => [
  locale, JSON.parse(await readFile(new URL(`../messages/${locale}.json`, import.meta.url), "utf8")),
])))
const messages = compile(await readFile(new URL("../i18n/messages.ts", import.meta.url), "utf8"), {
  "@/i18n/config": config,
  "@/i18n/extracted": { getExtractedMessage: () => undefined },
  ...Object.fromEntries(Object.entries(catalogs).map(([locale, catalog]) => [`@/messages/${locale}.json`, catalog])),
})
const { localizeAIAgentReleaseReviewComment } = compile(await readFile(new URL("./ai-agent-release-i18n.ts", import.meta.url), "utf8"), {
  "@/i18n/config": config,
  "@/i18n/messages": messages,
})

test("stored system release comments follow the selected locale without changing the record", () => {
  const chinese = "平台托管配置已同步，需按审核流程逐个部署"
  const english = "Platform-managed configuration has been synced. Each release must be reviewed before deployment."
  assert.equal(localizeAIAgentReleaseReviewComment(chinese, "en-US"), english)
  assert.equal(localizeAIAgentReleaseReviewComment(english, "zh-CN"), chinese)
  assert.equal(localizeAIAgentReleaseReviewComment(chinese, "zh-CN"), chinese)
  assert.equal(localizeAIAgentReleaseReviewComment(chinese, "en-US"), english)
})

test("all built-in release comments switch between supported languages", () => {
  for (const key of ["configurationSynced", "initialized", "collaborationSynced", "aiOnlySynced"]) {
    for (const locale of config.SUPPORTED_LOCALES) {
      const original = catalogs[locale].aiAgentRelease.systemComments[key]
      for (const target of config.SUPPORTED_LOCALES) {
        assert.equal(localizeAIAgentReleaseReviewComment(original, target), catalogs[target].aiAgentRelease.systemComments[key])
      }
    }
  }
})

test("custom reviewer comments and additions remain unchanged", () => {
  for (const comment of ["人工审核：需要补充资料", "Approved after manual review", "平台托管配置已同步，需按审核流程逐个部署；请联系管理员", ""]) {
    assert.equal(localizeAIAgentReleaseReviewComment(comment, "en-US"), comment)
  }
  assert.equal(localizeAIAgentReleaseReviewComment(undefined, "en-US"), "")
})
