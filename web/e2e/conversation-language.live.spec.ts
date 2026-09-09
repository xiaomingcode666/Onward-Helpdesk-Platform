import { expect, test, type APIRequestContext, type Page } from "@playwright/test"

const baseUrl = (
  process.env.E2E_FRONTEND_URL ??
  process.env.E2E_BASE_URL ??
  "https://remotehelpdesk.digintelspace.com:8443"
).replace(/\/$/, "")

const customerUsername = process.env.E2E_CUSTOMER_USERNAME ?? "e2e.t1.customer.real"
const customerPassword = process.env.E2E_CUSTOMER_PASSWORD ?? process.env.NEXT_PUBLIC_LOCAL_LOGIN_CUSTOMER_PASSWORD ?? ""
const engineerUsername = process.env.E2E_ENGINEER_USERNAME ?? "e2e.product1.engineer"
const engineerPassword = process.env.E2E_ENGINEER_PASSWORD ?? process.env.NEXT_PUBLIC_LOCAL_LOGIN_ENTERPRISE_PASSWORD ?? ""
const customerDeviceNo = process.env.E2E_CUSTOMER_DEVICE_NO ?? "T1-PRESS-REAL-002"

type Envelope<T> = {
  success: boolean
  data: T
  message?: string
}

type Session = {
  accessToken: string
  tenantId: number
  domainType: string
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
  senderType: string
  content: string
}

type Translation = {
  id: number
  message_id: number
  source_language: string
  target_language: string
  translated_text: string
  cached: boolean
}

type Ticket = {
  id: number
  status: string
  conversation_id: number
}

async function login(request: APIRequestContext, username: string, password: string, domainType: "customer" | "enterprise") {
  const response = await request.post(`${baseUrl}/api/auth/login`, {
    data: { username, password, domainType },
  })
  const payload = await response.json() as Envelope<Session>
  expect(response.ok(), payload.message).toBe(true)
  expect(payload.success, payload.message).toBe(true)
  expect(payload.data.domainType).toBe(domainType)
  return payload.data
}

