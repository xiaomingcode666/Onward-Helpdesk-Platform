"use client"

import "@xyflow/react/dist/style.css"

import {
  Background,
  BaseEdge,
  Controls,
  Handle,
  MarkerType,
  Position,
  ReactFlow,
  getBezierPath,
  type Edge,
  type EdgeProps,
  type Node,
  type NodeProps,
} from "@xyflow/react"
import {
  AlertTriangleIcon,
  CheckCircle2Icon,
  GitBranchIcon,
  InfoIcon,
  RouteIcon,
  TimerIcon,
  UserRoundCheckIcon,
  WorkflowIcon,
} from "lucide-react"
import { useMemo, useState } from "react"

import { JsonTreeViewer } from "@/components/json-tree-viewer"
import { useI18n } from "@/i18n/provider"
import { Badge } from "@/components/ui/badge"
import { ScrollArea } from "@/components/ui/scroll-area"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { cn } from "@/lib/utils"
import type {
  AIWorkflowDefinition,
  AIWorkflowNodeRun,
  AIWorkflowRun,
  AIWorkflowSkillAudit,
} from "@/lib/api/admin"

type AuditNodeData = Record<string, unknown> & {
  nodeId: string
  nodeType: string
  name: string
  executed: boolean
  statusName?: string
  durationMs?: number
  errorMessage?: string
  selected?: boolean
  skillName?: string
  skillState?: string
  externalHumanState?: "handled" | "waiting"
  targetPosition?: Position
  sourcePosition?: Position
}

type AuditNode = Node<AuditNodeData>
type AuditEdge = Edge<{ executed?: boolean; external?: boolean }>
type AuditGraphMode = "trace" | "definition"
type WorkflowDefinitionNode = AIWorkflowDefinition["nodes"][number]

type BranchDecision = {
  selectedEdgeId?: string
  selectedBranchId?: string
  selectedBranchName?: string
  selectedTargetNodeId?: string
  reason?: string
  evaluations?: BranchEvaluation[]
}

type BranchEvaluation = {
  edgeId?: string
  branchId?: string
  branchName?: string
  targetNodeId?: string
  sourceNodeId?: string
  sourceField?: string
  operator?: string
  leftValue?: unknown
  rightValue?: unknown
  matched?: boolean
}

const auditNodeTypes = {
  auditNode: AuditCanvasNode,
}

const auditEdgeTypes = {
  auditEdge: AuditCanvasEdge,
}

const fitViewOptions = {
  padding: 0.12,
  minZoom: 0.32,
  maxZoom: 1,
}

const defaultEdgeOptions = {
  type: "auditEdge",
  markerEnd: {
    type: MarkerType.ArrowClosed,
  },
}

const auditLayoutScale = {
  x: 1.35,
  y: 1.15,
}

const traceColumns = 6
const traceColumnGap = 210
const traceRowGap = 170

const workflowRunAuditI18nPrefix = "dashboardExtract.workflowRunAudit."
type WorkflowRunAuditT = ReturnType<typeof useI18n>
const wr = (t: WorkflowRunAuditT, key: string, values?: Record<string, string | number>) => t(`${workflowRunAuditI18nPrefix}${key}`, values)

