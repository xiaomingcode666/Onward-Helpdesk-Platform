"use client"

import "@xyflow/react/dist/style.css"

import {
  addEdge,
  Background,
  BaseEdge,
  ConnectionMode,
  Controls,
  EdgeLabelRenderer,
  getBezierPath,
  Handle,
  MarkerType,
  Position,
  ReactFlow,
  ViewportPortal,
  useEdgesState,
  useNodesState,
  type Connection,
  type ConnectionLineComponentProps,
  type Edge,
  type EdgeChange,
  type EdgeProps,
  type FinalConnectionState,
  type Node,
  type NodeChange,
  type OnNodeDrag,
  type NodeProps,
  type ReactFlowInstance,
} from "@xyflow/react"
import {
  AlertCircleIcon,
  CheckCircle2Icon,
  CircleStopIcon,
  DatabaseIcon,
  GitBranchIcon,
  MessageSquareTextIcon,
  PanelLeftCloseIcon,
  PanelLeftOpenIcon,
  PlayIcon,
  PlusIcon,
  Redo2Icon,
  RotateCcwIcon,
  SaveIcon,
  ScanLineIcon,
  SendIcon,
  ShieldCheckIcon,
  Undo2Icon,
  WrenchIcon,
  WorkflowIcon,
} from "lucide-react"
import { useCallback, useEffect, useMemo, useRef, useState } from "react"

import { useI18n } from "@/i18n/provider"
import { translateCurrentMessage } from "@/i18n/messages"
import { Button } from "@/components/ui/button"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import { cn } from "@/lib/utils"
import type {
  AIWorkflowDefinition,
  AIWorkflowNodeSpec,
  AIWorkflowTestRunResult,
} from "@/lib/api/admin"
import {
  applyAutoInputMappings,
  calculateWorkflowHelperLines,
  createWorkflowHistory,
  createWorkflowNodeFromSpec,
  fromApiDefinition,
  formatWorkflowVariableName,
  getAvailableVariables,
  getNodeSpec,
  getRequiredInputs,
  pushWorkflowHistory,
  redoWorkflowHistory,
  toApiDefinition,
  undoWorkflowHistory,
  validateWorkflowDraft,
  type WorkflowCondition,
  type WorkflowEditorNode,
  type WorkflowHistory,
  type WorkflowHelperLine,
  type WorkflowNodeConfig,
} from "./workflow-utils"
import { NodeConfigPanel, type WorkflowBranchSummary } from "./node-config-panel"
import type { WorkflowBranchTargetOption } from "./node-config-panel"

type WorkflowNodeData = Record<string, unknown> & {
  nodeType?: string
  name?: string
  config?: WorkflowNodeConfig
  inputs?: Record<string, { nodeId: string; field: string }>
  errorTargetNodeId?: string
  nodeSpecs?: AIWorkflowNodeSpec[]
  onAddAfter?: (sourceNodeId: string, spec: AIWorkflowNodeSpec) => void
  label?: string
  title?: string
  description?: string
  inputCount?: number
  outputCount?: number
  missingInputs?: string[]
  readOnly?: boolean
  runStatus?: string
  runDurationMs?: number
  runError?: string
  testRunAvailable?: boolean
}

type WorkflowFlowNode = Node<WorkflowNodeData>
type WorkflowFlowEdge = Edge
type WorkflowEditorSnapshot = {
  nodes: WorkflowFlowNode[]
  edges: WorkflowFlowEdge[]
}
type WorkflowEdgeRenderData = {
  active?: boolean
  runStatus?: string
  branchLabel?: string
  errorFallback?: boolean
}
type WorkflowFinalConnectionState = FinalConnectionState

type PendingNodeDrag = {
  spec: AIWorkflowNodeSpec
  startX: number
  startY: number
  x: number
  y: number
  active: boolean
}

const aiWorkflowsEditorI18nPrefix = "dashboardExtract.aiWorkflowsEditor."
type AiWorkflowsEditorT = ReturnType<typeof useI18n>
const we = (t: AiWorkflowsEditorT, key: string, values?: Record<string, string | number>) => t(`${aiWorkflowsEditorI18nPrefix}${key}`, values)
const weM = (key: string, values?: Record<string, string | number>) => translateCurrentMessage(`${aiWorkflowsEditorI18nPrefix}${key}`, values)

const nodeTypes = {
  workflowNode: WorkflowCanvasNode,
}

const edgeTypes = {
  workflowEdge: WorkflowCanvasEdge,
}

const fitViewOptions = {
  padding: 0.16,
  minZoom: 0.16,
  maxZoom: 1,
}

const defaultEdgeOptions = {
  type: "workflowEdge",
  markerEnd: {
    type: MarkerType.ArrowClosed,
  },
  style: {
    strokeWidth: 1.6,
  },
}

const workflowHandleRadius = 8

function toFlowNodes(definition: AIWorkflowDefinition): WorkflowFlowNode[] {
  return fromApiDefinition(definition).nodes.map((node) => ({
    id: node.id,
    type: "workflowNode",
    position: node.position,
    data: {
      nodeType: node.data?.nodeType ?? node.type,
      name: node.data?.name ?? node.id,
      label: node.data?.name ?? node.type ?? node.id,
      config: node.data?.config ?? {},
      inputs: node.data?.inputs ?? {},
      errorTargetNodeId: node.data?.errorTargetNodeId,
    },
  }))
}

function toFlowEdges(definition: AIWorkflowDefinition): WorkflowFlowEdge[] {
  return (definition.edges ?? []).map((edge) => ({
    id: edge.id,
    type: "workflowEdge",
    source: edge.source,
    target: edge.target,
  }))
}

function toDraft(nodes: WorkflowFlowNode[], edges: WorkflowFlowEdge[]) {
  return {
    nodes: nodes.map((node) => ({
      id: node.id,
      type: node.type,
      position: node.position,
      data: {
        nodeType: node.data.nodeType,
        name: node.data.name,
        config: node.data.config,
        inputs: node.data.inputs,
        errorTargetNodeId: node.data.errorTargetNodeId,
      },
    })) as WorkflowEditorNode[],
    edges: edges.map((edge) => ({
      id: edge.id,
      source: edge.source,
      target: edge.target,
    })),
  }
}

