import { expect, test, type Browser, type BrowserContext, type Page } from "@playwright/test"

const baseUrl = process.env.E2E_BASE_URL ?? "http://localhost:3000"
const frontendUrl = process.env.E2E_FRONTEND_URL ?? baseUrl
const productId = Number(process.env.E2E_PRODUCT_ID ?? "0")
const seedKnowledgeDocumentId = Number(process.env.E2E_SEED_KNOWLEDGE_DOCUMENT_ID ?? "0")
const customerUsername = process.env.E2E_CUSTOMER_USERNAME ?? ""
const customerPassword = process.env.E2E_CUSTOMER_PASSWORD ?? ""
const customerDeviceNo = process.env.E2E_CUSTOMER_DEVICE_NO ?? ""
const engineerUsername = process.env.E2E_ENGINEER_USERNAME ?? ""
const engineerPassword = process.env.E2E_ENGINEER_PASSWORD ?? ""
const supplierUsername = process.env.E2E_SUPPLIER_USERNAME ?? ""
const supplierPassword = process.env.E2E_SUPPLIER_PASSWORD ?? ""
const supplierModule = process.env.E2E_SUPPLIER_MODULE ?? ""
const adminUsername = process.env.E2E_ADMIN_USERNAME ?? ""
const adminPassword = process.env.E2E_ADMIN_PASSWORD ?? ""
const faultCode = process.env.E2E_FAULT_CODE ?? "RHD-FLOW-ALPHA-7742"
const expectedResetSeconds = process.env.E2E_EXPECTED_RESET_SECONDS ?? "30"
const manageDispatchFixture = process.env.E2E_MANAGE_DISPATCH_FIXTURE !== "0"
const manageEngineerStatus = process.env.E2E_MANAGE_ENGINEER_STATUS !== "0"
const publishKnowledgeCandidate = process.env.E2E_PUBLISH_KNOWLEDGE_CANDIDATE !== "0"
const requiresAdmin = manageDispatchFixture || publishKnowledgeCandidate

function requireLiveFixture() {
  const missing = Object.entries({
    E2E_PRODUCT_ID: productId > 0 ? String(productId) : "",
    E2E_SEED_KNOWLEDGE_DOCUMENT_ID: publishKnowledgeCandidate && seedKnowledgeDocumentId <= 0
      ? ""
      : String(seedKnowledgeDocumentId),
    E2E_CUSTOMER_USERNAME: customerUsername,
    E2E_CUSTOMER_PASSWORD: customerPassword,
    E2E_CUSTOMER_DEVICE_NO: customerDeviceNo,
    E2E_ENGINEER_USERNAME: engineerUsername,
    E2E_ENGINEER_PASSWORD: engineerPassword,
    E2E_SUPPLIER_USERNAME: supplierUsername,
    E2E_SUPPLIER_PASSWORD: supplierPassword,
    E2E_SUPPLIER_MODULE: supplierModule,
    E2E_ADMIN_USERNAME: requiresAdmin ? adminUsername : "not-required",
    E2E_ADMIN_PASSWORD: requiresAdmin ? adminPassword : "not-required",
  }).filter(([, value]) => !value).map(([name]) => name)
  if (missing.length > 0) {
    throw new Error(`真实售后闭环缺少环境变量：${missing.join(", ")}`)
  }
}

async function login(page: Page, nextPath: string, username: string, password: string) {
  const portal = nextPath.startsWith("/customer")
    ? "customer"
    : nextPath.startsWith("/partner")
      ? "partner"
      : "enterprise"
  await page.goto(`${frontendUrl}/dashboard/login?portal=${portal}&next=${encodeURIComponent(nextPath)}`)
  await page.locator('input[name="username"]').fill(username)
  await page.locator('input[name="password"]').fill(password)
  await page.locator('form button[type="submit"]').click()
  await page.waitForURL(
    (url) => !url.pathname.startsWith("/dashboard/login"),
    { timeout: 15_000 },
  ).catch(async (error) => {
    const visibleError = await page.locator('[data-sonner-toast], [role="alert"]').allTextContents()
    throw new Error(
      `E2E 登录失败：${username} (${portal})${visibleError.length ? ` · ${visibleError.join("；")}` : ""}`,
      { cause: error },
    )
  })
  await expect(page.getByText("无权访问此页面", { exact: true })).toHaveCount(0)
}

async function routeBrowserTrafficToBackend(page: Page) {
  if (frontendUrl === baseUrl) return

  await page.route(`${frontendUrl}/api/**`, async (route) => {
    const source = new URL(route.request().url())
    await route.continue({ url: `${baseUrl}${source.pathname}${source.search}` })
  })
  await page.addInitScript(({ backendUrl }) => {
    const backend = new URL(backendUrl)
    const NativeWebSocket = window.WebSocket
    window.WebSocket = class E2EWebSocket extends NativeWebSocket {
      constructor(url: string | URL, protocols?: string | string[]) {
        const target = new URL(String(url), window.location.href)
        if (target.pathname.startsWith("/api/ws/")) {
          target.protocol = backend.protocol === "https:" ? "wss:" : "ws:"
          target.host = backend.host
        }
        if (protocols === undefined) {
          super(target.toString())
        } else {
          super(target.toString(), protocols)
        }
      }
    }
  }, { backendUrl: baseUrl })
}

function messageFilename(page: Page, filename: string) {
  return page.locator("[data-sender-type]").getByText(filename, { exact: true })
}

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")
}

async function expectImageReady(page: Page, filename: string) {
  await expect(async () => {
    const image = page.locator(`img[alt="${filename}"]`).last()
    const stateHost = page.locator(".im-image").filter({ has: image }).last()
    await expect(stateHost).toBeVisible({ timeout: 1_000 })
    await stateHost.scrollIntoViewIfNeeded({ timeout: 1_000 })
    await expect(image).toHaveAttribute("data-media-state", "ready", { timeout: 1_000 })
    await expect(image).toBeVisible({ timeout: 1_000 })
  }).toPass({
    intervals: [250, 500, 1_000],
    timeout: 30_000,
  })
  return page.locator(`img[alt="${filename}"]`).last()
}

async function sendCustomerMessage(page: Page, content: string) {
  const editor = page.locator('[contenteditable="true"]').last()
  await expect(editor).toBeVisible()
  await editor.fill(content)
  await page.getByRole("button", { name: "发送", exact: true }).click()
}

async function createRolePage(browser: Browser, viewport: { width: number; height: number }) {
  const context = await browser.newContext({ viewport, permissions: ["camera", "microphone"] })
  return { context, page: await context.newPage() }
}

async function prepareMeetingPopup(page: Page) {
  await routeBrowserTrafficToBackend(page)
  const target = page.url()
  if (target && target !== "about:blank") {
    await page.goto(target, { waitUntil: "domcontentloaded" })
  } else {
    await page.waitForLoadState("domcontentloaded").catch(() => undefined)
  }
}

async function enterMeetingRoomAndWaitJoined(page: Page, roleName: string, joinedUrlPattern: RegExp) {
  await prepareMeetingPopup(page)
  const enterButton = page.getByRole("button", { name: "进入会议", exact: true })
  await expect(enterButton, `${roleName}应看到同源会议预入会按钮`).toBeVisible({ timeout: 45_000 })
  const [joinedResponse] = await Promise.all([
    page.waitForResponse((response) =>
      response.request().method() === "POST" && joinedUrlPattern.test(response.url()),
      { timeout: 90_000 },
    ),
    enterButton.click(),
  ])
  expect(joinedResponse.ok(), `${roleName}入会状态回写应返回成功`).toBe(true)
  await expect(page.getByText("正在进入会议...", { exact: true })).toHaveCount(0, { timeout: 45_000 })
}

async function closeRoleContexts(contexts: BrowserContext[]) {
  await Promise.allSettled(contexts.map(async (context) => {
    await Promise.allSettled(context.pages().map((page) => Promise.race([
      page.close({ runBeforeUnload: false }),
      new Promise<void>((resolve) => setTimeout(resolve, 2_000)),
    ])))
    await Promise.race([
      context.close(),
      new Promise<void>((resolve) => setTimeout(resolve, 5_000)),
    ])
  }))
}

async function waitForSeedKnowledgeIndex(page: Page) {
  await page.evaluate(async ({ expectedProductId, expectedDocumentId }) => {
    const rawSession = window.localStorage.getItem("remote-helpdesk-session")
    if (!rawSession) throw new Error("E2E knowledge index session is missing")
    const session = JSON.parse(rawSession) as { accessToken?: string; tenantId?: number }
    if (!session.accessToken || !session.tenantId) throw new Error("E2E knowledge index session is invalid")
    const deadline = Date.now() + 120_000
    while (Date.now() < deadline) {
      const response = await fetch(`/api/enterprise/v1/knowledge/index-tasks?product_id=${expectedProductId}`, {
        headers: {
          Authorization: `Bearer ${session.accessToken}`,
          "X-Tenant-Id": String(session.tenantId),
        },
      })
      const payload = await response.json() as {
        success?: boolean
        message?: string
        data?: Array<{ subject_type: string; subject_id: number; status: string; error_summary: string }>
      }
      if (!response.ok || !payload.success) {
        throw new Error(payload.message || `E2E knowledge index request failed: ${response.status}`)
      }
      const task = payload.data?.find((item) =>
        item.subject_type === "knowledge_document" && item.subject_id === expectedDocumentId
      )
      if (task?.status === "succeeded") return
      if (task?.status === "failed") {
        throw new Error(`E2E seed knowledge index failed: ${task.error_summary || "unknown error"}`)
      }
      await new Promise((resolve) => window.setTimeout(resolve, 1_000))
    }
    throw new Error("E2E seed knowledge index did not complete")
  }, { expectedProductId: productId, expectedDocumentId: seedKnowledgeDocumentId })
}

