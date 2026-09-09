import assert from "node:assert/strict"
import { describe, it } from "node:test"
import ts from "typescript"
import { readFile } from "node:fs/promises"
import vm from "node:vm"

function plain(value) {
  return JSON.parse(JSON.stringify(value))
}

async function loadModule() {
  const source = await readFile(new URL("./workflow-utils.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2017,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "workflow-utils.ts",
  })
  const sandbox = {
    exports: {},
    module: { exports: {} },
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

describe("validateWorkflowDraft", () => {
  it("rejects missing start", async () => {
    const { validateWorkflowDraft } = await loadModule()

    const result = validateWorkflowDraft({
      nodes: [{ id: "end_1", type: "end", position: { x: 0, y: 0 }, data: {} }],
      edges: [],
    })

    assert.equal(result.valid, false)
    assert.match(result.errors.join("\n"), /必须且只能包含一个开始节点/)
  })

  it("rejects dangling edge", async () => {
    const { validateWorkflowDraft } = await loadModule()

    const result = validateWorkflowDraft({
      nodes: [
        { id: "start_1", type: "start", position: { x: 0, y: 0 }, data: {} },
        { id: "end_1", type: "end", position: { x: 200, y: 0 }, data: {} },
      ],
      edges: [{ id: "e1", source: "start_1", target: "missing_1" }],
    })

    assert.equal(result.valid, false)
    assert.match(result.errors.join("\n"), /目标节点不存在/)
  })

  it("rejects missing required input mapping", async () => {
    const { validateWorkflowDraft } = await loadModule()

    const result = validateWorkflowDraft(
      {
        nodes: [
          { id: "start_1", type: "start", position: { x: 0, y: 0 }, data: {} },
          { id: "reply_1", type: "send_reply", position: { x: 200, y: 0 }, data: {} },
          { id: "end_1", type: "end", position: { x: 400, y: 0 }, data: {} },
        ],
        edges: [
          { id: "e1", source: "start_1", target: "reply_1" },
          { id: "e2", source: "reply_1", target: "end_1" },
        ],
      },
      [
        {
          type: "send_reply",
          inputSchema: [{ name: "replyText", type: "string", required: true }],
        },
      ]
    )

    assert.equal(result.valid, false)
    assert.match(result.errors.join("\n"), /缺少必填输入「回复内容」/)
  })

  it("requires a connected failure target", async () => {
    const { validateWorkflowDraft } = await loadModule()
    const result = validateWorkflowDraft({
      nodes: [
        { id: "start_1", type: "start", position: { x: 0, y: 0 }, data: { nodeType: "start" } },
        {
          id: "llm_1",
          type: "llm_reply",
          position: { x: 200, y: 0 },
          data: { nodeType: "llm_reply", name: "模型回答", errorTargetNodeId: "fallback_1" },
        },
        { id: "fallback_1", type: "end", position: { x: 400, y: 0 }, data: { nodeType: "end" } },
      ],
      edges: [{ id: "e1", source: "start_1", target: "llm_1" }],
    })

    assert.equal(result.valid, false)
    assert.match(result.errors.join("\n"), /失败出口需要连接到目标节点/)
  })
})

describe("applyAutoInputMappings", () => {
  it("maps start user message to knowledge retrieve query", async () => {
    const { applyAutoInputMappings } = await loadModule()

    const draft = applyAutoInputMappings(
      {
        nodes: [
          { id: "start_1", type: "start", position: { x: 0, y: 0 }, data: {} },
          { id: "retrieve_1", type: "knowledge_retrieve", position: { x: 200, y: 0 }, data: {} },
        ],
        edges: [{ id: "e1", source: "start_1", target: "retrieve_1" }],
      },
      "start_1",
      "retrieve_1",
      [
        {
          type: "start",
          outputSchema: [{ name: "userMessage", type: "string", description: "Message" }],
        },
        {
          type: "knowledge_retrieve",
          inputSchema: [{ name: "query", type: "string", required: true }],
        },
      ]
    )

    assert.deepEqual(plain(draft.nodes[1].data.inputs), {
      query: { nodeId: "start_1", field: "userMessage" },
    })
  })

  it("maps llm reply text to send reply content", async () => {
    const { applyAutoInputMappings } = await loadModule()

    const draft = applyAutoInputMappings(
      {
        nodes: [
          { id: "llm_1", type: "llm_reply", position: { x: 0, y: 0 }, data: {} },
          { id: "send_1", type: "send_reply", position: { x: 200, y: 0 }, data: {} },
        ],
        edges: [{ id: "e1", source: "llm_1", target: "send_1" }],
      },
      "llm_1",
      "send_1",
      [
        {
          type: "llm_reply",
          outputSchema: [{ name: "replyText", type: "string", description: "Reply" }],
        },
        {
          type: "send_reply",
          inputSchema: [{ name: "replyText", type: "string", required: true }],
        },
      ]
    )

    assert.deepEqual(plain(draft.nodes[1].data.inputs), {
      replyText: { nodeId: "llm_1", field: "replyText" },
    })
  })
})

describe("createWorkflowNodeFromSpec", () => {
  it("creates node at dropped canvas position with unique id", async () => {
    const { createWorkflowNodeFromSpec } = await loadModule()

    const node = createWorkflowNodeFromSpec(
      {
        type: "llm_reply",
        title: "生成回复",
        defaultInputs: {
          userMessage: { nodeId: "start_1", field: "userMessage" },
        },
      },
      [
        { id: "llm_reply_1", type: "workflowNode", position: { x: 0, y: 0 }, data: {} },
      ],
      { x: 120, y: 240 }
    )

    assert.deepEqual(plain(node), {
      id: "llm_reply_2",
      type: "workflowNode",
      position: { x: 120, y: 240 },
      data: {
        nodeType: "llm_reply",
        name: "生成回复",
        label: "生成回复",
        config: {},
        inputs: {
          userMessage: { nodeId: "start_1", field: "userMessage" },
        },
      },
    })
  })

  it("creates knowledge nodes with a logical binding method", async () => {
    const { createWorkflowNodeFromSpec } = await loadModule()

    const node = createWorkflowNodeFromSpec(
      { type: "knowledge_retrieve", title: "知识检索" },
      [],
      { x: 40, y: 60 }
    )

    assert.deepEqual(plain(node.data.config), {
      bindingMethod: "public_product",
      topK: 8,
      scoreThreshold: 0.3,
    })
  })
})

describe("calculateWorkflowHelperLines", () => {
  it("snaps dragged node to a nearby horizontal alignment", async () => {
    const { calculateWorkflowHelperLines } = await loadModule()

    const result = calculateWorkflowHelperLines(
      [
        { id: "start_1", position: { x: 100, y: 120 }, width: 220, height: 84 },
        { id: "reply_1", position: { x: 392, y: 124 }, width: 220, height: 84 },
      ],
      { id: "reply_1", position: { x: 392, y: 124 }, width: 220, height: 84 }
    )

    assert.deepEqual(plain(result), {
      position: { x: 392, y: 120 },
      horizontal: { y: 120, left: 100, width: 512 },
    })
  })

  it("does not show helper lines outside the alignment threshold", async () => {
    const { calculateWorkflowHelperLines } = await loadModule()

    const result = calculateWorkflowHelperLines(
      [
        { id: "start_1", position: { x: 100, y: 120 }, width: 220, height: 84 },
        { id: "reply_1", position: { x: 392, y: 132 }, width: 220, height: 84 },
      ],
      { id: "reply_1", position: { x: 392, y: 132 }, width: 220, height: 84 }
    )

    assert.deepEqual(plain(result), {
      position: { x: 392, y: 132 },
    })
  })
})

describe("workflow history", () => {
  it("undoes and redoes snapshots while clearing redo after a new edit", async () => {
    const {
      createWorkflowHistory,
      pushWorkflowHistory,
      undoWorkflowHistory,
      redoWorkflowHistory,
    } = await loadModule()

    const first = {
      nodes: [{ id: "start_1", position: { x: 0, y: 0 } }],
      edges: [],
    }
    const second = {
      nodes: [{ id: "start_1", position: { x: 100, y: 0 } }],
      edges: [],
    }
    const third = {
      nodes: [{ id: "start_1", position: { x: 200, y: 0 } }],
      edges: [],
    }
    const branch = {
      nodes: [{ id: "start_1", position: { x: 300, y: 0 } }],
      edges: [],
    }

    let history = createWorkflowHistory()
    history = pushWorkflowHistory(history, first)
    history = pushWorkflowHistory(history, second)

    const undone = undoWorkflowHistory(history, third)
    assert.deepEqual(plain(undone.snapshot), second)
    assert.equal(undone.history.past.length, 1)
    assert.equal(undone.history.future.length, 1)

    const redone = redoWorkflowHistory(undone.history, undone.snapshot)
    assert.deepEqual(plain(redone.snapshot), third)
    assert.equal(redone.history.past.length, 2)
    assert.equal(redone.history.future.length, 0)

    const branched = pushWorkflowHistory(undone.history, branch)
    assert.equal(branched.future.length, 0)
  })
})

describe("getAvailableVariables", () => {
  it("exposes start outputs to retrieve node", async () => {
    const { getAvailableVariables } = await loadModule()

    const variables = getAvailableVariables(
      {
        nodes: [
          { id: "start_1", type: "start", position: { x: 0, y: 0 }, data: { name: "Start" } },
          { id: "retrieve_1", type: "knowledge_retrieve", position: { x: 200, y: 0 }, data: {} },
        ],
        edges: [{ id: "e1", source: "start_1", target: "retrieve_1" }],
      },
      "retrieve_1",
      [
        {
          type: "start",
          outputSchema: [{ name: "userMessage", type: "string", description: "Message" }],
        },
      ]
    )

    assert.deepEqual(plain(variables), [
      {
        nodeId: "start_1",
        nodeName: "Start",
        field: "userMessage",
        type: "string",
        description: "Message",
      },
    ])
  })

  it("hides variables from downstream nodes", async () => {
    const { getAvailableVariables } = await loadModule()

    const variables = getAvailableVariables(
      {
        nodes: [
          { id: "start_1", type: "start", position: { x: 0, y: 0 }, data: {} },
          { id: "reply_1", type: "send_reply", position: { x: 200, y: 0 }, data: {} },
          { id: "end_1", type: "end", position: { x: 400, y: 0 }, data: {} },
        ],
        edges: [
          { id: "e1", source: "start_1", target: "reply_1" },
          { id: "e2", source: "reply_1", target: "end_1" },
        ],
      },
      "reply_1",
      [
        {
          type: "end",
          outputSchema: [{ name: "status", type: "string", description: "Status" }],
        },
      ]
    )

    assert.deepEqual(plain(variables), [])
  })
})

describe("toApiDefinition", () => {
  it("keeps condition branches on the condition node config and exports plain edges", async () => {
    const { toApiDefinition } = await loadModule()

    const definition = toApiDefinition({
      nodes: [
        {
          id: "start_1",
          type: "workflowNode",
          position: { x: 0, y: 0 },
          data: { nodeType: "start", name: "Start", config: {} },
        },
        {
          id: "condition_1",
          type: "workflowNode",
          position: { x: 200, y: 0 },
          data: {
            nodeType: "condition",
            name: "Route",
            config: {
              branches: [
                {
                  id: "vip",
                  name: "VIP",
                  targetNodeId: "vip_reply",
                  condition: {
                    left: { nodeId: "start_1", field: "userMessage" },
                    operator: "eq",
                    right: "vip",
                  },
                },
                {
                  id: "default",
                  name: "Default",
                  targetNodeId: "normal_reply",
                  default: true,
                },
              ],
            },
          },
        },
        {
          id: "vip_reply",
          type: "workflowNode",
          position: { x: 400, y: 0 },
          data: { nodeType: "llm_reply", name: "VIP", config: {} },
        },
        {
          id: "normal_reply",
          type: "workflowNode",
          position: { x: 400, y: 160 },
          data: { nodeType: "llm_reply", name: "Normal", config: {} },
        },
      ],
      edges: [
        { id: "e1", source: "start_1", target: "condition_1" },
        {
          id: "e2",
          source: "condition_1",
          target: "vip_reply",
          data: {
            condition: {
              left: { nodeId: "start_1", field: "userMessage" },
              operator: "eq",
              right: "legacy",
            },
          },
        },
        { id: "e3", source: "condition_1", target: "normal_reply" },
      ],
    })

    assert.deepEqual(plain(definition.edges), [
      { id: "e1", source: "start_1", target: "condition_1" },
      { id: "e2", source: "condition_1", target: "vip_reply" },
      { id: "e3", source: "condition_1", target: "normal_reply" },
    ])
    assert.deepEqual(plain(definition.nodes[1].config.branches), [
      {
        id: "vip",
        name: "VIP",
        targetNodeId: "vip_reply",
        condition: {
          left: { nodeId: "start_1", field: "userMessage" },
          operator: "eq",
          right: "vip",
        },
      },
      {
        id: "default",
        name: "Default",
        targetNodeId: "normal_reply",
        default: true,
      },
    ])
  })

  it("preserves a portable node failure target", async () => {
    const { fromApiDefinition, toApiDefinition } = await loadModule()
    const source = {
      schemaVersion: 1,
      entryNodeId: "start_1",
      nodes: [
        { id: "start_1", type: "start", name: "开始", position: { x: 0, y: 0 }, config: {} },
        {
          id: "llm_1",
          type: "llm_reply",
          name: "模型回答",
          position: { x: 200, y: 0 },
          config: {},
          errorTargetNodeId: "fallback_1",
        },
        { id: "fallback_1", type: "end", name: "兜底结束", position: { x: 400, y: 120 }, config: {} },
      ],
      edges: [
        { id: "e1", source: "start_1", target: "llm_1" },
        { id: "e2", source: "llm_1", target: "fallback_1" },
      ],
    }

    const definition = toApiDefinition(fromApiDefinition(source))
    assert.equal(definition.nodes[1].errorTargetNodeId, "fallback_1")
  })

  it("preserves xyflow node positions", async () => {
    const { toApiDefinition } = await loadModule()

    const definition = toApiDefinition({
      nodes: [
        {
          id: "start_1",
          type: "start",
          position: { x: 12, y: 34 },
          data: { name: "Start", config: { enabled: true } },
        },
        {
          id: "end_1",
          type: "end",
          position: { x: 240, y: 80 },
          data: { name: "End", config: {} },
        },
      ],
      edges: [{ id: "e1", source: "start_1", target: "end_1" }],
    })

    assert.deepEqual(plain(definition), {
      schemaVersion: 1,
      entryNodeId: "start_1",
      nodes: [
        {
          id: "start_1",
          type: "start",
          name: "Start",
          position: { x: 12, y: 34 },
          config: { enabled: true },
        },
        {
          id: "end_1",
          type: "end",
          name: "End",
          position: { x: 240, y: 80 },
          config: {},
        },
      ],
      edges: [{ id: "e1", source: "start_1", target: "end_1" }],
    })
  })

  it("uses node data type for xyflow default nodes", async () => {
    const { toApiDefinition } = await loadModule()

    const definition = toApiDefinition({
      nodes: [
        {
          id: "start_1",
          type: "default",
          position: { x: 0, y: 0 },
          data: { nodeType: "start", name: "Start", config: {} },
        },
        {
          id: "end_1",
          type: "default",
          position: { x: 200, y: 0 },
          data: { nodeType: "end", name: "End", config: {} },
        },
      ],
      edges: [{ id: "e1", source: "start_1", target: "end_1" }],
    })

    assert.equal(definition.entryNodeId, "start_1")
    assert.equal(definition.nodes[0].type, "start")
  })
})
