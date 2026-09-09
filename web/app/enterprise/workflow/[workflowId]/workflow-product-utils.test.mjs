import assert from "node:assert/strict"
import { describe, it } from "node:test"
import { readFile } from "node:fs/promises"
import vm from "node:vm"
import ts from "typescript"

const utilsSource = await readFile(new URL("./workflow-product-utils.ts", import.meta.url), "utf8")

function plain(value) {
  return JSON.parse(JSON.stringify(value))
}

async function loadModule() {
  const source = await readFile(new URL("./workflow-product-utils.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2020,
      module: ts.ModuleKind.CommonJS,
    },
  })
  const sandbox = {
    exports: {},
    module: { exports: {} },
    require: (id) => {
      if (id === "@/i18n/messages") {
        // The module translates on every call with the current locale; in tests the
        // identity stub makes assertions pin the exact message keys used by the source.
        return { translateCurrentMessage: (key) => key }
      }
      throw new Error(`unexpected import ${id}`)
    },
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

const aiOnlyDefinition = {
  entryNodeId: "start_1",
  nodes: [
    { id: "start_1", type: "start" },
    { id: "understand_1", type: "conversation_understanding" },
    { id: "knowledge_1", type: "knowledge_retrieve" },
    { id: "llm_1", type: "llm_reply" },
    { id: "reply_1", type: "send_reply" },
    { id: "end_1", type: "end" },
  ],
  edges: [
    { source: "start_1", target: "understand_1" },
    { source: "understand_1", target: "knowledge_1" },
    { source: "knowledge_1", target: "llm_1" },
    { source: "llm_1", target: "reply_1" },
    { source: "reply_1", target: "end_1" },
  ],
}

const dispatchOnlyDefinition = {
  entryNodeId: "start_1",
  nodes: [
    { id: "start_1", type: "start" },
    { id: "ticket_draft_1", type: "prepare_ticket_draft" },
    { id: "ticket_confirm_1", type: "human_confirm" },
    { id: "create_ticket_1", type: "create_ticket" },
    { id: "handoff_1", type: "handoff_to_human" },
    { id: "fallback_reply_1", type: "llm_reply", config: { staticReply: "我不会做 AI 问诊。请直接说明你要创建工单、转人工，或补充故障现象。" } },
    { id: "fallback_send_1", type: "send_reply" },
    { id: "end_1", type: "end" },
  ],
  edges: [
    { source: "start_1", target: "ticket_draft_1" },
    { source: "ticket_draft_1", target: "ticket_confirm_1" },
    { source: "ticket_confirm_1", target: "create_ticket_1" },
    { source: "create_ticket_1", target: "fallback_send_1" },
    { source: "fallback_send_1", target: "end_1" },
    { source: "handoff_1", target: "fallback_send_1" },
    { source: "start_1", target: "handoff_1" },
    { source: "start_1", target: "fallback_reply_1" },
    { source: "fallback_reply_1", target: "fallback_send_1" },
  ],
}

describe("getWorkflowProductProfile", () => {
  it("keeps workflow display data free of unused description copy", () => {
    assert.doesNotMatch(utilsSource, /\bdescription:/)
    assert.doesNotMatch(utilsSource, /\bdescription: string/)
    assert.doesNotMatch(utilsSource, /使用当前输入和接待配置/)
    assert.doesNotMatch(utilsSource, /验证.*完整/)
    assert.doesNotMatch(utilsSource, /当前流程不会自动/)
    assert.doesNotMatch(utilsSource, /自动完成设备诊断/)
    assert.doesNotMatch(utilsSource, /适合标准化/)
    assert.doesNotMatch(utilsSource, /流程包含生成回复节点/)
    assert.doesNotMatch(utilsSource, /存在可供产品配置/)
    assert.doesNotMatch(utilsSource, /知识沉淀/)
  })

  it("presents a lightweight automated workflow as a customer journey", async () => {
    const { getWorkflowProductProfile } = await loadModule()
    const profile = getWorkflowProductProfile(aiOnlyDefinition)

    assert.equal(profile.mode, "ai_only")
    assert.equal(profile.title, "workflowExtract.productUtils.serviceModes.lightProductQa")
    assert.deepEqual(
      Array.from(profile.journey, (item) => item.title),
      [
        "workflowExtract.productUtils.journeyStages.receive",
        "workflowExtract.productUtils.journeyStages.understand",
        "workflowExtract.productUtils.journeyStages.knowledge",
        "workflowExtract.productUtils.journeyStages.answer",
        "workflowExtract.productUtils.journeyStages.respond",
      ],
    )
    assert.equal(profile.capabilities.find((item) => item.key === "handoff").state, "prohibited")
    assert.equal(profile.capabilities.find((item) => item.key === "ticket").state, "disabled")
    assert.equal(profile.capabilities.find((item) => item.key === "learning").title, "workflowExtract.productUtils.capabilities.learning.title")
  })

  it("presents a dispatch-only workflow as a non-AI service flow", async () => {
    const { getWorkflowProductProfile } = await loadModule()
    const profile = getWorkflowProductProfile(dispatchOnlyDefinition)

    assert.equal(profile.mode, "dispatch_only")
    assert.equal(profile.title, "workflowExtract.productUtils.serviceModes.dispatchOnly")
    assert.equal(profile.capabilities.find((item) => item.key === "handoff").state, "enabled")
    assert.equal(profile.capabilities.find((item) => item.key === "ticket").state, "enabled")
  })

  it("detects automated and human collaboration from actual workflow nodes", async () => {
    const { getWorkflowProductProfile } = await loadModule()
    const definition = {
      ...aiOnlyDefinition,
      nodes: [...aiOnlyDefinition.nodes, { id: "handoff_1", type: "handoff_to_human" }],
    }
    const profile = getWorkflowProductProfile(definition)

    assert.equal(profile.mode, "ai_human")
    assert.equal(profile.title, "workflowExtract.productUtils.serviceModes.aiHuman")
    assert.equal(profile.capabilities.find((item) => item.key === "handoff").state, "enabled")
  })

  it("presents device-aware service as intelligent diagnosis", async () => {
    const { getWorkflowProductProfile } = await loadModule()
    const definition = {
      ...aiOnlyDefinition,
      nodes: [
        ...aiOnlyDefinition.nodes,
        { id: "entry_context_1", type: "entry_context" },
        { id: "answerability_1", type: "answerability_gate" },
      ],
    }
    const profile = getWorkflowProductProfile(definition)

    assert.equal(profile.mode, "ai_only")
    assert.equal(profile.title, "workflowExtract.productUtils.serviceModes.deviceDiagnosis")
    assert.equal(profile.capabilities.find((item) => item.key === "handoff").state, "prohibited")
  })
})

describe("getWorkflowServiceBlueprint", () => {
  it("derives reusable business blueprints from topology", async () => {
    const { getWorkflowServiceBlueprint } = await loadModule()
    assert.equal(getWorkflowServiceBlueprint(aiOnlyDefinition), "basic_ai")
    assert.equal(getWorkflowServiceBlueprint(dispatchOnlyDefinition), "dispatch_only")
    assert.equal(getWorkflowServiceBlueprint({
      ...aiOnlyDefinition,
      nodes: [...aiOnlyDefinition.nodes, { id: "entry_context_1", type: "entry_context" }, { id: "gate_1", type: "answerability_gate" }],
    }), "device_ai")
    assert.equal(getWorkflowServiceBlueprint({
      ...aiOnlyDefinition,
      nodes: [...aiOnlyDefinition.nodes, { id: "handoff_1", type: "handoff_to_human" }],
    }), "ai_human")
  })

  it("distinguishes a platform standard from a customized workflow of the same business type", async () => {
    const { isWorkflowDefinitionFunctionallyEquivalent } = await loadModule()
    const standard = {
      ...aiOnlyDefinition,
      schemaVersion: 1,
      modelPolicy: { credentialChain: ["product", "tenant_default"] },
      nodes: aiOnlyDefinition.nodes.map((node, index) => ({
        ...node,
        name: `节点 ${index + 1}`,
        position: { x: 100, y: index * 120 },
        config: {},
      })),
    }
    const layoutOnlyChange = {
      ...standard,
      nodes: standard.nodes.map((node) => ({ ...node, name: `自定义名称 ${node.id}`, position: { x: 900, y: 900 } })),
      edges: [...standard.edges].reverse(),
    }
    const behaviorChange = {
      ...layoutOnlyChange,
      modelPolicy: { credentialChain: ["tenant_default"] },
    }

    assert.equal(isWorkflowDefinitionFunctionallyEquivalent(layoutOnlyChange, standard), true)
    assert.equal(isWorkflowDefinitionFunctionallyEquivalent(behaviorChange, standard), false)
  })
})

describe("workflow model credential policy", () => {
  it("maps ordered credential chains without relying on workflow codes", async () => {
    const {
      describeWorkflowModelCredentialChain,
      getWorkflowModelCredentialChain,
      getWorkflowModelCredentialPreset,
    } = await loadModule()

    assert.equal(getWorkflowModelCredentialPreset(["product", "tenant_default"]), "product_then_tenant")
    assert.equal(getWorkflowModelCredentialPreset(["tenant_default", "product"]), "tenant_then_product")
    assert.deepEqual(plain(getWorkflowModelCredentialChain("tenant_only")), ["tenant_default"])
    assert.equal(
      describeWorkflowModelCredentialChain(["product", "tenant_default"]).label,
      "workflowExtract.productUtils.credentialChain.product → workflowExtract.productUtils.credentialChain.tenantDefault",
    )
  })
})

describe("getWorkflowTestScenarios", () => {
  it("keeps device-context scenarios on real condition evaluation", async () => {
    const { getWorkflowTestScenarios } = await loadModule()
    const definition = {
      entryNodeId: "start_1",
      nodes: [
        { id: "start_1", type: "start" },
        { id: "entry_context_1", type: "entry_context" },
        { id: "context_route_1", type: "condition", config: { branches: [
          { id: "device", targetNodeId: "device_retrieve_1" },
          { id: "default", targetNodeId: "quick_retrieve_1", default: true },
        ] } },
        { id: "quick_retrieve_1", type: "knowledge_retrieve" },
        { id: "device_retrieve_1", type: "knowledge_retrieve" },
        { id: "answerability_1", type: "answerability_gate" },
        { id: "end_1", type: "end" },
      ],
      edges: [
        { source: "start_1", target: "entry_context_1" },
        { source: "entry_context_1", target: "context_route_1" },
        { source: "context_route_1", target: "quick_retrieve_1" },
        { source: "context_route_1", target: "device_retrieve_1" },
        { source: "quick_retrieve_1", target: "end_1" },
        { source: "device_retrieve_1", target: "answerability_1" },
        { source: "answerability_1", target: "end_1" },
      ],
    }

    const scenarios = getWorkflowTestScenarios(definition)
    const unbound = scenarios.find((item) => item.key === "quick_ai_question")
    const bound = scenarios.find((item) => item.key === "bound_device_diagnosis")
    assert.deepEqual(plain(unbound.branchOverrides), {})
    assert.equal(unbound.runtimeContext.deviceId, undefined)
    assert.deepEqual(plain(bound.branchOverrides), {})
    assert.equal(bound.runtimeContext.deviceBound, true)
    assert.equal(bound.runtimeContext.deviceId, undefined)
  })

  it("derives business test scenarios from topology instead of fixed node ids", async () => {
    const { getWorkflowTestScenarios } = await loadModule()
    const definition = {
      entryNodeId: "entry_custom",
      nodes: [
        { id: "entry_custom", type: "start" },
        { id: "policy_custom", type: "condition", config: { branches: [
          { id: "knowledge_path", targetNodeId: "retrieve_custom" },
          { id: "human_now", name: "转人工", targetNodeId: "handoff_custom" },
          { id: "ticket_path", targetNodeId: "confirm_custom" },
          { id: "default", targetNodeId: "retrieve_custom", default: true },
        ] } },
        { id: "retrieve_custom", type: "knowledge_retrieve" },
        { id: "answer_route_custom", type: "condition", config: { branches: [
          { id: "answer", targetNodeId: "llm_custom" },
          { id: "fallback_human", name: "转人工处理", targetNodeId: "handoff_custom", default: true },
        ] } },
        { id: "llm_custom", type: "llm_reply" },
        { id: "confirm_custom", type: "condition", config: { branches: [
          { id: "confirmed", targetNodeId: "create_ticket_custom" },
          { id: "cancelled", targetNodeId: "end_custom", default: true },
        ] } },
        { id: "create_ticket_custom", type: "create_ticket" },
        { id: "handoff_custom", type: "handoff_to_human" },
        { id: "end_custom", type: "end" },
      ],
      edges: [
        { source: "entry_custom", target: "policy_custom" },
        { source: "policy_custom", target: "retrieve_custom" },
        { source: "policy_custom", target: "handoff_custom" },
        { source: "policy_custom", target: "confirm_custom" },
        { source: "retrieve_custom", target: "answer_route_custom" },
        { source: "answer_route_custom", target: "llm_custom" },
        { source: "answer_route_custom", target: "handoff_custom" },
        { source: "confirm_custom", target: "create_ticket_custom" },
        { source: "confirm_custom", target: "end_custom" },
        { source: "create_ticket_custom", target: "end_custom" },
        { source: "llm_custom", target: "end_custom" },
        { source: "handoff_custom", target: "end_custom" },
      ],
    }

    const scenarios = getWorkflowTestScenarios(definition)
    const fallback = scenarios.find((item) => item.key === "ai_unanswerable_handoff")
    assert.deepEqual(plain(fallback.branchOverrides), {
      policy_custom: "knowledge_path",
      answer_route_custom: "fallback_human",
    })
    const direct = scenarios.find((item) => item.key === "customer_requests_handoff")
    assert.deepEqual(plain(direct.branchOverrides), { policy_custom: "human_now" })
    assert.equal(direct.runtimeContext.deviceBound, true)
    const ticket = scenarios.find((item) => item.key === "confirmed_ticket")
    assert.deepEqual(plain(ticket.branchOverrides), {
      policy_custom: "ticket_path",
      confirm_custom: "confirmed",
    })
  })
})

describe("getWorkflowReadiness", () => {
  it("marks a published and adopted workflow ready", async () => {
    const { getWorkflowReadiness } = await loadModule()
    const checks = getWorkflowReadiness(aiOnlyDefinition, 12, 3)

    assert.equal(checks.every((item) => item.state === "pass"), true)
  })

  it("marks dispatch-only workflow ready without knowledge retrieval", async () => {
    const { getWorkflowReadiness } = await loadModule()
    const checks = getWorkflowReadiness(dispatchOnlyDefinition, 12, 3)

    assert.equal(checks.every((item) => item.state === "pass"), true)
    assert.equal(checks.find((item) => item.key === "knowledge").title, "workflowExtract.productUtils.checks.dispatchScript")
  })

  it("exposes blocking structure and publishing errors", async () => {
    const { getWorkflowReadiness } = await loadModule()
    const checks = getWorkflowReadiness({ entryNodeId: "missing", nodes: [], edges: [] }, 0, 0)

    assert.equal(checks.find((item) => item.key === "structure").state, "error")
    assert.equal(checks.find((item) => item.key === "version").state, "error")
    assert.equal(checks.find((item) => item.key === "adoption").state, "warning")
  })
})

describe("summarizeVersionChange", () => {
  it("describes business capability changes instead of database ids", async () => {
    const { summarizeVersionChange } = await loadModule()
    const next = {
      ...aiOnlyDefinition,
      nodes: [
        ...aiOnlyDefinition.nodes,
        { id: "ticket_1", type: "create_ticket" },
        { id: "handoff_1", type: "handoff_to_human" },
      ],
    }
    const summary = summarizeVersionChange(next, aiOnlyDefinition)

    assert.equal(summary.nodeDelta, 2)
    assert.deepEqual(Array.from(summary.addedCapabilities), [
      "workflowExtract.productUtils.capabilities.ticket.title",
      "workflowExtract.productUtils.capabilities.handoff.title",
    ])
    assert.deepEqual(Array.from(summary.removedCapabilities), [])
  })

  it("shows node, variable and credential changes for version review", async () => {
    const { summarizeVersionChange } = await loadModule()
    const previous = {
      ...aiOnlyDefinition,
      modelPolicy: { credentialChain: ["tenant_default"] },
      runtimeVariables: [{ key: "context.productId" }, { key: "context.legacy" }],
      nodes: aiOnlyDefinition.nodes.map((node) => ({ ...node, config: {} })),
    }
    const current = {
      ...previous,
      modelPolicy: { credentialChain: ["product", "tenant_default"] },
      runtimeVariables: [{ key: "context.productId" }, { key: "context.deviceId" }],
      nodes: [
        ...previous.nodes.map((node) => node.id === "llm_1" ? { ...node, config: { prompt: "new" } } : node),
        { id: "ticket_1", type: "create_ticket", name: "创建工单", config: {} },
      ],
    }
    const summary = summarizeVersionChange(current, previous)

    assert.deepEqual(Array.from(summary.addedNodes, (item) => item.id), ["ticket_1"])
    assert.deepEqual(Array.from(summary.changedNodes, (item) => item.id), ["llm_1"])
    assert.deepEqual(Array.from(summary.addedVariables), ["context.deviceId"])
    assert.deepEqual(Array.from(summary.removedVariables), ["context.legacy"])
    assert.deepEqual(plain(summary.credentialChange), {
      from: "workflowExtract.productUtils.credentialChain.tenantDefault",
      to: "workflowExtract.productUtils.credentialChain.product → workflowExtract.productUtils.credentialChain.tenantDefault",
    })
  })

  it("shows routing changes even when all nodes stay unchanged", async () => {
    const { summarizeVersionChange } = await loadModule()
    const current = {
      ...aiOnlyDefinition,
      edges: aiOnlyDefinition.edges.map((edge) => (
        edge.source === "knowledge_1"
          ? { source: "knowledge_1", target: "reply_1" }
          : edge
      )),
    }
    const summary = summarizeVersionChange(current, aiOnlyDefinition)

    assert.equal(summary.nodeDelta, 0)
    assert.deepEqual(Array.from(summary.changedNodes), [])
    assert.deepEqual(plain(summary.addedEdges), [{ key: "knowledge_1>reply_1", from: "knowledge_1", to: "reply_1" }])
    assert.deepEqual(plain(summary.removedEdges), [{ key: "knowledge_1>llm_1", from: "knowledge_1", to: "llm_1" }])
  })
})

describe("describeWorkflowRunError", () => {
  it("turns vector provider details into an actionable product message", async () => {
    const { describeWorkflowRunError } = await loadModule()
    const raw = "failed to search collection knowledge_chunks_active: Vector dimension error: expected dim: 8, got 1024"
    const presentation = describeWorkflowRunError(raw)

    assert.equal(presentation.summary, "workflowExtract.productUtils.runErrors.incompatibleVector")
    assert.equal(presentation.technicalDetails, raw)
  })

  it("keeps unknown workflow errors visible", async () => {
    const { describeWorkflowRunError } = await loadModule()
    const presentation = describeWorkflowRunError("custom node failed")

    assert.equal(presentation.summary, "custom node failed")
    assert.equal(presentation.technicalDetails, "")
  })

  it("turns missing workflow credentials into an administrator action", async () => {
    const { describeWorkflowRunError } = await loadModule()
    const raw = "resolve platform LLM capability: workflow model credential policy has no available credential"
    const presentation = describeWorkflowRunError(raw)

    assert.equal(presentation.summary, "workflowExtract.productUtils.runErrors.noCredential")
    assert.equal(presentation.technicalDetails, raw)
  })
})