export function WorkflowEditor({
  definition,
  nodeSpecs,
  onDefinitionChange,
  onRestoreDefault,
  restoreDefaultDisabled = false,
  onValidate,
  validateDisabled = false,
  onSaveDraft,
  saveDraftDisabled = false,
  onPublish,
  publishDisabled = false,
  onTestRun,
  testRunDisabled = false,
  testRunActive = false,
  testRun,
  readOnly = false,
}: {
  definition: AIWorkflowDefinition
  nodeSpecs: AIWorkflowNodeSpec[]
  onDefinitionChange: (definition: AIWorkflowDefinition) => void
  onRestoreDefault?: () => void
  restoreDefaultDisabled?: boolean
  onValidate?: () => void
  validateDisabled?: boolean
  onSaveDraft?: () => void
  saveDraftDisabled?: boolean
  onPublish?: () => void
  publishDisabled?: boolean
  onTestRun?: () => void
  testRunDisabled?: boolean
  testRunActive?: boolean
  testRun?: AIWorkflowTestRunResult | null
  readOnly?: boolean
}) {
  const t = useI18n()
  const [nodes, setNodes, onNodesChange] = useNodesState<WorkflowFlowNode>(
    toFlowNodes(definition)
  )
  const [edges, setEdges, onEdgesChange] = useEdgesState<WorkflowFlowEdge>(
    toFlowEdges(definition)
  )
  const [flowInstance, setFlowInstance] = useState<ReactFlowInstance<WorkflowFlowNode, WorkflowFlowEdge> | null>(null)
  const [nodeLibraryCollapsed, setNodeLibraryCollapsed] = useState(readOnly)
  const [nodeLibraryRendered, setNodeLibraryRendered] = useState(!readOnly)
  const [nodeLibraryVisible, setNodeLibraryVisible] = useState(!readOnly)
  const [nodeLibraryWidth, setNodeLibraryWidth] = useState(260)
  const [nodeLibraryResizing, setNodeLibraryResizing] = useState(false)
  const [pendingNodeDrag, setPendingNodeDrag] = useState<PendingNodeDrag | null>(null)
  const [helperLines, setHelperLines] = useState<WorkflowHelperLine>({})
  const [propertyPanelNode, setPropertyPanelNode] = useState<WorkflowFlowNode | null>(null)
  const [selectedEdgeId, setSelectedEdgeId] = useState<string | null>(null)
  const [propertyPanelVisible, setPropertyPanelVisible] = useState(false)
  const editorRef = useRef<HTMLDivElement | null>(null)
  const canvasRef = useRef<HTMLElement | null>(null)
  const pendingNodeDragRef = useRef<PendingNodeDrag | null>(null)
  const historyRef = useRef<WorkflowHistory<WorkflowEditorSnapshot>>(createWorkflowHistory())
  const dragStartSnapshotRef = useRef<WorkflowEditorSnapshot | null>(null)
  const suppressNextClickRef = useRef(false)
  const nodeLibraryAnimationTimerRef = useRef<number | null>(null)
  const propertyPanelAnimationTimerRef = useRef<number | null>(null)
  const onDefinitionChangeRef = useRef(onDefinitionChange)
  const draft = useMemo(() => toDraft(nodes, edges), [nodes, edges])
  const focusEntryInsteadOfShrinking = nodes.length > 12
  const entryViewport = useMemo(() => {
    const entryNode = nodes.find((node) => node.id === definition.entryNodeId) ?? nodes[0]
    const zoom = 0.72
    return {
      x: 96 - (entryNode?.position.x ?? 0) * zoom,
      y: 96 - (entryNode?.position.y ?? 0) * zoom,
      zoom,
    }
  }, [definition.entryNodeId, nodes])
  const [historyAvailability, setHistoryAvailability] = useState({
    canUndo: false,
    canRedo: false,
  })
  const validation = useMemo(
    () => validateWorkflowDraft(draft, nodeSpecs),
    [draft, nodeSpecs]
  )
  const propertyPanelNodeSpec = useMemo(
    () => getNodeSpec(nodeSpecs, propertyPanelNode?.data.nodeType ?? ""),
    [nodeSpecs, propertyPanelNode]
  )
  const propertyPanelAvailableVariables = useMemo(
    () => (propertyPanelNode ? getAvailableVariables(draft, propertyPanelNode.id, nodeSpecs) : []),
    [draft, nodeSpecs, propertyPanelNode]
  )
  const propertyPanelBranchSummaries = useMemo(
    () => (propertyPanelNode ? getBranchSummaries(nodes, propertyPanelNode.id) : []),
    [nodes, propertyPanelNode]
  )
  const propertyPanelBranchTargetOptions = useMemo(
    () => (propertyPanelNode ? getBranchTargetOptions(nodes, edges, propertyPanelNode.id) : []),
    [edges, nodes, propertyPanelNode]
  )
  useEffect(() => {
    onDefinitionChangeRef.current = onDefinitionChange
  }, [onDefinitionChange])

  useEffect(() => {
    onDefinitionChangeRef.current(toApiDefinition(draft) as AIWorkflowDefinition)
  }, [draft])

  useEffect(() => {
    return () => {
      if (nodeLibraryAnimationTimerRef.current !== null) {
        window.clearTimeout(nodeLibraryAnimationTimerRef.current)
      }
      if (propertyPanelAnimationTimerRef.current !== null) {
        window.clearTimeout(propertyPanelAnimationTimerRef.current)
      }
    }
  }, [])

  const showNodeLibrary = useCallback(() => {
    if (nodeLibraryAnimationTimerRef.current !== null) {
      window.clearTimeout(nodeLibraryAnimationTimerRef.current)
    }
    setNodeLibraryCollapsed(false)
    setNodeLibraryRendered(true)
    nodeLibraryAnimationTimerRef.current = window.setTimeout(() => {
      setNodeLibraryVisible(true)
      nodeLibraryAnimationTimerRef.current = null
    }, 0)
  }, [])

  const hideNodeLibrary = useCallback(() => {
    if (nodeLibraryAnimationTimerRef.current !== null) {
      window.clearTimeout(nodeLibraryAnimationTimerRef.current)
    }
    setNodeLibraryCollapsed(true)
    setNodeLibraryVisible(false)
    nodeLibraryAnimationTimerRef.current = window.setTimeout(() => {
      setNodeLibraryRendered(false)
      nodeLibraryAnimationTimerRef.current = null
    }, 220)
  }, [])

  const showPropertyPanelNode = useCallback((node: WorkflowFlowNode) => {
    if (propertyPanelAnimationTimerRef.current !== null) {
      window.clearTimeout(propertyPanelAnimationTimerRef.current)
    }
    setPropertyPanelNode(node)
    propertyPanelAnimationTimerRef.current = window.setTimeout(() => {
      setPropertyPanelVisible(true)
      propertyPanelAnimationTimerRef.current = null
    }, 0)
  }, [])

  const hidePropertyPanel = useCallback(() => {
    if (propertyPanelAnimationTimerRef.current !== null) {
      window.clearTimeout(propertyPanelAnimationTimerRef.current)
    }
    setPropertyPanelVisible(false)
    propertyPanelAnimationTimerRef.current = window.setTimeout(() => {
      setPropertyPanelNode(null)
      propertyPanelAnimationTimerRef.current = null
    }, 220)
  }, [])

  const syncHistoryAvailability = useCallback(() => {
    setHistoryAvailability({
      canUndo: historyRef.current.past.length > 0,
      canRedo: historyRef.current.future.length > 0,
    })
  }, [])

  const getCurrentSnapshot = useCallback((): WorkflowEditorSnapshot => ({
    nodes,
    edges,
  }), [edges, nodes])

  const pushSnapshotToHistory = useCallback(
    (snapshot: WorkflowEditorSnapshot) => {
      historyRef.current = pushWorkflowHistory(historyRef.current, snapshot)
      syncHistoryAvailability()
    },
    [syncHistoryAvailability]
  )

  const pushCurrentSnapshotToHistory = useCallback(() => {
    pushSnapshotToHistory(getCurrentSnapshot())
  }, [getCurrentSnapshot, pushSnapshotToHistory])

  const applySnapshot = useCallback(
    (snapshot: WorkflowEditorSnapshot) => {
      setNodes(snapshot.nodes)
      setEdges(snapshot.edges)
      setHelperLines({})
      setSelectedEdgeId((current) =>
        current && snapshot.edges.some((edge) => edge.id === current) ? current : null
      )
      setPropertyPanelNode((current) =>
        current ? snapshot.nodes.find((node) => node.id === current.id) ?? null : null
      )
    },
    [setEdges, setNodes]
  )

  const undoWorkflowEdit = useCallback(() => {
    const result = undoWorkflowHistory(historyRef.current, getCurrentSnapshot())
    if (!result) {
      return
    }
    historyRef.current = result.history
    applySnapshot(result.snapshot)
    syncHistoryAvailability()
  }, [applySnapshot, getCurrentSnapshot, syncHistoryAvailability])

  const redoWorkflowEdit = useCallback(() => {
    const result = redoWorkflowHistory(historyRef.current, getCurrentSnapshot())
    if (!result) {
      return
    }
    historyRef.current = result.history
    applySnapshot(result.snapshot)
    syncHistoryAvailability()
  }, [applySnapshot, getCurrentSnapshot, syncHistoryAvailability])

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (!event.metaKey && !event.ctrlKey) {
        return
      }
      if (isEditableKeyboardTarget(event.target)) {
        return
      }
      const key = event.key.toLowerCase()
      if (key === "z" && event.shiftKey) {
        event.preventDefault()
        redoWorkflowEdit()
        return
      }
      if (key === "y") {
        event.preventDefault()
        redoWorkflowEdit()
        return
      }
      if (key === "z") {
        event.preventDefault()
        undoWorkflowEdit()
      }
    }

    window.addEventListener("keydown", handleKeyDown)
    return () => window.removeEventListener("keydown", handleKeyDown)
  }, [redoWorkflowEdit, undoWorkflowEdit])

  const onConnect = useCallback(
    (connection: Connection) => {
      if (!connection.source || !connection.target) {
        return
      }
      pushCurrentSnapshotToHistory()
      const edge = {
        ...connection,
        id: uniqueEdgeId(edges, connection.source, connection.target),
        type: "workflowEdge",
      } as WorkflowFlowEdge
      setEdges((current) => addEdge(edge, current))
      setNodes((currentNodes) => {
        const currentDraft = toDraft(currentNodes, [...edges, edge])
        const nextDraft = applyAutoInputMappings(
          currentDraft,
          connection.source!,
          connection.target!,
          nodeSpecs
        )
        return currentNodes.map((node) => {
          const nextNode = nextDraft.nodes.find((item) => item.id === node.id)
          if (!nextNode) {
            return node
          }
          return {
            ...node,
            data: {
              ...node.data,
              inputs: nextNode.data?.inputs ?? node.data.inputs,
            },
          }
        })
      })
    },
    [edges, nodeSpecs, pushCurrentSnapshotToHistory, setEdges, setNodes]
  )

  const connectToNode = useCallback(
    (connectionState: WorkflowFinalConnectionState, targetNodeId: string) => {
      if (!connectionState.fromHandle || connectionState.toHandle || connectionState.fromHandle.nodeId === targetNodeId) {
        return
      }

      const fromHandle = connectionState.fromHandle
      const source = fromHandle.type === "target" ? targetNodeId : fromHandle.nodeId
      const target = fromHandle.type === "target" ? fromHandle.nodeId : targetNodeId
      const connection = {
        source,
        target,
        sourceHandle: fromHandle.type === "target" ? null : fromHandle.id ?? null,
        targetHandle: fromHandle.type === "target" ? fromHandle.id ?? null : null,
      } satisfies Connection
      onConnect(connection)
    },
    [onConnect]
  )

  const onConnectEnd = useCallback(
    (event: MouseEvent | TouchEvent, connectionState: WorkflowFinalConnectionState) => {
      if (connectionState.toHandle) {
        return
      }
      const point = getEventClientPoint(event)
      if (!point) {
        return
      }
      const nodeElement = document
        .elementFromPoint(point.x, point.y)
        ?.closest<HTMLElement>(".react-flow__node[data-id]")
      const targetNodeId = nodeElement?.dataset.id
      if (!targetNodeId) {
        return
      }
      connectToNode(connectionState, targetNodeId)
    },
    [connectToNode]
  )

  const onWorkflowNodesChange = useCallback(
    (changes: NodeChange<WorkflowFlowNode>[]) => {
      if (changes.some((change) => change.type === "remove")) {
        pushCurrentSnapshotToHistory()
      }
      onNodesChange(changes)
    },
    [onNodesChange, pushCurrentSnapshotToHistory]
  )

  const onWorkflowEdgesChange = useCallback(
    (changes: EdgeChange<WorkflowFlowEdge>[]) => {
      if (changes.some((change) => change.type === "remove")) {
        pushCurrentSnapshotToHistory()
      }
      onEdgesChange(changes)
    },
    [onEdgesChange, pushCurrentSnapshotToHistory]
  )

  const onNodeDragStart = useCallback<OnNodeDrag<WorkflowFlowNode>>(() => {
    dragStartSnapshotRef.current = getCurrentSnapshot()
  }, [getCurrentSnapshot])

  const onNodeDrag = useCallback<OnNodeDrag<WorkflowFlowNode>>(
    (_event, node) => {
      const nextHelperLines = calculateWorkflowHelperLines(nodes, node)
      setHelperLines({
        horizontal: nextHelperLines.horizontal,
        vertical: nextHelperLines.vertical,
      })
      if (
        nextHelperLines.position.x === node.position.x &&
        nextHelperLines.position.y === node.position.y
      ) {
        return
      }
      setNodes((current) =>
        current.map((item) =>
          item.id === node.id
            ? {
                ...item,
                position: nextHelperLines.position,
              }
            : item
        )
      )
    },
    [nodes, setNodes]
  )

  const onNodeDragStop = useCallback<OnNodeDrag<WorkflowFlowNode>>((_event, node) => {
    setHelperLines({})
    const startSnapshot = dragStartSnapshotRef.current
    dragStartSnapshotRef.current = null
    const startNode = startSnapshot?.nodes.find((item) => item.id === node.id)
    if (
      startSnapshot &&
      startNode &&
      (startNode.position.x !== node.position.x || startNode.position.y !== node.position.y)
    ) {
      pushSnapshotToHistory(startSnapshot)
    }
  }, [pushSnapshotToHistory])

  const addNode = (spec: AIWorkflowNodeSpec) => {
    pushCurrentSnapshotToHistory()
    setNodes((current) => {
      const node = createWorkflowNodeFromSpec(
        spec,
        current,
        { x: 120 + current.length * 28, y: 100 + current.length * 24 }
      ) as WorkflowFlowNode
      return [
        ...current,
        {
          ...node,
          data: {
            ...node.data,
          },
        },
      ]
    })
  }

  const addNodeAfter = useCallback(
    (sourceNodeId: string, spec: AIWorkflowNodeSpec) => {
      pushCurrentSnapshotToHistory()
      setNodes((current) => {
        const sourceNode = current.find((node) => node.id === sourceNodeId)
        const nextPosition = sourceNode
          ? { x: sourceNode.position.x + 280, y: sourceNode.position.y }
          : { x: 160 + current.length * 32, y: 120 + current.length * 24 }
        const nextNode = createWorkflowNodeFromSpec(
          spec,
          current,
          nextPosition
        ) as WorkflowFlowNode

        setEdges((currentEdges) => [
          ...currentEdges,
          {
            id: uniqueEdgeId(currentEdges, sourceNodeId, nextNode.id),
            source: sourceNodeId,
            target: nextNode.id,
            type: "workflowEdge",
          },
        ])

        return [...current, nextNode]
      })
    },
    [pushCurrentSnapshotToHistory, setEdges, setNodes]
  )

  const renderedNodes = useMemo(
    () => {
      const testNodeByID = new Map((testRun?.nodes ?? []).map((item) => [item.nodeId, item]))
      return enrichNodesForRender(nodes, nodeSpecs).map((node) => {
        const testNode = testNodeByID.get(node.id)
        return {
        ...node,
        data: {
          ...node.data,
          nodeSpecs,
          onAddAfter: readOnly ? undefined : addNodeAfter,
          readOnly,
          runStatus: testNode?.status,
          runDurationMs: testNode?.durationMs,
          runError: testNode?.errorMessage,
          testRunAvailable: Boolean(testRun),
        },
      }
      })
    },
    [addNodeAfter, nodes, nodeSpecs, readOnly, testRun]
  )

  const dropNodeOnCanvas = useCallback(
    (spec: AIWorkflowNodeSpec, x: number, y: number) => {
      if (!flowInstance || !canvasRef.current) {
        return false
      }
      const rect = canvasRef.current.getBoundingClientRect()
      if (x < rect.left || x > rect.right || y < rect.top || y > rect.bottom) {
        return false
      }
      pushCurrentSnapshotToHistory()
      const position = flowInstance.screenToFlowPosition({ x, y })
      setNodes((current) => [
        ...current,
        createWorkflowNodeFromSpec(spec, current, position) as WorkflowFlowNode,
      ])
      return true
    },
    [flowInstance, pushCurrentSnapshotToHistory, setNodes]
  )

  const onNodePointerDown = (event: React.PointerEvent<HTMLButtonElement>, spec: AIWorkflowNodeSpec) => {
    if (event.button !== 0) {
      return
    }
    const initialDrag = {
      spec,
      startX: event.clientX,
      startY: event.clientY,
      x: event.clientX,
      y: event.clientY,
      active: false,
    }
    pendingNodeDragRef.current = initialDrag
    setPendingNodeDrag(initialDrag)

    const handlePointerMove = (event: PointerEvent) => {
      const current = pendingNodeDragRef.current
      if (!current) {
        return
      }
      const moved = Math.hypot(event.clientX - current.startX, event.clientY - current.startY)
      const nextDrag = {
        ...current,
        x: event.clientX,
        y: event.clientY,
        active: current.active || moved > 6,
      }
      pendingNodeDragRef.current = nextDrag
      setPendingNodeDrag(nextDrag)
    }

    const handlePointerUp = (event: PointerEvent) => {
      window.removeEventListener("pointermove", handlePointerMove)
      window.removeEventListener("pointerup", handlePointerUp)
      const current = pendingNodeDragRef.current
      pendingNodeDragRef.current = null
      setPendingNodeDrag(null)
      if (current?.active) {
        suppressNextClickRef.current = true
        dropNodeOnCanvas(current.spec, event.clientX, event.clientY)
      }
    }

    window.addEventListener("pointermove", handlePointerMove)
    window.addEventListener("pointerup", handlePointerUp)
  }

  const updateNodeData = (nodeId: string, data: WorkflowNodeData) => {
    pushCurrentSnapshotToHistory()
    const nextData = {
      ...data,
      label: data.name ?? data.nodeType ?? nodeId,
    }
    setNodes((current) =>
      current.map((node) =>
        node.id === nodeId
          ? {
              ...node,
              data: nextData,
            }
          : node
      )
    )
    setPropertyPanelNode((current) =>
      current?.id === nodeId
        ? {
            ...current,
            data: nextData,
          }
        : current
    )
  }

  const renderedEdges = useMemo(
    () => {
      const path = testRun?.nodePath ?? []
      const traversed = new Set(path.slice(0, -1).map((source, index) => `${source}->${path[index + 1]}`))
      const statusByNodeID = new Map((testRun?.nodes ?? []).map((item) => [item.nodeId, item.status]))
      return edges.map((edge) => {
        const active = edge.id === selectedEdgeId
        const wasTraversed = traversed.has(`${edge.source}->${edge.target}`)
        const runStatus = wasTraversed ? statusByNodeID.get(edge.target) ?? "completed" : undefined
        const sourceNode = nodes.find((node) => node.id === edge.source)
        const branchLabels = (sourceNode?.data.config?.branches ?? [])
          .filter((branch) => branch.targetNodeId === edge.target)
          .map((branch) => branch.name || branch.id)
          .filter(Boolean)
        if (sourceNode?.data.errorTargetNodeId === edge.target) {
          branchLabels.push(we(t, "edge.failedBranch"))
        }
        return {
          ...edge,
          selected: active,
          data: {
            active: active || wasTraversed,
            runStatus,
            branchLabel: branchLabels.join(" / "),
            errorFallback: sourceNode?.data.errorTargetNodeId === edge.target,
          } satisfies WorkflowEdgeRenderData,
        }
      })
    },
    [edges, nodes, selectedEdgeId, testRun]
  )

  const clampNodeLibraryWidth = useCallback((width: number) => {
    const containerWidth = editorRef.current?.getBoundingClientRect().width ?? 0
    const maxWidth = containerWidth > 0 ? containerWidth * 0.34 : 520
    return Math.min(maxWidth, Math.max(192, width))
  }, [])

  const onNodeLibraryResizePointerDown = (event: React.PointerEvent<HTMLDivElement>) => {
    if (event.button !== 0) {
      return
    }
    event.preventDefault()
    const startX = event.clientX
    const startWidth = nodeLibraryWidth
    setNodeLibraryResizing(true)

    const handlePointerMove = (event: PointerEvent) => {
      setNodeLibraryWidth(clampNodeLibraryWidth(startWidth + event.clientX - startX))
    }

    const handlePointerUp = () => {
      window.removeEventListener("pointermove", handlePointerMove)
      window.removeEventListener("pointerup", handlePointerUp)
      setNodeLibraryResizing(false)
    }

    window.addEventListener("pointermove", handlePointerMove)
    window.addEventListener("pointerup", handlePointerUp)
  }

  return (
    <div ref={editorRef} className="flex h-full min-h-0 w-full">
      {nodeLibraryRendered ? (
        <>
          <div
            className={cn(
              "h-full min-h-0 shrink-0 overflow-hidden transition-[width,opacity,transform] duration-200 ease-out",
              nodeLibraryResizing && "transition-none",
              nodeLibraryVisible
                ? "translate-x-0 opacity-100"
                : "-translate-x-3 opacity-0"
            )}
            style={{ width: nodeLibraryVisible ? nodeLibraryWidth : 0 }}
          >
            <aside
              className={[
                "h-full min-h-0 bg-muted/20 transition-all duration-200 ease-out",
                nodeLibraryVisible
                  ? "translate-x-0 opacity-100"
                  : "-translate-x-3 opacity-0",
              ].join(" ")}
            >
              <ScrollArea className="h-full min-h-0">
                <div className="p-3">
                  <div className="mb-3 flex items-center justify-between gap-2">
                    <div className="min-w-0 truncate text-sm font-medium">{we(t, "library.title")}</div>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="size-7 shrink-0 text-muted-foreground hover:text-foreground"
                      onClick={hideNodeLibrary}
                      aria-label={we(t, "library.collapse")}
                    >
                      <PanelLeftCloseIcon className="size-3.5" />
                    </Button>
                  </div>
                  <div className="space-y-2">
                    {nodeSpecs.map((spec) => (
                      <button
                        key={spec.type}
                        type="button"
                        onPointerDown={(event) => onNodePointerDown(event, spec)}
                        onClick={() => {
                          if (suppressNextClickRef.current) {
                            suppressNextClickRef.current = false
                            return
                          }
                          addNode(spec)
                        }}
                        className="flex w-full cursor-grab rounded-md border bg-background px-3 py-2 text-left text-sm hover:bg-muted active:cursor-grabbing"
                      >
                        <span className="min-w-0">
                          <span className="block truncate font-medium">{spec.title}</span>
                          <span className="mt-1 line-clamp-2 text-xs text-muted-foreground">
                            {spec.description}
                          </span>
                          <span className="mt-1 flex gap-2 text-rhd-xs text-muted-foreground">
                            <span>{we(t, "library.inputs", { count: spec.inputSchema?.length ?? 0 })}</span>
                            <span>{we(t, "library.outputs", { count: spec.outputSchema?.length ?? 0 })}</span>
                          </span>
                        </span>
                      </button>
                    ))}
                  </div>
                </div>
              </ScrollArea>
            </aside>
          </div>
          <div
            className={cn(
              "relative flex w-1.5 shrink-0 cursor-col-resize items-center justify-center bg-transparent transition-opacity duration-200 ease-out hover:bg-primary/20",
              nodeLibraryVisible ? "opacity-100" : "pointer-events-none opacity-0"
            )}
            onPointerDown={onNodeLibraryResizePointerDown}
            role="separator"
            aria-orientation="vertical"
            aria-label={we(t, "library.resize")}
          >
            <div className="z-10 flex h-6 w-1 shrink-0 rounded-lg bg-border" />
          </div>
        </>
      ) : null}
      <div className="min-h-0 min-w-0 flex-1">
        <section
          data-workflow-canvas
          ref={canvasRef}
          className={[
            "relative h-full min-h-0",
            pendingNodeDrag?.active ? "ring-2 ring-primary/30" : "",
          ].join(" ")}
        >
          {nodeLibraryCollapsed && !readOnly ? (
            <Button
              type="button"
              variant="outline"
              size="icon"
              className="absolute top-12 left-3 z-20 size-7 rounded-full bg-background/95 text-muted-foreground shadow-sm hover:text-foreground"
              onClick={showNodeLibrary}
              aria-label={we(t, "library.expand")}
            >
              <PanelLeftOpenIcon className="size-3.5" />
            </Button>
          ) : null}
          <ReactFlow
            nodes={renderedNodes}
            edges={renderedEdges}
            nodeTypes={nodeTypes}
            edgeTypes={edgeTypes}
            defaultEdgeOptions={defaultEdgeOptions}
            connectionLineComponent={WorkflowConnectionLine}
            connectionMode={ConnectionMode.Loose}
            connectionRadius={34}
            connectOnClick
            onNodesChange={readOnly ? undefined : onWorkflowNodesChange}
            onEdgesChange={readOnly ? undefined : onWorkflowEdgesChange}
            onConnect={readOnly ? undefined : onConnect}
            onConnectEnd={readOnly ? undefined : onConnectEnd}
            onNodeDragStart={readOnly ? undefined : onNodeDragStart}
            onNodeDrag={readOnly ? undefined : onNodeDrag}
            onNodeDragStop={readOnly ? undefined : onNodeDragStop}
            onInit={setFlowInstance}
            onNodeClick={(event, node) => {
              event.stopPropagation()
              setSelectedEdgeId(null)
              if (!readOnly) showPropertyPanelNode(node)
            }}
            onEdgeClick={(event, edge) => {
              event.stopPropagation()
              setSelectedEdgeId(edge.id)
            }}
            onPaneClick={() => {
              setSelectedEdgeId(null)
              hidePropertyPanel()
            }}
            fitView={!focusEntryInsteadOfShrinking}
            defaultViewport={focusEntryInsteadOfShrinking ? entryViewport : { x: 0, y: 0, zoom: 1 }}
            fitViewOptions={fitViewOptions}
            minZoom={0.16}
            maxZoom={1.35}
            nodesDraggable={!readOnly}
            nodesConnectable={!readOnly}
            edgesReconnectable={!readOnly}
            deleteKeyCode={readOnly ? null : ["Backspace", "Delete"]}
            ariaLabelConfig={{
              "controls.ariaLabel": we(t, "controls.canvas"),
              "controls.zoomIn.ariaLabel": we(t, "controls.zoomIn"),
              "controls.zoomOut.ariaLabel": we(t, "controls.zoomOut"),
              "controls.fitView.ariaLabel": we(t, "controls.fitView"),
            }}
          >
            <Background
              gap={24}
              size={0.8}
              color="hsl(var(--muted-foreground) / 0.11)"
              className="bg-[#f4f6f8] dark:bg-zinc-950"
            />
            <Controls
              className="!bottom-4 !left-4 overflow-hidden !rounded-xl !border !border-border/70 !bg-background/95 !shadow-lg"
              showInteractive={false}
            />
            <WorkflowHelperLines lines={helperLines} />
          </ReactFlow>
          <div className="absolute left-3 top-3 z-20 flex items-center gap-2">
            <WorkflowCanvasToolbar
              validationErrors={validation.errors}
              validationValid={validation.valid}
              onValidate={onValidate}
              validateDisabled={validateDisabled}
              onSaveDraft={onSaveDraft}
              saveDraftDisabled={saveDraftDisabled}
              onPublish={onPublish}
              publishDisabled={publishDisabled}
              onTestRun={onTestRun}
              testRunDisabled={testRunDisabled}
              testRunActive={testRunActive}
              canUndo={historyAvailability.canUndo}
              canRedo={historyAvailability.canRedo}
              onUndo={undoWorkflowEdit}
              onRedo={redoWorkflowEdit}
              onRestoreDefault={onRestoreDefault}
              restoreDefaultDisabled={restoreDefaultDisabled}
              readOnly={readOnly}
            />
          </div>
          {propertyPanelNode ? (
            <aside
              className={[
                "absolute top-3 right-3 z-30 h-[calc(100%-1.5rem)] w-[min(380px,calc(100%-1.5rem))] overflow-hidden rounded-md border bg-background shadow-lg transition-all duration-200 ease-out",
                propertyPanelVisible
                  ? "translate-x-0 scale-100 opacity-100"
                  : "translate-x-3 scale-[0.98] opacity-0",
              ].join(" ")}
            >
              <ScrollArea className="h-full min-h-0">
                {propertyPanelNode ? (
                  <NodeConfigPanel
                    node={propertyPanelNode}
                    nodeSpec={propertyPanelNodeSpec}
                    availableVariables={propertyPanelAvailableVariables}
                    branchSummaries={propertyPanelBranchSummaries}
                    branchTargetOptions={propertyPanelBranchTargetOptions}
                    onChange={updateNodeData}
                  />
                ) : null}
                {!validation.valid ? (
                  <div className="border-t p-4">
                    <div className="mb-2 text-sm font-medium">{we(t, "validation.panelTitle")}</div>
                    <ul className="space-y-1 text-xs text-destructive">
                      {validation.errors.map((error) => (
                        <li key={error}>{error}</li>
                      ))}
                    </ul>
                  </div>
                ) : null}
              </ScrollArea>
            </aside>
          ) : null}
          {pendingNodeDrag?.active ? (
            <div
              className="pointer-events-none fixed z-50 rounded-md border bg-background px-3 py-2 text-sm font-medium shadow-lg"
              style={{
                left: pendingNodeDrag.x + 12,
                top: pendingNodeDrag.y + 12,
              }}
            >
              {pendingNodeDrag.spec.title}
            </div>
          ) : null}
        </section>
      </div>
    </div>
  )
}

