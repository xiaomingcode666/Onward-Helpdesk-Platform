import { expect, test, type APIRequestContext } from "@playwright/test"

const baseUrl = process.env.E2E_BASE_URL ?? "http://localhost:3000"

type Fixture = {
  tenantId: number
  productId: number
  customerUsername: string
  customerPassword: string
  customerDeviceNo: string
  adminUsername: string
  adminPassword: string
  engineerUsername: string
  engineerPassword: string
  supplierUsername: string
  supplierPassword: string
  supplierModule: string
}

const tenantOne: Fixture = {
  tenantId: Number(process.env.E2E_T1_TENANT_ID ?? "1"),
  productId: Number(process.env.E2E_T1_PRODUCT_ID ?? "1"),
  customerUsername: process.env.E2E_T1_CUSTOMER_USERNAME ?? "",
  customerPassword: process.env.E2E_T1_CUSTOMER_PASSWORD ?? "",
  customerDeviceNo: process.env.E2E_T1_CUSTOMER_DEVICE_NO ?? "",
  adminUsername: process.env.E2E_T1_ADMIN_USERNAME ?? "",
  adminPassword: process.env.E2E_T1_ADMIN_PASSWORD ?? "",
  engineerUsername: process.env.E2E_T1_ENGINEER_USERNAME ?? "",
  engineerPassword: process.env.E2E_T1_ENGINEER_PASSWORD ?? "",
  supplierUsername: process.env.E2E_T1_SUPPLIER_USERNAME ?? "",
  supplierPassword: process.env.E2E_T1_SUPPLIER_PASSWORD ?? "",
  supplierModule: process.env.E2E_T1_SUPPLIER_MODULE ?? "",
}

const tenantTwo: Fixture = {
  tenantId: Number(process.env.E2E_T2_TENANT_ID ?? "2"),
  productId: Number(process.env.E2E_T2_PRODUCT_ID ?? "4"),
  customerUsername: process.env.E2E_T2_CUSTOMER_USERNAME ?? "",
  customerPassword: process.env.E2E_T2_CUSTOMER_PASSWORD ?? "",
  customerDeviceNo: process.env.E2E_T2_CUSTOMER_DEVICE_NO ?? "",
  adminUsername: process.env.E2E_T2_ADMIN_USERNAME ?? "",
  adminPassword: process.env.E2E_T2_ADMIN_PASSWORD ?? "",
  engineerUsername: process.env.E2E_T2_ENGINEER_USERNAME ?? "",
  engineerPassword: process.env.E2E_T2_ENGINEER_PASSWORD ?? "",
  supplierUsername: process.env.E2E_T2_SUPPLIER_USERNAME ?? "",
  supplierPassword: process.env.E2E_T2_SUPPLIER_PASSWORD ?? "",
  supplierModule: process.env.E2E_T2_SUPPLIER_MODULE ?? "",
}

type Session = {
  accessToken: string
  tenantId: number
}

type Envelope<T> = {
  success: boolean
  errorCode: number
  message: string
  data: T
}

type FixtureSessions = {
  customer: Session
  admin: Session
  engineer: Session
  supplier: Session
}

type TenantArtifacts = {
  conversationId: number
  ticketId: number
  collaborationId: number
}

function requireFixtures() {
  const missing: string[] = []
  for (const [prefix, fixture] of [["E2E_T1", tenantOne], ["E2E_T2", tenantTwo]] as const) {
    for (const key of [
      "customerUsername",
      "customerPassword",
      "customerDeviceNo",
      "adminUsername",
      "adminPassword",
      "engineerUsername",
      "engineerPassword",
      "supplierUsername",
      "supplierPassword",
      "supplierModule",
    ] as const) {
      if (!fixture[key]) missing.push(`${prefix}_${key.replace(/[A-Z]/g, (value) => `_${value}`).toUpperCase()}`)
    }
  }
  if (missing.length > 0) throw new Error(`对抗性售后测试缺少环境变量：${missing.join(", ")}`)
}

async function login(request: APIRequestContext, username: string, password: string, domainType: string) {
  const response = await request.post(`${baseUrl}/api/auth/login`, {
    data: { username, password, domainType },
  })
  const payload = await response.json() as Envelope<Session>
  expect(payload.success, payload.message).toBe(true)
  expect(payload.data.accessToken).toBeTruthy()
  return payload.data
}

