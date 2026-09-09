import { expect, test, type APIRequestContext } from "@playwright/test"

const baseUrl = process.env.E2E_BASE_URL ?? "http://localhost:3000"
const frontendUrl = process.env.E2E_FRONTEND_URL ?? baseUrl
const tenantId = Number(process.env.E2E_TENANT_ID ?? "0")
const customerUsername = process.env.E2E_CUSTOMER_USERNAME ?? ""
const customerPassword = process.env.E2E_CUSTOMER_PASSWORD ?? ""
const customerDeviceNo = process.env.E2E_CUSTOMER_DEVICE_NO ?? ""
const serviceCode = process.env.E2E_SERVICE_CODE ?? ""
const adminUsername = process.env.E2E_ADMIN_USERNAME ?? ""
const adminPassword = process.env.E2E_ADMIN_PASSWORD ?? ""

type Envelope<T> = {
  success: boolean
  errorCode: number
  message: string
  data: T
}

type Session = {
  accessToken: string
  tenantId: number
  domainType: string
}

type Conversation = {
  id: number
  aiAgentId?: number
  productId: number
  deviceId: number
  status: number
  humanHandoffEnabled: boolean
}

type Message = {
  id: number
  conversationId: number
  workflowRunId?: number
  senderType: string
  content: string
}

type WorkflowRun = {
  id: number
  tenantId: number
  productId: number
  workflowId: number
  workflowVersionId: number
  conversationId: number
  aiAgentId: number
  agentReleaseId: number
  messageId: number
  status: number
  statusName: string
}

type WorkflowRunDetail = WorkflowRun & {
  nodes: Array<{
    nodeId: string
    nodeType: string
    statusName: string
    outputPreview: string
    errorMessage: string
  }>
  humanHandling?: {
    handoffOccurred: boolean
    handoffReason: string
    conversationStatus: number
  }
}

type RouteFixture = {
  name: string
  createPayload: { general: true; forceNew: true } | { deviceId: number; forceNew: true }
  expectedProductId: number
  expectedAgentId: number
  expectedReleaseId: number
  expectedWorkflowId: number
  expectedWorkflowVersionId: number
  allowsHumanHandoff: boolean
}

type KnowledgeIndexTask = {
  subject_type: string
  subject_id: number
  status: string
  error_summary: string
}

function requiredPositiveInt(name: string) {
  const value = Number(process.env[name] ?? "0")
  if (!Number.isSafeInteger(value) || value <= 0) {
    throw new Error(`生产 AI 路由回归缺少数值环境变量：${name}`)
  }
  return value
}

function requireFixture() {
  const missing = [
    ["E2E_TENANT_ID", tenantId > 0 ? String(tenantId) : ""],
    ["E2E_CUSTOMER_USERNAME", customerUsername],
    ["E2E_CUSTOMER_PASSWORD", customerPassword],
    ["E2E_CUSTOMER_DEVICE_NO", customerDeviceNo],
    ["E2E_SERVICE_CODE", serviceCode],
    ["E2E_ADMIN_USERNAME", adminUsername],
    ["E2E_ADMIN_PASSWORD", adminPassword],
  ].filter(([, value]) => !value).map(([name]) => name)
  if (missing.length > 0) {
    throw new Error(`生产 AI 路由回归缺少环境变量：${missing.join(", ")}`)
  }
}

async function login(
  request: APIRequestContext,
  username: string,
  password: string,
  domainType: "customer" | "enterprise",
) {
  const response = await request.post(`${baseUrl}/api/auth/login`, {
    data: { username, password, domainType },
  })
  const payload = await response.json() as Envelope<Session>
  expect(response.ok(), payload.message).toBe(true)
  expect(payload.success, payload.message).toBe(true)
  expect(payload.data.accessToken, `${domainType} 登录必须返回正式 access token`).toBeTruthy()
  expect(payload.data.tenantId).toBe(tenantId)
  expect(payload.data.domainType).toBe(domainType)
  return payload.data
}

