import { expect, test, type APIRequestContext, type BrowserContext } from "@playwright/test"

const baseUrl = process.env.E2E_BASE_URL ?? "http://localhost:3000"
const frontendUrl = process.env.E2E_FRONTEND_URL ?? baseUrl
const platformUsername = process.env.E2E_PLATFORM_USERNAME ?? ""
const platformPassword = process.env.E2E_PLATFORM_PASSWORD ?? ""
const tenantId = Number(process.env.E2E_TENANT_ID ?? "0")
const workflowId = Number(process.env.E2E_WORKFLOW_ID ?? "0")

type Envelope<T> = {
  success: boolean
  message: string
  data: T
}

type AuthSession = {
  accessToken: string
  tenantId: number
  domainType: string
  permissions: string[]
}

type WorkflowList = {
  results: Array<{
    id: number
    code: string
    name: string
    scope: "platform" | "tenant" | string
    humanHandoffEnabled: boolean
    currentStableVersionId: number
    description: string
    draftDefinition: {
      modelPolicy?: {
        credentialChain?: string[]
      }
      nodes: Array<{ id: string; type: string; name?: string; config?: Record<string, unknown> }>
      edges?: Array<{ id: string; source: string; target: string }>
    }
  }>
}

type WorkflowVersion = {
  id: number
  workflowId: number
  version: number
  sourceVersionId: number
  definitionHash: string
}

let cachedTenantSession: AuthSession | null = null
let pendingTenantSession: Promise<AuthSession> | null = null

test("企业管理员可以用服务蓝图配置流程而无需理解节点", async ({ browser, request }) => {
  requireFixture()
  const tenantSession = await issueTenantSession(request)
  const headers = { Authorization: `Bearer ${tenantSession.accessToken}` }
  const workflows = await responseData<WorkflowList>(await request.get(`${baseUrl}/api/enterprise/v1/ai-workflows?page=1&limit=100`, {
    headers,
  }))
  const source = workflows.results.find((item) => item.scope === "platform" && item.currentStableVersionId > 0)
  expect(source, "应存在可复制的平台稳定方案").toBeTruthy()
  const tenantWorkflow = await responseData<WorkflowList["results"][number]>(await request.post(`${baseUrl}/api/enterprise/v1/ai-workflows`, {
    headers,
    data: {
      name: `服务蓝图自动化验收 ${Date.now()}`,
      description: "验证两套平台服务方案",
      sourceVersionId: source!.currentStableVersionId,
    },
  }))
  await responseData<WorkflowVersion>(await request.post(`${baseUrl}/api/enterprise/v1/ai-workflows/${tenantWorkflow.id}/_publish`, {
    headers,
    data: {
      name: tenantWorkflow.name,
      description: tenantWorkflow.description,
      definition: tenantWorkflow.draftDefinition,
    },
  }))

  try {
    for (const viewport of [
      { name: "desktop", width: 1440, height: 1000 },
      { name: "mobile", width: 390, height: 844 },
    ]) {
      const context = await browser.newContext({ viewport })
      await installSession(context, tenantSession)
      const page = await context.newPage()
      await page.goto(`${frontendUrl}/enterprise/workflow/${tenantWorkflow.id}`)

    await expect(page.getByRole("tab", { name: "流程编排", exact: true })).toHaveAttribute("data-active")
    await expect(page.getByRole("tab", { name: "服务蓝图", exact: true })).toHaveAttribute("data-active")
    await expect(page.getByText("选择客户服务方案", { exact: true })).toBeVisible()
    await expect(page.getByText("轻量 AI 产品问答", { exact: true })).toHaveCount(0)
    await expect(
      page.getByTestId("workflow-blueprint-device_ai").getByText("AI 智能问诊", { exact: true }),
    ).toBeVisible()
    await expect(page.getByText("AI 诊断与人工协同", { exact: true }).first()).toBeVisible()
    await expect(
      page.getByText("当前标准", { exact: true })
        .or(page.getByText("当前类型 · 已自定义", { exact: true })),
    ).toHaveCount(1)
    await expect(page.getByRole("button", { name: "试运行", exact: true })).toBeVisible()
    await expect(page.getByRole("button", { name: "保存草稿", exact: true })).toBeDisabled()

    await page.getByRole("tab", { name: "专业节点设计", exact: true }).click()
    await expect(page.getByRole("button", { name: "校验", exact: true })).toBeVisible()

    await page.getByRole("tab", { name: "版本发布", exact: true }).click()
    const versionPanel = page.getByRole("tabpanel", { name: "版本发布", exact: true })
    await expect(versionPanel).toContainText("当前稳定版本")
    await expect(versionPanel).toContainText("流程步骤")
    if (viewport.name === "desktop") {
      await expect(page.locator("[data-workflow-version-desktop-table]")).toBeVisible()
      await expect(page.locator("[data-workflow-version-mobile-list]")).toBeHidden()
    } else {
      await expect(page.locator("[data-workflow-version-mobile-list]")).toBeVisible()
      await expect(page.locator("[data-workflow-version-desktop-table]")).toBeHidden()
    }

    const layout = await page.evaluate(() => ({
      viewportWidth: document.documentElement.clientWidth,
      documentWidth: document.documentElement.scrollWidth,
    }))
    expect(layout.documentWidth, `${viewport.name} 不应出现页面级横向滚动`).toBeLessThanOrEqual(layout.viewportWidth + 1)
      await context.close()
    }
  } finally {
    const response = await request.delete(`${baseUrl}/api/enterprise/v1/ai-workflows/${tenantWorkflow.id}`, { headers })
    expect(response.ok()).toBe(true)
  }
})