type LiveMediaRole = "customer" | "engineer" | "supplier"
type LiveMediaKind = "image" | "audio" | "attachment"

async function sendLiveMediaMessage(
  page: Page,
  input: {
    role: LiveMediaRole
    kind: LiveMediaKind
    conversationId: number
    collaborationId?: number
    filename: string
  },
) {
  return page.evaluate(async (media) => {
    const rawSession = window.localStorage.getItem("remote-helpdesk-session")
    if (!rawSession) throw new Error("E2E media session is missing")
    const session = JSON.parse(rawSession) as { accessToken?: string; tenantId?: number }
    if (!session.accessToken || !session.tenantId) throw new Error("E2E media session is invalid")
    const authHeaders = {
      Authorization: `Bearer ${session.accessToken}`,
      "X-Tenant-Id": String(session.tenantId),
    }
    const requestJSON = async <T>(url: string, init: RequestInit): Promise<T> => {
      const response = await fetch(url, init)
      const payload = await response.json() as {
        success?: boolean
        message?: string
        data?: T
      }
      if (!response.ok || !payload.success || !payload.data) {
        throw new Error(payload.message || `E2E media request failed: ${response.status} ${url}`)
      }
      return payload.data
    }
    const makeWave = () => {
      const sampleRate = 8_000
      const samples = sampleRate
      const dataLength = samples * 2
      const buffer = new ArrayBuffer(44 + dataLength)
      const view = new DataView(buffer)
      const writeASCII = (offset: number, value: string) => {
        for (let index = 0; index < value.length; index += 1) {
          view.setUint8(offset + index, value.charCodeAt(index))
        }
      }
      writeASCII(0, "RIFF")
      view.setUint32(4, 36 + dataLength, true)
      writeASCII(8, "WAVE")
      writeASCII(12, "fmt ")
      view.setUint32(16, 16, true)
      view.setUint16(20, 1, true)
      view.setUint16(22, 1, true)
      view.setUint32(24, sampleRate, true)
      view.setUint32(28, sampleRate * 2, true)
      view.setUint16(32, 2, true)
      view.setUint16(34, 16, true)
      writeASCII(36, "data")
      view.setUint32(40, dataLength, true)
      return buffer
    }
    const onePixelPNG = Uint8Array.from(
      atob("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9Z1S8AAAAASUVORK5CYII="),
      (value) => value.charCodeAt(0),
    )
    const file = media.kind === "image"
      ? new File([onePixelPNG], media.filename, { type: "image/png" })
      : media.kind === "audio"
        ? new File([makeWave()], media.filename, { type: "audio/wav" })
        : new File(["E2E 维修测量记录：48.0V，端子温升正常。"], media.filename, { type: "text/plain" })
    const form = new FormData()
    form.set("file", file)
    if (media.role !== "supplier") form.set("conversationId", String(media.conversationId))
    const uploadURL = media.role === "customer"
      ? `/api/message/upload_${media.kind}`
      : media.role === "engineer"
        ? `/api/dashboard/conversation/upload_${media.kind}`
        : `/api/partner/v1/tickets/${media.collaborationId}/upload_${media.kind}`
    const asset = await requestJSON<{ assetId: string; filename: string }>(uploadURL, {
      method: "POST",
      headers: authHeaders,
      body: form,
    })
    const messagePayload = JSON.stringify({
      assetId: asset.assetId,
      ...(media.kind === "audio" ? { durationSeconds: 1 } : {}),
    })
    if (media.role === "supplier") {
      return requestJSON<{ messages: Array<{ id: number }> }>(
        `/api/partner/v1/tickets/${media.collaborationId}/progress`,
        {
          method: "POST",
          headers: { ...authHeaders, "Content-Type": "application/json" },
          body: JSON.stringify({
            message_type: media.kind,
            asset_id: asset.assetId,
            duration_seconds: media.kind === "audio" ? 1 : 0,
          }),
        },
      )
    }
    const sendURL = media.role === "customer"
      ? "/api/message/send"
      : "/api/dashboard/conversation/send_message"
    return requestJSON<{ id: number }>(sendURL, {
      method: "POST",
      headers: { ...authHeaders, "Content-Type": "application/json" },
      body: JSON.stringify({
        conversationId: media.conversationId,
        messageType: media.kind,
        content: asset.filename,
        payload: messagePayload,
        clientMsgId: `e2e_${media.role}_${media.kind}_${Date.now()}`,
      }),
    })
  }, input)
}

async function cleanupInterruptedTicket(page: Page, ticketNo: string) {
	await page.evaluate(async ({ expectedTicketNo }) => {
    const rawSession = window.localStorage.getItem("remote-helpdesk-session")
    if (!rawSession) throw new Error("E2E cleanup admin session is missing")
    const session = JSON.parse(rawSession) as { accessToken?: string; tenantId?: number }
    if (!session.accessToken || !session.tenantId) throw new Error("E2E cleanup admin session is invalid")
    const headers = {
      Authorization: `Bearer ${session.accessToken}`,
      "Content-Type": "application/json",
      "X-Tenant-Id": String(session.tenantId),
    }
    const listResponse = await fetch(
      `/api/enterprise/v1/tickets?page_size=20&search=${encodeURIComponent(expectedTicketNo)}`,
      { headers },
    )
    const listPayload = await listResponse.json() as {
      success?: boolean
      data?: { items?: Array<{ id: number; ticket_no: string; status: string }> }
    }
    const ticket = listPayload.data?.items?.find((item) => item.ticket_no === expectedTicketNo)
    if (!ticket || ["closed", "cancelled", "done"].includes(ticket.status)) return
    const awaitingConfirmation = ["resolved", "pending_customer_confirm"].includes(ticket.status)
    const action = awaitingConfirmation ? "_close" : "_cancel"
    const body = awaitingConfirmation
      ? { resolution: "E2E 测试中断后的残留清理" }
      : { reason: "E2E 测试中断后的残留清理" }
    const cleanupResponse = await fetch(`/api/enterprise/v1/tickets/${ticket.id}/${action}`, {
      method: "POST",
      headers,
      body: JSON.stringify(body),
    })
    const cleanupPayload = await cleanupResponse.json() as { success?: boolean; message?: string }
    if (!cleanupResponse.ok || !cleanupPayload.success) {
      throw new Error(cleanupPayload.message || `E2E ticket cleanup failed with ${cleanupResponse.status}`)
    }
	}, { expectedTicketNo: ticketNo })
}

async function closeCustomerConversationIfPresent(page: Page, conversationId: number) {
	if (!Number.isSafeInteger(conversationId) || conversationId <= 0) return
	await page.evaluate(async ({ targetConversationId }) => {
		const rawSession = window.localStorage.getItem("remote-helpdesk-session")
		if (!rawSession) throw new Error("E2E customer conversation cleanup session is missing")
		const session = JSON.parse(rawSession) as { accessToken?: string; tenantId?: number }
		if (!session.accessToken || !session.tenantId) throw new Error("E2E customer conversation cleanup session is invalid")
		await fetch("/api/conversation/close", {
			method: "POST",
			headers: {
				Authorization: `Bearer ${session.accessToken}`,
				"Content-Type": "application/json",
				"X-Tenant-Id": String(session.tenantId),
			},
			body: JSON.stringify({ conversationId: targetConversationId }),
		})
	}, { targetConversationId: conversationId })
}

async function confirmEngineerAvailable(page: Page) {
  await page.evaluate(async () => {
    const rawSession = window.localStorage.getItem("remote-helpdesk-session")
    if (!rawSession) throw new Error("E2E engineer session is missing")
    const session = JSON.parse(rawSession) as { accessToken?: string; tenantId?: number }
    if (!session.accessToken || !session.tenantId) throw new Error("E2E engineer session is invalid")
    const response = await fetch("/api/dashboard/agent/self/work-status", {
      method: "POST",
      headers: {
        Authorization: `Bearer ${session.accessToken}`,
        "Content-Type": "application/json",
        "X-Tenant-Id": String(session.tenantId),
      },
      body: JSON.stringify({ status: "available", note: "" }),
    })
    const payload = await response.json() as { success?: boolean; message?: string }
    if (!response.ok || !payload.success) {
      throw new Error(payload.message || `E2E engineer work status update failed with ${response.status}`)
    }
  })
  const confirmButton = page.getByRole("button", { name: /进入工作台/ })
  if (await confirmButton.isVisible({ timeout: 5_000 }).catch(() => false)) {
    await confirmButton.click()
  }
  await expect(page.getByRole("dialog", { name: /工程师工作确认/ })).toHaveCount(0, { timeout: 10_000 })
}

type DispatchFixtureSnapshot = {
  engineerDisplayName: string
  profile: {
    id: number
    userId: number
    teamId: number
    agentCode: string
    displayName: string
    avatar: string
    serviceStatus: number
    maxConcurrentCount: number
    priorityLevel: number
    autoAssignEnabled: boolean
    receiveOfflineMessage: boolean
    remark: string
  }
  membership: {
    teamId: number
    userId: number
    dispatchEnabled: boolean
    dispatchWeight: number
  }
}

