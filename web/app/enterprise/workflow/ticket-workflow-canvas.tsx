"use client"

import "@xyflow/react/dist/style.css"
import { useMemo, useState } from "react"
import {
  Background, BaseEdge, Controls, Handle, MarkerType, Position, ReactFlow, getSmoothStepPath,
  type Edge, type EdgeProps, type Node, type NodeProps, type ReactFlowInstance, type XYPosition,
} from "@xyflow/react"
import { ArrowDown, Check, CircleCheck, LockKeyhole, Maximize2, Minimize2, Network, RotateCcw, Wrench, X } from "lucide-react"
import { RailopsButton } from "@railops/ui"
import { useAppLocale } from "@/i18n/provider"
import { caseStatusLabel } from "@/lib/ticket-case-labels"
import { ticketWorkflowCopy, ticketWorkflowDestinationLabel, ticketWorkflowTransitionSettingsLabel, type TicketWorkflowDetail } from "./ticket-status-workflow-i18n"

type Workflow = { transitions: Record<string, string[]> }
type Props = {
  states: string[]; workflow: Workflow; allowed: Workflow; required: Record<string, string[]>
  editing: boolean; busy: boolean
  onChangeTransition: (from: string, to: string, enabled: boolean) => void
}
const mainPath = ["new", "acknowledged", "in_triage", "assigned", "restored", "resolved", "closure_pending", "closed"]
function initialPosition(state: string): XYPosition {
  if (state === "cancelled") return { x: 0, y: 180 }
  if (state === "waiting") return { x: 900, y: 540 }
  return { x: 420, y: Math.max(0, mainPath.indexOf(state)) * 180 }
}
type StatusNode = Node<{ status: string; count: number; muted: boolean }, "status">
function StatusWorkflowNode({ data, selected, isConnectable }: NodeProps<StatusNode>) {
  const { locale } = useAppLocale()
  const copy = ticketWorkflowCopy(locale)
  const detail = copy.details[data.status]
  const Icon = data.status === "cancelled" ? X : data.status === "waiting" ? RotateCcw
    : data.status === "closed" ? CircleCheck : data.status === "new" ? Network : Wrench
  return <div className={`w-[216px] rounded-lg border border-t-[3px] bg-card shadow-sm transition-[border-color,box-shadow,opacity] ${selected ? "border-blue-500 ring-2 ring-blue-500/15" : data.status === "waiting" ? "border-border border-t-amber-500" : data.status === "cancelled" || data.status === "closed" ? "border-border border-t-slate-400" : "border-border border-t-blue-500"} ${data.muted ? "opacity-40" : ""}`}>
    <Handle id="in" type="target" position={Position.Top} isConnectable={isConnectable} className="!size-2.5 !border-2 !border-background !bg-blue-500" />
    <Handle id="in-left" type="target" position={Position.Left} isConnectable={false} style={{ top: "30%" }} className="!size-1.5 !border-0 !bg-slate-400" />
    <Handle id="in-right" type="target" position={Position.Right} isConnectable={false} style={{ top: "30%" }} className="!size-1.5 !border-0 !bg-slate-400" />
    <div className="flex items-center gap-2 px-3 pt-2.5">
      <span className="rounded bg-muted p-1.5 text-blue-600"><Icon size={15} /></span>
      <strong className="flex-1 text-[13px]">{caseStatusLabel(data.status)}</strong>
      <span className="text-[10px] text-muted-foreground">{detail.group}</span>
    </div>
    <p className="px-3 pt-1.5 text-[11px] text-muted-foreground">{detail.action}</p>
    <div className="mt-2 flex justify-between border-t px-3 py-1.5 text-[10px] text-muted-foreground"><span>{detail.role}</span><span>{ticketWorkflowDestinationLabel(locale, data.count)}</span></div>
    {data.status !== "cancelled" && <Handle id="out" type="source" position={Position.Bottom} isConnectable={isConnectable} className="!size-2.5 !border-2 !border-background !bg-blue-500" />}
    <Handle id="out-left" type="source" position={Position.Left} isConnectable={false} style={{ top: "70%" }} className="!size-1.5 !border-0 !bg-slate-400" />
    <Handle id="out-right" type="source" position={Position.Right} isConnectable={false} style={{ top: "70%" }} className="!size-1.5 !border-0 !bg-slate-400" />
  </div>
}
const nodeTypes = { status: StatusWorkflowNode }
const edgeId = (from: string, to: string) => `${from}-${to}`
type RoutedEdge = Edge<{ laneX?: number; bridgeY?: number; secondLaneX?: number }>

