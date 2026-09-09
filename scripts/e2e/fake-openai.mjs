import http from "node:http"

const port = Number(process.env.E2E_OPENAI_PORT || "18099")

function messageText(messages) {
  return (messages || []).map((message) => {
    if (typeof message?.content === "string") return message.content
    if (Array.isArray(message?.content)) {
      return message.content.map((part) => part?.text || "").join("\n")
    }
    return ""
  }).join("\n")
}

function latestUserMessageText(messages) {
  const message = [...(messages || [])].reverse().find((item) => item?.role === "user")
  if (typeof message?.content === "string") return message.content
  if (Array.isArray(message?.content)) {
    return message.content.map((part) => part?.text || "").join("\n")
  }
  return ""
}

function currentUserQueryText(messages) {
  const latest = latestUserMessageText(messages)
  return latest.split(/\n\nKnowledge context:\n/i, 1)[0]
}

function toolFunctionName(tool) {
  return tool?.function?.name || tool?.name || ""
}

function hasSkillTool(body) {
  return (body.tools || []).some((tool) => toolFunctionName(tool) === "skill")
}

function hasToolResult(messages) {
  return (messages || []).some((message) => message?.role === "tool")
}

function selectSkillID(body) {
  const text = messageText(body.messages)
  if (/RHD-FLOW-ALPHA-7742|复位|操作|维修指导|安全操作/.test(text)) {
    return "3"
  }
  if (/升级|人工|停机|高风险|5\s*次|反复上电/.test(text)) {
    return "2"
  }
  return "1"
}

function shouldCallSkillTool(body) {
  return hasSkillTool(body) && !hasToolResult(body.messages)
}

function deterministicAnswer(body) {
  const prompt = messageText(body.messages)
  const latestUserMessage = latestUserMessageText(body.messages)
  const currentUserQuery = currentUserQueryText(body.messages)
  const proof = currentUserQuery.match(/KCAP-\d+/)?.[0] ?? latestUserMessage.match(/KCAP-\d+/)?.[0] ?? prompt.match(/KCAP-\d+/)?.[0]
  if (/E2E_GENERAL_UNKNOWN/.test(currentUserQuery)) {
    return "GENERAL_SAFE_UNKNOWN：当前没有足够的产品或设备信息，我不会编造具体结论；请补充产品、型号或故障现象。"
  }
  if (/5\s*次|重复上电|反复上电/.test(currentUserQuery)) {
    return "不建议连续重复上电 5 次。请停止操作、保持设备断电，并联系技术工程师确认保护输入端。"
  }
  if (/RHD-AI-ONLY-BETA-8841/.test(currentUserQuery)) {
    return "AI_ONLY_KNOWLEDGE_OK：先关闭执行机构并等待 20 秒，确认压力归零后检查传感器接头；不得带压拆卸。"
  }
  if (/知识验收标识/.test(currentUserQuery) && proof) {
    return `维修知识已验证：端子受热回弹导致接触电阻升高；更换同规格端子并按标准扭矩重新压接，带载运行 30 分钟后母线稳定在 48.0V。知识验收标识 ${proof}。`
  }
  if (/RHD-FLOW-ALPHA-7742/.test(currentUserQuery)) {
    return "COLLAB_KNOWLEDGE_OK：先断开设备主电源并等待 30 秒，确认储能释放后检查保护输入端。"
  }
  if (proof) {
    return `维修知识已验证：端子受热回弹导致接触电阻升高；更换同规格端子并按标准扭矩重新压接，带载运行 30 分钟后母线稳定在 48.0V。知识验收标识 ${proof}。`
  }
  return "请先断开设备主电源并等待 30 秒，确认储能释放后再检查保护输入端。正常状态灯应为绿色慢闪；红灯常亮时不要继续上电。"
}

function deterministicEmbedding(input) {
  const text = Array.isArray(input) ? input.join("\n") : String(input || "")
  if (/E2E_UNKNOWN_NO_KNOWLEDGE/.test(text)) {
    return [0, 1, 0, 0, 0, 0, 0, 0]
  }
  return [1, 0, 0, 0, 0, 0, 0, 0]
}

