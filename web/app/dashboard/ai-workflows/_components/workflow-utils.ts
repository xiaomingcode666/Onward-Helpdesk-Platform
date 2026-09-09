export type WorkflowNodePosition = {
  x: number
  y: number
}

export type WorkflowHelperLineNode = {
  id: string
  position: WorkflowNodePosition
  width?: number | null
  height?: number | null
  measured?: {
    width?: number | null
    height?: number | null
  }
}

export type WorkflowHelperLine = {
  horizontal?: {
    y: number
    left: number
    width: number
  }
  vertical?: {
    x: number
    top: number
    height: number
  }
}

export type WorkflowHelperLineResult = WorkflowHelperLine & {
  position: WorkflowNodePosition
}

export type WorkflowEditorNode = {
  id: string
  type?: string
  position: WorkflowNodePosition
  data?: {
    nodeType?: string
    name?: string
    label?: string
    config?: WorkflowNodeConfig
    inputs?: Record<string, WorkflowVariableSelector>
    errorTargetNodeId?: string
  }
}

export type WorkflowEditorEdge = {
  id: string
  source: string
  target: string
}

export type WorkflowCondition = {
  expression?: string
  left?: WorkflowVariableSelector
  operator?: string
  right?: unknown
}

export type WorkflowConditionBranch = {
  id: string
  name?: string
  targetNodeId: string
  condition?: WorkflowCondition
  default?: boolean
}

export type WorkflowNodeConfig = Record<string, unknown> & {
  branches?: WorkflowConditionBranch[]
  bindingMethod?: "agent_default" | "product_context" | "public_product" | "internal_product" | string
  scope?: string
  topK?: number
  scoreThreshold?: number
  contextMaxTokens?: number
  maxContextItems?: number
  minScore?: number
  strongScore?: number
  minMatchedTerms?: number
  unboundMode?: "quick_ai" | "product_context"
  humanHandoffMode?: "inherit" | "always" | "never" | "device_only"
  ticketAccessMode?: "inherit" | "always" | "never" | "device_only"
  sources?: WorkflowVariableSelector[]
  maxItems?: number
  workflowVersionId?: number
  maxIterations?: number
  until?: WorkflowCondition
  failurePolicy?: "fail" | "continue"
}

export type WorkflowDraft = {
  nodes: WorkflowEditorNode[]
  edges: WorkflowEditorEdge[]
}

export type WorkflowDefinition = {
  schemaVersion: number
  entryNodeId: string
  nodes: {
    id: string
    type: string
    name: string
    position: WorkflowNodePosition
    config: WorkflowNodeConfig
    inputs?: Record<string, WorkflowVariableSelector>
    errorTargetNodeId?: string
  }[]
  edges: {
    id: string
    source: string
    target: string
  }[]
}

export type WorkflowVariableType =
  | "string"
  | "number"
  | "integer"
  | "boolean"
  | "object"
  | "array<string>"
  | "array<int>"
  | "array<object>"
  | "any"

export type WorkflowTranslate = (key: string, values?: Record<string, string | number>) => string

export function workflowExtractText(
  t: WorkflowTranslate | undefined,
  key: string,
  fallback: string,
  values?: Record<string, string | number>,
) {
  const fullKey = `workflowExtract.${key}`
  const translated = t?.(fullKey, values)
  return translated && translated !== fullKey ? translated : fallback
}