// Keep long transitions in gutters beside the nodes instead of crossing cards.
function WorkflowEdge(props: EdgeProps<RoutedEdge>) {
  const { sourceX, sourceY, targetX, targetY, data } = props
  let path: string
  if (data?.laneX !== undefined) {
    const points = [[sourceX, sourceY], [data.laneX, sourceY]]
    if (data.bridgeY !== undefined && data.secondLaneX !== undefined) {
      points.push([data.laneX, data.bridgeY], [data.secondLaneX, data.bridgeY], [data.secondLaneX, targetY])
    } else points.push([data.laneX, targetY])
    points.push([targetX, targetY])
    path = points.map(([x, y], index) => `${index ? "L" : "M"} ${x} ${y}`).join(" ")
  } else {
    [path] = getSmoothStepPath({ ...props, borderRadius: 10 })
  }
  return <BaseEdge id={props.id} path={path} markerEnd={props.markerEnd} style={{ ...props.style, strokeLinejoin: "round" }} interactionWidth={16} />
}
const edgeTypes = { routed: WorkflowEdge }

export function TicketWorkflowCanvas({ states, workflow, allowed, required, editing, busy, onChangeTransition }: Props) {
  const { locale } = useAppLocale()
  const copy = ticketWorkflowCopy(locale)
  const details: Record<string, TicketWorkflowDetail> = copy.details
  const [selected, setSelected] = useState("")
  const [selectedEdge, setSelectedEdge] = useState<{ from: string; to: string } | null>(null)
  const [positions, setPositions] = useState<Record<string, XYPosition>>({})
  const [expanded, setExpanded] = useState(false)
  const [instance, setInstance] = useState<ReactFlowInstance<StatusNode, Edge> | null>(null)
  const canEdit = editing && !busy
  const targets = selected ? workflow.transitions[selected] ?? [] : []
  const position = (state: string) => positions[state] ?? initialPosition(state)
  const nodes: StatusNode[] = states.map(state => ({
    id: state, type: "status", position: position(state), selected: selected === state,
    ariaLabel: ticketWorkflowTransitionSettingsLabel(locale, caseStatusLabel(state)),
    data: { status: state, count: workflow.transitions[state]?.length ?? 0, muted: Boolean(selected && selected !== state && !targets.includes(state) && !workflow.transitions[state]?.includes(selected)) },
  }))
  const allEdges = useMemo(() => Object.entries(workflow.transitions).flatMap(([from, next]) => next.map(to => ({ from, to }))), [workflow])
  const mainPairs = new Set(mainPath.slice(0, -1).map((from, index) => edgeId(from, mainPath[index + 1])))
  const laneCounts: Record<string, number> = {}
  const edges: RoutedEdge[] = allEdges.map(({ from, to }) => {
    const start = position(from), end = position(to)
    const main = mainPairs.has(edgeId(from, to))
    const backwards = !main && end.y <= start.y
    const related = !selected || from === selected || to === selected
    const color = from === selected ? "#2563eb" : to === selected ? "#0891b2" : to === "cancelled" ? "#94a3b8" : from === "waiting" || to === "waiting" ? "#b7791f" : backwards ? "#8b7aa6" : main ? "#475569" : "#8294ad"
    const group = to === "cancelled" ? "cancel" : from === "waiting" || to === "waiting" ? "waiting" : backwards ? "return" : "forward"
    const lane = laneCounts[group] ?? 0
    if (!main) laneCounts[group] = lane + 1
    let sourceHandle = "out", targetHandle = "in"
    let data: RoutedEdge["data"]
    if (!main) {
      if (to === "cancelled") {
        sourceHandle = "out-left"; targetHandle = "in-right"
        const laneX = end.x + 248 + lane * 16
        data = from === "waiting" ? { laneX: start.x - 28, bridgeY: start.y - 30, secondLaneX: laneX } : { laneX }
      } else if (from === "waiting" || to === "waiting") {
        sourceHandle = from === "waiting" ? "out-left" : "out-right"
        targetHandle = to === "waiting" ? "in-left" : "in-right"
        data = { laneX: Math.min(start.x, end.x) + 348 + lane * 10 }
      } else if (backwards) {
        sourceHandle = "out-left"; targetHandle = "in-left"
        data = { laneX: Math.min(start.x, end.x) - 28 - lane * 16 }
      } else {
        sourceHandle = "out-right"; targetHandle = "in-right"
        data = { laneX: Math.max(start.x, end.x) + 248 + lane * 16 }
      }
    }
    return {
      id: edgeId(from, to), source: from, target: to, sourceHandle, targetHandle, type: "routed", data,
      zIndex: selectedEdge?.from === from && selectedEdge.to === to ? 3 : selected && related ? 2 : 0,
      markerEnd: { type: MarkerType.ArrowClosed, color },
      style: { stroke: color, opacity: related ? 1 : 0.18, strokeWidth: selectedEdge?.from === from && selectedEdge.to === to ? 3 : selected && related ? 2 : main ? 1.8 : 1.2, strokeDasharray: to === "cancelled" || backwards ? "5 4" : undefined },
      ariaLabel: `${caseStatusLabel(from)} → ${caseStatusLabel(to)}`,
    }
  })
  function select(state: string) { setSelected(state); setSelectedEdge(null) }
  const total = allEdges.length

  return <div className="overflow-hidden rounded-lg border bg-background" data-testid="ticket-status-workflow-graph">
    <div className="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-2.5">
      <div className="flex items-center gap-2 text-sm"><Network size={16} className="text-blue-600" /><span className="font-medium">{selected ? `${caseStatusLabel(selected)} · ${copy.relatedTransitions}` : copy.graphTitle}</span><span className="text-xs text-muted-foreground">{copy.statusCount(total)}</span></div>
      <div className="flex gap-1">
        {selected && <RailopsButton size="small" onClick={() => select("")}>{copy.backOverview}</RailopsButton>}
        <RailopsButton size="small" aria-label={copy.autoLayout} onClick={() => { setPositions({}); requestAnimationFrame(() => { requestAnimationFrame(() => { void instance?.fitView({ padding: 0.12, maxZoom: 1 }) }) }) }}><RotateCcw size={14} />{copy.autoLayout}</RailopsButton>
        <RailopsButton size="small" aria-label={expanded ? copy.collapseCanvas : copy.expandCanvas} onClick={() => setExpanded(!expanded)}>{expanded ? <Minimize2 size={14} /> : <Maximize2 size={14} />}</RailopsButton>
      </div>
    </div>
    <div className="grid lg:grid-cols-[minmax(0,1fr)_272px]">
      <div className={`relative min-w-0 bg-slate-50/70 dark:bg-muted/20 ${expanded ? "h-[1100px]" : "h-[900px]"}`}>
        <ReactFlow<StatusNode, Edge>
          nodes={nodes} edges={edges} nodeTypes={nodeTypes} edgeTypes={edgeTypes} onInit={setInstance}
          fitView fitViewOptions={{ padding: 0.12, maxZoom: 0.9 }} minZoom={0.3} maxZoom={1.5}
          nodesDraggable={canEdit} nodesConnectable={canEdit} elementsSelectable={!busy}
          deleteKeyCode={null} edgesReconnectable={false} connectionRadius={28}
          onNodesChange={changes => {
            if (!canEdit) return
            const moved = changes.filter(change => change.type === "position" && change.position)
            if (moved.length) setPositions(current => {
              const next = { ...current }
              for (const change of moved) if (change.type === "position" && change.position) next[change.id] = change.position
              return next
            })
          }}
          onNodeClick={(_, node) => select(node.id)} onPaneClick={() => select("")}
          onNodeDragStop={(_, node) => setPositions(current => ({ ...current, [node.id]: node.position }))}
          onEdgeClick={(_, edge) => { setSelected(edge.source); setSelectedEdge({ from: edge.source, to: edge.target }) }}
          isValidConnection={connection => canEdit && Boolean(connection.source && connection.target && allowed.transitions[connection.source]?.includes(connection.target) && !workflow.transitions[connection.source]?.includes(connection.target))}
          onConnect={connection => {
            if (!canEdit || !connection.source || !connection.target || !allowed.transitions[connection.source]?.includes(connection.target) || workflow.transitions[connection.source]?.includes(connection.target)) return
            onChangeTransition(connection.source, connection.target, true)
            setSelected(connection.source); setSelectedEdge({ from: connection.source, to: connection.target })
          }}
          aria-label={copy.graphAria}
        >
          <Background gap={22} size={1} color="var(--border)" />
          <Controls showInteractive={false} />
        </ReactFlow>
        <div className="pointer-events-none absolute left-3 top-3 max-w-[260px] rounded border bg-card/95 px-3 py-2 text-[11px] text-muted-foreground">
          {selected ? copy.graphHintSelected : copy.graphHintOverview}
        </div>
      </div>
      <aside className="border-t bg-card lg:border-l lg:border-t-0" aria-label={copy.transitionSettingsAria}>
        <div className="border-b p-4">
          <label className="text-xs font-medium" htmlFor="workflow-selected-state">{copy.viewStatus}</label>
          <select id="workflow-selected-state" className="mt-2 w-full rounded-md border bg-background px-2 py-2 text-sm" value={selected} onChange={event => select(event.target.value)}>
            <option value="">{copy.overview}</option>
            {states.map(state => <option key={state} value={state}>{caseStatusLabel(state)}</option>)}
          </select>
        </div>
        {selected ? <div className="space-y-4 p-4">
          <div><h3 className="text-sm font-semibold">{caseStatusLabel(selected)}</h3><p className="mt-2 text-xs leading-5 text-muted-foreground">{details[selected].hint}</p></div>
          {selectedEdge && <div className="space-y-2 rounded-md border border-blue-200 bg-blue-50/30 p-3 text-xs dark:bg-blue-950/20" data-testid="selected-workflow-edge">
            <div>{caseStatusLabel(selectedEdge.from)} → {caseStatusLabel(selectedEdge.to)}</div>
            {required[selectedEdge.from]?.includes(selectedEdge.to) ? <p className="flex items-center gap-1 text-muted-foreground"><LockKeyhole size={12} />{copy.requiredTransition}</p>
              : canEdit ? <RailopsButton size="small" onClick={() => { onChangeTransition(selectedEdge.from, selectedEdge.to, false); setSelectedEdge(null) }}>{copy.removeTransition}</RailopsButton> : <p className="text-muted-foreground">{editing ? copy.saving : copy.adjustHint}</p>}
          </div>}
          <div>
            <h4 className="flex items-center gap-1.5 text-xs font-semibold"><ArrowDown size={13} />{copy.nextStep}</h4>
            <p className="mt-1 text-[11px] leading-5 text-muted-foreground">{editing ? copy.editTransitionHint : copy.currentTransitionHint}</p>
            <div className="mt-2 space-y-1">
              {(allowed.transitions[selected] ?? []).map(to => <label key={to} className="flex items-center gap-2 rounded-md px-2 py-2 text-xs hover:bg-muted/50">
                <input type="checkbox" className="size-3.5 accent-blue-600" aria-label={`${caseStatusLabel(selected)} → ${caseStatusLabel(to)}`} checked={targets.includes(to)} disabled={!canEdit || required[selected]?.includes(to)} onChange={event => { onChangeTransition(selected, to, event.target.checked); setSelectedEdge(null) }} />
                <span className="flex-1">{caseStatusLabel(to)}</span>{required[selected]?.includes(to) && <span className="flex items-center gap-1 text-[10px] text-muted-foreground"><LockKeyhole size={10} />{copy.required}</span>}
              </label>)}
              {!allowed.transitions[selected]?.length && <p className="rounded-md bg-muted/40 p-3 text-xs text-muted-foreground">{copy.noNextStep}</p>}
            </div>
          </div>
        </div> : <div className="space-y-5 p-4 text-xs leading-5">
          <div><h3 className="font-semibold">{copy.setupTitle}</h3><p className="mt-2 text-muted-foreground">{copy.setupDescription}</p></div>
          <div className="space-y-3 text-muted-foreground"><p>{copy.setupStep1}</p><p>{copy.setupStep2}</p><p>{copy.setupStep3}</p></div>
          <p className="flex items-start gap-2 rounded-md bg-muted/40 p-3 text-muted-foreground"><Check size={15} className="mt-0.5 shrink-0" />{copy.prepared}</p>
        </div>}
        <div className="border-t px-4 py-3 text-[11px] leading-5 text-muted-foreground">{copy.footer}</div>
      </aside>
    </div>
  </div>
}
