"use client"

import {
  Background,
  Handle,
  MarkerType,
  MiniMap,
  Position,
  ReactFlow,
  type Edge,
  type Node,
  type NodeProps,
  type ReactFlowInstance,
} from "@xyflow/react"
import {
  ActivityIcon,
  ArrowLeftIcon,
  ArrowRightIcon,
  ArrowUpRightIcon,
  BookOpenIcon,
  BracesIcon,
  CheckCircle2Icon,
  ChevronDownIcon,
  ChevronRightIcon,
  CircleAlertIcon,
  CircleStopIcon,
  ClipboardCheckIcon,
  Clock3Icon,
  CopyPlusIcon,
  DatabaseIcon,
  GitBranchIcon,
  HistoryIcon,
  KeyRoundIcon,
  LayoutTemplateIcon,
  Layers3Icon,
  ListChecksIcon,
  Loader2Icon,
  LockKeyholeIcon,
  LocateFixedIcon,
  MessageSquareTextIcon,
  NetworkIcon,
  PlayIcon,
  RefreshCwIcon,
  RotateCcwIcon,
  RouteIcon,
  ScanLineIcon,
  ShieldCheckIcon,
  TicketIcon,
  Trash2Icon,
  UsersIcon,
  VideoIcon,
  WorkflowIcon,
  WrenchIcon,
  ZoomInIcon,
  ZoomOutIcon,
  type LucideIcon,
} from "lucide-react"
import Link from "next/link"
import { useParams, useRouter } from "next/navigation"
import { useCallback, useEffect, useMemo, useState } from "react"
import { toast } from "sonner"

import { DetailDrawer, SelectField, StandardModal, UnderlineTabs, type RailopsTabItem } from "@railops/ui"
import { AIWorkflowRunsWorkspace } from "@/app/dashboard/ai-workflow-runs/_components/workspace"
import { useConfirm } from "@/components/confirm-provider"
import { CanUseButton } from "@/components/layout/permission-guard"
import { RouteBreadcrumbs } from "@/components/layout/route-breadcrumbs"
import { ListPagination } from "@/components/list-pagination"
import { ModuleLoading } from "@/components/shared/loading-states"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Checkbox as AntCheckbox } from "antd"
import { useI18n } from "@/i18n/provider"
import { Input } from "@/components/ui/input"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"
import {
  type AIAgent,
  type AIWorkflow,
  type AIWorkflowDefinition,
  type AIWorkflowNodeSpec,
  type AIWorkflowRuntimeVariable,
  type AIWorkflowTestRunResult,
  type AIWorkflowVersion,
} from "@/lib/api/admin"
import {
  bindAIAgentWorkflowVersion,
  createEnterpriseAIWorkflowTemplate,
  fetchEnterpriseAIAgents,
  deleteEnterpriseAIWorkflow,
  fetchEnterpriseAIWorkflow,
  fetchEnterpriseAIWorkflowAdoption,
  fetchEnterpriseAIWorkflows,
  fetchEnterpriseAIWorkflowVersions,
  publishEnterpriseAIWorkflow,
  rollbackEnterpriseAIWorkflowVersion,
  testEnterpriseAIWorkflow,
  updateEnterpriseAIWorkflowDraft,
} from "@/lib/api/enterprise-ai"
import { readSession } from "@/lib/auth"
import {
  buildEnterpriseAIPath,
  buildEnterpriseWorkflowPath,
} from "@/lib/enterprise-detail-route"
import { Status } from "@/lib/generated/enums"
import { formatDateTime } from "@/lib/utils"

import {
  describeWorkflowRunError,
  describeWorkflowModelCredentialChain,
  getWorkflowModelCredentialChain,
  getWorkflowModelCredentialPreset,
  getWorkflowProductProfile,
  getWorkflowReadiness,
  getWorkflowServiceBlueprint,
  getWorkflowTestScenarios,
  isKnowledgeSupportWorkflowDefinition,
  isWorkflowDefinitionFunctionallyEquivalent,
  summarizeVersionChange,
  type WorkflowProductCapability,
  type WorkflowJourneyStage,
  type WorkflowServiceBlueprint,
  type WorkflowModelCredentialPreset,
  type WorkflowModelCredentialSource,
  type WorkflowTestScenario,
  type WorkflowTestRuntimeContext,
} from "./workflow-product-utils"

const workflowDetailI18nPrefix = "workflowExtract.enterpriseWorkflowDetail."
type WorkflowDetailT = ReturnType<typeof useI18n>
const wd = (t: WorkflowDetailT, key: string, values?: Record<string, string | number>) => t(`${workflowDetailI18nPrefix}${key}`, values)

function shortHash(value: string) {
  return value ? value.slice(0, 12) : "-"
}

type OptionPageState = {
  page: number
  limit: number
  total: number
}

const WORKFLOW_AGENT_OPTION_PAGE_SIZE = 50
const WORKFLOW_ADOPTION_PAGE_SIZE = 12
const WORKFLOW_VERSION_PAGE_SIZE = 12
const WORKFLOW_BLUEPRINT_TEMPLATE_PAGE_SIZE = 12
const emptyAgentOptionPage: OptionPageState = { page: 1, limit: WORKFLOW_AGENT_OPTION_PAGE_SIZE, total: 0 }
const emptyAdoptionPage: OptionPageState = { page: 1, limit: WORKFLOW_ADOPTION_PAGE_SIZE, total: 0 }
const emptyVersionPage: OptionPageState = { page: 1, limit: WORKFLOW_VERSION_PAGE_SIZE, total: 0 }

function unwrapEnterpriseAgentPage(
  response: Awaited<ReturnType<typeof fetchEnterpriseAIAgents>>,
  t: WorkflowDetailT,
  fallbackKey = "errors.testConfigLoadFailed",
) {
  if (!response.success || !response.data) {
    throw new Error(response.error?.message || wd(t, fallbackKey))
  }
  return response.data
}