function uniqueEdgeId(edges: WorkflowFlowEdge[], source: string, target: string) {
  let nextIndex = edges.length + 1
  let id = `edge_${source}_${target}_${nextIndex}`
  while (edges.some((edge) => edge.id === id)) {
    nextIndex += 1
    id = `edge_${source}_${target}_${nextIndex}`
  }
  return id
}

function enrichNodesForRender(
  nodes: WorkflowFlowNode[],
  nodeSpecs: AIWorkflowNodeSpec[]
): WorkflowFlowNode[] {
  return nodes.map((node) => {
    const spec = getNodeSpec(nodeSpecs, node.data.nodeType ?? "")
    const missingInputs = getRequiredInputs(spec).filter((input) => {
      const selector = node.data.inputs?.[input.name]
      return !selector?.nodeId || !selector.field
    })
    return {
      ...node,
      data: {
        ...node.data,
        title: spec?.title ?? node.data.name ?? node.id,
        description: spec?.description ?? "",
        inputCount: spec?.inputSchema?.length ?? 0,
        outputCount: spec?.outputSchema?.length ?? 0,
        missingInputs: missingInputs.map((input) => input.name),
      },
    }
  })
}

function conditionOperatorLabel(operator: string) {
  const labels: Record<string, string> = {
    eq: weM("condition.eq"),
    neq: weM("condition.neq"),
    contains: weM("condition.contains"),
    exists: weM("condition.exists"),
    not_exists: weM("condition.notExists"),
    truthy: weM("condition.truthy"),
    falsy: weM("condition.falsy"),
    gt: weM("condition.gt"),
    gte: weM("condition.gte"),
    lt: weM("condition.lt"),
    lte: weM("condition.lte"),
  }
  return labels[operator] ?? operator
}