export function WorkflowRunAuditGraph({ run }: { run: AIWorkflowRun }) {
  const t = useI18n()
  const [viewMode, setViewMode] = useState<AuditGraphMode>("trace")
  const nodeRuns = useMemo(() => run.nodes ?? [], [run.nodes])
  const definitionNodes = useMemo(() => run.definition?.nodes ?? [], [run.definition?.nodes])
  const nodeRunByNodeId = useMemo(() => {
    const map = new Map<string, AIWorkflowNodeRun>()
    for (const node of nodeRuns) {
      map.set(node.nodeId, node)
    }
    return map
  }, [nodeRuns])
  const activeEdgeIds = useMemo(() => buildActiveEdgeIds(run.definition, nodeRuns), [run.definition, nodeRuns])
  const externalHumanState = run.humanHandling?.handledByHuman
    ? "handled"
    : run.humanHandling?.handoffOccurred
      ? "waiting"
      : undefined
  const externalHumanNodeId = useMemo(() => {
    if (!externalHumanState || nodeRuns.some((node) => node.nodeType === "handoff_to_human")) {
      return ""
    }
    return definitionNodes.find((node) => node.type === "handoff_to_human")?.id || "__external_human_handling"
  }, [definitionNodes, externalHumanState, nodeRuns])
  const graphDefinitionNodes = useMemo(() => {
    if (!externalHumanNodeId || definitionNodes.some((node) => node.id === externalHumanNodeId)) {
      return definitionNodes
    }
    const maxX = definitionNodes.reduce((value, node) => Math.max(value, node.position?.x ?? 0), 0)
    return [
      ...definitionNodes,
      createSyntheticDefinitionNode(externalHumanNodeId, "human_handling", wr(t, "syntheticNode.humanHandling"), maxX + traceColumnGap, 0),
    ]
  }, [definitionNodes, externalHumanNodeId])
  const traceDefinitionNodes = useMemo(() => {
    const definitionByID = new Map(graphDefinitionNodes.map((node) => [node.id, node]))
    const seen = new Set<string>()
    const traceNodes: WorkflowDefinitionNode[] = []
    for (const nodeRun of nodeRuns) {
      if (seen.has(nodeRun.nodeId)) {
        continue
      }
      seen.add(nodeRun.nodeId)
      traceNodes.push(
        definitionByID.get(nodeRun.nodeId) ||
          createSyntheticDefinitionNode(nodeRun.nodeId, nodeRun.nodeType, nodeRun.nodeId, 0, 0)
      )
    }
    if (externalHumanNodeId && !seen.has(externalHumanNodeId)) {
      const externalNode = definitionByID.get(externalHumanNodeId)
      if (externalNode) {
        traceNodes.push(externalNode)
      }
    }
    return traceNodes
  }, [externalHumanNodeId, graphDefinitionNodes, nodeRuns])
  const visibleDefinitionNodes = viewMode === "trace" ? traceDefinitionNodes : graphDefinitionNodes
  const skillAuditNodeId = useMemo(() => {
    for (let index = nodeRuns.length - 1; index >= 0; index -= 1) {
      if (nodeRuns[index]?.nodeType === "llm_reply") {
        return nodeRuns[index].nodeId
      }
    }
    return ""
  }, [nodeRuns])
  const firstExecutedNodeId = nodeRuns[0]?.nodeId ?? run.definition?.entryNodeId ?? ""
  const [selectedNodeId, setSelectedNodeId] = useState(firstExecutedNodeId)
  const effectiveSelectedNodeId = visibleDefinitionNodes.some((node) => node.id === selectedNodeId)
    ? selectedNodeId
    : visibleDefinitionNodes[0]?.id ?? ""

  const nodes = useMemo<AuditNode[]>(() => {
    return visibleDefinitionNodes.map((node, index) => {
      const nodeRun = nodeRunByNodeId.get(node.id)
      const nodeExternalHumanState = node.id === externalHumanNodeId && !nodeRun ? externalHumanState : undefined
      const traceLayout = viewMode === "trace" ? getTraceNodeLayout(index, visibleDefinitionNodes.length) : undefined
      return {
        id: node.id,
        type: "auditNode",
        position: traceLayout?.position ?? scaleAuditPosition(node.position),
        data: {
          nodeId: node.id,
          nodeType: node.type,
          name: node.name || node.id,
          executed: Boolean(nodeRun),
          statusName: nodeRun?.statusName,
          durationMs: nodeRun?.durationMs,
          errorMessage: nodeRun?.errorMessage,
          selected: effectiveSelectedNodeId === node.id,
          skillName:
            node.id === skillAuditNodeId && run.skillAudit?.state === "selected"
              ? run.skillAudit.selectedSkillName || `Skill #${run.skillAudit.selectedSkillId}`
              : undefined,
          skillState: node.id === skillAuditNodeId ? run.skillAudit?.state : undefined,
          externalHumanState: nodeExternalHumanState,
          targetPosition: traceLayout?.targetPosition,
          sourcePosition: traceLayout?.sourcePosition,
        },
      }
    })
  }, [effectiveSelectedNodeId, externalHumanNodeId, externalHumanState, nodeRunByNodeId, run.skillAudit, skillAuditNodeId, viewMode, visibleDefinitionNodes])

  const edges = useMemo<AuditEdge[]>(() => {
    if (viewMode === "trace") {
      return buildTraceEdges(
        traceDefinitionNodes.map((node) => node.id),
        run.definition,
        externalHumanNodeId
      )
    }
    return (run.definition?.edges ?? []).map((edge) => ({
      id: edge.id,
      source: edge.source,
      target: edge.target,
      type: "auditEdge",
      data: {
        executed: activeEdgeIds.has(edge.id),
      },
    }))
  }, [activeEdgeIds, externalHumanNodeId, run.definition, traceDefinitionNodes, viewMode])

  const selectedNodeRun = effectiveSelectedNodeId ? nodeRunByNodeId.get(effectiveSelectedNodeId) : undefined
  const selectedDefinitionNode = graphDefinitionNodes.find((node) => node.id === effectiveSelectedNodeId)

  if (!definitionNodes.length && !nodeRuns.length) {
    return (
      <div className="rounded-md border border-dashed bg-muted/20 px-3 py-8 text-center text-sm text-muted-foreground">
        {wr(t, "empty.snapshotMissing")}
      </div>
    )
  }

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium">{wr(t, "header.trace")}</span>
          <Badge variant="secondary">{wr(t, "header.executedNodeCount", { count: nodeRuns.length })}</Badge>
        </div>
        <ToggleGroup
          multiple={false}
          value={[viewMode]}
          onValueChange={(value) => {
            const nextMode = value[0] as AuditGraphMode | undefined
            if (nextMode) {
              setViewMode(nextMode)
            }
          }}
          variant="outline"
          size="sm"
          aria-label={wr(t, "header.modeAria")}
        >
          <ToggleGroupItem value="trace" aria-label={wr(t, "mode.trace")}>
            <RouteIcon className="size-3.5" />
            {wr(t, "mode.trace")}
          </ToggleGroupItem>
          <ToggleGroupItem value="definition" aria-label={wr(t, "mode.definition")} disabled={!definitionNodes.length}>
            <WorkflowIcon className="size-3.5" />
            {wr(t, "mode.definition")}
          </ToggleGroupItem>
        </ToggleGroup>
      </div>
      <div className="grid min-h-[600px] overflow-hidden rounded-md border bg-background lg:grid-cols-[minmax(0,1fr)_390px]">
        <div className="h-[600px] min-w-0 border-b bg-muted/10 lg:border-b-0 lg:border-r">
          <ReactFlow
            key={viewMode}
            nodes={nodes}
            edges={edges}
            nodeTypes={auditNodeTypes}
            edgeTypes={auditEdgeTypes}
            defaultEdgeOptions={defaultEdgeOptions}
            fitView
            fitViewOptions={fitViewOptions}
            nodesDraggable={false}
            nodesConnectable={false}
            edgesFocusable={false}
            elementsSelectable
            onNodeClick={(_, node) => setSelectedNodeId(node.id)}
          >
            <Background />
            <Controls showInteractive={false} />
          </ReactFlow>
        </div>
        <AuditSidePanel
          definitionNode={selectedDefinitionNode}
          nodeRun={selectedNodeRun}
          skillAudit={effectiveSelectedNodeId === skillAuditNodeId ? run.skillAudit : undefined}
          externalHumanHandling={
            effectiveSelectedNodeId === externalHumanNodeId && !selectedNodeRun
              ? run.humanHandling
              : undefined
          }
        />
      </div>
    </div>
  )
}

