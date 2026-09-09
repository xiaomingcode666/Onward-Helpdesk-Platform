import { translateCurrentMessage } from "@/i18n/messages"

function pt(key: string, values?: Record<string, string | number>) {
  return translateCurrentMessage(`workflowExtract.productUtils.${key}`, values)
}

export type ProductWorkflowDefinition = {
  schemaVersion?: number
  entryNodeId: string
  modelPolicy?: {
    credentialChain?: WorkflowModelCredentialSource[]
  }
  runtimeVariables?: Array<Record<string, unknown>>
  nodes: Array<{
    id: string
    type: string
    name?: string
    position?: { x: number; y: number }
    config?: Record<string, unknown> & {
      branches?: Array<{
        id?: string
        name?: string
        targetNodeId?: string
        default?: boolean
      }>
    }
    inputs?: Record<string, { nodeId: string; field: string }>
    errorTargetNodeId?: string
  }>
  edges: Array<{ id?: string; source: string; target: string }>
}

export type WorkflowModelCredentialSource = "tenant_default" | "product"
export type WorkflowModelCredentialPreset =
  | "product_then_tenant"
  | "tenant_then_product"
  | "product_only"
  | "tenant_only"
  | "custom"

const workflowModelCredentialChains: Record<Exclude<WorkflowModelCredentialPreset, "custom">, WorkflowModelCredentialSource[]> = {
  product_then_tenant: ["product", "tenant_default"],
  tenant_then_product: ["tenant_default", "product"],
  product_only: ["product"],
  tenant_only: ["tenant_default"],
}

export function getWorkflowModelCredentialPreset(
  chain?: WorkflowModelCredentialSource[],
): WorkflowModelCredentialPreset {
  const signature = (chain?.length ? chain : workflowModelCredentialChains.product_then_tenant).join(">")
  for (const [preset, sources] of Object.entries(workflowModelCredentialChains)) {
    if (sources.join(">") === signature) return preset as WorkflowModelCredentialPreset
  }
  return "custom"
}

export function getWorkflowModelCredentialChain(
  preset: Exclude<WorkflowModelCredentialPreset, "custom">,
): WorkflowModelCredentialSource[] {
  return [...workflowModelCredentialChains[preset]]
}

export function describeWorkflowModelCredentialChain(chain?: WorkflowModelCredentialSource[]) {
  const effective = chain?.length ? chain : workflowModelCredentialChains.product_then_tenant
  const labels: Record<WorkflowModelCredentialSource, string> = {
    product: pt("credentialChain.product"),
    tenant_default: pt("credentialChain.tenantDefault"),
  }
  return {
    label: effective.map((source) => labels[source]).join(" → "),
  }
}

export type WorkflowTestScenario = {
  key: string
  title: string
  message?: string
  branchOverrides: Record<string, string>
  runtimeContext: WorkflowTestRuntimeContext
  autoConfirm: boolean
}

export type WorkflowTestRuntimeContext = {
  productModelId?: number
  deviceId?: number
  serviceCodeId?: number
  customerEntrySessionId?: number
  deviceBound?: boolean
  serviceMode?: "ai_only" | "human_only" | "ai_first"
}

export type WorkflowJourneyStage = {
  key: string
  title: string
  nodeTypes: string[]
}

export type WorkflowProductCapability = {
  key: "knowledge" | "ticket" | "handoff" | "video" | "learning"
  title: string
  state: "enabled" | "disabled" | "prohibited"
  stateLabel: string
}

export type WorkflowReadinessCheck = {
  key: string
  title: string
  state: "pass" | "warning" | "error"
}

export type WorkflowProductProfile = {
  mode: "ai_only" | "dispatch_only" | "ai_human"
  title: string
  journey: WorkflowJourneyStage[]
  capabilities: WorkflowProductCapability[]
  nodeTypeSet: Set<string>
}

export type WorkflowServiceBlueprint = "basic_ai" | "dispatch_only" | "device_ai" | "ai_human"

export type WorkflowRunErrorPresentation = {
  summary: string
  technicalDetails: string
}

export function isKnowledgeSupportWorkflowDefinition(definition: ProductWorkflowDefinition) {
  return definition.nodes.some((node) => node.id === "tenant_retrieve_1" && node.type === "knowledge_retrieve")
}