async function call<T>(
  request: APIRequestContext,
  session: Session,
  path: string,
  options: { method?: "GET" | "POST"; data?: unknown } = {},
) {
  const response = await request.fetch(`${baseUrl}${path}`, {
    method: options.method ?? "GET",
    ...(options.data === undefined ? {} : { data: options.data }),
    headers: {
      Authorization: `Bearer ${session.accessToken}`,
      "X-Tenant-Id": String(session.tenantId),
    },
  })
  const payload = await response.json() as Envelope<T>
  expect(response.ok(), payload.message).toBe(true)
  expect(payload.success, payload.message).toBe(true)
  return payload.data
}

async function expectCustomerRoutingRejectsInternalIDs(
  request: APIRequestContext,
  customerSession: Session,
  agentId: number,
  workflowId: number,
) {
  const response = await request.post(`${baseUrl}/api/conversation/create_or_match`, {
    data: { general: true, forceNew: true, aiAgentId: agentId, workflowId },
    headers: {
      Authorization: `Bearer ${customerSession.accessToken}`,
      "X-Tenant-Id": String(customerSession.tenantId),
    },
  })
  const payload = await response.json() as Envelope<unknown>
  expect(payload.success, "客户入口必须拒绝内部 Agent/Workflow ID").toBe(false)
  expect(payload.message).toContain("unknown field")
}

async function waitForKnowledgeIndex(
  request: APIRequestContext,
  adminSession: Session,
  productId: number,
  documentId: number,
) {
  await expect.poll(async () => {
    const tasks = await call<KnowledgeIndexTask[]>(
      request,
      adminSession,
      `/api/enterprise/v1/knowledge/index-tasks?product_id=${productId}`,
    )
    const task = tasks.find((item) => item.subject_type === "knowledge_document" && item.subject_id === documentId)
    if (task?.status === "failed") {
      throw new Error(`E2E knowledge index failed: ${task.error_summary || "unknown error"}`)
    }
    return task?.status ?? "missing"
  }, {
    message: `等待 product=${productId} document=${documentId} 完成知识索引`,
    timeout: 120_000,
    intervals: [500, 1_000, 2_000],
  }).toBe("succeeded")
}

async function waitForWorkflowRun(
  request: APIRequestContext,
  adminSession: Session,
  conversationId: number,
  messageId: number,
) {
  let matched: WorkflowRun | undefined
  await expect.poll(async () => {
    const page = await call<{ results: WorkflowRun[] }>(
      request,
      adminSession,
      `/api/dashboard/ai-workflow/run/list?page=1&limit=20&conversationId=${conversationId}&messageId=${messageId}`,
    )
    matched = page.results.find((item) => item.conversationId === conversationId && item.messageId === messageId)
    return matched?.statusName ?? "pending"
  }, {
    message: `等待 conversation=${conversationId} message=${messageId} 的线上 Workflow Run`,
    timeout: 90_000,
    intervals: [500, 1_000, 2_000],
  }).toMatch(/^(completed|interrupted|failed)$/)
  expect(matched).toBeTruthy()
  expect(matched?.statusName, `Workflow Run ${matched?.id} 不应失败`).not.toBe("failed")
  return matched!
}