function buildActiveEdgeIds(definition: AIWorkflowDefinition | undefined, nodeRuns: AIWorkflowNodeRun[]) {
  const active = new Set<string>()
  const edges = definition?.edges ?? []
  for (let i = 0; i < nodeRuns.length - 1; i += 1) {
    const source = nodeRuns[i]?.nodeId
    const target = nodeRuns[i + 1]?.nodeId
    const edge = edges.find((item) => item.source === source && item.target === target)
    if (edge) {
      active.add(edge.id)
    }
  }
  return active
}

function buildTraceEdges(
  nodeIds: string[],
  definition: AIWorkflowDefinition | undefined,
  externalHumanNodeId: string
) {
  const definitionEdges = definition?.edges ?? []
  const edges: AuditEdge[] = []
  for (let index = 0; index < nodeIds.length - 1; index += 1) {
    const source = nodeIds[index]
    const target = nodeIds[index + 1]
    const definitionEdge = definitionEdges.find((edge) => edge.source === source && edge.target === target)
    const external = Boolean(externalHumanNodeId && target === externalHumanNodeId)
    edges.push({
      id: external ? `external_${source}_${target}` : definitionEdge?.id || `trace_${index}_${source}_${target}`,
      source,
      target,
      type: "auditEdge",
      data: {
        executed: !external,
        external,
      },
    })
  }
  return edges
}

