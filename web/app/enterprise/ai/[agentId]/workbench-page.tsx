"use client"

import { translateCurrentMessage } from "@/i18n/messages"
import {
  ArrowLeftIcon,
  CheckIcon,
  CircleAlertIcon,
  HistoryIcon,
  Loader2Icon,
  PackageIcon,
  PlayIcon,
  PlusIcon,
  RefreshCwIcon,
  RotateCcwIcon,
  SendIcon,
  ShieldCheckIcon,
  WorkflowIcon,
  XIcon,
} from "lucide-react"
import Link from "next/link"
import { useParams, useRouter, useSearchParams } from "next/navigation"
import { useCallback, useEffect, useMemo, useState } from "react"
import { toast } from "sonner"
import { Input } from "antd"

import {
  AIAgentConfigWorkbench,
  type AIAgentWorkbenchSection,
} from "@/app/dashboard/ai-agents/_components/config-workbench"
import {
  DataTable,
  IconButton,
  RailopsButton,
  StandardModal,
  StatusTag,
  UnderlineTabs,
  type RailopsTabItem,
} from "@railops/ui"
import type { StatusTagTone } from "@railops/ui"
import { AIWorkflowRunsWorkspace } from "@/app/dashboard/ai-workflow-runs/_components/workspace"
import { useConfirm } from "@/components/confirm-provider"
import { CanUseButton } from "@/components/layout/permission-guard"
import { RouteBreadcrumbs } from "@/components/layout/route-breadcrumbs"
import { OptionCombobox } from "@/components/option-combobox"
import {
  fetchAIAgent,
  fetchAIAgentWorkflow,
  updateAIAgent,
  type AIAgent,
  type AIWorkflow,
  type AIWorkflowDefinition,
  type AIWorkflowVersion,
} from "@/lib/api/admin"
import {
  approveAIAgentRelease,
  bindAIAgentWorkflowVersion,
  createAIAgentRelease,
  deployAIAgentRelease,
  fetchAIAgentReleases,
  fetchEnterpriseAIAgents,
  fetchEnterpriseAIWorkflows,
  fetchEnterpriseAIWorkflowVersions,
  rejectAIAgentRelease,
  rollbackAIAgentRelease,
  submitAIAgentReleaseReview,
  type AIAgentRelease,
} from "@/lib/api/enterprise-ai"
import {
  getEnterpriseAICapabilities,
  type EnterpriseAICapability,
} from "@/lib/api/enterprise-models"
import { readSession } from "@/lib/auth"
import {
  buildEnterpriseAIPath,
  buildEnterpriseWorkflowPath,
} from "@/lib/enterprise-detail-route"
import { Status } from "@/lib/generated/enums"
import { formatDateTime } from "@/lib/utils"
function ee(key: string, values?: Record<string, unknown>) {
  if (!values) {
    return translateCurrentMessage(`enterpriseExtract.${key}`)
  }
  return translateCurrentMessage(
    `enterpriseExtract.${key}`,
    Object.fromEntries(
      Object.entries(values).map(([name, value]) => [
        name,
        typeof value === "string" || typeof value === "number" ? value : String(value ?? ""),
      ]),
    ),
  )
}


type WorkbenchView = "configure" | "release" | "runs"

type OptionPageState = {
  page: number
  limit: number
  total: number
}

const OPTION_PAGE_SIZE = 50
const emptyOptionPage: OptionPageState = { page: 1, limit: OPTION_PAGE_SIZE, total: 0 }

function unwrapEnterpriseAgentPage(
  response: Awaited<ReturnType<typeof fetchEnterpriseAIAgents>>,
  fallbackMessage = ee("aiWorkbench.text001"),
) {
  if (!response.success || !response.data) {
    throw new Error(response.error?.message || fallbackMessage)
  }
  return response.data
}

const workbenchSections = new Set<AIAgentWorkbenchSection>([
  "basic",
  "model",
  "knowledge",
  "workflow",
  "skills",
  "tools",
  "handoff",
])

function workflowVersionCapabilityLabel(definition: AIWorkflowDefinition) {
  return definition.nodes.some((node) => node.type === "handoff_to_human")
    ? ee("aiWorkbench.text002")
    : ee("aiWorkbench.text003")
}

function normalizeView(value: string | null): WorkbenchView {
  if (value === "release" || value === "runs") return value
  return "configure"
}

function normalizeSection(value: string | null): AIAgentWorkbenchSection {
  return workbenchSections.has(value as AIAgentWorkbenchSection)
    ? (value as AIAgentWorkbenchSection)
    : "basic"
}

function releaseReviewTone(status: string): StatusTagTone {
  if (status === "approved") return "blue"
  if (status === "rejected") return "error"
  return "neutral"
}

function deploymentLabel(status: string) {
  if (status === "active") return ee("aiWorkbench.text004")
  if (status === "retired") return ee("aiWorkbench.text005")
  if (status === "rolled_back") return ee("aiWorkbench.text006")
  return ee("aiWorkbench.text007")
}

function deploymentTone(status: string): StatusTagTone {
  if (status === "active") return "blue"
  if (status === "rolled_back") return "error"
  return "neutral"
}

function shortHash(value: string) {
  return value ? value.slice(0, 10) : "-"
}

function optionPageCount(page: OptionPageState) {
  return Math.max(1, Math.ceil(page.total / page.limit))
}

function filterSwitchableAgents(items: AIAgent[]) {
  return items.filter((item) =>
    item.status !== Status.Deleted && (item.productId > 0 || item.source === "tenant_default")
  )
}

function filterWorkflowTemplates(items: AIWorkflow[]) {
  return items.filter((item) => item.agentId === 0 && item.status !== Status.Deleted)
}

