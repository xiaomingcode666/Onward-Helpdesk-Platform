import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const routeAliasSource = await readFile(new URL("./route-alias.tsx", import.meta.url), "utf8")
const dashboardAgentConfigAliasSource = await readFile(
  new URL("../../app/dashboard/ai-agents/config/page.tsx", import.meta.url),
  "utf8",
)
const localeSources = await Promise.all([
  "../../messages/zh-CN.json",
  "../../messages/en-US.json",
  "../../messages/es-ES.json",
].map(async (file) => readFile(new URL(file, import.meta.url), "utf8")))
const localeMessages = localeSources.map((source) => JSON.parse(source))

test("route aliases do not render default explanatory copy", () => {
  assert.match(routeAliasSource, /const resolvedTitle = title \?\? t\("routeAlias\.title"\)/)
  assert.doesNotMatch(routeAliasSource, /description\?: string/)
  assert.doesNotMatch(routeAliasSource, /description \? <span/)
  assert.doesNotMatch(routeAliasSource, /resolvedDescription/)
  assert.doesNotMatch(routeAliasSource, /routeAlias\.description/)
})

test("route alias locale copy only keeps the action title", () => {
  const combinedLocales = localeSources.join("\n")
  for (const messages of localeMessages) {
    assert.equal(typeof messages.routeAlias?.title, "string")
    assert.equal(Object.hasOwn(messages.routeAlias, "description"), false)
  }
  assert.doesNotMatch(combinedLocales, /海外设备智能售后 SaaS 平台/)
  assert.doesNotMatch(combinedLocales, /Overseas equipment after-sales SaaS platform/)
  assert.doesNotMatch(combinedLocales, /Plataforma SaaS posventa para equipos en el extranjero/)
})

test("legacy dashboard agent config route renders a visible alias instead of a blank redirect", () => {
  assert.match(dashboardAgentConfigAliasSource, /<RouteAlias/)
  assert.match(dashboardAgentConfigAliasSource, /buildEnterpriseAIPath\(agentId\)/)
  assert.doesNotMatch(dashboardAgentConfigAliasSource, /return null/)
  assert.doesNotMatch(dashboardAgentConfigAliasSource, /router\.replace/)
  assert.doesNotMatch(dashboardAgentConfigAliasSource, /useEffect/)
})
