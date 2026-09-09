"use client"

import { translateCurrentMessage } from "@/i18n/messages"
import {
  ArrowRightIcon,
  DatabaseIcon,
  PlusIcon,
  RefreshCwIcon,
  SettingsIcon,
  ShieldCheckIcon,
  SendIcon,
  WorkflowIcon,
} from "lucide-react"
import { useRouter } from "next/navigation"
import { useCallback, useEffect, useMemo, useState } from "react"
import { toast } from "sonner"
import type { TableColumnsType } from "antd"
import {
  ContentModule,
  FilterTabs,
  SearchField,
  SelectField,
  StatusTag,
  TableToolbar,
  type RailopsTabItem,
  type StatusTagTone,
} from "@railops/ui"

import { useAuth } from "@/components/auth-provider"
import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { CanUseButton } from "@/components/layout/permission-guard"
import { ProductTree, buildProductTreeGroups } from "@/components/product/product-tree"
import { ModuleLoading } from "@/components/shared/loading-states"
import { DataTable, IconButton, PageShell, RailopsButton, TablePagination } from "@railops/ui"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { ScrollArea } from "@/components/ui/scroll-area"
import { type AIAgent } from "@/lib/api/admin"
import {
  ensureProductAIAgent,
  ensureTenantDefaultAIAgent,
  fetchEnterpriseAIAgentSummary,
  fetchEnterpriseAIAgents,
  provisionProductAIAgents,
  type EnterpriseAIAgentSummary,
} from "@/lib/api/enterprise-ai"
import {
  getEnterpriseAICapabilities,
  updateEnterpriseAIDefaultLLMModel,
  type EnterpriseAICapability,
  type EnterpriseAICapabilities,
} from "@/lib/api/enterprise-models"
import {
  AIAgentHandoffMode,
  Status,
} from "@/lib/generated/enums"
import { fetchProductCount, listAllProducts } from "@/lib/api/enterprise-products"
import type { ProductListItem } from "@/lib/api/types"
import { buildEnterpriseAIPath } from "@/lib/enterprise-detail-route"
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


const AGENT_PAGE_SIZE = 10

const emptySummary: EnterpriseAIAgentSummary = {
  total: 0,
  active: 0,
  notDeployed: 0,
  productTotal: 0,
  productAgentTotal: 0,
  missingProductAgents: 0,
  tenantDefaultReady: false,
  productAgentProductIds: [],
}

const handoffModeLabels: Record<AIAgentHandoffMode, string> = {
  [AIAgentHandoffMode.WaitPool]: ee("aiServiceCenter.text001"),
  [AIAgentHandoffMode.DefaultTeamPool]: ee("aiServiceCenter.text002"),
  [AIAgentHandoffMode.AIHoldAndNotify]: ee("aiServiceCenter.text003"),
}

function reviewLabel(agent: AIAgent) {
  if (agent.reviewStatusName) return agent.reviewStatusName
  if (agent.reviewStatus === "pending") return ee("aiServiceCenter.text004")
  if (agent.reviewStatus === "approved") return ee("aiServiceCenter.text005")
  if (agent.reviewStatus === "rejected") return ee("aiServiceCenter.text006")
  return ee("aiServiceCenter.text007")
}

function reviewTone(agent: AIAgent): StatusTagTone {
  if (agent.reviewStatus === "approved") return "success"
  if (agent.reviewStatus === "rejected") return "error"
  if (agent.reviewStatus === "pending") return "warning"
  return "neutral"
}

function runtimeLabel(agent: AIAgent) {
  if (agent.status === Status.Deleted) return ee("aiServiceCenter.text008")
  if (agent.activeReleaseId <= 0) return ee("aiServiceCenter.text009")
  if (agent.status === Status.Ok) return ee("aiServiceCenter.text010")
  return ee("aiServiceCenter.text011")
}

function runtimeTone(agent: AIAgent): StatusTagTone {
  if (agent.status === Status.Deleted) return "disabled"
  if (agent.status === Status.Ok && agent.activeReleaseId > 0) return "success"
  if (agent.activeReleaseId <= 0) return "warning"
  return "neutral"
}