function AuditCanvasEdge({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  markerEnd,
  data,
}: EdgeProps<AuditEdge>) {
  const [edgePath] = getBezierPath({
    sourceX,
    sourceY,
    sourcePosition,
    targetX,
    targetY,
    targetPosition,
    curvature: 0.18,
  })
  const executed = Boolean(data?.executed)
  const external = Boolean(data?.external)
  return (
    <BaseEdge
      id={id}
      path={edgePath}
      markerEnd={markerEnd}
      className={cn(
        "transition-all",
        external
          ? "!stroke-sky-500 !stroke-[2.2px]"
          : executed
            ? "!stroke-primary !stroke-[2.6px]"
            : "!stroke-muted-foreground/25 !stroke-[1.4px]"
      )}
      style={external ? { strokeDasharray: "7 5" } : undefined}
    />
  )
}

function AuditCanvasNode({ data }: NodeProps<AuditNode>) {
  const t = useI18n()
  const executed = Boolean(data.executed)
  const failed = data.statusName === "failed" || Boolean(data.errorMessage)
  const interrupted = data.statusName === "interrupted"
  const externalHumanState = data.externalHumanState
  const externalHuman = Boolean(externalHumanState)
  const selected = Boolean(data.selected)
  const condition = data.nodeType === "condition"
  const targetPosition = data.targetPosition ?? Position.Left
  const sourcePosition = data.sourcePosition ?? Position.Right
  const toneClass = failed
    ? "border-destructive bg-destructive/5 text-destructive"
      : interrupted
        ? "border-amber-500 bg-amber-500/10 text-amber-700"
      : externalHuman
        ? "border-sky-500 bg-sky-500/10 text-sky-700"
      : executed
        ? "border-primary/20 bg-primary/10 text-primary"
        : "border-border bg-muted/40 text-muted-foreground"

  if (condition) {
    return (
      <div className={cn("relative flex size-24 items-center justify-center opacity-60", executed && "opacity-100")}>
        <Handle type="target" position={targetPosition} className="!size-2.5 !border-0 !bg-muted-foreground/50" />
        <div
          className={cn(
            "absolute inset-3 rotate-45 rounded-lg border shadow-sm transition-all",
            toneClass,
            selected && "ring-4 ring-primary/15"
          )}
        />
        <div className="relative z-10 flex max-w-18 flex-col items-center text-center">
          <GitBranchIcon className="mb-0.5 size-3.5" />
          <div className="line-clamp-2 text-rhd-xs font-medium leading-tight">{data.name}</div>
          <div className="mt-1 text-rhd-2xs opacity-75">{data.statusName || wr(t, "node.notExecuted")}</div>
        </div>
        <Handle type="source" position={sourcePosition} className="!size-2.5 !border-0 !bg-muted-foreground/50" />
      </div>
    )
  }

  return (
    <div
      className={cn(
        "w-40 overflow-hidden rounded-md border bg-background shadow-sm opacity-55 transition-all",
        (executed || externalHuman) && "opacity-100",
        selected && "ring-4 ring-primary/15"
      )}
    >
      <Handle type="target" position={targetPosition} className="!size-2.5 !border-0 !bg-muted-foreground/50" />
      <div className={cn("border-b px-2.5 py-1.5", toneClass)}>
        <div className="flex items-center gap-1.5">
          {failed ? (
            <AlertTriangleIcon className="size-3.5 shrink-0" />
          ) : externalHuman ? (
            <UserRoundCheckIcon className="size-3.5 shrink-0" />
          ) : (
            <CheckCircle2Icon className="size-3.5 shrink-0" />
          )}
          <div className="min-w-0">
            <div className="truncate text-xs font-medium">{data.name}</div>
            <div className="truncate text-rhd-xs opacity-75">{data.nodeType}</div>
          </div>
        </div>
      </div>
      <div className="flex items-center justify-between gap-2 px-2.5 py-1.5 text-rhd-xs text-muted-foreground">
        <span className="min-w-0 truncate">
          {externalHumanState === "handled"
            ? wr(t, "node.handledByHuman")
            : externalHumanState === "waiting"
              ? wr(t, "node.handoffToHuman")
              : data.skillName
                ? wr(t, "node.skillWithName", { name: data.skillName })
                : data.skillState === "not_selected"
                  ? wr(t, "node.skillNotMatched")
                  : data.statusName || wr(t, "node.notExecuted")}
        </span>
        {executed ? (
          <span className="inline-flex items-center gap-1">
            <TimerIcon className="size-3" />
            {data.durationMs ?? 0} ms
          </span>
        ) : null}
      </div>
      <Handle type="source" position={sourcePosition} className="!size-2.5 !border-0 !bg-muted-foreground/50" />
    </div>
  )
}