async function sendAndAudit(
  request: APIRequestContext,
  customerSession: Session,
  adminSession: Session,
  conversation: Conversation,
  route: RouteFixture,
  content: string,
  expectedNodeId: string,
) {
  const message = await call<Message>(request, customerSession, "/api/message/send", {
    method: "POST",
    data: {
      conversationId: conversation.id,
      clientMsgId: `routing-${route.expectedAgentId}-${Date.now()}-${Math.random()}`,
      messageType: "html",
      content,
      payload: "",
    },
  })
  const run = await waitForWorkflowRun(request, adminSession, conversation.id, message.id)
  expect(run).toMatchObject({
    tenantId,
    productId: route.expectedProductId,
    workflowId: route.expectedWorkflowId,
    workflowVersionId: route.expectedWorkflowVersionId,
    conversationId: conversation.id,
    aiAgentId: route.expectedAgentId,
    agentReleaseId: route.expectedReleaseId,
    messageId: message.id,
  })
  const auditPage = await call<{ results: WorkflowRun[] }>(
    request,
    adminSession,
    "/api/dashboard/ai-workflow/run/list?" + new URLSearchParams({
      page: "1",
      limit: "20",
      conversationId: String(conversation.id),
      messageId: String(message.id),
      workflowVersionId: String(route.expectedWorkflowVersionId),
      aiAgentId: String(route.expectedAgentId),
      status: String(run.status),
    }).toString(),
  )
  expect(auditPage.results.some((item) => item.id === run.id), `组合审计筛选应返回 Workflow Run ${run.id}`).toBe(true)
  expect(auditPage.results.every((item) =>
    item.conversationId === conversation.id &&
    item.messageId === message.id &&
    item.workflowVersionId === route.expectedWorkflowVersionId &&
    item.aiAgentId === route.expectedAgentId &&
    item.status === run.status,
  )).toBe(true)
  const detail = await call<WorkflowRunDetail>(request, adminSession, `/api/dashboard/ai-workflow/run/${run.id}`)
  const expectedNode = detail.nodes.find((node) => node.nodeId === expectedNodeId)
  expect(expectedNode, `Workflow Run ${run.id} 应执行节点 ${expectedNodeId}`).toBeTruthy()
  expect(expectedNode?.statusName).toBe("completed")
  expect(expectedNode?.errorMessage ?? "").toBe("")
  return { message, run, detail }
}

async function waitForWorkflowReply(
  request: APIRequestContext,
  customerSession: Session,
  conversationId: number,
  afterMessageId: number,
  expectedText: RegExp,
) {
  let matched: Message | undefined
  await expect.poll(async () => {
    const page = await call<{ results?: Message[] }>(
      request,
      customerSession,
      `/api/message/list?conversationId=${conversationId}&limit=100`,
    )
    matched = (page.results ?? []).find((item) => item.id > afterMessageId && item.senderType === "ai" && expectedText.test(item.content))
    return matched?.content ?? ""
  }, {
    message: `等待客户消息 ${afterMessageId} 之后的客户可见 AI 回复`,
    timeout: 30_000,
    intervals: [250, 500, 1_000],
  }).toMatch(expectedText)
  const finalPage = await call<{ results?: Message[] }>(
    request,
    customerSession,
    `/api/message/list?conversationId=${conversationId}&limit=100`,
  )
  expect((finalPage.results ?? []).some((item) => Object.prototype.hasOwnProperty.call(item, "workflowRunId"))).toBe(false)
  return matched!
}

async function requestHuman(
  request: APIRequestContext,
  customerSession: Session,
  conversationId: number,
  allowed: boolean,
  expectedRejectMessage = "当前产品仅提供 AI 客服，不支持转人工",
) {
  const response = await request.post(`${baseUrl}/api/conversation/request_human`, {
    data: { conversationId, reason: "P0 客户显式请求人工" },
    headers: {
      Authorization: `Bearer ${customerSession.accessToken}`,
      "X-Tenant-Id": String(customerSession.tenantId),
    },
  })
  expect(response.status(), "转人工业务结果不能由服务器故障产生").toBeLessThan(500)
  const payload = await response.json() as Envelope<unknown>
  if (allowed) {
    expect(response.ok(), payload.message).toBe(true)
    expect(payload.success, payload.message).toBe(true)
    expect(payload.errorCode).toBe(0)
    return
  }
  expect(response.status()).toBe(200)
  expect(payload.success).toBe(false)
  expect(payload.errorCode).toBe(3001)
  expect(payload.message).toContain(expectedRejectMessage)
}

