import { expect, test, type Page } from "@playwright/test"

test.use({ video: "off" })

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

type Captured = {
  transcribeContentType?: string
  voiceTicketBody?: string
  conversationCalls: string[]
}

async function mockCustomerPortal(page: Page, locale: "zh-CN" | "en-US", transcript: string): Promise<Captured> {
  const captured: Captured = { conversationCalls: [] }
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
  await page.route("**/api/**", async (route) => {
    const request = route.request()
    const path = new URL(request.url()).pathname
    let data: unknown = {}
    if (path.startsWith("/api/message/") || path.startsWith("/api/conversation/")) captured.conversationCalls.push(path)
    if (path === "/api/auth/profile") data = profile
    else if (path.includes("/page") || path.endsWith("/list")) data = { results: [], page: { page: 1, limit: 20, total: 0 }, hasMore: false }
    else if (path === "/api/customer/v1/voice-tickets/_transcribe") {
      captured.transcribeContentType = request.headers()["content-type"]
      data = { text: transcript, language: "zh-CN", provider: "local_whisper", durationSeconds: 1 }
    } else if (path === "/api/customer/v1/voice-tickets") {
      captured.voiceTicketBody = request.postDataBuffer()?.toString("latin1")
      data = { ticketId: 501, ticketNo: "TK-VOICE-501" }
    }
    await route.fulfill({ json: { success: true, data } })
  })
  return captured
}

async function openVoiceDialog(page: Page) {
  await page.goto("/customer/chat", { waitUntil: "domcontentloaded" })
  await page.getByTestId("voice-ticket-open").click()
  await page.getByTestId("voice-ticket-file").setInputFiles({ name: "voice.wav", mimeType: "audio/wav", buffer: sineWav() })
}

test("customer creates a standalone ticket by voice from the chat page header", async ({ page }) => {
  const transcript = "打印机今天早上无法开机，指示灯一直闪红色。"
  const captured = await mockCustomerPortal(page, "zh-CN", transcript)
  await openVoiceDialog(page)

  const textarea = page.getByTestId("voice-ticket-transcript")
  await expect(textarea).toHaveValue(transcript)
  expect(captured.transcribeContentType).toContain("multipart/form-data")

  await textarea.fill(`${transcript}已经重启过。`)
  await page.getByTestId("voice-ticket-submit").click()
  await expect(page).toHaveURL(/\/customer\/ticket/)

  const body = captured.voiceTicketBody ?? ""
  expect(body).toContain('name="audio"')
  expect(body).toContain('name="requestKey"')
  expect(body).toContain('name="transcript"')
  // Nothing is posted into any conversation.
  expect(captured.conversationCalls).toEqual([])
})

test("voice ticket dialog shows only English in English mode", async ({ page }) => {
  await mockCustomerPortal(page, "en-US", "The printer will not power on.")
  await openVoiceDialog(page)
  const dialog = page.getByRole("dialog")
  await expect(page.getByTestId("voice-ticket-transcript")).toHaveValue("The printer will not power on.")
  await expect(dialog).toContainText("Report an issue by voice")
  expect(await dialog.innerText()).not.toMatch(/[一-鿿]/)
})