function CapabilityStatus({
  capability,
  savingModelName,
  onDefaultModelChange,
}: {
  capability: EnterpriseAICapability
  savingModelName?: string | null
  onDefaultModelChange?: (modelName: string) => void
}) {
  const models = capability.models ?? []
  const label = capability.type === "embedding" ? ee("aiServiceCenter.text012") : ee("aiServiceCenter.text013")
  const canSelectDefaultModel = capability.type === "llm" && models.length > 0 && onDefaultModelChange
  const selectedModelName = savingModelName || capability.modelName || undefined

  return (
    <div className="flex min-w-0 items-center gap-2 px-3 py-2">
      {capability.type === "embedding" ? (
        <DatabaseIcon className="size-4 shrink-0 text-muted-foreground" />
      ) : (
        <SettingsIcon className="size-4 shrink-0 text-muted-foreground" />
      )}
      <span className="whitespace-nowrap text-sm font-medium">{label}</span>
      <StatusTag tone={capability.available ? "success" : "error"}>
        {capability.available ? ee("aiServiceCenter.text014") : ee("aiServiceCenter.text015")}
      </StatusTag>
      {canSelectDefaultModel ? (
        <SelectField
          className="min-w-0 flex-1"
          style={{ marginBottom: 0 }}
          selectProps={{
            "aria-label": ee("aiServiceCenter.text016"),
            placeholder: ee("aiServiceCenter.text017"),
            value: selectedModelName,
            onChange: (value) => {
              if (value && value !== capability.modelName) onDefaultModelChange(value)
            },
            disabled: Boolean(savingModelName) || !capability.available,
            loading: Boolean(savingModelName),
            options: [
              ...(selectedModelName && !models.includes(selectedModelName)
                ? [{ value: selectedModelName, label: selectedModelName }]
                : []),
              ...models.map((model) => ({ value: model, label: model })),
            ],
            style: { width: "100%" },
          }}
        />
      ) : capability.modelName ? (
        <span className="max-w-56 truncate font-mono text-xs text-muted-foreground">
          {capability.modelName}
        </span>
      ) : null}
      {!canSelectDefaultModel && models.length > 0 ? (
        <Popover>
          <PopoverTrigger render={<RailopsButton size="small" className="px-2 text-xs" />}>
            {models.length}{ee("aiServiceCenter.text018")}</PopoverTrigger>
          <PopoverContent align="start" className="w-[min(32rem,calc(100vw-2rem))] gap-0 overflow-hidden p-0">
            <div className="border-b px-3 py-2.5">
              <div className="text-sm font-medium">{label}{ee("aiServiceCenter.text019")}</div>
            </div>
            <ScrollArea className="h-64">
              <div className="grid grid-cols-1 gap-1 p-2 sm:grid-cols-2">
                {models.map((model) => (
                  <div key={model} className="flex min-w-0 items-center justify-between gap-2 rounded-sm px-2 py-1.5 text-xs hover:bg-muted">
                    <span className="truncate font-mono">{model}</span>
                    {model === capability.modelName ? <StatusTag tone="blue">{ee("aiServiceCenter.text020")}</StatusTag> : null}
                  </div>
                ))}
              </div>
            </ScrollArea>
          </PopoverContent>
        </Popover>
      ) : null}
    </div>
  )
}