const workflowVariableNameLabelFallbacks: Record<string, string> = {
  conversationId: "Conversation ID",
  messageId: "Message ID",
  aiAgentId: "Reception config ID",
  userMessage: "User message",
  knowledgeBaseIds: "Knowledge base IDs",
  normalizedMessage: "Normalized message",
  messageIntent: "Message intent",
  answerScope: "Answer scope",
  confidence: "Confidence",
  riskSignals: "Risk signals",
  reason: "Reason",
  answerability: "Answerability",
  action: "Policy action",
  replyText: "Reply content",
  requiresFlow: "Requires flow",
  targetFlow: "Target flow",
  finalReplySource: "Final reply source",
  query: "Search query",
  knowledgeItems: "Knowledge items",
  items: "Knowledge items",
  summary: "Summary",
  sourceCount: "Source count",
  matched: "Matched",
  intent: "User intent",
  riskLevel: "Risk level",
  needTicket: "Suggest ticket",
  needHumanHandoff: "Suggest human handoff",
  issue: "Issue summary",
  ticketDraft: "Ticket draft",
  prompt: "Confirmation prompt",
  confirmed: "Confirmation result",
  responseText: "Confirmation reply",
  ticketId: "Ticket ID",
  ticketNo: "Ticket no.",
  created: "Created",
  message: "Message",
  handoffId: "Handoff ID",
  decision: "Assignment result",
  teamId: "Team ID",
  assigneeId: "Assignee ID",
  ticketCreated: "Ticket created",
  sent: "Sent",
  replyMessageId: "Reply message ID",
  status: "Status",
  nodePath: "Node path",
  iteration: "Iteration count",
  completed: "Completed",
}

const workflowVariableTypeLabelFallbacks: Record<string, string> = {
  string: "Text",
  number: "Number",
  integer: "Integer",
  boolean: "Boolean",
  object: "Object",
  "array<string>": "Text list",
  "array<int>": "Integer list",
  "array<object>": "Object list",
  any: "Any type",
}

export function formatWorkflowVariableName(name: string, t?: WorkflowTranslate): string {
  const fallback = workflowVariableNameLabelFallbacks[name] ?? name
  return workflowExtractText(t, `variables.names.${name}`, fallback)
}

export function formatWorkflowVariableType(type: string, t?: WorkflowTranslate): string {
  const fallback = workflowVariableTypeLabelFallbacks[type] ?? type
  return workflowExtractText(t, `variables.types.${type}`, fallback)
}

export function formatWorkflowNodeSpecTitle(spec: WorkflowNodeSpec, t?: WorkflowTranslate): string {
  return workflowExtractText(t, `nodeTypes.${spec.type}.title`, spec.title ?? spec.type)
}

export function formatWorkflowNodeSpecDescription(spec: WorkflowNodeSpec, t?: WorkflowTranslate): string {
  return workflowExtractText(t, `nodeTypes.${spec.type}.description`, spec.description ?? "")
}

export type WorkflowVariableSelector = {
  nodeId: string
  field: string
}

export type WorkflowVariableSpec = {
  name: string
  type: WorkflowVariableType
  required?: boolean
  description?: string
}

export type WorkflowNodeSpec = {
  type: string
  title?: string
  description?: string
  inputSchema?: WorkflowVariableSpec[]
  outputSchema?: WorkflowVariableSpec[]
  defaultInputs?: Record<string, WorkflowVariableSelector>
}

export type WorkflowVariableRef = {
  nodeId: string
  nodeName: string
  field: string
  type: string
  description: string
}

export type WorkflowDraftValidation = {
  valid: boolean
  errors: string[]
}

export type WorkflowHistory<T> = {
  past: T[]
  future: T[]
  limit: number
}

export type WorkflowHistoryChange<T> = {
  history: WorkflowHistory<T>
  snapshot: T
}

const helperLineAlignmentThreshold = 6
const defaultWorkflowHistoryLimit = 50

function cloneHistorySnapshot<T>(snapshot: T): T {
  return JSON.parse(JSON.stringify(snapshot)) as T
}

export function createWorkflowHistory<T>(limit = defaultWorkflowHistoryLimit): WorkflowHistory<T> {
  return {
    past: [],
    future: [],
    limit,
  }
}

export function pushWorkflowHistory<T>(
  history: WorkflowHistory<T>,
  snapshot: T
): WorkflowHistory<T> {
  const past = [...history.past, cloneHistorySnapshot(snapshot)]
  return {
    past: past.slice(Math.max(0, past.length - history.limit)),
    future: [],
    limit: history.limit,
  }
}