function getBranchSummaries(
  nodes: WorkflowFlowNode[],
  nodeId: string
): WorkflowBranchSummary[] {
  const node = nodes.find((item) => item.id === nodeId)
  const branches = node?.data.config?.branches ?? []
  return branches
    .map((branch) => {
      const target = nodes.find((item) => item.id === branch.targetNodeId)
      return {
        branchId: branch.id,
        targetNodeId: branch.targetNodeId,
        targetName: target?.data.name ?? target?.data.title ?? branch.targetNodeId,
        conditionLabel: branch.condition ? formatConditionLabel(branch.condition) : weM("condition.noCondition"),
        isDefault: Boolean(branch.default),
      }
    })
}

function getBranchTargetOptions(
  nodes: WorkflowFlowNode[],
  edges: WorkflowFlowEdge[],
  nodeId: string
): WorkflowBranchTargetOption[] {
  return edges
    .filter((edge) => edge.source === nodeId)
    .map((edge) => {
      const target = nodes.find((node) => node.id === edge.target)
      return {
        value: edge.target,
        label: target?.data.name ?? target?.data.title ?? edge.target,
      }
    })
}

function formatConditionLabel(condition: WorkflowCondition) {
  const left = condition.left?.nodeId && condition.left.field
    ? `${condition.left.nodeId}.${formatWorkflowVariableName(condition.left.field)}`
    : weM("condition.noVariable")
  const operator = condition.operator
    ? conditionOperatorLabel(condition.operator)
    : weM("condition.noOperator")

  if (["exists", "not_exists", "truthy", "falsy"].includes(condition.operator ?? "")) {
    return `${left} ${operator}`
  }

  return `${left} ${operator} ${formatConditionRight(condition.right)}`
}

