import { expect, test, type APIRequestContext, type APIResponse, type Page } from "@playwright/test"

type JsonObject = Record<string, unknown>

interface ApiEnvelope<T> {
  success?: boolean
  message?: string
  data?: T
}

interface PartnerSession {
  accessToken: string
  tenantId?: number
}

interface PartnerProfile {
  user_id: number
  account_id: number
  roles: string[]
  can_manage_team?: boolean
}

interface PartnerAccount {
  id: number
  username: string
  display_name: string
  email?: string
  phone?: string
  languages?: string[]
  languages_json?: string
  roles?: string[]
  status: number
  is_current?: boolean
}

interface PartnerAccountCreateResult {
  account: PartnerAccount
  initial_password: string
}

interface CollaborationParticipant {
  partner_account_id: number
  status: number
}

interface Collaboration {
  id: number
  ticket_id: number
  conversation_id: number
  ticket_no?: string
  ticket_title?: string
  ticket_status?: string
  product_module_name?: string
  status: string
  fault_code?: string
  symptom_summary?: string
  diagnosis_summary?: string
  authorization_active: boolean
}

interface PartnerTicketDetail {
  collaboration: Collaboration
  participants?: CollaborationParticipant[]
  progresses?: Array<{ content?: string; author_id?: number }>
  messages?: Array<{ content?: string }>
}

interface Meeting {
  id: string
}

interface ParsedResponse<T> {
  status: number
  ok: boolean
  body: ApiEnvelope<T> | JsonObject | string
  text: string
}

const baseURL = requiredEnv("E2E_BASE_URL").replace(/\/+$/, "")
const adminUsername = envWithFallback("E2E_PARTNER_ADMIN_USERNAME", "E2E_SUPPLIER_USERNAME")
const adminPassword = envWithFallback("E2E_PARTNER_ADMIN_PASSWORD", "E2E_SUPPLIER_PASSWORD")

test.describe.configure({ mode: "serial" })
test.use({ trace: "off" })