export function describeWorkflowRunError(errorMessage: string): WorkflowRunErrorPresentation {
  const technicalDetails = errorMessage.trim()
  if (!technicalDetails) return { summary: "", technicalDetails: "" }

  const normalized = technicalDetails.toLowerCase()
  if (normalized.includes("vector dimension error") || normalized.includes("knowledge index dimension mismatch")) {
    return {
      summary: pt("runErrors.incompatibleVector"),
      technicalDetails,
    }
  }
  if (normalized.includes("failed to search collection") || normalized.includes("failed to search vectors")) {
    return {
      summary: pt("runErrors.searchFailed"),
      technicalDetails,
    }
  }
  if (normalized.includes("generate query embedding failed") || normalized.includes("failed to generate query embedding")) {
    return {
      summary: pt("runErrors.embeddingFailed"),
      technicalDetails,
    }
  }
  if (normalized.includes("model timeout") || normalized.includes("deadline exceeded")) {
    return {
      summary: pt("runErrors.modelTimeout"),
      technicalDetails,
    }
  }
  if (normalized.includes("model credential policy has no available credential")) {
    return {
      summary: pt("runErrors.noCredential"),
      technicalDetails,
    }
  }
  if (normalized.includes("model credential policy conflicts")) {
    return {
      summary: pt("runErrors.credentialConflict"),
      technicalDetails,
    }
  }
  return { summary: technicalDetails, technicalDetails: "" }
}

const journeyCatalog: Array<Omit<WorkflowJourneyStage, "title"> & { titleKey: string }> = [
  {
    key: "receive",
    titleKey: "journeyStages.receive",
    nodeTypes: ["start", "entry_context"],
  },
  {
    key: "understand",
    titleKey: "journeyStages.understand",
    nodeTypes: ["service_access_policy", "conversation_understanding", "reply_policy", "analyze_conversation", "condition"],
  },
  {
    key: "knowledge",
    titleKey: "journeyStages.knowledge",
    nodeTypes: ["knowledge_retrieve", "knowledge_merge", "answerability_gate"],
  },
  {
    key: "answer",
    titleKey: "journeyStages.answer",
    nodeTypes: ["llm_reply"],
  },
  {
    key: "service_action",
    titleKey: "journeyStages.serviceAction",
    nodeTypes: ["prepare_ticket_draft", "human_confirm", "create_ticket", "handoff_to_human", "create_video_meeting"],
  },
  {
    key: "respond",
    titleKey: "journeyStages.respond",
    nodeTypes: ["send_reply", "end"],
  },
]

function hasAny(nodeTypes: Set<string>, expected: string[]) {
  return expected.some((type) => nodeTypes.has(type))
}

function nodeHasStaticReply(node: ProductWorkflowDefinition["nodes"][number]) {
  if (node.type !== "llm_reply") return false
  const config = (node.config ?? {}) as Record<string, unknown>
  return typeof config.staticReply === "string" && config.staticReply.trim() !== ""
}

function hasDynamicReplyNode(definition: ProductWorkflowDefinition) {
  return definition.nodes.some((node) => node.type === "llm_reply" && !nodeHasStaticReply(node))
}

function hasDispatchActionNode(nodeTypes: Set<string>) {
  return hasAny(nodeTypes, ["prepare_ticket_draft", "human_confirm", "create_ticket", "handoff_to_human", "create_video_meeting", "create_knowledge_candidate"])
}

function conditionBranches(definition: ProductWorkflowDefinition, nodeId: string) {
  return definition.nodes.find((node) => node.id === nodeId)?.config?.branches ?? []
}

function pathToNode(definition: ProductWorkflowDefinition, targetNodeId: string) {
  const outgoing = new Map<string, Array<{ source: string; target: string }>>()
  definition.edges.forEach((edge) => outgoing.set(edge.source, [...(outgoing.get(edge.source) ?? []), edge]))
  const visit = (nodeId: string, visited: Set<string>): Array<{ source: string; target: string }> | null => {
    if (nodeId === targetNodeId) return []
    if (visited.has(nodeId)) return null
    const nextVisited = new Set(visited).add(nodeId)
    for (const edge of outgoing.get(nodeId) ?? []) {
      const tail = visit(edge.target, nextVisited)
      if (tail) return [edge, ...tail]
    }
    return null
  }
  return visit(definition.entryNodeId, new Set())
}