function requireFixture() {
  const missing = [
    ["E2E_PLATFORM_USERNAME", platformUsername],
    ["E2E_PLATFORM_PASSWORD", platformPassword],
    ["E2E_TENANT_ID", tenantId],
    ["E2E_WORKFLOW_ID", workflowId],
  ].filter(([, value]) => !value).map(([key]) => key)

  if (missing.length > 0) {
    throw new Error(`工作流产品页回归缺少环境变量：${missing.join(", ")}`)
  }
}

async function responseData<T>(response: Awaited<ReturnType<APIRequestContext["post"]>>) {
  const payload = await response.json() as Envelope<T>
  expect(response.ok(), payload.message).toBe(true)
  expect(payload.success, payload.message).toBe(true)
  return payload.data
}

async function issueTenantSession(request: APIRequestContext) {
  if (cachedTenantSession) return cachedTenantSession
  if (!pendingTenantSession) {
    pendingTenantSession = (async () => {
      const platformSession = await responseData<AuthSession>(await request.post(`${baseUrl}/api/auth/login`, {
        data: {
          username: platformUsername,
          password: platformPassword,
          domainType: "platform",
        },
      }))

      return responseData<AuthSession>(await request.post(`${baseUrl}/api/platform/tenant/enter`, {
        headers: { Authorization: `Bearer ${platformSession.accessToken}` },
        data: { tenantId, reason: "工作流产品页自动化验收" },
      }))
    })()
  }
  cachedTenantSession = await pendingTenantSession
  return cachedTenantSession
}