test("partner engineer can read company collaborations while work actions stay authorized", async ({ browser, request }) => {
  test.setTimeout(180_000)

  const runTag = `E2E-PCS-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
  const engineerUsername = `e2e.partner.engineer.companyscope.${Date.now()}.${Math.random().toString(36).slice(2, 7)}`
  const engineerDisplayName = `${runTag} engineer`

  let adminSession: PartnerSession | undefined
  let adminAccount: PartnerAccount | undefined
  let target: Collaboration | undefined
  let engineerAccount: PartnerAccount | undefined
  let engineerPassword: string | undefined
  let engineerSession: PartnerSession | undefined
  let unexpectedAdminAccount: PartnerAccount | undefined

  const adminContext = await browser.newContext()
  const engineerContext = await browser.newContext()
  const adminPage = await adminContext.newPage()
  const engineerPage = await engineerContext.newPage()

  try {
    adminSession = await loginThroughPartnerPortal(
      adminPage,
      adminUsername,
      adminPassword,
    )

    const adminProfile = await apiSuccess<PartnerProfile>(
      request,
      adminSession,
      "GET",
      "/api/partner/v1/profile",
    )
    expect(adminProfile.roles, "Configured supplier account must be a partner admin").toContain(
      "partner_admin",
    )

    const accounts = await apiSuccess<PartnerAccount[]>(
      request,
      adminSession,
      "GET",
      "/api/partner/v1/accounts",
    )
    adminAccount =
      accounts.find((account) => account.id === adminProfile.account_id) ??
      accounts.find((account) => account.is_current)
    expect(adminAccount, "Current partner admin account must be present in account list").toBeTruthy()

    const collaborations = await apiSuccess<Collaboration[]>(
      request,
      adminSession,
      "GET",
      "/api/partner/v1/tickets",
    )
    target = selectActiveCollaboration(collaborations)
    expect(target, "Partner admin needs an active collaboration in processing state").toBeTruthy()

    const adminDetail = await apiSuccess<PartnerTicketDetail>(
      request,
      adminSession,
      "GET",
      `/api/partner/v1/tickets/${target!.id}`,
    )
    target = adminDetail.collaboration

    const meetings = await apiSuccess<Meeting[]>(
      request,
      adminSession,
      "GET",
      `/api/partner/v1/tickets/${target.ticket_id}/meetings`,
    )
    const meetingID = meetings[0]?.id ?? `${runTag}-missing-meeting`

    const createdAccount = await apiSuccess<PartnerAccountCreateResult>(
      request,
      adminSession,
      "POST",
      "/api/partner/v1/accounts",
      {
        username: engineerUsername,
        display_name: engineerDisplayName,
        email: `${engineerUsername}@example.invalid`,
        languages: ["en-US"],
        role_code: "partner_engineer",
      },
    )
    engineerAccount = createdAccount.account
    engineerPassword = createdAccount.initial_password
    expect(engineerPassword, "Partner account creation must return an initial password").toBeTruthy()

    engineerSession = await loginThroughPartnerPortal(
      engineerPage,
      engineerUsername,
      engineerPassword!,
    )
    const engineerProfile = await apiSuccess<PartnerProfile>(
      request,
      engineerSession,
      "GET",
      "/api/partner/v1/profile",
    )
    expect(engineerProfile.roles).toContain("partner_engineer")
    expect(engineerProfile.roles).not.toContain("partner_admin")
    expect(engineerProfile.can_manage_team).toBeFalsy()

    await expectCompanyScopedReadAccess(request, engineerSession, target)
    await engineerPage.goto(`${baseURL}/partner/tickets`)
    await expect(engineerPage.getByRole("link", { name: target.ticket_no, exact: true })).toBeVisible()
    await expectCollaborationWorkAPIsRejected(
      request,
      engineerSession,
      target,
      meetingID,
      runTag,
      "before participant authorization",
    )

    const forbiddenAdminUsername = `${engineerUsername}.admin`
    const createAdminResponse = await apiCall<PartnerAccountCreateResult>(
      request,
      engineerSession,
      "POST",
      "/api/partner/v1/accounts",
      {
        username: forbiddenAdminUsername,
        display_name: `${runTag} forbidden admin`,
        email: `${forbiddenAdminUsername}@example.invalid`,
        languages: ["en-US"],
        role_code: "partner_admin",
      },
    )
    if (isApiSuccess(createAdminResponse)) {
      unexpectedAdminAccount = createAdminResponse.body.data.account
    }
    expectRejected(createAdminResponse, "Partner engineer must not create a partner admin")

    const updateAdminResponse = await apiCall<PartnerAccount>(
      request,
      engineerSession,
      "PUT",
      `/api/partner/v1/accounts/${adminAccount!.id}`,
      accountUpdatePayload(adminAccount!, "partner_admin", adminAccount!.status),
    )
    expectRejected(updateAdminResponse, "Partner engineer must not update another account")

    await apiSuccess<PartnerTicketDetail>(
      request,
      adminSession,
      "POST",
      `/api/partner/v1/tickets/${target.id}/participants`,
      { partner_account_id: engineerAccount.id },
    )

    const authorizedList = await apiSuccess<Collaboration[]>(
      request,
      engineerSession,
      "GET",
      "/api/partner/v1/tickets",
    )
    expect(authorizedList.some((item) => item.id === target!.id)).toBeTruthy()
    await engineerPage.goto(`${baseURL}/partner/tickets`)
    await expect(engineerPage.getByRole("link", { name: target.ticket_no, exact: true })).toBeVisible()

    const authorizedDetail = await apiSuccess<PartnerTicketDetail>(
      request,
      engineerSession,
      "GET",
      `/api/partner/v1/tickets/${target.id}`,
    )
    expect(authorizedDetail.collaboration.id).toBe(target.id)
    expect(
      authorizedDetail.participants?.some(
        (participant) =>
          participant.partner_account_id === engineerAccount!.id && participant.status === 0,
      ),
    ).toBeTruthy()

    const progressContent = `${runTag} partner engineer company-scope progress`
    await apiSuccess<unknown>(
      request,
      engineerSession,
      "POST",
      `/api/partner/v1/tickets/${target.id}/progress`,
      { content: progressContent },
    )
    const detailAfterProgress = await apiSuccess<PartnerTicketDetail>(
      request,
      engineerSession,
      "GET",
      `/api/partner/v1/tickets/${target.id}`,
    )
    expect(detailAfterProgress.progresses?.some((progress) => progress.content === progressContent)).toBeTruthy()

    await removeParticipant(request, adminSession, target.id, engineerAccount.id)

    await expectCompanyScopedReadAccess(request, engineerSession, target)
    await engineerPage.goto(`${baseURL}/partner/tickets`)
    await expect(engineerPage.getByRole("link", { name: target.ticket_no, exact: true })).toBeVisible()
    await expectCollaborationWorkAPIsRejected(
      request,
      engineerSession,
      target,
      meetingID,
      runTag,
      "after participant removal",
    )

    await disableAccount(request, adminSession, engineerAccount, "partner_engineer")

    const oldTokenResponse = await apiCall<PartnerProfile>(
      request,
      engineerSession,
      "GET",
      "/api/auth/profile",
    )
    expectRejected(oldTokenResponse, "Disabled partner account token must be revoked")

    const reloginResponse = await unauthenticatedApiCall<unknown>(
      request,
      "POST",
      "/api/auth/login",
      {
        username: engineerUsername,
        password: engineerPassword,
        domainType: "partner",
      },
    )
    expectRejected(reloginResponse, "Disabled partner account must not be able to log in again")
  } finally {
    if (adminSession && target && engineerAccount) {
      await removeParticipant(request, adminSession, target.id, engineerAccount.id).catch(() => undefined)
    }
    if (adminSession && engineerAccount) {
      await disableAccount(request, adminSession, engineerAccount, "partner_engineer").catch(() => undefined)
    }
    if (adminSession && unexpectedAdminAccount) {
      await disableAccount(request, adminSession, unexpectedAdminAccount, "partner_admin").catch(
        () => undefined,
      )
    }
    await Promise.all([adminContext.close(), engineerContext.close()])
  }
})

function requiredEnv(name: string): string {
  const value = process.env[name]?.trim()
  if (!value) {
    throw new Error(`${name} is required`)
  }
  return value
}

function envWithFallback(primary: string, fallback: string): string {
  return process.env[primary]?.trim() || requiredEnv(fallback)
}

async function loginThroughPartnerPortal(
  page: Page,
  username: string,
  password: string,
): Promise<PartnerSession> {
  const next = encodeURIComponent("/partner/tickets")
  await page.goto(`${baseURL}/dashboard/login?portal=partner&next=${next}`, {
    waitUntil: "domcontentloaded",
  })
  await page.locator('input[name="username"]').fill(username)
  await page.locator('input[name="password"]').fill(password)
  await page.locator('button[type="submit"]').click()
  await page.waitForURL((url) => !url.pathname.includes("/dashboard/login"), { timeout: 30_000 })

  const session = await page.evaluate(() => {
    const raw = window.localStorage.getItem("remote-helpdesk-session")
    return raw ? JSON.parse(raw) : null
  })
  expect(session?.accessToken, "Partner portal login must persist an access token").toBeTruthy()

  return {
    accessToken: String(session.accessToken),
    tenantId: typeof session.tenantId === "number" ? session.tenantId : undefined,
  }
}

function selectActiveCollaboration(collaborations: Collaboration[]): Collaboration | undefined {
  const terminalStatuses = new Set(["resolved", "timeout", "expired", "cancelled", "closed"])
  return collaborations.find(
    (item) =>
      item.authorization_active &&
      !terminalStatuses.has(item.status) &&
      item.status === "processing",
  )
}

async function expectCompanyScopedReadAccess(
  request: APIRequestContext,
  session: PartnerSession,
  collaboration: Collaboration,
): Promise<PartnerTicketDetail> {
  const list = await apiSuccess<Collaboration[]>(
    request,
    session,
    "GET",
    "/api/partner/v1/tickets",
  )
  expect(list.some((item) => item.id === collaboration.id), "Same-company partner ticket list must include the collaboration").toBeTruthy()
  const detail = await apiSuccess<PartnerTicketDetail>(
    request,
    session,
    "GET",
    `/api/partner/v1/tickets/${collaboration.id}`,
  )
  expect(detail.collaboration.id, "Same-company partner detail must be readable").toBe(collaboration.id)
  await apiSuccess<Meeting[]>(
    request,
    session,
    "GET",
    `/api/partner/v1/tickets/${collaboration.ticket_id}/meetings`,
  )
  return detail
}

async function expectCollaborationWorkAPIsRejected(
  request: APIRequestContext,
  session: PartnerSession,
  collaboration: Collaboration,
  meetingID: number | string,
  runTag: string,
  phase: string,
): Promise<void> {
  const checks: Array<Promise<ParsedResponse<unknown>>> = [
    apiCall(request, session, "POST", `/api/partner/v1/tickets/${collaboration.id}/progress`, {
      content: `${runTag} unauthorized progress ${phase}`,
    }),
    apiMultipartCall(
      request,
      session,
      `/api/partner/v1/tickets/${collaboration.id}/upload_attachment`,
      runTag,
    ),
    apiCall(
      request,
      session,
      "GET",
      `/api/partner/v1/supplier-collaborations/${collaboration.id}/meetings/${meetingID}/join`,
    ),
  ]

  const responses = await Promise.all(checks)
  for (const response of responses) {
    expectRejected(response, `Unauthorized collaboration work API must reject access ${phase}`)
  }
}

async function removeParticipant(
  request: APIRequestContext,
  session: PartnerSession,
  collaborationID: number,
  accountID: number,
): Promise<void> {
  const response = await apiCall<unknown>(
    request,
    session,
    "POST",
    `/api/partner/v1/tickets/${collaborationID}/participants/${accountID}/_remove`,
  )
  if (!isApiSuccess(response) && response.status !== 404 && response.status !== 409) {
    throw new Error(`Participant cleanup failed with status ${response.status}`)
  }
}

async function disableAccount(
  request: APIRequestContext,
  session: PartnerSession,
  account: PartnerAccount,
  roleCode: "partner_admin" | "partner_engineer",
): Promise<void> {
  await apiSuccess<PartnerAccount>(
    request,
    session,
    "PUT",
    `/api/partner/v1/accounts/${account.id}`,
    accountUpdatePayload(account, roleCode, 1),
  )
}

function accountUpdatePayload(
  account: PartnerAccount,
  roleCode: "partner_admin" | "partner_engineer",
  status: number,
): JsonObject {
  return {
    display_name: account.display_name,
    email: account.email ?? "",
    phone: account.phone ?? "",
    languages: account.languages ?? parseLanguages(account.languages_json),
    role_code: roleCode,
    status,
  }
}

function parseLanguages(value?: string): string[] {
  if (!value) return ["en-US"]
  try {
    const parsed = JSON.parse(value)
    return Array.isArray(parsed) ? parsed.filter((item): item is string => typeof item === "string") : ["en-US"]
  } catch {
    return ["en-US"]
  }
}

async function apiSuccess<T>(
  request: APIRequestContext,
  session: PartnerSession,
  method: "GET" | "POST" | "PUT",
  path: string,
  data?: JsonObject,
): Promise<T> {
  const response = await apiCall<T>(request, session, method, path, data)
  expect(response.ok, `Expected ${method} ${path} to return an HTTP success`).toBeTruthy()
  expect(isApiSuccess(response), `Expected ${method} ${path} to return a business success`).toBeTruthy()
  return response.body.data as T
}

async function apiCall<T>(
  request: APIRequestContext,
  session: PartnerSession,
  method: "GET" | "POST" | "PUT",
  path: string,
  data?: JsonObject,
): Promise<ParsedResponse<T>> {
  const response = await request.fetch(`${baseURL}${path}`, {
    method,
    headers: authorizationHeaders(session),
    data,
    failOnStatusCode: false,
  })
  return parseResponse<T>(response)
}

async function unauthenticatedApiCall<T>(
  request: APIRequestContext,
  method: "POST",
  path: string,
  data: JsonObject,
): Promise<ParsedResponse<T>> {
  const response = await request.fetch(`${baseURL}${path}`, {
    method,
    data,
    failOnStatusCode: false,
  })
  return parseResponse<T>(response)
}

async function apiMultipartCall(
  request: APIRequestContext,
  session: PartnerSession,
  path: string,
  runTag: string,
): Promise<ParsedResponse<unknown>> {
  const response = await request.post(`${baseURL}${path}`, {
    headers: authorizationHeaders(session),
    multipart: {
      file: {
        name: `${runTag}.txt`,
        mimeType: "text/plain",
        buffer: Buffer.from(`${runTag} unauthorized upload probe`),
      },
    },
    failOnStatusCode: false,
  })
  return parseResponse<unknown>(response)
}

function authorizationHeaders(session: PartnerSession): Record<string, string> {
  const headers: Record<string, string> = {
    Authorization: `Bearer ${session.accessToken}`,
  }
  if (session.tenantId) {
    headers["X-Tenant-ID"] = String(session.tenantId)
  }
  return headers
}

async function parseResponse<T>(response: APIResponse): Promise<ParsedResponse<T>> {
  const text = await response.text()
  let body: ApiEnvelope<T> | JsonObject | string = text
  if (text) {
    try {
      body = JSON.parse(text) as ApiEnvelope<T> | JsonObject
    } catch {
      body = text
    }
  }
  return {
    status: response.status(),
    ok: response.ok(),
    body,
    text,
  }
}

function isApiSuccess<T>(response: ParsedResponse<T>): response is ParsedResponse<T> & {
  body: ApiEnvelope<T> & { success: true; data: T }
} {
  return (
    typeof response.body === "object" &&
    response.body !== null &&
    "success" in response.body &&
    response.body.success === true
  )
}

function expectRejected<T>(response: ParsedResponse<T>, message: string): void {
  expect(!response.ok || !isApiSuccess(response), message).toBeTruthy()
}