function branchOverridesForPath(
  definition: ProductWorkflowDefinition,
  path: Array<{ source: string; target: string }>,
) {
  const overrides: Record<string, string> = {}
  path.forEach((edge) => {
    const branch = conditionBranches(definition, edge.source)
      .find((item) => item.targetNodeId === edge.target && item.id)
    if (branch?.id) overrides[edge.source] = branch.id
  })
  return overrides
}

export function getWorkflowTestScenarios(definition: ProductWorkflowDefinition): WorkflowTestScenario[] {
  const nodeById = new Map(definition.nodes.map((node) => [node.id, node]))
  const scenarios: WorkflowTestScenario[] = [{
    key: "actual",
    title: pt("scenarios.actual"),
    branchOverrides: {},
    runtimeContext: {},
    autoConfirm: false,
  }]
  const quickAINode = definition.nodes.find((node) => node.id === "quick_retrieve_1")
    ?? definition.nodes.find((node) => /quick|自助/.test(`${node.id} ${node.name ?? ""}`) && node.type === "knowledge_retrieve")
  if (quickAINode) {
    const path = pathToNode(definition, quickAINode.id)
    if (path) {
      scenarios.push({
        key: "quick_ai_question",
        title: pt("scenarios.quickAiQuestion.title"),
        message: pt("scenarios.quickAiQuestion.message"),
        branchOverrides: {},
        runtimeContext: { serviceMode: "ai_first" },
        autoConfirm: false,
      })
    }
  }
  const deviceDiagnosisNode = definition.nodes.find((node) => node.id === "device_retrieve_1")
    ?? definition.nodes.find((node) => /device|设备/.test(`${node.id} ${node.name ?? ""}`) && node.type === "knowledge_retrieve")
  if (deviceDiagnosisNode && definition.nodes.some((node) => node.type === "answerability_gate")) {
    const path = pathToNode(definition, deviceDiagnosisNode.id)
    if (path) {
      scenarios.push({
        key: "bound_device_diagnosis",
        title: pt("scenarios.boundDeviceDiagnosis.title"),
        message: pt("scenarios.boundDeviceDiagnosis.message"),
        branchOverrides: {},
        runtimeContext: { deviceBound: true, serviceMode: "ai_only" },
        autoConfirm: false,
      })
    }
  }
  const conditionNodes = definition.nodes.filter((node) => node.type === "condition")
  const fallbackCondition = conditionNodes.find((node) => {
    const targetTypes = conditionBranches(definition, node.id)
      .map((branch) => nodeById.get(branch.targetNodeId ?? "")?.type)
    return targetTypes.includes("handoff_to_human") && targetTypes.includes("llm_reply")
  })
  if (fallbackCondition) {
    const handoffBranch = conditionBranches(definition, fallbackCondition.id)
      .find((branch) => nodeById.get(branch.targetNodeId ?? "")?.type === "handoff_to_human" && branch.id)
    const path = pathToNode(definition, fallbackCondition.id)
    if (handoffBranch?.id && path) {
      scenarios.push({
        key: "ai_unanswerable_handoff",
        title: pt("scenarios.aiUnanswerableHandoff.title"),
        message: pt("scenarios.aiUnanswerableHandoff.message"),
        branchOverrides: {
          ...branchOverridesForPath(definition, path),
          [fallbackCondition.id]: handoffBranch.id,
        },
        runtimeContext: { deviceBound: true, serviceMode: "ai_first" },
        autoConfirm: true,
      })
    }
  }

  const directHandoffCandidates = conditionNodes
    .filter((node) => node.id !== fallbackCondition?.id)
    .map((node) => ({
      node,
      branch: conditionBranches(definition, node.id).find((branch) => (
        nodeById.get(branch.targetNodeId ?? "")?.type === "handoff_to_human"
        && branch.id
        && /handoff|人工/.test(`${branch.id} ${branch.name ?? ""}`)
      )),
    }))
    .filter((item) => item.branch)
  const directHandoff = directHandoffCandidates.find((item) => /handoff/.test(item.branch?.id ?? ""))
    ?? directHandoffCandidates[0]
  if (directHandoff?.branch?.id) {
    const path = pathToNode(definition, directHandoff.node.id)
    if (path) {
      scenarios.push({
        key: "customer_requests_handoff",
        title: pt("scenarios.customerRequestsHandoff.title"),
        message: pt("scenarios.customerRequestsHandoff.message"),
        branchOverrides: {
          ...branchOverridesForPath(definition, path),
          [directHandoff.node.id]: directHandoff.branch.id,
        },
        runtimeContext: { deviceBound: true, serviceMode: "ai_first" },
        autoConfirm: true,
      })
    }
  }

  const createTicketNode = definition.nodes.find((node) => node.type === "create_ticket")
  if (createTicketNode) {
    const path = pathToNode(definition, createTicketNode.id)
    if (path) {
      scenarios.push({
        key: "confirmed_ticket",
        title: pt("scenarios.confirmedTicket.title"),
        message: pt("scenarios.confirmedTicket.message"),
        branchOverrides: branchOverridesForPath(definition, path),
        runtimeContext: { deviceBound: true, serviceMode: "ai_first" },
        autoConfirm: true,
      })
    }
  }
  return scenarios
}