function formatConditionRight(value: unknown) {
  if (value === undefined || value === null || value === "") {
    return weM("condition.noValue")
  }
  if (typeof value === "object") {
    return JSON.stringify(value)
  }
  return String(value)
}

function getEventClientPoint(event: MouseEvent | TouchEvent) {
  if ("changedTouches" in event) {
    const touch = event.changedTouches[0] ?? event.touches[0]
    return touch ? { x: touch.clientX, y: touch.clientY } : null
  }
  return { x: event.clientX, y: event.clientY }
}

function isEditableKeyboardTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) {
    return false
  }
  if (target.isContentEditable) {
    return true
  }
  return Boolean(target.closest("input, textarea, select, [contenteditable='true']"))
}

function getEdgeEndpointOffset(position: Position, amount: number) {
  switch (position) {
    case Position.Left:
      return { x: amount, y: 0 }
    case Position.Right:
      return { x: -amount, y: 0 }
    case Position.Top:
      return { x: 0, y: amount }
    case Position.Bottom:
      return { x: 0, y: -amount }
  }
}

function WorkflowConnectionLine({
  fromX,
  fromY,
  fromPosition,
  toX,
  toY,
  toPosition,
  toHandle,
}: ConnectionLineComponentProps) {
  const sourceOffset = getEdgeEndpointOffset(fromPosition ?? Position.Right, workflowHandleRadius)
  const targetOffset = getEdgeEndpointOffset(toPosition ?? Position.Left, toHandle ? workflowHandleRadius : 0)
  const sourceX = fromX + sourceOffset.x
  const sourceY = fromY + sourceOffset.y
  const targetX = toX + targetOffset.x
  const targetY = toY + targetOffset.y
  const [edgePath] = getBezierPath({
    sourceX,
    sourceY,
    sourcePosition: fromPosition ?? Position.Right,
    targetX,
    targetY,
    targetPosition: toPosition ?? Position.Left,
    curvature: 0.18,
  })

  return (
    <g>
      <path
        fill="none"
        stroke="var(--primary)"
        opacity={0.72}
        strokeDasharray="6 5"
        strokeLinecap="round"
        strokeWidth={2}
        d={edgePath}
      />
      <circle cx={toX} cy={toY} r={4} fill="var(--primary)" />
    </g>
  )
}

