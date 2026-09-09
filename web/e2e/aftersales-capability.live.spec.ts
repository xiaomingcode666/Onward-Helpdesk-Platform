import { expect, test, type APIRequestContext } from "@playwright/test"

const baseUrl = process.env.E2E_BASE_URL ?? "http://localhost:3000"
const adminUsername = process.env.E2E_ADMIN_USERNAME ?? ""
const adminPassword = process.env.E2E_ADMIN_PASSWORD ?? ""
const customerUsername = process.env.E2E_CUSTOMER_USERNAME ?? ""
const customerPassword = process.env.E2E_CUSTOMER_PASSWORD ?? ""
const customerDeviceNo = process.env.E2E_CUSTOMER_DEVICE_NO ?? ""
const agentId = Number(process.env.E2E_AGENT_ID ?? "0")

type Envelope<T> = {
  success: boolean
  errorCode: number
  message: string
  data: T
}

type Session = {
  accessToken: string
  tenantId: number
}

type Device = {
  id: number
  device_no: string
}

type Conversation = {
  id: number
}

type Message = {
  id: number
}

type WorkflowRun = {
  id: number
  runtimeEngine: string
  statusName: string
  conversationId: number
  aiAgentId: number
  messageId: number
  skillAudit: {
    state: string
    middlewareEnabled: boolean
    selectedSkillId: number
    selectedSkillName: string
    sourceMessageId: number
  }
}

async function login(
  request: APIRequestContext,
  username: string,
  password: string,
  domainType: "customer" | "enterprise",
) {
  const response = await request.post(`${baseUrl}/api/auth/login`, {
    data: {
      username,
      password,
      domainType,
    },
  })
  const payload = await response.json() as Envelope<Session>
  expect(payload.success, payload.message).toBe(true)
  return payload.data
}

async function triggerFormalSkillRun(
  request: APIRequestContext,
  customerSession: Session,
  adminSession: Session,
) {
  const devices = await call<Device[]>(request, customerSession, "/api/customer/v1/devices")
  const device = devices.find((item) => item.device_no === customerDeviceNo)
  expect(device, `能力回归未找到正式设备 ${customerDeviceNo}`).toBeTruthy()

  const matched = await call<Conversation>(request, customerSession, "/api/conversation/create_or_match", {
    deviceId: device!.id,
    forceNew: true,
  })
  await cleanupConversation(request, customerSession, adminSession, matched.id)

  const conversation = await call<Conversation>(request, customerSession, "/api/conversation/create_or_match", {
    deviceId: device!.id,
    forceNew: true,
  })
  const runTag = `CAPABILITY-SKILL-${Date.now()}`
  const message = await call<Message>(request, customerSession, "/api/message/send", {
    conversationId: conversation.id,
    messageType: "text",
    content: `${runTag} RHD-FLOW-ALPHA-7742 怎么复位？请给出安全操作步骤。`,
    clientMsgId: `${runTag}-message`,
  })
  return { conversationId: conversation.id, messageId: message.id }
}

async function cleanupConversation(
  request: APIRequestContext,
  customerSession: Session,
  adminSession: Session,
  conversationId: number,
) {
  const ticketPage = await call<{ items: Array<{ id: number; status: string }> }>(
    request,
    adminSession,
    `/api/enterprise/v1/tickets?page=1&page_size=20&conversation_id=${conversationId}`,
  ).catch(() => ({ items: [] }))
  for (const ticket of ticketPage.items) {
    if (["closed", "cancelled", "done"].includes(ticket.status)) continue
    await call(request, adminSession, `/api/enterprise/v1/tickets/${ticket.id}/_cancel`, {
      reason: "能力回归清理同设备旧会话",
    }).catch(() => undefined)
  }
  await call(request, customerSession, "/api/conversation/close", {
    conversationId,
  }).catch(() => undefined)
}