export function getWorkflowProductProfile(definition: ProductWorkflowDefinition): WorkflowProductProfile {
  const nodeTypeSet = new Set(definition.nodes.map((node) => node.type))
  const knowledgeSupport = isKnowledgeSupportWorkflowDefinition(definition)
  const humanHandoff = nodeTypeSet.has("handoff_to_human")
  const fullDeviceDiagnosis = nodeTypeSet.has("entry_context") && nodeTypeSet.has("answerability_gate")
  const dynamicReply = hasDynamicReplyNode(definition)
  const dispatchOnly = !dynamicReply && hasDispatchActionNode(nodeTypeSet)
  const mode = dispatchOnly
    ? "dispatch_only"
    : humanHandoff
      ? "ai_human"
      : "ai_only"

  const journey = journeyCatalog
    .filter((stage) => hasAny(nodeTypeSet, stage.nodeTypes))
    .map((stage) => ({
      key: stage.key,
      title: knowledgeSupport && stage.key === "knowledge" ? pt("journeyStages.tenantKnowledge") : pt(stage.titleKey),
      nodeTypes: stage.nodeTypes,
    }))
  const capabilities: WorkflowProductCapability[] = [
    {
      key: "knowledge",
      title: pt(knowledgeSupport ? "capabilities.tenantKnowledge.title" : "capabilities.knowledge.title"),
      state: nodeTypeSet.has("knowledge_retrieve") ? "enabled" : "disabled",
      stateLabel: nodeTypeSet.has("knowledge_retrieve") ? pt("capabilities.stateEnabled") : pt("capabilities.stateDisabled"),
    },
    {
      key: "ticket",
      title: pt("capabilities.ticket.title"),
      state: nodeTypeSet.has("create_ticket") ? "enabled" : "disabled",
      stateLabel: nodeTypeSet.has("create_ticket") ? pt("capabilities.stateEnabled") : pt("capabilities.stateDisabled"),
    },
    {
      key: "handoff",
      title: pt("capabilities.handoff.title"),
      state: humanHandoff ? "enabled" : "prohibited",
      stateLabel: humanHandoff ? pt("capabilities.stateEnabled") : pt("capabilities.stateProhibited"),
    },
    {
      key: "video",
      title: pt("capabilities.video.title"),
      state: nodeTypeSet.has("create_video_meeting") ? "enabled" : "disabled",
      stateLabel: nodeTypeSet.has("create_video_meeting") ? pt("capabilities.stateEnabled") : pt("capabilities.stateDisabled"),
    },
    {
      key: "learning",
      title: pt("capabilities.learning.title"),
      state: nodeTypeSet.has("create_knowledge_candidate") ? "enabled" : "disabled",
      stateLabel: nodeTypeSet.has("create_knowledge_candidate") ? pt("capabilities.stateEnabled") : pt("capabilities.stateDisabled"),
    },
  ]

  return {
    mode,
    title: knowledgeSupport && humanHandoff
      ? pt("serviceModes.knowledgeHuman")
      : dispatchOnly
      ? pt("serviceModes.dispatchOnly")
      : humanHandoff
        ? pt("serviceModes.aiHuman")
        : fullDeviceDiagnosis
        ? pt("serviceModes.deviceDiagnosis")
        : pt("serviceModes.lightProductQa"),
    journey,
    capabilities,
    nodeTypeSet,
  }
}