function AuditSidePanel({
  definitionNode,
  nodeRun,
  skillAudit,
  externalHumanHandling,
}: {
  definitionNode?: AIWorkflowDefinition["nodes"][number]
  nodeRun?: AIWorkflowNodeRun
  skillAudit?: AIWorkflowSkillAudit
  externalHumanHandling?: AIWorkflowRun["humanHandling"]
}) {
  const t = useI18n()
  const inputValue = safeParseJSON(nodeRun?.inputPreview ?? "")
  const outputValue = safeParseJSON(nodeRun?.outputPreview ?? "")
  const branchDecision = extractBranchDecision(outputValue)

  return (
    <ScrollArea className="h-[600px]">
      <div className="space-y-4 p-4">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <InfoIcon className="size-4 text-muted-foreground" />
            <h3 className="text-sm font-semibold">{definitionNode?.name || nodeRun?.nodeId || wr(t, "panel.title")}</h3>
          </div>
          <div className="text-xs text-muted-foreground">
            {definitionNode?.id || nodeRun?.nodeId || "-"} · {definitionNode?.type || nodeRun?.nodeType || "unknown"}
          </div>
        </div>

        {nodeRun ? (
          <div className="grid grid-cols-2 gap-2 text-xs">
            <AuditMeta label={wr(t, "panel.status")} value={nodeRun.statusName || String(nodeRun.status)} />
            <AuditMeta label={wr(t, "panel.duration")} value={`${nodeRun.durationMs || 0} ms`} />
            <AuditMeta label={wr(t, "panel.startedAt")} value={nodeRun.startedAt || "-"} />
            <AuditMeta label={wr(t, "panel.endedAt")} value={nodeRun.endedAt || "-"} />
          </div>
        ) : (
          <div className="rounded-md border border-dashed bg-muted/20 px-3 py-2 text-xs text-muted-foreground">
            {wr(t, "panel.notExecuted")}
          </div>
        )}

        {nodeRun?.errorMessage ? (
          <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs text-destructive">
            {nodeRun.errorMessage}
          </div>
        ) : null}

        {branchDecision ? <BranchDecisionBlock decision={branchDecision} /> : null}

        {skillAudit ? <SkillNodeAudit audit={skillAudit} /> : null}
        {externalHumanHandling?.handledByHuman || externalHumanHandling?.handoffOccurred ? (
          <section className="space-y-1 border-y py-3 text-xs" aria-label={wr(t, "external.aria")}>
            <div className="flex items-center gap-2 font-medium text-sky-700">
              <UserRoundCheckIcon className="size-4" />
              {externalHumanHandling.handledByHuman ? wr(t, "external.handledByHuman") : wr(t, "external.handoffToHuman")}
            </div>
            <p className="leading-5 text-muted-foreground">
              {externalHumanHandling.handledByHuman
                ? wr(t, "external.handledDetail")
                : wr(t, "external.handoffDetail")}
            </p>
            {externalHumanHandling.handlerName ? <div>{wr(t, "external.handlerName", { name: externalHumanHandling.handlerName })}</div> : null}
            {externalHumanHandling.firstHumanReplyAt ? <div>{wr(t, "external.firstReplyAt", { time: externalHumanHandling.firstHumanReplyAt })}</div> : null}
          </section>
        ) : null}

        <PreviewBlock title={wr(t, "preview.input")} raw={nodeRun?.inputPreview ?? ""} value={inputValue} />
        <PreviewBlock title={wr(t, "preview.output")} raw={nodeRun?.outputPreview ?? ""} value={outputValue} />
      </div>
    </ScrollArea>
  )
}