async function installSession(context: BrowserContext, session: AuthSession) {
  if (frontendUrl !== baseUrl) {
    await context.route(`${frontendUrl}/api/**`, async (route) => {
      const source = new URL(route.request().url())
      await route.continue({ url: `${baseUrl}${source.pathname}${source.search}` })
    })
    await context.addInitScript(({ backendUrl }) => {
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
  await context.addInitScript((value) => {
    if (window.location.protocol !== "http:" && window.location.protocol !== "https:") return
    window.localStorage.setItem("remote-helpdesk-session", JSON.stringify(value))
  }, session)
}

test("AI 租户平台仅提供 AI 智能问诊与 AI 人工协同两种工作流", async ({ browser, request }) => {
  requireFixture()
  const tenantSession = await issueTenantSession(request)
  const workflows = await responseData<WorkflowList>(await request.get(`${baseUrl}/api/enterprise/v1/ai-workflows?page=1&limit=100`, {
    headers: { Authorization: `Bearer ${tenantSession.accessToken}` },
  }))
  const collaboration = workflows.results.find((item) => item.code === "aftersales_customer_service_default")
  const deviceAI = workflows.results.find((item) => item.code === "aftersales_device_diagnosis_ai_only")
  const dispatchOnly = workflows.results.find((item) => item.code === "aftersales_customer_service_dispatch_only")
  const basicAI = workflows.results.find((item) => item.code === "aftersales_customer_service_ai_only")

  expect(collaboration?.humanHandoffEnabled).toBe(true)
  expect(collaboration?.draftDefinition.nodes.some((node) => node.type === "handoff_to_human")).toBe(true)
  expect(deviceAI?.humanHandoffEnabled).toBe(false)
  expect(deviceAI?.draftDefinition.nodes.some((node) => node.type === "answerability_gate")).toBe(true)
  expect(deviceAI?.draftDefinition.nodes.some((node) => node.type === "handoff_to_human")).toBe(false)
  expect(dispatchOnly, "AI 租户不应显示功能派发平台模板").toBeUndefined()
  expect(basicAI, "重复的基础 AI 平台模板应已退场").toBeUndefined()

  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
  await installSession(context, tenantSession)
  const page = await context.newPage()
  await page.goto(`${frontendUrl}/enterprise/workflow`)
  const collaborationRow = page.getByRole("row").filter({ hasText: collaboration!.name })
  const deviceAIRow = page.getByRole("row").filter({ hasText: deviceAI!.name })
  const dispatchOnlyRow = page.getByRole("row").filter({ hasText: "平台 功能派发闭环流程" })
  await expect(collaborationRow).toHaveCount(1)
  await expect(deviceAIRow).toHaveCount(1)
  await expect(dispatchOnlyRow).toHaveCount(0)
  await expect(page.getByText("平台 AI 专属产品客服流程", { exact: true })).toHaveCount(0)
  await expect(collaborationRow.getByText("AI + 人工协同", { exact: true })).toBeVisible()
  await expect(deviceAIRow.getByText("AI 智能问诊", { exact: true })).toBeVisible()
  await expect(page.getByText("Eino Graph", { exact: true })).toHaveCount(0)

  await page.getByRole("button", { name: "新建工作流", exact: true }).click()
  await expect(page.getByRole("heading", { name: "新建企业工作流", exact: true })).toBeVisible()
  await expect(page.getByText("创建后仅生成企业草稿，不会影响任何线上产品。", { exact: false })).toBeVisible()
  await expect(page.getByRole("button", { name: "创建并编排", exact: true })).toBeEnabled()
  await page.getByRole("button", { name: "取消", exact: true }).click()
  await context.close()

  const mobileContext = await browser.newContext({ viewport: { width: 390, height: 844 } })
  await installSession(mobileContext, tenantSession)
  const mobilePage = await mobileContext.newPage()
  await mobilePage.goto(`${frontendUrl}/enterprise/workflow`)
  await mobilePage.getByRole("button", { name: "新建工作流", exact: true }).click()
  const mobileDialogLayout = await mobilePage.locator('[data-slot="dialog-content"]').evaluate((element) => {
    const rect = element.getBoundingClientRect()
    return {
      top: rect.top,
      bottom: rect.bottom,
      viewportHeight: window.innerHeight,
      documentWidth: document.documentElement.scrollWidth,
      viewportWidth: document.documentElement.clientWidth,
    }
  })
  expect(mobileDialogLayout.top).toBeGreaterThanOrEqual(0)
  expect(mobileDialogLayout.bottom).toBeLessThanOrEqual(mobileDialogLayout.viewportHeight)
  expect(mobileDialogLayout.documentWidth).toBeLessThanOrEqual(mobileDialogLayout.viewportWidth + 1)
  await mobileContext.close()
})

test("历史版本回滚会创建新稳定版本且保留不可变历史", async ({ request }) => {
  requireFixture()
  const tenantSession = await issueTenantSession(request)
  const headers = { Authorization: `Bearer ${tenantSession.accessToken}` }
  const workflows = await responseData<WorkflowList>(await request.get(`${baseUrl}/api/enterprise/v1/ai-workflows?page=1&limit=100`, { headers }))
  const source = workflows.results.find((item) => item.scope === "platform" && item.currentStableVersionId > 0)
  expect(source, "应存在可复制的平台稳定方案").toBeTruthy()

  let createdId = 0
  try {
    const created = await responseData<WorkflowList["results"][number]>(await request.post(`${baseUrl}/api/enterprise/v1/ai-workflows`, {
      headers,
      data: {
        name: `回滚自动化验收 ${Date.now()}`,
        description: "验证不可变版本回滚",
        sourceVersionId: source!.currentStableVersionId,
      },
    }))
    createdId = created.id
    const publishPayload = {
      name: created.name,
      description: created.description,
      definition: created.draftDefinition,
    }
    const first = await responseData<WorkflowVersion>(await request.post(`${baseUrl}/api/enterprise/v1/ai-workflows/${created.id}/_publish`, {
      headers,
      data: publishPayload,
    }))
    const changedDefinition = structuredClone(created.draftDefinition)
    const changedLLMNode = changedDefinition.nodes.find((node) => node.type === "llm_reply")
    expect(changedLLMNode, "回滚验收流程应包含 AI 回答节点").toBeTruthy()
    changedLLMNode!.config = {
      ...changedLLMNode!.config,
      prompt: `自动化版本差异 ${Date.now()}`,
    }
    const second = await responseData<WorkflowVersion>(await request.post(`${baseUrl}/api/enterprise/v1/ai-workflows/${created.id}/_publish`, {
      headers,
      data: { ...publishPayload, definition: changedDefinition },
    }))
    const rolledBack = await responseData<WorkflowVersion>(await request.post(`${baseUrl}/api/enterprise/v1/ai-workflows/${created.id}/versions/${first.id}/_rollback`, { headers }))

    expect([first.version, second.version, rolledBack.version]).toEqual([1, 2, 3])
    expect(rolledBack.sourceVersionId).toBe(first.id)
    expect(rolledBack.definitionHash).toBe(first.definitionHash)
    const versionPage = await responseData<{ results: WorkflowVersion[] }>(await request.get(`${baseUrl}/api/enterprise/v1/ai-workflows/${created.id}/versions?page=1&limit=100`, { headers }))
    expect(versionPage.results.map((item) => item.version).sort()).toEqual([1, 2, 3])
  } finally {
    if (createdId > 0) {
      const response = await request.delete(`${baseUrl}/api/enterprise/v1/ai-workflows/${createdId}`, { headers })
      expect(response.ok()).toBe(true)
    }
  }
})

test("AI 智能问诊同时覆盖有设备诊断和无设备基础服务", async ({ browser, request }) => {
  requireFixture()
  const tenantSession = await issueTenantSession(request)
  const workflows = await responseData<WorkflowList>(await request.get(`${baseUrl}/api/enterprise/v1/ai-workflows?page=1&limit=100`, {
    headers: { Authorization: `Bearer ${tenantSession.accessToken}` },
  }))

  const workflow = workflows.results.find((item) => item.code === "aftersales_device_diagnosis_ai_only")
  expect(workflow, "应存在 AI 智能问诊平台模板").toBeTruthy()
  expect(workflows.results.some((item) => item.code === "aftersales_customer_service_ai_only")).toBe(false)

  for (const scenario of ["未绑定设备的快速 AI 问答", "绑定设备的完整 AI 问诊"]) {
    const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
    await installSession(context, tenantSession)
    const page = await context.newPage()
    await page.goto(`${frontendUrl}/enterprise/workflow/${workflow?.id}`)
    await page.getByRole("button", { name: "试运行", exact: true }).first().click()
    await expect(page.getByText("此流程不包含人工节点", { exact: true })).toBeVisible()
    if (scenario === "未绑定设备的快速 AI 问答") {
      const batchButton = page.getByRole("button").filter({ hasText: "个场景" })
      await expect(batchButton).toHaveCount(1)
      await batchButton.click()
      await expect(page.locator('[data-workflow-batch-status="passed"]')).toHaveCount(
        await page.locator("[data-workflow-batch-scenario]").count(),
      )
      await expect(page.locator('[data-workflow-batch-status="failed"]')).toHaveCount(0)
    }
    await page.getByLabel("验证场景", { exact: true }).click()
    await page.getByRole("option", { name: scenario, exact: true }).click()
    await page.getByRole("button", { name: "开始试运行", exact: true }).click()
    await expect(page.getByText("全部通过", { exact: true })).toBeVisible()
    await expect(page.locator('[data-workflow-node-type="knowledge_retrieve"][data-workflow-node-status="completed"], [data-workflow-node-type="knowledge_retrieve"][data-workflow-node-status="recovered"]')).toHaveCount(1)
    await expect(page.locator('[data-workflow-node-type="handoff_to_human"][data-workflow-node-status="completed"]')).toHaveCount(0)
    if (scenario === "绑定设备的完整 AI 问诊") {
      await expect(page.locator('[data-workflow-node-type="answerability_gate"][data-workflow-node-status="completed"], [data-workflow-node-type="answerability_gate"][data-workflow-node-status="recovered"]')).toHaveCount(1)
    } else {
      await expect(page.locator('[data-workflow-node-type="answerability_gate"][data-workflow-node-status="completed"]')).toHaveCount(0)
    }
    const credentialChain = workflow?.draftDefinition.modelPolicy?.credentialChain ?? []
    expect(credentialChain.length, "AI 智能问诊应声明模型凭据链").toBeGreaterThan(0)
    const credentialResult = page.locator("[data-workflow-credential-scope]").last()
    await expect(credentialResult).toBeVisible()
    const actualCredentialScope = await credentialResult.getAttribute("data-workflow-credential-scope")
    expect(credentialChain, "实际使用的凭据必须来自声明的回退链").toContain(actualCredentialScope)
    await expect(credentialResult).toHaveAttribute(
      "data-workflow-credential-fallback",
      String(actualCredentialScope !== credentialChain[0]),
    )
    await context.close()
  }
})

test("工作流详情默认进入编排，并提供逐节点试运行入口", async ({ browser, request }) => {
  requireFixture()
  const tenantSession = await issueTenantSession(request)

  for (const viewport of [
    { name: "desktop", width: 1440, height: 1000 },
    { name: "mobile", width: 390, height: 844 },
  ]) {
    const context = await browser.newContext({ viewport })
    await installSession(context, tenantSession)
    const page = await context.newPage()
    const pageErrors: string[] = []
    page.on("pageerror", (error) => pageErrors.push(error.message))

    await page.goto(`${frontendUrl}/enterprise/workflow/${workflowId}`)
    await expect(page.getByRole("tab", { name: "流程编排", exact: true })).toHaveAttribute("data-active")
    await expect(page.getByText("平台标准流程为只读版本", { exact: true }).or(page.getByLabel("模板名称"))).toBeVisible()
    await expect(page.getByText(/节点数量|连接数量|入口节点/)).toHaveCount(0)

    await page.getByRole("button", { name: "试运行", exact: true }).first().click()
    await expect(page.getByRole("heading", { name: "试运行当前草稿", exact: true })).toBeVisible()
    await expect(page.getByLabel("客户输入", { exact: true })).toBeVisible()
    await expect(
      page.getByText("此流程包含人工路径", { exact: true })
        .or(page.getByText("此流程不包含人工节点", { exact: true })),
    ).toBeVisible()
    await expect(page.getByText("业务写入已隔离；模型与知识检索按测试机器人执行", { exact: true })).toBeVisible()
    await page.keyboard.press("Escape")

    const runRequestPromise = page.waitForRequest((candidate) => (
      candidate.url().includes("/api/dashboard/ai-workflow/run/list")
    ))
    await page.getByRole("tab", { name: "运行记录", exact: true }).click()
    await expect(page.getByText("线上运行记录", { exact: true })).toBeVisible()
    const runRequest = await runRequestPromise
    expect(new URL(runRequest.url()).searchParams.get("workflowId")).toBe(String(workflowId))

    for (const [tab, expected] of [
      ["知识与变量", "产品知识随机器人版本注入"],
      ["版本发布", "当前稳定版本"],
      ["产品应用", "版本升级不会自动影响线上服务"],
    ] as const) {
      await page.getByRole("tab", { name: tab, exact: true }).click()
      await expect(page.getByText(expected, { exact: true }).first()).toBeVisible()
    }

    const layoutAudit = await page.evaluate(() => ({
      viewportWidth: document.documentElement.clientWidth,
      documentWidth: document.documentElement.scrollWidth,
      emptyButtons: Array.from(document.querySelectorAll("button")).filter((element) => {
        const rect = element.getBoundingClientRect()
        const accessibleName = element.getAttribute("aria-label")
          || element.getAttribute("title")
          || element.textContent
          || ""
        return rect.width > 0 && rect.height > 0 && !accessibleName.trim()
      }).length,
    }))

    expect(layoutAudit.documentWidth, `${viewport.name} 不应出现页面级横向滚动`).toBeLessThanOrEqual(layoutAudit.viewportWidth + 1)
    expect(layoutAudit.emptyButtons, `${viewport.name} 不应出现无名称按钮`).toBe(0)
    expect(pageErrors, `${viewport.name} 不应出现页面运行异常`).toEqual([])
    await context.close()
  }
})

test("支持人工的默认流程可以真实试跑到转人工节点", async ({ browser, request }) => {
  requireFixture()
  const tenantSession = await issueTenantSession(request)
  const workflows = await responseData<WorkflowList>(await request.get(`${baseUrl}/api/enterprise/v1/ai-workflows?page=1&limit=100`, {
    headers: { Authorization: `Bearer ${tenantSession.accessToken}` },
  }))
  const handoffWorkflow = workflows.results.find((item) => item.code === "aftersales_customer_service_default")
  expect(handoffWorkflow, "应存在支持人工路径的工作流").toBeTruthy()

  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
  await installSession(context, tenantSession)
  const page = await context.newPage()
  await page.goto(`${frontendUrl}/enterprise/workflow/${handoffWorkflow?.id}`)
  await page.getByRole("button", { name: "试运行", exact: true }).first().click()
  await expect(page.getByText("此流程包含人工路径", { exact: true })).toBeVisible()

  const batchScenarios = page.locator("[data-workflow-batch-scenario]")
  const batchScenarioCount = await batchScenarios.count()
  expect(batchScenarioCount, "人工协同流程应生成多个关键路径场景").toBeGreaterThanOrEqual(3)
  const batchButton = page.getByRole("button").filter({ hasText: "个场景" })
  await expect(batchButton).toHaveCount(1)
  await batchButton.click()
  await expect(page.locator('[data-workflow-batch-status="passed"]')).toHaveCount(batchScenarioCount)
  await expect(page.locator('[data-workflow-batch-status="failed"]')).toHaveCount(0)

  await page.getByLabel("验证场景", { exact: true }).click()
  await page.getByRole("option", { name: "客户主动要求人工", exact: true }).click()
  await expect(page.getByLabel("验证场景", { exact: true })).toContainText("客户主动要求人工")
  await expect(page.getByText("2 项已指定", { exact: true })).toBeVisible()
  await page.getByRole("button", { name: "开始试运行", exact: true }).click()
  await expect(page.getByText("全部通过", { exact: true })).toBeVisible()
  await expect(page.getByText("执行链路", { exact: true })).toBeVisible()
  const handoffStep = page.locator('[data-workflow-node-type="handoff_to_human"][data-workflow-node-status="completed"]')
  await expect(handoffStep).toHaveCount(1)
  await context.close()
})

test("默认流程的未绑定设备场景只走 AI 自助路径", async ({ browser, request }) => {
  requireFixture()
  const tenantSession = await issueTenantSession(request)
  const workflows = await responseData<WorkflowList>(await request.get(`${baseUrl}/api/enterprise/v1/ai-workflows?page=1&limit=100`, {
    headers: { Authorization: `Bearer ${tenantSession.accessToken}` },
  }))
  const workflow = workflows.results.find((item) => item.code === "aftersales_customer_service_default")
  expect(workflow, "应存在平台 AI 优先人工协同售后流程").toBeTruthy()

  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
  await installSession(context, tenantSession)
  const page = await context.newPage()
  await page.goto(`${frontendUrl}/enterprise/workflow/${workflow?.id}`)
  await page.getByRole("button", { name: "试运行", exact: true }).first().click()
  await page.getByLabel("验证场景", { exact: true }).click()
  await page.getByRole("option", { name: "未绑定设备的快速 AI 问答", exact: true }).click()
  await expect(page.getByText("模拟已识别服务码并绑定设备", { exact: false })).toHaveCount(0)
  await page.getByRole("button", { name: "开始试运行", exact: true }).click()
  await expect(page.getByText("全部通过", { exact: true })).toBeVisible()
  await expect(page.locator('[data-workflow-node-type="knowledge_retrieve"][data-workflow-node-status="completed"], [data-workflow-node-type="knowledge_retrieve"][data-workflow-node-status="recovered"]')).toHaveCount(1)
  await expect(page.locator('[data-workflow-node-type="handoff_to_human"][data-workflow-node-status="completed"]')).toHaveCount(0)
  await context.close()
})