function WorkflowCanvasEdge({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  selected,
  data,
  markerEnd,
}: EdgeProps<WorkflowFlowEdge>) {
  const sourceOffset = getEdgeEndpointOffset(sourcePosition, workflowHandleRadius)
  const targetOffset = getEdgeEndpointOffset(targetPosition, workflowHandleRadius)
  const [edgePath, labelX, labelY] = getBezierPath({
    sourceX: sourceX + sourceOffset.x,
    sourceY: sourceY + sourceOffset.y,
    sourcePosition,
    targetX: targetX + targetOffset.x,
    targetY: targetY + targetOffset.y,
    targetPosition,
    curvature: 0.18,
  })
  const edgeData = data as WorkflowEdgeRenderData | undefined
  const active = selected || edgeData?.active
  const runStatus = edgeData?.runStatus

  return (
    <>
      <BaseEdge
        id={id}
        path={edgePath}
        markerEnd={markerEnd}
        className={cn(
          "transition-all duration-300",
          !active && "!stroke-zinc-400/60 !stroke-[1.6px]",
          !active && edgeData?.errorFallback && "!stroke-amber-500/80 [stroke-dasharray:7_5]",
          active && !runStatus && "!stroke-blue-500 !stroke-[2.4px]",
          runStatus === "completed" && "!stroke-primary !stroke-[2.8px]",
          runStatus === "failed" && "!stroke-destructive !stroke-[2.8px]",
          runStatus === "interrupted" && "!stroke-amber-500 !stroke-[2.8px]",
        )}
      />
      {edgeData?.branchLabel ? (
        <EdgeLabelRenderer>
          <div
            className={cn(
              "pointer-events-none absolute max-w-44 rounded border bg-white/95 px-2 py-1 text-rhd-2xs font-medium leading-4 text-zinc-600 shadow-sm dark:bg-zinc-950/95 dark:text-zinc-300",
              runStatus === "completed" && "border-primary/20 text-primary",
              runStatus === "failed" && "border-destructive/20 text-destructive",
              runStatus === "interrupted" && "border-amber-300 text-amber-700",
            )}
            style={{ transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)` }}
          >
            {edgeData.branchLabel}
          </div>
        </EdgeLabelRenderer>
      ) : null}
    </>
  )
}

function WorkflowNodeHandle({
  type,
  position,
  className,
}: {
  type: "source" | "target"
  position: Position
  className?: string
}) {
  return (
    <Handle
      type={type}
      position={position}
      className={className}
    >
      <PlusIcon className="size-2.5" />
    </Handle>
  )
}

function WorkflowAddAfterButton({
  nodeId,
  visible,
  className,
  nodeSpecs,
  onAddAfter,
}: {
  nodeId: string
  visible: boolean
  className?: string
  nodeSpecs?: AIWorkflowNodeSpec[]
  onAddAfter?: (sourceNodeId: string, spec: AIWorkflowNodeSpec) => void
}) {
  const t = useI18n()
  if (!nodeSpecs?.length || !onAddAfter) {
    return null
  }
  return (
    <Popover>
      <PopoverTrigger
        render={
          <button
            type="button"
            className={cn(
              "absolute z-20 flex size-5 items-center justify-center rounded-full bg-primary text-primary-foreground shadow-lg transition-all duration-150",
              visible ? "opacity-100" : "pointer-events-none opacity-0",
              className
            )}
            aria-label={we(t, "addAfter.aria")}
          >
            <PlusIcon className="size-3" />
          </button>
        }
      />
      <PopoverContent side="right" align="center" className="w-72 p-2">
        <div className="px-2 pb-2 text-xs font-medium text-muted-foreground">{we(t, "addAfter.title")}</div>
        <div className="max-h-72 space-y-1 overflow-y-auto">
          {nodeSpecs.map((spec) => (
            <button
              key={spec.type}
              type="button"
              className="flex w-full rounded-md px-2 py-2 text-left hover:bg-muted"
              onClick={() => onAddAfter(nodeId, spec)}
            >
              <span className="min-w-0">
                <span className="block truncate text-sm font-medium">{spec.title}</span>
                <span className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">
                  {spec.description}
                </span>
              </span>
            </button>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  )
}

function WorkflowCanvasNode({ id, data, selected }: NodeProps<WorkflowFlowNode>) {
  const t = useI18n()
  const [hovered, setHovered] = useState(false)
  const missingInputs = data.missingInputs ?? []
  const hasIssue = missingInputs.length > 0
  const nodeSpecs = data.nodeSpecs as AIWorkflowNodeSpec[] | undefined
  const onAddAfter = data.onAddAfter as
    | ((sourceNodeId: string, spec: AIWorkflowNodeSpec) => void)
    | undefined
  const readOnly = Boolean(data.readOnly)
  const showHandles = !readOnly && (selected || hovered)
  const nodeIcon = workflowNodeIcon(data.nodeType)
  const visualState = workflowNodeVisualState(data.runStatus, hasIssue, Boolean(data.testRunAvailable))
  const StatusIcon = visualState.Icon
  const handleClassName = cn(
    "!size-3.5 !rounded-full !border-2 !border-white !bg-zinc-900 !text-white !shadow-md dark:!border-zinc-950 dark:!bg-white dark:!text-zinc-950",
    "flex items-center justify-center opacity-0 transition-all duration-150",
    showHandles ? "pointer-events-auto opacity-100" : "pointer-events-none"
  )
  return (
    <div
      className={cn(
        "group/node relative w-56 rounded-md border bg-white shadow-[0_10px_26px_rgba(15,23,42,0.09)] transition-[border-color,box-shadow,transform] dark:bg-zinc-950",
        !readOnly && "hover:-translate-y-0.5 hover:shadow-[0_16px_34px_rgba(15,23,42,0.13)]",
        selected ? "border-primary ring-2 ring-primary/15" : visualState.borderClass,
      )}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
    >
      <span className={cn("absolute inset-y-0 left-0 w-1 rounded-l-[5px]", visualState.railClass)} />
      <WorkflowNodeHandle
        type="target"
        position={Position.Left}
        className={cn("!left-0", handleClassName)}
      />
      <div className="px-3.5 py-3">
        <div className="flex items-start gap-3">
          <div className={cn("flex size-8 shrink-0 items-center justify-center rounded-md border", visualState.iconClass)}>
            {nodeIcon}
          </div>
          <div className="min-w-0 flex-1">
            <div className="truncate text-sm font-semibold text-zinc-900 dark:text-zinc-100">{data.name ?? data.title}</div>
            <div className="mt-1 flex items-center gap-2 text-rhd-xs text-zinc-500">
              <span className="truncate">{data.title}</span>
              <span className="shrink-0 font-mono">I {data.inputCount ?? 0} / O {data.outputCount ?? 0}</span>
            </div>
          </div>
        </div>
        <div className="mt-3 flex items-center justify-between gap-2 border-t border-zinc-200 pt-2.5 text-rhd-xs dark:border-zinc-800">
          <span className={cn("flex min-w-0 items-center gap-1.5 font-medium", visualState.textClass)}>
            <StatusIcon className={cn("size-3.5 shrink-0", data.runStatus === "running" && "animate-pulse")} />
            <span className="truncate">{visualState.label}</span>
          </span>
          {data.runDurationMs !== undefined ? (
            <span className="shrink-0 font-mono text-zinc-500">{data.runDurationMs} ms</span>
          ) : (
            <span className="shrink-0 font-mono text-zinc-400">{data.nodeType}</span>
          )}
        </div>
        {hasIssue && !data.runStatus ? <div className="mt-2 text-rhd-xs text-destructive">{we(t, "node.missingInputs", { names: missingInputs.join("、") })}</div> : null}
        {data.runError ? <div className="mt-2 line-clamp-2 text-rhd-xs leading-4 text-destructive" title={data.runError}>{data.runError}</div> : null}
      </div>
      <WorkflowNodeHandle
        type="source"
        position={Position.Right}
        className={cn("!right-0", handleClassName)}
      />
      <WorkflowAddAfterButton
        nodeId={id}
        visible={showHandles}
        className="right-2 top-2"
        nodeSpecs={nodeSpecs}
        onAddAfter={onAddAfter}
      />
    </div>
  )
}

function workflowNodeIcon(nodeType?: string) {
  switch (nodeType) {
    case "start":
      return <PlayIcon className="size-4" />
    case "end":
      return <CircleStopIcon className="size-4" />
    case "condition":
    case "reply_policy":
    case "answerability_gate":
      return <GitBranchIcon className="size-4" />
    case "entry_context":
      return <ScanLineIcon className="size-4" />
    case "service_access_policy":
      return <ShieldCheckIcon className="size-4" />
    case "knowledge_retrieve":
    case "knowledge_merge":
      return <DatabaseIcon className="size-4" />
    case "llm_reply":
    case "conversation_understanding":
    case "analyze_conversation":
      return <MessageSquareTextIcon className="size-4" />
    case "send_reply":
      return <MessageSquareTextIcon className="size-4" />
    case "create_ticket":
    case "create_video_meeting":
    case "create_knowledge_candidate":
    case "handoff_to_human":
      return <WrenchIcon className="size-4" />
    default:
      return <WorkflowIcon className="size-4" />
  }
}

function workflowNodeVisualState(runStatus: string | undefined, hasIssue: boolean, testRunAvailable: boolean) {
  if (runStatus === "failed") {
    return { label: weM("state.failed"), Icon: AlertCircleIcon, borderClass: "border-destructive", railClass: "bg-destructive", iconClass: "border-destructive/20 bg-destructive/10 text-destructive", textClass: "text-destructive" }
  }
  if (runStatus === "interrupted") {
    return { label: weM("state.interrupted"), Icon: CircleStopIcon, borderClass: "border-amber-400", railClass: "bg-amber-500", iconClass: "border-amber-200 bg-amber-50 text-amber-700", textClass: "text-amber-700" }
  }
  if (runStatus === "running") {
    return { label: weM("state.running"), Icon: WorkflowIcon, borderClass: "border-blue-400", railClass: "bg-blue-500", iconClass: "border-blue-200 bg-blue-50 text-blue-700", textClass: "text-blue-700" }
  }
  if (runStatus === "completed") {
    return { label: weM("state.completed"), Icon: CheckCircle2Icon, borderClass: "border-primary", railClass: "bg-primary", iconClass: "border-primary/20 bg-primary/10 text-primary", textClass: "text-primary" }
  }
  if (testRunAvailable) {
    return { label: weM("state.notTraversed"), Icon: CircleStopIcon, borderClass: "border-zinc-300", railClass: "bg-zinc-300", iconClass: "border-zinc-200 bg-zinc-50 text-zinc-500", textClass: "text-zinc-500" }
  }
  if (hasIssue) {
    return { label: weM("state.pendingConfig"), Icon: AlertCircleIcon, borderClass: "border-destructive/40", railClass: "bg-destructive/80", iconClass: "border-destructive/20 bg-destructive/10 text-destructive", textClass: "text-destructive" }
  }
  return { label: weM("state.complete"), Icon: CheckCircle2Icon, borderClass: "border-zinc-300 dark:border-zinc-700", railClass: "bg-zinc-400", iconClass: "border-zinc-200 bg-zinc-50 text-zinc-700 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300", textClass: "text-zinc-600 dark:text-zinc-400" }
}

function WorkflowCanvasToolbar({
  validationErrors,
  validationValid,
  onValidate,
  validateDisabled,
  onSaveDraft,
  saveDraftDisabled,
  onPublish,
  publishDisabled,
  onTestRun,
  testRunDisabled,
  testRunActive,
  canUndo,
  canRedo,
  onUndo,
  onRedo,
  onRestoreDefault,
  restoreDefaultDisabled,
  readOnly,
}: {
  validationErrors: string[]
  validationValid: boolean
  onValidate?: () => void
  validateDisabled?: boolean
  onSaveDraft?: () => void
  saveDraftDisabled?: boolean
  onPublish?: () => void
  publishDisabled?: boolean
  onTestRun?: () => void
  testRunDisabled?: boolean
  testRunActive?: boolean
  canUndo: boolean
  canRedo: boolean
  onUndo: () => void
  onRedo: () => void
  onRestoreDefault?: () => void
  restoreDefaultDisabled?: boolean
  readOnly?: boolean
}) {
  const t = useI18n()
  return (
    <div className="flex overflow-hidden rounded-md border border-zinc-300 bg-white/95 shadow-[0_8px_24px_rgba(15,23,42,0.08)] dark:border-zinc-700 dark:bg-zinc-950/95">
      <WorkflowValidationIndicator errors={validationErrors} valid={validationValid} />
      {onTestRun ? (
        <>
          <WorkflowToolbarDivider />
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 rounded-none px-2.5 text-xs font-medium text-primary hover:text-primary"
            onClick={onTestRun}
            disabled={testRunDisabled || testRunActive}
          >
            {testRunActive ? <WorkflowIcon className="size-3.5 animate-pulse" /> : <PlayIcon className="size-3.5" />}
            {testRunActive ? we(t, "toolbar.testRunning") : we(t, "toolbar.testRun")}
          </Button>
        </>
      ) : null}
      {onValidate && !readOnly ? (
        <>
          <WorkflowToolbarDivider />
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 rounded-none px-2 text-xs text-muted-foreground hover:text-foreground"
            onClick={onValidate}
            disabled={validateDisabled}
          >
            <CheckCircle2Icon className="size-3.5" />
            {we(t, "toolbar.validate")}
          </Button>
        </>
      ) : null}
      {onSaveDraft && !readOnly ? (
        <>
          <WorkflowToolbarDivider />
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 rounded-none px-2 text-xs text-muted-foreground hover:text-foreground"
            onClick={onSaveDraft}
            disabled={saveDraftDisabled}
          >
            <SaveIcon className="size-3.5" />
            {we(t, "toolbar.saveDraft")}
          </Button>
        </>
      ) : null}
      {onPublish && !readOnly ? (
        <>
          <WorkflowToolbarDivider />
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 rounded-none px-2 text-xs font-medium text-foreground hover:text-foreground"
            onClick={onPublish}
            disabled={publishDisabled}
          >
            <SendIcon className="size-3.5" />
            {we(t, "toolbar.publish")}
          </Button>
        </>
      ) : null}
      {onRestoreDefault && !readOnly ? (
        <>
          <WorkflowToolbarDivider />
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 rounded-none px-2 text-xs text-muted-foreground hover:text-foreground"
            onClick={onRestoreDefault}
            disabled={restoreDefaultDisabled}
          >
            <RotateCcwIcon className="size-3.5" />
            {we(t, "toolbar.restoreDefault")}
          </Button>
        </>
      ) : null}
      {!readOnly ? (
        <>
          <WorkflowToolbarDivider />
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="size-7 rounded-none text-muted-foreground hover:text-foreground"
            onClick={onUndo}
            disabled={!canUndo}
            aria-label={we(t, "toolbar.undo")}
            title={we(t, "toolbar.undo")}
          >
            <Undo2Icon className="size-3.5" />
          </Button>
          <WorkflowToolbarDivider />
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="size-7 rounded-none text-muted-foreground hover:text-foreground"
            onClick={onRedo}
            disabled={!canRedo}
            aria-label={we(t, "toolbar.redo")}
            title={we(t, "toolbar.redo")}
          >
            <Redo2Icon className="size-3.5" />
          </Button>
        </>
      ) : null}
    </div>
  )
}

function WorkflowToolbarDivider() {
  return <div className="my-1.5 h-4 w-px shrink-0 self-center bg-border/70" />
}

function WorkflowValidationIndicator({
  errors,
  valid,
}: {
  errors: string[]
  valid: boolean
}) {
  const t = useI18n()
  if (valid) {
    return (
      <div className="flex h-7 items-center gap-1.5 px-2 text-xs font-medium text-primary">
        <CheckCircle2Icon className="size-3.5" />
        {we(t, "validation.valid")}
      </div>
    )
  }

  return (
    <Popover>
      <PopoverTrigger
        render={
          <button
            type="button"
            className="inline-flex h-7 items-center gap-1.5 px-2 text-xs font-medium text-destructive outline-none hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring"
          />
        }
      >
        <AlertCircleIcon className="size-3.5" />
        {we(t, "validation.pendingCount", { count: errors.length })}
      </PopoverTrigger>
      <PopoverContent side="bottom" align="start" className="w-80">
        <div className="text-sm font-medium">Validation issues</div>
        <ul className="mt-2 max-h-72 space-y-1 overflow-y-auto text-xs text-destructive">
          {errors.map((error) => (
            <li key={error} className="rounded-md bg-destructive/10 px-2 py-1.5">
              {error}
            </li>
          ))}
        </ul>
      </PopoverContent>
    </Popover>
  )
}

function WorkflowHelperLines({ lines }: { lines: WorkflowHelperLine }) {
  if (!lines.horizontal && !lines.vertical) {
    return null
  }

  return (
    <ViewportPortal>
      {lines.horizontal ? (
        <div
          className="pointer-events-none absolute z-10 h-px bg-primary/70 shadow-[0_0_0_1px_hsl(var(--primary)/0.18)]"
          style={{
            left: lines.horizontal.left,
            top: lines.horizontal.y,
            width: lines.horizontal.width,
          }}
        />
      ) : null}
      {lines.vertical ? (
        <div
          className="pointer-events-none absolute z-10 w-px bg-primary/70 shadow-[0_0_0_1px_hsl(var(--primary)/0.18)]"
          style={{
            left: lines.vertical.x,
            top: lines.vertical.top,
            height: lines.vertical.height,
          }}
        />
      ) : null}
    </ViewportPortal>
  )
}
