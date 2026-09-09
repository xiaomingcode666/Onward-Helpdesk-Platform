import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const locales = ["zh-CN", "en-US", "es-ES"]

function getMessage(source, key) {
  return key.split(".").reduce((current, part) => current?.[part], source)
}

test("customer portal messages do not keep duplicate page eyebrow copy", async () => {
  for (const locale of locales) {
    const messages = JSON.parse(
      await readFile(new URL(`../../messages/${locale}.json`, import.meta.url), "utf8")
    )

    for (const key of [
      "customerChat.pageEyebrow",
      "customerTickets.pageEyebrow",
      "customerMeeting.pageEyebrow",
      "customerProfile.pageEyebrow",
      "customerDevices.sectionLabel",
      "customerChat.myConversationsDescription",
      "customerChat.emptyDescription",
      "customerChat.selectConversationDescription",
      "customerTickets.emptyDescription",
      "customerTickets.syncingDescription",
      "customerTickets.selectDescription",
      "customerMeeting.noUpcomingDescription",
      "customerMeeting.noHistoryDescription",
      "customerMeeting.selectDescription",
      "customerDevices.emptyDescription",
      "customerDevices.selectDeviceDescription",
      "customerDevices.repairArchiveDescription",
      "customerDevices.manualsDescription",
      "customerDevices.noManualsDescription",
      "customerProfile.loadDescription",
    ]) {
      assert.equal(getMessage(messages, key), undefined, `${locale} should not keep ${key}`)
    }
  }
})

test("customer-facing navigation avoids AI-branded helper copy", async () => {
  const expectations = {
    "zh-CN": {
      diagnosisTitle: "诊断",
      mobileChat: "在线咨询",
      selfServiceRate: "自助解决率",
    },
    "en-US": {
      diagnosisTitle: "Diagnosis",
      mobileChat: "Online support",
      selfServiceRate: "Self-service rate",
    },
    "es-ES": {
      diagnosisTitle: "Diagnóstico",
      mobileChat: "Online support",
      selfServiceRate: "Tasa de autoservicio",
    },
  }

  for (const locale of locales) {
    const messages = JSON.parse(
      await readFile(new URL(`../../messages/${locale}.json`, import.meta.url), "utf8")
    )
    const expected = expectations[locale]

    assert.equal(messages.diagnosis.title, expected.diagnosisTitle)
    assert.equal(messages.remoteNav.mobile.chat, expected.mobileChat)
    assert.equal(messages.enterpriseWorkbench.quality.aiResolveRate, expected.selfServiceRate)
    assert.equal(/AI|IA/.test(messages.diagnosis.title), false, `${locale} diagnosis title should stay neutral`)
    assert.equal(/AI|IA/.test(messages.remoteNav.mobile.chat), false, `${locale} mobile chat label should stay neutral`)
    assert.equal(/AI|IA/.test(messages.customerChat.assistantResponding), false, `${locale} assistant responding copy should stay neutral`)
    assert.equal(/AI|IA/.test(messages.customerChat.realtimeAssistantResponding), false, `${locale} realtime assistant copy should stay neutral`)
    assert.equal(/AI|IA/.test(messages.customerChat.realtimeAssistantOnline), false, `${locale} assistant online copy should stay neutral`)
    assert.equal(/AI|IA/.test(messages.customerChat.statusAiServing), false, `${locale} serving status should stay neutral`)
  }
})

test("locale files do not keep old prototype shell copy", async () => {
  const removedRemoteShellKeys = [
    "prototypeIA",
    "routeContract",
    "menuPermission",
    "operatingRecords",
    "moduleShellMounted",
    "fallbackOpenWork",
    "fallbackRisk",
    "fallbackUpdates",
    "fallbackActions",
    "fallbackOverview",
    "fallbackQueue",
    "fallbackRecords",
    "fallbackAudit",
    "viewAll",
    "quickActions",
    "currentOperatingContext",
  ]

  for (const locale of locales) {
    const messages = JSON.parse(
      await readFile(new URL(`../../messages/${locale}.json`, import.meta.url), "utf8")
    )

    assert.equal(messages.remoteOverview, undefined, `${locale} should not keep remoteOverview previews`)
    assert.equal(messages.remoteModule, undefined, `${locale} should not keep remoteModule previews`)

    for (const key of removedRemoteShellKeys) {
      assert.equal(messages.remoteShell?.[key], undefined, `${locale} should not keep remoteShell.${key}`)
    }
  }
})
