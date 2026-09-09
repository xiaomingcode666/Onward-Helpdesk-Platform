import assert from "node:assert/strict"
import test from "node:test"

import { normalizePlatformOverview } from "./platform-overview-normalizer.ts"

test("normalizes a complete platform overview without availability warnings", () => {
  const overview = normalizePlatformOverview({
    generatedAt: "2026-08-12T10:00:00+08:00",
    tenants: { total: 2, active: 1, trial: 1, frozen: 0, expiringSoon: 1 },
    products: 3,
    devices: 4,
    tickets: { total: 8, open: 2, createdToday: 1, slaRisk: 0 },
    knowledge: { documents: 5, indexed: 4, pending: 1 },
    aiUsage: { requests: 9, tokens: 10, costAmount: 1.5 },
    meetings: { monthly: 3, active: 1 },
    topTenants: [],
    dailyUsage: [],
    recentEvents: [],
  })

  assert.equal(overview.tickets.open, 2)
  assert.deepEqual(overview.unavailableSections, [])
})

test("missing overview groups become renderable zero values with explicit warnings", () => {
  const overview = normalizePlatformOverview({ generatedAt: "2026-08-12T10:00:00+08:00" })

  assert.deepEqual(overview.tickets, { total: 0, open: 0, createdToday: 0, slaRisk: 0 })
  assert.deepEqual(overview.knowledge, { documents: 0, indexed: 0, pending: 0 })
  assert.deepEqual(overview.topTenants, [])
  assert.deepEqual(overview.dailyUsage, [])
  assert.deepEqual(overview.recentEvents, [])
  assert.ok(overview.unavailableSections.includes("工单统计"))
  assert.ok(overview.unavailableSections.includes("知识统计"))
  assert.ok(overview.unavailableSections.includes("模型用量"))
  assert.equal(overview.unavailableSections.includes("AI 用量"), false)
  assert.ok(overview.unavailableSections.includes("调用趋势"))
})

test("targeted overview sections ignore intentionally omitted groups", () => {
  const overview = normalizePlatformOverview(
    {
      generatedAt: "2026-08-12T10:00:00+08:00",
      topTenants: [{ tenantId: 1, tenantName: "Atlas", devices: 4 }],
    },
    ["租户规模"],
  )

  assert.deepEqual(overview.unavailableSections, [])
  assert.equal(overview.topTenants.length, 1)
  assert.deepEqual(overview.tickets, { total: 0, open: 0, createdToday: 0, slaRisk: 0 })
})

test("invalid rows are discarded and numeric strings remain compatible", () => {
  const overview = normalizePlatformOverview({
    tenants: { total: "2", active: "1", trial: 0, frozen: 1, expiringSoon: 0 },
    tickets: { total: "8", open: "2", createdToday: "1", slaRisk: "0" },
    knowledge: { documents: "5", indexed: "4", pending: "1" },
    aiUsage: { requests: "9", tokens: "10", costAmount: "1.5" },
    meetings: { monthly: "3", active: "1" },
    topTenants: [null, { tenantId: "1", tenantName: "Atlas", devices: "4" }],
    dailyUsage: [{ date: "2026-08-12", aiRequests: "9" }, { aiRequests: 3 }],
    recentEvents: [null, { id: "7", action: "tenant.updated" }],
  })

  assert.equal(overview.tickets.total, 8)
  assert.equal(overview.aiUsage.costAmount, 1.5)
  assert.equal(overview.topTenants.length, 1)
  assert.equal(overview.dailyUsage.length, 1)
  assert.equal(overview.recentEvents.length, 1)
})