async function ensureDispatchAvailability(
  page: Page,
  expectedEngineerUsername: string,
  expectedProductId: number,
): Promise<{ teamId: number; engineerDisplayName: string }> {
  return page.evaluate(async ({ username, productID }) => {
    const rawSession = window.localStorage.getItem("remote-helpdesk-session")
    if (!rawSession) throw new Error("E2E dispatch availability admin session is missing")
    const session = JSON.parse(rawSession) as { accessToken?: string; tenantId?: number }
    if (!session.accessToken || session.tenantId === undefined) throw new Error("E2E dispatch availability admin session is invalid")
    const headers = {
      Authorization: `Bearer ${session.accessToken}`,
      "Content-Type": "application/json",
      "X-Tenant-Id": String(session.tenantId),
    }
    const requestJSON = async <T>(url: string, init?: RequestInit): Promise<T> => {
      const response = await fetch(url, { ...init, headers: { ...headers, ...(init?.headers || {}) } })
      const payload = await response.json() as { success?: boolean; message?: string; data?: T }
      if (!response.ok || !payload.success) {
        throw new Error(payload.message || `E2E dispatch availability request failed: ${response.status} ${url}`)
      }
      return payload.data as T
    }
    type Profile = {
      userId: number
      teamId: number
      teamIds?: number[]
      username?: string
      displayName: string
      serviceStatus: number
      maxConcurrentCount: number
      autoAssignEnabled: boolean
      teamDispatchEnabled: boolean
    }
    type Team = { id: number; productId: number }
    type Candidate = {
      userId: number
      workStatusConfirmed: boolean
      reachable: boolean
      capacityAvailable: boolean
    }
    type Ticket = {
      id: number
      status: string
      currentAssigneeId?: number
      current_assignee_id?: number
    }

    const profiles = await requestJSON<Profile[]>("/api/dashboard/agent/list_all")
    const profile = profiles.find((item) => item.username === username)
    if (!profile) throw new Error(`E2E engineer profile not found: ${username}`)
    if (profile.serviceStatus !== 0) throw new Error("E2E engineer is not idle")
    if (!profile.autoAssignEnabled) throw new Error("E2E engineer auto assignment is disabled")
    if (profile.maxConcurrentCount <= 0) throw new Error("E2E engineer dispatch capacity is not configured")

    const candidateTeamIDs = Array.from(new Set([profile.teamId, ...(profile.teamIds || [])]))
      .filter((teamID) => teamID > 0)
    const teams = await Promise.all(
      candidateTeamIDs.map((teamID) => requestJSON<Team>(`/api/dashboard/agent-team/${teamID}`)),
    )
    const team = teams.find((item) => item.productId === productID)
    if (!team) throw new Error(`E2E engineer is not assigned to product ${productID}`)
    const teamProfiles = await requestJSON<Profile[]>(`/api/dashboard/agent/list_all?teamId=${team.id}`)
    const teamProfile = teamProfiles.find((item) => item.userId === profile.userId)
    if (!teamProfile?.teamDispatchEnabled) throw new Error("E2E engineer product-team dispatch is disabled")

    const pending = await requestJSON<{ results?: Array<{ currentAssigneeId: number }> }>(
      `/api/dashboard/conversation/list?status=2&productId=${team.productId}&pageSize=100`,
    )
    if ((pending.results || []).some((item) => !item.currentAssigneeId)) {
      throw new Error("E2E product has pending conversations; refusing to trigger a global dispatch scan")
    }
    const ticketPage = await requestJSON<{ results?: Ticket[] }>(
      `/api/dashboard/ticket/list?productId=${team.productId}&unassigned=1&pageSize=200`,
    )
    const terminalTicketStatuses = new Set(["cancelled", "canceled", "closed", "resolved"])
    const activeUnassignedTickets = (ticketPage.results || []).filter((item) =>
      !terminalTicketStatuses.has(item.status) && !(item.currentAssigneeId ?? item.current_assignee_id ?? 0),
    )
    if (activeUnassignedTickets.length > 0) {
      throw new Error(
        `E2E product has active unassigned tickets; refusing to trigger a global dispatch scan: ${activeUnassignedTickets.map((item) => item.id).join(",")}`,
      )
    }

    const candidates = await requestJSON<Candidate[]>(`/api/dashboard/agent/dispatch-candidates?teamId=${team.id}`)
    const candidate = candidates.find((item) => item.userId === profile.userId)
    if (!candidate) {
      throw new Error("E2E engineer is not an eligible dispatch candidate under personal availability")
    }
    if (!candidate.workStatusConfirmed || !candidate.reachable || !candidate.capacityAvailable) {
      throw new Error("E2E engineer work status, reachability, or capacity is not eligible")
    }
    return { teamId: team.id, engineerDisplayName: profile.displayName }
  }, { username: expectedEngineerUsername, productID: expectedProductId })
}

async function prepareDeterministicDispatchFixture(
  page: Page,
  expectedEngineerUsername: string,
  expectedProductId: number,
): Promise<DispatchFixtureSnapshot> {
  return page.evaluate(async ({ username, productID }) => {
    const rawSession = window.localStorage.getItem("remote-helpdesk-session")
    if (!rawSession) throw new Error("E2E dispatch fixture admin session is missing")
    const session = JSON.parse(rawSession) as { accessToken?: string; tenantId?: number }
    if (!session.accessToken || !session.tenantId) throw new Error("E2E dispatch fixture admin session is invalid")
    const headers = {
      Authorization: `Bearer ${session.accessToken}`,
      "Content-Type": "application/json",
      "X-Tenant-Id": String(session.tenantId),
    }
    const requestJSON = async <T>(url: string, init?: RequestInit): Promise<T> => {
      const response = await fetch(url, { ...init, headers: { ...headers, ...(init?.headers || {}) } })
      const payload = await response.json() as { success?: boolean; message?: string; data?: T }
      if (!response.ok || !payload.success) {
        throw new Error(payload.message || `E2E fixture request failed: ${response.status} ${url}`)
      }
      return payload.data as T
    }
		type Profile = DispatchFixtureSnapshot["profile"] & {
			username?: string
			teamIds?: number[]
			teamDispatchEnabled: boolean
			dispatchWeight?: number
		}
    type Team = { id: number; productId: number }
    type Candidate = {
      userId: number
      workStatusConfirmed: boolean
      reachable: boolean
      capacityAvailable: boolean
    }
    const profiles = await requestJSON<Profile[]>("/api/dashboard/agent/list_all")
    const profile = profiles.find((item) => item.username === username)
    if (!profile) throw new Error(`E2E engineer profile not found: ${username}`)
    const candidateTeamIDs = Array.from(new Set([profile.teamId, ...(profile.teamIds || [])]))
      .filter((teamID) => teamID > 0)
    const teams = await Promise.all(
      candidateTeamIDs.map((teamID) => requestJSON<Team>(`/api/dashboard/agent-team/${teamID}`)),
    )
    const team = teams.find((item) => item.productId === productID)
    if (!team) {
      throw new Error(
        `E2E engineer is not assigned to product ${productID}; candidate teams: ${candidateTeamIDs.join(", ")}`,
      )
    }
    const pending = await requestJSON<{ results?: Array<{ id: number; currentAssigneeId: number }> }>(
      `/api/dashboard/conversation/list?status=2&productId=${team.productId}&pageSize=100`,
    )
    const unassignedPending = (pending.results || []).filter((item) => !item.currentAssigneeId)
    if (unassignedPending.length > 0) {
      throw new Error("E2E product has pending conversations; refusing to trigger a global dispatch scan")
    }
		const teamProfiles = await requestJSON<Profile[]>(`/api/dashboard/agent/list_all?teamId=${team.id}`)
		const teamProfile = teamProfiles.find((item) => item.userId === profile.userId)
		if (!teamProfile) throw new Error("E2E engineer is not an active product-team member")
		if (typeof teamProfile.teamDispatchEnabled !== "boolean") {
			throw new Error("E2E product-team member response is missing teamDispatchEnabled")
		}
		const membership = {
			teamId: team.id,
			userId: profile.userId,
			dispatchEnabled: teamProfile.teamDispatchEnabled,
			dispatchWeight: teamProfile.dispatchWeight || 1,
		}

    const profileSnapshot: DispatchFixtureSnapshot["profile"] = {
      id: profile.id,
      userId: profile.userId,
      teamId: profile.teamId,
      agentCode: profile.agentCode,
      displayName: profile.displayName,
      avatar: profile.avatar || "",
      serviceStatus: profile.serviceStatus,
      maxConcurrentCount: profile.maxConcurrentCount,
      priorityLevel: profile.priorityLevel,
      autoAssignEnabled: profile.autoAssignEnabled,
      receiveOfflineMessage: profile.receiveOfflineMessage,
      remark: profile.remark || "",
    }
    try {
      await requestJSON<void>("/api/dashboard/agent/update", {
        method: "POST",
        body: JSON.stringify({
          ...profileSnapshot,
          serviceStatus: 0,
          maxConcurrentCount: Math.max(profileSnapshot.maxConcurrentCount, 100),
          autoAssignEnabled: true,
        }),
      })
      await requestJSON<void>("/api/dashboard/agent-team/member/upsert", {
        method: "POST",
        body: JSON.stringify({ ...membership, dispatchEnabled: true }),
      })
      const candidates = await requestJSON<Candidate[]>(`/api/dashboard/agent/dispatch-candidates?teamId=${team.id}`)
      const candidate = candidates.find((item) => item.userId === profile.userId)
      if (!candidate) {
        throw new Error("E2E engineer is not an eligible dispatch candidate under personal availability")
      }
      if (!candidate.workStatusConfirmed || !candidate.reachable || !candidate.capacityAvailable) {
        throw new Error("E2E engineer work status, reachability, or capacity is not eligible")
      }
      return {
        engineerDisplayName: profile.displayName,
        profile: profileSnapshot,
        membership,
      }
    } catch (error) {
      await requestJSON<void>("/api/dashboard/agent-team/member/upsert", {
        method: "POST",
        body: JSON.stringify(membership),
      }).catch(() => undefined)
      await requestJSON<void>("/api/dashboard/agent/update", {
        method: "POST",
        body: JSON.stringify(profileSnapshot),
      }).catch(() => undefined)
      throw error
    }
  }, { username: expectedEngineerUsername, productID: expectedProductId })
}