export function undoWorkflowHistory<T>(
  history: WorkflowHistory<T>,
  current: T
): WorkflowHistoryChange<T> | null {
  const snapshot = history.past.at(-1)
  if (!snapshot) {
    return null
  }
  return {
    snapshot: cloneHistorySnapshot(snapshot),
    history: {
      past: history.past.slice(0, -1),
      future: [cloneHistorySnapshot(current), ...history.future],
      limit: history.limit,
    },
  }
}

export function redoWorkflowHistory<T>(
  history: WorkflowHistory<T>,
  current: T
): WorkflowHistoryChange<T> | null {
  const snapshot = history.future[0]
  if (!snapshot) {
    return null
  }
  const past = [...history.past, cloneHistorySnapshot(current)]
  return {
    snapshot: cloneHistorySnapshot(snapshot),
    history: {
      past: past.slice(Math.max(0, past.length - history.limit)),
      future: history.future.slice(1),
      limit: history.limit,
    },
  }
}

function getNodeSize(node: WorkflowHelperLineNode) {
  return {
    width: node.measured?.width ?? node.width ?? 0,
    height: node.measured?.height ?? node.height ?? 0,
  }
}

function getNodeAnchorValues(node: WorkflowHelperLineNode) {
  const size = getNodeSize(node)
  return {
    x: [
      node.position.x,
      node.position.x + size.width / 2,
      node.position.x + size.width,
    ],
    y: [
      node.position.y,
      node.position.y + size.height / 2,
      node.position.y + size.height,
    ],
  }
}

function getNearestAlignment(
  axis: "x" | "y",
  nodes: WorkflowHelperLineNode[],
  draggingNode: WorkflowHelperLineNode
) {
  const draggingAnchors = getNodeAnchorValues(draggingNode)[axis]
  let nearest:
    | {
        diff: number
        targetValue: number
        candidate: WorkflowHelperLineNode
      }
    | undefined

  for (const candidate of nodes) {
    if (candidate.id === draggingNode.id) {
      continue
    }
    const candidateAnchors = getNodeAnchorValues(candidate)[axis]
    for (const draggingAnchor of draggingAnchors) {
      for (const candidateAnchor of candidateAnchors) {
        const diff = candidateAnchor - draggingAnchor
        if (Math.abs(diff) > helperLineAlignmentThreshold) {
          continue
        }
        if (!nearest || Math.abs(diff) < Math.abs(nearest.diff)) {
          nearest = {
            diff,
            targetValue: candidateAnchor,
            candidate,
          }
        }
      }
    }
  }

  return nearest
}

export function calculateWorkflowHelperLines(
  nodes: WorkflowHelperLineNode[],
  draggingNode: WorkflowHelperLineNode
): WorkflowHelperLineResult {
  const xAlignment = getNearestAlignment("x", nodes, draggingNode)
  const yAlignment = getNearestAlignment("y", nodes, draggingNode)
  const position = {
    x: draggingNode.position.x + (xAlignment?.diff ?? 0),
    y: draggingNode.position.y + (yAlignment?.diff ?? 0),
  }
  const draggingSize = getNodeSize(draggingNode)
  const snappedDraggingNode = {
    ...draggingNode,
    position,
  }
  const result: WorkflowHelperLineResult = {
    position,
  }

  if (yAlignment) {
    const candidateSize = getNodeSize(yAlignment.candidate)
    const left = Math.min(snappedDraggingNode.position.x, yAlignment.candidate.position.x)
    const right = Math.max(
      snappedDraggingNode.position.x + draggingSize.width,
      yAlignment.candidate.position.x + candidateSize.width
    )
    result.horizontal = {
      y: yAlignment.targetValue,
      left,
      width: right - left,
    }
  }

  if (xAlignment) {
    const candidateSize = getNodeSize(xAlignment.candidate)
    const top = Math.min(snappedDraggingNode.position.y, xAlignment.candidate.position.y)
    const bottom = Math.max(
      snappedDraggingNode.position.y + draggingSize.height,
      xAlignment.candidate.position.y + candidateSize.height
    )
    result.vertical = {
      x: xAlignment.targetValue,
      top,
      height: bottom - top,
    }
  }

  return result
}

