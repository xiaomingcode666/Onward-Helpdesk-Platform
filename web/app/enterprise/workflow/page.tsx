"use client"

import {
  ActivityIcon,
  ArrowRightIcon,
  BookOpenIcon,
  CheckCircle2Icon,
  DatabaseIcon,
  HeadphonesIcon,
  Layers3Icon,
  ListChecksIcon,
  Loader2Icon,
  LockKeyholeIcon,
  MessageSquareTextIcon,
  PlusIcon,
  RouteIcon,
  ScanLineIcon,
  ShieldCheckIcon,
  TicketIcon,
  UsersIcon,
  VideoIcon,
  WorkflowIcon,
  WrenchIcon,
  type LucideIcon,
} from "lucide-react"
import { useRouter, useSearchParams } from "next/navigation"
import { useCallback, useEffect, useMemo, useState } from "react"
import { toast } from "sonner"

import { PageShell, RailopsButton, SearchField, StandardModal, StatusTag, UnderlineTabs, type RailopsTabItem, type StatusTagTone } from "@railops/ui"
import { AIWorkflowRunsWorkspace } from "@/app/dashboard/ai-workflow-runs/_components/workspace"
import { EnterpriseWorkflowDetailPage } from "@/app/enterprise/workflow/[workflowId]/template-page"
import { EnterpriseAIFeatureGuard } from "@/components/enterprise/ai-feature-guard"
import { DashboardListPage, type DashboardListColumn } from "@/components/dashboard/list"
import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { CanUseButton } from "@/components/layout/permission-guard"
import { Input } from "antd"
import { useI18n } from "@/i18n/provider"

import { type AIWorkflow } from "@/lib/api/admin"
import {
  createEnterpriseAIWorkflowTemplate,
  fetchEnterpriseAIWorkflowAdoption,
  fetchEnterpriseAIWorkflowSummary,
  fetchEnterpriseAIWorkflows,
} from "@/lib/api/enterprise-ai"
import { readSession } from "@/lib/auth"
import {
  buildEnterpriseWorkflowPath,
  normalizeEnterpriseDetailId,
} from "@/lib/enterprise-detail-route"
import { formatDateTime } from "@/lib/utils"
import {
  getWorkflowProductProfile,
  getWorkflowServiceBlueprint,
  isKnowledgeSupportWorkflowDefinition,
  type WorkflowServiceBlueprint,
} from "./[workflowId]/workflow-product-utils"

const workflowListI18nPrefix = "workflowExtract.enterpriseWorkflowList."
type WorkflowListT = ReturnType<typeof useI18n>
const wl = (t: WorkflowListT, key: string, values?: Record<string, string | number>) => t(`${workflowListI18nPrefix}${key}`, values)

function toNumberMap(record?: Record<string, number>) {
  const ret = new Map<number, number>()
  Object.entries(record ?? {}).forEach(([key, value]) => {
    const id = Number(key)
    if (Number.isFinite(id)) ret.set(id, Number(value) || 0)
  })
  return ret
}

type WorkflowGovernanceState = {
  label: string
  detail: string
  tone: StatusTagTone
  needsAttention: boolean
}

function workflowKind(item: AIWorkflow, t: WorkflowListT) {
  if (item.scope === "platform") {
    return { label: wl(t, "kind.platform.label"), detail: wl(t, "kind.platform.detail"), variant: "default" as const }
  }
  return { label: wl(t, "kind.tenant.label"), detail: wl(t, "kind.tenant.detail"), variant: "secondary" as const }
}

const workflowBlueprintOrder: Record<WorkflowServiceBlueprint, number> = {
  basic_ai: 1,
  dispatch_only: 2,
  device_ai: 3,
  ai_human: 4,
}

function workflowTemplateRank(item: AIWorkflow) {
  return workflowBlueprintOrder[getWorkflowServiceBlueprint(item.draftDefinition)]
}

function enterpriseTemplateName(item: AIWorkflow, t: WorkflowListT) {
  return item.name.replace(/^平台\s*/, wl(t, "templateName.enterprisePrefix")).replace(/^企业默认/, wl(t, "templateName.enterpriseStandard"))
}