async function call<T>(
  request: APIRequestContext,
  session: Session,
  path: string,
  options: { method?: "GET" | "POST"; data?: unknown; tenantId?: number; requestId?: string } = {},
) {
  const method = options.method ?? "GET"
  const response = await request.fetch(`${baseUrl}${path}`, {
    method,
    data: options.data,
    headers: {
      Authorization: `Bearer ${session.accessToken}`,
      "X-Tenant-Id": String(options.tenantId ?? session.tenantId),
      ...(options.requestId ? { "X-Request-Id": options.requestId } : {}),
    },
  })
  return response.json() as Promise<Envelope<T>>
}

async function loginFixture(request: APIRequestContext, fixture: Fixture) {
  const [customer, admin, engineer, supplier] = await Promise.all([
    login(request, fixture.customerUsername, fixture.customerPassword, "customer"),
    login(request, fixture.adminUsername, fixture.adminPassword, "enterprise"),
    login(request, fixture.engineerUsername, fixture.engineerPassword, "enterprise"),
    login(request, fixture.supplierUsername, fixture.supplierPassword, "partner"),
  ])
  return { customer, admin, engineer, supplier }
}

async function createTenantArtifacts(
  request: APIRequestContext,
  sessions: FixtureSessions,
  fixture: Fixture,
): Promise<TenantArtifacts> {
  type Device = { id: number; device_no: string }
  const devices = await call<Device[]>(request, sessions.customer, "/api/customer/v1/devices")
  expect(devices.success, devices.message).toBe(true)
  const device = devices.data.find((item) => item.device_no === fixture.customerDeviceNo)
  expect(device, `未找到正式设备 ${fixture.customerDeviceNo}`).toBeTruthy()

  type Conversation = { id: number }
  const runTag = `ADVERSARIAL-ISOLATION-${fixture.tenantId}-${Date.now()}`
  const conversation = await call<Conversation>(request, sessions.customer, "/api/conversation/create_or_match", {
    method: "POST",
    data: { deviceId: device!.id, forceNew: true },
    requestId: `${runTag}-conversation`,
  })
  expect(conversation.success, conversation.message).toBe(true)

  const initialMessage = await call(request, sessions.customer, "/api/message/send", {
    method: "POST",
    data: {
      conversationId: conversation.data.id,
      messageType: "text",
      content: `${runTag} 设备异常升温，需要建立隔离探测用的真实会话上下文。`,
      clientMsgId: `${runTag}-message`,
    },
  })
  expect(initialMessage.success, initialMessage.message).toBe(true)

  const handoff = await call(request, sessions.customer, "/api/conversation/request_human", {
    method: "POST",
    data: { conversationId: conversation.data.id, reason: `${runTag} request human` },
    requestId: `${runTag}-handoff`,
  })
  expect(handoff.success, handoff.message).toBe(true)

  type TicketPage = { items: Array<{ id: number; status: string; conversation_id: number }> }
  let tickets: TicketPage["items"] = []
  await expect.poll(async () => {
    const result = await call<TicketPage>(
      request,
      sessions.admin,
      `/api/enterprise/v1/tickets?page_size=20&conversation_id=${conversation.data.id}`,
    )
    expect(result.success, result.message).toBe(true)
    tickets = result.data.items.filter((item) => item.conversation_id === conversation.data.id)
    return tickets.length
  }, { timeout: 20_000 }).toBe(1)

  const accept = await call(request, sessions.engineer, `/api/enterprise/v1/tickets/${tickets[0].id}/_accept`, {
    method: "POST",
    requestId: `${runTag}-accept`,
  })
  expect(accept.success, accept.message).toBe(true)

  type ProductModule = { id: number; name: string; module_code: string; default_supplier_id: number }
  const modules = await call<ProductModule[]>(
    request,
    sessions.admin,
    `/api/enterprise/v1/products/${fixture.productId}/modules`,
  )
  expect(modules.success, modules.message).toBe(true)
  const productModule = modules.data.find((item) => item.name === fixture.supplierModule || item.module_code === fixture.supplierModule)
  expect(productModule, `未找到正式供应商模块 ${fixture.supplierModule}`).toBeTruthy()
  expect(productModule!.default_supplier_id).toBeGreaterThan(0)

  type Collaboration = { id: number }
  const collaboration = await call<Collaboration>(
    request,
    sessions.engineer,
    `/api/enterprise/v1/tickets/${tickets[0].id}/supplier-collaborations`,
    {
      method: "POST",
      data: {
        product_module_id: productModule!.id,
        reason: `${runTag} supplier escalation`,
        access_days: 1,
      },
      requestId: `${runTag}-supplier`,
    },
  )
  expect(collaboration.success, collaboration.message).toBe(true)

  type PartnerTicket = { id: number; ticket_id: number }
  const partnerTickets = await call<PartnerTicket[]>(request, sessions.supplier, "/api/partner/v1/tickets")
  expect(partnerTickets.success, partnerTickets.message).toBe(true)
  expect(partnerTickets.data.some((item) => item.id === collaboration.data.id)).toBe(true)

  return {
    conversationId: conversation.data.id,
    ticketId: tickets[0].id,
    collaborationId: collaboration.data.id,
  }
}