export function validateWorkflowDraft(
  draft: WorkflowDraft,
  nodeSpecs: WorkflowNodeSpec[] = [],
  t?: WorkflowTranslate,
): WorkflowDraftValidation {
  const errors: string[] = []
  const nodeIds = new Set<string>()
  let startCount = 0
  let endCount = 0

  for (const node of draft.nodes) {
    const id = node.id.trim()
    if (!id) {
      errors.push(workflowExtractText(t, "validation.nodeIdRequired", "Node ID is required"))
      continue
    }
    if (nodeIds.has(id)) {
      errors.push(workflowExtractText(t, "validation.duplicateNodeId", "Duplicate node ID: {id}", { id }))
    }
    nodeIds.add(id)
    const nodeType = node.data?.nodeType ?? node.type
    if (nodeType === "start") {
      startCount += 1
    }
    if (nodeType === "end") {
      endCount += 1
    }
    const errorTargetNodeId = node.data?.errorTargetNodeId?.trim()
    if (
      errorTargetNodeId
      && !nodeIds.has(errorTargetNodeId)
      && draft.nodes.every((candidate) => candidate.id.trim() !== errorTargetNodeId)
    ) {
      errors.push(workflowExtractText(t, "validation.errorTargetMissing", "{node} has an error fallback target that does not exist.", {
        node: node.data?.name ?? node.id,
      }))
    }
  }

  if (startCount !== 1) {
    errors.push(workflowExtractText(t, "validation.startNodeCount", "The flow must contain exactly one start node."))
  }
  if (endCount < 1) {
    errors.push(workflowExtractText(t, "validation.endNodeRequired", "The flow needs at least one end node."))
  }

  const edgeIds = new Set<string>()
  const outgoingTargets = new Map<string, Set<string>>()
  for (const edge of draft.edges) {
    const id = edge.id.trim()
    if (!id) {
      errors.push(workflowExtractText(t, "validation.edgeIdRequired", "Edge ID is required"))
    } else if (edgeIds.has(id)) {
      errors.push(workflowExtractText(t, "validation.duplicateEdgeId", "Duplicate edge ID: {id}", { id }))
    }
    edgeIds.add(id)
    if (!nodeIds.has(edge.source)) {
      errors.push(workflowExtractText(t, "validation.edgeSourceMissing", "Edge source node does not exist: {source}", { source: edge.source }))
    }
    if (!nodeIds.has(edge.target)) {
      errors.push(workflowExtractText(t, "validation.edgeTargetMissing", "Edge target node does not exist: {target}", { target: edge.target }))
    }
    if (!outgoingTargets.has(edge.source)) {
      outgoingTargets.set(edge.source, new Set())
    }
    outgoingTargets.get(edge.source)?.add(edge.target)
  }

  for (const node of draft.nodes) {
    const nodeType = node.data?.nodeType ?? node.type ?? ""
    const errorTargetNodeId = node.data?.errorTargetNodeId?.trim()
    if (errorTargetNodeId && !(outgoingTargets.get(node.id) ?? new Set<string>()).has(errorTargetNodeId)) {
      errors.push(workflowExtractText(t, "validation.errorTargetNeedsEdge", "{node} needs an edge to the error fallback target.", {
        node: node.data?.name ?? node.id,
      }))
    }
    const spec = getNodeSpec(nodeSpecs, nodeType)
    if (!spec) {
      continue
    }
    for (const input of getRequiredInputs(spec)) {
      const selector = node.data?.inputs?.[input.name]
      if (!selector?.nodeId || !selector.field) {
        const nodeName = node.data?.name ?? spec.title ?? node.id
        errors.push(workflowExtractText(t, "validation.requiredInputMissing", "{node} is missing required input \"{input}\". Select an upstream output variable.", {
          node: nodeName,
          input: formatWorkflowVariableName(input.name, t),
        }))
      }
    }
    if (nodeType === "condition") {
      const branches = node.data?.config?.branches ?? []
      if (branches.length === 0) {
        errors.push(workflowExtractText(t, "validation.conditionBranchRequired", "{node} needs at least one branch.", {
          node: node.data?.name ?? node.id,
        }))
        continue
      }
      let defaultCount = 0
      const branchIds = new Set<string>()
      const targets = outgoingTargets.get(node.id) ?? new Set<string>()
      for (const branch of branches) {
        const branchName = branch.name || branch.id || workflowExtractText(t, "validation.unnamedBranch", "Unnamed branch")
        if (!branch.id) {
          errors.push(workflowExtractText(t, "validation.branchIdRequired", "{node} has a branch without an ID.", {
            node: node.data?.name ?? node.id,
          }))
        } else if (branchIds.has(branch.id)) {
          errors.push(workflowExtractText(t, "validation.duplicateBranchId", "{node} has duplicate branch ID \"{branchId}\".", {
            node: node.data?.name ?? node.id,
            branchId: branch.id,
          }))
        }
        branchIds.add(branch.id)
        if (!branch.targetNodeId) {
          errors.push(workflowExtractText(t, "validation.branchTargetRequired", "{node} branch \"{branch}\" is missing a target node.", {
            node: node.data?.name ?? node.id,
            branch: branchName,
          }))
        } else if (!nodeIds.has(branch.targetNodeId)) {
          errors.push(workflowExtractText(t, "validation.branchTargetMissing", "{node} branch \"{branch}\" target node does not exist.", {
            node: node.data?.name ?? node.id,
            branch: branchName,
          }))
        } else if (!targets.has(branch.targetNodeId)) {
          errors.push(workflowExtractText(t, "validation.branchNeedsEdge", "{node} branch \"{branch}\" needs an edge to the target node.", {
            node: node.data?.name ?? node.id,
            branch: branchName,
          }))
        }
        if (branch.default) {
          defaultCount += 1
          if (branch.condition) {
            errors.push(workflowExtractText(t, "validation.defaultBranchNoCondition", "{node} default branch cannot have conditions.", {
              node: node.data?.name ?? node.id,
            }))
          }
          continue
        }
        if (!branch.condition?.left?.nodeId || !branch.condition.left.field) {
          errors.push(workflowExtractText(t, "validation.branchVariableRequired", "{node} branch \"{branch}\" is missing a condition variable.", {
            node: node.data?.name ?? node.id,
            branch: branchName,
          }))
        }
        if (!branch.condition?.operator) {
          errors.push(workflowExtractText(t, "validation.branchOperatorRequired", "{node} branch \"{branch}\" is missing a condition operator.", {
            node: node.data?.name ?? node.id,
            branch: branchName,
          }))
        }
      }
      if (defaultCount !== 1) {
        errors.push(workflowExtractText(t, "validation.defaultBranchCount", "{node} must have exactly one default branch.", {
          node: node.data?.name ?? node.id,
        }))
      }
    }
  }

  return {
    valid: errors.length === 0,
    errors,
  }
}