function workflowServiceCapability(item: AIWorkflow, t: WorkflowListT) {
  const blueprint = getWorkflowServiceBlueprint(item.draftDefinition)
  switch (blueprint) {
    case "ai_human":
      return {
        label: wl(t, "capability.aiHuman"),
        icon: HeadphonesIcon,
        variant: "outline" as const,
      }
    case "dispatch_only":
      return {
        label: wl(t, "capability.dispatchOnly"),
        icon: RouteIcon,
        variant: "secondary" as const,
      }
    case "device_ai":
      return {
        label: wl(t, "capability.deviceAi"),
        icon: WrenchIcon,
        variant: "secondary" as const,
      }
    default:
      return {
        label: wl(t, "capability.basicAi"),
        icon: MessageSquareTextIcon,
        variant: "secondary" as const,
      }
  }
}

type WorkflowExperienceProfile = {
  title: string
  subtitle: string
  bestFor: string
  impact: string
  path: string[]
  accentClass: string
  icon: LucideIcon
}

function workflowExperienceProfile(item: AIWorkflow, t: WorkflowListT): WorkflowExperienceProfile {
  const blueprint = getWorkflowServiceBlueprint(item.draftDefinition)
  const profile = getWorkflowProductProfile(item.draftDefinition)
  if (isKnowledgeSupportWorkflowDefinition(item.draftDefinition)) {
    return {
      title: profile.title,
      subtitle: wl(t, "profile.knowledgeSupport.subtitle"),
      bestFor: wl(t, "profile.knowledgeSupport.bestFor"),
      impact: wl(t, "profile.knowledgeSupport.impact"),
      path: ["profile.path.customerQuestion", "profile.path.searchKnowledge", "profile.path.generateAnswer", "profile.path.handoff", "profile.path.end"].map((key) => wl(t, key)),
      accentClass: "border-emerald-200 bg-emerald-50 text-emerald-700",
      icon: BookOpenIcon,
    }
  }
  switch (blueprint) {
    case "ai_human":
      return {
        title: profile.title,
        subtitle: wl(t, "profile.aiHuman.subtitle"),
        bestFor: wl(t, "profile.aiHuman.bestFor"),
        impact: wl(t, "profile.aiHuman.impact"),
        path: ["profile.path.scan", "profile.path.identifyDevice", "profile.path.troubleshoot", "profile.path.handoff", "profile.path.ticketVideo"].map((key) => wl(t, key)),
        accentClass: "border-emerald-200 bg-emerald-50 text-emerald-700",
        icon: UsersIcon,
      }
    case "dispatch_only":
      return {
        title: profile.title,
        subtitle: wl(t, "profile.dispatchOnly.subtitle"),
        bestFor: wl(t, "profile.dispatchOnly.bestFor"),
        impact: wl(t, "profile.dispatchOnly.impact"),
        path: ["profile.path.customerEntry", "profile.path.actionIdentify", "profile.path.serviceMenu", "profile.path.ticketHuman", "profile.path.end"].map((key) => wl(t, key)),
        accentClass: "border-cyan-200 bg-cyan-50 text-cyan-700",
        icon: RouteIcon,
      }
    case "device_ai":
      return {
        title: profile.title,
        subtitle: wl(t, "profile.deviceAi.subtitle"),
        bestFor: wl(t, "profile.deviceAi.bestFor"),
        impact: wl(t, "profile.deviceAi.impact"),
        path: ["profile.path.scan", "profile.path.identifyDevice", "profile.path.searchKnowledge", "profile.path.diagnosis", "profile.path.end"].map((key) => wl(t, key)),
        accentClass: "border-sky-200 bg-sky-50 text-sky-700",
        icon: WrenchIcon,
      }
    default:
      return {
        title: profile.title,
        subtitle: wl(t, "profile.basicAi.subtitle"),
        bestFor: wl(t, "profile.basicAi.bestFor"),
        impact: wl(t, "profile.basicAi.impact"),
        path: ["profile.path.customerQuestion", "profile.path.searchKnowledge", "profile.path.generateAnswer", "profile.path.followUp", "profile.path.end"].map((key) => wl(t, key)),
        accentClass: "border-violet-200 bg-violet-50 text-violet-700",
        icon: BookOpenIcon,
      }
  }
}