async function cleanupTenantArtifacts(
  request: APIRequestContext,
  sessions: FixtureSessions,
  artifacts: TenantArtifacts,
) {
  await call(request, sessions.admin, `/api/enterprise/v1/tickets/${artifacts.ticketId}/_cancel`, {
    method: "POST",
    data: { reason: "对抗性隔离测试完成后清理" },
    requestId: `adversarial-isolation-cleanup-ticket-${artifacts.ticketId}`,
  }).catch(() => undefined)
  await call(request, sessions.customer, "/api/conversation/close", {
    method: "POST",
    data: { conversationId: artifacts.conversationId },
    requestId: `adversarial-isolation-cleanup-conversation-${artifacts.conversationId}`,
  }).catch(() => undefined)
}

test("两个正式租户不能探测彼此的售后与知识资源", async ({ request }) => {
  test.setTimeout(180_000)
  requireFixtures()
  const cleanupQueue: Array<{ sessions: FixtureSessions; artifacts: TenantArtifacts }> = []
  const [one, two] = await Promise.all([
    loginFixture(request, tenantOne),
    loginFixture(request, tenantTwo),
  ])

  try {
    const oneArtifacts = await createTenantArtifacts(request, one, tenantOne)
    cleanupQueue.push({ sessions: one, artifacts: oneArtifacts })
    const twoArtifacts = await createTenantArtifacts(request, two, tenantTwo)
    cleanupQueue.push({ sessions: two, artifacts: twoArtifacts })

    type KnowledgeBase = { id: number; name: string }
    const [oneBases, twoBases] = await Promise.all([
      call<KnowledgeBase[]>(request, one.admin, "/api/dashboard/knowledge-base/list_all"),
      call<KnowledgeBase[]>(request, two.admin, "/api/dashboard/knowledge-base/list_all"),
    ])
    expect(oneBases.success, oneBases.message).toBe(true)
    expect(twoBases.success, twoBases.message).toBe(true)
    expect(oneBases.data.length).toBeGreaterThan(0)
    expect(twoBases.data.length).toBeGreaterThan(0)
    const oneBaseIDs = new Set(oneBases.data.map((item) => item.id))
    expect(twoBases.data.every((item) => !oneBaseIDs.has(item.id))).toBe(true)

    const spoofedBases = await call<KnowledgeBase[]>(
      request,
      one.admin,
      "/api/dashboard/knowledge-base/list_all",
      { tenantId: tenantTwo.tenantId },
    )
    expect(spoofedBases.success, spoofedBases.message).toBe(true)
    expect(spoofedBases.data.map((item) => item.id)).toEqual(oneBases.data.map((item) => item.id))
    const foreignBase = await call<KnowledgeBase>(
      request,
      one.admin,
      `/api/dashboard/knowledge-base/${twoBases.data[0].id}`,
    )
    expect(foreignBase.success).toBe(false)

    const foreignConversation = await call(
      request,
      one.customer,
      `/api/conversation/${twoArtifacts.conversationId}`,
    )
    expect(foreignConversation.success).toBe(false)
    const foreignMessage = await call(
      request,
      one.customer,
      "/api/message/send",
      {
        method: "POST",
        data: {
          conversationId: twoArtifacts.conversationId,
          messageType: "text",
          content: "cross-tenant-probe",
          clientMsgId: `cross-tenant-probe-${Date.now()}`,
        },
      },
    )
    expect(foreignMessage.success).toBe(false)
    expect(foreignMessage.message).not.toContain("已关闭")

    const foreignTicket = await call(
      request,
      one.admin,
      `/api/enterprise/v1/tickets/${twoArtifacts.ticketId}`,
      { tenantId: tenantTwo.tenantId },
    )
    expect(foreignTicket.success).toBe(false)

    const foreignCollaboration = await call(
      request,
      one.supplier,
      `/api/partner/v1/tickets/${twoArtifacts.collaborationId}`,
      { tenantId: tenantTwo.tenantId },
    )
    expect(foreignCollaboration.success).toBe(false)
  } finally {
    for (const item of cleanupQueue.reverse()) {
      await cleanupTenantArtifacts(request, item.sessions, item.artifacts)
    }
  }
})