async function restoreDispatchFixture(page: Page, fixture: DispatchFixtureSnapshot) {
  await page.evaluate(async ({ snapshot }) => {
    const rawSession = window.localStorage.getItem("remote-helpdesk-session")
    if (!rawSession) throw new Error("E2E dispatch cleanup admin session is missing")
    const session = JSON.parse(rawSession) as { accessToken?: string; tenantId?: number }
    if (!session.accessToken || !session.tenantId) throw new Error("E2E dispatch cleanup admin session is invalid")
    const headers = {
      Authorization: `Bearer ${session.accessToken}`,
      "Content-Type": "application/json",
      "X-Tenant-Id": String(session.tenantId),
    }
    const request = async (url: string, body: unknown) => {
      const response = await fetch(url, { method: "POST", headers, body: JSON.stringify(body) })
      const payload = await response.json() as { success?: boolean; message?: string }
      if (!response.ok || !payload.success) throw new Error(payload.message || `E2E fixture cleanup failed: ${url}`)
    }
    await request("/api/dashboard/agent-team/member/upsert", snapshot.membership)
    await request("/api/dashboard/agent/update", snapshot.profile)
  }, { snapshot: fixture })
}

type KnowledgeLoopProof = {
  candidateId: number
  knowledgeEntryId: number
  knowledgeBaseId: number
  indexTaskId: number
  searchHitCount: number
  answerModel: string
}

type ClosedLoopResourceProof = {
  meetingCount: number
  collaborationCount: number
}

async function verifyClosedLoopResources(
  adminPage: Page,
  supplierPage: Page,
  customerPage: Page,
  ticketNo: string,
  conversationId: number,
  collaborationId: number,
): Promise<ClosedLoopResourceProof> {
  const adminProof = await adminPage.evaluate(async ({ expectedTicketNo }) => {
    const rawSession = window.localStorage.getItem("remote-helpdesk-session")
    if (!rawSession) throw new Error("E2E close audit admin session is missing")
    const session = JSON.parse(rawSession) as { accessToken?: string; tenantId?: number }
    if (!session.accessToken || !session.tenantId) throw new Error("E2E close audit admin session is invalid")
    const headers = {
      Authorization: `Bearer ${session.accessToken}`,
      "Content-Type": "application/json",
      "X-Tenant-Id": String(session.tenantId),
    }
    const requestPayload = async <T>(url: string, init?: RequestInit) => {
      const response = await fetch(url, { ...init, headers })
      return response.json() as Promise<{ success?: boolean; message?: string; data?: T }>
    }
    const ticketPage = await requestPayload<{ items?: Array<{ id: number; ticket_no: string; status: string }> }>(
      `/api/enterprise/v1/tickets?page_size=20&search=${encodeURIComponent(expectedTicketNo)}`,
    )
    const ticket = ticketPage.data?.items?.find((item) => item.ticket_no === expectedTicketNo)
    if (!ticket || ticket.status !== "closed") throw new Error("E2E close audit did not find the closed ticket")

    const meetings = await requestPayload<Array<{ id: string; status: string; ended_at?: string }>>(
      `/api/enterprise/v1/tickets/${ticket.id}/meetings`,
    )
    if (!meetings.success || !meetings.data?.length) throw new Error("E2E close audit did not find the video meeting")
    if (meetings.data.some((item) => item.status !== "finished" || !item.ended_at)) {
      throw new Error("E2E close audit found a meeting that was not ended")
    }

    const collaborations = await requestPayload<Array<{
      id: number
      status: string
      authorization_active: boolean
      resolved_at?: string
    }>>(`/api/enterprise/v1/tickets/${ticket.id}/supplier-collaborations`)
    if (!collaborations.success || !collaborations.data?.length) {
      throw new Error("E2E close audit did not find supplier collaboration")
    }
    if (collaborations.data.some((item) =>
      item.status !== "resolved" || item.authorization_active || !item.resolved_at
    )) {
      throw new Error("E2E close audit found active supplier authorization")
    }

    const meetingRetry = await requestPayload(`/api/enterprise/v1/tickets/${ticket.id}/meetings`, {
      method: "POST",
      body: JSON.stringify({ title: "closed-ticket-negative-check" }),
    })
    if (meetingRetry.success) throw new Error("closed ticket unexpectedly allowed a new meeting")
    return {
      meetingCount: meetings.data.length,
      collaborationCount: collaborations.data.length,
    }
  }, { expectedTicketNo: ticketNo })

  await supplierPage.evaluate(async ({ expectedTicketNo, expectedCollaborationId }) => {
    const rawSession = window.localStorage.getItem("remote-helpdesk-session")
    if (!rawSession) throw new Error("E2E close audit supplier session is missing")
    const session = JSON.parse(rawSession) as { accessToken?: string; tenantId?: number }
    if (!session.accessToken || !session.tenantId) throw new Error("E2E close audit supplier session is invalid")
    const headers = {
      Authorization: `Bearer ${session.accessToken}`,
      "Content-Type": "application/json",
      "X-Tenant-Id": String(session.tenantId),
    }
    const listResponse = await fetch("/api/partner/v1/tickets", { headers })
    const listPayload = await listResponse.json() as {
      data?: Array<{ id: number; ticket_no: string; status: string; authorization_active: boolean }>
    }
    const collaboration = listPayload.data?.find((item) => item.ticket_no === expectedTicketNo)
    if (collaboration && (collaboration.status !== "resolved" || collaboration.authorization_active)) {
      throw new Error("supplier close audit did not find a revoked collaboration")
    }
    const progressResponse = await fetch(`/api/partner/v1/tickets/${expectedCollaborationId}/progress`, {
      method: "POST",
      headers,
      body: JSON.stringify({ content: "closed-collaboration-negative-check" }),
    })
    const progressPayload = await progressResponse.json() as { success?: boolean }
    if (progressPayload.success) throw new Error("resolved supplier collaboration unexpectedly allowed a reply")
  }, { expectedTicketNo: ticketNo, expectedCollaborationId: collaborationId })

  await customerPage.evaluate(async ({ closedConversationId }) => {
    const rawSession = window.localStorage.getItem("remote-helpdesk-session")
    if (!rawSession) throw new Error("E2E close audit customer session is missing")
    const session = JSON.parse(rawSession) as { accessToken?: string; tenantId?: number }
    if (!session.accessToken || !session.tenantId) throw new Error("E2E close audit customer session is invalid")
    const response = await fetch("/api/message/send", {
      method: "POST",
      headers: {
        Authorization: `Bearer ${session.accessToken}`,
        "Content-Type": "application/json",
        "X-Tenant-Id": String(session.tenantId),
      },
      body: JSON.stringify({
        conversationId: closedConversationId,
        messageType: "text",
        content: "closed-conversation-negative-check",
        clientMsgId: `closed-conversation-negative-check-${closedConversationId}`,
      }),
    })
    const payload = await response.json() as { success?: boolean }
    if (payload.success) throw new Error("closed customer conversation unexpectedly allowed a message")
  }, { closedConversationId: conversationId })

  return adminProof
}