async function expectTicketCount(
  request: APIRequestContext,
  adminSession: Session,
  conversationId: number,
  expected: number,
) {
  await expect.poll(async () => {
    const page = await call<{ items: Array<{ conversation_id: number }>; total: number }>(
      request,
      adminSession,
      `/api/enterprise/v1/tickets?page=1&page_size=20&conversation_id=${conversationId}`,
    )
    expect(page.items.every((item) => item.conversation_id === conversationId)).toBe(true)
    return page.total
  }, {
    message: `等待 conversation=${conversationId} 的工单数量为 ${expected}`,
    timeout: 30_000,
    intervals: [250, 500, 1_000],
  }).toBe(expected)
}

async function cleanupRoutingArtifacts(
  request: APIRequestContext,
  customerSession: Session,
  adminSession: Session,
  conversationIds: number[],
) {
  for (const conversationId of [...new Set(conversationIds)].reverse()) {
    const ticketPage = await call<{
      items: Array<{ id: number; status: string }>
    }>(
      request,
      adminSession,
      `/api/enterprise/v1/tickets?page=1&page_size=20&conversation_id=${conversationId}`,
    )
    for (const ticket of ticketPage.items) {
      if (["closed", "cancelled", "done"].includes(ticket.status)) continue
      await call<void>(request, adminSession, `/api/enterprise/v1/tickets/${ticket.id}/_cancel`, {
        method: "POST",
        data: { reason: "P0 生产路由测试数据清理" },
      })
    }
    await call<void>(request, customerSession, "/api/conversation/close", {
      method: "POST",
      data: { conversationId },
    })
  }
}

async function createFreshRouteConversation(
  request: APIRequestContext,
  customerSession: Session,
  adminSession: Session,
  route: RouteFixture,
) {
  const matched = await call<Conversation>(request, customerSession, "/api/conversation/create_or_match", {
    method: "POST",
    data: route.createPayload,
  })
  await cleanupRoutingArtifacts(request, customerSession, adminSession, [matched.id])
  return call<Conversation>(request, customerSession, "/api/conversation/create_or_match", {
    method: "POST",
    data: route.createPayload,
  })
}