function getWorkflowLaunchSteps(t: WorkflowListT, knowledgeSupport = false) {
  return [
    { label: wl(t, "launch.selectTemplate"), icon: ListChecksIcon },
    { label: wl(t, "launch.adjustFlow"), icon: RouteIcon },
    { label: wl(t, "launch.testRun"), icon: ActivityIcon },
    { label: wl(t, "launch.publishVersion"), icon: ShieldCheckIcon },
    { label: wl(t, knowledgeSupport ? "launch.enableReception" : "launch.enableProducts"), icon: Layers3Icon },
  ]
}

function workflowGovernanceState(
  item: AIWorkflow,
  adopted: number,
  stableAdopted: number,
  sourceStableVersionId: number,
  t: WorkflowListT,
): WorkflowGovernanceState {
  if (item.currentStableVersionId <= 0) {
    return {
      label: wl(t, "governance.pendingPublish.label"),
      detail: wl(t, "governance.pendingPublish.detail"),
      tone: "warning",
      needsAttention: true,
    }
  }
  if (
    item.scope === "tenant" &&
    item.sourceVersionId > 0 &&
    sourceStableVersionId > 0 &&
    item.sourceVersionId !== sourceStableVersionId
  ) {
    return {
      label: wl(t, "governance.sourceUpdated.label"),
      detail: wl(t, "governance.sourceUpdated.detail"),
      tone: "warning",
      needsAttention: true,
    }
  }
  if (adopted > stableAdopted) {
    return {
      label: wl(t, "governance.upgradePending.label"),
      detail: wl(t, "governance.upgradePending.detail", { count: adopted - stableAdopted }),
      tone: "warning",
      needsAttention: true,
    }
  }
  if (adopted === 0) {
    return item.scope === "platform"
      ? {
          label: wl(t, "governance.pendingPilot.label"),
          detail: wl(t, "governance.pendingPilot.detail"),
          tone: "blue",
          needsAttention: true,
        }
      : {
          label: wl(t, "governance.pendingReview.label"),
          detail: wl(t, "governance.pendingReview.detail"),
          tone: "neutral",
          needsAttention: true,
        }
  }
  return {
    label: wl(t, "governance.aligned.label"),
    detail: wl(t, "governance.aligned.detail"),
    tone: "blue",
    needsAttention: false,
  }
}

export default function EnterpriseWorkflowPage() {
  const searchParams = useSearchParams()
  const workflowId = normalizeEnterpriseDetailId(searchParams.get("workflowId"))
  return (
    <EnterpriseAIFeatureGuard title="AI Workflow">
      {workflowId > 0
        ? <EnterpriseWorkflowDetailPage workflowId={workflowId} defaultPreviewOpen />
        : <EnterpriseWorkflowListPage />}
    </EnterpriseAIFeatureGuard>
  )
}