async function approveKnowledgeCandidateAndVerifyRAG(
  page: Page,
  ticketNo: string,
  proof: string,
): Promise<KnowledgeLoopProof> {
  return page.evaluate(async ({ expectedTicketNo, expectedProof }) => {
    const rawSession = window.localStorage.getItem("remote-helpdesk-session")
    if (!rawSession) throw new Error("E2E knowledge verification admin session is missing")
    const session = JSON.parse(rawSession) as { accessToken?: string; tenantId?: number }
    if (!session.accessToken || !session.tenantId) throw new Error("E2E knowledge verification admin session is invalid")
    const headers = {
      Authorization: `Bearer ${session.accessToken}`,
      "Content-Type": "application/json",
      "X-Tenant-Id": String(session.tenantId),
    }
    const requestJSON = async <T>(url: string, init?: RequestInit): Promise<T> => {
      const response = await fetch(url, { ...init, headers: { ...headers, ...(init?.headers || {}) } })
      const payload = await response.json() as { success?: boolean; message?: string; data?: T }
      if (!response.ok || !payload.success) {
        throw new Error(payload.message || `E2E knowledge request failed: ${response.status} ${url}`)
      }
      return payload.data as T
    }
    type Ticket = { id: number; ticket_no: string; product_id: number }
    type Candidate = {
      id: number
      ticket_id: number
      product_id: number
      product_model_id: number
      title: string
      root_cause_summary: string
      solution_summary: string
      knowledge_base_id: number
      knowledge_entry_id: number
      quality_score: number
      value_score: number
      candidate_score: number
      review_eligible: boolean
      review_status: string
      merged_to_candidate_id: number
    }
    type IndexTask = {
      id: number
      subject_type: string
      subject_id: number
      status: string
      error_summary: string
    }
    type SearchHit = {
      documentId: number
      documentTitle: string
      title: string
      content: string
    }
    type SearchResult = { results: SearchHit[]; hitCount: number }
    type AnswerResult = {
      answer: string
      answerStatus: number
      hitCount: number
      modelName: string
      hits: SearchHit[]
    }

    const ticketPage = await requestJSON<{ items: Ticket[] }>(
      `/api/enterprise/v1/tickets?page_size=20&search=${encodeURIComponent(expectedTicketNo)}`,
    )
    const ticket = ticketPage.items.find((item) => item.ticket_no === expectedTicketNo)
    if (!ticket || !ticket.product_id) throw new Error(`E2E knowledge ticket not found: ${expectedTicketNo}`)

    const candidates = await requestJSON<Candidate[]>(
      `/api/enterprise/v1/knowledge/candidates?product_id=${ticket.product_id}&review_status=pending,duplicate`,
    )
    const candidate = candidates
      .filter((item) => item.ticket_id === ticket.id)
      .sort((left, right) => right.id - left.id)[0]
    if (!candidate) throw new Error(`E2E ticket has no reviewable knowledge candidate: ${expectedTicketNo}`)
    if (!`${candidate.root_cause_summary}\n${candidate.solution_summary}`.includes(expectedProof)) {
      throw new Error("E2E knowledge candidate does not contain the latest repair conclusion")
    }

    const reusedPublishedEntry = candidate.review_status === "duplicate"
    let approved: Candidate
    if (reusedPublishedEntry) {
      if (candidate.merged_to_candidate_id <= 0) {
        throw new Error(`E2E duplicate candidate has no primary candidate: ${JSON.stringify(candidate)}`)
      }
      approved = await requestJSON<Candidate>(
        `/api/enterprise/v1/knowledge/candidates/${candidate.merged_to_candidate_id}`,
      )
      if (approved.knowledge_entry_id <= 0) {
        throw new Error(`E2E duplicate group has no published knowledge to reuse: ${JSON.stringify(candidate)}`)
      }
    } else {
      if (candidate.review_status !== "pending" || !candidate.review_eligible || candidate.quality_score < 75 || candidate.value_score < 40) {
        throw new Error(`E2E knowledge candidate did not pass the quality gate: ${JSON.stringify(candidate)}`)
      }
      approved = await requestJSON<Candidate>(
        `/api/enterprise/v1/knowledge/candidates/${candidate.id}/_approve`,
        {
          method: "POST",
          body: JSON.stringify({
            title: `售后维修闭环 ${expectedProof}`,
            content: [
              `知识验收标识：${expectedProof}`,
              `故障根因：${candidate.root_cause_summary}`,
              `处理方案：${candidate.solution_summary}`,
            ].join("\n\n"),
            language: "zh-CN",
            category: "repair_case",
            visibility: "public",
            publish: true,
          }),
        },
      )
    }
    if ((!reusedPublishedEntry && approved.review_status !== "approved") || approved.knowledge_entry_id % 10 !== 1) {
      throw new Error("E2E knowledge candidate was not approved as a document")
    }
    const documentId = Math.floor(approved.knowledge_entry_id / 10)
    const indexDeadline = Date.now() + 120_000
    let indexTask: IndexTask | undefined
    while (Date.now() < indexDeadline) {
      const tasks = await requestJSON<IndexTask[]>(
        `/api/enterprise/v1/knowledge/index-tasks?product_id=${ticket.product_id}`,
      )
      indexTask = tasks.find((item) => item.subject_type === "knowledge_document" && item.subject_id === documentId)
      if (indexTask?.status === "succeeded") break
      if (indexTask?.status === "failed") {
        throw new Error(`E2E knowledge index failed: ${indexTask.error_summary || "unknown error"}`)
      }
      await new Promise((resolve) => window.setTimeout(resolve, 1_000))
    }
    if (!indexTask || indexTask.status !== "succeeded") {
      throw new Error(`E2E knowledge index did not complete: ${indexTask?.status || "task missing"}`)
    }

    const publishedProof = `${approved.title}\n${approved.root_cause_summary}\n${approved.solution_summary}`.match(/KCAP-\d+/)?.[0]
    const retrievalProof = reusedPublishedEntry && publishedProof ? publishedProof : expectedProof
    const query = `知识验收标识 ${retrievalProof} 对应的根因和处理方案是什么？`
    const productScopeInput = {
      productId: ticket.product_id,
      knowledgeBaseIds: [approved.knowledge_base_id],
      locale: "zh-CN",
      audience: "customer",
      question: query,
      topK: 5,
      scoreThreshold: 0,
      rerankLimit: 5,
      channel: "customer_portal",
      scene: "aftersales_e2e",
    }
    if (candidate.product_model_id > 0 && documentId > 0) {
      const scopeProbeInput = {
        ...productScopeInput,
        question: `知识验收标识 ${retrievalProof} 对应的型号维修根因和处理方案是什么？`,
      }
      const productScopeSearch = await requestJSON<SearchResult>("/api/dashboard/knowledge-retrieve/debug/search", {
        method: "POST",
        body: JSON.stringify(scopeProbeInput),
      })
      if (productScopeSearch.results.some((item) => item.documentId === documentId)) {
        throw new Error("E2E product-only scope unexpectedly exposed model-specific repair knowledge")
      }
      const modelScopeSearch = await requestJSON<SearchResult>("/api/dashboard/knowledge-retrieve/debug/search", {
        method: "POST",
        body: JSON.stringify({ ...scopeProbeInput, productModelId: candidate.product_model_id }),
      })
      if (!modelScopeSearch.results.some((item) => item.documentId === documentId)) {
        throw new Error("E2E model scope did not expose its model-specific repair knowledge")
      }
    }
    const retrievalInput = {
      ...productScopeInput,
      productModelId: candidate.product_model_id,
    }
    const search = await requestJSON<SearchResult>("/api/dashboard/knowledge-retrieve/debug/search", {
      method: "POST",
      body: JSON.stringify(retrievalInput),
    })
    const matchingHit = search.results.find((item) => item.documentId === documentId &&
      `${item.documentTitle}\n${item.title}\n${item.content}`.includes(retrievalProof)
    )
    if (search.hitCount <= 0 || !matchingHit || matchingHit.documentId !== documentId) {
      throw new Error("E2E customer-scope retrieval did not hit the published repair knowledge")
    }

    const answer = await requestJSON<AnswerResult>("/api/dashboard/knowledge-retrieve/debug/answer", {
      method: "POST",
      body: JSON.stringify({ ...retrievalInput, answerMode: 1 }),
    })
    if (![1, 3].includes(answer.answerStatus) || answer.hitCount <= 0) {
      throw new Error(`E2E RAG answer did not use the published repair knowledge: ${answer.answer}`)
    }
    if (!/受热回弹|接触电阻/.test(answer.answer) || !/同规格端子|标准扭矩|48\.0V/.test(answer.answer)) {
      throw new Error(`E2E RAG answer omitted the verified repair action: ${answer.answer}`)
    }
    return {
      candidateId: candidate.id,
      knowledgeEntryId: approved.knowledge_entry_id,
      knowledgeBaseId: approved.knowledge_base_id,
      indexTaskId: indexTask.id,
      searchHitCount: search.hitCount,
      answerModel: answer.modelName,
    }
  }, { expectedTicketNo: ticketNo, expectedProof: proof })
}