async function loginThroughPortal(page: Page, username: string, password: string, nextPath: string) {
  await page.goto(
    `${baseUrl}/dashboard/login?portal=enterprise&next=${encodeURIComponent(nextPath)}`,
    { waitUntil: "domcontentloaded" },
  )
  await page.locator('input[name="username"]').fill(username)
  await page.locator('input[name="password"]').fill(password)
  await page.locator('form button[type="submit"]').click()
  await page.waitForURL((url) => !url.pathname.startsWith("/dashboard/login"), { timeout: 30_000 })
  const briefing = page.getByRole("dialog", { name: /工程师工作确认|工作状态提醒/ })
  if (await briefing.isVisible().catch(() => false)) {
    await page.keyboard.press("Escape")
  }
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

function visibleText(value: string) {
  return value.replace(/<[^>]+>/g, " ").replace(/\s+/g, " ").trim()
}

function scriptCount(value: string, script: "Han" | "Latin") {
  return (value.match(new RegExp(`\\p{Script=${script}}`, "gu")) ?? []).length
}

test("英文客户问题得到英文 AI 回复且可正确翻译为中文", async ({ page, request }) => {
  test.setTimeout(180_000)
  expect(customerPassword, "缺少正式客户密码").toBeTruthy()
  expect(engineerPassword, "缺少正式工程师密码").toBeTruthy()

  const [customer, engineer] = await Promise.all([
    login(request, customerUsername, customerPassword, "customer"),
    login(request, engineerUsername, engineerPassword, "enterprise"),
  ])
  expect(engineer.tenantId).toBe(customer.tenantId)

  const devices = await call<Device[]>(request, customer, "/api/customer/v1/devices")
  const device = devices.find((item) => item.device_no === customerDeviceNo)
  expect(device, `未找到正式客户设备 ${customerDeviceNo}`).toBeTruthy()

  const conversation = await call<Conversation>(request, customer, "/api/conversation/create_or_match", {
    method: "POST",
    data: { deviceId: device!.id, forceNew: true },
  })

  let ticketId = 0

  try {
    const runTag = `E2E-LANGUAGE-${Date.now()}`
    const question = await call<Message>(request, customer, "/api/message/send", {
      method: "POST",
      data: {
        conversationId: conversation.id,
        clientMsgId: `${runTag}-customer`,
        messageType: "text",
        content: `${runTag} The device shows fault code RHD-FLOW-ALPHA-7742. The red light stays on after reset. Can I safely power it on again?`,
        payload: "",
      },
    })

    let aiReply: Message | undefined
    await expect.poll(async () => {
      const page = await call<{ results?: Message[] }>(
        request,
        customer,
        `/api/message/list?conversationId=${conversation.id}&limit=100`,
      )
      aiReply = (page.results ?? []).find((item) => item.id > question.id && item.senderType === "ai")
      return aiReply?.content ?? ""
    }, {
      message: "等待英文客户问题的 AI 回复",
      timeout: 90_000,
      intervals: [500, 1_000, 2_000],
    }).not.toBe("")

    const replyText = visibleText(aiReply!.content)
    expect(scriptCount(replyText, "Latin"), replyText).toBeGreaterThan(20)
    expect(scriptCount(replyText, "Han"), replyText).toBe(0)

    await call<unknown>(request, customer, "/api/conversation/request_human", {
      method: "POST",
      data: { conversationId: conversation.id, reason: `${runTag} English support handoff` },
    })

    let ticket: Ticket | undefined
    await expect.poll(async () => {
      const page = await call<{ items: Ticket[] }>(
        request,
        engineer,
        `/api/enterprise/v1/tickets?page=1&page_size=20&conversation_id=${conversation.id}`,
      )
      ticket = page.items.find((item) => item.conversation_id === conversation.id)
      return ticket?.id ?? 0
    }, {
      message: "等待英文客户会话进入工程师工单队列",
      timeout: 30_000,
      intervals: [500, 1_000],
    }).toBeGreaterThan(0)
    ticketId = ticket!.id

    if (!["accepted", "in_progress", "processing"].includes(ticket!.status)) {
      await call<unknown>(request, engineer, `/api/enterprise/v1/tickets/${ticketId}/_accept`, {
        method: "POST",
      })
    }

    await loginThroughPortal(
      page,
      engineerUsername,
      engineerPassword,
      `/enterprise/ticket-workbench?ticket_id=${ticketId}`,
    )
    const aiBubble = page.locator('[data-sender-type="ai"]').filter({
      hasText: replyText.slice(0, 80),
    }).last()
    await expect(aiBubble).toBeVisible({ timeout: 30_000 })
    await expect(page.getByLabel("翻译目标语言")).toHaveValue("zh-CN")

    const [translationResponse] = await Promise.all([
      page.waitForResponse((response) =>
        response.request().method() === "POST" &&
        new URL(response.url()).pathname === `/api/enterprise/v1/conversations/${conversation.id}/_translate`,
      ),
      aiBubble.getByRole("button", { name: "翻译消息" }).click(),
    ])
    const translationPayload = await translationResponse.json() as Envelope<Translation>
    expect(translationResponse.ok(), translationPayload.message).toBe(true)
    expect(translationPayload.success, translationPayload.message).toBe(true)
    const translation = translationPayload.data
    expect(translation.id).toBeGreaterThan(0)
    expect(translation.message_id).toBe(aiReply!.id)
    expect(translation.source_language).toBe("en")
    expect(translation.target_language).toBe("zh-CN")
    expect(scriptCount(translation.translated_text, "Han"), translation.translated_text).toBeGreaterThan(5)
    await expect(aiBubble.getByText("中文译文", { exact: true })).toBeVisible()
    await expect(aiBubble).toContainText(translation.translated_text)

    const messagesAfterTranslation = await call<{ results?: Message[] }>(
      request,
      customer,
      `/api/message/list?conversationId=${conversation.id}&limit=100`,
    )
    expect(messagesAfterTranslation.results?.find((item) => item.id === aiReply!.id)?.content).toBe(aiReply!.content)

    const cachedTranslation = await call<Translation>(
      request,
      engineer,
      `/api/enterprise/v1/conversations/${conversation.id}/_translate`,
      {
        method: "POST",
        data: { message_id: aiReply!.id, target_language: "zh-CN" },
      },
    )
    expect(cachedTranslation.id).toBe(translation.id)
    expect(cachedTranslation.message_id).toBe(aiReply!.id)
    expect(cachedTranslation.translated_text).toBe(translation.translated_text)
    expect(cachedTranslation.cached).toBe(true)
  } finally {
    if (ticketId > 0) {
      await request.post(`${baseUrl}/api/enterprise/v1/tickets/${ticketId}/_cancel`, {
        data: { reason: "E2E language regression cleanup" },
        headers: {
          Authorization: `Bearer ${engineer.accessToken}`,
          "X-Tenant-Id": String(engineer.tenantId),
        },
      })
    }
    await call<void>(request, customer, "/api/conversation/close", {
      method: "POST",
      data: { conversationId: conversation.id },
    })
  }
})
