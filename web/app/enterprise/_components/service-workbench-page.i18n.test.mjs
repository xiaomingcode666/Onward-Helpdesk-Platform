import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const locales = ["zh-CN", "en-US", "es-ES"]

function getMessage(source, key) {
  return key.split(".").reduce((current, part) => current?.[part], source)
}

test("enterprise service workbench uses i18n keys for static copy", async () => {
  const source = await readFile(new URL("./service-workbench-page.tsx", import.meta.url), "utf8")

  assert.doesNotMatch(source, /[\u4e00-\u9fff]/)

  const keys = new Set(
    [...source.matchAll(/\bt\("([^"]+)"/g)].map((match) => match[1])
  )

  for (const key of [
    "enterpriseWorkbench.queue.sla_risk.title",
    "enterpriseWorkbench.queue.unassigned.title",
    "enterpriseWorkbench.queue.unassigned.generalTitle",
    "enterpriseWorkbench.queue.pending.title",
    "enterpriseWorkbench.queue.processing.title",
    "enterpriseWorkbench.queue.awaiting_customer.title",
    "enterpriseWorkbench.queue.urgent.title",
    "enterpriseWorkbench.alerts.sla_breached",
    "enterpriseWorkbench.alerts.sla_risk",
    "enterpriseWorkbench.alerts.urgent",
    "enterpriseWorkbench.alerts.unassigned",
    "enterpriseWorkbench.alerts.suspended",
    "enterpriseWorkbench.alerts.usage",
    "enterpriseWorkbench.alerts.healthy",
    "enterpriseWorkbench.status.pending",
    "enterpriseWorkbench.status.accepted",
    "enterpriseWorkbench.status.pendingDispatch",
    "enterpriseWorkbench.status.pendingAssigneeAccept",
    "enterpriseWorkbench.status.processing",
    "enterpriseWorkbench.status.videoSupport",
    "enterpriseWorkbench.status.supplierSupport",
    "enterpriseWorkbench.status.waitingCustomer",
    "enterpriseWorkbench.status.closed",
    "enterpriseWorkbench.status.reopened",
    "enterpriseWorkbench.status.cancelled",
    "enterpriseWorkbench.status.active",
    "enterpriseWorkbench.status.ended",
    "enterpriseWorkbench.ticket.supportTeamPending",
  ]) {
    keys.add(key)
  }

  for (const locale of locales) {
    const messages = JSON.parse(
      await readFile(new URL(`../../../messages/${locale}.json`, import.meta.url), "utf8")
    )
    for (const key of keys) {
      assert.equal(
        typeof getMessage(messages, key),
        "string",
        `${locale} missing ${key}`
      )
    }
  }
})

test("enterprise service workbench keeps split APIs, pagination, and module loading states", async () => {
  const source = await readFile(new URL("./service-workbench-page.tsx", import.meta.url), "utf8")
  const apiSource = await readFile(new URL("../../../lib/api/enterprise-workbench.ts", import.meta.url), "utf8")

  assert.match(source, /fetchEnterpriseWorkbenchCore/)
  assert.match(source, /fetchEnterpriseWorkbenchCollaboration/)
  assert.match(source, /fetchEnterpriseWorkbenchResources/)
  assert.match(source, /fetchEnterpriseWorkbenchQueue/)
  assert.doesNotMatch(source, /fetchEnterpriseWorkbenchOverview/)
  assert.doesNotMatch(apiSource, /fetchEnterpriseWorkbenchOverview/)
  assert.doesNotMatch(apiSource, /\/workbench\/overview/)
  assert.match(source, /pageSize: WORKBENCH_QUEUE_PAGE_SIZE/)
  assert.match(source, /queueKey/)
  assert.match(apiSource, /apiGet<EnterpriseWorkbenchQueue>\("\/workbench\/queue", \{[\s\S]*page: query\.page,[\s\S]*page_size:/)
  assert.match(source, /const \[resourcesLoading, setResourcesLoading\] = useState\(true\)/)
  assert.match(source, /const \[reportOverviewLoading, setReportOverviewLoading\] = useState\(true\)/)
  assert.match(source, /const \[reportFailuresLoading, setReportFailuresLoading\] = useState\(true\)/)
  assert.match(source, /const \[keyWorkspaceLoading, setKeyWorkspaceLoading\] = useState\(true\)/)
  assert.match(source, /const API_KEY_USAGE_PANEL_LIMIT = 3/)
  assert.match(source, /pageSize: API_KEY_USAGE_PANEL_LIMIT/)
  assert.match(source, /WORKBENCH_VIEW_ROUTES/)
  assert.match(source, /if \(workbenchView === "queue"\) \{[\s\S]*void loadCollaboration\(\)/)
  assert.match(source, /if \(workbenchView === "overview"\) \{[\s\S]*void loadReportOverview\(\)[\s\S]*void loadReportFailures\(\)[\s\S]*void loadKeyWorkspace\(\)/)
  assert.match(source, /if \(workbenchView === "resources"\) \{[\s\S]*void loadCollaboration\(\)[\s\S]*void loadResources\(\)[\s\S]*void loadUpcomingMeetings\(\)/)
  assert.match(source, /const canShowAdminModules = !core \|\| !core\.scope\.restricted/)
  assert.match(source, /failuresLoading=\{reportFailuresLoading && !reportFailuresLoaded\}/)
  assert.match(source, /keyWorkspaceLoading=\{keyWorkspaceLoading && !keyWorkspaceLoaded\}/)
  assert.match(source, /overviewLoading=\{reportOverviewLoading && !reportOverviewLoaded\}/)
  assert.match(source, /loading=\{resourcesLoading && !resourcesLoaded\}/)
  assert.doesNotMatch(source, /failuresLoading=\{initialCoreLoading \|\|/)
  assert.doesNotMatch(source, /keyWorkspaceLoading=\{initialCoreLoading \|\|/)
  assert.doesNotMatch(source, /overviewLoading=\{initialCoreLoading \|\|/)
  assert.doesNotMatch(source, /loading=\{initialCoreLoading \|\| \(resourcesLoading/)
  assert.doesNotMatch(source, /pageSize:\s*100/)

  assert.match(source, /function PanelLoading/)
  assert.match(source, /role="status"/)
  assert.match(source, /aria-busy="true"/)
  assert.match(source, /<PanelLoading title=\{title\} label=\{t\("common\.loadingData"\)\} rows=\{4\} \/>/)
  assert.match(source, /<PanelSkeleton rows=\{5\} label=\{t\("common\.loadingData"\)\} \/>/)
  assert.doesNotMatch(source, /if \(loading\) return <Skeleton className="h-64 rounded-lg" \/>/)
})

test("enterprise service workbench keeps task queue copy concise", async () => {
  const source = await readFile(new URL("./service-workbench-page.tsx", import.meta.url), "utf8")

  assert.doesNotMatch(source, /queueDescription/)
  assert.doesNotMatch(source, /enterpriseWorkbench\.queue\.emptyDescription/)
  assert.doesNotMatch(source, /enterpriseWorkbench\.kpi\.[^"]*Meta/)
  assert.doesNotMatch(source, /enterpriseWorkbench\.quality\.(slaRiskMeta|aiSessions|newTodayMeta|pending)/)
  assert.doesNotMatch(source, />\{item\.meta\}</)
  assert.doesNotMatch(source, />\{meta\}</)

  for (const locale of locales) {
    const messages = JSON.parse(
      await readFile(new URL(`../../../messages/${locale}.json`, import.meta.url), "utf8")
    )
    const queue = getMessage(messages, "enterpriseWorkbench.queue")
    const kpi = getMessage(messages, "enterpriseWorkbench.kpi")
    const quality = getMessage(messages, "enterpriseWorkbench.quality")
    assert.equal(queue.defaultDescription, undefined, `${locale} should not keep queue.defaultDescription`)
    assert.equal(queue.emptyDescription, undefined, `${locale} should not keep queue.emptyDescription`)

    for (const key of ["sla_risk", "unassigned", "pending", "processing", "awaiting_customer", "urgent"]) {
      assert.equal(queue[key]?.description, undefined, `${locale} should not keep queue.${key}.description`)
    }
    for (const key of Object.keys(kpi)) {
      assert.equal(/Meta$/.test(key), false, `${locale} should not keep kpi.${key}`)
    }
    for (const key of ["slaRiskMeta", "aiSessions", "newTodayMeta", "pending"]) {
      assert.equal(quality[key], undefined, `${locale} should not keep quality.${key}`)
    }
  }
})

test("enterprise service workbench localizes legacy meeting statuses", async () => {
  const source = await readFile(new URL("./service-workbench-page.tsx", import.meta.url), "utf8")

  assert.match(source, /case "finished":\s+return t\("enterpriseWorkbench\.status\.ended"\)/)
})

test("knowledge-support workbench replaces product queue and device labels", async () => {
  const source = await readFile(new URL("./service-workbench-page.tsx", import.meta.url), "utf8")

  assert.match(source, /ticketOwnerText\(ticket: TicketListItem, t: Translate, hasDeviceConcept = true\)/)
  assert.match(source, /hasDeviceConcept \? "enterpriseWorkbench\.ticket\.productTeamPending" : "enterpriseWorkbench\.ticket\.supportTeamPending"/)
  assert.match(source, /queue\.key === "unassigned" && !hasDeviceConcept/)
  assert.match(source, /t\(hasDeviceConcept \? "enterpriseWorkbench\.ticketTable\.customerDevice" : "enterpriseWorkbench\.ticketTable\.customer"\)/)
  assert.match(source, /canShowAdminModules && hasDeviceConcept \? \(/)
})

test("enterprise service workbench maps product API keys to product display names", async () => {
  const source = await readFile(new URL("./service-workbench-page.tsx", import.meta.url), "utf8")

  assert.match(source, /getEnterpriseAIKeyDisplayName\(key\)/)
  assert.match(source, /getEnterpriseAIKeyProductMeta\(key\)/)
  assert.match(source, /keyProductMeta \?/)
  assert.doesNotMatch(source, /\{key\.name \|\| `Key #\$\{key\.id\}`\}/)
})

test("enterprise service workbench shows API key loading while requests are pending", async () => {
  const source = await readFile(new URL("./service-workbench-page.tsx", import.meta.url), "utf8")

  assert.match(source, /API_KEY_USAGE_TIMEOUT_MS/)
  assert.match(source, /function withTimeout/)
  assert.match(source, /Loader2Icon className="size-3\.5 animate-spin text-primary"/)
  assert.match(source, /aria-label=\{t\("enterpriseWorkbench\.apiKeys\.loading"\)\}/)
  assert.match(source, /setKeyWorkspaceLoading\(true\)/)
  assert.match(source, /workbenchView === "overview" && \(reportOverviewLoading \|\| reportFailuresLoading \|\| keyWorkspaceLoading\)/)
  assert.match(source, /disabled=\{pageRefreshing\}/)
})

test("enterprise service workbench hides the canonical forbidden response", async () => {
  const source = await readFile(new URL("./service-workbench-page.tsx", import.meta.url), "utf8")

  assert.match(source, /message\.includes\("do not have permission"\)/)
  assert.match(source, /function DailyUsageTrendWidget[\s\S]*if \(!loading && isPermissionDeniedMessage\(error\)\) return null/)
})

test("enterprise service workbench is split into route-level pages", async () => {
  const source = await readFile(new URL("./service-workbench-page.tsx", import.meta.url), "utf8")
  const aliasPage = await readFile(new URL("../workbench/page.tsx", import.meta.url), "utf8")
  const tasksPage = await readFile(new URL("../workbench/tasks/page.tsx", import.meta.url), "utf8")
  const overviewPage = await readFile(new URL("../workbench/overview/page.tsx", import.meta.url), "utf8")
  const resourcesPage = await readFile(new URL("../workbench/resources/page.tsx", import.meta.url), "utf8")

  assert.match(aliasPage, /to="\/enterprise\/workbench\/tasks"/)
  assert.match(tasksPage, /view="queue"/)
  assert.match(overviewPage, /view="overview"/)
  assert.match(resourcesPage, /view="resources"/)
  assert.match(source, /href: WORKBENCH_VIEW_ROUTES\.queue/)
  assert.match(source, /aria-current=\{active \? "page" : undefined\}/)
  assert.doesNotMatch(source, /useState<WorkbenchView>/)
  assert.doesNotMatch(source, /FilterTabs/)
})

test("enterprise service workbench clears loading states when module APIs throw", async () => {
  const source = await readFile(new URL("./service-workbench-page.tsx", import.meta.url), "utf8")
  const coreLoadStart = source.indexOf("const load = useCallback")
  const coreLoadEnd = source.indexOf("useEffect(() =>", coreLoadStart)
  const coreLoadSource = source.slice(coreLoadStart, coreLoadEnd)

  assert.match(source, /const loadCollaboration = useCallback\(async \(\) => \{[\s\S]*try \{[\s\S]*fetchEnterpriseWorkbenchCollaboration\(\)[\s\S]*\} catch \(error\) \{[\s\S]*setCollaborationError[\s\S]*\} finally \{[\s\S]*setCollaborationLoading\(false\)/)
  assert.match(source, /const loadResources = useCallback\(async \(\) => \{[\s\S]*try \{[\s\S]*fetchEnterpriseWorkbenchResources\(\)[\s\S]*\} catch \(error\) \{[\s\S]*setResourcesError[\s\S]*\} finally \{[\s\S]*setResourcesLoading\(false\)/)
  assert.match(source, /const loadReportOverview = useCallback\(async \(\) => \{[\s\S]*try \{[\s\S]*fetchReportsOverview\(\)[\s\S]*\} catch \(error\) \{[\s\S]*setReportOverviewError[\s\S]*\} finally \{[\s\S]*setReportOverviewLoading\(false\)/)
  assert.match(source, /const loadReportFailures = useCallback\(async \(\) => \{[\s\S]*try \{[\s\S]*fetchReportTopFailures[\s\S]*\} catch \(error\) \{[\s\S]*setReportFailuresError[\s\S]*\} finally \{[\s\S]*setReportFailuresLoading\(false\)/)
  assert.match(coreLoadSource, /fetchEnterpriseWorkbenchCore\(\)[\s\S]*setCoreError[\s\S]*setCoreLoading\(false\)/)
  assert.doesNotMatch(coreLoadSource, /void loadCollaboration\(\)/)
  assert.doesNotMatch(coreLoadSource, /void loadResources\(\)/)
  assert.doesNotMatch(coreLoadSource, /void loadReportOverview\(\)/)
  assert.doesNotMatch(coreLoadSource, /void loadKeyWorkspace\(\)/)
})