test("客户、工程师和供应商完成真实售后闭环", async ({ browser }) => {
  test.setTimeout(600_000)
  requireLiveFixture()
  const runTag = `PW-E2E-${Date.now()}`
  const knowledgeProof = `KCAP-${Date.now()}`
  const customerRole = await createRolePage(browser, { width: 430, height: 900 })
  const engineerRole = await createRolePage(browser, { width: 1440, height: 960 })
  const supplierRole = await createRolePage(browser, { width: 1440, height: 960 })
  const adminRole = await createRolePage(browser, { width: 1280, height: 800 })
  let createdTicketNo = ""
  let completed = false
  let dispatchFixture: DispatchFixtureSnapshot | null = null
  const unexpectedAPIErrors: string[] = []
  for (const page of [customerRole.page, engineerRole.page, supplierRole.page, adminRole.page]) {
    await routeBrowserTrafficToBackend(page)
    page.on("response", (response) => {
      const status = response.status()
      if (response.url().includes("/api/") && (status === 429 || status >= 500)) {
        unexpectedAPIErrors.push(`${status} ${response.request().method()} ${response.url()}`)
      }
    })
  }

  try {
    const customer = customerRole.page
    await login(customer, "/customer/chat", customerUsername, customerPassword)
    await login(engineerRole.page, "/enterprise/ticket-workbench", engineerUsername, engineerPassword)
    await login(supplierRole.page, "/partner/tickets", supplierUsername, supplierPassword)
	    if (requiresAdmin) {
	      await login(adminRole.page, "/enterprise", adminUsername, adminPassword)
	    }
		    if (publishKnowledgeCandidate) {
	      await waitForSeedKnowledgeIndex(adminRole.page)
	    }
	    if (manageDispatchFixture) {
	      dispatchFixture = await prepareDeterministicDispatchFixture(adminRole.page, engineerUsername, productId)
	    }
    await ensureDispatchAvailability(adminRole.page, engineerUsername, productId)
	    if (manageEngineerStatus) {
	      await confirmEngineerAvailable(engineerRole.page)
	    }

	    const previousConversationId = new URL(customer.url()).searchParams.get("conversationId")
    await closeCustomerConversationIfPresent(customer, Number(previousConversationId || "0"))
    await customer.getByRole("button", { name: "新建会话", exact: true }).click()
    const deviceChoice = customer.getByRole("dialog", { name: "发起服务会话" })
      .getByRole("button")
      .filter({ hasText: customerDeviceNo })
    await expect(deviceChoice).toHaveCount(1)
    await deviceChoice.click()
    const [createDeviceConversationRequest] = await Promise.all([
      customer.waitForRequest((request) => {
        if (request.method() !== "POST") return false
        return new URL(request.url()).pathname === "/api/conversation/create_or_match"
      }),
      customer.getByRole("button", { name: "关联设备并开始", exact: true }).click(),
    ])
    const createDeviceConversationPayload = JSON.parse(createDeviceConversationRequest.postData() || "{}") as {
      deviceId?: number
      forceNew?: boolean
      aiAgentId?: number
      workflowId?: number
    }
    expect(Object.keys(createDeviceConversationPayload).sort()).toEqual(["deviceId", "forceNew"])
    expect(createDeviceConversationPayload.deviceId, "客户设备入口只能提交 deviceId").toBeGreaterThan(0)
    expect(createDeviceConversationPayload.forceNew).toBe(true)
    expect(createDeviceConversationPayload.aiAgentId, "客户侧不得提交内部 Agent 路由字段").toBeUndefined()
    expect(createDeviceConversationPayload.workflowId, "客户侧不得提交内部 Workflow 路由字段").toBeUndefined()
    await expect(customer.getByText(customerDeviceNo, { exact: true }).last()).toBeVisible()
    await expect(customer.locator('[contenteditable="true"]').last()).toBeVisible()
    await expect(customer.getByRole("button", { name: "转人工", exact: true })).toBeVisible()
    await expect(customer.getByRole("button", { name: "创建工单", exact: true })).toHaveCount(0)
    await expect.poll(
      () => new URL(customer.url()).searchParams.get("conversationId"),
      { timeout: 15_000 },
    ).not.toBe(previousConversationId)
    const conversationId = Number(new URL(customer.url()).searchParams.get("conversationId") || "0")
    expect(conversationId, "正式客户会话应写入 URL").toBeGreaterThan(0)

    await sendCustomerMessage(customer, `${runTag}：设备报故障码 ${faultCode}。标准复位步骤和正常指示灯状态是什么？请先由 AI 回答。`)
    const escapedResetSeconds = expectedResetSeconds.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")
    await expect(
      customer.locator('[data-sender-type="ai"]').filter({
        hasText: new RegExp(`断开.*主电源.*${escapedResetSeconds}\\s*秒`),
      }).last(),
    ).toBeVisible({ timeout: 90_000 })

    const aiMessages = customer.locator('[data-sender-type="ai"]')
    const aiMessageCountBeforeSecondQuestion = await aiMessages.count()
    await sendCustomerMessage(customer, `${runTag}：按步骤复位后红灯仍然常亮，能否连续重复上电 5 次？`)
    await expect.poll(() => aiMessages.count(), { timeout: 90_000 }).toBeGreaterThan(aiMessageCountBeforeSecondQuestion)
    const secondAIReply = aiMessages.last()
    await expect(secondAIReply).toContainText(/不建议|不要|停止|禁止|严禁|立即停机|切断.*电源/)
    await expect(secondAIReply).toContainText(/5 次|重复上电|反复尝试/)

    await sendCustomerMessage(customer, `${runTag}：复位后红灯仍常亮，已停止上电，请人工确认。`)
    await customer.getByRole("button", { name: "转人工", exact: true }).click()
    await expect(
      customer.locator('[data-sender-type="system"]').filter({
        hasText: "客户已申请转人工",
      }).filter({
        hasText: "客户在会话工作台请求人工支持",
      }),
    ).toBeVisible()
    await customer.goto(`${frontendUrl}/customer/tickets`)
    const currentTicketButton = customer.locator("button").filter({ hasText: runTag }).first()
    await expect(currentTicketButton).toBeVisible({ timeout: 60_000 })
    const ticketButtonText = await currentTicketButton.innerText()
    createdTicketNo = ticketButtonText.match(/TK\d+/)?.[0] ?? ""
    expect(createdTicketNo, "正式客户侧应显示正式工单编号").toBeTruthy()
    await expect(async () => {
      await customer.goto(`${frontendUrl}/customer/chat?conversationId=${conversationId}`)
      await expect(customer.locator('[contenteditable="true"]').last()).toBeVisible({ timeout: 5_000 })
    }).toPass({
      intervals: [500, 1_000, 2_000],
      timeout: 30_000,
    })
    const assignedMessage = dispatchFixture
      ? `已分配给 ${dispatchFixture.engineerDisplayName}，等待工程师接入`
      : ""
    const assignmentPattern = assignedMessage
      ? new RegExp(`${escapeRegExp(assignedMessage)}|已进入.*维修组待认领|组内工程师可以查看并接单`)
      : /已分配给 .+，等待工程师接入|已进入.*维修组待认领|组内工程师可以查看并接单/
    await expect(async () => {
      const headerText = await customer.getByTestId("customer-assignment-status").textContent({ timeout: 1_000 }).catch(() => "")
      const timelineText = (await customer.locator("[data-sender-type]").allTextContents()).join("\n")
      expect(`${headerText}\n${timelineText}`).toMatch(assignmentPattern)
    }).toPass({
      intervals: [500, 1_000, 2_000],
      timeout: 15_000,
    })

    const engineer = engineerRole.page
    await engineer.goto(`${frontendUrl}/enterprise/ticket-workbench`)
    const queueItem = engineer.locator("button").filter({ hasText: runTag }).first()
    await expect(queueItem).toBeVisible({ timeout: 60_000 })
    await queueItem.click()
    const acceptButton = engineer.getByRole("button", { name: /^(重新)?接单$/ })
    if (await acceptButton.isVisible()) {
      await acceptButton.click()
    }

    await expect(engineer.getByText("客户在线", { exact: true })).toBeVisible({ timeout: 15_000 })
    await expect(customer.getByText(/工程师在线/)).toBeVisible({ timeout: 15_000 })

    const customerHumanComposer = customer.locator('[contenteditable="true"]').last()
    await expect(customerHumanComposer).toBeVisible()
    await customerHumanComposer.fill(`${runTag}：正在补充一条尚未发送的现场信息。`)
    await expect(engineer.getByTestId("enterprise-typing-status")).toHaveText("客户正在输入...", { timeout: 10_000 })
    await customerHumanComposer.fill("")

    const engineerReply = `${runTag}：已接单，请保持设备断电，我先核对保护输入端和 48V 母线。`
    const engineerComposer = engineer.getByPlaceholder("输入回复")
    await engineerComposer.fill(engineerReply)
    await expect(customer.getByText("工程师正在输入...", { exact: true })).toBeVisible({ timeout: 10_000 })
    await engineer.getByRole("button", { name: "发送", exact: true }).click()
    await expect(customer.getByText(engineerReply).last()).toBeVisible()

    const missedByEngineer = `${runTag}：工程师断线期间发送的客户补充，恢复网络后应自动补齐。`
    await engineerRole.context.setOffline(true)
    await customerHumanComposer.fill(missedByEngineer)
    await customer.getByRole("button", { name: "发送", exact: true }).click()
    await engineerRole.context.setOffline(false)
    await expect(engineer.getByText(missedByEngineer).last()).toBeVisible({ timeout: 20_000 })

    const missedByCustomer = `${runTag}：客户断线期间发送的工程师回复，恢复网络后应自动补齐。`
    await customerRole.context.setOffline(true)
    await engineerComposer.fill(missedByCustomer)
    await engineer.getByRole("button", { name: "发送", exact: true }).click()
    await customerRole.context.setOffline(false)
    await expect(customer.getByText(missedByCustomer).last()).toBeVisible({ timeout: 20_000 })

    const moduleSelect = engineer.locator("select").filter({ has: engineer.locator("option", { hasText: supplierModule }) })
    await expect(moduleSelect, `应加载产品模块 ${supplierModule}`).toHaveCount(1, { timeout: 30_000 })
    await moduleSelect.selectOption({ label: supplierModule })
    await engineer.getByPlaceholder(/说明需要供应商协助确认的问题/).fill(`${runTag}：请原厂核对端子规范和 48V 母线允许范围。`)
    await engineer.getByRole("button", { name: "升级模块供应商" }).click()
    await expect(
      customer.locator('[data-sender-type="system"]').filter({ hasText: "已邀请供应商协作" }).first(),
    ).toBeVisible({ timeout: 15_000 })
    await expect(engineer.getByText(/已邀请供应商协作/).first()).toBeVisible({ timeout: 15_000 })

    const supplier = supplierRole.page
    await supplier.goto(`${frontendUrl}/partner/tickets`)
    const supplierTicketLink = supplier.getByRole("link", { name: createdTicketNo }).first()
    await expect(supplierTicketLink).toBeVisible({ timeout: 60_000 })
    await supplierTicketLink.click()
    await expect(supplier).toHaveURL(/\/partner\/ticket-detail\?collaboration_id=\d+/)
    const collaborationId = Number(new URL(supplier.url()).searchParams.get("collaboration_id") || "0")
    expect(collaborationId, "供应商详情页应包含协作 ID").toBeGreaterThan(0)
    await expect(supplier.getByRole("heading", { name: createdTicketNo, exact: true })).toBeVisible()
    await supplier.getByRole("button", { name: "接单", exact: true }).click()
    await expect(
      customer.locator('[data-sender-type="system"]').filter({ hasText: "已加入协作会话" }).first(),
    ).toBeVisible({ timeout: 15_000 })
    await expect(engineer.getByText(/已加入.*会话/).first()).toBeVisible({ timeout: 15_000 })
    await expect(supplier.getByText(/已加入.*会话/).first()).toBeVisible({ timeout: 15_000 })

    await expect(supplier.getByText("客户在线", { exact: true })).toBeVisible({ timeout: 15_000 })
    await expect(supplier.getByText("企业工程师在线", { exact: true })).toBeVisible({ timeout: 15_000 })
    await expect(engineer.getByText("供应商在线 1", { exact: true })).toBeVisible({ timeout: 15_000 })
    await expect(customer.getByText("工程师在线，供应商在线 1", { exact: true })).toBeVisible({ timeout: 15_000 })

    await expect(customer.getByRole("button", { name: "发送图片", exact: true })).toBeVisible()
    await expect(customer.getByRole("button", { name: "发送附件", exact: true })).toBeVisible()
    await expect(customer.getByRole("button", { name: "录制语音", exact: true })).toBeVisible()
    await expect(engineer.getByRole("button", { name: "图片", exact: true })).toBeVisible()
    await expect(engineer.getByRole("button", { name: "附件", exact: true })).toBeVisible()
    await expect(engineer.getByRole("button", { name: "录制语音", exact: true })).toBeVisible()
    await expect(supplier.getByRole("button", { name: "发送图片", exact: true })).toBeVisible()
    await expect(supplier.getByRole("button", { name: "发送附件", exact: true })).toBeVisible()
    await expect(supplier.getByRole("button", { name: "录制语音", exact: true })).toBeVisible()

    const customerImageName = `${runTag}-customer-site.png`
    await sendLiveMediaMessage(customer, {
      role: "customer",
      kind: "image",
      conversationId,
      filename: customerImageName,
    })
    const images = await Promise.all([
      expectImageReady(customer, customerImageName),
      expectImageReady(engineer, customerImageName),
      expectImageReady(supplier, customerImageName),
    ])
    for (const [index, page] of [customer, engineer, supplier].entries()) {
      await images[index].click()
      const preview = page.getByRole("dialog", { name: customerImageName, exact: true })
      await expect(preview).toBeVisible()
      await preview.getByRole("button", { name: "关闭", exact: true }).click()
      await expect(preview).toBeHidden()
    }

    const engineerAttachmentName = `${runTag}-measurement.txt`
    await sendLiveMediaMessage(engineer, {
      role: "engineer",
      kind: "attachment",
      conversationId,
      filename: engineerAttachmentName,
    })
    await Promise.all([
      expect(messageFilename(customer, engineerAttachmentName)).toBeVisible({ timeout: 15_000 }),
      expect(messageFilename(engineer, engineerAttachmentName)).toBeVisible({ timeout: 15_000 }),
      expect(messageFilename(supplier, engineerAttachmentName)).toBeVisible({ timeout: 15_000 }),
    ])

    const supplierAudioName = `${runTag}-supplier-voice.wav`
    await sendLiveMediaMessage(supplier, {
      role: "supplier",
      kind: "audio",
      conversationId,
      collaborationId,
      filename: supplierAudioName,
    })
    await Promise.all([
      expect(customer.locator("audio")).toHaveCount(1, { timeout: 15_000 }),
      expect(engineer.locator("audio")).toHaveCount(1, { timeout: 15_000 }),
      expect(supplier.locator("audio")).toHaveCount(1, { timeout: 15_000 }),
    ])
    await Promise.all([
      expect(messageFilename(customer, supplierAudioName)).toBeVisible(),
      expect(messageFilename(engineer, supplierAudioName)).toBeVisible(),
      expect(messageFilename(supplier, supplierAudioName)).toBeVisible(),
    ])

    const missedBySupplier = `${runTag}：供应商断线期间发送的客户补充，恢复网络后应自动补齐。`
    await supplierRole.context.setOffline(true)
    await customerHumanComposer.fill(missedBySupplier)
    await customer.getByRole("button", { name: "发送", exact: true }).click()
    await supplierRole.context.setOffline(false)
    await expect(supplier.getByText(missedBySupplier).last()).toBeVisible({ timeout: 20_000 })

    await customerHumanComposer.fill(`${runTag}：三方输入状态验证，暂不发送。`)
    await Promise.all([
      expect(engineer.getByTestId("enterprise-typing-status")).toHaveText("客户正在输入...", { timeout: 10_000 }),
      expect(supplier.getByTestId("partner-typing-status")).toContainText("正在输入...", { timeout: 10_000 }),
    ])
    await customerHumanComposer.fill("")

    const supplierReply = `${runTag}：原厂正在核对端子与母线记录。`
    const supplierComposer = supplier.getByPlaceholder(/回复会同步给客户和企业工程师/)
    await supplierComposer.fill(supplierReply)
    await Promise.all([
      expect(engineer.getByTestId("enterprise-typing-status")).toHaveText("供应商正在输入...", { timeout: 10_000 }),
      expect(customer.getByText(/供应商.*正在输入\.\.\./)).toBeVisible({ timeout: 10_000 }),
    ])
    await supplier.getByRole("button", { name: "发送回复" }).click()
    await expect(engineer.getByText(supplierReply).first()).toBeVisible()
    await expect(customer.getByText(supplierReply).first()).toBeVisible()

    await engineer.getByRole("button", { name: "发起视频协作", exact: true }).click()
    const meetingDialog = engineer.getByRole("dialog", { name: "发起视频协作", exact: true })
    await expect(meetingDialog).toBeVisible()
    const [meetingCreateResponse, engineerMeetingPage] = await Promise.all([
      engineer.waitForResponse((response) =>
        response.request().method() === "POST" &&
        /\/tickets\/\d+\/meetings(?:\?|$)/.test(response.url()),
        { timeout: 30_000 },
      ),
      engineerRole.context.waitForEvent("page", { timeout: 30_000 }),
      meetingDialog.getByRole("button", { name: "立即发起", exact: true }).click(),
    ])
    expect(meetingCreateResponse.ok()).toBeTruthy()
    await enterMeetingRoomAndWaitJoined(
      engineerMeetingPage,
      "工程师",
      /\/api\/enterprise\/v1\/meetings\/[^/]+\/_joined(?:\?|$)/,
    )
    await expect(
      customer.locator('[data-sender-type="system"]').filter({ hasText: "工程师已发起视频协作" }).first(),
    ).toBeVisible({ timeout: 30_000 })
    await expect(engineer.getByText("已发起视频协作", { exact: true })).toBeVisible({ timeout: 30_000 })
    const supplierMeetingButton = supplier.getByRole("button", { name: "进入视频协作" })
    await expect(supplierMeetingButton).toBeVisible()
    const [supplierMeetingPage] = await Promise.all([
      supplierRole.context.waitForEvent("page", { timeout: 30_000 }),
      supplierMeetingButton.click(),
    ])
    await enterMeetingRoomAndWaitJoined(
      supplierMeetingPage,
      "供应商",
      /\/api\/partner\/v1\/supplier-collaborations\/\d+\/meetings\/[^/]+\/_joined(?:\?|$)/,
    )

    await customer.goto(`${frontendUrl}/customer/meeting`)
    const customerMeetingItem = customer.locator("button").filter({ hasText: createdTicketNo }).first()
    await expect(customerMeetingItem).toBeVisible({ timeout: 15_000 })
    await customerMeetingItem.click()
    const customerMeetingButton = customer.getByRole("button", { name: /加入会议/ })
    await expect(customerMeetingButton).toBeVisible({ timeout: 15_000 })
    await expect(customerMeetingButton).toBeEnabled()
    const [customerJoinResponse] = await Promise.all([
      customer.waitForResponse((response) =>
        response.request().method() === "GET" &&
        /\/api\/customer\/v1\/meetings\/[^/]+\/join(?:\?|$)/.test(response.url()),
        { timeout: 30_000 },
      ),
      customerMeetingButton.click(),
    ])
    expect(customerJoinResponse.ok(), "客户入会接口应返回成功").toBe(true)
    const customerEnterMeetingButton = customer.getByRole("button", { name: "进入会议", exact: true })
    await expect(customerEnterMeetingButton, "客户应看到同源会议预入会按钮").toBeVisible({ timeout: 45_000 })
    const [customerJoinedResponse] = await Promise.all([
      customer.waitForResponse((response) =>
        response.request().method() === "POST" &&
        /\/api\/customer\/v1\/meetings\/[^/]+\/_joined(?:\?|$)/.test(response.url()),
        { timeout: 90_000 },
      ),
      customerEnterMeetingButton.click(),
    ])
    expect(customerJoinedResponse.ok(), "客户入会状态回写应返回成功").toBe(true)
    await customer.goto(`${frontendUrl}/customer/chat?conversationId=${conversationId}`)
    await expect(customer.locator('[contenteditable="true"]').last()).toBeVisible()

    await supplier.getByRole("button", { name: "提交最终结论" }).click()
    await supplier.getByRole("textbox", { name: "填写根因、处理措施与验证结果" }).fill(
      `${runTag}：保护输入端需断电后重新压接；48V 母线按 47.2V 至 49.0V 验收，红灯复发时禁止重复上电。`,
    )
    await supplier.getByRole("button", { name: "确认提交" }).click()
    await expect(engineer.getByText(/供应商处理完成/).last()).toBeVisible()

    await engineer.getByRole("button", { name: "提交维修结论" }).click()
    await engineer.getByRole("textbox", { name: "故障码 / 类型" }).fill(faultCode)
    await engineer.getByRole("textbox", { name: "根因" }).fill("保护输入端端子接触不良，导致供电保护持续触发。")
    await engineer.getByRole("textbox", { name: "处理方案与客户注意事项" }).fill(
      "完全断电后重新压接端子；负载复测母线稳定在 48.1V，连续运行 30 分钟保持绿色慢闪。若再次红灯常亮应立即停机。",
    )
    await engineer.getByRole("button", { name: "提交并等待客户确认" }).click()

    await customer.goto(`${frontendUrl}/customer/tickets`)
    await currentTicketButton.click()
    await customer.getByRole("button", { name: "5 星" }).click()
    await customer.getByPlaceholder(/补充本次服务反馈/).fill(`${runTag}：工程师和供应商处理清楚，问题已解决。`)
    const [firstConfirmResponse] = await Promise.all([
      customer.waitForResponse((response) =>
        response.request().method() === "POST" && /\/api\/customer\/v1\/tickets\/\d+\/_confirm$/.test(response.url()),
      ),
      customer.getByRole("button", { name: "评价并确认已解决" }).click(),
    ])
    expect(firstConfirmResponse.ok(), "首次客户确认接口应返回成功").toBe(true)
    expect((await firstConfirmResponse.json() as { success?: boolean }).success).toBe(true)
    await expect(customer.getByTestId("customer-selected-ticket-status")).toHaveText("已关闭")

    const recurrenceReason = `${runTag}：设备连续运行 20 分钟后红灯再次常亮，请按原工单继续处理。`
    const reopenComposer = customer.getByPlaceholder("描述仍未解决的问题")
    await expect(reopenComposer).toBeVisible()
    await reopenComposer.fill(recurrenceReason)
    const [reopenResponse] = await Promise.all([
      customer.waitForResponse((response) =>
        response.request().method() === "POST" && /\/api\/customer\/v1\/tickets\/\d+\/_reopen$/.test(response.url()),
      ),
      customer.getByRole("button", { name: "问题未解决，继续处理" }).click(),
    ])
    expect(reopenResponse.ok(), "客户重开接口应返回成功").toBe(true)
    const reopenPayload = await reopenResponse.json() as { data?: { status?: string }, success?: boolean }
    expect(reopenPayload.success).toBe(true)
    expect(["reopened", "pending_dispatch", "pending_assignee_accept"]).toContain(reopenPayload.data?.status)
    await expect(customer.getByText("工单已重新打开", { exact: true }).last()).toBeVisible({ timeout: 30_000 })
    await expect(customer.getByTestId("customer-selected-ticket-status")).toHaveText(/已重新打开|待派单|待工程师接单/)
    await customer.goto(`${frontendUrl}/customer/chat?conversationId=${conversationId}`)
    await expect(customer.locator('[contenteditable="true"]').last()).toBeVisible()
    await expect(
      customer.locator('[data-sender-type="system"]').filter({ hasText: "客户已完成服务评价" }).first(),
    ).toBeVisible()

    await engineer.reload()
    await expect(queueItem).toBeVisible({ timeout: 60_000 })
    await queueItem.click()
    await expect(engineer.getByText(`客户重新打开工单：${recurrenceReason}`, { exact: true }).first()).toBeVisible()
    await expect(acceptButton).toBeVisible()
    await acceptButton.click()

    const secondEngineerReply = `${runTag}：已重新接单，继续保持断电，我会复查压接端子并完成 30 分钟带载验证。`
    await engineerComposer.fill(secondEngineerReply)
    await engineer.getByRole("button", { name: "发送", exact: true }).click()
    await expect(
      customer.locator('[data-sender-type="agent"]').filter({ hasText: secondEngineerReply }),
    ).toBeVisible()

    await engineer.getByRole("button", { name: "提交维修结论" }).click()
    await engineer.getByRole("textbox", { name: "故障码 / 类型" }).fill(faultCode)
    await engineer.getByRole("textbox", { name: "根因" }).fill("首次压接后端子受热回弹，接触电阻再次升高。")
    await engineer.getByRole("textbox", { name: "处理方案与客户注意事项" }).fill(
      `更换同规格端子并按标准扭矩重新压接；带载运行 30 分钟后母线稳定在 48.0V，状态灯保持绿色慢闪。知识验收标识 ${knowledgeProof}。`,
    )
    await engineer.getByRole("button", { name: "提交并等待客户确认" }).click()

    await customer.goto(`${frontendUrl}/customer/tickets`)
    await currentTicketButton.click()
    await customer.getByRole("button", { name: "4 星" }).click()
    await customer.getByPlaceholder(/补充本次服务反馈/).fill(`${runTag}：二次处理后问题解决，复测过程清楚。`)
    const [secondConfirmResponse] = await Promise.all([
      customer.waitForResponse((response) =>
        response.request().method() === "POST" && /\/api\/customer\/v1\/tickets\/\d+\/_confirm$/.test(response.url()),
      ),
      customer.getByRole("button", { name: "评价并确认已解决" }).click(),
    ])
    expect(secondConfirmResponse.ok(), "二次客户确认接口应返回成功").toBe(true)
    expect((await secondConfirmResponse.json() as { success?: boolean }).success).toBe(true)
    await expect(customer.getByTestId("customer-selected-ticket-status")).toHaveText("已关闭")
    await customer.goto(`${frontendUrl}/customer/chat?conversationId=${conversationId}`)
    await expect(customer.getByTestId("customer-conversation-closed")).toBeVisible({ timeout: 15_000 })
    await expect(customer.locator('[contenteditable="true"]')).toHaveCount(0)

    await supplier.reload()
    await expect(
      supplier.getByText(/已提交结论|无权限执行该操作/).first(),
    ).toBeVisible({ timeout: 15_000 })
    await expect(supplier.getByPlaceholder(/回复会同步给客户和企业工程师/)).toHaveCount(0)
    await expect(supplier.getByRole("button", { name: "进入视频协作" })).toHaveCount(0)

    await engineer.goto(`${frontendUrl}/enterprise/ticket-workbench?conversation_id=${conversationId}`)
    await expect(engineer.getByText(createdTicketNo, { exact: true }).first()).toBeVisible({ timeout: 30_000 })
    await expect(engineer.getByText(new RegExp(runTag)).first()).toBeVisible()
    await expect(engineer.getByText(supplierReply, { exact: true }).first()).toBeVisible()
    await expect(engineer.getByText("已关闭").first()).toBeVisible()
    await expect(engineer.getByText("客户已完成服务评价", { exact: true }).first()).toBeVisible()
    await expect(engineer.getByText(new RegExp(faultCode)).first()).toBeVisible()
    await expect(engineer.getByText("4/5", { exact: true }).first()).toBeVisible()
    await expect(engineer.getByText(/^(待审核|重复候选)$/).first()).toBeVisible()
    if (publishKnowledgeCandidate) {
      const knowledgeLoop = await approveKnowledgeCandidateAndVerifyRAG(
        adminRole.page,
        createdTicketNo,
        knowledgeProof,
      )
      expect(knowledgeLoop.candidateId).toBeGreaterThan(0)
      expect(knowledgeLoop.knowledgeEntryId).toBeGreaterThan(0)
      expect(knowledgeLoop.indexTaskId).toBeGreaterThan(0)
      expect(knowledgeLoop.searchHitCount).toBeGreaterThan(0)
      expect(knowledgeLoop.answerModel).toBeTruthy()
    }
    const resourceProof = await verifyClosedLoopResources(
      publishKnowledgeCandidate ? adminRole.page : engineer,
      supplier,
      customer,
      createdTicketNo,
      conversationId,
      collaborationId,
    )
    expect(resourceProof.meetingCount).toBeGreaterThan(0)
    expect(resourceProof.collaborationCount).toBeGreaterThan(0)
    expect(unexpectedAPIErrors, "完整闭环不应出现 API 限流或服务端错误").toEqual([])
    completed = true
  } finally {
    if (!completed && createdTicketNo) {
      await cleanupInterruptedTicket(requiresAdmin ? adminRole.page : engineerRole.page, createdTicketNo).catch(() => undefined)
    }
    if (dispatchFixture) {
      await restoreDispatchFixture(adminRole.page, dispatchFixture).catch(() => undefined)
    }
    await closeRoleContexts([
      customerRole.context,
      engineerRole.context,
      supplierRole.context,
      adminRole.context,
    ])
  }
})