test("正式客户链路覆盖默认 Agent、两类 Workflow、多产品与同产品多设备", async ({ request }) => {
  test.setTimeout(480_000)
  requireFixture()
  const [customerSession, adminSession] = await Promise.all([
    login(request, customerUsername, customerPassword, "customer"),
    login(request, adminUsername, adminPassword, "enterprise"),
  ])

  const general: RouteFixture = {
    name: "通用咨询",
    createPayload: { general: true, forceNew: true },
    expectedProductId: 0,
    expectedAgentId: requiredPositiveInt("E2E_TENANT_DEFAULT_AGENT_ID"),
    expectedReleaseId: requiredPositiveInt("E2E_TENANT_DEFAULT_AGENT_RELEASE_ID"),
    expectedWorkflowId: requiredPositiveInt("E2E_TENANT_DEFAULT_WORKFLOW_ID"),
    expectedWorkflowVersionId: requiredPositiveInt("E2E_TENANT_DEFAULT_WORKFLOW_VERSION_ID"),
    allowsHumanHandoff: false,
  }
  const aiOnly: RouteFixture = {
    name: "产品 A 设备 AI 智能问诊",
    createPayload: { deviceId: requiredPositiveInt("E2E_AI_ONLY_DEVICE_ID"), forceNew: true },
    expectedProductId: requiredPositiveInt("E2E_AI_ONLY_PRODUCT_ID"),
    expectedAgentId: requiredPositiveInt("E2E_AI_ONLY_AGENT_ID"),
    expectedReleaseId: requiredPositiveInt("E2E_AI_ONLY_AGENT_RELEASE_ID"),
    expectedWorkflowId: requiredPositiveInt("E2E_AI_ONLY_WORKFLOW_ID"),
    expectedWorkflowVersionId: requiredPositiveInt("E2E_AI_ONLY_WORKFLOW_VERSION_ID"),
    allowsHumanHandoff: false,
  }
  const collaboration: RouteFixture = {
    name: "产品 B 主设备 AI 人工协同",
    createPayload: { deviceId: requiredPositiveInt("E2E_COLLAB_DEVICE_ID"), forceNew: true },
    expectedProductId: requiredPositiveInt("E2E_COLLAB_PRODUCT_ID"),
    expectedAgentId: requiredPositiveInt("E2E_COLLAB_AGENT_ID"),
    expectedReleaseId: requiredPositiveInt("E2E_COLLAB_AGENT_RELEASE_ID"),
    expectedWorkflowId: requiredPositiveInt("E2E_COLLAB_WORKFLOW_ID"),
    expectedWorkflowVersionId: requiredPositiveInt("E2E_COLLAB_WORKFLOW_VERSION_ID"),
    allowsHumanHandoff: true,
  }
  const collaborationSecondary: RouteFixture = {
    ...collaboration,
    name: "产品 B 第二台设备 AI 人工协同",
    createPayload: { deviceId: requiredPositiveInt("E2E_COLLAB_SECONDARY_DEVICE_ID"), forceNew: true },
  }
  const routes = [general, aiOnly, collaboration, collaborationSecondary]

  expect(new Set(routes.map((item) => item.expectedAgentId)).size, "四个入口应收敛到三个生产 Agent").toBe(3)
  expect(collaborationSecondary.expectedAgentId, "同产品不同设备必须命中同一 Agent").toBe(collaboration.expectedAgentId)
  expect(collaborationSecondary.expectedReleaseId, "同产品不同设备必须命中同一 active release").toBe(collaboration.expectedReleaseId)
  expect(new Set(routes.filter((item) => "deviceId" in item.createPayload).map((item) => item.expectedProductId)).size).toBe(2)
  expect(general.expectedWorkflowId, "通用咨询应使用 AI-only 流程，不能复用产品 B 的人工协同流程").not.toBe(collaboration.expectedWorkflowId)
  expect(aiOnly.expectedWorkflowId, "产品 A 应使用独立的 AI 智能问诊流程").not.toBe(collaboration.expectedWorkflowId)

  await Promise.all([
    waitForKnowledgeIndex(
      request,
      adminSession,
      collaboration.expectedProductId,
      requiredPositiveInt("E2E_SEED_KNOWLEDGE_DOCUMENT_ID"),
    ),
    waitForKnowledgeIndex(
      request,
      adminSession,
      aiOnly.expectedProductId,
      requiredPositiveInt("E2E_AI_ONLY_KNOWLEDGE_DOCUMENT_ID"),
    ),
  ])

  for (const route of routes) {
    expect(route.createPayload).not.toHaveProperty("aiAgentId")
    expect(route.createPayload).not.toHaveProperty("workflowId")
  }
  await expectCustomerRoutingRejectsInternalIDs(request, customerSession, collaboration.expectedAgentId, collaboration.expectedWorkflowId)

  const createdConversationIds: number[] = []
  try {
    const generalConversation = await createFreshRouteConversation(request, customerSession, adminSession, general)
    createdConversationIds.push(generalConversation.id)
    expect(generalConversation, "客户响应不应暴露内部 Agent ID").not.toHaveProperty("aiAgentId")
    expect(generalConversation).toMatchObject({
      productId: 0,
      deviceId: 0,
      humanHandoffEnabled: false,
    })
    const generalRun = await sendAndAudit(
      request,
      customerSession,
      adminSession,
      generalConversation,
      general,
      "E2E_GENERAL_UNKNOWN：请说明在没有产品和设备信息时你能提供哪些帮助。",
      "quick_reply_1",
    )
    await waitForWorkflowReply(request, customerSession, generalConversation.id, generalRun.message.id, /GENERAL_SAFE_UNKNOWN/)
    await requestHuman(
      request,
      customerSession,
      generalConversation.id,
      false,
      "通用咨询仅支持 AI 自助，不支持转人工或创建工单",
    )
    await expectTicketCount(request, adminSession, generalConversation.id, 0)

    const aiOnlyConversation = await createFreshRouteConversation(request, customerSession, adminSession, aiOnly)
    createdConversationIds.push(aiOnlyConversation.id)
    expect(aiOnlyConversation).toMatchObject({
      productId: aiOnly.expectedProductId,
      deviceId: requiredPositiveInt("E2E_AI_ONLY_DEVICE_ID"),
      humanHandoffEnabled: false,
    })
    const aiOnlyKnown = await sendAndAudit(
      request,
      customerSession,
      adminSession,
      aiOnlyConversation,
      aiOnly,
      "设备出现故障码 RHD-AI-ONLY-BETA-8841，应如何安全复位？",
      "device_reply_1",
    )
    await waitForWorkflowReply(request, customerSession, aiOnlyConversation.id, aiOnlyKnown.message.id, /AI_ONLY_KNOWLEDGE_OK/)
    const aiOnlyUnknown = await sendAndAudit(
      request,
      customerSession,
      adminSession,
      aiOnlyConversation,
      aiOnly,
      "E2E_UNKNOWN_NO_KNOWLEDGE：设备出现量子相位漂移，现有资料没有对应说明。",
      "diagnostic_safe_reply_1",
    )
    expect(aiOnlyUnknown.detail.nodes.some((node) => node.nodeType === "handoff_to_human")).toBe(false)
    await waitForWorkflowReply(
      request,
      customerSession,
      aiOnlyConversation.id,
      aiOnlyUnknown.message.id,
      /暂时无法|不足|补充|不会创建工单或转接人工/,
    )
    await requestHuman(request, customerSession, aiOnlyConversation.id, false)
    await expectTicketCount(request, adminSession, aiOnlyConversation.id, 0)

    const collaborationConversation = await createFreshRouteConversation(request, customerSession, adminSession, collaboration)
    createdConversationIds.push(collaborationConversation.id)
    expect(collaborationConversation).toMatchObject({
      productId: collaboration.expectedProductId,
      deviceId: requiredPositiveInt("E2E_COLLAB_DEVICE_ID"),
      humanHandoffEnabled: true,
    })
    const collaborationKnown = await sendAndAudit(
      request,
      customerSession,
      adminSession,
      collaborationConversation,
      collaboration,
      "设备出现故障码 RHD-FLOW-ALPHA-7742，应如何安全处理？",
      "reply_1",
    )
    await waitForWorkflowReply(request, customerSession, collaborationConversation.id, collaborationKnown.message.id, /COLLAB_KNOWLEDGE_OK/)
    const collaborationUnknown = await sendAndAudit(
      request,
      customerSession,
      adminSession,
      collaborationConversation,
      collaboration,
      "E2E_UNKNOWN_NO_KNOWLEDGE：设备出现量子相位漂移，知识库没有答案，请继续处理。",
      "diagnostic_safe_reply_1",
    )
    expect(collaborationUnknown.detail.humanHandling?.handoffOccurred ?? false).toBe(false)
    expect(collaborationUnknown.detail.nodes.some((node) => node.nodeType === "handoff_to_human")).toBe(false)
    await waitForWorkflowReply(
      request,
      customerSession,
      collaborationConversation.id,
      collaborationUnknown.message.id,
      /暂时无法|不足|转人工|不会自动转人工或创建工单/,
    )
    await expectTicketCount(request, adminSession, collaborationConversation.id, 0)
    await requestHuman(request, customerSession, collaborationConversation.id, true)
    await expectTicketCount(request, adminSession, collaborationConversation.id, 1)

    const secondaryConversation = await createFreshRouteConversation(request, customerSession, adminSession, collaborationSecondary)
    createdConversationIds.push(secondaryConversation.id)
    expect(secondaryConversation).toMatchObject({
      productId: collaboration.expectedProductId,
      deviceId: requiredPositiveInt("E2E_COLLAB_SECONDARY_DEVICE_ID"),
      humanHandoffEnabled: true,
    })
    const secondaryRun = await sendAndAudit(
      request,
      customerSession,
      adminSession,
      secondaryConversation,
      collaborationSecondary,
      "设备出现故障码 RHD-FLOW-ALPHA-7742，应如何安全处理？",
      "reply_1",
    )
    await waitForWorkflowReply(request, customerSession, secondaryConversation.id, secondaryRun.message.id, /COLLAB_KNOWLEDGE_OK/)
  } finally {
    await cleanupRoutingArtifacts(request, customerSession, adminSession, createdConversationIds)
  }
})