for (const fixture of [tenantOne, tenantTwo]) {
  test(`租户 ${fixture.tenantId} 正式客户并发请求人工只创建一个工单和一条转人工记录`, async ({ request }) => {
  test.setTimeout(120_000)
  requireFixtures()
  const { customer, admin } = await loginFixture(request, fixture)
  let conversationID = 0
  let ticketID = 0
  try {
    type Device = { id: number; device_no: string }
    const devices = await call<Device[]>(request, customer, "/api/customer/v1/devices")
    const device = devices.data.find((item) => item.device_no === fixture.customerDeviceNo)
    expect(device, `未找到正式设备 ${fixture.customerDeviceNo}`).toBeTruthy()

    type Conversation = { id: number }
    const conversation = await call<Conversation>(request, customer, "/api/conversation/create_or_match", {
      method: "POST",
      data: { deviceId: device!.id },
      requestId: `adversarial-conversation-${Date.now()}`,
    })
    expect(conversation.success, conversation.message).toBe(true)
    conversationID = conversation.data.id
    const runTag = `ADVERSARIAL-${Date.now()}`
    const initialMessage = await call(request, customer, "/api/message/send", {
      method: "POST",
      data: {
        conversationId: conversation.data.id,
        messageType: "text",
        content: `${runTag} 设备出现间歇性异响，本条仅建立故障上下文。`,
        clientMsgId: `${runTag}-message`,
      },
    })
    expect(initialMessage.success, initialMessage.message).toBe(true)

    const handoffs = await Promise.all(
      Array.from({ length: 8 }, (_, index) => call(
        request,
        customer,
        "/api/conversation/request_human",
        {
          method: "POST",
          data: { conversationId: conversation.data.id, reason: `${runTag} request ${index + 1}` },
          requestId: `${runTag}-request-${index + 1}`,
        },
      )),
    )
    expect(
      handoffs.every((item) => item.success),
      handoffs.map((item) => item.message).join("; "),
    ).toBe(true)

    type TicketPage = { items: Array<{ id: number; status: string; conversation_id: number }> }
    let tickets: TicketPage["items"] = []
    await expect.poll(async () => {
      const result = await call<TicketPage>(
        request,
        admin,
        `/api/enterprise/v1/tickets?page_size=20&conversation_id=${conversation.data.id}`,
      )
      tickets = result.data.items.filter((item) => item.conversation_id === conversation.data.id)
      return tickets.length
    }, { timeout: 20_000 }).toBe(1)
    ticketID = tickets[0].id

    type MessagePage = { results: Array<{ content: string; senderType: string }> }
    const messages = await call<MessagePage>(
      request,
      customer,
      `/api/message/list?conversationId=${conversation.data.id}&limit=100`,
    )
    const requestNotices = messages.data.results.filter((item) =>
      item.senderType === "system" && item.content === "客户主动请求转接技术工程师，系统正在安排人工支持。"
    )
    expect(requestNotices).toHaveLength(1)
  } finally {
    if (ticketID > 0) {
      const cleanup = await call(request, admin, `/api/enterprise/v1/tickets/${ticketID}/_cancel`, {
        method: "POST",
        data: { reason: "对抗性并发测试完成后清理" },
        requestId: `adversarial-cleanup-ticket-${ticketID}`,
      })
      expect(cleanup.success, cleanup.message).toBe(true)
    }
    if (conversationID > 0) {
      const close = await call(request, customer, "/api/conversation/close", {
        method: "POST",
        data: { conversationId: conversationID },
        requestId: `adversarial-cleanup-conversation-${conversationID}`,
      })
      expect(close.success, close.message).toBe(true)
    }
  }
  })
}
