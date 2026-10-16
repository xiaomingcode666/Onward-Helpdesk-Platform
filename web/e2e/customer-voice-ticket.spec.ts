import { expect, test, type Page } from "@playwright/test"

test.use({ video: "off" })

const CONVERSATION_ID = 4101

function sineWav(seconds = 1, sampleRate = 16_000) {
  const frames = seconds * sampleRate
  const buffer = Buffer.alloc(44 + frames * 2)
  buffer.write("RIFF", 0)
  buffer.writeUInt32LE(36 + frames * 2, 4)
  buffer.write("WAVEfmt ", 8)
  buffer.writeUInt32LE(16, 16)
  buffer.writeUInt16LE(1, 20)
  buffer.writeUInt16LE(1, 22)
  buffer.writeUInt32LE(sampleRate, 24)
  buffer.writeUInt32LE(sampleRate * 2, 28)
  buffer.writeUInt16LE(2, 32)
  buffer.writeUInt16LE(16, 34)
  buffer.write("data", 36)
  buffer.writeUInt32LE(frames * 2, 40)
  for (let i = 0; i < frames; i += 1) {
    buffer.writeInt16LE(Math.round(Math.sin((2 * Math.PI * 440 * i) / sampleRate) * 8000), 44 + i * 2)
  }
  return buffer
}

type Captured = { transcribeContentType?: string; voiceTicketBody?: Record<string, unknown>; sentMessages: number }

async function mockCustomerPortal(page: Page, locale: "zh-CN" | "en-US", transcript: string): Promise<Captured> {
  const captured: Captured = { sentMessages: 0 }
  const profile = {
    accessToken: "voice-ticket-fixture", domainType: "customer", tenantId: 7001,
    user: { id: 8101, username: "voice-customer", nickname: "Voice Customer", avatar: "", status: 0, roles: ["customer"] },
    roles: ["customer"], permissions: [], locale, timezone: "UTC",
  }
  await page.addInitScript(({ value, loc }) => {
    localStorage.setItem("remote-helpdesk-session:customer", JSON.stringify(value))
    localStorage.setItem("remote-helpdesk-session", JSON.stringify(value))
    localStorage.setItem("remote-helpdesk-locale", loc)
    localStorage.setItem("remote-helpdesk-locale:customer:7001", loc)
  }, { value: profile, loc: locale })
  const conversation = {
    id: CONVERSATION_ID, status: "active", service_mode: 1, human_handoff_enabled: true, ticket_creation_enabled: true,
    priority: 0, last_message_summary: "", last_message_at: "2026-10-16T08:00:00Z", last_active_at: "2026-10-16T08:00:00Z",
    customer_unread_count: 0, device_id: 0, device_no: "", product_name: "", current_assignee_name: "",
    current_ticket_id: 0, current_ticket_no: "", current_meeting_id: "", current_meeting_status: "",
  }
  await page.route("**/api/**", async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const path = url.pathname
    let data: unknown = {}
    if (path === "/api/auth/profile") data = profile
    else if (path === "/api/customer/v1/conversations/page") data = { results: [conversation], page: { page: 1, limit: 20, total: 1 } }
    else if (path === `/api/customer/v1/conversations/${CONVERSATION_ID}`) data = conversation
    else if (path.includes("/page") || path.endsWith("/list")) data = { results: [], page: { page: 1, limit: 20, total: 0 }, hasMore: false }
    else if (path === "/api/message/upload_audio") data = { assetId: "voice-asset-e2e", filename: "voice.wav", contentType: "audio/wav", size: 32044 }
    else if (path === "/api/message/send") {
      captured.sentMessages += 1
      data = { id: 9001, conversationId: CONVERSATION_ID, clientMsgId: "voice", senderType: "customer", messageType: "audio", content: "voice.wav", payload: "{}", createdAt: "2026-10-16T08:01:00Z" }
    } else if (path === "/api/message/transcribe_audio") {
      captured.transcribeContentType = request.headers()["content-type"]
      data = { text: transcript, language: "zh-CN", provider: "local_whisper", durationSeconds: 1 }
    } else if (path === "/api/conversation/voice_ticket") {
      captured.voiceTicketBody = request.postDataJSON()
      data = { ticketId: 501, ticketNo: "TK-VOICE-501", created: true }
    }
    await route.fulfill({ json: { success: true, data } })
  })
  return captured
}

async function openVoiceDialog(page: Page) {
  await page.goto(`/customer/chat?conversationId=${CONVERSATION_ID}`, { waitUntil: "domcontentloaded" })
  await page.getByTestId("voice-ticket-open").click()
  await page.getByTestId("voice-ticket-file").setInputFiles({ name: "voice.wav", mimeType: "audio/wav", buffer: sineWav() })
}

test("customer uploads voice, reviews transcript and creates a ticket", async ({ page }) => {
  const transcript = "打印机今天早上无法开机，指示灯一直闪红色。"
  const captured = await mockCustomerPortal(page, "zh-CN", transcript)
  await openVoiceDialog(page)

  const textarea = page.getByTestId("voice-ticket-transcript")
  await expect(textarea).toHaveValue(transcript)
  expect(captured.sentMessages).toBe(1)
  expect(captured.transcribeContentType).toContain("multipart/form-data")

  await textarea.fill(`${transcript}已经重启过。`)
  await page.getByTestId("voice-ticket-submit").click()
  await expect(page.getByText("已为您创建工单 TK-VOICE-501")).toBeVisible()
  expect(captured.voiceTicketBody).toMatchObject({
    conversationId: CONVERSATION_ID,
    transcript: `${transcript}已经重启过。`,
    audioAssetId: "voice-asset-e2e",
  })
})

test("voice ticket dialog shows only English in English mode", async ({ page }) => {
  await mockCustomerPortal(page, "en-US", "The printer will not power on.")
  await openVoiceDialog(page)
  const dialog = page.getByRole("dialog")
  await expect(page.getByTestId("voice-ticket-transcript")).toHaveValue("The printer will not power on.")
  await expect(dialog).toContainText("Report an issue by voice")
  expect(await dialog.innerText()).not.toMatch(/[一-鿿]/)
})