test("服务码匿名识别后通过正式客户登录恢复设备会话", async ({ page }) => {
  test.setTimeout(120_000)
  requireFixture()
  const expectedDeviceId = requiredPositiveInt("E2E_COLLAB_DEVICE_ID")

  if (frontendUrl !== baseUrl) {
    await page.route(`${frontendUrl}/api/**`, async (route) => {
      const source = new URL(route.request().url())
      await route.continue({ url: `${baseUrl}${source.pathname}${source.search}` })
    })
  }

  await page.goto(`${frontendUrl}/c/${encodeURIComponent(serviceCode)}`)
  await expect(page.getByText(customerDeviceNo, { exact: true }).first()).toBeVisible({ timeout: 30_000 })
  await expect(page.getByRole("button", { name: "登录后确认设备", exact: true })).toBeVisible()
  await expect(page.locator('[contenteditable="true"]')).toHaveCount(0)

  await page.getByRole("button", { name: "登录后确认设备", exact: true }).click()
  await page.waitForURL((url) => url.pathname === "/dashboard/login", { timeout: 15_000 })
  const loginUrl = new URL(page.url())
  expect(loginUrl.searchParams.get("portal")).toBe("customer")
  expect(loginUrl.searchParams.get("next")).toBe(`/c/${serviceCode}`)

  await page.locator('input[name="username"]').fill(customerUsername)
  await page.locator('input[name="password"]').fill(customerPassword)
  await page.locator('form button[type="submit"]').click()
  await page.waitForURL((url) => url.pathname === "/customer/chat" && Number(url.searchParams.get("conversationId")) > 0, {
    timeout: 30_000,
  })

  const conversationId = Number(new URL(page.url()).searchParams.get("conversationId"))
  const result = await page.evaluate(async ({ expectedConversationId }) => {
    const rawSession = window.localStorage.getItem("remote-helpdesk-session")
    if (!rawSession) throw new Error("customer session is missing after service-code login")
    const session = JSON.parse(rawSession) as Session
    const response = await fetch(`/api/conversation/${expectedConversationId}`, {
      headers: {
        Authorization: `Bearer ${session.accessToken}`,
        "X-Tenant-Id": String(session.tenantId),
      },
    })
    return {
      status: response.status,
      session,
      payload: await response.json() as Envelope<Conversation>,
    }
  }, { expectedConversationId: conversationId })
  expect(result.status).toBe(200)
  expect(result.session.accessToken).toBeTruthy()
  expect(result.session.domainType).toBe("customer")
  expect(result.session.tenantId).toBe(tenantId)
  expect(result.payload.success, result.payload.message).toBe(true)
  expect(result.payload.data).toMatchObject({
    id: conversationId,
    deviceId: expectedDeviceId,
    productId: requiredPositiveInt("E2E_COLLAB_PRODUCT_ID"),
  })

  const closeResponse = await page.request.post(`${baseUrl}/api/conversation/close`, {
    data: { conversationId },
    headers: {
      Authorization: `Bearer ${result.session.accessToken}`,
      "X-Tenant-Id": String(result.session.tenantId),
    },
  })
  const closePayload = await closeResponse.json() as Envelope<unknown>
  expect(closeResponse.ok(), closePayload.message).toBe(true)
  expect(closePayload.success, closePayload.message).toBe(true)
})