function SkillNodeAudit({ audit }: { audit: AIWorkflowSkillAudit }) {
  const t = useI18n()
  const selected = audit.state === "selected" && audit.selectedSkillId > 0
  return (
    <section className="space-y-2 border-y py-3" aria-label={wr(t, "skill.aria")}>
      <div className="flex flex-wrap items-center gap-2">
        <RouteIcon className="size-4 text-muted-foreground" />
        <span className="text-xs font-medium">{wr(t, "skill.title")}</span>
        <Badge variant={selected ? "default" : audit.state === "not_selected" ? "outline" : "secondary"}>
          {selected ? wr(t, "skill.selected") : audit.state === "not_selected" ? wr(t, "skill.notSelected") : wr(t, "skill.notEnabled")}
        </Badge>
      </div>
      {selected ? (
        <div>
          <div className="text-sm font-medium">{audit.selectedSkillName || `Skill #${audit.selectedSkillId}`}</div>
          <div className="mt-1 text-xs leading-5 text-muted-foreground">
            {audit.selectedSkillDescription || wr(t, "skill.noDescription")}
          </div>
        </div>
      ) : null}
      <div className="space-y-1 text-xs text-muted-foreground">
        <div>{wr(t, "skill.candidates", { value: audit.candidateSkills?.map((skill) => skill.name || `#${skill.id}`).join("、") || "-" })}</div>
        <div>{wr(t, "skill.reason", { value: audit.matchReason || "-" })}</div>
        <div>{wr(t, "skill.invokedTools", { value: audit.invokedToolCodes?.join("、") || "-" })}</div>
        <div>{wr(t, "skill.exposedTools", { value: audit.exposedToolCodes?.join("、") || "-" })}</div>
      </div>
    </section>
  )
}

function AuditMeta({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0 rounded-md border bg-muted/20 px-2 py-1.5">
      <div className="text-rhd-xs text-muted-foreground">{label}</div>
      <div className="truncate font-medium">{value}</div>
    </div>
  )
}

function scaleAuditPosition(position: AIWorkflowDefinition["nodes"][number]["position"] | undefined) {
  return {
    x: Math.round((position?.x ?? 0) * auditLayoutScale.x),
    y: Math.round((position?.y ?? 0) * auditLayoutScale.y),
  }
}