export function getWorkflowServiceBlueprint(definition: ProductWorkflowDefinition): WorkflowServiceBlueprint {
  const profile = getWorkflowProductProfile(definition)
  if (profile.mode === "dispatch_only") return "dispatch_only"
  if (profile.mode === "ai_human") return "ai_human"
  return profile.nodeTypeSet.has("entry_context") && profile.nodeTypeSet.has("answerability_gate")
    ? "device_ai"
    : "basic_ai"
}

function canonicalWorkflowValue(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(canonicalWorkflowValue)
  if (!value || typeof value !== "object") return value

  return Object.keys(value as Record<string, unknown>)
    .sort()
    .reduce<Record<string, unknown>>((result, key) => {
      const item = (value as Record<string, unknown>)[key]
      if (item !== undefined) result[key] = canonicalWorkflowValue(item)
      return result
    }, {})
}

function workflowBehaviorSnapshot(definition: ProductWorkflowDefinition) {
  const nodes = definition.nodes
    .map((node) => ({
      id: node.id,
      type: node.type,
      config: node.config ?? {},
      inputs: node.inputs ?? {},
      errorTargetNodeId: node.errorTargetNodeId ?? "",
    }))
    .sort((left, right) => left.id.localeCompare(right.id))
  const edges = definition.edges
    .map((edge) => ({ source: edge.source, target: edge.target }))
    .sort((left, right) => `${left.source}>${left.target}`.localeCompare(`${right.source}>${right.target}`))
  const runtimeVariables = [...(definition.runtimeVariables ?? [])]
    .sort((left, right) => String(left.key ?? "").localeCompare(String(right.key ?? "")))

  return canonicalWorkflowValue({
    schemaVersion: definition.schemaVersion ?? 1,
    entryNodeId: definition.entryNodeId,
    modelPolicy: definition.modelPolicy ?? {},
    runtimeVariables,
    nodes,
    edges,
  })
}

export function isWorkflowDefinitionFunctionallyEquivalent(
  current: ProductWorkflowDefinition,
  standard: ProductWorkflowDefinition,
) {
  return JSON.stringify(workflowBehaviorSnapshot(current)) === JSON.stringify(workflowBehaviorSnapshot(standard))
}

export function getWorkflowReadiness(
  definition: ProductWorkflowDefinition,
  stableVersionId: number,
  adoptionCount: number,
): WorkflowReadinessCheck[] {
  const profile = getWorkflowProductProfile(definition)
  const knowledgeSupport = isKnowledgeSupportWorkflowDefinition(definition)
  const entry = definition.nodes.find((node) => node.id === definition.entryNodeId)
  const hasReply = profile.nodeTypeSet.has("send_reply")
  const hasEnd = profile.nodeTypeSet.has("end")
  const hasGeneratedReply = hasDynamicReplyNode(definition)
  const hasDispatchActions = hasDispatchActionNode(profile.nodeTypeSet)
  const hasKnowledge = profile.nodeTypeSet.has("knowledge_retrieve")

  return [
    {
      key: "structure",
      title: pt("checks.structure"),
      state: entry?.type === "start" && hasReply && hasEnd ? "pass" : "error",
    },
    {
      key: "ai",
      title: profile.mode === "dispatch_only" ? pt("checks.dispatchCapability") : pt("checks.replyCapability"),
      state: profile.mode === "dispatch_only"
        ? hasDispatchActions ? "pass" : "error"
        : hasGeneratedReply ? "pass" : "error",
    },
    {
      key: "knowledge",
      title: profile.mode === "dispatch_only" ? pt("checks.dispatchScript") : pt(knowledgeSupport ? "checks.tenantKnowledgeBasis" : "checks.knowledgeBasis"),
      state: profile.mode === "dispatch_only" || hasKnowledge ? "pass" : "warning",
    },
    {
      key: "version",
      title: pt("checks.version"),
      state: stableVersionId > 0 ? "pass" : "error",
    },
    {
      key: "adoption",
      title: pt(knowledgeSupport ? "checks.serviceConfig" : "checks.adoption"),
      state: adoptionCount > 0 ? "pass" : "warning",
    },
  ]
}