export function toApiDefinition(draft: WorkflowDraft): WorkflowDefinition {
  const startNode = draft.nodes.find((node) => (node.data?.nodeType ?? node.type) === "start")
  return {
    schemaVersion: 1,
    entryNodeId: startNode?.id ?? "",
    nodes: draft.nodes.map((node) => ({
      id: node.id,
      type: node.data?.nodeType ?? node.type ?? "",
      name: node.data?.name ?? node.type ?? node.id,
      position: {
        x: node.position.x,
        y: node.position.y,
      },
      config: node.data?.config ?? {},
      ...(node.data?.inputs ? { inputs: node.data.inputs } : {}),
      ...(node.data?.errorTargetNodeId ? { errorTargetNodeId: node.data.errorTargetNodeId } : {}),
    })),
    edges: draft.edges.map((edge) => ({
      id: edge.id,
      source: edge.source,
      target: edge.target,
    })),
  }
}

export function fromApiDefinition(definition: WorkflowDefinition): WorkflowDraft {
  return {
    nodes: (definition.nodes ?? []).map((node) => ({
      id: node.id,
      type: node.type,
      position: node.position ?? { x: 0, y: 0 },
      data: {
        nodeType: node.type,
        name: node.name,
        config: node.config ?? {},
        inputs: node.inputs ?? {},
        errorTargetNodeId: node.errorTargetNodeId,
      },
    })),
    edges: (definition.edges ?? []).map((edge) => ({
      id: edge.id,
      source: edge.source,
      target: edge.target,
    })),
  }
}