function optionPageCount(page: OptionPageState) {
  return Math.max(1, Math.ceil(page.total / page.limit))
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

async function fetchWorkflowVersionPage(
  workflowId: number,
  page: number,
  limit: number,
  currentStableVersionId = 0,
) {
  const versionPage = await fetchEnterpriseAIWorkflowVersions(workflowId, { page, limit })
  let currentStableVersion: AIWorkflowVersion | null = null
  if (currentStableVersionId > 0 && !versionPage.results.some((item) => item.id === currentStableVersionId)) {
    const stablePage = await fetchEnterpriseAIWorkflowVersions(workflowId, {
      page: 1,
      limit: 1,
      id: currentStableVersionId,
    })
    currentStableVersion = stablePage.results[0] ?? null
  }
  return {
    latestVersionNumber: Math.max(
      0,
      ...versionPage.results.map((item) => item.version),
      currentStableVersion?.version ?? 0,
    ),
    page: versionPage.page,
    versions: mergeById(versionPage.results, currentStableVersion),
  }
}

type WorkflowDefinitionNode = AIWorkflowDefinition["nodes"][number]
type WorkflowDefinitionEdge = AIWorkflowDefinition["edges"][number]
type WorkflowNodeTone = "entry" | "analysis" | "routing" | "knowledge" | "model" | "human" | "tool" | "reply" | "end"
type WorkflowBatchTestCaseResult = {
  scenarioKey: string
  status: "pending" | "running" | "passed" | "failed"
  result?: AIWorkflowTestRunResult
  errorMessage?: string
}

type WorkflowGraphNodeData = Record<string, unknown> & {
  title: string
  subtitle: string
  nodeId: string
  typeLabel: string
  riskLabel: string
  tone: WorkflowNodeTone
  icon: LucideIcon
  sequence: number
  layer: number
  chip?: string
  runStatus?: string
  runDurationMs?: number
}

type WorkflowGraphNode = Node<WorkflowGraphNodeData, "workflowGraphNode">

type WorkflowBranchConfig = {
  id?: string
  name?: string
  targetNodeId?: string
  default?: boolean
}

type WorkflowGraphLayoutItem = {
  layer: number
  position: { x: number; y: number }
}

const WORKFLOW_GRAPH_NODE_WIDTH = 220
const WORKFLOW_GRAPH_NODE_HEIGHT = 88
const WORKFLOW_GRAPH_COLUMN_STEP = 272
const WORKFLOW_GRAPH_ROW_STEP = 148

const graphNodeToneClass: Record<WorkflowNodeTone, { accent: string; icon: string; badge: string }> = {
  entry: {
    accent: "bg-muted-foreground",
    icon: "bg-muted text-foreground",
    badge: "border-border bg-muted text-muted-foreground",
  },
  analysis: {
    accent: "bg-primary",
    icon: "bg-primary text-primary-foreground",
    badge: "border-primary/20 bg-primary/10 text-primary",
  },
  routing: {
    accent: "bg-amber-500",
    icon: "bg-amber-100 text-amber-800",
    badge: "border-amber-200 bg-amber-50 text-amber-800",
  },
  knowledge: {
    accent: "bg-primary",
    icon: "bg-primary/10 text-primary",
    badge: "border-primary/20 bg-primary/10 text-primary",
  },
  model: {
    accent: "bg-primary",
    icon: "bg-primary text-primary-foreground",
    badge: "border-primary/20 bg-primary/10 text-primary",
  },
  human: {
    accent: "bg-amber-500",
    icon: "bg-amber-100 text-amber-800",
    badge: "border-amber-200 bg-amber-50 text-amber-800",
  },
  tool: {
    accent: "bg-amber-500",
    icon: "bg-amber-100 text-amber-800",
    badge: "border-amber-200 bg-amber-50 text-amber-800",
  },
  reply: {
    accent: "bg-primary",
    icon: "bg-primary/10 text-primary",
    badge: "border-primary/20 bg-primary/10 text-primary",
  },
  end: {
    accent: "bg-muted-foreground",
    icon: "bg-muted text-foreground",
    badge: "border-border bg-muted text-muted-foreground",
  },
}

const graphLegendItems = [
  { label: "nodeLabels.legend.reception", color: "bg-primary", tones: ["entry", "analysis", "model", "reply", "end"] },
  { label: "nodeLabels.legend.routing", color: "bg-amber-400", tones: ["routing"] },
  { label: "nodeLabels.legend.knowledge", color: "bg-primary", tones: ["knowledge"] },
  { label: "nodeLabels.legend.actions", color: "bg-amber-500", tones: ["human", "tool"] },
] satisfies Array<{
  label: string
  color: string
  tones: WorkflowNodeTone[]
}>

const workflowBusinessNodeTypeLabels: Record<string, string> = {
  start: "nodeLabels.types.start",
  entry_context: "nodeLabels.types.entry_context",
  conversation_understanding: "nodeLabels.types.conversation_understanding",
  service_access_policy: "nodeLabels.types.service_access_policy",
  reply_policy: "nodeLabels.types.reply_policy",
  condition: "nodeLabels.types.condition",
  knowledge_retrieve: "nodeLabels.types.knowledge_retrieve",
  knowledge_merge: "nodeLabels.types.knowledge_merge",
  answerability_gate: "nodeLabels.types.answerability_gate",
  llm_reply: "nodeLabels.types.llm_reply",
  human_confirm: "nodeLabels.types.human_confirm",
  prepare_ticket_draft: "nodeLabels.types.prepare_ticket_draft",
  create_ticket: "nodeLabels.types.create_ticket",
  create_video_meeting: "nodeLabels.types.create_video_meeting",
  create_knowledge_candidate: "nodeLabels.types.create_knowledge_candidate",
  handoff_to_human: "nodeLabels.types.handoff_to_human",
  send_reply: "nodeLabels.types.send_reply",
  subflow: "nodeLabels.types.subflow",
  loop: "nodeLabels.types.loop",
  end: "nodeLabels.types.end",
}

function cleanWorkflowBusinessLabel(t: WorkflowDetailT, value: string) {
  const cleaned = value
    .replace(/\bAI\b\s*/gi, "")
    .replace(/\bRAG\b/gi, wd(t, "nodeLabels.sanitize.rag"))
    .replace(/\bLLM\b/gi, wd(t, "nodeLabels.sanitize.llm"))
    .replace(/智能问诊/g, wd(t, "nodeLabels.sanitize.intelligentDiagnosis"))
    .replace(/智能客服/g, wd(t, "nodeLabels.sanitize.intelligentReception"))
    .replace(/\s+/g, " ")
    .trim()
  return cleaned || value
}

function workflowBusinessNodeType(t: WorkflowDetailT, type: string, spec?: AIWorkflowNodeSpec, node?: WorkflowDefinitionNode) {
  if (type === "knowledge_retrieve" && node?.id === "tenant_retrieve_1") {
    return wd(t, "nodeLabels.types.tenant_knowledge_retrieve")
  }
  const labelKey = workflowBusinessNodeTypeLabels[type]
  return cleanWorkflowBusinessLabel(t, labelKey ? wd(t, labelKey) : spec?.title || wd(t, "nodeLabels.types.unknown"))
}

function workflowBusinessNodeTitle(t: WorkflowDetailT, node: WorkflowDefinitionNode, spec?: AIWorkflowNodeSpec) {
  return cleanWorkflowBusinessLabel(t, node.name || spec?.title || workflowBusinessNodeType(t, node.type, spec, node))
}

const workflowGraphNodeTypes = {
  workflowGraphNode: WorkflowGraphNodeCard,
}

function WorkflowGraphNodeCard({ data, selected }: NodeProps<WorkflowGraphNode>) {
  const t = useI18n()
  const tone = graphNodeToneClass[data.tone] ?? graphNodeToneClass.analysis
  const Icon = data.icon
  const runMeta = data.runStatus ? workflowTestNodeStatusMeta(t, data.runStatus) : null
  const RunIcon = runMeta?.Icon
  return (
    <div
      className={`relative h-[88px] w-[220px] overflow-hidden rounded-lg border bg-background shadow-[0_8px_24px_rgba(15,23,42,0.08)] transition-[border-color,box-shadow,transform] ${
        data.runStatus === "completed"
          ? "border-primary shadow-[0_10px_30px_color-mix(in_srgb,var(--primary)_18%,transparent)] ring-2 ring-primary/15"
          : data.runStatus === "failed"
            ? "border-destructive ring-2 ring-destructive/15"
            : selected
          ? "-translate-y-0.5 border-primary shadow-[0_10px_30px_color-mix(in_srgb,var(--primary)_20%,transparent)] ring-2 ring-primary/15"
          : "border-border/90"
      }`}
    >
      <div className={`absolute inset-x-0 top-0 h-1 ${tone.accent}`} />
      <Handle type="target" position={Position.Top} className="!size-2.5 !border-2 !border-background !bg-muted-foreground" />
      <div className="flex items-start gap-2.5 px-3 pb-2 pt-3.5">
        <span className={`flex size-8 shrink-0 items-center justify-center rounded-md ${tone.icon}`}>
          <Icon className="size-4" />
        </span>
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-center gap-1.5">
            <div className="min-w-0 flex-1 truncate text-sm font-semibold text-foreground">{data.title}</div>
            {data.chip ? (
              <span className={`shrink-0 rounded border px-1.5 py-0.5 text-rhd-2xs leading-none ${tone.badge}`}>{data.chip}</span>
            ) : null}
          </div>
          <div className="mt-1 flex items-center gap-1.5 text-rhd-2xs text-muted-foreground">
            <span className="font-mono">{String(data.sequence).padStart(2, "0")}</span>
            <span className="size-0.5 rounded-full bg-muted-foreground/50" />
            <span className="truncate">{data.typeLabel}</span>
          </div>
        </div>
      </div>
      <div className="absolute inset-x-3 bottom-0 flex h-6 items-center justify-between border-t text-rhd-2xs text-muted-foreground">
        <span>{wd(t, "graph.layer", { layer: data.layer + 1 })}</span>
        {runMeta && RunIcon ? (
          <span className={`flex items-center gap-1 font-medium ${runMeta.textClass}`}><RunIcon className="size-3" />{runMeta.label}{typeof data.runDurationMs === "number" ? ` · ${data.runDurationMs} ms` : ""}</span>
        ) : (
          <span className={data.riskLabel === wd(t, "nodeLabels.risk.high") ? "font-medium text-destructive" : undefined}>{data.riskLabel}</span>
        )}
      </div>
      <Handle type="source" position={Position.Bottom} className="!size-2.5 !border-2 !border-background !bg-primary" />
    </div>
  )
}

function workflowNodeMeta(t: WorkflowDetailT, node: WorkflowDefinitionNode, spec?: AIWorkflowNodeSpec) {
  const config = (node.config ?? {}) as Record<string, unknown>
  const staticReply = typeof config.staticReply === "string" ? config.staticReply.trim() : ""
  const fallbackTitle = workflowBusinessNodeTitle(t, node, spec)

  switch (node.type) {
    case "start":
      return {
        title: fallbackTitle,
        subtitle: wd(t, "nodeLabels.subtitle.entry"),
        tone: "entry" as const,
        icon: MessageSquareTextIcon,
        chip: wd(t, "nodeLabels.chip.entry"),
      }
    case "conversation_understanding":
      return {
        title: fallbackTitle,
        subtitle: wd(t, "nodeLabels.subtitle.understand"),
        tone: "analysis" as const,
        icon: ActivityIcon,
        chip: wd(t, "nodeLabels.chip.understand"),
      }
    case "entry_context":
      return {
        title: fallbackTitle,
        subtitle: wd(t, "nodeLabels.subtitle.entryIdentify"),
        tone: "entry" as const,
        icon: ScanLineIcon,
        chip: wd(t, "nodeLabels.chip.identify"),
      }
    case "service_access_policy":
      return {
        title: fallbackTitle,
        subtitle: wd(t, "nodeLabels.subtitle.accessPolicy"),
        tone: "routing" as const,
        icon: ShieldCheckIcon,
        chip: wd(t, "nodeLabels.chip.access"),
      }
    case "reply_policy":
      return {
        title: fallbackTitle,
        subtitle: wd(t, "nodeLabels.subtitle.replyPolicy"),
        tone: "analysis" as const,
        icon: MessageSquareTextIcon,
        chip: wd(t, "nodeLabels.chip.policy"),
      }
    case "condition":
      return {
        title: fallbackTitle,
        subtitle: wd(t, "nodeLabels.subtitle.condition"),
        tone: "routing" as const,
        icon: GitBranchIcon,
        chip: wd(t, "nodeLabels.chip.condition"),
      }
    case "knowledge_retrieve":
      return {
        title: fallbackTitle,
        subtitle: wd(t, node.id === "tenant_retrieve_1" ? "nodeLabels.subtitle.tenantKnowledgeRetrieve" : "nodeLabels.subtitle.knowledgeRetrieve"),
        tone: "knowledge" as const,
        icon: DatabaseIcon,
        chip: wd(t, "nodeLabels.chip.knowledge"),
      }
    case "knowledge_merge":
      return {
        title: fallbackTitle,
        subtitle: wd(t, "nodeLabels.subtitle.knowledgeMerge"),
        tone: "knowledge" as const,
        icon: BookOpenIcon,
        chip: wd(t, "nodeLabels.chip.merge"),
      }
    case "answerability_gate":
      return {
        title: fallbackTitle,
        subtitle: wd(t, "nodeLabels.subtitle.answerGate"),
        tone: "routing" as const,
        icon: CheckCircle2Icon,
        chip: wd(t, "nodeLabels.chip.gate"),
      }
    case "llm_reply":
      return {
        title: fallbackTitle,
        subtitle: staticReply ? wd(t, "nodeLabels.subtitle.staticReply") : wd(t, "nodeLabels.subtitle.generateReply"),
        tone: "model" as const,
        icon: MessageSquareTextIcon,
        chip: staticReply ? wd(t, "nodeLabels.chip.staticReply") : wd(t, "nodeLabels.chip.reply"),
      }
    case "human_confirm":
      return {
        title: fallbackTitle,
        subtitle: wd(t, "nodeLabels.subtitle.humanConfirm"),
        tone: "human" as const,
        icon: UsersIcon,
        chip: wd(t, "nodeLabels.chip.confirm"),
      }
    case "prepare_ticket_draft":
      return {
        title: fallbackTitle,
        subtitle: wd(t, "nodeLabels.subtitle.ticketDraft"),
        tone: "tool" as const,
        icon: WrenchIcon,
        chip: wd(t, "nodeLabels.chip.draft"),
      }
    case "create_ticket":
      return {
        title: fallbackTitle,
        subtitle: wd(t, "nodeLabels.subtitle.createTicket"),
        tone: "tool" as const,
        icon: TicketIcon,
        chip: wd(t, "nodeLabels.chip.ticket"),
      }
    case "create_video_meeting":
      return {
        title: fallbackTitle,
        subtitle: wd(t, "nodeLabels.subtitle.videoMeeting"),
        tone: "tool" as const,
        icon: VideoIcon,
        chip: wd(t, "nodeLabels.chip.video"),
      }
    case "create_knowledge_candidate":
      return {
        title: fallbackTitle,
        subtitle: wd(t, "nodeLabels.subtitle.knowledgeCandidate"),
        tone: "knowledge" as const,
        icon: BookOpenIcon,
        chip: wd(t, "nodeLabels.chip.candidate"),
      }
    case "handoff_to_human":
      return {
        title: fallbackTitle,
        subtitle: wd(t, "nodeLabels.subtitle.handoff"),
        tone: "tool" as const,
        icon: UsersIcon,
        chip: wd(t, "nodeLabels.chip.handoff"),
      }
    case "send_reply":
      return {
        title: fallbackTitle,
        subtitle: wd(t, "nodeLabels.subtitle.sendReply"),
        tone: "reply" as const,
        icon: ArrowRightIcon,
        chip: wd(t, "nodeLabels.chip.output"),
      }
    case "subflow":
      return {
        title: fallbackTitle,
        subtitle: wd(t, "nodeLabels.subtitle.subflow"),
        tone: "routing" as const,
        icon: NetworkIcon,
        chip: wd(t, "nodeLabels.chip.subflow"),
      }
    case "loop":
      return {
        title: fallbackTitle,
        subtitle: wd(t, "nodeLabels.subtitle.loop"),
        tone: "routing" as const,
        icon: RefreshCwIcon,
        chip: wd(t, "nodeLabels.chip.loop"),
      }
    case "end":
      return {
        title: fallbackTitle,
        subtitle: wd(t, "nodeLabels.subtitle.end"),
        tone: "end" as const,
        icon: CheckCircle2Icon,
        chip: wd(t, "nodeLabels.chip.end"),
      }
    default:
      return {
        title: fallbackTitle,
        subtitle: workflowBusinessNodeType(t, node.type, spec, node),
        tone: "analysis" as const,
        icon: NetworkIcon,
        chip: wd(t, "nodeLabels.chip.unknown"),
      }
  }
}

function workflowRiskLabel(t: WorkflowDetailT, spec?: AIWorkflowNodeSpec) {
  if (spec?.riskLevel === "high") return wd(t, "nodeLabels.risk.high")
  if (spec?.riskLevel === "medium") return wd(t, "nodeLabels.risk.medium")
  return wd(t, "nodeLabels.risk.low")
}

function getConditionBranches(node?: WorkflowDefinitionNode): WorkflowBranchConfig[] {
  const branches = (node?.config as Record<string, unknown> | undefined)?.branches
  return Array.isArray(branches) ? (branches as WorkflowBranchConfig[]) : []
}

function workflowEdgeLabel(t: WorkflowDetailT, edge: WorkflowDefinitionEdge, nodeById: Map<string, WorkflowDefinitionNode>) {
  const source = nodeById.get(edge.source)
  if (source?.errorTargetNodeId === edge.target) return wd(t, "nodeLabels.edge.onFailure")
  if (source?.type !== "condition") return undefined
  const branch = getConditionBranches(source).find((item) => item.targetNodeId === edge.target)
  if (!branch) return undefined
  return branch.default ? branch.name || wd(t, "nodeLabels.edge.defaultBranch") : branch.name || branch.id
}

function workflowGraphLayout(definition: AIWorkflowDefinition) {
  const nodeIndex = new Map(definition.nodes.map((node, index) => [node.id, index]))
  const nodeById = new Map(definition.nodes.map((node) => [node.id, node]))
  const incomingCount = new Map(definition.nodes.map((node) => [node.id, 0]))
  const outgoing = new Map(definition.nodes.map((node) => [node.id, [] as string[]]))

  definition.edges.forEach((edge) => {
    if (!nodeById.has(edge.source) || !nodeById.has(edge.target)) return
    incomingCount.set(edge.target, (incomingCount.get(edge.target) ?? 0) + 1)
    outgoing.get(edge.source)?.push(edge.target)
  })

  const rank = new Map<string, number>()
  const queue = definition.nodes
    .filter((node) => (incomingCount.get(node.id) ?? 0) === 0)
    .sort((left, right) => {
      if (left.id === definition.entryNodeId) return -1
      if (right.id === definition.entryNodeId) return 1
      return (nodeIndex.get(left.id) ?? 0) - (nodeIndex.get(right.id) ?? 0)
    })

  queue.forEach((node) => rank.set(node.id, node.id === definition.entryNodeId ? 0 : Math.max(0, Math.round((node.position?.x ?? 0) / 320))))

  const processed = new Set<string>()
  while (queue.length > 0) {
    const source = queue.shift()
    if (!source) break
    processed.add(source.id)
    const sourceRank = rank.get(source.id) ?? 0
    for (const targetId of outgoing.get(source.id) ?? []) {
      rank.set(targetId, Math.max(rank.get(targetId) ?? 0, sourceRank + 1))
      const nextIncomingCount = (incomingCount.get(targetId) ?? 1) - 1
      incomingCount.set(targetId, nextIncomingCount)
      if (nextIncomingCount === 0) {
        const target = nodeById.get(targetId)
        if (target) queue.push(target)
      }
    }
  }

  definition.nodes.forEach((node, index) => {
    if (processed.has(node.id)) return
    const authoredRank = Number.isFinite(node.position?.x) ? Math.round(node.position.x / 320) : index
    rank.set(node.id, Math.max(0, authoredRank))
  })

  const layers = new Map<number, WorkflowDefinitionNode[]>()
  definition.nodes.forEach((node) => {
    const layer = rank.get(node.id) ?? 0
    const items = layers.get(layer) ?? []
    items.push(node)
    layers.set(layer, items)
  })
  layers.forEach((items) => {
    items.sort((left, right) => {
      const yDifference = (left.position?.y ?? 0) - (right.position?.y ?? 0)
      return yDifference || (nodeIndex.get(left.id) ?? 0) - (nodeIndex.get(right.id) ?? 0)
    })
  })

  const widestLayer = Math.max(1, ...Array.from(layers.values(), (items) => items.length))
  const layout = new Map<string, WorkflowGraphLayoutItem>()
  layers.forEach((items, layer) => {
    const leftOffset = ((widestLayer - items.length) * WORKFLOW_GRAPH_COLUMN_STEP) / 2
    items.forEach((node, index) => {
      layout.set(node.id, {
        layer,
        position: {
          x: 36 + leftOffset + index * WORKFLOW_GRAPH_COLUMN_STEP,
          y: 36 + layer * WORKFLOW_GRAPH_ROW_STEP,
        },
      })
    })
  })
  return layout
}

function workflowGraphNodes(
  t: WorkflowDetailT,
  definition: AIWorkflowDefinition,
  specByType: Map<string, AIWorkflowNodeSpec>,
  layout: Map<string, WorkflowGraphLayoutItem>,
  selectedNodeId: string,
  testRun?: AIWorkflowTestRunResult | null,
): WorkflowGraphNode[] {
  const resultById = new Map((testRun?.nodes ?? []).map((item) => [item.nodeId, item]))
  return definition.nodes.map((node, index) => {
    const spec = specByType.get(node.type)
    const meta = workflowNodeMeta(t, node, spec)
    const layoutItem = layout.get(node.id) ?? {
      layer: index,
      position: { x: 36, y: 36 + index * WORKFLOW_GRAPH_ROW_STEP },
    }
    const runResult = resultById.get(node.id)
    return {
      id: node.id,
      type: "workflowGraphNode",
      position: layoutItem.position,
      selected: node.id === selectedNodeId,
      width: WORKFLOW_GRAPH_NODE_WIDTH,
      height: WORKFLOW_GRAPH_NODE_HEIGHT,
      style: {
        width: WORKFLOW_GRAPH_NODE_WIDTH,
        height: WORKFLOW_GRAPH_NODE_HEIGHT,
      },
      data: {
        title: meta.title,
        subtitle: meta.subtitle,
        nodeId: node.id,
        typeLabel: workflowBusinessNodeType(t, node.type, spec, node),
        riskLabel: workflowRiskLabel(t, spec),
        tone: meta.tone,
        icon: meta.icon,
        sequence: index + 1,
        layer: layoutItem.layer,
        chip: meta.chip,
        runStatus: runResult?.status,
        runDurationMs: runResult?.durationMs,
      },
    }
  })
}

function workflowGraphEdges(t: WorkflowDetailT, definition: AIWorkflowDefinition, selectedNodeId: string, testRun?: AIWorkflowTestRunResult | null): Edge[] {
  const nodeById = new Map(definition.nodes.map((node) => [node.id, node]))
  const executedEdges = new Set<string>()
  for (let index = 0; index + 1 < (testRun?.nodePath.length ?? 0); index += 1) {
    executedEdges.add(`${testRun?.nodePath[index]}:${testRun?.nodePath[index + 1]}`)
  }
  return definition.edges.map((edge) => {
    const label = workflowEdgeLabel(t, edge, nodeById)
    const highlighted = edge.source === selectedNodeId || edge.target === selectedNodeId
    const executed = executedEdges.has(`${edge.source}:${edge.target}`)
    const errorFallback = nodeById.get(edge.source)?.errorTargetNodeId === edge.target
    return {
      id: edge.id,
      source: edge.source,
      target: edge.target,
      type: "smoothstep",
      label,
      zIndex: highlighted ? 2 : 0,
      markerEnd: {
        type: MarkerType.ArrowClosed,
        width: highlighted || executed ? 20 : 17,
        height: highlighted || executed ? 20 : 17,
        color: executed ? "#059669" : highlighted ? "var(--primary)" : errorFallback ? "#d97706" : "var(--muted-foreground)",
      },
      style: {
        stroke: executed
          ? "#059669"
          : highlighted
          ? "var(--primary)"
          : errorFallback
            ? "#d97706"
            : "color-mix(in srgb, var(--muted-foreground) 48%, transparent)",
        strokeWidth: highlighted || executed ? 2.4 : 1.5,
        strokeDasharray: errorFallback ? "7 5" : undefined,
      },
      labelBgPadding: [6, 3],
      labelBgBorderRadius: 4,
      labelBgStyle: {
        fill: "var(--background)",
        fillOpacity: 0.96,
        stroke: highlighted ? "var(--primary)" : errorFallback ? "#d97706" : "var(--border)",
        strokeWidth: 1,
      },
      labelStyle: {
        fill: "var(--foreground)",
        fontSize: 11,
        fontWeight: 600,
      },
    }
  })
}

function WorkflowGraphInspector({
  node,
  spec,
  layoutItem,
  incomingEdges,
  outgoingEdges,
  nodeById,
}: {
  node?: WorkflowDefinitionNode
  spec?: AIWorkflowNodeSpec
  layoutItem?: WorkflowGraphLayoutItem
  incomingEdges: WorkflowDefinitionEdge[]
  outgoingEdges: WorkflowDefinitionEdge[]
  nodeById: Map<string, WorkflowDefinitionNode>
}) {
  const t = useI18n()
  if (!node) {
    return <aside className="border-t bg-background p-5 text-sm text-muted-foreground xl:border-l xl:border-t-0">{wd(t, "graph.noNode")}</aside>
  }

  const meta = workflowNodeMeta(t, node, spec)
  const tone = graphNodeToneClass[meta.tone] ?? graphNodeToneClass.analysis
  const Icon = meta.icon
  const nodeName = (nodeId: string) => {
    const item = nodeById.get(nodeId)
    return item ? workflowBusinessNodeTitle(t, item) : nodeId
  }

  return (
    <aside className="border-t bg-background xl:border-l xl:border-t-0">
      <div className="border-b px-5 py-5">
        <div className="flex items-start gap-3">
          <span className={`flex size-10 shrink-0 items-center justify-center rounded-md ${tone.icon}`}>
            <Icon className="size-5" />
          </span>
          <div className="min-w-0 flex-1">
            <div className="text-xs text-muted-foreground">{wd(t, "graph.layer", { layer: (layoutItem?.layer ?? 0) + 1 })}</div>
            <div className="mt-1 text-base font-semibold text-foreground">{meta.title}</div>
            <div className="mt-2 flex flex-wrap gap-1.5">
              <Badge variant="outline">{workflowBusinessNodeType(t, node.type, spec, node)}</Badge>
              <Badge variant={spec?.riskLevel === "high" ? "destructive" : "secondary"}>{workflowRiskLabel(t, spec)}</Badge>
            </div>
          </div>
        </div>
      </div>

      <div className="space-y-6 px-5 py-5">
        <section>
          <div className="text-xs font-medium text-muted-foreground">{wd(t, "graph.nodeResponsibility")}</div>
          <p className="mt-2 text-sm leading-6 text-foreground/80">{meta.subtitle}</p>
        </section>

        <div className="grid grid-cols-2 border-y">
          <div className="py-3 pr-3">
            <div className="text-xs text-muted-foreground">{wd(t, "graph.upstreamConnections")}</div>
            <div className="mt-1 text-lg font-semibold tabular-nums">{incomingEdges.length}</div>
          </div>
          <div className="border-l py-3 pl-4">
            <div className="text-xs text-muted-foreground">{wd(t, "graph.downstreamPaths")}</div>
            <div className="mt-1 text-lg font-semibold tabular-nums">{outgoingEdges.length}</div>
          </div>
        </div>

        {incomingEdges.length > 0 ? (
          <section>
            <div className="text-xs font-medium text-muted-foreground">{wd(t, "graph.from")}</div>
            <div className="mt-2 divide-y border-y">
              {incomingEdges.map((edge) => (
                <div key={edge.id} className="flex items-center gap-2 py-2.5 text-sm">
                  <ArrowRightIcon className="size-3.5 shrink-0 text-muted-foreground" />
                  <span className="truncate">{nodeName(edge.source)}</span>
                </div>
              ))}
            </div>
          </section>
        ) : null}

        {outgoingEdges.length > 0 ? (
          <section>
            <div className="text-xs font-medium text-muted-foreground">{wd(t, "graph.next")}</div>
            <div className="mt-2 divide-y border-y">
              {outgoingEdges.map((edge) => {
                const branchLabel = workflowEdgeLabel(t, edge, nodeById)
                return (
                  <div key={edge.id} className="flex items-center gap-2 py-2.5 text-sm">
                    <ArrowRightIcon className="size-3.5 shrink-0 text-primary" />
                    <span className="min-w-0 flex-1 truncate">{nodeName(edge.target)}</span>
                    {branchLabel ? <Badge variant="outline" className="shrink-0">{branchLabel}</Badge> : null}
                  </div>
                )
              })}
            </div>
          </section>
        ) : null}

      </div>
    </aside>
  )
}

function WorkflowGraphPreview({
  definition,
  nodeSpecs,
  testRun,
  fullViewport = false,
}: {
  definition: AIWorkflowDefinition
  nodeSpecs: AIWorkflowNodeSpec[]
  testRun?: AIWorkflowTestRunResult | null
  fullViewport?: boolean
}) {
  const t = useI18n()
  const knowledgeSupport = isKnowledgeSupportWorkflowDefinition(definition)
  const specByType = useMemo(() => new Map(nodeSpecs.map((item) => [item.type, item])), [nodeSpecs])
  const nodeById = useMemo(() => new Map(definition.nodes.map((node) => [node.id, node])), [definition.nodes])
  const layout = useMemo(() => workflowGraphLayout(definition), [definition])
  const [selectedNodeId, setSelectedNodeId] = useState(definition.entryNodeId || definition.nodes[0]?.id || "")
  const effectiveSelectedNodeId = nodeById.has(selectedNodeId)
    ? selectedNodeId
    : definition.entryNodeId || definition.nodes[0]?.id || ""
  const nodes = useMemo(
    () => workflowGraphNodes(t, definition, specByType, layout, effectiveSelectedNodeId, testRun),
    [definition, effectiveSelectedNodeId, layout, specByType, t, testRun],
  )
  const edges = useMemo(
    () => workflowGraphEdges(t, definition, effectiveSelectedNodeId, testRun),
    [definition, effectiveSelectedNodeId, t, testRun],
  )
  const initialFocusNodes = useMemo(
    () => nodes.filter((node) => Number(node.data.layer) <= 2),
    [nodes],
  )
  const [graphInstance, setGraphInstance] = useState<ReactFlowInstance<WorkflowGraphNode, Edge> | null>(null)
  const [graphZoom, setGraphZoom] = useState(1)
  const selectedNode = nodeById.get(effectiveSelectedNodeId)
  const selectedSpec = selectedNode ? specByType.get(selectedNode.type) : undefined
  const incomingEdges = definition.edges.filter((edge) => edge.target === effectiveSelectedNodeId)
  const outgoingEdges = definition.edges.filter((edge) => edge.source === effectiveSelectedNodeId)
  const branchingNodeCount = useMemo(() => {
    const counts = new Map<string, number>()
    definition.edges.forEach((edge) => counts.set(edge.source, (counts.get(edge.source) ?? 0) + 1))
    return Array.from(counts.values()).filter((count) => count > 1).length
  }, [definition.edges])
  const activeGraphLegendItems = useMemo(() => {
    const activeTones = new Set(
      definition.nodes.map((node) => workflowNodeMeta(t, node, specByType.get(node.type)).tone),
    )
    return graphLegendItems.filter((item) => item.tones.some((tone) => activeTones.has(tone)))
  }, [definition.nodes, specByType, t])

  function focusGraphEntry() {
    if (!graphInstance) return
    void graphInstance.fitView({ nodes: initialFocusNodes, padding: 0.12, minZoom: 1, maxZoom: 1, duration: 240 })
  }

  function fitEntireGraph() {
    if (!graphInstance) return
    void graphInstance.fitView({ padding: 0.1, minZoom: 0.18, maxZoom: 0.8, duration: 280 })
  }

  function zoomGraph(direction: "in" | "out") {
    if (!graphInstance) return
    if (direction === "in") void graphInstance.zoomIn({ duration: 180 })
    else void graphInstance.zoomOut({ duration: 180 })
  }

  return (
    <div className={fullViewport
      ? "flex min-h-0 flex-1 flex-col overflow-hidden bg-background"
      : "overflow-hidden rounded-lg border bg-background"}
    >
      <div className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-3">
        <div className="flex min-w-0 items-center gap-3">
          <span className="flex size-9 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
            <NetworkIcon className="size-4" />
          </span>
          <div className="min-w-0">
            <div className="text-sm font-semibold">{wd(t, "graph.servicePath")}</div>
          </div>
        </div>
        <div className="flex flex-wrap items-center justify-end gap-2">
          <div className="hidden items-center gap-3 2xl:flex">
            {activeGraphLegendItems.map((item) => (
              <span key={item.label} className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <span className={`size-2 rounded-sm ${item.color}`} />
                {wd(t, knowledgeSupport && item.label === "nodeLabels.legend.knowledge" ? "nodeLabels.legend.tenantKnowledge" : item.label)}
              </span>
            ))}
          </div>
          <span className="h-4 w-px bg-border" />
          <span className="text-xs text-muted-foreground">{wd(t, "graph.nodeCount", { count: definition.nodes.length })}</span>
          {branchingNodeCount > 0 ? (
            <span className="text-xs text-muted-foreground">{wd(t, "graph.branchPointCount", { count: branchingNodeCount })}</span>
          ) : null}
          <div className="ml-1 flex items-center gap-1.5" role="toolbar" aria-label={wd(t, "graph.controlAria")}>
            <div className="flex h-8 items-center overflow-hidden rounded-md border bg-background shadow-sm">
              <Button
                variant="ghost"
                size="icon-sm"
                className="rounded-none border-r"
                aria-label={wd(t, "graph.focusEntry")}
                title={wd(t, "graph.focusEntry")}
                disabled={!graphInstance}
                onClick={focusGraphEntry}
              >
                <LocateFixedIcon className="size-4" />
              </Button>
              <Button
                variant="ghost"
                size="icon-sm"
                className="rounded-none"
                aria-label={wd(t, "graph.fitAll")}
                title={wd(t, "graph.fitAll")}
                disabled={!graphInstance}
                onClick={fitEntireGraph}
              >
                <ScanLineIcon className="size-4" />
              </Button>
            </div>
            <Button
              size="icon"
              className="shadow-sm"
              aria-label={wd(t, "graph.zoomOut")}
              title={wd(t, "graph.zoomOut")}
              disabled={!graphInstance}
              onClick={() => zoomGraph("out")}
            >
              <ZoomOutIcon className="size-5" />
            </Button>
            <span className="flex h-8 min-w-14 items-center justify-center rounded-md border bg-muted/40 px-2 text-xs font-semibold tabular-nums text-foreground" aria-label={wd(t, "graph.zoomLabel", { percent: Math.round(graphZoom * 100) })}>
              {Math.round(graphZoom * 100)}%
            </span>
            <Button
              size="icon"
              className="shadow-sm"
              aria-label={wd(t, "graph.zoomIn")}
              title={wd(t, "graph.zoomIn")}
              disabled={!graphInstance}
              onClick={() => zoomGraph("in")}
            >
              <ZoomInIcon className="size-5" />
            </Button>
          </div>
        </div>
      </div>
      <div className={fullViewport
        ? "grid min-h-0 flex-1 overflow-auto xl:grid-cols-[minmax(0,1fr)_310px] xl:grid-rows-[minmax(0,1fr)] xl:overflow-hidden"
        : "grid xl:grid-cols-[minmax(0,1fr)_310px]"}
      >
        <div className={fullViewport
          ? "relative h-[58dvh] min-w-0 bg-muted/15 sm:h-[66dvh] xl:h-auto xl:min-h-0"
          : "relative h-[560px] min-w-0 bg-muted/15 sm:h-[640px] xl:h-[720px]"}
        >
          <ReactFlow
            className="[&_.react-flow__pane]:cursor-grab [&_.react-flow__pane.dragging]:cursor-grabbing"
            nodes={nodes}
            edges={edges}
            nodeTypes={workflowGraphNodeTypes}
            minZoom={0.18}
            maxZoom={1.35}
            defaultViewport={{ x: 0, y: 0, zoom: 1 }}
            nodesDraggable={false}
            nodesConnectable={false}
            elementsSelectable
            zoomOnScroll={false}
            panOnScroll={false}
            panOnDrag={[0, 1]}
            selectionOnDrag={false}
            onInit={(instance: ReactFlowInstance<WorkflowGraphNode, Edge>) => {
              setGraphInstance(instance)
              void instance.fitView({ nodes: initialFocusNodes, padding: 0.12, minZoom: 1, maxZoom: 1 }).then(() => {
                setGraphZoom(instance.getZoom())
              })
            }}
            onMoveEnd={(_, viewport) => setGraphZoom(viewport.zoom)}
            onNodeClick={(_, node) => setSelectedNodeId(node.id)}
            proOptions={{ hideAttribution: true }}
            aria-label={wd(t, "graph.previewAria")}
            ariaLabelConfig={{
              "controls.ariaLabel": wd(t, "graph.previewControlAria"),
              "controls.zoomIn.ariaLabel": wd(t, "graph.zoomIn"),
              "controls.zoomOut.ariaLabel": wd(t, "graph.zoomOut"),
              "controls.fitView.ariaLabel": wd(t, "graph.fitAll"),
            }}
          >
            <Background gap={24} size={0.8} color="var(--border)" />
            <MiniMap
              className="!bottom-4 !right-4 !h-24 !w-36 overflow-hidden !rounded-md !border !border-border/70 !bg-background/95 !shadow-lg"
              nodeColor="var(--primary)"
              nodeStrokeColor="var(--background)"
              maskColor="color-mix(in srgb, var(--muted) 72%, transparent)"
              pannable
              zoomable
              ariaLabel={wd(t, "graph.miniMap")}
            />
          </ReactFlow>
        </div>
        <WorkflowGraphInspector
          node={selectedNode}
          spec={selectedSpec}
          layoutItem={layout.get(effectiveSelectedNodeId)}
          incomingEdges={incomingEdges}
          outgoingEdges={outgoingEdges}
          nodeById={nodeById}
        />
      </div>
    </div>
  )
}

export function DefinitionPreview({
  definition,
  defaultOpen = false,
  nodeSpecs,
  testRun,
}: {
  definition: AIWorkflowDefinition
  defaultOpen?: boolean
  nodeSpecs: AIWorkflowNodeSpec[]
  testRun?: AIWorkflowTestRunResult | null
}) {
  const t = useI18n()
  const [previewOpen, setPreviewOpen] = useState(defaultOpen)
  const [previewView, setPreviewView] = useState("graph")
  const specByType = useMemo(
    () => new Map(nodeSpecs.map((item) => [item.type, item])),
    [nodeSpecs],
  )
  const inbound = useMemo(() => {
    const result = new Map<string, number>()
    definition.edges.forEach((edge) => result.set(edge.target, (result.get(edge.target) ?? 0) + 1))
    return result
  }, [definition.edges])

  return (
    <div>
      <section className="flex flex-col gap-3 border-y px-3 py-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 items-center gap-2.5">
          <span className="flex size-9 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
            <NetworkIcon className="size-4" />
          </span>
          <div className="min-w-0">
            <div className="text-sm font-semibold">{wd(t, "graph.overview")}</div>
            <p className="mt-1 text-xs text-muted-foreground">
              {wd(t, "graph.overviewStats", { nodes: definition.nodes.length, edges: definition.edges.length })}
            </p>
          </div>
        </div>
        <Button className="w-full shrink-0 sm:w-auto" onClick={() => setPreviewOpen(true)}>
          <ScanLineIcon />{wd(t, "graph.openOverview")}
        </Button>
      </section>

      <StandardModal
        open={previewOpen}
        onCancel={() => setPreviewOpen(false)}
        width="80vw"
        rootClassName="rhd-railops-workflow-preview-modal"
        title={<div className="min-w-0">
                  <span className="flex items-center gap-2">
                    <NetworkIcon className="size-4 text-primary" />{wd(t, "graph.overview")}
                  </span>
                  <div className="mt-2 flex flex-wrap gap-1.5">
                    <Badge variant="outline">{wd(t, "graph.nodeCount", { count: definition.nodes.length })}</Badge>
                    <Badge variant="outline">{wd(t, "graph.edgeCount", { count: definition.edges.length })}</Badge>
                  </div>
                </div>}
      >
        <div className="flex h-full min-h-0 flex-col overflow-hidden">
          <div className="shrink-0 pb-3">
            <UnderlineTabs
              ariaLabel={wd(t, "graph.viewAria")}
              items={getPreviewViewTabs(t)}
              value={previewView}
              onChange={(value) => setPreviewView(value)}
            />
          </div>
{ previewView === "graph" ? (
<div className="mt-0 flex min-h-0 flex-1 flex-col overflow-hidden">
              <WorkflowGraphPreview
                definition={definition}
                nodeSpecs={nodeSpecs}
                testRun={testRun}
                fullViewport
              />
            </div>
            ) : null}
            { previewView === "list" ? (
            <div className="mt-0 min-h-0 flex-1 overflow-auto">
              <Table>
                <TableHeader className="sticky top-0 z-10 bg-muted/95 backdrop-blur-sm">
                  <TableRow>
                    <TableHead className="w-24">{wd(t, "graph.columns.order")}</TableHead>
                    <TableHead className="min-w-56">{wd(t, "graph.columns.node")}</TableHead>
                    <TableHead className="min-w-48">{wd(t, "graph.columns.nodeType")}</TableHead>
                    <TableHead className="min-w-56">{wd(t, "graph.columns.inputSource")}</TableHead>
                    <TableHead className="w-28">{wd(t, "graph.columns.risk")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {definition.nodes.map((node, index) => {
                    const spec = specByType.get(node.type)
                    const sources = definition.edges.filter((edge) => edge.target === node.id).map((edge) => edge.source)
                    return (
                      <TableRow key={node.id}>
                        <TableCell className="font-mono text-xs">{String(index + 1).padStart(2, "0")}</TableCell>
                        <TableCell>
                          <div className="font-medium">{workflowBusinessNodeTitle(t, node, spec)}</div>
                        </TableCell>
                        <TableCell>
                          <Badge variant="secondary">{workflowBusinessNodeType(t, node.type, spec, node)}</Badge>
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                          {sources.length > 0 ? sources.join("、") : inbound.get(node.id) ? wd(t, "graph.source.upstream") : wd(t, "graph.source.context")}
                        </TableCell>
                        <TableCell>
                          <Badge variant={spec?.riskLevel === "high" ? "destructive" : "outline"}>
                            {spec?.riskLevel === "high" ? wd(t, "graph.risk.high") : spec?.riskLevel === "medium" ? wd(t, "graph.risk.medium") : wd(t, "graph.risk.low")}
                          </Badge>
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            </div>
            ) : null}
          
        </div>
      </StandardModal>
    </div>
  )
}

const journeyStageIcons = {
  receive: MessageSquareTextIcon,
  understand: ActivityIcon,
  knowledge: DatabaseIcon,
  answer: MessageSquareTextIcon,
  service_action: WrenchIcon,
  respond: CheckCircle2Icon,
} satisfies Record<string, LucideIcon>

const capabilityIcons = {
  knowledge: DatabaseIcon,
  ticket: TicketIcon,
  handoff: UsersIcon,
  video: VideoIcon,
  learning: BookOpenIcon,
} satisfies Record<WorkflowProductCapability["key"], LucideIcon>

const blueprintOrder: Record<WorkflowServiceBlueprint, number> = {
  basic_ai: 1,
  dispatch_only: 2,
  device_ai: 3,
  ai_human: 4,
}

const blueprintFacts: Record<WorkflowServiceBlueprint, string[]> = {
  basic_ai: ["blueprint.facts.basic_ai.0", "blueprint.facts.basic_ai.1", "blueprint.facts.basic_ai.2"],
  dispatch_only: ["blueprint.facts.dispatch_only.0", "blueprint.facts.dispatch_only.1", "blueprint.facts.dispatch_only.2"],
  device_ai: ["blueprint.facts.device_ai.0", "blueprint.facts.device_ai.1", "blueprint.facts.device_ai.2"],
  ai_human: ["blueprint.facts.ai_human.0", "blueprint.facts.ai_human.1", "blueprint.facts.ai_human.2"],
}

const workflowBlueprintLabels: Record<WorkflowServiceBlueprint, string> = {
  basic_ai: "blueprint.labels.basic_ai",
  dispatch_only: "blueprint.labels.dispatch_only",
  device_ai: "blueprint.labels.device_ai",
  ai_human: "blueprint.labels.ai_human",
}

const workflowStageIntent: Record<string, string> = {
  receive: "orchestration.stageIntent.receive",
  understand: "orchestration.stageIntent.understand",
  knowledge: "orchestration.stageIntent.knowledge",
  answer: "orchestration.stageIntent.answer",
  service_action: "orchestration.stageIntent.service_action",
  respond: "orchestration.stageIntent.respond",
}

const workflowStageActionText: Record<string, string> = {
  receive: "orchestration.stageAction.receive",
  understand: "orchestration.stageAction.understand",
  knowledge: "orchestration.stageAction.knowledge",
  answer: "orchestration.stageAction.answer",
  service_action: "orchestration.stageAction.service_action",
  respond: "orchestration.stageAction.respond",
}

const workflowStageBlueprintHint: Record<string, WorkflowServiceBlueprint> = {
  receive: "device_ai",
  understand: "device_ai",
  knowledge: "device_ai",
  answer: "basic_ai",
  service_action: "dispatch_only",
  respond: "dispatch_only",
}

function workflowBlueprintOptions(templates: AIWorkflow[]) {
  return Array.from(
    templates.reduce((result, template) => {
      if (template.scope !== "platform" || template.currentStableVersionId <= 0) return result
      const blueprint = getWorkflowServiceBlueprint(template.draftDefinition)
      if (!result.has(blueprint)) result.set(blueprint, template)
      return result
    }, new Map<WorkflowServiceBlueprint, AIWorkflow>()),
  ).sort(([left], [right]) => blueprintOrder[left] - blueprintOrder[right])
}

function workflowTemplateForBlueprint(templates: AIWorkflow[], blueprint: WorkflowServiceBlueprint) {
  return workflowBlueprintOptions(templates).find(([item]) => item === blueprint)?.[1]
}

function workflowBlueprintForCapability(capabilityKey: WorkflowProductCapability["key"]): WorkflowServiceBlueprint {
  if (capabilityKey === "ticket" || capabilityKey === "handoff") {
    return "dispatch_only"
  }
  if (capabilityKey === "video" || capabilityKey === "learning") {
    return "ai_human"
  }
  return "device_ai"
}

function workflowNodeConfigSummary(t: WorkflowDetailT, node: WorkflowDefinitionNode) {
  const branches = getConditionBranches(node)
  const inputCount = Object.keys(node.inputs ?? {}).length
  const config = (node.config ?? {}) as Record<string, unknown>
  const staticReply = typeof config.staticReply === "string" ? config.staticReply.trim() : ""
  const labels: string[] = []
  if (branches.length > 0) labels.push(wd(t, "orchestration.summaryBranches", { count: branches.length }))
  if (inputCount > 0) labels.push(wd(t, "orchestration.summaryInputs", { count: inputCount }))
  if (staticReply) labels.push(wd(t, "orchestration.staticScript"))
  if (node.errorTargetNodeId) labels.push(wd(t, "orchestration.hasFallback"))
  return labels.length > 0 ? labels.join(" · ") : wd(t, "orchestration.defaultPolicy")
}

function workflowNodeInputSummary(t: WorkflowDetailT, node: WorkflowDefinitionNode, nodeById: Map<string, WorkflowDefinitionNode>) {
  const inputs = Object.entries((node.inputs ?? {}) as Record<string, { nodeId: string; field: string }>)
  if (inputs.length === 0) return wd(t, "orchestration.useSessionContext")
  return inputs
    .slice(0, 3)
    .map(([field, selector]) => {
      const source = nodeById.get(selector.nodeId)
      return `${field} ← ${source ? workflowBusinessNodeTitle(t, source) : selector.nodeId}`
    })
    .join("，")
}

function workflowStageStatusLabel(t: WorkflowDetailT, stage: WorkflowJourneyStage, nodes: WorkflowDefinitionNode[]) {
  if (nodes.length === 0) return wd(t, "orchestration.pendingOrchestration")
  if (stage.key === "service_action") {
    const actionCount = nodes.filter((node) => ["prepare_ticket_draft", "create_ticket", "handoff_to_human", "create_video_meeting"].includes(node.type)).length
    return actionCount > 0 ? wd(t, "orchestration.stageActionCount", { count: actionCount }) : wd(t, "orchestration.pendingActions")
  }
  return wd(t, "orchestration.stageStepCount", { count: nodes.length })
}

function WorkflowBlueprintPicker({
  definition,
  loading = false,
  templates,
  onApply,
}: {
  definition: AIWorkflowDefinition
  loading?: boolean
  templates: AIWorkflow[]
  onApply: (template: AIWorkflow) => void
}) {
  const t = useI18n()
  const currentBlueprint = getWorkflowServiceBlueprint(definition)
  const options = workflowBlueprintOptions(templates)
  const hasBlueprintOptions = options.length > 0
  const showBlueprintSkeleton = loading && !hasBlueprintOptions

  return (
    <section className="border-y py-5" aria-label={wd(t, "blueprint.sectionTitle")}>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <div className="flex items-center gap-2 text-sm font-semibold">
            <LayoutTemplateIcon className="size-4 text-primary" />{wd(t, "blueprint.sectionTitle")}
          </div>
        </div>
        <Badge variant="outline">{wd(t, "blueprint.current", { title: getWorkflowProductProfile(definition).title })}</Badge>
      </div>
      <div className="mt-4 grid gap-3 lg:grid-cols-2">
        {showBlueprintSkeleton ? <ModuleLoading variant="list" count={3} label={wd(t, "blueprint.loading")} /> : null}
        {options.map(([blueprint, template]) => {
          const profile = getWorkflowProductProfile(template.draftDefinition)
          const sameType = blueprint === currentBlueprint
          const selected = isWorkflowDefinitionFunctionallyEquivalent(definition, template.draftDefinition)
          const credential = describeWorkflowModelCredentialChain(template.draftDefinition.modelPolicy?.credentialChain)
          const modeLabel = profile.mode === "ai_human"
            ? wd(t, "serviceModes.autoHuman")
            : profile.mode === "dispatch_only"
              ? wd(t, "serviceModes.dispatchOnly")
              : wd(t, "serviceModes.auto")
          return (
            <article
              key={blueprint}
              data-testid={`workflow-blueprint-${blueprint}`}
              className={`flex min-h-56 flex-col rounded-lg border p-4 transition-colors ${selected ? "border-primary bg-primary/5" : sameType ? "border-primary/30 bg-primary/5" : "bg-background hover:border-foreground/30"}`}
            >
              <div className="flex items-start justify-between gap-3">
                <span className={`flex size-9 shrink-0 items-center justify-center rounded-md ${selected ? "bg-primary text-primary-foreground" : sameType ? "bg-primary/10 text-primary" : "bg-muted text-foreground"}`}>
                  {blueprint === "ai_human" ? <UsersIcon className="size-4" /> : blueprint === "dispatch_only" ? <RouteIcon className="size-4" /> : blueprint === "device_ai" ? <WrenchIcon className="size-4" /> : <MessageSquareTextIcon className="size-4" />}
                </span>
                <Badge variant={selected ? "default" : "outline"}>
                  {selected ? wd(t, "blueprint.currentStandard") : sameType ? wd(t, "blueprint.customizedCurrentType") : modeLabel}
                </Badge>
              </div>
              <div className="mt-4 text-base font-semibold text-foreground">{profile.title}</div>
              <div className="mt-3 flex flex-wrap gap-1.5">
                {blueprintFacts[blueprint].map((fact) => <Badge key={fact} variant="outline" className="font-normal">{wd(t, fact)}</Badge>)}
              </div>
              <div className="mt-3 flex items-center gap-1.5 text-xs text-muted-foreground">
                <KeyRoundIcon className="size-3.5" />{credential.label}
              </div>
              <Button className="mt-auto" size="sm" variant={selected ? "secondary" : "outline"} disabled={selected} onClick={() => onApply(template)}>
                {selected ? <CheckCircle2Icon /> : <ArrowRightIcon />}
                {selected ? wd(t, "actions.inUse") : sameType ? wd(t, "blueprint.restoreStandard") : wd(t, "blueprint.applyThis")}
              </Button>
            </article>
          )
        })}
        {!loading && !hasBlueprintOptions ? (
          <div className="border-y py-10 text-center text-sm text-muted-foreground lg:col-span-2">{wd(t, "blueprint.noOptions")}</div>
        ) : null}
      </div>
    </section>
  )
}

function WorkflowServiceHero({
  workflow,
  definition,
  stableVersion,
  adoptionCount,
  currentVersionAdoptionCount,
}: {
  workflow: AIWorkflow
  definition: AIWorkflowDefinition
  stableVersion?: AIWorkflowVersion
  adoptionCount: number
  currentVersionAdoptionCount: number
}) {
  const t = useI18n()
  const knowledgeSupport = isKnowledgeSupportWorkflowDefinition(definition)
  const profile = getWorkflowProductProfile(definition)
  const serviceModeLabel = profile.mode === "ai_human"
    ? wd(t, "serviceModes.autoHuman")
    : profile.mode === "dispatch_only"
      ? wd(t, "serviceModes.dispatchOnly")
      : wd(t, "serviceModes.auto")
  const checks = getWorkflowReadiness(definition, workflow.currentStableVersionId, adoptionCount)
  const blockingCount = checks.filter((item) => item.state === "error").length
  const warningCount = checks.filter((item) => item.state === "warning").length
  const passCount = checks.filter((item) => item.state === "pass").length
  const readiness = Math.round((passCount / Math.max(1, checks.length)) * 100)
  const liveState = blockingCount > 0
    ? wd(t, "hero.cannotLaunch")
    : stableVersion
      ? adoptionCount > currentVersionAdoptionCount
        ? wd(t, knowledgeSupport ? "hero.configurationsToUpgrade" : "hero.productsToUpgrade")
        : adoptionCount > 0
          ? wd(t, "hero.productionRunning")
          : wd(t, "hero.pilotReady")
      : wd(t, "versionsPanel.pendingPublish")
  const serviceActions = profile.capabilities.filter((capability) => capability.state === "enabled")

  return (
    <section className="border-y bg-background px-4 py-3">
      <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <div className="text-base font-semibold text-foreground">{profile.title}</div>
            <Badge variant={workflow.scope === "platform" ? "default" : "secondary"}>
              {workflow.scope === "platform" ? wd(t, "scope.platform") : wd(t, "scope.tenant")}
            </Badge>
            {workflow.locked ? <Badge variant="outline"><LockKeyholeIcon />{wd(t, "status.readOnly")}</Badge> : null}
            <Badge variant={blockingCount > 0 ? "destructive" : warningCount > 0 ? "outline" : "default"}>
              {liveState}
            </Badge>
          </div>
          <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1.5 text-xs text-muted-foreground">
            <span>{serviceModeLabel}</span>
            <span>{wd(t, "hero.stageCount", { count: profile.journey.length })}</span>
            <span>{wd(t, "hero.actionCount", { count: serviceActions.length })}</span>
          </div>
        </div>
        <div className="grid gap-2 sm:grid-cols-4 xl:w-[34rem]">
          <div className="border-l px-3">
            <div className="text-rhd-xs text-muted-foreground">{wd(t, "releaseSettings.stableVersion")}</div>
            <div className="mt-1 text-sm font-semibold">{stableVersion ? `V${stableVersion.version}` : wd(t, "versionsPanel.pendingPublish")}</div>
          </div>
          <div className="border-l px-3">
            <div className="text-rhd-xs text-muted-foreground">{wd(t, knowledgeSupport ? "releaseSettings.enabledReception" : "releaseSettings.enabledProducts")}</div>
            <div className="mt-1 text-sm font-semibold tabular-nums">{adoptionCount}</div>
          </div>
          <div className="border-l px-3">
            <div className="text-rhd-xs text-muted-foreground">{wd(t, "hero.currentVersion")}</div>
            <div className="mt-1 text-sm font-semibold tabular-nums">{currentVersionAdoptionCount} / {adoptionCount}</div>
          </div>
          <div className="border-l px-3">
            <div className="text-rhd-xs text-muted-foreground">{wd(t, "hero.launchStatus")}</div>
            <div className="mt-1 flex items-center gap-2">
              <span className="text-sm font-semibold tabular-nums">{readiness}%</span>
              <span className="h-1.5 min-w-12 flex-1 overflow-hidden rounded-full bg-muted">
                <span className={`block h-full rounded-full ${blockingCount > 0 ? "bg-destructive" : warningCount > 0 ? "bg-amber-500" : "bg-primary"}`} style={{ width: `${readiness}%` }} />
              </span>
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}

function WorkflowModelCredentialPolicy({
  definition,
  editable,
  saving,
  onChange,
  onSave,
}: {
  definition: AIWorkflowDefinition
  editable: boolean
  saving: boolean
  onChange: (value: WorkflowModelCredentialSource[]) => void
  onSave: () => void
}) {
  const t = useI18n()
  const profile = getWorkflowProductProfile(definition)
  if (profile.mode === "dispatch_only") {
    return (
      <section className="grid gap-4 border-y py-5 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center">
        <div>
          <div className="flex items-center gap-2 text-sm font-semibold">
            <KeyRoundIcon className="size-4 text-primary" />{wd(t, "versionsPanel.credential")}
          </div>
          <p className="mt-1 text-xs leading-5 text-muted-foreground">
            {wd(t, "credentialPolicy.noModelAccount")}
          </p>
        </div>
        <Badge variant="outline" className="w-fit">{wd(t, "credentialPolicy.notNeeded")}</Badge>
      </section>
    )
  }
  const chain = definition.modelPolicy?.credentialChain
  const preset = getWorkflowModelCredentialPreset(chain)
  const presentation = describeWorkflowModelCredentialChain(chain)

  return (
    <section className="grid gap-4 border-y py-5 lg:grid-cols-[minmax(0,1fr)_minmax(16rem,0.55fr)_auto] lg:items-end">
      <div>
        <div className="flex items-center gap-2 text-sm font-semibold">
          <KeyRoundIcon className="size-4 text-primary" />{wd(t, "versionsPanel.credential")}
        </div>
      </div>
      <div className="space-y-2">
        <label className="text-xs text-muted-foreground" htmlFor="workflow-model-credential-scope">{wd(t, "credentialPolicy.chargeOrder")}</label>
        <SelectField
          style={{ marginBottom: 0 }}
          selectProps={{
            id: "workflow-model-credential-scope",
            "aria-label": wd(t, "versionsPanel.credential"),
            value: preset,
            onChange: (value) => {
              const nextPreset = value as WorkflowModelCredentialPreset
              if (nextPreset !== "custom") onChange(getWorkflowModelCredentialChain(nextPreset))
            },
            disabled: !editable,
            options: [
              { value: preset, label: presentation.label, disabled: preset === "custom" },
              { value: "product_then_tenant", label: wd(t, "credentialPolicy.productThenTenant") },
              { value: "tenant_then_product", label: wd(t, "credentialPolicy.tenantThenProduct") },
              { value: "product_only", label: wd(t, "credentialPolicy.productOnly") },
              { value: "tenant_only", label: wd(t, "credentialPolicy.tenantOnly") },
              ...(preset === "custom" ? [{ value: "custom", label: wd(t, "credentialPolicy.custom"), disabled: true }] : []),
            ],
            style: { width: "100%" },
          }}
        />
      </div>
      {editable ? (
        <Button variant="outline" onClick={onSave} disabled={saving}>
          {saving ? <Loader2Icon className="animate-spin" /> : <CheckCircle2Icon />}
          {wd(t, "credentialPolicy.save")}
        </Button>
      ) : (
        <Badge variant="outline" className="w-fit">{wd(t, "credentialPolicy.platformPreset")}</Badge>
      )}
    </section>
  )
}

function WorkflowBasicSettings({
  name,
  description,
  onNameChange,
  onDescriptionChange,
}: {
  name: string
  description: string
  onNameChange: (value: string) => void
  onDescriptionChange: (value: string) => void
}) {
  const t = useI18n()
  return (
    <section className="grid gap-4 border-y py-5 lg:grid-cols-[minmax(0,0.55fr)_minmax(0,1fr)]">
      <div className="space-y-2">
        <label htmlFor="workflow-name" className="text-sm font-medium">{wd(t, "basicSettings.name")}</label>
        <Input id="workflow-name" value={name} onChange={(event) => onNameChange(event.target.value)} />
      </div>
      <div className="space-y-2">
        <label htmlFor="workflow-description" className="text-sm font-medium">{wd(t, "basicSettings.description")}</label>
        <Textarea id="workflow-description" rows={2} value={description} onChange={(event) => onDescriptionChange(event.target.value)} />
      </div>
    </section>
  )
}

function WorkflowOrchestrationPanel({
  definition,
  editable,
  templates,
  templatesLoading,
  onBlueprintApply,
  onDefinitionChange,
}: {
  definition: AIWorkflowDefinition
  editable: boolean
  templates: AIWorkflow[]
  templatesLoading: boolean
  onBlueprintApply: (template: AIWorkflow) => void
  onDefinitionChange: (definition: AIWorkflowDefinition) => void
}) {
  const t = useI18n()
  const profile = useMemo(() => getWorkflowProductProfile(definition), [definition])
  const knowledgeSupport = isKnowledgeSupportWorkflowDefinition(definition)
  const currentBlueprint = getWorkflowServiceBlueprint(definition)
  const blueprintOptions = useMemo(() => workflowBlueprintOptions(templates), [templates])
  const nodeById = useMemo(() => new Map(definition.nodes.map((node) => [node.id, node])), [definition.nodes])
  const stageItems = useMemo(() => profile.journey.map((stage) => {
    const nodes = definition.nodes.filter((node) => stage.nodeTypes.includes(node.type))
    return {
      ...stage,
      nodes,
      branchCount: nodes.reduce((count, node) => count + getConditionBranches(node).length, 0),
    }
  }), [definition.nodes, profile.journey])
  const firstStageKey = stageItems[0]?.key ?? "receive"
  const stageSignature = stageItems.map((stage) => `${stage.key}:${stage.nodes.map((node) => node.id).join(",")}`).join("|")
  const [selectedStageKey, setSelectedStageKey] = useState(firstStageKey)
  const selectedStage = stageItems.find((stage) => stage.key === selectedStageKey) ?? stageItems[0]
  const firstNodeId = selectedStage?.nodes[0]?.id ?? ""
  const [selectedNodeId, setSelectedNodeId] = useState(firstNodeId)
  const selectedNode = selectedStage?.nodes.find((node) => node.id === selectedNodeId) ?? selectedStage?.nodes[0]
  const selectedNodeBranches = getConditionBranches(selectedNode)
  const selectedNodeConfig = (selectedNode?.config ?? {}) as Record<string, unknown>
  const selectedNodeStaticReply = typeof selectedNodeConfig.staticReply === "string" ? selectedNodeConfig.staticReply : ""

  useEffect(() => {
    const nextStage = stageItems.find((stage) => stage.key === selectedStageKey) ?? stageItems[0]
    if (nextStage && nextStage.key !== selectedStageKey) {
      setSelectedStageKey(nextStage.key)
      setSelectedNodeId(nextStage.nodes[0]?.id ?? "")
      return
    }
    if (nextStage && nextStage.nodes.length > 0 && !nextStage.nodes.some((node) => node.id === selectedNodeId)) {
      setSelectedNodeId(nextStage.nodes[0].id)
    }
  }, [firstStageKey, selectedNodeId, selectedStageKey, stageItems, stageSignature])

  const updateNode = useCallback((nodeId: string, patch: Partial<WorkflowDefinitionNode>) => {
    if (!editable) return
    onDefinitionChange({
      ...definition,
      nodes: definition.nodes.map((node) => node.id === nodeId ? { ...node, ...patch } : node),
    })
  }, [definition, editable, onDefinitionChange])

  const updateNodeConfig = useCallback((nodeId: string, patch: Record<string, unknown>) => {
    if (!editable) return
    onDefinitionChange({
      ...definition,
      nodes: definition.nodes.map((node) => node.id === nodeId
        ? { ...node, config: { ...(node.config ?? {}), ...patch } }
        : node),
    })
  }, [definition, editable, onDefinitionChange])

  const updateBranchTarget = useCallback((nodeId: string, branchIndex: number, targetNodeId: string) => {
    if (!editable || !targetNodeId) return
    const sourceNode = definition.nodes.find((node) => node.id === nodeId)
    const branches = getConditionBranches(sourceNode)
    const previousTargetId = branches[branchIndex]?.targetNodeId ?? ""
    const nextBranches = branches.map((branch, index) => index === branchIndex ? { ...branch, targetNodeId } : branch)
    const previousTargetStillUsed = Boolean(previousTargetId && nextBranches.some((branch, index) => index !== branchIndex && branch.targetNodeId === previousTargetId))
    const nextEdges = [
      ...definition.edges.filter((edge) => !(edge.source === nodeId && edge.target === previousTargetId && !previousTargetStillUsed)),
      ...(definition.edges.some((edge) => edge.source === nodeId && edge.target === targetNodeId)
        ? []
        : [{ id: `${nodeId}_${targetNodeId}`, source: nodeId, target: targetNodeId }]),
    ]
    onDefinitionChange({
      ...definition,
      nodes: definition.nodes.map((node) => node.id === nodeId
        ? { ...node, config: { ...(node.config ?? {}), branches: nextBranches } }
        : node),
      edges: nextEdges,
    })
  }, [definition, editable, onDefinitionChange])

  const applyBlueprint = useCallback((blueprint: WorkflowServiceBlueprint) => {
    const template = workflowTemplateForBlueprint(templates, blueprint)
    if (!template) {
      toast.info(templatesLoading ? wd(t, "blueprint.loading") : wd(t, "blueprint.noneAvailable"))
      return
    }
    onBlueprintApply(template)
  }, [onBlueprintApply, templates, templatesLoading])

  return (
    <section className="rhd-railops-workflow-orchestrator" aria-label={wd(t, "orchestration.aria")}>
      <div className="rhd-railops-workflow-orchestrator-head">
        <div className="min-w-0">
          <div className="rhd-railops-workflow-section-title">
            <RouteIcon className="size-4" />{wd(t, "orchestration.title")}
          </div>
          <div className="mt-1 text-xs text-muted-foreground">
            {wd(t, "orchestration.hint")}
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant="outline">{wd(t, "orchestration.totalStages", { count: profile.journey.length })}</Badge>
          <Badge variant="outline">{wd(t, "orchestration.totalSteps", { count: definition.nodes.length })}</Badge>
          <Badge variant="outline">{wd(t, "orchestration.totalEdges", { count: definition.edges.length })}</Badge>
          <Badge variant={editable ? "secondary" : "outline"}>{editable ? wd(t, "orchestration.editable") : wd(t, "status.readOnly")}</Badge>
        </div>
      </div>

      <div className="rhd-railops-workflow-orchestrator-grid">
        <aside className="rhd-railops-workflow-stage-list" aria-label={wd(t, "orchestration.stagesAria")}>
          {stageItems.map((stage, index) => {
            const Icon = journeyStageIcons[stage.key as keyof typeof journeyStageIcons] ?? NetworkIcon
            const active = selectedStage?.key === stage.key
            return (
              <button
                key={stage.key}
                type="button"
                className={`rhd-railops-workflow-stage-button ${active ? "is-active" : ""}`}
                onClick={() => {
                  setSelectedStageKey(stage.key)
                  setSelectedNodeId(stage.nodes[0]?.id ?? "")
                }}
              >
                <span className="rhd-railops-workflow-stage-index">{String(index + 1).padStart(2, "0")}</span>
                <span className="rhd-railops-workflow-stage-icon"><Icon className="size-4" /></span>
                <span className="min-w-0 flex-1">
                  <span className="rhd-railops-workflow-stage-title">{stage.title}</span>
                  <span className="rhd-railops-workflow-stage-meta">
                    {workflowStageStatusLabel(t, stage, stage.nodes)}
                    {stage.branchCount > 0 ? ` · ${wd(t, "orchestration.stageBranchCount", { count: stage.branchCount })}` : ""}
                  </span>
                </span>
              </button>
            )
          })}
        </aside>

        <section className="rhd-railops-workflow-config-panel">
          {selectedStage ? (
            <>
              <div className="rhd-railops-workflow-panel-head">
                <div className="min-w-0">
                  <div className="rhd-railops-workflow-section-title">{selectedStage.title}</div>
                  <p>{wd(t, knowledgeSupport && selectedStage.key === "receive"
                    ? "orchestration.knowledgeStageIntent.receive"
                    : knowledgeSupport && selectedStage.key === "knowledge"
                      ? "orchestration.knowledgeStageIntent.knowledge"
                      : workflowStageIntent[selectedStage.key] ?? "orchestration.stageHintFallback")}</p>
                </div>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={!editable}
                  onClick={() => applyBlueprint(workflowStageBlueprintHint[selectedStage.key] ?? currentBlueprint)}
                >
                  <LayoutTemplateIcon />
                  {wd(t, workflowStageActionText[selectedStage.key] ?? "orchestration.adjustPlan")}
                </Button>
              </div>

              <div className="rhd-railops-workflow-node-list">
                {selectedStage.nodes.map((node) => {
                  const meta = workflowNodeMeta(t, node)
                  const Icon = meta.icon
                  const active = selectedNode?.id === node.id
                  return (
                    <button
                      key={node.id}
                      type="button"
                      className={`rhd-railops-workflow-node-row ${active ? "is-active" : ""}`}
                      onClick={() => setSelectedNodeId(node.id)}
                    >
                      <span className="rhd-railops-workflow-node-icon"><Icon className="size-4" /></span>
                      <span className="min-w-0 flex-1">
                        <span className="rhd-railops-workflow-node-title">{workflowBusinessNodeTitle(t, node)}</span>
                        <span className="rhd-railops-workflow-node-meta">{meta.subtitle} · {workflowNodeConfigSummary(t, node)}</span>
                      </span>
                      <span className="rhd-railops-workflow-node-action">{active ? wd(t, "actions.adjusting") : wd(t, "actions.adjust")}</span>
                    </button>
                  )
                })}
                {selectedStage.nodes.length === 0 ? (
                  <div className="rhd-railops-workflow-empty">{wd(t, "orchestration.emptyStage")}</div>
                ) : null}
              </div>

              {selectedNode ? (
                <div className="rhd-railops-workflow-node-config">
                  <div className="rhd-railops-workflow-config-title">
                    <span>{wd(t, "orchestration.stepConfigTitle")}</span>
                    <Badge variant="secondary">{workflowBusinessNodeType(t, selectedNode.type, undefined, selectedNode)}</Badge>
                  </div>
                  <div className="grid gap-3 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
                    <div className="space-y-2">
                      <label htmlFor={`workflow-node-name-${selectedNode.id}`}>{wd(t, "orchestration.stepName")}</label>
                      <Input
                        id={`workflow-node-name-${selectedNode.id}`}
                        value={workflowBusinessNodeTitle(t, selectedNode)}
                        disabled={!editable}
                        onChange={(event) => updateNode(selectedNode.id, { name: event.target.value })}
                      />
                    </div>
                    <div className="space-y-2">
                      <label>{wd(t, "orchestration.inputSource")}</label>
                      <div className="rhd-railops-workflow-readonly-field">{workflowNodeInputSummary(t, selectedNode, nodeById)}</div>
                    </div>
                  </div>
                  {selectedNode.type === "llm_reply" ? (
                    <div className="mt-3 space-y-2">
                      <label htmlFor={`workflow-static-reply-${selectedNode.id}`}>{wd(t, "orchestration.customerPhrase")}</label>
                      <Textarea
                        id={`workflow-static-reply-${selectedNode.id}`}
                        rows={3}
                        value={selectedNodeStaticReply}
                        disabled={!editable}
                        onChange={(event) => updateNodeConfig(selectedNode.id, { staticReply: event.target.value })}
                      />
                    </div>
                  ) : null}
                  {selectedNodeBranches.length > 0 ? (
                    <div className="mt-4">
                      <div className="rhd-railops-workflow-config-title">
                        <span>{wd(t, "orchestration.branchTargets")}</span>
                        <Badge variant="outline">{wd(t, "orchestration.branchCountUnit", { count: selectedNodeBranches.length })}</Badge>
                      </div>
                      <div className="rhd-railops-workflow-branch-list">
                        {selectedNodeBranches.map((branch, index) => {
                          const target = branch.targetNodeId ? nodeById.get(branch.targetNodeId) : undefined
                          const branchLabel = branch.name || branch.id || wd(t, "nodeLabels.branchName", { index: index + 1 })
                          return (
                            <div key={`${branch.id || "branch"}:${index}`} className="rhd-railops-workflow-branch-row">
                              <div className="min-w-0">
                                <div className="rhd-railops-workflow-branch-name">{branchLabel}</div>
                                <div className="rhd-railops-workflow-branch-meta">{branch.default ? wd(t, "orchestration.defaultPath") : wd(t, "orchestration.conditionPath")}</div>
                              </div>
                              <SelectField
                                style={{ marginBottom: 0 }}
                                selectProps={{
                                  "aria-label": wd(t, "orchestration.branchAria", { name: branchLabel }),
                                  value: branch.targetNodeId || "__none__",
                                  onChange: (value) => {
                                    if (value !== "__none__") updateBranchTarget(selectedNode.id, index, value)
                                  },
                                  disabled: !editable,
                                  options: [
                                    { value: "__none__", label: wd(t, "orchestration.unsetTarget"), disabled: true },
                                    ...definition.nodes.filter((node) => node.id !== selectedNode.id).map((node) => ({ value: node.id, label: workflowBusinessNodeTitle(t, node) })),
                                  ],
                                  style: { width: 224 },
                                }}
                              />
                            </div>
                          )
                        })}
                      </div>
                    </div>
                  ) : null}
                </div>
              ) : null}
            </>
          ) : (
            <div className="rhd-railops-workflow-empty">{wd(t, "orchestration.emptyFlow")}</div>
          )}
        </section>

        <aside className="rhd-railops-workflow-action-panel">
          <section>
            <div className="rhd-railops-workflow-section-title">
              <WrenchIcon className="size-4" />{wd(t, "orchestration.serviceActions")}
            </div>
            <div className="mt-3 grid gap-2">
              {profile.capabilities.map((capability) => {
                const Icon = capabilityIcons[capability.key]
                const enabled = capability.state === "enabled"
                const suggestedBlueprint = workflowBlueprintForCapability(capability.key)
                return (
                  <div key={capability.key} className="rhd-railops-workflow-action-row">
                    <span className={`rhd-railops-workflow-action-icon ${enabled ? "is-enabled" : ""}`}>
                      <Icon className="size-4" />
                    </span>
                    <div className="min-w-0 flex-1">
                      <div className="rhd-railops-workflow-action-title">{capability.title}</div>
                      <div className="rhd-railops-workflow-action-meta">{capability.stateLabel}</div>
                    </div>
                    <Button
                      size="sm"
                      variant={enabled ? "outline" : "secondary"}
                      disabled={!editable || templatesLoading || (enabled && currentBlueprint === suggestedBlueprint)}
                      onClick={() => applyBlueprint(suggestedBlueprint)}
                    >
                      {enabled ? wd(t, "actions.adjust") : wd(t, "actions.enable")}
                    </Button>
                  </div>
                )
              })}
            </div>
          </section>

          <section className="rhd-railops-workflow-template-switcher">
            <div className="rhd-railops-workflow-section-title">
              <LayoutTemplateIcon className="size-4" />{wd(t, "orchestration.servicePlans")}
            </div>
            <div className="mt-3 grid gap-2">
              {templatesLoading && blueprintOptions.length === 0 ? <ModuleLoading variant="list" count={3} label={wd(t, "orchestration.plansLoading")} /> : null}
              {blueprintOptions.map(([blueprint, template]) => {
                const selected = isWorkflowDefinitionFunctionallyEquivalent(definition, template.draftDefinition)
                return (
                  <button
                    key={blueprint}
                    type="button"
                    className={`rhd-railops-workflow-template-option ${selected ? "is-active" : ""}`}
                    disabled={!editable || selected}
                    onClick={() => onBlueprintApply(template)}
                  >
                    <span className="min-w-0">
                      <span>{wd(t, workflowBlueprintLabels[blueprint])}</span>
                      <span>{blueprintFacts[blueprint].map((fact) => wd(t, fact)).join(" · ")}</span>
                    </span>
                    <Badge variant={selected ? "default" : "outline"}>{selected ? wd(t, "actions.inUse") : wd(t, "actions.replace")}</Badge>
                  </button>
                )
              })}
              {!templatesLoading && blueprintOptions.length === 0 ? (
                <div className="rhd-railops-workflow-empty">{wd(t, "orchestration.noReplacePlans")}</div>
              ) : null}
            </div>
          </section>
        </aside>
      </div>
    </section>
  )
}
function WorkflowCapabilitySettings({
  definition,
  editable,
}: {
  definition: AIWorkflowDefinition
  editable: boolean
}) {
  const t = useI18n()
  const profile = getWorkflowProductProfile(definition)
  const knowledgeSupport = isKnowledgeSupportWorkflowDefinition(definition)
  const capabilityByKey = new Map(profile.capabilities.map((capability) => [capability.key, capability]))
  const rows: Array<{
    key: WorkflowProductCapability["key"]
    title: string
    description: string
    icon: LucideIcon
  }> = [
    {
      key: "knowledge",
      title: wd(t, knowledgeSupport ? "capabilitySettings.rows.tenantKnowledge.title" : "capabilitySettings.rows.knowledge.title"),
      description: wd(t, knowledgeSupport ? "capabilitySettings.rows.tenantKnowledge.description" : "capabilitySettings.rows.knowledge.description"),
      icon: DatabaseIcon,
    },
    {
      key: "ticket",
      title: wd(t, "capabilitySettings.rows.ticket.title"),
      description: wd(t, "capabilitySettings.rows.ticket.description"),
      icon: TicketIcon,
    },
    {
      key: "handoff",
      title: wd(t, "capabilitySettings.rows.handoff.title"),
      description: wd(t, "capabilitySettings.rows.handoff.description"),
      icon: UsersIcon,
    },
    {
      key: "video",
      title: wd(t, "capabilitySettings.rows.video.title"),
      description: wd(t, "capabilitySettings.rows.video.description"),
      icon: VideoIcon,
    },
    {
      key: "learning",
      title: wd(t, "capabilitySettings.rows.learning.title"),
      description: wd(t, "capabilitySettings.rows.learning.description"),
      icon: BookOpenIcon,
    },
  ]

  return (
    <section className="border-y py-5">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2 text-sm font-semibold">
          <ListChecksIcon className="size-4 text-primary" />{wd(t, "capabilitySettings.title")}
        </div>
        <Badge variant="outline">{editable ? wd(t, "capabilitySettings.adjustByPlan") : wd(t, "capabilitySettings.readOnlyConfig")}</Badge>
      </div>
      <div className="mt-4 grid gap-2 lg:grid-cols-2">
        {rows.map((row) => {
          const capability = capabilityByKey.get(row.key)
          const enabled = capability?.state === "enabled"
          const prohibited = capability?.state === "prohibited"
          const Icon = row.icon
          return (
            <div key={row.key} className="grid gap-3 rounded-md border bg-background p-3 sm:grid-cols-[auto_minmax(0,1fr)_auto] sm:items-center">
              <span className={`flex size-9 shrink-0 items-center justify-center rounded-md ${enabled ? "bg-primary/10 text-primary" : prohibited ? "bg-foreground text-primary-foreground" : "bg-muted text-muted-foreground"}`}>
                <Icon className="size-4" />
              </span>
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-sm font-medium">{row.title}</span>
                  <Badge variant={enabled ? "default" : prohibited ? "secondary" : "outline"} className="h-5 px-1.5 text-rhd-2xs">
                    {capability?.stateLabel ?? wd(t, "capabilitySettings.notConfigured")}
                  </Badge>
                </div>
                <p className="mt-1 text-xs leading-5 text-muted-foreground">{row.description}</p>
              </div>
              <Switch
                checked={enabled}
                disabled
                aria-label={enabled ? wd(t, "capabilitySettings.switchAriaOn", { title: row.title }) : wd(t, "capabilitySettings.switchAriaOff", { title: row.title })}
              />
            </div>
          )
        })}
      </div>
      {editable ? (
        <p className="mt-3 text-xs leading-5 text-muted-foreground">
          {wd(t, "capabilitySettings.hint")}
        </p>
      ) : null}
    </section>
  )
}

function WorkflowReadiness({
  definition,
  stableVersionId,
  adoptionCount,
}: {
  definition: AIWorkflowDefinition
  stableVersionId: number
  adoptionCount: number
}) {
  const t = useI18n()
  const checks = getWorkflowReadiness(definition, stableVersionId, adoptionCount)
  const blockingCount = checks.filter((item) => item.state === "error").length
  const passCount = checks.filter((item) => item.state === "pass").length
  const readiness = Math.round((passCount / checks.length) * 100)

  return (
    <section>
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2 text-sm font-semibold">
          <ClipboardCheckIcon className="size-4 text-primary" />{wd(t, "readiness.title")}
        </div>
        <span className="text-sm font-semibold tabular-nums">{readiness}%</span>
      </div>
      <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-muted">
        <div className={`h-full rounded-full ${blockingCount > 0 ? "bg-destructive" : readiness === 100 ? "bg-primary" : "bg-amber-500"}`} style={{ width: `${readiness}%` }} />
      </div>
      <div className="mt-4 divide-y border-y">
        {checks.map((check) => (
          <div key={check.key} className="flex items-start gap-3 py-3">
            {check.state === "pass" ? (
              <CheckCircle2Icon className="mt-0.5 size-4 shrink-0 text-primary" />
            ) : (
              <CircleAlertIcon className={`mt-0.5 size-4 shrink-0 ${check.state === "error" ? "text-destructive" : "text-amber-600"}`} />
            )}
            <div className="min-w-0">
              <div className="text-sm font-medium">{check.title}</div>
            </div>
          </div>
        ))}
      </div>
    </section>
  )
}

function WorkflowReleaseSettings({
  workflow,
  stableVersion,
  adoptionCount,
  currentVersionAdoptionCount,
  knowledgeSupport,
}: {
  workflow: AIWorkflow
  stableVersion?: AIWorkflowVersion
  adoptionCount: number
  currentVersionAdoptionCount: number
  knowledgeSupport: boolean
}) {
  const t = useI18n()
  return (
    <section>
      <div className="flex items-center gap-2 text-sm font-semibold">
        <ShieldCheckIcon className="size-4 text-primary" />{wd(t, "releaseSettings.title")}
      </div>
      <div className="mt-4 divide-y border-y">
        <div className="grid gap-2 py-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
          <div>
            <div className="text-sm font-medium">{wd(t, "releaseSettings.stableVersion")}</div>
            <div className="mt-1 text-xs text-muted-foreground">{wd(t, "releaseSettings.stableHint")}</div>
          </div>
          <Badge variant={stableVersion ? "default" : "destructive"} className="w-fit">
            {stableVersion ? `V${stableVersion.version}` : wd(t, "versionsPanel.pendingPublish")}
          </Badge>
        </div>
        <div className="grid gap-2 py-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
          <div>
            <div className="text-sm font-medium">{wd(t, knowledgeSupport ? "releaseSettings.enabledReception" : "releaseSettings.enabledProducts")}</div>
            <div className="mt-1 text-xs text-muted-foreground">{wd(t, knowledgeSupport ? "releaseSettings.enabledReceptionHint" : "releaseSettings.enabledProductsHint")}</div>
          </div>
          <Badge variant={adoptionCount > 0 ? "default" : "outline"} className="w-fit">
            {adoptionCount > 0 ? wd(t, knowledgeSupport ? "releaseSettings.receptionCount" : "releaseSettings.productCount", { count: adoptionCount }) : wd(t, "releaseSettings.notEnabled")}
          </Badge>
        </div>
        <div className="grid gap-2 py-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
          <div>
            <div className="text-sm font-medium">{wd(t, "releaseSettings.currentCoverage")}</div>
            <div className="mt-1 text-xs text-muted-foreground">{wd(t, knowledgeSupport ? "releaseSettings.receptionCoverageHint" : "releaseSettings.coverageHint")}</div>
          </div>
          <Badge variant={adoptionCount === currentVersionAdoptionCount ? "default" : "outline"} className="w-fit">
            {currentVersionAdoptionCount} / {adoptionCount}
          </Badge>
        </div>
        <div className="grid gap-2 py-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
          <div>
            <div className="text-sm font-medium">{wd(t, "releaseSettings.flowSource")}</div>
            <div className="mt-1 text-xs text-muted-foreground">{wd(t, "releaseSettings.sourceHint")}</div>
          </div>
          <Badge variant={workflow.scope === "platform" ? "outline" : "secondary"} className="w-fit">
            {workflow.scope === "platform" ? wd(t, "scope.platform") : wd(t, "scope.tenant")}
          </Badge>
        </div>
      </div>
    </section>
  )
}

function WorkflowConfigTabs({
  adoptionCount,
  currentVersionAdoptionCount,
  workflow,
  definition,
  stableVersion,
  editable,
  saving,
  name,
  description,
  templates,
  templatesLoading,
  defaultPreviewOpen,
  testRun,
  onNameChange,
  onDescriptionChange,
  onBlueprintApply,
  onDefinitionChange,
  onCredentialChainChange,
  onSave,
}: {
  adoptionCount: number
  currentVersionAdoptionCount: number
  workflow: AIWorkflow
  definition: AIWorkflowDefinition
  stableVersion?: AIWorkflowVersion
  editable: boolean
  saving: boolean
  name: string
  description: string
  templates: AIWorkflow[]
  templatesLoading: boolean
  defaultPreviewOpen: boolean
  testRun?: AIWorkflowTestRunResult | null
  onNameChange: (value: string) => void
  onDescriptionChange: (value: string) => void
  onBlueprintApply: (template: AIWorkflow) => void
  onDefinitionChange: (definition: AIWorkflowDefinition) => void
  onCredentialChainChange: (value: WorkflowModelCredentialSource[]) => void
  onSave: () => void
}) {
  const knowledgeSupport = isKnowledgeSupportWorkflowDefinition(definition)
  return (
    <div className="rhd-railops-workflow-config-space">
      {editable ? (
        <WorkflowBasicSettings
          name={name}
          description={description}
          onNameChange={onNameChange}
          onDescriptionChange={onDescriptionChange}
        />
      ) : null}

      <WorkflowOrchestrationPanel
        definition={definition}
        editable={editable}
        templates={templates}
        templatesLoading={templatesLoading}
        onBlueprintApply={onBlueprintApply}
        onDefinitionChange={onDefinitionChange}
      />

      <div className="rhd-railops-workflow-preview-row">
        <DefinitionPreview
          defaultOpen={defaultPreviewOpen}
          definition={definition}
          nodeSpecs={[]}
          testRun={testRun}
        />
      </div>

      <div className="rhd-railops-workflow-config-grid">
        <WorkflowCapabilitySettings definition={definition} editable={editable} />
        <WorkflowModelCredentialPolicy
          definition={definition}
          editable={editable}
          saving={saving}
          onChange={onCredentialChainChange}
          onSave={onSave}
        />
        <div className="rhd-railops-workflow-release-grid">
          <WorkflowReleaseSettings
            workflow={workflow}
            stableVersion={stableVersion}
            adoptionCount={adoptionCount}
            currentVersionAdoptionCount={currentVersionAdoptionCount}
            knowledgeSupport={knowledgeSupport}
          />
          <WorkflowReadiness definition={definition} stableVersionId={workflow.currentStableVersionId} adoptionCount={adoptionCount} />
        </div>
      </div>
    </div>
  )
}

function WorkflowVersionPanel({
  workflow,
  versions,
  versionsPage,
  loading,
  canRollback,
  rollingBackVersionId,
  onPageChange,
  onLimitChange,
  onRollback,
}: {
  workflow: AIWorkflow
  versions: AIWorkflowVersion[]
  versionsPage: OptionPageState
  loading: boolean
  canRollback: boolean
  rollingBackVersionId: number
  onPageChange: (page: number) => void
  onLimitChange: (limit: number) => void
  onRollback: (version: AIWorkflowVersion) => void
}) {
  const t = useI18n()
  const stable = versions.find((item) => item.id === workflow.currentStableVersionId)
  const previous = stable
    ? versions
        .filter((item) => item.version < stable.version)
        .sort((left, right) => right.version - left.version)[0]
    : undefined
  const [compareVersionId, setCompareVersionId] = useState(previous?.id ?? 0)
  const historicalVersions = stable
    ? versions.filter((item) => item.id !== stable.id && item.version < stable.version)
    : []
  const effectiveCompareVersionId = historicalVersions.some((item) => item.id === compareVersionId)
    ? compareVersionId
    : previous?.id ?? 0
  const compared = versions.find((item) => item.id === effectiveCompareVersionId) ?? previous
  const change = stable ? summarizeVersionChange(stable.definition, compared?.definition) : undefined
  const deltaLabel = (value: number) => value === 0 ? wd(t, "versionsPanel.deltaNone") : value > 0 ? wd(t, "versionsPanel.deltaAdd", { count: value }) : wd(t, "versionsPanel.deltaRemove", { count: Math.abs(value) })
  const hasDetailedChange = Boolean(change && (
    change.addedNodes.length
    || change.removedNodes.length
    || change.changedNodes.length
    || change.addedEdges.length
    || change.removedEdges.length
    || change.addedCapabilities.length
    || change.removedCapabilities.length
    || change.addedVariables.length
    || change.removedVariables.length
    || change.credentialChange
  ))
  const hasVersions = versions.length > 0
  const showVersionSkeleton = loading && !hasVersions

  return (
    <div className="space-y-6">
      <section className="grid border-y lg:grid-cols-[minmax(0,1.2fr)_repeat(3,minmax(10rem,0.4fr))]">
        <div className="px-4 py-5">
          <div className="text-xs text-muted-foreground">{wd(t, "versionsPanel.currentStable")}</div>
          <div className="mt-2 flex items-center gap-3">
            <span className="text-3xl font-semibold">{stable ? `V${stable.version}` : wd(t, "versionsPanel.pendingPublish")}</span>
            {stable ? <Badge>{wd(t, "versionsPanel.stable")}</Badge> : <Badge variant="destructive">{wd(t, "versionsPanel.notOnline")}</Badge>}
          </div>
          {stable?.changeSummary.trim() ? (
            <p className="mt-3 text-sm leading-6 text-muted-foreground">{cleanWorkflowBusinessLabel(t, stable.changeSummary)}</p>
          ) : null}
        </div>
        <div className="border-t px-4 py-5 lg:border-l lg:border-t-0">
          <div className="text-xs text-muted-foreground">{wd(t, "versionsPanel.steps")}</div>
          <div className="mt-2 text-lg font-semibold">{change ? deltaLabel(change.nodeDelta) : "-"}</div>
          <div className="mt-1 text-xs text-muted-foreground">{wd(t, "versionsPanel.relativeTo", { version: compared ? `V${compared.version}` : wd(t, "versionsPanel.initialVersion") })}</div>
        </div>
        <div className="border-t px-4 py-5 lg:border-l lg:border-t-0">
          <div className="text-xs text-muted-foreground">{wd(t, "versionsPanel.addedCapability")}</div>
          <div className="mt-2 text-lg font-semibold">{change?.addedCapabilities.length ?? 0}</div>
          <div className="mt-1 truncate text-xs text-muted-foreground" title={change?.addedCapabilities.join("、")}>
            {change?.addedCapabilities.join("、") || wd(t, "versionsPanel.noAddedCapability")}
          </div>
        </div>
        <div className="border-t px-4 py-5 lg:border-l lg:border-t-0">
          <div className="text-xs text-muted-foreground">{wd(t, "versionsPanel.removedCapability")}</div>
          <div className="mt-2 text-lg font-semibold">{change?.removedCapabilities.length ?? 0}</div>
          <div className="mt-1 truncate text-xs text-muted-foreground" title={change?.removedCapabilities.join("、")}>
            {change?.removedCapabilities.join("、") || wd(t, "versionsPanel.noRemovedCapability")}
          </div>
        </div>
      </section>

      {stable && historicalVersions.length > 0 ? (
        <section className="border-y">
          <div className="flex flex-col gap-3 border-b px-4 py-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <div className="flex items-center gap-2 text-sm font-semibold"><GitBranchIcon className="size-4 text-primary" />{wd(t, "versionsPanel.diff")}</div>
            </div>
            <SelectField
              style={{ marginBottom: 0 }}
              selectProps={{
                "aria-label": wd(t, "versionsPanel.compareAria"),
                value: compared ? String(compared.id) : "",
                onChange: (value) => setCompareVersionId(Number(value)),
                options: [
                  ...(compared && historicalVersions.some((version) => String(version.id) === String(compared.id))
                    ? []
                    : [{ value: compared ? String(compared.id) : "", label: compared ? wd(t, "versionsPanel.compareVersion", { version: compared.version }) : wd(t, "versionsPanel.selectHistory"), disabled: true }]),
                  ...historicalVersions.map((version) => ({ value: String(version.id), label: wd(t, "versionsPanel.compareVersion", { version: version.version }) })),
                ],
                style: { width: 192 },
              }}
            />
          </div>
          {hasDetailedChange && change ? (
            <div className="grid md:grid-cols-3">
              <div className="px-4 py-5">
                <div className="text-xs font-semibold text-muted-foreground">{wd(t, "versionsPanel.stepChanges")}</div>
                <div className="mt-3 space-y-2 text-sm">
                  {change.addedNodes.map((node) => <div key={`add:${node.id}`} className="flex items-center gap-2"><Badge>{wd(t, "versionsPanel.added")}</Badge><span>{node.title}</span></div>)}
                  {change.changedNodes.map((node) => <div key={`change:${node.id}`} className="flex items-center gap-2"><Badge variant="outline">{wd(t, "versionsPanel.changed")}</Badge><span>{node.title}</span></div>)}
                  {change.removedNodes.map((node) => <div key={`remove:${node.id}`} className="flex items-center gap-2"><Badge variant="destructive">{wd(t, "versionsPanel.removed")}</Badge><span>{node.title}</span></div>)}
                  {change.addedEdges.map((edge) => <div key={`edge-add:${edge.key}`} className="flex items-center gap-2"><Badge>{wd(t, "versionsPanel.addedPath")}</Badge><span>{edge.from} <ArrowRightIcon className="inline size-3.5" /> {edge.to}</span></div>)}
                  {change.removedEdges.map((edge) => <div key={`edge-remove:${edge.key}`} className="flex items-center gap-2"><Badge variant="destructive">{wd(t, "versionsPanel.removedPath")}</Badge><span>{edge.from} <ArrowRightIcon className="inline size-3.5" /> {edge.to}</span></div>)}
                  {!change.addedNodes.length && !change.changedNodes.length && !change.removedNodes.length && !change.addedEdges.length && !change.removedEdges.length ? <div className="text-muted-foreground">{wd(t, "versionsPanel.noPathChange")}</div> : null}
                </div>
              </div>
              <div className="border-t px-4 py-5 md:border-l md:border-t-0">
                <div className="text-xs font-semibold text-muted-foreground">{wd(t, "versionsPanel.serviceCapability")}</div>
                <div className="mt-3 space-y-2 text-sm">
                  {change.addedCapabilities.map((item) => <div key={`cap-add:${item}`} className="flex items-center gap-2"><CheckCircle2Icon className="size-4 text-primary" />{wd(t, "versionsPanel.added")} {item}</div>)}
                  {change.removedCapabilities.map((item) => <div key={`cap-remove:${item}`} className="flex items-center gap-2"><CircleAlertIcon className="size-4 text-destructive" />{wd(t, "versionsPanel.removed")} {item}</div>)}
                  {!change.addedCapabilities.length && !change.removedCapabilities.length ? <div className="text-muted-foreground">{wd(t, "versionsPanel.noCapabilityChange")}</div> : null}
                </div>
              </div>
              <div className="border-t px-4 py-5 md:border-l md:border-t-0">
                <div className="text-xs font-semibold text-muted-foreground">{wd(t, "versionsPanel.runtimeConfig")}</div>
                <div className="mt-3 space-y-2 text-sm">
                  {change.credentialChange ? (
                    <div><div className="text-muted-foreground">{wd(t, "versionsPanel.credential")}</div><div className="mt-1 leading-5">{change.credentialChange.from} <ArrowRightIcon className="inline size-3.5" /> {change.credentialChange.to}</div></div>
                  ) : null}
                  {change.addedVariables.map((item) => <div key={`var-add:${item}`}><Badge variant="outline">{wd(t, "versionsPanel.addedConfig")}</Badge> <span className="break-all text-xs">{item}</span></div>)}
                  {change.removedVariables.map((item) => <div key={`var-remove:${item}`}><Badge variant="destructive">{wd(t, "versionsPanel.removedConfig")}</Badge> <span className="break-all text-xs">{item}</span></div>)}
                  {!change.credentialChange && !change.addedVariables.length && !change.removedVariables.length ? <div className="text-muted-foreground">{wd(t, "versionsPanel.noRuntimeChange")}</div> : null}
                </div>
              </div>
            </div>
          ) : (
            <div className="px-4 py-8 text-center text-sm text-muted-foreground">{wd(t, "versionsPanel.sameBehavior")}</div>
          )}
        </section>
      ) : null}

      <div className="divide-y border-y md:hidden" data-workflow-version-mobile-list aria-busy={loading || undefined}>
        {showVersionSkeleton ? <ModuleLoading variant="list" count={4} /> : null}
        {versions.map((version) => (
          <article key={version.id} className="space-y-3 px-4 py-4">
            <div className="flex items-start justify-between gap-3">
              <div>
                <div className="text-base font-semibold">V{version.version}</div>
                <div className="mt-1 text-xs text-muted-foreground">
                  {formatDateTime(version.publishedAt || version.createdAt)} · {version.publishedByName || "system"}
                </div>
              </div>
              {version.id === workflow.currentStableVersionId ? <Badge>{wd(t, "versionsPanel.currentStableBadge")}</Badge> : <Badge variant="outline">{wd(t, "versionsPanel.historyVersion")}</Badge>}
            </div>
            {version.changeSummary.trim() ? <p className="text-sm leading-6 text-muted-foreground">{version.changeSummary}</p> : null}
            <div className="flex items-center justify-between gap-3 border-t pt-3 text-xs text-muted-foreground">
              <span>{wd(t, "versionsPanel.versionHash")}</span>
              <span className="font-mono" title={version.definitionHash}>{shortHash(version.definitionHash)}</span>
            </div>
            {version.id !== stable?.id ? (
              <div className="grid grid-cols-2 gap-2">
                <Button size="sm" variant="outline" onClick={() => setCompareVersionId(version.id)}>{wd(t, "versionsPanel.compare")}</Button>
                {canRollback ? (
                  <Button size="sm" variant="outline" disabled={rollingBackVersionId > 0} onClick={() => onRollback(version)}>
                    {rollingBackVersionId === version.id ? <Loader2Icon className="animate-spin" /> : <RotateCcwIcon />}{wd(t, "versionsPanel.rollback")}
                  </Button>
                ) : <span />}
              </div>
            ) : null}
          </article>
        ))}
        {!loading && !hasVersions ? <div className="px-4 py-12 text-center text-sm text-muted-foreground">{wd(t, "versionsPanel.empty")}</div> : null}
      </div>

      <div className="hidden divide-y border md:block" data-workflow-version-desktop-table aria-busy={loading || undefined}>
        {versions.map((version) => (
          <article key={version.id} className="grid gap-4 px-4 py-4 lg:grid-cols-[7rem_minmax(0,1fr)_9rem_10rem_11rem_9rem_14rem] lg:items-center">
            <div className="font-semibold">V{version.version}</div>
            <div className="text-sm leading-5 text-muted-foreground">{cleanWorkflowBusinessLabel(t, version.changeSummary.trim() || "-")}</div>
            <div>{version.id === workflow.currentStableVersionId ? <Badge>{wd(t, "versionsPanel.currentStableBadge")}</Badge> : <Badge variant="outline">{wd(t, "versionsPanel.historyVersion")}</Badge>}</div>
            <div className="text-sm">{version.publishedByName || "system"}</div>
            <div className="text-sm text-muted-foreground">{formatDateTime(version.publishedAt || version.createdAt)}</div>
            <div className="break-all text-xs text-muted-foreground" title={version.definitionHash}>{shortHash(version.definitionHash)}</div>
            <div className="flex justify-end gap-2">
              {version.id !== stable?.id ? (
                <Button size="sm" variant="outline" onClick={() => setCompareVersionId(version.id)}>{wd(t, "versionsPanel.compare")}</Button>
              ) : null}
              {version.id !== stable?.id && canRollback ? (
                <Button size="sm" variant="outline" disabled={rollingBackVersionId > 0} onClick={() => onRollback(version)}>
                  {rollingBackVersionId === version.id ? <Loader2Icon className="animate-spin" /> : <RotateCcwIcon />}{wd(t, "versionsPanel.rollbackAsNew")}
                </Button>
              ) : null}
            </div>
          </article>
        ))}
        {showVersionSkeleton ? (
          <div className="px-4 py-8"><ModuleLoading variant="list" count={3} /></div>
        ) : !hasVersions ? (
          <div className="px-4 py-12 text-center text-sm text-muted-foreground">{wd(t, "versionsPanel.empty")}</div>
        ) : null}
      </div>
      {versionsPage.total > versionsPage.limit || versionsPage.page > 1 ? (
        <ListPagination
          page={versionsPage.page}
          total={versionsPage.total}
          limit={versionsPage.limit}
          loading={loading}
          pageSizeOptions={[12, 24, 48]}
          onPageChange={onPageChange}
          onLimitChange={onLimitChange}
        />
      ) : null}
    </div>
  )
}

function WorkflowAdoptionPanel({
  agents,
  agentsPage,
  versions,
  stableVersion,
  initialLoading,
  loading,
  canBind,
  upgradingAgentId,
  onPageChange,
  onLimitChange,
  onPrepareUpgrade,
  knowledgeSupport,
}: {
  agents: AIAgent[]
  agentsPage: OptionPageState
  versions: AIWorkflowVersion[]
  stableVersion?: AIWorkflowVersion
  initialLoading: boolean
  loading: boolean
  canBind: boolean
  upgradingAgentId: number
  onPageChange: (page: number) => void
  onLimitChange: (limit: number) => void
  onPrepareUpgrade: (agent: AIAgent) => void
  knowledgeSupport: boolean
}) {
  const t = useI18n()
  const versionById = new Map(versions.map((version) => [version.id, version]))

  return (
    <div className="space-y-4">
      <div className="flex items-start gap-3 border-y py-4">
        <span className="flex size-9 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
          <ShieldCheckIcon className="size-4" />
        </span>
        <div>
          <div className="text-sm font-semibold">{wd(t, "adoptionPanel.upgradeTitle")}</div>
        </div>
      </div>
      {initialLoading ? (
        <ModuleLoading variant="list" count={4} />
      ) : (
        <div className="grid gap-3 lg:grid-cols-2">
          {agents.map((agent) => {
            const version = versionById.get(agent.workflowVersionId)
            const updateAvailable = Boolean(stableVersion && agent.workflowVersionId !== stableVersion.id)
            const pendingDeployment = agent.reviewStatus === "unreviewed" && agent.activeReleaseId > 0
            const awaitingFirstDeployment = agent.activeReleaseId <= 0
            const deploymentLabel = awaitingFirstDeployment
              ? agent.reviewStatus === "pending" ? wd(t, "adoptionPanel.reviewing") : wd(t, "adoptionPanel.pendingReviewDeploy")
              : pendingDeployment ? wd(t, "adoptionPanel.pendingDeploy") : updateAvailable ? wd(t, "adoptionPanel.newVersion") : wd(t, "adoptionPanel.productionRunning")
            return (
              <article key={agent.id} className="rounded-lg border bg-background p-4 shadow-[0_8px_24px_rgba(15,23,42,0.05)]">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="truncate text-base font-semibold">{cleanWorkflowBusinessLabel(t, knowledgeSupport ? agent.name : agent.productName || wd(t, "adoptionPanel.unboundProduct"))}</div>
                    <div className="mt-1 flex items-center gap-1.5 truncate text-sm text-muted-foreground"><ShieldCheckIcon className="size-3.5 shrink-0" />{cleanWorkflowBusinessLabel(t, agent.name)}</div>
                  </div>
                  <Badge variant={awaitingFirstDeployment || pendingDeployment || updateAvailable ? "outline" : "default"}>
                    {deploymentLabel}
                  </Badge>
                </div>
                <div className="mt-4 flex items-center justify-between border-t pt-3">
                  <div>
                    <div className="text-xs text-muted-foreground">{wd(t, "adoptionPanel.configVersion")}</div>
                    <div className="mt-1 text-sm font-semibold">{version ? `V${version.version}` : wd(t, "adoptionPanel.pendingSelect")}</div>
                    {updateAvailable && stableVersion ? <div className="mt-1 text-xs text-amber-700">{wd(t, "adoptionPanel.canUpdateTo", { version: stableVersion.version })}</div> : null}
                  </div>
                  {updateAvailable && stableVersion && canBind ? (
                    <Button size="sm" variant="outline" disabled={upgradingAgentId === agent.id} onClick={() => onPrepareUpgrade(agent)}>
                      {upgradingAgentId === agent.id ? <Loader2Icon className="animate-spin" /> : <HistoryIcon />}
                      {wd(t, "adoptionPanel.prepareUpgrade")}
                    </Button>
                  ) : (
                    <Button size="sm" variant="outline" render={<Link href={buildEnterpriseAIPath(agent.id)} />}>
                      {wd(t, "adoptionPanel.configureAgent")}<ArrowUpRightIcon />
                    </Button>
                  )}
                </div>
                {pendingDeployment ? <p className="mt-3 text-xs leading-5 text-muted-foreground">{wd(t, "adoptionPanel.pendingDeployHint")}</p> : null}
                {awaitingFirstDeployment ? <p className="mt-3 text-xs leading-5 text-muted-foreground">{wd(t, "adoptionPanel.pendingFirstDeployHint")}</p> : null}
              </article>
            )
          })}
        </div>
      )}
      {agents.length === 0 && !loading ? (
        <div className="border-y py-12 text-center">
          <ShieldCheckIcon className="mx-auto size-6 text-muted-foreground" />
          <div className="mt-3 text-sm font-medium">{wd(t, knowledgeSupport ? "adoptionPanel.emptyReception" : "adoptionPanel.empty")}</div>
        </div>
      ) : null}
      {agentsPage.total > agentsPage.limit || agentsPage.page > 1 ? (
        <ListPagination
          page={agentsPage.page}
          total={agentsPage.total}
          limit={agentsPage.limit}
          loading={loading}
          pageSizeOptions={[12, 24, 48]}
          onPageChange={onPageChange}
          onLimitChange={onLimitChange}
        />
      ) : null}
    </div>
  )
}

function WorkflowTestPanel({
  open,
  onOpenChange,
  workflow,
  definition,
  agents,
  agentsPage,
  agentsLoading,
  agentId,
  onAgentChange,
  onAgentPageChange,
  message,
  onMessageChange,
  autoConfirm,
  onAutoConfirmChange,
  branchOverrides,
  onBranchOverridesChange,
  runtimeContext,
  onRuntimeContextChange,
  running,
  batchRunning,
  batchResults,
  result,
  onRun,
  onRunBatch,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  workflow: AIWorkflow
  definition: AIWorkflowDefinition
  agents: AIAgent[]
  agentsPage: OptionPageState
  agentsLoading: boolean
  agentId: number
  onAgentChange: (agentId: number) => void
  onAgentPageChange: (page: number) => void
  message: string
  onMessageChange: (message: string) => void
  autoConfirm: boolean
  onAutoConfirmChange: (checked: boolean) => void
  branchOverrides: Record<string, string>
  onBranchOverridesChange: (overrides: Record<string, string>) => void
  runtimeContext: WorkflowTestRuntimeContext
  onRuntimeContextChange: (context: WorkflowTestRuntimeContext) => void
  running: boolean
  batchRunning: boolean
  batchResults: WorkflowBatchTestCaseResult[]
  result: AIWorkflowTestRunResult | null
  onRun: () => void
  onRunBatch: (scenarios: WorkflowTestScenario[]) => void
}) {
  const t = useI18n()
  const nodeByID = new Map(definition.nodes.map((node) => [node.id, node]))
  const resultByID = new Map((result?.nodes ?? []).map((node) => [node.nodeId, node]))
  const orderedNodes = result
    ? [
        ...result.nodes.map((nodeResult, index) => {
          const node = nodeByID.get(nodeResult.nodeId)
          return {
            key: `${nodeResult.nodeId}:executed:${index}`,
            nodeId: nodeResult.nodeId,
            nodeType: node?.type || nodeResult.nodeType,
            name: node?.name || nodeResult.nodeId,
            nodeResult,
          }
        }),
        ...definition.nodes.filter((node) => !resultByID.has(node.id)).map((node) => ({
          key: `${node.id}:not-visited`,
          nodeId: node.id,
          nodeType: node.type,
          name: node.name || node.id,
          nodeResult: undefined,
        })),
      ]
    : []
  const statusMeta = result ? workflowTestStatusMeta(t, result.status) : null
  const StatusIcon = statusMeta?.Icon
  const stoppedNodeID = result?.failedNodeId || result?.interruptNodeId || ""
  const stoppedNode = stoppedNodeID ? nodeByID.get(stoppedNodeID) : undefined
  const stoppedNodeLabel = stoppedNode?.name || stoppedNode?.id || stoppedNodeID
  const resultError = describeWorkflowRunError(result?.errorMessage || "")
  const conditionNodes = definition.nodes
    .filter((node) => node.type === "condition")
    .map((node) => ({ node, branches: getConditionBranches(node).filter((branch) => Boolean(branch.id)) }))
    .filter((item) => item.branches.length > 0)
  const hasHandoffNode = definition.nodes.some((node) => node.type === "handoff_to_human")
  const hasConfirmationNode = definition.nodes.some((node) => node.type === "human_confirm")
  const overrideCount = Object.keys(branchOverrides).length
  const testScenarios = useMemo(() => getWorkflowTestScenarios(definition), [definition])
  const [excludedBatchKeys, setExcludedBatchKeys] = useState<string[]>([])
  const selectedBatchKeys = testScenarios.filter((scenario) => !excludedBatchKeys.includes(scenario.key)).map((scenario) => scenario.key)
  const batchResultByKey = new Map(batchResults.map((item) => [item.scenarioKey, item]))
  const passedBatchCount = batchResults.filter((item) => item.status === "passed").length
  const failedBatchCount = batchResults.filter((item) => item.status === "failed").length
  const selectedBatchScenarios = testScenarios.filter((scenario) => selectedBatchKeys.includes(scenario.key))
  const selectedScenario = testScenarios.find((scenario) => {
    const expected = Object.entries(scenario.branchOverrides)
    const actual = Object.entries(branchOverrides)
    return expected.length === actual.length
      && expected.every(([nodeId, branchId]) => branchOverrides[nodeId] === branchId)
      && JSON.stringify(scenario.runtimeContext) === JSON.stringify(runtimeContext)
  })
  const selectedAgent = agents.find((agent) => agent.id === agentId)
  const selectedAgentLabel = selectedAgent
    ? `${selectedAgent.productName ? `${selectedAgent.productName} · ` : ""}${selectedAgent.name}`
    : wd(t, "testPanel.testAgent")

  return (
    <DetailDrawer
      open={open}
      onClose={() => onOpenChange(false)}
      title={(
        <span className="flex min-w-0 items-center gap-2">
          <PlayIcon className="size-4 shrink-0 text-primary" />
          <span className="truncate">{wd(t, "testPanel.title", { name: workflow.name })}</span>
        </span>
      )}
      width={672}
      styles={{ body: { padding: 0 } }}
    >
        <ScrollArea className="h-full">
          <div className="space-y-5 p-5">
            <section className="space-y-4 border-b pb-5">
              <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div>
                  <div className="flex items-center gap-2 text-sm font-semibold"><ListChecksIcon className="size-4 text-primary" />{wd(t, "testPanel.batchTitle")}</div>
                </div>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={running || batchRunning || agentId <= 0 || selectedBatchScenarios.length === 0}
                  onClick={() => onRunBatch(selectedBatchScenarios)}
                >
                  {batchRunning ? <Loader2Icon className="animate-spin" /> : <PlayIcon />}
                  {batchRunning ? wd(t, "testPanel.batchRunning") : wd(t, "testPanel.runScenarios", { count: selectedBatchScenarios.length })}
                </Button>
              </div>
              {batchResults.length > 0 ? (
                <div className="flex flex-wrap gap-2 text-xs">
                  <Badge variant="outline">{wd(t, "testPanel.totalItems", { count: batchResults.length })}</Badge>
                  <Badge>{wd(t, "testPanel.passedItems", { count: passedBatchCount })}</Badge>
                  {failedBatchCount > 0 ? <Badge variant="destructive">{wd(t, "testPanel.failedItems", { count: failedBatchCount })}</Badge> : null}
                </div>
              ) : null}
              <div className="divide-y border-y">
                {testScenarios.map((scenario) => {
                  const batchItem = batchResultByKey.get(scenario.key)
                  const failedNodeId = batchItem?.result?.failedNodeId || batchItem?.result?.interruptNodeId || ""
                  const failedNode = failedNodeId ? nodeByID.get(failedNodeId) : undefined
                  const error = describeWorkflowRunError(batchItem?.result?.errorMessage || batchItem?.errorMessage || "")
                  return (
                    <label
                      key={scenario.key}
                      data-workflow-batch-scenario={scenario.key}
                      data-workflow-batch-status={batchItem?.status ?? "not_run"}
                      className="flex cursor-pointer items-start gap-3 py-3"
                    >
                      <AntCheckbox
                        checked={selectedBatchKeys.includes(scenario.key)}
                        disabled={batchRunning}
                        onChange={(event) => setExcludedBatchKeys((keys) => (
                          event.target.checked ? keys.filter((key) => key !== scenario.key) : [...new Set([...keys, scenario.key])]
                        ))}
                      />
                      <span className="min-w-0 flex-1">
                        <span className="flex flex-wrap items-center justify-between gap-2">
                          <span className="text-sm font-medium">{scenario.title}</span>
                          {batchItem?.status === "running" ? <Badge variant="outline"><Loader2Icon className="animate-spin" />{wd(t, "testPanel.running")}</Badge> : null}
                          {batchItem?.status === "passed" ? <Badge><CheckCircle2Icon />{wd(t, "testPanel.passed")}</Badge> : null}
                          {batchItem?.status === "failed" ? <Badge variant="destructive"><CircleAlertIcon />{wd(t, "testPanel.failed")}</Badge> : null}
                        </span>
                        {batchItem?.result ? <span className="mt-1 block text-xs text-muted-foreground">{wd(t, "testPanel.durationNodes", { duration: batchItem.result.durationMs, nodes: batchItem.result.nodes.length })}</span> : null}
                        {failedNodeId ? <span className="mt-1 block text-xs text-destructive">{wd(t, "testPanel.stoppedAt", { node: failedNode?.name || failedNodeId })}</span> : null}
                        {error.summary ? <span className="mt-1 block text-xs leading-5 text-destructive">{error.summary}</span> : null}
                      </span>
                    </label>
                  )
                })}
              </div>
            </section>
            <section className="space-y-4 border-b pb-5">
              {hasHandoffNode ? (
                <div className="flex items-start gap-3 border-y py-3 text-sm">
                  <UsersIcon className="mt-0.5 size-4 shrink-0 text-primary" />
                  <div>
                    <div className="font-medium">{wd(t, "testPanel.hasHandoff")}</div>
                  </div>
                </div>
              ) : (
                <div className="flex items-start justify-between gap-4 border-y py-3 text-sm">
                  <div className="flex min-w-0 items-start gap-3">
                    <CircleAlertIcon className="mt-0.5 size-4 shrink-0 text-amber-700" />
                    <div>
                      <div className="font-medium">{wd(t, "testPanel.noHandoff")}</div>
                    </div>
                  </div>
                  <Button size="sm" variant="outline" render={<Link href="/enterprise/workflow" />}>
                    {wd(t, "testPanel.chooseOther")}
                  </Button>
                </div>
              )}
              <div className="space-y-2">
                <label className="flex items-center gap-1.5 text-sm font-medium"><ShieldCheckIcon className="size-4 text-primary" />{wd(t, "testPanel.testAgent")}</label>
                <SelectField
                  style={{ marginBottom: 0 }}
                  selectProps={{
                    value: agentId > 0 ? String(agentId) : "",
                    onChange: (value) => onAgentChange(Number(value)),
                    options: [
                      ...(agentId > 0 && agents.some((agent) => agent.id === agentId)
                        ? []
                        : [{
                            value: agentId > 0 ? String(agentId) : "",
                            label: (
                              <span className="flex items-center gap-2">
                                <ShieldCheckIcon className="size-4 shrink-0 text-muted-foreground" />
                                {selectedAgentLabel}
                              </span>
                            ),
                            disabled: true,
                          }]),
                      ...agents.map((agent) => ({
                        value: String(agent.id),
                        label: (
                          <span className="flex items-center gap-2">
                            <ShieldCheckIcon className="size-4 shrink-0 text-muted-foreground" />
                            {agent.productName ? `${agent.productName} · ` : ""}
                            {agent.name}
                          </span>
                        ),
                      })),
                    ],
                    style: { width: "100%" },
                  }}
                />
                {agents.length === 0 ? <p className="text-xs text-amber-700">{wd(t, "testPanel.noAgent")}</p> : null}
                {agentsPage.total > agentsPage.limit || agentsPage.page > 1 ? (
                  <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
                    <span className="tabular-nums">{wd(t, "testPanel.pageStatus", { page: agentsPage.page, total: optionPageCount(agentsPage) })}</span>
                    <div className="flex items-center gap-1.5">
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={agentsLoading || agentsPage.page <= 1}
                        onClick={() => onAgentPageChange(agentsPage.page - 1)}
                      >
                        {wd(t, "testPanel.prevPage")}
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={agentsLoading || agentsPage.page >= optionPageCount(agentsPage)}
                        onClick={() => onAgentPageChange(agentsPage.page + 1)}
                      >
                        {agentsLoading ? <Loader2Icon className="animate-spin" /> : null}
                        {wd(t, "testPanel.nextPage")}
                      </Button>
                    </div>
                  </div>
                ) : null}
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">{wd(t, "testPanel.scenario")}</label>
                <SelectField
                  style={{ marginBottom: 0 }}
                  selectProps={{
                    "aria-label": wd(t, "testPanel.scenario"),
                    value: selectedScenario?.key || "__custom__",
                    onChange: (value) => {
                      const scenario = testScenarios.find((item) => item.key === value)
                      if (!scenario) return
                      onBranchOverridesChange(scenario.branchOverrides)
                      onRuntimeContextChange(scenario.runtimeContext)
                      onAutoConfirmChange(scenario.autoConfirm)
                      if (scenario.message) onMessageChange(scenario.message)
                    },
                    options: [
                      ...(selectedScenario
                        ? []
                        : [{ value: "__custom__", label: wd(t, "testPanel.customScenario"), disabled: true }]),
                      ...testScenarios.map((scenario) => ({ value: scenario.key, label: scenario.title })),
                    ],
                    style: { width: "100%" },
                  }}
                />
                {runtimeContext.deviceBound || runtimeContext.deviceId ? (
                  <div className="flex items-center gap-2 border-y py-2 text-xs text-muted-foreground">
                    <ScanLineIcon className="size-3.5 text-primary" />
                    {wd(t, "testPanel.deviceBound")}
                  </div>
                ) : null}
              </div>
              <div className="space-y-2">
                <label htmlFor="workflow-test-message" className="text-sm font-medium">{wd(t, "testPanel.customerInput")}</label>
                <Textarea
                  id="workflow-test-message"
                  value={message}
                  onChange={(event) => onMessageChange(event.target.value)}
                  rows={4}
                  placeholder={wd(t, "testPanel.messagePlaceholder")}
                />
              </div>
              {conditionNodes.length > 0 ? (
                <details className="border-y py-3">
                  <summary className="flex cursor-pointer list-none items-center justify-between gap-3 text-sm font-medium">
                    <span className="flex items-center gap-2"><GitBranchIcon className="size-4 text-primary" />{wd(t, "testPanel.advancedBranch")}</span>
                    <Badge variant={overrideCount > 0 ? "default" : "outline"}>{overrideCount > 0 ? wd(t, "testPanel.overrideCount", { count: overrideCount }) : wd(t, "testPanel.actualCondition")}</Badge>
                  </summary>
                  <div className="mt-4 space-y-4">
                    {conditionNodes.map(({ node, branches }) => {
                      const selectedBranch = branches.find((branch) => branch.id === branchOverrides[node.id])
                      return (
                        <div key={node.id} className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(14rem,1.2fr)] sm:items-center">
                          <div>
                            <div className="text-sm font-medium">{node.name || node.id}</div>
                            <div className="mt-0.5 text-rhd-xs text-muted-foreground">{wd(t, "testPanel.onlyThisRun")}</div>
                          </div>
                          <SelectField
                            style={{ marginBottom: 0 }}
                            selectProps={{
                              "aria-label": wd(t, "testPanel.branchOverrideAria", { node: node.name || node.id }),
                              value: branchOverrides[node.id] || "__actual__",
                              onChange: (value) => {
                                if (!value) return
                                const next = { ...branchOverrides }
                                if (value === "__actual__") delete next[node.id]
                                else next[node.id] = value
                                onBranchOverridesChange(next)
                              },
                              options: [
                                ...(branches.some((branch) => (branch.id || "") === (branchOverrides[node.id] || "__actual__"))
                                  ? []
                                  : [{ value: branchOverrides[node.id] || "__actual__", label: wd(t, "testPanel.actualConditionDecision"), disabled: true }]),
                                { value: "__actual__", label: wd(t, "testPanel.actualConditionDecision") },
                                ...branches.map((branch) => ({ value: branch.id || "", label: `${branch.name || branch.id}${branch.default ? wd(t, "testPanel.defaultBranch") : ""}` })),
                              ],
                              style: { width: "100%" },
                            }}
                          />
                        </div>
                      )
                    })}
                  </div>
                </details>
              ) : null}
              {hasConfirmationNode ? (
                <label className="flex cursor-pointer items-start gap-3 border-y py-3">
                  <AntCheckbox checked={autoConfirm} onChange={(event) => onAutoConfirmChange(event.target.checked)} />
                  <span className="block text-sm font-medium">{wd(t, "testPanel.autoConfirm")}</span>
                </label>
              ) : null}
              <div className="flex items-center justify-between gap-3">
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <ShieldCheckIcon className="size-3.5 text-primary" />{wd(t, "testPanel.isolation")}
                </div>
                <Button onClick={onRun} disabled={running || batchRunning || agentId <= 0 || !message.trim()}>
                  {running ? <Loader2Icon className="animate-spin" /> : <PlayIcon />}
                  {running ? wd(t, "testPanel.running") : wd(t, "testPanel.start")}
                </Button>
              </div>
            </section>

            {running ? (
              <div className="border-y py-10 text-center">
                <Loader2Icon className="mx-auto size-6 animate-spin text-primary" />
                <div className="mt-3 text-sm font-medium">{wd(t, "testPanel.runningDraft")}</div>
              </div>
            ) : null}

            {result && statusMeta && StatusIcon ? (
              <section className="space-y-4">
                <div className="grid grid-cols-3 border-y">
                  <div className="min-w-0 px-3 py-4">
                    <div className="text-xs text-muted-foreground">{wd(t, "testPanel.result")}</div>
                    <div className={`mt-2 flex items-center gap-1.5 whitespace-nowrap text-sm font-semibold ${statusMeta.textClass}`}>
                      <StatusIcon className="size-4" />{statusMeta.label}
                    </div>
                    {stoppedNodeLabel ? <div className={`mt-2 text-xs ${result.status === "interrupted" ? "text-amber-700" : "text-destructive"}`}>{wd(t, "testPanel.stoppedAt", { node: stoppedNodeLabel })}</div> : null}
                    {resultError.summary ? <p className="mt-2 text-xs leading-5 text-destructive">{resultError.summary}</p> : null}
                    {resultError.technicalDetails ? (
                      <details className="mt-2 text-xs text-muted-foreground">
                        <summary className="cursor-pointer">{wd(t, "testPanel.technicalDetails")}</summary>
                        <p className="mt-1 break-all font-mono text-rhd-2xs leading-4">{resultError.technicalDetails}</p>
                      </details>
                    ) : null}
                  </div>
                  <div className="min-w-0 border-l px-3 py-4">
                    <div className="text-xs text-muted-foreground">{wd(t, "testPanel.duration")}</div>
                    <div className="mt-2 whitespace-nowrap font-mono text-base font-semibold">{result.durationMs} ms</div>
                  </div>
                  <div className="min-w-0 border-l px-3 py-4">
                    <div className="text-xs text-muted-foreground">{wd(t, "testPanel.visitedNodes")}</div>
                    <div className="mt-2 whitespace-nowrap text-base font-semibold">{result.nodes.length} / {definition.nodes.length}</div>
                  </div>
                </div>

                <div
                  data-workflow-credential-scope={result.modelCredentialScope}
                  data-workflow-credential-fallback={String(result.modelCredentialFallback)}
                  className="flex flex-wrap items-center gap-x-4 gap-y-2 border-y px-3 py-3 text-xs"
                >
                  <span className="flex items-center gap-1.5 font-medium"><KeyRoundIcon className="size-3.5 text-primary" />{wd(t, "testPanel.modelAccount")}</span>
                  <span>
                    {result.modelCredentialScope === "product"
                      ? wd(t, "testPanel.productKey")
                      : result.modelCredentialScope === "tenant_default"
                        ? wd(t, "testPanel.tenantKey")
                        : wd(t, "testPanel.customModel")}
                  </span>
                  <span className="font-mono text-muted-foreground">
                    {result.modelApiKeyId ? `Key ID ${result.modelApiKeyId}` : wd(t, "testPanel.noUsageKey")}
                  </span>
                  {result.modelCredentialFallback ? <Badge variant="outline">{wd(t, "testPanel.fallbackTenantKey")}</Badge> : null}
                </div>

                <div>
                  <div className="mb-3 text-sm font-semibold">{wd(t, "testPanel.executionChain")}</div>
                  <ol className="border-y">
                    {orderedNodes.map((node, index) => {
                      const nodeResult = node.nodeResult
                      const meta = workflowTestNodeStatusMeta(t, nodeResult?.status)
                      const NodeStatusIcon = meta.Icon
                      const nodeError = describeWorkflowRunError(nodeResult?.errorMessage || "")
                      return (
                        <li
                          key={node.key}
                          data-workflow-node-type={node.nodeType}
                          data-workflow-node-status={nodeResult?.status ?? "not_visited"}
                          className="grid grid-cols-[1.75rem_minmax(0,1fr)_auto] gap-3 border-b py-3 last:border-b-0"
                        >
                          <div className="relative flex justify-center">
                            {index < orderedNodes.length - 1 ? <span className="absolute bottom-[-0.75rem] top-4 w-px bg-border" /> : null}
                            <span className={`relative z-10 flex size-5 items-center justify-center rounded-full border bg-background ${meta.textClass}`}>
                              <NodeStatusIcon className="size-3" />
                            </span>
                          </div>
                          <div className="min-w-0">
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="text-sm font-medium">{cleanWorkflowBusinessLabel(t, node.name)}</span>
                              <span className="text-rhd-2xs text-muted-foreground">{workflowBusinessNodeType(t, node.nodeType)}</span>
                            </div>
                            <div className={`mt-1 text-xs ${meta.textClass}`}>{meta.label}</div>
                            {nodeError.summary ? <p className={`mt-1 text-xs leading-5 ${nodeResult?.status === "recovered" ? "text-amber-700" : "text-destructive"}`}>{nodeError.summary}</p> : null}
                            {nodeError.technicalDetails ? (
                              <details className="mt-1 text-xs text-muted-foreground">
                                <summary className="cursor-pointer">{wd(t, "testPanel.errorDetails")}</summary>
                                <p className="mt-1 break-all font-mono text-rhd-2xs leading-4">{nodeError.technicalDetails}</p>
                              </details>
                            ) : null}
                            {nodeResult ? (
                              <details className="mt-2 text-xs">
                                <summary className="cursor-pointer text-muted-foreground">{wd(t, "testPanel.inputOutput")}</summary>
                                <div className="mt-2 grid gap-2">
                                  <pre className="max-h-36 overflow-auto whitespace-pre-wrap break-all border bg-muted/30 p-2 font-mono text-rhd-2xs leading-4">{prettyWorkflowPreview(nodeResult.inputPreview)}</pre>
                                  <pre className="max-h-36 overflow-auto whitespace-pre-wrap break-all border bg-muted/30 p-2 font-mono text-rhd-2xs leading-4">{prettyWorkflowPreview(nodeResult.outputPreview)}</pre>
                                </div>
                              </details>
                            ) : null}
                          </div>
                          <div className="flex items-center gap-1 font-mono text-rhd-2xs text-muted-foreground">
                            {nodeResult ? <Clock3Icon className="size-3" /> : null}
                            {nodeResult ? `${nodeResult.durationMs} ms` : "-"}
                          </div>
                        </li>
                      )
                    })}
                  </ol>
                </div>

                {result.replyText ? (
                  <div className="border-y py-4">
                    <div className="text-xs text-muted-foreground">{wd(t, "testPanel.finalReply")}</div>
                    <p className="mt-2 whitespace-pre-wrap text-sm leading-6">{result.replyText}</p>
                  </div>
                ) : null}
              </section>
            ) : null}

            {!running && !result ? (
              <div className="border-y py-10 text-center text-sm text-muted-foreground">{wd(t, "testPanel.emptyResult")}</div>
            ) : null}
          </div>
        </ScrollArea>
    </DetailDrawer>
  )
}

function workflowTestStatusMeta(t: WorkflowDetailT, status: string) {
  if (status === "completed") return { label: wd(t, "testPanel.statusCompleted"), Icon: CheckCircle2Icon, textClass: "text-primary" }
  if (status === "interrupted") return { label: wd(t, "testPanel.statusInterrupted"), Icon: CircleStopIcon, textClass: "text-amber-700" }
  return { label: wd(t, "testPanel.statusFailed"), Icon: CircleAlertIcon, textClass: "text-destructive" }
}

function workflowTestNodeStatusMeta(t: WorkflowDetailT, status?: string) {
  if (status === "completed") return { label: wd(t, "testPanel.passed"), Icon: CheckCircle2Icon, textClass: "text-primary" }
  if (status === "recovered") return { label: wd(t, "testPanel.nodeRecovered"), Icon: RouteIcon, textClass: "text-amber-700" }
  if (status === "failed") return { label: wd(t, "testPanel.failed"), Icon: CircleAlertIcon, textClass: "text-destructive" }
  if (status === "interrupted") return { label: wd(t, "testPanel.statusInterrupted"), Icon: CircleStopIcon, textClass: "text-amber-700" }
  return { label: wd(t, "testPanel.nodeNotVisited"), Icon: ChevronRightIcon, textClass: "text-muted-foreground" }
}

function prettyWorkflowPreview(value: string) {
  if (!value) return "-"
  try {
    return JSON.stringify(JSON.parse(value), null, 2)
  } catch {
    return value
  }
}

function runtimeVariableFallback(): AIWorkflowRuntimeVariable[] {
  return [
    { key: "context.tenantId", type: "int64", source: "conversation.tenantId", required: true },
    { key: "context.productId", type: "int64", source: "conversation.productId", required: true },
    { key: "context.deviceId", type: "int64", source: "conversation.deviceId" },
    { key: "context.serviceCodeId", type: "int64", source: "conversation.serviceCodeId" },
    { key: "context.conversationId", type: "int64", source: "conversation.id", required: true },
    { key: "agent.rag.knowledgeBaseIds", type: "array<int64>", source: "agentRelease.knowledgeScope.bindings[].knowledgeBaseId", required: true },
    { key: "agent.rag.revisionIds", type: "array<int64>", source: "agentRelease.knowledgeScope.revisions[].revisionId", required: true },
    { key: "agent.model.llm", type: "string", source: "agentRelease.agentConfig.llmModelName", required: true },
    { key: "agent.skill.ids", type: "array<int64>", source: "agentRelease.agentConfig.skillIds" },
    { key: "agent.tool.codes", type: "array<string>", source: "agentRelease.agentConfig.allowedTools" },
    { key: "agent.handoff.teamIds", type: "array<int64>", source: "agentRelease.agentConfig.teamIds" },
  ]
}

function RuntimeBindingsPanel({ definition }: { definition: AIWorkflowDefinition }) {
  const t = useI18n()
  const knowledgeSupport = isKnowledgeSupportWorkflowDefinition(definition)
  const variables = definition.runtimeVariables?.length ? definition.runtimeVariables : runtimeVariableFallback()
  const knowledgeNodes = definition.nodes.filter((node) => node.type === "knowledge_retrieve")

  return (
    <div className="space-y-6">
      <section className="grid border-y lg:grid-cols-[minmax(0,1.2fr)_repeat(3,minmax(10rem,0.45fr))]">
        <div className="px-4 py-5">
          <div className="flex items-center gap-2 text-sm font-semibold"><DatabaseIcon className="size-4 text-primary" />{wd(t, knowledgeSupport ? "runtime.tenantKnowledgeTitle" : "runtime.knowledgeTitle")}</div>
        </div>
        <div className="border-t px-4 py-5 lg:border-l lg:border-t-0">
          <div className="text-xs text-muted-foreground">{wd(t, "runtime.knowledgeStage")}</div>
          <div className="mt-2 text-2xl font-semibold tabular-nums">{knowledgeNodes.length}</div>
            <div className="mt-1 text-xs text-muted-foreground">{wd(t, "runtime.knowledgeCalls")}</div>
        </div>
        <div className="border-t px-4 py-5 lg:border-l lg:border-t-0">
          <div className="text-xs text-muted-foreground">{wd(t, "runtime.isolation")}</div>
          <div className="mt-2 text-base font-semibold">{wd(t, knowledgeSupport ? "runtime.byTenant" : "runtime.byProduct")}</div>
          <div className="mt-1 text-xs text-muted-foreground">{wd(t, "runtime.noKnowledgeBaseId")}</div>
        </div>
        <div className="border-t px-4 py-5 lg:border-l lg:border-t-0">
          <div className="text-xs text-muted-foreground">{wd(t, "runtime.knowledgeVersion")}</div>
          <div className="mt-2 text-base font-semibold">{wd(t, "runtime.fixedOnPublish")}</div>
          <div className="mt-1 text-xs text-muted-foreground">{wd(t, "runtime.traceableAnswers")}</div>
        </div>
      </section>

      <section>
        <div className="text-sm font-semibold">{wd(t, "runtime.callPosition")}</div>
        <div className="mt-3 grid gap-3 lg:grid-cols-2">
          {knowledgeNodes.map((node, index) => (
            <article key={node.id} className="rounded-lg border bg-background p-4">
              <div className="flex items-start gap-3">
                <span className="flex size-9 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
                  <DatabaseIcon className="size-4" />
                </span>
                <div className="min-w-0">
                  <div className="text-sm font-semibold">{node.name || wd(t, "runtime.knowledgeNode", { index: index + 1 })}</div>
                </div>
              </div>
            </article>
          ))}
          {knowledgeNodes.length === 0 ? (
            <div className="border-y py-10 text-center text-sm text-muted-foreground">{wd(t, knowledgeSupport ? "runtime.emptyTenantKnowledge" : "runtime.emptyKnowledge")}</div>
          ) : null}
        </div>
      </section>

      <details className="group border-y">
        <summary className="flex cursor-pointer list-none items-center justify-between gap-3 py-4 text-sm font-medium">
          <span className="flex items-center gap-2"><BracesIcon className="size-4 text-muted-foreground" />{wd(t, "runtime.advancedConfig")}</span>
          <ChevronDownIcon className="size-4 text-muted-foreground transition-transform group-open:rotate-180" />
        </summary>
        <div className="grid gap-2 border-t py-4 sm:grid-cols-2">
          {variables.map((variable) => (
            <div key={variable.key} className="rounded-md border bg-muted/20 p-3">
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <div className="break-all text-xs font-medium">{variable.key}</div>
                  <div className="mt-1 break-all text-xs leading-5 text-muted-foreground">{variable.source}</div>
                </div>
                <Badge variant="outline" className="shrink-0">{variable.required ? wd(t, "runtime.required") : variable.type}</Badge>
              </div>
            </div>
          ))}
        </div>
      </details>
    </div>
  )
}

export function EnterpriseWorkflowDetailPage({
  defaultPreviewOpen = false,
  workflowId: requestedWorkflowId,
}: {
  defaultPreviewOpen?: boolean
  workflowId?: number
} = {}) {
  const t = useI18n()
  const params = useParams<{ workflowId?: string }>()
  const router = useRouter()
  const confirm = useConfirm()
  const workflowId = requestedWorkflowId ?? Number(params.workflowId)
  const session = useMemo(() => readSession(), [])
  const canUpdate = CanUseButton("aiWorkflow.update", session?.permissions)
  const canPublish = CanUseButton("aiWorkflow.publish", session?.permissions)
  const canBindAgent = CanUseButton("aiAgent.update", session?.permissions)

  const [workflow, setWorkflow] = useState<AIWorkflow | null>(null)
  const [versions, setVersions] = useState<AIWorkflowVersion[]>([])
  const [versionsPage, setVersionsPage] = useState<OptionPageState>(emptyVersionPage)
  const [versionsLoading, setVersionsLoading] = useState(false)
  const [latestVersionNumber, setLatestVersionNumber] = useState(0)
  const [agents, setAgents] = useState<AIAgent[]>([])
  const [availableAgents, setAvailableAgents] = useState<AIAgent[]>([])
  const [availableAgentsPage, setAvailableAgentsPage] = useState<OptionPageState>(emptyAgentOptionPage)
  const [availableAgentsLoading, setAvailableAgentsLoading] = useState(false)
  const [adoptedAgentsPage, setAdoptedAgentsPage] = useState<OptionPageState>(emptyAdoptionPage)
  const [adoptedAgentsLoading, setAdoptedAgentsLoading] = useState(false)
  const [adoptionCount, setAdoptionCount] = useState(0)
  const [currentVersionAdoptionCount, setCurrentVersionAdoptionCount] = useState(0)
  const [blueprintTemplates, setBlueprintTemplates] = useState<AIWorkflow[]>([])
  const [blueprintTemplatesLoading, setBlueprintTemplatesLoading] = useState(false)
  const [definition, setDefinition] = useState<AIWorkflowDefinition | null>(null)
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [loading, setLoading] = useState(true)
  const [templateLoaded, setTemplateLoaded] = useState(false)
  const [adoptedAgentsLoaded, setAdoptedAgentsLoaded] = useState(false)
  const [saving, setSaving] = useState("")
  const [testOpen, setTestOpen] = useState(false)
  const [testAgentId, setTestAgentId] = useState(0)
  const [testMessage, setTestMessage] = useState(() => wd(t, "test.defaultMessage"))
  const [testAutoConfirm, setTestAutoConfirm] = useState(true)
  const [testBranchOverrides, setTestBranchOverrides] = useState<Record<string, string>>({})
  const [testRuntimeContext, setTestRuntimeContext] = useState<WorkflowTestRuntimeContext>({})
  const [testRunning, setTestRunning] = useState(false)
  const [batchTestRunning, setBatchTestRunning] = useState(false)
  const [batchTestResults, setBatchTestResults] = useState<WorkflowBatchTestCaseResult[]>([])
  const [testResult, setTestResult] = useState<AIWorkflowTestRunResult | null>(null)
  const [upgradingAgentId, setUpgradingAgentId] = useState(0)
  const [rollingBackVersionId, setRollingBackVersionId] = useState(0)
  const [activeTab, setActiveTab] = useState("compose")

  const load = useCallback(async (quiet = false) => {
    if (!Number.isInteger(workflowId) || workflowId <= 0) {
      setLoading(false)
      return
    }
    if (!quiet) setLoading(true)
    try {
      const workflowData = await fetchEnterpriseAIWorkflow(workflowId)
      setWorkflow(workflowData)
      setDefinition(workflowData.draftDefinition)
      setName(workflowData.name)
      setDescription(workflowData.description)
      setTestResult(null)
      setBatchTestResults([])
      setTestBranchOverrides({})
      setTestRuntimeContext({})
      setTemplateLoaded(true)
      setLoading(false)

      setVersionsLoading(true)
      const versionsTask = fetchWorkflowVersionPage(
        workflowId,
        1,
        WORKFLOW_VERSION_PAGE_SIZE,
        workflowData.currentStableVersionId,
      )
        .then((versionData) => {
          setVersions(versionData.versions)
          setVersionsPage(versionData.page)
          setLatestVersionNumber(versionData.latestVersionNumber)
        })
        .catch((error) => toast.error(error instanceof Error ? error.message : wd(t, "errors.versionLoadFailed")))
        .finally(() => setVersionsLoading(false))

      setAvailableAgentsLoading(true)
      const availableAgentsTask = fetchEnterpriseAIAgents({ productScoped: 0, page: 1, limit: WORKFLOW_AGENT_OPTION_PAGE_SIZE })
        .then((response) => {
          const availableAgentPage = unwrapEnterpriseAgentPage(response, t)
          const activeAgents = availableAgentPage.results.filter((item) => item.status === Status.Ok)
          setAvailableAgents(activeAgents)
          setAvailableAgentsPage(availableAgentPage.page)
          setTestAgentId((current) =>
            current ||
            activeAgents.find((item) => item.workflowId === workflowId)?.id ||
            activeAgents[0]?.id ||
            0
          )
        })
        .catch((error) => toast.error(error instanceof Error ? error.message : wd(t, "errors.testConfigLoadFailed")))
        .finally(() => setAvailableAgentsLoading(false))

      setAdoptedAgentsLoading(true)
      const adoptedAgentsTask = fetchEnterpriseAIAgents({ workflowId, page: 1, limit: WORKFLOW_ADOPTION_PAGE_SIZE })
        .then(async (response) => {
          const adoptedAgentPage = unwrapEnterpriseAgentPage(response, t, "errors.productAdoptionLoadFailed")
          const adoptedAgents = adoptedAgentPage.results.filter((item) => item.status !== Status.Deleted)
          setAgents(adoptedAgents)
          setAdoptedAgentsPage(adoptedAgentPage.page)
          setAdoptedAgentsLoaded(true)
          setTestAgentId((current) => current || adoptedAgents.find((item) => item.status === Status.Ok)?.id || 0)
          try {
            const adoptionStatus = await fetchEnterpriseAIWorkflowAdoption([workflowId])
            const adoptionKey = String(workflowId)
            setAdoptionCount(adoptionStatus.adoption[adoptionKey] ?? adoptedAgentPage.page.total ?? adoptedAgents.length)
            setCurrentVersionAdoptionCount(adoptionStatus.stableAdoption[adoptionKey] ?? 0)
          } catch (error) {
            setAdoptionCount(adoptedAgentPage.page.total ?? adoptedAgents.length)
            setCurrentVersionAdoptionCount(0)
            toast.error(error instanceof Error ? error.message : wd(t, "errors.productAdoptionStatsLoadFailed"))
          }
        })
        .catch((error) => toast.error(error instanceof Error ? error.message : wd(t, "errors.productAdoptionLoadFailed")))
        .finally(() => setAdoptedAgentsLoading(false))

      setBlueprintTemplatesLoading(true)
      const blueprintTemplatesTask = fetchEnterpriseAIWorkflows({ page: 1, limit: WORKFLOW_BLUEPRINT_TEMPLATE_PAGE_SIZE, scope: "platform" })
        .then((workflowPage) => {
          setBlueprintTemplates(workflowPage.results.filter((item) => item.scope === "platform" && item.currentStableVersionId > 0))
        })
        .catch((error) => toast.error(error instanceof Error ? error.message : wd(t, "errors.blueprintLoadFailed")))
        .finally(() => setBlueprintTemplatesLoading(false))

      await Promise.all([versionsTask, availableAgentsTask, adoptedAgentsTask, blueprintTemplatesTask])
    } catch (error) {
      toast.error(error instanceof Error ? error.message : wd(t, "errors.templateLoadFailed"))
      setWorkflow((current) => (current?.id === workflowId ? current : null))
      setLoading(false)
      setVersionsLoading(false)
      setAvailableAgentsLoading(false)
      setAdoptedAgentsLoading(false)
      setBlueprintTemplatesLoading(false)
    }
  }, [t, workflowId])

  useEffect(() => {
    void load()
  }, [load])

  const loadVersionPage = useCallback(async (
    page = versionsPage.page,
    limit = versionsPage.limit,
  ) => {
    if (!Number.isInteger(workflowId) || workflowId <= 0) return
    setVersionsLoading(true)
    try {
      const versionData = await fetchWorkflowVersionPage(
        workflowId,
        page,
        limit,
        workflow?.currentStableVersionId ?? 0,
      )
      setVersions(versionData.versions)
      setVersionsPage(versionData.page)
      setLatestVersionNumber((current) => Math.max(current, versionData.latestVersionNumber))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : wd(t, "errors.versionLoadFailed"))
    } finally {
      setVersionsLoading(false)
    }
  }, [t, versionsPage.limit, versionsPage.page, workflow?.currentStableVersionId, workflowId])

  const loadAvailableAgentPage = useCallback(async (page = 1) => {
    if (!Number.isInteger(workflowId) || workflowId <= 0) return
    setAvailableAgentsLoading(true)
    try {
      const nextPage = unwrapEnterpriseAgentPage(await fetchEnterpriseAIAgents({
        productScoped: 0,
        page,
        limit: availableAgentsPage.limit,
      }), t)
      const selectedAgent = availableAgents.find((item) => item.id === testAgentId)
      setAvailableAgents(mergeById(nextPage.results.filter((item) => item.status === Status.Ok), selectedAgent))
      setAvailableAgentsPage(nextPage.page)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : wd(t, "errors.testConfigLoadFailed"))
    } finally {
      setAvailableAgentsLoading(false)
    }
  }, [availableAgents, availableAgentsPage.limit, t, testAgentId, workflowId])

  const loadAdoptedAgentPage = useCallback(async (
    page = adoptedAgentsPage.page,
    limit = adoptedAgentsPage.limit,
  ) => {
    if (!Number.isInteger(workflowId) || workflowId <= 0) return
    setAdoptedAgentsLoading(true)
    try {
      const nextPage = unwrapEnterpriseAgentPage(
        await fetchEnterpriseAIAgents({ workflowId, page, limit }),
        t,
        "errors.productAdoptionLoadFailed",
      )
      setAgents(nextPage.results.filter((item) => item.status !== Status.Deleted))
      setAdoptedAgentsPage(nextPage.page)
      setAdoptedAgentsLoaded(true)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : wd(t, "errors.productAdoptionLoadFailed"))
    } finally {
      setAdoptedAgentsLoading(false)
    }
  }, [adoptedAgentsPage.limit, adoptedAgentsPage.page, t, workflowId])

  useEffect(() => {
    setBatchTestResults([])
  }, [definition])

  const editable = Boolean(
    workflow && workflow.scope === "tenant" && workflow.agentId === 0 && !workflow.locked && canUpdate,
  )
  const stableVersion = workflow
    ? versions.find((item) => item.id === workflow.currentStableVersionId)
    : undefined
  const draftDirty = Boolean(workflow && definition && (
    name.trim() !== workflow.name
    || description.trim() !== workflow.description
    || JSON.stringify(definition) !== JSON.stringify(workflow.draftDefinition)
  ))
  const hasUnpublishedDraft = Boolean(workflow && stableVersion
    && JSON.stringify(workflow.draftDefinition) !== JSON.stringify(stableVersion.definition))
  const staleAgentCount = stableVersion
    ? Math.max(0, adoptionCount - currentVersionAdoptionCount)
    : 0
  async function saveDraft() {
    if (!workflow || !definition || !editable || saving) return
    setSaving("save")
    try {
      const updated = await updateEnterpriseAIWorkflowDraft(workflow.id, {
        name: name.trim(),
        description: description.trim(),
        definition,
      })
      setWorkflow(updated)
      toast.success(wd(t, "messages.draftSaved"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : wd(t, "errors.draftSaveFailed"))
    } finally {
      setSaving("")
    }
  }

  async function publish() {
    if (!workflow || !definition || !editable || !canPublish || saving) return
    setSaving("publish")
    try {
      const published = await publishEnterpriseAIWorkflow(workflow.id, {
        name: name.trim(),
        description: description.trim(),
        definition,
      })
      toast.success(published.id === workflow.currentStableVersionId
        ? wd(t, "messages.publishNoChange")
        : wd(t, "messages.published", { version: published.version }))
      await load(true)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : wd(t, "errors.publishFailed"))
    } finally {
      setSaving("")
    }
  }

  async function runDraftTest() {
    if (!workflow || !definition || testRunning || batchTestRunning) return
    if (testAgentId <= 0) {
      toast.error(wd(t, "errors.selectTestAgent"))
      return
    }
    if (!testMessage.trim()) {
      toast.error(wd(t, "errors.enterTestMessage"))
      return
    }
    setTestRunning(true)
    setTestResult(null)
    try {
      const result = await testEnterpriseAIWorkflow(workflow.id, {
        aiAgentId: testAgentId,
        definition,
        userMessage: testMessage.trim(),
        autoConfirm: testAutoConfirm,
        branchOverrides: testBranchOverrides,
        runtimeContext: testRuntimeContext,
      })
      setTestResult(result)
      if (result.status === "completed") toast.success(wd(t, "messages.testCompleted"))
      else if (result.status === "interrupted") toast.info(wd(t, "messages.testInterrupted"))
      else toast.error(result.errorMessage || wd(t, "errors.testFailed"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : wd(t, "errors.workflowTestFailed"))
    } finally {
      setTestRunning(false)
    }
  }

  async function runBatchTests(scenarios: WorkflowTestScenario[]) {
    if (!workflow || !definition || testRunning || batchTestRunning || scenarios.length === 0) return
    if (testAgentId <= 0) {
      toast.error(wd(t, "errors.selectTestAgent"))
      return
    }
    setBatchTestRunning(true)
    let nextResults: WorkflowBatchTestCaseResult[] = scenarios.map((scenario) => ({ scenarioKey: scenario.key, status: "pending" }))
    setBatchTestResults(nextResults)
    for (const scenario of scenarios) {
      nextResults = nextResults.map((item) => item.scenarioKey === scenario.key ? { ...item, status: "running" } : item)
      setBatchTestResults(nextResults)
      try {
        const result = await testEnterpriseAIWorkflow(workflow.id, {
          aiAgentId: testAgentId,
          definition,
          userMessage: (scenario.message || testMessage).trim(),
          autoConfirm: scenario.autoConfirm,
          branchOverrides: scenario.branchOverrides,
          runtimeContext: scenario.runtimeContext,
        })
        const passed = result.status === "completed" || (result.status === "interrupted" && !scenario.autoConfirm)
        nextResults = nextResults.map((item) => item.scenarioKey === scenario.key ? {
          ...item,
          status: passed ? "passed" : "failed",
          result,
        } : item)
      } catch (error) {
        nextResults = nextResults.map((item) => item.scenarioKey === scenario.key ? {
          ...item,
          status: "failed",
          errorMessage: error instanceof Error ? error.message : wd(t, "errors.scenarioFailed"),
        } : item)
      }
      setBatchTestResults(nextResults)
    }
    const failed = nextResults.filter((item) => item.status === "failed").length
    if (failed > 0) toast.error(wd(t, "messages.batchFailed", { failed }))
    else toast.success(wd(t, "messages.batchPassed", { count: nextResults.length }))
    setBatchTestRunning(false)
  }

  async function applyServiceBlueprint(template: AIWorkflow) {
    if (!editable || !definition || saving) return
    const profile = getWorkflowProductProfile(template.draftDefinition)
    const accepted = await confirm({
      title: wd(t, "confirm.applyBlueprintTitle", { title: profile.title }),
      confirmText: wd(t, "confirm.applyToDraft"),
    })
    if (!accepted) return
    setDefinition(structuredClone(template.draftDefinition))
    setTestResult(null)
    setTestBranchOverrides({})
    setTestRuntimeContext({})
    toast.success(wd(t, "messages.blueprintApplied", { title: profile.title }))
  }

  async function prepareAgentUpgrade(agent: AIAgent) {
    if (!workflow || !stableVersion || !canBindAgent || upgradingAgentId > 0) return
    const accepted = await confirm({
      title: wd(t, "confirm.prepareUpgradeTitle", { name: agent.productName || agent.name, version: stableVersion.version }),
      confirmText: wd(t, "confirm.prepareUpgrade"),
    })
    if (!accepted) return
    setUpgradingAgentId(agent.id)
    try {
      await bindAIAgentWorkflowVersion(agent.id, stableVersion.id)
      const patchAgent = (item: AIAgent): AIAgent => item.id === agent.id ? {
        ...item,
        workflowId: workflow.id,
        workflowVersionId: stableVersion.id,
        draftRevision: item.draftRevision + 1,
        reviewStatus: "unreviewed",
        reviewStatusName: wd(t, "status.pendingReview"),
      } : item
      setAgents((items) => items.map(patchAgent))
      setAvailableAgents((items) => items.map(patchAgent))
      if (agent.workflowVersionId !== stableVersion.id) {
        setCurrentVersionAdoptionCount((count) => Math.min(adoptionCount, count + 1))
      }
      toast.success(wd(t, "messages.upgradePrepared", { version: stableVersion.version }))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : wd(t, "errors.prepareUpgradeFailed"))
    } finally {
      setUpgradingAgentId(0)
    }
  }

  async function rollbackVersion(version: AIWorkflowVersion) {
    if (!workflow || !editable || !canPublish || rollingBackVersionId > 0) return
    const nextVersion = Math.max(latestVersionNumber, ...versions.map((item) => item.version), 0) + 1
    const accepted = await confirm({
      title: wd(t, "confirm.rollbackTitle", { version: version.version, nextVersion }),
      confirmText: wd(t, "confirm.rollbackAndPublish"),
    })
    if (!accepted) return
    setRollingBackVersionId(version.id)
    try {
      const created = await rollbackEnterpriseAIWorkflowVersion(workflow.id, version.id)
      toast.success(wd(t, "messages.rollbackCreated", { fromVersion: version.version, version: created.version }))
      await load(true)
      setActiveTab("versions")
    } catch (error) {
      toast.error(error instanceof Error ? error.message : wd(t, "errors.rollbackFailed"))
    } finally {
      setRollingBackVersionId(0)
    }
  }

  async function archiveTemplate() {
    if (!workflow || workflow.scope !== "tenant" || workflow.agentId !== 0 || adoptionCount > 0 || saving) return
    const accepted = await confirm({
      title: wd(t, "confirm.archiveTitle", { name: workflow.name }),
      confirmText: wd(t, "confirm.archive"),
      variant: "destructive",
    })
    if (!accepted) return
    setSaving("archive")
    try {
      await deleteEnterpriseAIWorkflow(workflow.id)
      toast.success(wd(t, "messages.archived"))
      router.push("/enterprise/workflow")
    } catch (error) {
      toast.error(error instanceof Error ? error.message : wd(t, "errors.archiveFailed"))
    } finally {
      setSaving("")
    }
  }

  async function copyAsTenantTemplate() {
    if (!workflow || workflow.currentStableVersionId <= 0 || !canUpdate || saving) return
    setSaving("copy")
    try {
      const created = await createEnterpriseAIWorkflowTemplate({
        name: wd(t, "messages.copiedName", { name: cleanWorkflowBusinessLabel(t, workflow.name) }),
        description: workflow.description,
        sourceVersionId: workflow.currentStableVersionId,
      })
      toast.success(wd(t, "messages.copied"))
      router.push(buildEnterpriseWorkflowPath(created.id))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : wd(t, "errors.copyFailed"))
    } finally {
      setSaving("")
    }
  }

  const initialLoading = loading && !templateLoaded

  if (!workflow || !definition) {
    if (initialLoading) return <EnterpriseWorkflowTemplateLoading activeTab={activeTab} onTabChange={setActiveTab} />
    return (
      <div className="border-y py-12 text-center">
        <CircleAlertIcon className="mx-auto size-6 text-destructive" />
        <h1 className="mt-3 font-semibold">{wd(t, "empty.notFound")}</h1>
        <Button className="mt-4" variant="outline" render={<Link href="/enterprise/workflow" />}>{wd(t, "actions.backToList")}</Button>
      </div>
    )
  }

  return (
    <div className="rhd-railops-workflow-detail-page space-y-5">
      <header className="flex flex-col gap-4 border-b pb-4 xl:flex-row xl:items-start xl:justify-between">
        <div className="flex min-w-0 items-start gap-3">
          <Button variant="outline" size="icon" title={wd(t, "actions.backToList")} render={<Link href="/enterprise/workflow" />}>
            <ArrowLeftIcon />
          </Button>
          <span className="flex size-10 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
            <WorkflowIcon className="size-5" />
          </span>
          <div className="min-w-0">
            <RouteBreadcrumbs size="page" />
            <div className="mt-2 flex flex-wrap items-center gap-2">
              <h1 className="sr-only">{cleanWorkflowBusinessLabel(t, workflow.name)}</h1>
              <span className="text-sm font-semibold text-foreground">{cleanWorkflowBusinessLabel(t, workflow.name)}</span>
              <Badge variant={workflow.scope === "platform" ? "default" : "secondary"}>
                {workflow.scope === "platform" ? wd(t, "scope.platform") : wd(t, "scope.tenant")}
              </Badge>
              {workflow.locked ? <Badge variant="outline"><LockKeyholeIcon />{wd(t, "status.protected")}</Badge> : null}
              <Badge variant={workflow.currentStableVersionId > 0 ? "outline" : "destructive"}>
                {stableVersion ? wd(t, "version.stable", { version: stableVersion.version }) : wd(t, "status.notPublished")}
              </Badge>
            </div>
            {workflow.description.trim() ? (
              <p className="mt-2 max-w-4xl text-sm leading-6 text-muted-foreground">{cleanWorkflowBusinessLabel(t, workflow.description)}</p>
            ) : null}
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          {!editable ? (
            <Button variant="outline" onClick={() => setTestOpen(true)} disabled={testRunning || batchTestRunning}>
              {testRunning ? <Loader2Icon className="animate-spin" /> : <PlayIcon />}
              {wd(t, "actions.testRun")}
            </Button>
          ) : null}
          <Button variant="outline" size="icon" title={wd(t, "actions.refresh")} onClick={() => void load()} disabled={loading || Boolean(saving)}>
            <RefreshCwIcon />
          </Button>
          {canUpdate && workflow.currentStableVersionId > 0 ? (
            <Button variant="outline" onClick={() => void copyAsTenantTemplate()} disabled={Boolean(saving)}>
              {saving === "copy" ? <Loader2Icon className="animate-spin" /> : <CopyPlusIcon />}
              {wd(t, "actions.copyAndConfigure")}
            </Button>
          ) : null}
          {workflow.scope === "tenant" && workflow.agentId === 0 && canUpdate ? (
            <Button
              variant="outline"
              disabled={adoptionCount > 0 || Boolean(saving)}
              title={adoptionCount > 0 ? wd(t, "actions.archiveDisabledTitle") : wd(t, "actions.archiveTitle")}
              onClick={() => void archiveTemplate()}
            >
              {saving === "archive" ? <Loader2Icon className="animate-spin" /> : <Trash2Icon />}
              {wd(t, "actions.archive")}
            </Button>
          ) : null}
        </div>
      </header>

      <div>
        <WorkflowTemplateTabList value={activeTab} onChange={setActiveTab} knowledgeSupport={isKnowledgeSupportWorkflowDefinition(definition)} />

        { activeTab === "compose" ? (
        <div className="mt-4 space-y-4">
          {editable ? (
            <>
              <section className="flex flex-col gap-3 border-y bg-muted/20 px-3 py-3 lg:flex-row lg:items-center lg:justify-between">
                <div className="flex min-w-0 items-center gap-2 text-sm">
                  {draftDirty ? <CircleAlertIcon className="size-4 text-amber-600" /> : <CheckCircle2Icon className="size-4 text-primary" />}
                  <span className="font-medium">{draftDirty ? wd(t, "compose.unsaved") : hasUnpublishedDraft ? wd(t, "compose.draftSaved") : wd(t, "compose.stableAligned")}</span>
                </div>
                <div className="flex flex-wrap gap-2">
                  <Button variant="outline" onClick={() => setTestOpen(true)} disabled={testRunning || batchTestRunning || Boolean(saving)}>
                    <PlayIcon />{wd(t, "actions.testRun")}
                  </Button>
                  <Button variant="outline" onClick={() => void saveDraft()} disabled={!draftDirty || Boolean(saving) || !name.trim()}>
                    {saving === "save" ? <Loader2Icon className="animate-spin" /> : <ClipboardCheckIcon />}{wd(t, "compose.saveDraft")}
                  </Button>
                  {canPublish ? (
                    <Button onClick={() => void publish()} disabled={Boolean(saving) || !name.trim() || (!draftDirty && !hasUnpublishedDraft)}>
                      {saving === "publish" ? <Loader2Icon className="animate-spin" /> : <ShieldCheckIcon />}{wd(t, "compose.publishStable")}
                    </Button>
                  ) : null}
                </div>
              </section>
              {stableVersion && staleAgentCount > 0 && !draftDirty && !hasUnpublishedDraft ? (
                <div className="flex flex-col gap-3 border-b border-amber-300 bg-amber-50 px-4 py-3 text-sm sm:flex-row sm:items-center sm:justify-between">
                  <div className="flex items-start gap-2">
                    <CircleAlertIcon className="mt-0.5 size-4 shrink-0 text-amber-700" />
                    <div>
                      <div className="font-medium text-amber-950">{wd(t, "compose.staleNotice", { version: stableVersion.version, count: staleAgentCount })}</div>
                    </div>
                  </div>
                  <Button size="sm" variant="outline" className="shrink-0 bg-background" onClick={() => setActiveTab("adoption")}>
                    {wd(t, "compose.handleUpgrade")}<ArrowRightIcon />
                  </Button>
                </div>
              ) : null}
              <WorkflowServiceHero
                workflow={workflow}
                definition={definition}
                stableVersion={stableVersion}
                adoptionCount={adoptionCount}
                currentVersionAdoptionCount={currentVersionAdoptionCount}
              />
              <WorkflowConfigTabs
                adoptionCount={adoptionCount}
                currentVersionAdoptionCount={currentVersionAdoptionCount}
                workflow={workflow}
                definition={definition}
                stableVersion={stableVersion}
                editable={editable}
                saving={saving === "save"}
                name={name}
                description={description}
                templates={blueprintTemplates}
                templatesLoading={blueprintTemplatesLoading}
                defaultPreviewOpen={defaultPreviewOpen}
                testRun={testResult}
                onNameChange={setName}
                onDescriptionChange={setDescription}
                onBlueprintApply={(template) => void applyServiceBlueprint(template)}
                onDefinitionChange={(nextDefinition) => {
                  setDefinition(nextDefinition)
                  setTestResult(null)
                }}
                onCredentialChainChange={(credentialChain) => {
                  setDefinition({ ...definition, modelPolicy: { ...definition.modelPolicy, credentialChain } })
                  setTestResult(null)
                }}
                onSave={() => void saveDraft()}
              />
            </>
          ) : (
            <>
              <div className="flex items-start justify-between gap-4 border-y py-3 text-sm">
                <div className="flex min-w-0 items-start gap-2.5">
                  <LockKeyholeIcon className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
                  <div>
                    <div className="font-medium">{wd(t, "compose.readonlyPlatform")}</div>
                  </div>
                </div>
              </div>
              <WorkflowServiceHero
                workflow={workflow}
                definition={definition}
                stableVersion={stableVersion}
                adoptionCount={adoptionCount}
                currentVersionAdoptionCount={currentVersionAdoptionCount}
              />
              <WorkflowConfigTabs
                adoptionCount={adoptionCount}
                currentVersionAdoptionCount={currentVersionAdoptionCount}
                workflow={workflow}
                definition={definition}
                stableVersion={stableVersion}
                editable={false}
                saving={false}
                name={name}
                description={description}
                templates={[]}
                templatesLoading={false}
                defaultPreviewOpen={defaultPreviewOpen}
                testRun={testResult}
                onNameChange={() => undefined}
                onDescriptionChange={() => undefined}
                onBlueprintApply={() => undefined}
                onDefinitionChange={() => undefined}
                onCredentialChainChange={() => undefined}
                onSave={() => undefined}
              />
            </>
          )}
        </div>
        ) : null}

        { activeTab === "runs" ? (
        <div className="mt-6 space-y-5">
          <section className="grid border-y lg:grid-cols-[minmax(0,1.35fr)_minmax(20rem,0.65fr)]">
            <div className="px-4 py-5">
              <div className="flex items-center gap-2 text-sm font-semibold"><ActivityIcon className="size-4 text-primary" />{wd(t, "runsPanel.title")}</div>
            </div>
            <div className="border-t px-4 py-5 lg:border-l lg:border-t-0">
              <div className="text-xs text-muted-foreground">{wd(t, "runsPanel.scope")}</div>
              <div className="mt-2 flex items-center gap-2 text-base font-semibold"><Layers3Icon className="size-4 text-primary" />{wd(t, "runsPanel.currentAdoption")}</div>
            </div>
          </section>
          <AIWorkflowRunsWorkspace workflowId={workflow.id} layout="fragment" />
        </div>
        ) : null}

        { activeTab === "bindings" ? (
        <div className="mt-6">
          <RuntimeBindingsPanel definition={definition} />
        </div>
        ) : null}

        { activeTab === "versions" ? (
        <div className="mt-6">
          <WorkflowVersionPanel
            workflow={workflow}
            versions={versions}
            versionsPage={versionsPage}
            loading={versionsLoading}
            canRollback={editable && canPublish}
            rollingBackVersionId={rollingBackVersionId}
            onPageChange={(page) => void loadVersionPage(page)}
            onLimitChange={(limit) => void loadVersionPage(1, limit)}
            onRollback={(version) => void rollbackVersion(version)}
          />
        </div>
        ) : null}

        { activeTab === "adoption" ? (
        <div className="mt-6">
          <WorkflowAdoptionPanel
            agents={agents}
            agentsPage={adoptedAgentsPage}
            versions={versions}
            stableVersion={stableVersion}
            initialLoading={adoptedAgentsLoading && !adoptedAgentsLoaded}
            loading={adoptedAgentsLoading}
            canBind={canBindAgent}
            upgradingAgentId={upgradingAgentId}
            onPageChange={(page) => void loadAdoptedAgentPage(page)}
            onLimitChange={(limit) => void loadAdoptedAgentPage(1, limit)}
            onPrepareUpgrade={(agent) => void prepareAgentUpgrade(agent)}
            knowledgeSupport={isKnowledgeSupportWorkflowDefinition(definition)}
          />
        </div>
        ) : null}

      </div>
      <WorkflowTestPanel
        open={testOpen}
        onOpenChange={setTestOpen}
        workflow={workflow}
        definition={definition}
        agents={availableAgents}
        agentsPage={availableAgentsPage}
        agentsLoading={availableAgentsLoading}
        agentId={testAgentId}
        onAgentChange={setTestAgentId}
        onAgentPageChange={(page) => void loadAvailableAgentPage(page)}
        message={testMessage}
        onMessageChange={setTestMessage}
        autoConfirm={testAutoConfirm}
        onAutoConfirmChange={setTestAutoConfirm}
        branchOverrides={testBranchOverrides}
        onBranchOverridesChange={(overrides) => {
          setTestBranchOverrides(overrides)
          setTestResult(null)
        }}
        runtimeContext={testRuntimeContext}
        onRuntimeContextChange={(context) => {
          setTestRuntimeContext(context)
          setTestResult(null)
        }}
        running={testRunning}
        batchRunning={batchTestRunning}
        batchResults={batchTestResults}
        result={testResult}
        onRun={() => void runDraftTest()}
        onRunBatch={(scenarios) => void runBatchTests(scenarios)}
      />
    </div>
  )
}

function getWorkflowTemplateTabs(t: WorkflowDetailT, knowledgeSupport = false): RailopsTabItem[] {
  return [
    { value: "compose", label: wd(t, "tabs.compose"), icon: <WorkflowIcon /> },
    { value: "runs", label: wd(t, "tabs.runs"), icon: <ActivityIcon /> },
    { value: "bindings", label: wd(t, "tabs.bindings"), icon: <DatabaseIcon /> },
    { value: "versions", label: wd(t, "tabs.versions"), icon: <HistoryIcon /> },
    { value: "adoption", label: wd(t, knowledgeSupport ? "tabs.reception" : "tabs.adoption"), icon: <Layers3Icon /> },
  ]
}

function getPreviewViewTabs(t: WorkflowDetailT): RailopsTabItem[] {
  return [
    { value: "graph", label: wd(t, "tabs.graph"), icon: <NetworkIcon /> },
    { value: "list", label: wd(t, "tabs.list"), icon: <BracesIcon /> },
  ]
}

function WorkflowTemplateTabList({ value, onChange, knowledgeSupport = false }: { value: string; onChange: (value: string) => void; knowledgeSupport?: boolean }) {
  const t = useI18n()
  return (
    <UnderlineTabs ariaLabel={wd(t, "tabs.ariaLabel")} items={getWorkflowTemplateTabs(t, knowledgeSupport)} value={value} onChange={onChange} />
  )
}

function EnterpriseWorkflowTemplateLoading({
  activeTab,
  onTabChange,
}: {
  activeTab: string
  onTabChange: (value: string) => void
}) {
  const t = useI18n()
  return (
    <div className="rhd-railops-workflow-detail-page space-y-5" aria-busy="true" aria-label={wd(t, "loading.page")}>
      <header className="flex flex-col gap-4 border-b pb-4 xl:flex-row xl:items-start xl:justify-between">
        <div className="flex min-w-0 items-start gap-3">
          <Button variant="outline" size="icon" title={wd(t, "actions.backToList")} render={<Link href="/enterprise/workflow" />}>
            <ArrowLeftIcon />
          </Button>
          <span className="flex size-10 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
            <WorkflowIcon className="size-5" />
          </span>
          <div className="min-w-0 flex-1">
            <RouteBreadcrumbs size="page" />
            <div className="mt-2 flex flex-wrap items-center gap-2">
              <Skeleton className="h-4 w-40 max-w-[62vw]" />
              <Skeleton className="h-6 w-16 rounded-md" />
              <Skeleton className="h-6 w-20 rounded-md" />
            </div>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <Skeleton className="h-9 w-9 rounded-md" />
          <Skeleton className="h-9 w-28 rounded-md" />
        </div>
      </header>
      <div>
        <WorkflowTemplateTabList value={activeTab} onChange={onTabChange} />
        { activeTab === "compose" ? (
        <div className="mt-4 space-y-4">
          <ModuleLoading variant="metrics" count={4} label={wd(t, "loading.status")} />
          <ModuleLoading variant="detail" count={4} label={wd(t, "loading.compose")} />
        </div>
        ) : null}
        { activeTab === "runs" ? (
        <div className="mt-6 space-y-5">
          <ModuleLoading variant="list" count={4} label={wd(t, "loading.runs")} />
        </div>
        ) : null}
        { activeTab === "bindings" ? (
        <div className="mt-6">
          <ModuleLoading variant="table" count={4} label={wd(t, "loading.bindings")} />
        </div>
        ) : null}
        { activeTab === "versions" ? (
        <div className="mt-6">
          <ModuleLoading variant="table" count={4} label={wd(t, "loading.versions")} />
        </div>
        ) : null}
        { activeTab === "adoption" ? (
        <div className="mt-6">
          <ModuleLoading variant="list" count={4} label={wd(t, "loading.adoption")} />
        </div>
        ) : null}
      </div>
    </div>
  )
}