function ModelSettingsPopover({
  canUpdateDefaultModel,
  capabilitiesLoading,
  embeddingCapability,
  hasCapabilities,
  llmCapability,
  savingDefaultModelName,
  updateDefaultModel,
}: {
  canUpdateDefaultModel: boolean
  capabilitiesLoading: boolean
  embeddingCapability?: EnterpriseAICapability
  hasCapabilities: boolean
  llmCapability?: EnterpriseAICapability
  savingDefaultModelName: string | null
  updateDefaultModel: (modelName: string) => void
}) {
  return (
    <Popover>
      <PopoverTrigger render={<RailopsButton size="small" title={ee("aiServiceCenter.text021")} aria-label={ee("aiServiceCenter.text021")} />}>
        <SettingsIcon className="size-3.5" />
      </PopoverTrigger>
      <PopoverContent align="end" className="w-[min(34rem,calc(100vw-2rem))] gap-0 overflow-hidden p-0">
        <div className="flex items-center justify-between gap-3 border-b px-3 py-2.5">
          <div>
            <div className="text-sm font-semibold">{ee("aiServiceCenter.text021")}</div>
            <div className="mt-0.5 text-xs text-muted-foreground">
              {capabilitiesLoading ? ee("aiServiceCenter.text022") : llmCapability?.available ? ee("aiServiceCenter.text023") : ee("aiServiceCenter.text024")}
            </div>
          </div>
          <SettingsIcon className="size-4 text-muted-foreground" />
        </div>
        {capabilitiesLoading ? (
          <div className="p-3">
            <ModuleLoading label={ee("aiServiceCenter.text025")} count={2} variant="list" />
          </div>
        ) : hasCapabilities ? (
          <div
            className={llmCapability && embeddingCapability
              ? "grid divide-y md:grid-cols-2 md:divide-x md:divide-y-0"
              : "grid"}
            aria-label={ee("aiServiceCenter.text026")}
          >
            {llmCapability ? (
              <CapabilityStatus
                capability={llmCapability}
                savingModelName={savingDefaultModelName}
                onDefaultModelChange={canUpdateDefaultModel ? updateDefaultModel : undefined}
              />
            ) : null}
            {embeddingCapability ? <CapabilityStatus capability={embeddingCapability} /> : null}
          </div>
        ) : (
          <div className="px-3 py-6 text-center text-sm text-muted-foreground">{ee("aiServiceCenter.text027")}</div>
        )}
      </PopoverContent>
    </Popover>
  )
}

type AgentStatusFilter = "all" | "running" | "not_deployed" | "disabled" | "pending" | "rejected"
type AgentScopeSelection = "tenant" | "all" | number
type DefaultCredential = NonNullable<EnterpriseAICapabilities["defaultCredential"]>

function knowledgeSummary(agent: AIAgent) {
  if (agent.knowledgeBaseNames?.length) {
    return agent.knowledgeBaseNames.slice(0, 2).join("、")
  }
  return ee("aiServiceCenter.text028", { value0: agent.knowledgeIds?.length ?? 0 })
}

function handoffTeamSummary(agent: AIAgent) {
  const names = (agent.teams ?? []).map((team) => team.name).filter(Boolean)
  if (names.length > 0) return names.slice(0, 2).join("、")
  return ee("aiServiceCenter.text029")
}

function handoffModeSummary(agent: AIAgent) {
  return handoffModeLabels[agent.handoffMode as AIAgentHandoffMode] || agent.handoffModeName || ee("aiServiceCenter.text002")
}

function agentReleaseActionLabel(agent: AIAgent, canCreateRelease: boolean, canReviewRelease: boolean) {
  if (agent.reviewStatus === "pending" && canReviewRelease) return ee("aiServiceCenter.text082")
  if ((agent.reviewStatus === "unreviewed" || agent.reviewStatus === "rejected") && canCreateRelease) return ee("aiServiceCenter.text083")
  return ee("aiServiceCenter.text084")
}

function agentReleaseActionIcon(agent: AIAgent, canCreateRelease: boolean, canReviewRelease: boolean) {
  if ((agent.reviewStatus === "unreviewed" || agent.reviewStatus === "rejected") && canCreateRelease) {
    return <SendIcon className="size-3.5" />
  }
  return <ShieldCheckIcon className="size-3.5" />
}