function mergeById<T extends { id: number }>(items: T[], current?: T | null) {
  const seen = new Set<number>()
  return [
    ...(current ? [current] : []),
    ...items,
  ].filter((item) => {
    if (seen.has(item.id)) return false
    seen.add(item.id)
    return true
  })
}

function OptionPager({
  loading,
  onPageChange,
  page,
}: {
  loading: boolean
  onPageChange: (page: number) => void
  page: OptionPageState
}) {
  const totalPages = optionPageCount(page)
  if (page.total <= page.limit && page.page <= 1) return null

  return (
    <div className="mt-2 flex items-center justify-between gap-2 text-xs text-muted-foreground">
      <span className="tabular-nums">{ee("aiWorkbench.text008")}{page.page}{ee("aiWorkbench.text009")}{totalPages}{ee("aiWorkbench.text010")}</span>
      <div className="flex items-center gap-1.5">
        <RailopsButton
          size="small"
          disabled={loading || page.page <= 1}
          onClick={() => onPageChange(page.page - 1)}
        >{ee("aiWorkbench.text011")}</RailopsButton>
        <RailopsButton
          size="small"
          disabled={loading || page.page >= totalPages}
          onClick={() => onPageChange(page.page + 1)}
        >
          {loading ? <Loader2Icon className="size-4 animate-spin" /> : null}{ee("aiWorkbench.text012")}</RailopsButton>
      </div>
    </div>
  )
}