function getTraceNodeLayout(index: number, total: number) {
  const row = Math.floor(index / traceColumns)
  const offset = index % traceColumns
  const rowLength = Math.min(traceColumns, total - row * traceColumns)
  const reversed = row % 2 === 1
  const column = reversed ? traceColumns - 1 - offset : offset
  const hasNextRow = index < total - 1 && offset === rowLength - 1

  return {
    position: {
      x: column * traceColumnGap,
      y: row * traceRowGap,
    },
    targetPosition: row > 0 && offset === 0 ? Position.Top : reversed ? Position.Right : Position.Left,
    sourcePosition: hasNextRow ? Position.Bottom : reversed ? Position.Left : Position.Right,
  }
}

function createSyntheticDefinitionNode(
  id: string,
  type: string,
  name: string,
  x: number,
  y: number
): WorkflowDefinitionNode {
  return {
    id,
    type,
    name,
    position: { x, y },
    config: {},
  }
}

function BranchDecisionBlock({ decision }: { decision: BranchDecision }) {
  const t = useI18n()
  return (
    <div className="space-y-2 rounded-md border bg-muted/20 p-3">
      <div className="flex items-center justify-between gap-2">
        <div className="text-xs font-medium">{wr(t, "branch.title")}</div>
        <Badge variant="outline">{decision.selectedBranchName || decision.selectedBranchId || "default"}</Badge>
      </div>
      <div className="space-y-1 text-xs text-muted-foreground">
        <div>{wr(t, "branch.targetNode", { value: decision.selectedTargetNodeId || "-" })}</div>
        <div>{wr(t, "branch.reason", { value: decision.reason || "-" })}</div>
      </div>
      {decision.evaluations?.length ? (
        <div className="space-y-2 pt-1">
          {decision.evaluations.map((item, index) => (
            <div key={`${item.edgeId || item.branchId || index}`} className="rounded-md border bg-background px-2 py-1.5 text-xs">
              <div className="flex items-center justify-between gap-2">
                <span className="font-medium">{item.branchName || item.branchId || item.edgeId || wr(t, "branch.conditionFallback", { index: index + 1 })}</span>
                <Badge variant={item.matched ? "default" : "secondary"}>{item.matched ? wr(t, "branch.matched") : wr(t, "branch.notMatched")}</Badge>
              </div>
              <div className="mt-1 break-all text-muted-foreground">
                {item.sourceNodeId}.{item.sourceField} {item.operator} {formatUnknown(item.rightValue)}
              </div>
              <div className="mt-1 break-all text-muted-foreground">
                {wr(t, "branch.actualValue", { value: formatUnknown(item.leftValue) })}
              </div>
            </div>
          ))}
        </div>
      ) : null}
    </div>
  )
}

function PreviewBlock({ title, raw, value }: { title: string; raw: string; value: unknown }) {
  return (
    <div className="min-w-0">
      <div className="mb-1 text-xs font-medium text-muted-foreground">{title}</div>
      {value !== null ? (
        <JsonTreeViewer value={value} collapsed={2} />
      ) : raw.trim() ? (
        <pre className="max-h-72 overflow-auto rounded-md border bg-muted/20 p-3 text-xs whitespace-pre-wrap break-all">
          {raw}
        </pre>
      ) : (
        <div className="rounded-md border bg-muted/20 px-3 py-2 text-xs text-muted-foreground">-</div>
      )}
    </div>
  )
}

function extractBranchDecision(value: unknown): BranchDecision | null {
  if (!value || typeof value !== "object") {
    return null
  }
  const record = value as Record<string, unknown>
  const decision = record.branchDecision
  if (!decision || typeof decision !== "object") {
    return null
  }
  return decision as BranchDecision
}

function safeParseJSON(raw: string): unknown | null {
  const trimmed = raw.trim()
  if (!trimmed) {
    return null
  }
  try {
    return JSON.parse(trimmed)
  } catch {
    return null
  }
}

function formatUnknown(value: unknown) {
  if (typeof value === "string") {
    return value
  }
  if (value === null || value === undefined) {
    return "-"
  }
  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}
