import assert from "node:assert/strict"
import test from "node:test"

import { collectKeyReferences, findScopedTranslators } from "./check-i18n-keys.mjs"

test("collects direct translator keys", () => {
  const source = [
    'const title = t("knowledge.title")',
    'toast.error(translateCurrentMessage("common.loadFailed", { name }))',
    'const label = format("not.a.translation")',
  ].join("\n")

  assert.deepEqual(collectKeyReferences(source), [
    { key: "knowledge.title", line: 1 },
    { key: "common.loadFailed", line: 2 },
  ])
})

test("expands keys passed to scoped translators", () => {
  const source = [
    "function tr(key: string, values?: Record<string, string | number>) {",
    "  return translateCurrentMessage(`iamExtract.${key}`, values)",
    "}",
    "const pc = (key: string) => translate(`projectConfiguration.${key}`)",
    "const introT = useCallback((key: string, values?: Record<string, string | number>) => t(`systemIntroPage.${key}`, values), [t])",
    'tr("workspace.title")',
    'pc("saveFailed")',
    'introT("pageTitle")',
  ].join("\n")

  assert.deepEqual(Object.fromEntries(findScopedTranslators(source)), {
    tr: "iamExtract",
    pc: "projectConfiguration",
    introT: "systemIntroPage",
  })
  assert.deepEqual(collectKeyReferences(source).map((reference) => reference.key).sort(), [
    "iamExtract.workspace.title",
    "projectConfiguration.saveFailed",
    "systemIntroPage.pageTitle",
  ])
})

test("treats a locally redefined t as a scoped translator", () => {
  const source = [
    "function t(key: string) {",
    "  return translateCurrentMessage(`ticketQuality.${key}`)",
    "}",
    't("summary.title")',
  ].join("\n")

  assert.deepEqual(collectKeyReferences(source).map((reference) => reference.key), ["ticketQuality.summary.title"])
})

test("accepts a translator passed as the first argument", () => {
  const source = [
    "function wd(t: Translate, key: string) {",
    "  return t(`workflowExtract.detail.${key}`)",
    "}",
    'wd(t, "nodeLabels.start")',
  ].join("\n")

  assert.deepEqual(collectKeyReferences(source).map((reference) => reference.key), ["workflowExtract.detail.nodeLabels.start"])
})