export function EnterpriseAiServiceCenter() {
  const router = useRouter()
  const { ready: authReady, session } = useAuth()
  const hasProductConcept = session?.featureFlags?.product !== false
  const showProductTree = authReady && hasProductConcept
  const [summary, setSummary] = useState<EnterpriseAIAgentSummary>(emptySummary)
  const [agents, setAgents] = useState<AIAgent[]>([])
  const [agentPage, setAgentPage] = useState(1)
  const [agentPageSize, setAgentPageSize] = useState(AGENT_PAGE_SIZE)
  const [agentTotal, setAgentTotal] = useState(0)
  const [products, setProducts] = useState<ProductListItem[]>([])
  const [catalogProductTotal, setCatalogProductTotal] = useState<number | null>(null)
  const [capabilities, setCapabilities] = useState<EnterpriseAICapability[]>([])
  const [defaultCredential, setDefaultCredential] = useState<DefaultCredential>()
  const [summaryLoading, setSummaryLoading] = useState(true)
  const [productsLoading, setProductsLoading] = useState(true)
  const [agentsLoading, setAgentsLoading] = useState(true)
  const [capabilitiesLoading, setCapabilitiesLoading] = useState(true)
  const [provisioning, setProvisioning] = useState(false)
  const [savingDefaultModelName, setSavingDefaultModelName] = useState<string | null>(null)
  const [selectedScope, setSelectedScope] = useState<AgentScopeSelection>("tenant")
  const [treeSearch, setTreeSearch] = useState("")
  const [agentSearch, setAgentSearch] = useState("")
  const [agentStatusFilter, setAgentStatusFilter] = useState<AgentStatusFilter>("all")

  const loadSummary = useCallback(async () => {
    setSummaryLoading(true)
    try {
      const response = await fetchEnterpriseAIAgentSummary()
      if (!response.success || !response.data) {
        throw new Error(response.error?.message || ee("aiServiceCenter.text030"))
      }
      setSummary(response.data)
    } catch (error) {
      setSummary(emptySummary)
      toast.error(error instanceof Error ? error.message : ee("aiServiceCenter.text030"))
    } finally {
      setSummaryLoading(false)
    }
  }, [])

  const loadProducts = useCallback(async () => {
    if (!authReady) return
    if (!hasProductConcept) {
      setProducts([])
      setCatalogProductTotal(0)
      setTreeSearch("")
      setSelectedScope("tenant")
      setProductsLoading(false)
      return
    }
    setProductsLoading(true)
    try {
      const [productItems, countResponse] = await Promise.all([
        listAllProducts(),
        fetchProductCount().catch(() => null),
      ])
      setProducts(productItems)
      setCatalogProductTotal(countResponse?.success && countResponse.data ? countResponse.data.total : productItems.length)
    } catch (error) {
      setProducts([])
      toast.error(error instanceof Error ? error.message : ee("aiServiceCenter.text031"))
    } finally {
      setProductsLoading(false)
    }
  }, [authReady, hasProductConcept])

  const loadCapabilities = useCallback(async () => {
    setCapabilitiesLoading(true)
    try {
      const capabilityResponse = await getEnterpriseAICapabilities()
      setCapabilities(capabilityResponse.success ? capabilityResponse.data.capabilities ?? [] : [])
      setDefaultCredential(capabilityResponse.success ? capabilityResponse.data.defaultCredential : undefined)
    } catch (error) {
      setCapabilities([])
      setDefaultCredential(undefined)
      toast.error(error instanceof Error ? error.message : ee("aiServiceCenter.text032"))
    } finally {
      setCapabilitiesLoading(false)
    }
  }, [])

  const loadAgents = useCallback(async () => {
    setAgentsLoading(true)
    try {
      const query = {
        page: agentPage,
        limit: agentPageSize,
        search: selectedScope === "all" ? agentSearch.trim() || undefined : undefined,
        productId: selectedScope === "tenant" ? 0 : typeof selectedScope === "number" ? selectedScope : undefined,
        productScoped: selectedScope === "all" ? 0 : undefined,
        source: selectedScope === "tenant" ? "tenant_default" : undefined,
        reviewStatus: selectedScope === "all" && (agentStatusFilter === "pending" || agentStatusFilter === "rejected") ? agentStatusFilter : undefined,
        runtimeStatus: selectedScope === "all" && (agentStatusFilter === "running" || agentStatusFilter === "not_deployed" || agentStatusFilter === "disabled") ? agentStatusFilter : undefined,
      }
      const response = await fetchEnterpriseAIAgents(query)
      if (!response.success || !response.data) {
        throw new Error(response.error?.message || ee("aiServiceCenter.text033"))
      }
      setAgents(response.data.results)
      setAgentTotal(response.data.page.total)
    } catch (error) {
      setAgents([])
      setAgentTotal(0)
      toast.error(error instanceof Error ? error.message : ee("aiServiceCenter.text033"))
    } finally {
      setAgentsLoading(false)
    }
  }, [agentPage, agentPageSize, agentSearch, agentStatusFilter, selectedScope])

  useEffect(() => {
    void loadSummary()
    void loadProducts()
    void loadCapabilities()
  }, [loadCapabilities, loadProducts, loadSummary])

  useEffect(() => {
    void loadAgents()
  }, [loadAgents])

  useEffect(() => {
    setAgentPage(1)
  }, [agentPageSize, agentSearch, agentStatusFilter, selectedScope])

  const refreshWorkspace = useCallback(async () => {
    await Promise.all([
      loadSummary(),
      loadProducts(),
      loadCapabilities(),
      loadAgents(),
    ])
  }, [loadAgents, loadCapabilities, loadProducts, loadSummary])

  async function provisionAgents() {
    setProvisioning(true)
    try {
      const result = await provisionProductAIAgents()
      toast.success(ee("aiServiceCenter.text034", { value0: result.created, value1: result.existing }))
      if (result.failed > 0) toast.warning(ee("aiServiceCenter.text035", { value0: result.failed }))
      await refreshWorkspace()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ee("aiServiceCenter.text036"))
    } finally {
      setProvisioning(false)
    }
  }

  async function createSelectedProductAgent() {
    const selectedProductId = typeof selectedScope === "number" ? selectedScope : null
    if (!selectedProductId || provisioning) return
    setProvisioning(true)
    try {
      const item = await ensureProductAIAgent(selectedProductId)
      toast.success(ee("aiServiceCenter.text037"))
      await refreshWorkspace()
      router.push(buildEnterpriseAIPath(item.id))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ee("aiServiceCenter.text038"))
    } finally {
      setProvisioning(false)
    }
  }

  async function createTenantDefaultAgent() {
    if (provisioning) return
    setProvisioning(true)
    try {
      const item = await ensureTenantDefaultAIAgent()
      toast.success(ee("aiServiceCenter.text039"))
      await refreshWorkspace()
      router.push(buildEnterpriseAIPath(item.id))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ee("aiServiceCenter.text040"))
      await refreshWorkspace()
    } finally {
      setProvisioning(false)
    }
  }

  const llmCapability = capabilities.find((item) => item.type === "llm")
  const embeddingCapability = capabilities.find((item) => item.type === "embedding")
  const hasCapabilities = Boolean(llmCapability || embeddingCapability)
  const canUpdateDefaultModel = CanUseButton("aiConfig.update", session?.permissions)
  const canCreateRelease = CanUseButton("aiAgentRelease.create", session?.permissions)
  const canReviewRelease = CanUseButton("aiAgentRelease.review", session?.permissions)

  const updateDefaultModel = useCallback(async (modelName: string) => {
    if (!canUpdateDefaultModel || savingDefaultModelName) return
    setSavingDefaultModelName(modelName)
    try {
      const response = await updateEnterpriseAIDefaultLLMModel(modelName)
      if (!response.success || !response.data) {
        throw new Error(response.error?.message || ee("aiServiceCenter.text041"))
      }
      setCapabilities(response.data.capabilities ?? [])
      setDefaultCredential(response.data.defaultCredential)
      toast.success(ee("aiServiceCenter.text042", { value0: modelName }))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ee("aiServiceCenter.text041"))
    } finally {
      setSavingDefaultModelName(null)
    }
  }, [canUpdateDefaultModel, savingDefaultModelName])

  const productAgentProductIds = useMemo(
    () => new Set(summary.productAgentProductIds ?? []),
    [summary.productAgentProductIds],
  )
  const filteredProducts = useMemo(() => {
    const keyword = treeSearch.trim().toLowerCase()
    return products.filter((product) => {
      if (!keyword) return true
      return `${product.name} ${product.code} ${product.product_line}`.toLowerCase().includes(keyword)
    })
  }, [products, treeSearch])
  const treeGroups = useMemo(() => buildProductTreeGroups(filteredProducts), [filteredProducts])
  const selectedProductId = typeof selectedScope === "number" ? selectedScope : null
  const selectedProduct = products.find((item) => item.id === selectedProductId) ?? null
  const selectedProductHasAgent = selectedProductId ? productAgentProductIds.has(selectedProductId) : false
  const visibleAgents = agents
  const productTotal = catalogProductTotal ?? summary.productTotal ?? products.length
  const missingProductAgentCount = Math.max(0, productTotal - productAgentProductIds.size)
  const showHeaderAction = selectedScope === "tenant"
    ? !summary.tenantDefaultReady || !defaultCredential?.ready
    : typeof selectedScope === "number"
      ? !selectedProductHasAgent
      : missingProductAgentCount > 0
  const headerActionLabel = selectedScope === "tenant"
    ? summary.tenantDefaultReady ? ee("aiServiceCenter.text043") : ee("aiServiceCenter.text044")
    : typeof selectedScope === "number"
      ? ee("aiServiceCenter.text045")
      : ee("aiServiceCenter.text046", { value0: missingProductAgentCount })
  const refreshing = summaryLoading || (hasProductConcept && productsLoading) || agentsLoading || capabilitiesLoading

  const openAgent = useCallback((agent: AIAgent) => {
    router.push(buildEnterpriseAIPath(agent.id))
  }, [router])

  const openAgentRelease = useCallback((agent: AIAgent) => {
    const query = new URLSearchParams({ view: "release" })
    router.push(buildEnterpriseAIPath(agent.id, query))
  }, [router])

  const agentStatusTabs = useMemo<RailopsTabItem[]>(() => [
    { value: "all", label: ee("aiServiceCenter.text047"), count: agentTotal },
    { value: "running", label: ee("aiServiceCenter.text010"), count: summary.active },
    { value: "not_deployed", label: ee("aiServiceCenter.text009"), count: summary.notDeployed },
    { value: "pending", label: ee("aiServiceCenter.text004") },
    { value: "rejected", label: ee("aiServiceCenter.text006") },
  ], [agentTotal, summary.active, summary.notDeployed])

  const agentColumns = useMemo<TableColumnsType<AIAgent>>(() => [
    {
      title: ee("aiServiceCenter.text048"),
      key: "agent",
      width: 260,
      fixed: "left",
      render: (_, agent) => {
        const isTenantDefault = agent.source === "tenant_default" && agent.productId === 0
        return (
          <div className="rhd-railops-table-cell">
            <strong>{agent.name}</strong>
            <span>{isTenantDefault ? ee("aiServiceCenter.text049") : agent.productName || ee("aiServiceCenter.text050")}</span>
            {agent.productCode ? <span className="rhd-railops-service-code">{agent.productCode}</span> : null}
          </div>
        )
      },
    },
    {
      title: ee("aiServiceCenter.text051"),
      key: "runtime",
      width: 116,
      render: (_, agent) => <StatusTag tone={runtimeTone(agent)}>{runtimeLabel(agent)}</StatusTag>,
    },
    {
      title: ee("aiServiceCenter.text052"),
      key: "review",
      width: 116,
      render: (_, agent) => <StatusTag tone={reviewTone(agent)}>{reviewLabel(agent)}</StatusTag>,
    },
    {
      title: ee("aiServiceCenter.text053"),
      key: "workflow",
      width: 160,
      render: (_, agent) => (
        <div className="rhd-railops-table-cell">
          <strong>{agent.source === "tenant_default" && agent.productId === 0 ? ee("aiServiceCenter.text054") : agent.workflowVersionId > 0 ? ee("aiServiceCenter.text055") : ee("aiServiceCenter.text056")}</strong>
          <span className="inline-flex items-center gap-1"><WorkflowIcon className="size-3.5" />{agent.activeReleaseId > 0 ? `Release #${agent.activeReleaseId}` : ee("aiServiceCenter.text057")}</span>
        </div>
      ),
    },
    {
      title: ee("aiServiceCenter.text058"),
      key: "knowledge",
      width: 180,
      render: (_, agent) => (
        <div className="rhd-railops-table-cell">
          <strong>{knowledgeSummary(agent)}</strong>
          <span>{agent.knowledgeIds?.length ?? 0}{ee("aiServiceCenter.text059")}</span>
        </div>
      ),
    },
    {
      title: ee("aiServiceCenter.text060"),
      key: "handoff",
      width: 220,
      render: (_, agent) => {
        const isTenantDefault = agent.source === "tenant_default" && agent.productId === 0
        return (
          <div className="rhd-railops-table-cell">
            <strong>{isTenantDefault ? ee(hasProductConcept ? "aiServiceCenter.text061" : "aiServiceCenter.text085") : handoffModeSummary(agent)}</strong>
            <span>{isTenantDefault ? ee(hasProductConcept ? "aiServiceCenter.text062" : "aiServiceCenter.text086") : handoffTeamSummary(agent)}</span>
          </div>
        )
      },
    },
    {
      title: ee("aiServiceCenter.text063"),
      key: "updatedAt",
      width: 150,
      render: (_, agent) => formatDateTime(agent.updatedAt),
    },
    {
      title: ee("aiServiceCenter.text064"),
      key: "actions",
      width: 184,
      fixed: "right",
      align: "right",
      render: (_, agent) => (
        <div className="rhd-railops-row-actions justify-end">
          <RailopsButton size="small" onClick={() => openAgentRelease(agent)}>
            {agentReleaseActionIcon(agent, canCreateRelease, canReviewRelease)}
            {agentReleaseActionLabel(agent, canCreateRelease, canReviewRelease)}
          </RailopsButton>
          <IconButton icon={<ArrowRightIcon className="size-3.5" />} tooltip={ee("aiServiceCenter.text065", { value0: agent.name })} aria-label={ee("aiServiceCenter.text065", { value0: agent.name })} onClick={() => openAgent(agent)} />
        </div>
      ),
    },
  ], [canCreateRelease, canReviewRelease, openAgent, openAgentRelease])

  return (
    <PageShell
      title={ee("aiServiceCenter.text066")}
      breadcrumb={useRouteBreadcrumbItems()}
      className="enterprise-redesign ent-object-page ent-light-workbench-page rhd-railops-ai-page"
      actions={
          <>
            <ModelSettingsPopover
              canUpdateDefaultModel={canUpdateDefaultModel}
              capabilitiesLoading={capabilitiesLoading}
              embeddingCapability={embeddingCapability}
              hasCapabilities={hasCapabilities}
              llmCapability={llmCapability}
              savingDefaultModelName={savingDefaultModelName}
              updateDefaultModel={updateDefaultModel}
            />
            {showHeaderAction ? (
              <RailopsButton
                variant="primary"
                onClick={() => {
                  if (selectedScope === "tenant") {
                    void createTenantDefaultAgent()
                    return
                  }
                  if (typeof selectedScope === "number") {
                    void createSelectedProductAgent()
                    return
                  }
                  void provisionAgents()
                }}
                disabled={provisioning}
              >
                <PlusIcon className={provisioning ? "animate-pulse" : undefined} />
                {headerActionLabel}
              </RailopsButton>
            ) : null}
          </>
        }
      >

      <section
        className="rhd-railops-ai-workspace"
        style={showProductTree ? undefined : { gridTemplateColumns: "minmax(0, 1fr)" }}
      >
        {showProductTree ? (
          <div className="rhd-railops-product-panel rhd-railops-product-directory-panel">
            <ProductTree
              title={ee("aiServiceCenter.text067")}
              totalCount={productTotal}
              countLabel={ee("aiServiceCenter.text068", { value0: productTotal })}
              groups={treeGroups}
              mode="directory"
              rootLabel={ee("aiServiceCenter.text069")}
              loading={productsLoading || summaryLoading}
              selectedProductId={selectedProductId}
              rootSelected={selectedScope === "all"}
              pinnedItems={[{
                key: "tenant-default",
                label: ee("aiServiceCenter.text049"),
                badge: <span className="ent-product-list-badge">{ee("aiServiceCenter.text070")}</span>,
                selected: selectedScope === "tenant",
                onSelect: () => setSelectedScope("tenant"),
              }]}
              search={treeSearch}
              onSearchChange={setTreeSearch}
              onSelectRoot={() => setSelectedScope("all")}
              onSelectProduct={(product) => setSelectedScope(product.id)}
              emptyText={ee("aiServiceCenter.text071")}
              renderProductMeta={(product) => <small className="kb-product-code">{product.code || `PROD-${product.id}`}</small>}
            />
          </div>
        ) : null}
        <main className="min-w-0">
          <ContentModule
            className="rhd-railops-ai-agent-module"
            title={(
              <span className="rhd-railops-ai-module-title">
                <ShieldCheckIcon className="size-4" />
                {selectedScope === "tenant" ? ee("aiServiceCenter.text049") : selectedProduct?.name || ee("aiServiceCenter.text072")}
              </span>
            )}
            note={selectedScope === "tenant"
              ? ee(hasProductConcept ? "aiServiceCenter.text062" : "aiServiceCenter.text086")
              : selectedProduct ? `${selectedProduct.code || `PROD-${selectedProduct.id}`} · ${selectedProduct.product_line || ee("aiServiceCenter.text073")}` : ee("aiServiceCenter.text074", { value0: agentTotal, value1: missingProductAgentCount })}
            extra={(
              <div className="rhd-railops-ai-module-actions">
                <StatusTag tone={defaultCredential?.ready ? "success" : "warning"}>{defaultCredential?.ready ? ee("aiServiceCenter.text075") : ee("aiServiceCenter.text076")}</StatusTag>
                <IconButton icon={<RefreshCwIcon className={refreshing ? "size-3.5 animate-spin" : "size-3.5"} />} tooltip={ee("aiServiceCenter.text077")} aria-label={ee("aiServiceCenter.text077")} onClick={() => void refreshWorkspace()} disabled={refreshing} />
              </div>
            )}
          >
            <TableToolbar
              className="rhd-railops-ai-toolbar"
              search={selectedScope === "all" ? (
                  <SearchField
                    allowClear
                    aria-label={ee("aiServiceCenter.text078")}
                    className="rhd-railops-ai-search"
                    placeholder={ee("aiServiceCenter.text078")}
                    value={agentSearch}
                    onChange={(event) => setAgentSearch(event.target.value)}
                  />
                ) : undefined}
              filters={(
                <div className="rhd-railops-ai-filter-selects">
                  {selectedScope === "all" ? (
                    <FilterTabs
                      ariaLabel={ee("aiServiceCenter.text079")}
                      items={agentStatusTabs}
                      value={agentStatusFilter}
                      onChange={(value) => setAgentStatusFilter(value as AgentStatusFilter)}
                    />
                  ) : null}
                </div>
              )}
            />

            <div className="rhd-railops-ai-agent-list">
              <DataTable<AIAgent>
                className="rhd-railops-ai-table"
                columns={agentColumns}
                dataSource={visibleAgents}
                rowKey="id"
                loading={agentsLoading}
                pagination={false}
                size="small"
                scroll={{ x: 1268 }}
                emptyDescription={
                  <div className="rhd-railops-ai-table-empty">
                    <span>{selectedScope === "tenant" ? ee("aiServiceCenter.text080") : ee("aiServiceCenter.text081")}</span>
                    {selectedScope === "tenant" && !summary.tenantDefaultReady ? (
                      <RailopsButton variant="primary" size="small" onClick={() => void createTenantDefaultAgent()} disabled={provisioning}>
                        <PlusIcon className={provisioning ? "animate-pulse" : undefined} />{ee("aiServiceCenter.text044")}</RailopsButton>
                    ) : null}
                  </div>
                }
              />
              {selectedScope === "all" ? (
                <div className="rhd-railops-ai-pagination">
                  <TablePagination
                    current={agentPage}
                    total={agentTotal}
                    pageSize={agentPageSize}
                    showSizeChanger
                    pageSizeOptions={[10, 20, 50]}
                    onChange={(page, pageSize) => {
                      if (pageSize !== agentPageSize) {
                        setAgentPageSize(pageSize)
                        setAgentPage(1)
                      } else {
                        setAgentPage(page)
                      }
                    }}
                  />
                </div>
              ) : null}
            </div>
          </ContentModule>
        </main>
      </section>
    </PageShell>
  )
}