export function summarizeVersionChange(
  current: ProductWorkflowDefinition,
  previous?: ProductWorkflowDefinition,
) {
  const currentNodeById = new Map(current.nodes.map((node) => [node.id, node]))
  const previousNodeById = new Map((previous?.nodes ?? []).map((node) => [node.id, node]))
  const presentNode = (node: ProductWorkflowDefinition["nodes"][number]) => ({
    id: node.id,
    title: node.name || node.id,
    type: node.type,
  })
  const edgeKey = (edge: ProductWorkflowDefinition["edges"][number]) => `${edge.source}>${edge.target}`
  const presentEdge = (
    edge: ProductWorkflowDefinition["edges"][number],
    nodes: Map<string, ProductWorkflowDefinition["nodes"][number]>,
  ) => ({
    key: edgeKey(edge),
    from: nodes.get(edge.source)?.name || edge.source,
    to: nodes.get(edge.target)?.name || edge.target,
  })
  const currentEdgeKeys = new Set(current.edges.map(edgeKey))
  const previousEdgeKeys = new Set((previous?.edges ?? []).map(edgeKey))
  const changedNodes = previous
    ? current.nodes.filter((node) => {
      const before = previousNodeById.get(node.id)
      if (!before) return false
      return JSON.stringify(canonicalWorkflowValue({
        type: node.type,
        config: node.config ?? {},
        inputs: node.inputs ?? {},
        errorTargetNodeId: node.errorTargetNodeId ?? "",
      })) !== JSON.stringify(canonicalWorkflowValue({
        type: before.type,
        config: before.config ?? {},
        inputs: before.inputs ?? {},
        errorTargetNodeId: before.errorTargetNodeId ?? "",
      }))
    })
    : []
  const currentVariables = new Set((current.runtimeVariables ?? []).map((item) => String(item.key ?? "")).filter(Boolean))
  const previousVariables = new Set((previous?.runtimeVariables ?? []).map((item) => String(item.key ?? "")).filter(Boolean))
  const currentCredential = describeWorkflowModelCredentialChain(current.modelPolicy?.credentialChain)
  const previousCredential = describeWorkflowModelCredentialChain(previous?.modelPolicy?.credentialChain)
  const credentialChanged = Boolean(previous && currentCredential.label !== previousCredential.label)

  if (!previous) {
    return {
      nodeDelta: current.nodes.length,
      edgeDelta: current.edges.length,
      addedCapabilities: getWorkflowProductProfile(current).capabilities
        .filter((item) => item.state === "enabled")
        .map((item) => item.title),
      removedCapabilities: [] as string[],
      addedNodes: current.nodes.map(presentNode),
      removedNodes: [] as ReturnType<typeof presentNode>[],
      changedNodes: [] as ReturnType<typeof presentNode>[],
      addedEdges: current.edges.map((edge) => presentEdge(edge, currentNodeById)),
      removedEdges: [] as ReturnType<typeof presentEdge>[],
      addedVariables: [...currentVariables],
      removedVariables: [] as string[],
      credentialChange: null as { from: string; to: string } | null,
    }
  }

  const currentProfile = getWorkflowProductProfile(current)
  const previousProfile = getWorkflowProductProfile(previous)
  const currentEnabled = new Set(currentProfile.capabilities.filter((item) => item.state === "enabled").map((item) => item.key))
  const previousEnabled = new Set(previousProfile.capabilities.filter((item) => item.state === "enabled").map((item) => item.key))

  return {
    nodeDelta: current.nodes.length - previous.nodes.length,
    edgeDelta: current.edges.length - previous.edges.length,
    addedCapabilities: currentProfile.capabilities
      .filter((item) => currentEnabled.has(item.key) && !previousEnabled.has(item.key))
      .map((item) => item.title),
    removedCapabilities: previousProfile.capabilities
      .filter((item) => previousEnabled.has(item.key) && !currentEnabled.has(item.key))
      .map((item) => item.title),
    addedNodes: current.nodes.filter((node) => !previousNodeById.has(node.id)).map(presentNode),
    removedNodes: previous.nodes.filter((node) => !currentNodeById.has(node.id)).map(presentNode),
    changedNodes: changedNodes.map(presentNode),
    addedEdges: current.edges.filter((edge) => !previousEdgeKeys.has(edgeKey(edge))).map((edge) => presentEdge(edge, currentNodeById)),
    removedEdges: previous.edges.filter((edge) => !currentEdgeKeys.has(edgeKey(edge))).map((edge) => presentEdge(edge, previousNodeById)),
    addedVariables: [...currentVariables].filter((key) => !previousVariables.has(key)),
    removedVariables: [...previousVariables].filter((key) => !currentVariables.has(key)),
    credentialChange: credentialChanged ? { from: previousCredential.label, to: currentCredential.label } : null,
  }
}