export function AgentWorkbenchPage({ agentId: requestedAgentId }: { agentId?: number } = {}) {
  const params = useParams<{ agentId?: string }>()
  const searchParams = useSearchParams()
  const router = useRouter()
  const confirm = useConfirm()
  const agentId = requestedAgentId ?? Number(params.agentId)
  const activeView = normalizeView(searchParams.get("view"))

  const activeSection = normalizeSection(searchParams.get("section"))
  const session = useMemo(() => readSession(), [])
  const hasProductConcept = session?.featureFlags?.product !== false

  const [agent, setAgent] = useState<AIAgent | null>(null)
  const [agents, setAgents] = useState<AIAgent[]>([])
  const [workflow, setWorkflow] = useState<AIWorkflow | null>(null)
  const [workflowTemplates, setWorkflowTemplates] = useState<AIWorkflow[]>([])
  const [bindingVersions, setBindingVersions] = useState<AIWorkflowVersion[]>([])
  const [releases, setReleases] = useState<AIAgentRelease[]>([])
  const [capabilities, setCapabilities] = useState<EnterpriseAICapability[]>([])
  const [loading, setLoading] = useState(true)
  const [agentLoaded, setAgentLoaded] = useState(false)
  const [releasesLoading, setReleasesLoading] = useState(false)
  const [capabilitiesLoading, setCapabilitiesLoading] = useState(false)
  const [workflowLoading, setWorkflowLoading] = useState(false)
  const [actionId, setActionId] = useState("")
  const [rejectingRelease, setRejectingRelease] = useState<AIAgentRelease | null>(null)
  const [rejectComment, setRejectComment] = useState("")
  const [selectedWorkflowId, setSelectedWorkflowId] = useState("")
  const [selectedWorkflowVersionId, setSelectedWorkflowVersionId] = useState("")
  const [loadingBindingVersions, setLoadingBindingVersions] = useState(false)
  const [tenantDefaultModelName, setTenantDefaultModelName] = useState("")
  const [savingTenantDefaultModel, setSavingTenantDefaultModel] = useState(false)
  const [agentOptionsPage, setAgentOptionsPage] = useState<OptionPageState>(emptyOptionPage)
  const [agentOptionsLoading, setAgentOptionsLoading] = useState(false)
  const [workflowOptionsPage, setWorkflowOptionsPage] = useState<OptionPageState>(emptyOptionPage)
  const [workflowOptionsLoading, setWorkflowOptionsLoading] = useState(false)
  const [bindingVersionsPage, setBindingVersionsPage] = useState<OptionPageState>(emptyOptionPage)

  const canCreateRelease = CanUseButton("aiAgentRelease.create", session?.permissions)
  const canReviewRelease = CanUseButton("aiAgentRelease.review", session?.permissions)
  const canDeployRelease = CanUseButton("aiAgentRelease.deploy", session?.permissions)

  const loadAgentOptions = useCallback(async (page = 1, currentAgent = agent) => {
    setAgentOptionsLoading(true)
    try {
      const nextPage = unwrapEnterpriseAgentPage(await fetchEnterpriseAIAgents({ page, limit: OPTION_PAGE_SIZE }))
      setAgentOptionsPage(nextPage.page)
      setAgents(mergeById(filterSwitchableAgents(nextPage.results), currentAgent))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ee("aiWorkbench.text001"))
    } finally {
      setAgentOptionsLoading(false)
    }
  }, [agent])

  const loadWorkflowOptions = useCallback(async (page = 1, currentWorkflow = workflow) => {
    setWorkflowOptionsLoading(true)
    try {
      const nextPage = await fetchEnterpriseAIWorkflows({ page, limit: OPTION_PAGE_SIZE })
      setWorkflowOptionsPage(nextPage.page)
      setWorkflowTemplates(mergeById(filterWorkflowTemplates(nextPage.results), currentWorkflow))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ee("aiWorkbench.text013"))
    } finally {
      setWorkflowOptionsLoading(false)
    }
  }, [workflow])

  const loadWorkbench = useCallback(async (quiet = false) => {
    if (!Number.isInteger(agentId) || agentId <= 0) {
      setLoading(false)
      return
    }
    if (!quiet) setLoading(true)
    try {
      const agentData = await fetchAIAgent(agentId)
      setAgent(agentData)
      setAgents((current) => mergeById(filterSwitchableAgents(current), agentData))
      setAgentLoaded(true)
      setLoading(false)

      setAgentOptionsLoading(true)
      const agentOptionsTask = fetchEnterpriseAIAgents({ page: 1, limit: OPTION_PAGE_SIZE })
        .then((response) => {
          const agentPage = unwrapEnterpriseAgentPage(response)
          setAgentOptionsPage(agentPage.page)
          setAgents(mergeById(filterSwitchableAgents(agentPage.results), agentData))
        })
        .catch((error) => toast.error(error instanceof Error ? error.message : ee("aiWorkbench.text001")))
        .finally(() => setAgentOptionsLoading(false))

      setReleasesLoading(true)
      const releasesTask = fetchAIAgentReleases(agentId)
        .then(setReleases)
        .catch((error) => toast.error(error instanceof Error ? error.message : ee("aiWorkbench.text014")))
        .finally(() => setReleasesLoading(false))

      setCapabilitiesLoading(true)
      const capabilitiesTask = getEnterpriseAICapabilities()
        .then((capabilityResponse) => setCapabilities(capabilityResponse.success ? capabilityResponse.data.capabilities ?? [] : []))
        .catch((error) => toast.error(error instanceof Error ? error.message : ee("aiWorkbench.text015")))
        .finally(() => setCapabilitiesLoading(false))

      setWorkflowLoading(agentData.workflowId > 0)
      const workflowTask = (agentData.workflowId > 0 ? fetchAIAgentWorkflow(agentId) : Promise.resolve(null))
        .then((workflowData) => {
          setWorkflow(workflowData)
          setWorkflowOptionsLoading(true)
          return fetchEnterpriseAIWorkflows({ page: 1, limit: OPTION_PAGE_SIZE })
            .then((templatePage) => {
              setWorkflowTemplates(mergeById(filterWorkflowTemplates(templatePage.results), workflowData))
              setWorkflowOptionsPage(templatePage.page)
            })
            .catch((error) => toast.error(error instanceof Error ? error.message : ee("aiWorkbench.text013")))
            .finally(() => setWorkflowOptionsLoading(false))
        })
        .catch((error) => {
          setWorkflow(null)
          toast.error(error instanceof Error ? error.message : ee("aiWorkbench.text016"))
        })
        .finally(() => setWorkflowLoading(false))

      await Promise.all([agentOptionsTask, releasesTask, capabilitiesTask, workflowTask])
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ee("aiWorkbench.text017"))
      setLoading(false)
      setAgentOptionsLoading(false)
      setWorkflowOptionsLoading(false)
      setReleasesLoading(false)
      setCapabilitiesLoading(false)
      setWorkflowLoading(false)
    }
  }, [agentId])

  useEffect(() => {
    void loadWorkbench()
  }, [loadWorkbench])

  function updateLocation(next: { view?: WorkbenchView; section?: AIAgentWorkbenchSection }) {
    const query = new URLSearchParams(searchParams.toString())
    if (next.view) query.set("view", next.view)
    if (next.section) query.set("section", next.section)
    router.replace(buildEnterpriseAIPath(agentId, query), { scroll: false })
  }

  async function runAction(key: string, action: () => Promise<unknown>, success: string) {
    if (actionId) return
    setActionId(key)
    try {
      await action()
      toast.success(success)
      await loadWorkbench(true)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ee("aiWorkbench.text018"))
    } finally {
      setActionId("")
    }
  }

  async function createRelease() {
    await runAction("create", () => createAIAgentRelease(agentId), ee("aiWorkbench.text019"))
  }

  async function deployRelease(release: AIAgentRelease) {
    const accepted = await confirm({
      title: ee("aiWorkbench.text020", { value0: release.releaseNo }),
      confirmText: ee("aiWorkbench.text021"),
    })
    if (!accepted) return
    await runAction(`deploy-${release.id}`, () => deployAIAgentRelease(release.id), ee("aiWorkbench.text022", { value0: release.releaseNo }))
  }

  async function rollbackRelease(release: AIAgentRelease) {
    const accepted = await confirm({
      title: ee("aiWorkbench.text023", { value0: release.releaseNo }),
      confirmText: ee("aiWorkbench.text024"),
      variant: "destructive",
    })
    if (!accepted) return
    await runAction(`rollback-${release.id}`, () => rollbackAIAgentRelease(release.id), ee("aiWorkbench.text025", { value0: release.releaseNo }))
  }

  async function rejectRelease() {
    if (!rejectingRelease || !rejectComment.trim()) return
    await runAction(
      `reject-${rejectingRelease.id}`,
      () => rejectAIAgentRelease(rejectingRelease.id, rejectComment.trim()),
      ee("aiWorkbench.text026", { value0: rejectingRelease.releaseNo }),
    )
    setRejectingRelease(null)
    setRejectComment("")
  }

  const loadBindingVersions = useCallback(async (workflowId: number, currentVersionId = 0, pageNumber = 1) => {
    if (workflowId <= 0) {
      setBindingVersions([])
      setBindingVersionsPage(emptyOptionPage)
      setSelectedWorkflowVersionId("")
      return
    }
    setLoadingBindingVersions(true)
    try {
      const versionPage = await fetchEnterpriseAIWorkflowVersions(workflowId, {
        page: pageNumber,
        limit: OPTION_PAGE_SIZE,
        releaseChannel: "stable",
        status: Status.Ok,
      })
      let currentVersion: AIWorkflowVersion | null = null
      const pagedStableVersions = versionPage.results.filter(
        (item) => item.status === Status.Ok && item.releaseChannel === "stable",
      )
      if (currentVersionId > 0 && !pagedStableVersions.some((item) => item.id === currentVersionId)) {
        const currentVersionPage = await fetchEnterpriseAIWorkflowVersions(workflowId, {
          page: 1,
          limit: 1,
          id: currentVersionId,
          releaseChannel: "stable",
          status: Status.Ok,
        })
        currentVersion = currentVersionPage.results.find(
          (item) => item.status === Status.Ok && item.releaseChannel === "stable",
        ) ?? null
      }
      const versions = mergeById(pagedStableVersions, currentVersion)
      setBindingVersions(versions)
      setBindingVersionsPage(versionPage.page)
      const preferred = versions.find((item) => item.id === currentVersionId) ?? versions[0]
      setSelectedWorkflowVersionId(preferred ? String(preferred.id) : "")
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ee("aiWorkbench.text027"))
      setBindingVersions([])
      setBindingVersionsPage(emptyOptionPage)
      setSelectedWorkflowVersionId("")
    } finally {
      setLoadingBindingVersions(false)
    }
  }, [])

  function openWorkflowBindingSection() {
    const workflowId = agent?.workflowId ?? 0
    setSelectedWorkflowId(workflowId > 0 ? String(workflowId) : "")
    void loadBindingVersions(workflowId, agent?.workflowVersionId ?? 0)
    updateLocation({ view: "configure", section: "workflow" })
  }

  async function bindWorkflowVersion() {
    const versionId = Number(selectedWorkflowVersionId)
    if (!agent || versionId <= 0 || actionId) return
    await runAction(
      "bind-workflow",
      () => bindAIAgentWorkflowVersion(agent.id, versionId),
      ee("aiWorkbench.text028"),
    )
  }

  useEffect(() => {
    if (!agent || selectedWorkflowId) return
    const workflowId = agent.workflowId ?? 0
    setSelectedWorkflowId(workflowId > 0 ? String(workflowId) : "")
    void loadBindingVersions(workflowId, agent.workflowVersionId ?? 0)
  }, [agent, loadBindingVersions, selectedWorkflowId])

  useEffect(() => {
    if (!agent) {
      setTenantDefaultModelName("")
      return
    }
    if (agent.aiConfigId > 0) {
      setTenantDefaultModelName(`__route__:${agent.aiConfigId}`)
      return
    }
    setTenantDefaultModelName(agent.llmModelName || "")
  }, [agent])

  const llmCapability = capabilities.find((item) => item.type === "llm")
  const lockedKnowledgeIds = agent?.productDefaultKnowledgeBaseId
    ? [agent.productDefaultKnowledgeBaseId]
    : []
  const activeRelease = releases.find((item) => item.id === agent?.activeReleaseId)
  const pendingCount = releases.filter((item) => item.reviewStatus === "pending").length
  const hasReleaseRows = releases.length > 0
  const showReleaseSkeleton = (loading && !agentLoaded) || (releasesLoading && !hasReleaseRows)
  const isTenantDefaultAgent = agent?.source === "tenant_default" && agent.productId === 0

  const workbenchTabs: RailopsTabItem[] = [
    { value: "configure", label: ee("aiWorkbench.text029"), icon: <ShieldCheckIcon /> },
    { value: "release", label: isTenantDefaultAgent ? ee("aiWorkbench.text030") : ee("aiWorkbench.text031"), icon: <ShieldCheckIcon /> },
    { value: "runs", label: ee("aiWorkbench.text032"), icon: <HistoryIcon /> },
  ]
  const tenantDefaultModelOptions = useMemo(() => {
    const defaultModelName = llmCapability?.modelName || ""
    const currentRouteValue = agent?.aiConfigId ? `__route__:${agent.aiConfigId}` : ""
    const availableModels = Array.from(
      new Set(
        [...(llmCapability?.models ?? []), tenantDefaultModelName]
          .map((item) => item.trim())
          .filter((item) => item && !item.startsWith("__route__:"))
      )
    )
    return [
      ...(currentRouteValue ? [{
        value: currentRouteValue,
        label: ee("aiWorkbench.text033", { value0: agent?.aiConfigName || ee("aiWorkbench.text080", { value0: agent?.aiConfigId }) }),
      }] : []),
      {
        value: "",
        label: defaultModelName ? ee("aiWorkbench.text034", { value0: defaultModelName }) : ee("aiWorkbench.text035"),
      },
      ...availableModels.map((name) => ({
        value: name,
        label: name === defaultModelName ? ee("aiWorkbench.text036", { value0: name }) : name,
      })),
    ]
  }, [agent?.aiConfigId, agent?.aiConfigName, llmCapability?.modelName, llmCapability?.models, tenantDefaultModelName])

  async function updateAgentModelDraft(nextValue: string) {
    if (!agent || savingTenantDefaultModel || nextValue.startsWith("__route__:")) return
    const previousValue = tenantDefaultModelName
    const switchingFromDedicatedRoute = agent.aiConfigId > 0
    if (!switchingFromDedicatedRoute && nextValue === (agent.llmModelName || "")) return
    setTenantDefaultModelName(nextValue)
    setSavingTenantDefaultModel(true)
    try {
      await updateAIAgent({
        id: agent.id,
        name: agent.name,
        description: agent.description,
        aiConfigId: 0,
        llmModelName: nextValue.trim(),
        serviceMode: agent.serviceMode,
        systemPrompt: agent.systemPrompt,
        welcomeMessage: agent.welcomeMessage,
        replyTimeoutSeconds: agent.replyTimeoutSeconds,
        teamIds: agent.teams.map((team) => team.id),
        handoffMode: agent.handoffMode,
        fallbackMode: agent.fallbackMode,
        fallbackMessage: agent.fallbackMessage,
        knowledgeIds: agent.knowledgeIds ?? [],
        skillIds: agent.skillIds ?? [],
        directTools: agent.directTools ?? [],
        graphTools: agent.graphTools ?? [],
      })
      if (switchingFromDedicatedRoute) {
        toast.success(nextValue.trim()
          ? ee("aiWorkbench.text037", { value0: nextValue.trim() })
          : ee("aiWorkbench.text038"))
      } else {
        toast.success(nextValue.trim()
          ? ee("aiWorkbench.text039", { value0: nextValue.trim() })
          : ee("aiWorkbench.text040"))
      }
      await loadWorkbench(true)
    } catch (error) {
      setTenantDefaultModelName(previousValue)
      toast.error(error instanceof Error ? error.message : ee("aiWorkbench.text041"))
    } finally {
      setSavingTenantDefaultModel(false)
    }
  }

  if (!Number.isInteger(agentId) || agentId <= 0) {
    return (
      <div className="border-y py-12 text-center">
        <CircleAlertIcon className="mx-auto size-6 text-destructive" />
        <h1 className="mt-3 font-semibold">{ee("aiWorkbench.text042")}</h1>
        <Link href="/enterprise/ai" className="mt-4 inline-block">
          <RailopsButton>{ee("aiWorkbench.text043")}</RailopsButton>
        </Link>
      </div>
    )
  }

  const agentInitialLoading = loading && !agentLoaded
  const displayAgent: AIAgent = agent ?? {
    id: agentId,
    tenantId: 0,
    name: ee("aiWorkbench.text044"),
    description: "",
    status: Status.Ok,
    statusName: ee("aiWorkbench.text045"),
    productId: 0,
    productName: "",
    productCode: "",
    source: "tenant_default",
    workflowId: 0,
    workflowVersionId: 0,
    draftRevision: 0,
    activeReleaseId: 0,
    aiConfigId: 0,
    aiConfigName: "",
    llmModelName: "",
    serviceMode: 0,
    serviceModeName: "",
    systemPrompt: "",
    welcomeMessage: "",
    replyTimeoutSeconds: 0,
    handoffMode: 0,
    handoffModeName: "",
    fallbackMode: 0,
    fallbackModeName: "",
    fallbackMessage: "",
    teams: [],
    knowledgeIds: [],
    knowledgeBaseNames: [],
    skillIds: [],
    skills: [],
    directTools: [],
    graphTools: [],
    productDefaultKnowledgeBaseId: 0,
    workflowPublished: false,
    workflowState: "",
    workflowStateText: "",
    sortNo: 0,
    createdAt: "",
    updatedAt: "",
    createUserName: "",
    updateUserName: "",
  }

  if (!agent && !agentInitialLoading) {
    return (
      <div className="border-y py-12 text-center">
        <CircleAlertIcon className="mx-auto size-6 text-destructive" />
        <h1 className="mt-3 font-semibold">{ee("aiWorkbench.text046")}</h1>
        <Link href="/enterprise/ai" className="mt-4 inline-block">
          <RailopsButton>{ee("aiWorkbench.text043")}</RailopsButton>
        </Link>
      </div>
    )
  }

  return (
    <div className="space-y-3">
      <header className="space-y-3 border-b pb-3">
        <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
          <div className="flex min-w-0 items-center gap-3">
            <Link href="/enterprise/ai">
              <IconButton icon={<ArrowLeftIcon className="size-4" />} tooltip={ee("aiWorkbench.text043")} aria-label={ee("aiWorkbench.text043")} />
            </Link>
            <span className="flex size-10 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
              <ShieldCheckIcon className="size-5" />
            </span>
            <div className="min-w-0">
              <RouteBreadcrumbs size="page" />
              <div className="mt-2 flex flex-wrap items-center gap-2">
                <h1 className="sr-only">{displayAgent.name}</h1>
                <span className="truncate text-sm font-semibold text-foreground">{displayAgent.name}</span>
                <StatusTag tone={displayAgent.status === Status.Ok ? "blue" : "neutral"}>{agent ? displayAgent.statusName : ee("aiWorkbench.text045")}</StatusTag>
                <StatusTag tone={activeRelease ? "blue" : "neutral"}>
                  {showReleaseSkeleton || agentInitialLoading ? ee("aiWorkbench.text047") : activeRelease ? ee("aiWorkbench.text048", { value0: activeRelease.releaseNo }) : ee("aiWorkbench.text007")}
                </StatusTag>
                {pendingCount > 0 ? <StatusTag tone="neutral">{pendingCount}{ee("aiWorkbench.text049")}</StatusTag> : null}
              </div>
              <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                <span className="inline-flex items-center gap-1"><PackageIcon className="size-3.5" />{isTenantDefaultAgent ? ee("aiWorkbench.text050") : displayAgent.productName || ee("aiWorkbench.text051")}</span>
                {displayAgent.productCode ? <span className="font-mono">{displayAgent.productCode}</span> : null}
                <span>{ee("aiWorkbench.text052")}{displayAgent.tenantId}</span>
                <span>{ee("aiWorkbench.text053")}{displayAgent.draftRevision || 1}</span>
              </div>
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <div className="w-full sm:w-64">
              <OptionCombobox
                value={String(displayAgent.id)}
                options={agents.map((item) => ({
                  value: String(item.id),
                  label: item.source === "tenant_default" && item.productId === 0
                    ? ee("aiWorkbench.text054", { value0: item.name })
                    : `${item.productName || ee("aiWorkbench.text055")} · ${item.name}`,
                  icon: <ShieldCheckIcon className="size-4" />,
                }))}
                placeholder={ee("aiWorkbench.text056")}
                searchPlaceholder={ee("aiWorkbench.text057")}
                onChange={(value) => {
                  const nextId = Number(value)
                  if (nextId > 0 && nextId !== displayAgent.id) router.push(buildEnterpriseAIPath(nextId))
                }}
                disabled={!agent}
              />
              <OptionPager
                loading={agentOptionsLoading}
                page={agentOptionsPage}
                onPageChange={(page) => void loadAgentOptions(page)}
              />
            </div>
            <RailopsButton onClick={() => void loadWorkbench()} disabled={loading}>
              <RefreshCwIcon className={loading ? "animate-spin" : undefined} />{ee("aiWorkbench.text058")}</RailopsButton>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-sm">
          <span className="inline-flex items-center gap-1.5">
            <WorkflowIcon className="size-4 text-muted-foreground" />
            {workflowLoading || agentInitialLoading ? ee("aiWorkbench.text059") : workflow?.scope === "platform" ? ee("aiWorkbench.text060") : ee("aiWorkbench.text061")}
            <StatusTag tone="neutral">#{displayAgent.workflowVersionId || "-"}</StatusTag>
          </span>
          {agentInitialLoading ? (
            <span className="text-muted-foreground">{ee("aiWorkbench.text059")}</span>
          ) : displayAgent.workflowId > 0 ? (
            <Link className="inline-flex items-center gap-1.5 text-muted-foreground underline-offset-4 hover:underline" href={buildEnterpriseWorkflowPath(displayAgent.workflowId)}>
              <WorkflowIcon className="size-3.5" />{ee("aiWorkbench.text062")}{workflow?.name || `#${displayAgent.workflowId}`}
            </Link>
          ) : (
            <span className="text-muted-foreground">{ee("aiWorkbench.text063")}</span>
          )}
          <div className="flex min-w-0 items-center gap-2">
            <span className="whitespace-nowrap text-muted-foreground">{ee("aiWorkbench.text064")}</span>
            <div className="w-[min(22rem,calc(100vw-2rem))] max-w-full">
              <OptionCombobox
                value={tenantDefaultModelName}
                options={tenantDefaultModelOptions}
                placeholder={capabilitiesLoading ? ee("aiWorkbench.text065") : llmCapability?.available ? ee("aiWorkbench.text066") : ee("aiWorkbench.text067")}
                searchPlaceholder={ee("aiWorkbench.text068")}
                emptyText={ee("aiWorkbench.text069")}
                disabled={capabilitiesLoading || !llmCapability?.available || savingTenantDefaultModel || !agent}
                onChange={(value) => {
                  void updateAgentModelDraft(value)
                }}
              />
            </div>
            {savingTenantDefaultModel ? <Loader2Icon className="size-4 animate-spin text-muted-foreground" /> : null}
          </div>
          {isTenantDefaultAgent ? (
            <StatusTag tone="neutral">{ee("aiWorkbench.text070")}</StatusTag>
          ) : (
            <RailopsButton size="small" onClick={openWorkflowBindingSection}>
              <WorkflowIcon className="size-4" />{ee("aiWorkbench.text071")}</RailopsButton>
          )}
        </div>
      </header>

      <div>
      <UnderlineTabs
        ariaLabel={ee("aiWorkbench.text072")}
        items={workbenchTabs}
        value={activeView}
        onChange={(value) => updateLocation({ view: value as WorkbenchView })}
      />

        {activeView === "configure" ? (
          <div className="mt-3">
          {isTenantDefaultAgent ? (
            <div className="grid border-y md:grid-cols-2 xl:grid-cols-4">
              {[
                [ee("aiWorkbench.text073"), ee(hasProductConcept ? "aiWorkbench.text074" : "aiWorkbench.text125")],
                [ee("aiWorkbench.text075"), workflow?.name || ee("aiWorkbench.text076", { value0: displayAgent.workflowId })],
                [ee("aiWorkbench.text077"), ee(hasProductConcept ? "aiWorkbench.text078" : "aiWorkbench.text126")],
                [ee("aiWorkbench.text079"), displayAgent.aiConfigId > 0 ? displayAgent.aiConfigName || ee("aiWorkbench.text080", { value0: displayAgent.aiConfigId }) : ee("aiWorkbench.text081")],
              ].map(([label, value]) => (
                <div key={label} className="min-w-0 border-b px-4 py-3 last:border-b-0 md:border-b-0 md:border-r md:last:border-r-0">
                  <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                    {label === ee("aiWorkbench.text075") ? <WorkflowIcon className="size-3.5" /> : null}
                    {label}
                  </div>
                  <div className="mt-1 break-words text-sm font-medium">{value}</div>
                </div>
              ))}
            </div>
          ) : (
            <div className="h-[calc(100dvh-15rem)] min-h-[680px] overflow-hidden border">
              <AIAgentConfigWorkbench
                key={`${displayAgent.id}:${displayAgent.workflowVersionId}:${displayAgent.draftRevision}`}
                agentId={displayAgent.id}
                defaultLlmModel={llmCapability?.modelName || ""}
                llmModels={llmCapability?.models ?? []}
                lockedKnowledgeIds={lockedKnowledgeIds}
                initialSection={activeSection}
                dialogHeaderInset={false}
                workflowEditing={false}
                workflowBindingPanel={(
                  <div className="space-y-5">
                    <div className="border-y py-3">
                      <div className="flex items-center gap-2 text-sm font-medium">
                        <WorkflowIcon className="size-4 text-primary" />{ee("aiWorkbench.text082")}</div>
                    </div>
                    <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
                      <div className="space-y-2">
                        <div className="flex items-center gap-1.5 text-sm font-medium">
                          <WorkflowIcon className="size-4 text-muted-foreground" />{ee("aiWorkbench.text083")}</div>
                        <OptionCombobox
                          value={selectedWorkflowId}
                          options={workflowTemplates.map((item) => ({
                            value: String(item.id),
                            label: `${item.scope === "platform" ? ee("aiWorkbench.text084") : ee("aiWorkbench.text085")} · ${item.name}`,
                            icon: <WorkflowIcon className="size-4" />,
                          }))}
                          placeholder={ee("aiWorkbench.text086")}
                          searchPlaceholder={ee("aiWorkbench.text087")}
                          emptyText={ee("aiWorkbench.text088")}
                          onChange={(value) => {
                            setSelectedWorkflowId(value)
                            void loadBindingVersions(Number(value))
                          }}
                        />
                        <OptionPager
                          loading={workflowOptionsLoading}
                          page={workflowOptionsPage}
                          onPageChange={(page) => void loadWorkflowOptions(page)}
                        />
                      </div>
                      <div className="space-y-2">
                        <div className="text-sm font-medium">{ee("aiWorkbench.text089")}</div>
                        <OptionCombobox
                          value={selectedWorkflowVersionId}
                          options={bindingVersions.map((item) => ({
                            value: String(item.id),
                            label: `v${item.version} · ${workflowVersionCapabilityLabel(item.definition)} · ${item.definitionHash.slice(0, 10)}`,
                            icon: <WorkflowIcon className="size-4" />,
                          }))}
                          placeholder={loadingBindingVersions ? ee("aiWorkbench.text090") : ee("aiWorkbench.text091")}
                          emptyText={ee("aiWorkbench.text092")}
                          disabled={loadingBindingVersions || !selectedWorkflowId}
                          onChange={setSelectedWorkflowVersionId}
                        />
                        <OptionPager
                          loading={loadingBindingVersions}
                          page={bindingVersionsPage}
                          onPageChange={(page) => void loadBindingVersions(Number(selectedWorkflowId), Number(selectedWorkflowVersionId), page)}
                        />
                      </div>
                    </div>
                    <div className="flex flex-col gap-3 border-t pt-4 sm:flex-row sm:items-center sm:justify-between">
                      <div className="text-xs text-muted-foreground">{ee("aiWorkbench.text093")}{displayAgent.workflowId || "-"}{ee("aiWorkbench.text094")}{displayAgent.workflowVersionId || "-"}
                      </div>
                      <RailopsButton
                        variant="primary"
                        disabled={!selectedWorkflowVersionId || Boolean(actionId) || !agent}
                        onClick={() => void bindWorkflowVersion()}
                      >
                        {actionId === "bind-workflow" ? <Loader2Icon className="size-4 animate-spin" /> : <CheckIcon className="size-4" />}{ee("aiWorkbench.text095")}</RailopsButton>
                    </div>
                  </div>
                )}
                contextBadges={<StatusTag tone="neutral">{workflow?.scope === "platform" ? ee("aiWorkbench.text096") : ee("aiWorkbench.text085")}</StatusTag>}
                onSectionChange={(section) => updateLocation({ section })}
                onAgentSaved={() => void loadWorkbench(true)}
              />
            </div>
          )}
          </div>
        ) : null}

        {activeView === "release" ? (
          <div className="mt-3 space-y-3">
          <div className="flex flex-col gap-3 border-y py-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <div className="font-medium">{isTenantDefaultAgent ? ee("aiWorkbench.text097") : ee("aiWorkbench.text098")}</div>
            </div>
            {canCreateRelease && !isTenantDefaultAgent ? (
              <RailopsButton variant="primary" onClick={() => void createRelease()} disabled={Boolean(actionId) || displayAgent.workflowVersionId <= 0 || !agent}>
                {actionId === "create" ? <Loader2Icon className="size-4 animate-spin" /> : <PlusIcon className="size-4" />}{ee("aiWorkbench.text099")}</RailopsButton>
            ) : null}
          </div>

          <div className="overflow-x-auto border">
            <DataTable<AIAgentRelease>
              className="rhd-railops-release-table"
              size="small"
              rowKey={(record) => record.id}
              dataSource={releases}
              loading={showReleaseSkeleton}
              emptyDescription={ee("aiWorkbench.text100")}
              scroll={{ x: 1080 }}
              columns={[
                {
                  title: ee("aiWorkbench.text101"),
                  dataIndex: "releaseNo",
                  key: "releaseNo",
                  width: 112,
                  render: (_value, release) => (
                    <div>
                      <div className="font-semibold">R{release.releaseNo}</div>
                      <div className="mt-1 font-mono text-xs text-muted-foreground">#{release.id}</div>
                    </div>
                  ),
                },
                {
                  title: ee("aiWorkbench.text102"),
                  dataIndex: "reviewStatus",
                  key: "reviewStatus",
                  width: 150,
                  render: (_value, release) => (
                    <div>
                      <StatusTag tone={releaseReviewTone(release.reviewStatus)}>{release.reviewStatusName || release.reviewStatus}</StatusTag>
                      {release.reviewComment ? <div className="mt-1 line-clamp-2 text-xs text-muted-foreground">{release.reviewComment}</div> : null}
                    </div>
                  ),
                },
                {
                  title: ee("aiWorkbench.text103"),
                  dataIndex: "deploymentStatus",
                  key: "deploymentStatus",
                  width: 150,
                  render: (_value, release) => {
                    const isActive = release.id === displayAgent.activeReleaseId
                    return (
                      <div>
                        <StatusTag tone={deploymentTone(release.deploymentStatus)}>{deploymentLabel(release.deploymentStatus)}</StatusTag>
                        {isActive ? <div className="mt-1 text-xs text-primary">{ee("aiWorkbench.text104")}</div> : null}
                      </div>
                    )
                  },
                },
                {
                  title: ee("aiWorkbench.text105"),
                  dataIndex: "workflowVersionId",
                  key: "workflowVersionId",
                  width: 200,
                  render: (_value, release) => (
                    <div>
                      <div className="inline-flex items-center gap-1.5"><WorkflowIcon className="size-3.5 text-muted-foreground" />{ee("aiWorkbench.text106")}{release.workflowVersionId}</div>
                      <div className="mt-1 text-xs text-muted-foreground">{ee("aiWorkbench.text107")}{release.workflowId}</div>
                    </div>
                  ),
                },
                {
                  title: ee("aiWorkbench.text108"),
                  dataIndex: "workflowDefinitionHash",
                  key: "workflowDefinitionHash",
                  width: 240,
                  render: (_value, release) => (
                    <div className="font-mono text-xs">
                      <div>{ee("aiWorkbench.text109")}{shortHash(release.workflowDefinitionHash)}</div>
                      <div className="inline-flex items-center gap-1.5"><ShieldCheckIcon className="size-3.5 text-muted-foreground" />{ee("aiWorkbench.text110")}{shortHash(release.agentConfigHash)}</div>
                      <div>{ee("aiWorkbench.text111")}{shortHash(release.knowledgeScopeHash)}</div>
                    </div>
                  ),
                },
                {
                  title: ee("aiWorkbench.text112"),
                  dataIndex: "createdAt",
                  key: "createdAt",
                  width: 160,
                  render: (_value, release) => (
                    <div className="text-xs text-muted-foreground">
                      <div>{formatDateTime(release.createdAt)}</div>
                      <div className="mt-1">{release.deployedAt ? formatDateTime(release.deployedAt) : ee("aiWorkbench.text113")}</div>
                    </div>
                  ),
                },
                {
                  title: ee("aiWorkbench.text114"),
                  key: "actions",
                  width: 240,
                  align: "right",
                  render: (_value, release) => {
                    const busy = actionId.endsWith(`-${release.id}`)
                    return (
                      <div className="flex justify-end gap-1.5">
                        {canCreateRelease && (release.reviewStatus === "unreviewed" || release.reviewStatus === "rejected") ? (
                          <RailopsButton size="small" disabled={Boolean(actionId)} onClick={() => void runAction(`submit-${release.id}`, () => submitAIAgentReleaseReview(release.id), ee("aiWorkbench.text115", { value0: release.releaseNo }))}>
                            {busy ? <Loader2Icon className="size-4 animate-spin" /> : <SendIcon className="size-4" />}{ee("aiWorkbench.text116")}</RailopsButton>
                        ) : null}
                        {canReviewRelease && release.reviewStatus === "pending" ? (
                          <>
                            <RailopsButton size="small" variant="primary" disabled={Boolean(actionId)} onClick={() => void runAction(`approve-${release.id}`, () => approveAIAgentRelease(release.id), ee("aiWorkbench.text117", { value0: release.releaseNo }))}>
                              {busy ? <Loader2Icon className="size-4 animate-spin" /> : <CheckIcon className="size-4" />}{ee("aiWorkbench.text118")}</RailopsButton>
                            <RailopsButton size="small" disabled={Boolean(actionId)} onClick={() => setRejectingRelease(release)}>
                              <XIcon className="size-4" />{ee("aiWorkbench.text119")}</RailopsButton>
                          </>
                        ) : null}
                        {canDeployRelease && release.reviewStatus === "approved" && release.deploymentStatus === "inactive" ? (
                          <RailopsButton size="small" variant="primary" disabled={Boolean(actionId)} onClick={() => void deployRelease(release)}>
                            {busy ? <Loader2Icon className="size-4 animate-spin" /> : <PlayIcon className="size-4" />}{ee("aiWorkbench.text103")}</RailopsButton>
                        ) : null}
                        {!isTenantDefaultAgent && canDeployRelease && release.reviewStatus === "approved" && release.id !== displayAgent.activeReleaseId && displayAgent.activeReleaseId > 0 && release.deploymentStatus !== "inactive" ? (
                          <RailopsButton size="small" disabled={Boolean(actionId)} onClick={() => void rollbackRelease(release)}>
                            {busy ? <Loader2Icon className="size-4 animate-spin" /> : <RotateCcwIcon className="size-4" />}{ee("aiWorkbench.text120")}</RailopsButton>
                        ) : null}
                      </div>
                    )
                  },
                },
              ]}
            />
          </div>
          </div>
        ) : null}

        {activeView === "runs" ? (
          <div className="mt-3">
          <AIWorkflowRunsWorkspace agentId={displayAgent.id} layout="fragment" />
          </div>
        ) : null}
      </div>

      <StandardModal
        open={Boolean(rejectingRelease)}
        onCancel={() => { if (!actionId) { setRejectingRelease(null)
            setRejectComment("") } }}
        title={ee("aiWorkbench.text121")}
        width={512}
        footer={
          <>
            <RailopsButton disabled={Boolean(actionId)} onClick={() => setRejectingRelease(null)}>{ee("aiWorkbench.text122")}</RailopsButton>
            <RailopsButton danger disabled={!rejectComment.trim() || Boolean(actionId)} onClick={() => void rejectRelease()}>
              {actionId.startsWith("reject-") ? <Loader2Icon className="size-4 animate-spin" /> : <XIcon className="size-4" />}{ee("aiWorkbench.text123")}</RailopsButton>

          </>
        }
      >
          <Input.TextArea
            rows={5}
            value={rejectComment}
            onChange={(event) => setRejectComment(event.target.value)}
            placeholder={ee("aiWorkbench.text124")}
          />
          
      </StandardModal>

    </div>
  )
}