function sendJSON(response, status, payload) {
  response.writeHead(status, { "content-type": "application/json" })
  response.end(JSON.stringify(payload))
}

const server = http.createServer((request, response) => {
  if (request.method === "GET" && request.url === "/health") {
    sendJSON(response, 200, { ok: true })
    return
  }
  let raw = ""
  request.on("data", (chunk) => { raw += chunk })
  request.on("end", () => {
    let body = {}
    try { body = raw ? JSON.parse(raw) : {} } catch {
      sendJSON(response, 400, { error: { message: "invalid JSON" } })
      return
    }
    if (request.method === "POST" && request.url?.endsWith("/embeddings")) {
      sendJSON(response, 200, {
        object: "list",
        model: body.model || "e2e-embedding",
        data: [{ object: "embedding", index: 0, embedding: deterministicEmbedding(body.input) }],
        usage: { prompt_tokens: 8, total_tokens: 8 },
      })
      return
    }
    if (request.method === "POST" && request.url?.endsWith("/chat/completions")) {
      const content = deterministicAnswer(body)
      const id = `chatcmpl-e2e-${Date.now()}`
      if (shouldCallSkillTool(body)) {
        const toolCall = {
          index: 0,
          id: `call_e2e_skill_${Date.now()}`,
          type: "function",
          function: {
            name: "skill",
            arguments: JSON.stringify({ skill: selectSkillID(body) }),
          },
        }
        if (body.stream) {
          response.writeHead(200, { "content-type": "text/event-stream", "cache-control": "no-cache" })
          response.write(`data: ${JSON.stringify({ id, object: "chat.completion.chunk", created: Math.floor(Date.now() / 1000), model: body.model || "e2e-llm", choices: [{ index: 0, delta: { role: "assistant", tool_calls: [toolCall] }, finish_reason: null }] })}\n\n`)
          response.write(`data: ${JSON.stringify({ id, object: "chat.completion.chunk", created: Math.floor(Date.now() / 1000), model: body.model || "e2e-llm", choices: [{ index: 0, delta: {}, finish_reason: "tool_calls" }] })}\n\n`)
          response.end("data: [DONE]\n\n")
          return
        }
        sendJSON(response, 200, {
          id,
          object: "chat.completion",
          created: Math.floor(Date.now() / 1000),
          model: body.model || "e2e-llm",
          choices: [{ index: 0, message: { role: "assistant", content: null, tool_calls: [toolCall] }, finish_reason: "tool_calls" }],
          usage: { prompt_tokens: 32, completion_tokens: 4, total_tokens: 36 },
        })
        return
      }
      if (body.stream) {
        response.writeHead(200, { "content-type": "text/event-stream", "cache-control": "no-cache" })
        response.write(`data: ${JSON.stringify({ id, object: "chat.completion.chunk", created: Math.floor(Date.now() / 1000), model: body.model || "e2e-llm", choices: [{ index: 0, delta: { role: "assistant", content }, finish_reason: null }] })}\n\n`)
        response.write(`data: ${JSON.stringify({ id, object: "chat.completion.chunk", created: Math.floor(Date.now() / 1000), model: body.model || "e2e-llm", choices: [{ index: 0, delta: {}, finish_reason: "stop" }] })}\n\n`)
        response.end("data: [DONE]\n\n")
        return
      }
      sendJSON(response, 200, {
        id,
        object: "chat.completion",
        created: Math.floor(Date.now() / 1000),
        model: body.model || "e2e-llm",
        choices: [{ index: 0, message: { role: "assistant", content }, finish_reason: "stop" }],
        usage: { prompt_tokens: 32, completion_tokens: 24, total_tokens: 56 },
      })
      return
    }
    sendJSON(response, 404, { error: { message: "not found" } })
  })
})

server.listen(port, "127.0.0.1", () => {
  process.stdout.write(`fake OpenAI listening on http://127.0.0.1:${port}/v1\n`)
})
