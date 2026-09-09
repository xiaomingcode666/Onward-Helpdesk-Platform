import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./page.tsx", import.meta.url), "utf8")
const oldPageLoader = "Platform" + "Loading"
const oldPageError = "Platform" + "Error"

test("platform overview renders the page shell before API responses", () => {
  assert.doesNotMatch(source, new RegExp(oldPageLoader))
  assert.doesNotMatch(source, new RegExp(oldPageError))
  assert.doesNotMatch(source, /\{data \? \(\s*<div className="space-y-3">/)
  assert.match(source, /<div className="space-y-3">[\s\S]*<TenantScalePanel/)
})

test("platform overview uses the shared platform page header", () => {
  assert.match(source, /PlatformPageHeader/)
  assert.match(source, /title="平台总览"/)
  assert.doesNotMatch(source, /<header className=/)
  assert.doesNotMatch(source, /<h1[^>]*>平台运营总览<\/h1>/)
})

test("platform overview keeps loading states local to modules", () => {
  assert.match(source, /function PanelLoading/)
  assert.match(source, /function KpiLoadingGrid/)
  assert.match(source, /function PanelError/)
  assert.match(source, /role="status"/)
  assert.match(source, /aria-busy="true"/)
  assert.doesNotMatch(source, /fetchPlatformOverview\(/)
  assert.match(source, /fetchPlatformOverviewMetrics/)
  assert.match(source, /fetchPlatformOverviewTopTenants/)
  assert.match(source, /fetchPlatformOverviewUsageTrend/)
  assert.match(source, /fetchPlatformOverviewEvents/)
  assert.match(source, /const \[metricsLoading, setMetricsLoading\]/)
  assert.match(source, /const \[metricsLoaded, setMetricsLoaded\] = useState\(false\)/)
  assert.match(source, /const \[tenantScaleLoading, setTenantScaleLoading\]/)
  assert.match(source, /const \[tenantScaleLoaded, setTenantScaleLoaded\] = useState\(false\)/)
  assert.match(source, /const \[usageTrendLoading, setUsageTrendLoading\]/)
  assert.match(source, /const \[usageTrendLoaded, setUsageTrendLoaded\] = useState\(false\)/)
  assert.match(source, /const \[eventsLoading, setEventsLoading\]/)
  assert.match(source, /const \[eventsLoaded, setEventsLoaded\] = useState\(false\)/)
  assert.match(source, /const \[opsLoading, setOpsLoading\]/)
  assert.match(source, /const \[opsLoaded, setOpsLoaded\] = useState\(false\)/)
  assert.match(source, /const loadMetrics = useCallback/)
  assert.match(source, /const loadTenantScale = useCallback/)
  assert.match(source, /const loadUsageTrend = useCallback/)
  assert.match(source, /const loadEvents = useCallback/)
  assert.match(source, /const loadOps = useCallback/)
  assert.match(source, /Promise\.allSettled\(\[[\s\S]*loadMetrics\(\),[\s\S]*loadTenantScale\(\),[\s\S]*loadUsageTrend\(\),[\s\S]*loadEvents\(\),[\s\S]*loadOps\(\),[\s\S]*\]\)/)
})

test("platform overview wires each visible module to its own loading props", () => {
  assert.match(source, /<TenantScalePanel data=\{tenantScale\} error=\{tenantScaleError\} initialLoading=\{tenantScaleLoading && !tenantScaleLoaded\} loading=\{tenantScaleLoading\}/)
  assert.match(source, /<TicketPanel data=\{metrics\} error=\{metricsError\} initialLoading=\{metricsLoading && !metricsLoaded\} loading=\{metricsLoading\}/)
  assert.match(source, /<KnowledgePanel data=\{metrics\} error=\{metricsError\} initialLoading=\{metricsLoading && !metricsLoaded\} loading=\{metricsLoading\}/)
  assert.match(source, /<UsageTrendPanel data=\{usageTrend\} error=\{usageTrendError\} initialLoading=\{usageTrendLoading && !usageTrendLoaded\} loading=\{usageTrendLoading\}/)
  assert.match(source, /<OpsHealthPanel ops=\{ops\} error=\{opsError\} initialLoading=\{opsLoading && !opsLoaded\} loading=\{opsLoading\}/)
  assert.match(source, /<EventPanel data=\{events\} error=\{eventsError\} initialLoading=\{eventsLoading && !eventsLoaded\} loading=\{eventsLoading\}/)
})