function WorkflowCreateIntro({ t, knowledgeSupport }: { t: WorkflowListT; knowledgeSupport: boolean }) {
  const workflowLaunchSteps = getWorkflowLaunchSteps(t, knowledgeSupport)
  return (
    <div className="space-y-4 border-y py-4">
      <div>
        <div className="text-sm font-semibold">{wl(t, knowledgeSupport ? "createIntro.knowledgeTitle" : "createIntro.title")}</div>
        <p className="mt-1 text-xs leading-5 text-muted-foreground">
          {wl(t, knowledgeSupport ? "createIntro.knowledgeDescription" : "createIntro.description")}
        </p>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <StatusTag tone="blue">
          <ScanLineIcon className="size-3.5" />{wl(t, "createIntro.customerEntry")}
        </StatusTag>
        <StatusTag tone="success">
          <DatabaseIcon className="size-3.5" />{wl(t, knowledgeSupport ? "createIntro.tenantKnowledge" : "createIntro.productKnowledge")}
        </StatusTag>
        <StatusTag tone="warning">
          <TicketIcon className="size-3.5" />{wl(t, "createIntro.ticketClosure")}
        </StatusTag>
        <StatusTag tone="blue">
          <VideoIcon className="size-3.5" />{wl(t, "createIntro.video")}
        </StatusTag>
      </div>
      <div>
        <ol className="grid gap-2 sm:grid-cols-2 xl:grid-cols-5">
          {workflowLaunchSteps.map((step, index) => {
            const Icon = step.icon
            return (
              <li key={step.label} className="relative min-h-20 rounded-md border bg-muted/20 p-3">
                {index < workflowLaunchSteps.length - 1 ? (
                  <span className="absolute right-[-0.55rem] top-1/2 z-10 hidden h-px w-3 bg-border sm:block" />
                ) : null}
                <div className="flex items-center justify-between gap-2">
                  <span className="flex size-7 items-center justify-center rounded-md bg-background text-primary shadow-sm">
                    <Icon className="size-3.5" />
                  </span>
                  <span className="font-mono text-rhd-xs text-muted-foreground">0{index + 1}</span>
                </div>
                <div className="mt-2 text-sm font-semibold">{step.label}</div>
              </li>
            )
          })}
        </ol>
      </div>
    </div>
  )
}

function getWorkflowTabs(t: WorkflowListT): RailopsTabItem[] {
  return [
    { value: "templates", label: wl(t, "tabs.templates"), icon: <WorkflowIcon /> },
    { value: "runs", label: wl(t, "tabs.runs"), icon: <ActivityIcon /> },
  ]
}