export function applyAutoInputMappings(
  draft: WorkflowDraft,
  sourceNodeId: string,
  targetNodeId: string,
  nodeSpecs: WorkflowNodeSpec[]
): WorkflowDraft {
  const sourceNode = draft.nodes.find((node) => node.id === sourceNodeId)
  const targetNode = draft.nodes.find((node) => node.id === targetNodeId)
  if (!sourceNode || !targetNode) {
    return draft
  }
  const sourceSpec = getNodeSpec(nodeSpecs, sourceNode.data?.nodeType ?? sourceNode.type ?? "")
  const targetSpec = getNodeSpec(nodeSpecs, targetNode.data?.nodeType ?? targetNode.type ?? "")
  if (!sourceSpec || !targetSpec) {
    return draft
  }
  const nextInputs = { ...(targetNode.data?.inputs ?? {}) }
  let changed = false

  for (const input of targetSpec.inputSchema ?? []) {
    if (nextInputs[input.name]) {
      continue
    }
    const output = findPreferredOutput(input.name, input.type, sourceSpec.outputSchema ?? [])
    if (!output) {
      continue
    }
    nextInputs[input.name] = { nodeId: sourceNodeId, field: output.name }
    changed = true
  }

  if (!changed) {
    return draft
  }

  return {
    ...draft,
    nodes: draft.nodes.map((node) =>
      node.id === targetNodeId
        ? {
            ...node,
            data: {
              ...node.data,
              inputs: nextInputs,
            },
          }
        : node
    ),
  }
}

export function createWorkflowNodeFromSpec(
  spec: WorkflowNodeSpec,
  existingNodes: Pick<WorkflowEditorNode, "id">[],
  position: WorkflowNodePosition,
  t?: WorkflowTranslate,
): WorkflowEditorNode {
  const id = uniqueNodeId(existingNodes, spec.type)
  const config = defaultWorkflowNodeConfig(spec.type, id)
  const title = formatWorkflowNodeSpecTitle(spec, t)
  return {
    id,
    type: "workflowNode",
    position,
    data: {
      nodeType: spec.type,
      name: title,
      label: title,
      config,
      inputs: spec.defaultInputs ?? {},
    },
  }
}

function defaultWorkflowNodeConfig(nodeType: string, nodeId: string): WorkflowNodeConfig {
  switch (nodeType) {
    case "entry_context":
      return { unboundMode: "quick_ai" }
    case "service_access_policy":
      return { humanHandoffMode: "device_only", ticketAccessMode: "device_only" }
    case "knowledge_retrieve":
      return { bindingMethod: "public_product", topK: 8, scoreThreshold: 0.3 }
    case "knowledge_merge":
      return { sources: [], maxItems: 10 }
    case "answerability_gate":
      return { minScore: 0.35, strongScore: 0.72, minMatchedTerms: 1 }
    case "subflow":
      return { workflowVersionId: 0 }
    case "loop":
      return {
        workflowVersionId: 0,
        maxIterations: 3,
        failurePolicy: "fail",
        until: {
          left: { nodeId, field: "iteration" },
          operator: "gte",
          right: 3,
        },
      }
    default:
      return {}
  }
}