async function waitForSelectedSkillRun(
  request: APIRequestContext,
  adminSession: Session,
  conversationId: number,
  messageId: number,
) {
  let matched: WorkflowRun | undefined
  await expect.poll(async () => {
    const runs = await call<{ results: WorkflowRun[] }>(
      request,
      adminSession,
      `/api/dashboard/ai-workflow/run/list?page=1&limit=20&conversationId=${conversationId}&messageId=${messageId}&aiAgentId=${agentId}`,
    )
    matched = runs.results.find((item) => (
      item.conversationId === conversationId
        && item.messageId === messageId
        && item.aiAgentId === agentId
        && item.runtimeEngine === "eino_graph"
    ))
    if (matched?.statusName === "failed") {
      throw new Error(`正式 Skill 审计触发的 Workflow Run 失败：run=${matched.id}`)
    }
    if ((matched?.skillAudit?.selectedSkillId ?? 0) > 0) {
      return "selected"
    }
    return matched?.skillAudit?.state || matched?.statusName || "pending"
  }, {
    message: `等待正式会话消息 ${messageId} 产生 Skill 选择审计`,
    timeout: 120_000,
    intervals: [500, 1_000, 2_000],
  }).toBe("selected")
  expect(matched, "正式 Agent 的 Workflow Run 应存在").toBeTruthy()
  return matched!
}

async function call<T>(
  request: APIRequestContext,
  session: Session,
  path: string,
  data?: unknown,
) {
  const response = await request.fetch(`${baseUrl}${path}`, {
    method: data === undefined ? "GET" : "POST",
    data,
    headers: {
      Authorization: `Bearer ${session.accessToken}`,
      "X-Tenant-Id": String(session.tenantId),
    },
  })
  const payload = await response.json() as Envelope<T>
  expect(response.status(), payload.message).toBeLessThan(500)
  expect(payload.success, payload.message).toBe(true)
  return payload.data
}

test("正式 Agent 的 Skill 审计与 MCP 工具调用可用", async ({ request }) => {
  test.setTimeout(180_000)
  if (!adminUsername || !adminPassword || !customerUsername || !customerPassword || !customerDeviceNo || agentId <= 0) {
    throw new Error("能力回归缺少 E2E_ADMIN_USERNAME、E2E_ADMIN_PASSWORD、E2E_CUSTOMER_USERNAME、E2E_CUSTOMER_PASSWORD、E2E_CUSTOMER_DEVICE_NO 或 E2E_AGENT_ID")
  }
  const [session, customerSession] = await Promise.all([
    login(request, adminUsername, adminPassword, "enterprise"),
    login(request, customerUsername, customerPassword, "customer"),
  ])

  const triggered = await triggerFormalSkillRun(request, customerSession, session)
  try {
    const skillRun = await waitForSelectedSkillRun(request, session, triggered.conversationId, triggered.messageId)
    expect(skillRun.skillAudit.middlewareEnabled).toBe(true)
    expect(skillRun.skillAudit.selectedSkillName).toBeTruthy()
    expect(skillRun.skillAudit.sourceMessageId).toBe(triggered.messageId)

    const servers = await call<Array<{ code: string; enabled: boolean }>>(
      request,
      session,
      "/api/dashboard/mcp/list_servers",
    )
    expect(servers).toContainEqual(expect.objectContaining({ code: "system", enabled: true }))

    const connection = await call<{ protocol: string; serverName: string }>(
      request,
      session,
      "/api/dashboard/mcp/test_connection",
      { serverCode: "system" },
    )
    expect(connection.serverName).toBe("remote-helpdesk-mcp-server")
    expect(connection.protocol).toBeTruthy()

    const tools = await call<Array<{ name: string }>>(
      request,
      session,
      "/api/dashboard/mcp/list_tools",
      { serverCode: "system" },
    )
    expect(tools.map((item) => item.name)).toEqual(expect.arrayContaining(["server_time", "service_info"]))

    const toolResult = await call<{
      serverCode: string
      toolName: string
      isError: boolean
      content: Array<{ type: string; text?: string }>
      structuredContent?: unknown
    }>(request, session, "/api/dashboard/mcp/call_tool", {
      serverCode: "system",
      toolName: "server_time",
      arguments: { timezone: "Asia/Shanghai" },
    })
    expect(toolResult.serverCode).toBe("system")
    expect(toolResult.toolName).toBe("server_time")
    expect(toolResult.isError).toBe(false)
    expect(toolResult.content.length).toBeGreaterThan(0)
    expect(toolResult.structuredContent).toBeTruthy()
  } finally {
    await cleanupConversation(request, customerSession, session, triggered.conversationId)
  }
})