function EnterpriseWorkflowListPage() {
  const t = useI18n()
  const router = useRouter()
  const session = readSession()
  const tenantAIEnabled = session?.featureFlags?.ai !== false
  const knowledgeSupport = session?.featureFlags?.device === false
  const canCreate = tenantAIEnabled && CanUseButton("aiWorkflow.update", session?.permissions)
  const [adoption, setAdoption] = useState<Map<number, number>>(new Map())
  const [stableAdoption, setStableAdoption] = useState<Map<number, number>>(new Map())
  const [stableVersionByWorkflow, setStableVersionByWorkflow] = useState<Map<number, number>>(new Map())
  const [platformTemplates, setPlatformTemplates] = useState<AIWorkflow[]>([])
  const [summaryLoading, setSummaryLoading] = useState(true)
  const [workflowSearchDraft, setWorkflowSearchDraft] = useState("")
  const [workflowSearchQuery, setWorkflowSearchQuery] = useState("")
  const [workflowListReloadKey, setWorkflowListReloadKey] = useState(0)
  const [createOpen, setCreateOpen] = useState(false)
  const [workflowTab, setWorkflowTab] = useState<"templates" | "runs">("templates")
  const [creating, setCreating] = useState(false)
  const [sourceVersionId, setSourceVersionId] = useState(0)
  const [createName, setCreateName] = useState("")

  const loadSummary = useCallback(async () => {
    setSummaryLoading(true)
    try {
      const nextSummary = await fetchEnterpriseAIWorkflowSummary()
      setStableVersionByWorkflow(toNumberMap(nextSummary.stableVersionByWorkflow))
      setPlatformTemplates(nextSummary.platformTemplates
        .filter((item) => item.currentStableVersionId > 0 && (tenantAIEnabled || getWorkflowServiceBlueprint(item.draftDefinition) === "dispatch_only"))
        .sort((left, right) => workflowTemplateRank(left) - workflowTemplateRank(right)))
    } catch (error) {
      setPlatformTemplates([])
      toast.error(error instanceof Error ? error.message : wl(t, "errors.summaryLoadFailed"))
    } finally {
      setSummaryLoading(false)
    }
  }, [t, tenantAIEnabled])

  const fetchWorkflowList = useCallback(async (query?: Record<string, string | number | undefined>) => {
    const page = await fetchEnterpriseAIWorkflows(query)
    const workflowIds = page.results.map((item) => item.id).filter((id) => id > 0)
    if (workflowIds.length === 0) {
      setAdoption(new Map())
      setStableAdoption(new Map())
      return page
    }
    try {
      const status = await fetchEnterpriseAIWorkflowAdoption(workflowIds)
      setAdoption(toNumberMap(status.adoption))
      setStableAdoption(toNumberMap(status.stableAdoption))
      const nextStableVersionByWorkflow = toNumberMap(status.stableVersionByWorkflow)
      setStableVersionByWorkflow((current) => new Map([...current, ...nextStableVersionByWorkflow]))
    } catch (error) {
      setAdoption(new Map())
      setStableAdoption(new Map())
      toast.error(error instanceof Error ? error.message : wl(t, "errors.adoptionLoadFailed"))
    }
    return page
  }, [t])

  const fetchFilteredWorkflowList = useCallback(async (query?: Record<string, string | number | undefined>) => (
    fetchWorkflowList({
      ...query,
      name: workflowSearchQuery || undefined,
    })
  ), [fetchWorkflowList, workflowSearchQuery])

  // Debounced live search, consistent with the ticket center
  useEffect(() => {
    const timer = window.setTimeout(() => setWorkflowSearchQuery(workflowSearchDraft.trim()), 320)
    return () => window.clearTimeout(timer)
  }, [workflowSearchDraft])

  function selectSourceTemplate(item: AIWorkflow) {
    setSourceVersionId(item.currentStableVersionId)
    setCreateName(enterpriseTemplateName(item, t))
  }

  function openCreateDialog() {
    const source = platformTemplates[0]
    if (!source) {
      toast.error(wl(t, "errors.noStableTemplate"))
      return
    }
    selectSourceTemplate(source)
    setCreateOpen(true)
  }

  async function createWorkflow() {
    if (creating || sourceVersionId <= 0 || !createName.trim()) return
    setCreating(true)
    try {
      const created = await createEnterpriseAIWorkflowTemplate({
        name: createName.trim(),
        description: "",
        sourceVersionId,
      })
      toast.success(wl(t, "messages.created"))
      setCreateOpen(false)
      router.push(buildEnterpriseWorkflowPath(created.id))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : wl(t, "errors.createFailed"))
    } finally {
      setCreating(false)
    }
  }

  useEffect(() => {
    void loadSummary()
  }, [loadSummary])

  const columns = useMemo<DashboardListColumn<AIWorkflow>[]>(
    () => [
      {
        key: "template",
        label: wl(t, "columns.workflow"),
        className: "w-[170px] max-w-[170px] sm:w-[260px] sm:max-w-[260px] xl:w-[300px] xl:max-w-[300px]",
        render: (item) => (
          <div className="flex items-start gap-3">
            <span className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
              <WorkflowIcon className="size-4" />
            </span>
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <span className="truncate font-medium">{item.name}</span>
                {item.locked ? <LockKeyholeIcon className="size-3.5 shrink-0 text-muted-foreground" /> : null}
              </div>
            </div>
          </div>
        ),
      },
      {
        key: "scope",
        label: wl(t, "columns.source"),
        className: "hidden w-[100px] max-w-[100px] md:table-cell",
        render: (item) => {
          const kind = workflowKind(item, t)
          return <StatusTag tone={kind.variant === "default" ? "blue" : "neutral"}>{kind.label}</StatusTag>
        },
      },
      {
        key: "capability",
        label: wl(t, "columns.capability"),
        className: "hidden w-[135px] max-w-[135px] lg:table-cell",
        render: (item) => {
          const capability = workflowServiceCapability(item, t)
          const Icon = capability.icon
          return (
            <StatusTag tone="neutral" className="inline-flex max-w-full items-center gap-1.5 whitespace-nowrap">
              <Icon className="size-3.5 shrink-0" />
              <span>{capability.label}</span>
            </StatusTag>
          )
        },
      },
      {
        key: "status",
        label: wl(t, "columns.status"),
        className: "w-[100px] max-w-[100px] sm:w-[150px] sm:max-w-[150px]",
        render: (item) => {
          const state = workflowGovernanceState(
            item,
            adoption.get(item.id) ?? 0,
            stableAdoption.get(item.id) ?? 0,
            stableVersionByWorkflow.get(item.sourceWorkflowId) ?? 0,
            t,
          )
          return (
            <div className="flex flex-wrap gap-1.5">
              <StatusTag tone={item.currentStableVersionId > 0 ? "blue" : "neutral"}>
                {item.currentStableVersionId > 0 ? wl(t, "status.published") : wl(t, "status.draft")}
              </StatusTag>
              {state.needsAttention ? (
                <StatusTag tone={state.tone}>{state.label}</StatusTag>
              ) : null}
            </div>
          )
        },
      },
      {
        key: "adoption",
        label: wl(t, "columns.adoption"),
        className: "w-[70px] max-w-[70px] sm:w-[110px] sm:max-w-[110px]",
        render: (item) => {
          const adopted = adoption.get(item.id) ?? 0
          const stableAdopted = stableAdoption.get(item.id) ?? 0
          return (
            <div className="space-y-1">
              <div className="flex items-center gap-1.5 font-medium">
                {adopted > 0 ? <ShieldCheckIcon className="size-3.5 shrink-0 text-primary" /> : null}
                <span className="sm:hidden">{adopted}</span>
                <span className="hidden sm:inline">{adopted > 0 ? wl(t, "adoption.products", { count: adopted }) : wl(t, "adoption.notApplied")}</span>
              </div>
              {adopted > stableAdopted ? (
                <div className="text-xs text-amber-700">
                  <span className="sm:hidden">{wl(t, "adoption.pendingShort", { count: adopted - stableAdopted })}</span>
                  <span className="hidden sm:inline">{wl(t, "adoption.pending", { count: adopted - stableAdopted })}</span>
                </div>
              ) : null}
            </div>
          )
        },
      },
      {
        key: "updated",
        label: wl(t, "columns.updatedAt"),
        className: "hidden w-[135px] max-w-[135px] xl:table-cell",
        render: (item) => <span className="text-sm">{formatDateTime(item.updatedAt)}</span>,
      },
      {
        key: "action",
        label: <span className="sr-only">{wl(t, "columns.actions")}</span>,
        className: "hidden w-28 max-w-28 text-right sm:table-cell",
        render: (item) => (
          <RailopsButton
            size="small"
            onClick={(event) => {
              event.stopPropagation()
              router.push(buildEnterpriseWorkflowPath(item.id))
            }}
          >
            {item.scope === "platform" ? wl(t, "actions.viewPlan") : wl(t, "actions.editFlow")}
            <ArrowRightIcon />
          </RailopsButton>
        ),
      },
    ],
    [adoption, router, stableAdoption, stableVersionByWorkflow, t],
  )
  const workflowTabs = useMemo(() => getWorkflowTabs(t), [t])

  return (
    <PageShell
      title={wl(t, "pageTitle")}
      breadcrumb={useRouteBreadcrumbItems()}
      className="rhd-railops-workflow-page"
      actions={canCreate ? (
        <RailopsButton variant="primary" onClick={openCreateDialog} disabled={summaryLoading || platformTemplates.length === 0}>
          <PlusIcon />{wl(t, "actions.createFromTemplate")}
        </RailopsButton>
      ) : null}
    >

      <div className="rhd-railops-workflow-tabs space-y-3">
        <div className="flex flex-col gap-3 pb-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="rhd-railops-workflow-tabs-list min-w-0 lg:flex-1">
            <UnderlineTabs
              ariaLabel={wl(t, "tabs.ariaLabel")}
              items={workflowTabs}
              value={workflowTab}
              onChange={(value) => setWorkflowTab(value as "templates" | "runs")}
            />
          </div>
          <SearchField
            allowClear
            aria-label={wl(t, "search.ariaLabel")}
            className="rhd-railops-search-standard"
            placeholder={wl(t, "search.placeholder")}
            value={workflowSearchDraft}
            onChange={(event) => setWorkflowSearchDraft(event.target.value)}
          />
        </div>

        {workflowTab === "templates" ? (
          <div className="rhd-railops-workflow-content mt-0">
          <DashboardListPage<AIWorkflow>
            key={`workflow-list-${workflowSearchQuery}-${workflowListReloadKey}`}
            layout="fragment"
            showToolbar={false}
            reloadKey={workflowListReloadKey}
            pageSize={20}
            fetchList={fetchFilteredWorkflowList}
            columns={columns}
            getItemId={(item) => item.id}
            getRowClassName={() => "cursor-pointer"}
            onRowClick={(item) => router.push(buildEnterpriseWorkflowPath(item.id))}
            labels={{
              refresh: wl(t, "listLabels.refresh"),
              query: wl(t, "listLabels.query"),
              loading: wl(t, "listLabels.loading"),
              empty: wl(t, "listLabels.empty"),
              loadFailed: wl(t, "listLabels.loadFailed"),
            }}
          />
          </div>
        ) : null}

        {workflowTab === "runs" ? (
          <div className="rhd-railops-workflow-content mt-0">
          <AIWorkflowRunsWorkspace layout="fragment" />
          </div>
        ) : null}
      </div>

      <StandardModal
        open={createOpen}
        onCancel={() => { if (!creating) setCreateOpen(false) }}
        title={<><WorkflowIcon className="size-4 text-primary" />{wl(t, "createModal.title")}</>}
        width={1024}
        rootClassName="rhd-railops-scrollable-modal"
        footer={
          <>
            <RailopsButton onClick={() => { if (!creating) setCreateOpen(false) }}>{wl(t, "actions.cancel")}</RailopsButton>
            <RailopsButton variant="primary" onClick={() => void createWorkflow()} disabled={creating || sourceVersionId <= 0 || !createName.trim()}>
              {creating ? <Loader2Icon className="animate-spin" /> : <ArrowRightIcon />}
              {wl(t, "actions.startConfigure")}
            </RailopsButton>
          
          </>
        }
      >
          <WorkflowCreateIntro t={t} knowledgeSupport={knowledgeSupport} />
          <div className="text-sm font-medium">{wl(t, "createModal.recommendedStart")}</div>
          <div className="grid gap-3 lg:grid-cols-3">
            {platformTemplates.map((item) => {
              const profile = workflowExperienceProfile(item, t)
              const Icon = profile.icon
              const selected = sourceVersionId === item.currentStableVersionId
              return (
                <button
                  key={item.id}
                  type="button"
                  aria-pressed={selected}
                  className={`rhd-railops-workflow-template-card flex min-h-60 flex-col rounded-lg border p-4 text-left transition-colors ${selected ? "border-primary bg-primary/5" : "hover:border-foreground/30"}`}
                  onClick={() => selectSourceTemplate(item)}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className={`flex size-9 items-center justify-center rounded-md border ${selected ? "border-primary bg-primary text-primary-foreground" : profile.accentClass}`}>
                      <Icon className="size-4" />
                    </span>
                    {selected ? <CheckCircle2Icon className="size-4 text-primary" /> : <StatusTag tone="neutral">{wl(t, "createModal.canPilot")}</StatusTag>}
                  </div>
                  <div className="mt-4 text-base font-semibold">{profile.title}</div>
                  <p className="mt-2 text-xs leading-5 text-muted-foreground">{profile.subtitle}</p>
                  <div className="mt-3 rounded-md border bg-muted/20 p-2 text-xs leading-5 text-muted-foreground">
                    {wl(t, "createModal.bestFor", { value: profile.bestFor })}
                  </div>
                  <div className="mt-3 flex flex-wrap gap-1.5">
                    {profile.path.map((step) => <StatusTag key={step} tone="neutral">{step}</StatusTag>)}
                  </div>
                  <div className="mt-auto pt-4 text-xs font-medium text-foreground">{profile.impact}</div>
                </button>
              )
            })}
          </div>
          <div className="grid gap-3">
            <div className="space-y-2">
              <label htmlFor="new-workflow-name" className="text-sm font-medium">{wl(t, "createModal.workflowName")}</label>
              <Input id="new-workflow-name" value={createName} onChange={(event) => setCreateName(event.target.value)} />
            </div>
          </div>
          
      </StandardModal>
    </PageShell>
  )
}