function uniqueNodeId(existingNodes: Pick<WorkflowEditorNode, "id">[], nodeType: string) {
  let nextIndex = existingNodes.length + 1
  let id = `${nodeType}_${nextIndex}`
  while (existingNodes.some((node) => node.id === id)) {
    nextIndex += 1
    id = `${nodeType}_${nextIndex}`
  }
  return id
}

function findPreferredOutput(
  inputName: string,
  inputType: WorkflowVariableType,
  outputs: WorkflowVariableSpec[]
): WorkflowVariableSpec | undefined {
  const preferred = preferredOutputName(inputName)
  if (preferred) {
    const exact = outputs.find((output) => output.name === preferred && variableTypesCompatible(inputType, output.type))
    if (exact) {
      return exact
    }
  }
  const sameName = outputs.find((output) => output.name === inputName && variableTypesCompatible(inputType, output.type))
  if (sameName) {
    return sameName
  }
  return outputs.find((output) => variableTypesCompatible(inputType, output.type))
}

function preferredOutputName(inputName: string): string {
  switch (inputName) {
    case "query":
    case "userMessage":
    case "issue":
    case "prompt":
      return "userMessage"
    case "knowledgeItems":
      return "items"
    case "replyText":
      return "replyText"
    case "confirmed":
      return "confirmed"
    case "ticketDraft":
      return "ticketDraft"
    case "reason":
      return "reason"
    default:
      return ""
  }
}

function variableTypesCompatible(input: WorkflowVariableType, output: WorkflowVariableType): boolean {
  return input === "any" || output === "any" || input === output
}

export function getNodeSpec(
  nodeSpecs: WorkflowNodeSpec[],
  nodeType: string
): WorkflowNodeSpec | undefined {
  return nodeSpecs.find((spec) => spec.type === nodeType)
}

export function getRequiredInputs(spec: WorkflowNodeSpec | undefined): WorkflowVariableSpec[] {
  return (spec?.inputSchema ?? []).filter((item) => item.required)
}

export function getAvailableVariables(
  draft: WorkflowDraft,
  nodeId: string,
  nodeSpecs: WorkflowNodeSpec[]
): WorkflowVariableRef[] {
  const ancestors = collectAncestorNodeIds(draft, nodeId)
  const nodesById = new Map(draft.nodes.map((node) => [node.id, node]))
  const variables: WorkflowVariableRef[] = []

  for (const sourceNodeId of ancestors) {
    const sourceNode = nodesById.get(sourceNodeId)
    if (!sourceNode) {
      continue
    }
    const nodeType = sourceNode.data?.nodeType ?? sourceNode.type ?? ""
    const spec = getNodeSpec(nodeSpecs, nodeType)
    for (const output of spec?.outputSchema ?? []) {
      variables.push({
        nodeId: sourceNode.id,
        nodeName: sourceNode.data?.name ?? spec?.title ?? sourceNode.id,
        field: output.name,
        type: output.type,
        description: output.description ?? "",
      })
    }
  }

  return variables
}

function collectAncestorNodeIds(draft: WorkflowDraft, nodeId: string): string[] {
  const incoming = new Map<string, string[]>()
  for (const edge of draft.edges) {
    const sources = incoming.get(edge.target) ?? []
    sources.push(edge.source)
    incoming.set(edge.target, sources)
  }

  const visited = new Set<string>()
  const ordered: string[] = []

  function visit(current: string) {
    for (const source of incoming.get(current) ?? []) {
      if (visited.has(source)) {
        continue
      }
      visited.add(source)
      visit(source)
      ordered.push(source)
    }
  }

  visit(nodeId)
  return ordered
}
