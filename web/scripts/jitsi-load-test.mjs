import crypto from "node:crypto"
import process from "node:process"
import { chromium } from "@playwright/test"

const baseURL = process.env.JITSI_LOAD_URL || "https://meet.digintelspace.com"
const appID = process.env.JITSI_LOAD_APP_ID || "remotehelpdesk"
const appSecret = process.env.JITSI_LOAD_APP_SECRET
const participants = Number.parseInt(process.env.JITSI_LOAD_PARTICIPANTS || "8", 10)
const holdSeconds = Number.parseInt(process.env.JITSI_LOAD_HOLD_SECONDS || "60", 10)
const rampMs = Number.parseInt(process.env.JITSI_LOAD_RAMP_MS || "500", 10)
const room = process.env.JITSI_LOAD_ROOM || `rhd-load-${Date.now()}`

if (!appSecret) throw new Error("JITSI_LOAD_APP_SECRET is required")
if (!Number.isInteger(participants) || participants < 1 || participants > 50) {
  throw new Error("JITSI_LOAD_PARTICIPANTS must be between 1 and 50")
}

function base64url(value) {
  return Buffer.from(value).toString("base64url")
}

function token(index) {
  const now = Math.floor(Date.now() / 1000)
  const header = base64url(JSON.stringify({ alg: "HS256", typ: "JWT" }))
  const payload = base64url(JSON.stringify({
    aud: "jitsi",
    iss: appID,
    sub: new URL(baseURL).hostname,
    room,
    nbf: now - 10,
    exp: now + holdSeconds + 300,
    context: {
      user: {
        id: `load-${index}-${crypto.randomUUID()}`,
        name: `Load participant ${index + 1}`,
        moderator: index === 0,
      },
      features: {
        livestreaming: false,
        recording: false,
        "outbound-call": false,
        transcription: false,
        "sip-outbound-call": false,
      },
      group: `load-${index}`,
    },
  }))
  const signature = crypto.createHmac("sha256", appSecret).update(`${header}.${payload}`).digest("base64url")
  return `${header}.${payload}.${signature}`
}

const browser = await chromium.launch({
  headless: true,
  args: [
    "--use-fake-device-for-media-stream",
    "--use-fake-ui-for-media-stream",
    "--autoplay-policy=no-user-gesture-required",
  ],
})

const sessions = []
try {
  const startedAt = Date.now()
  const settled = await Promise.allSettled(Array.from({ length: participants }, async (_, index) => {
    if (index > 0 && rampMs > 0) {
      await new Promise((resolve) => setTimeout(resolve, index * rampMs))
    }
    const context = await browser.newContext({ permissions: ["camera", "microphone"] })
    const page = await context.newPage()
    sessions.push(context)
    await page.goto(baseURL, { waitUntil: "domcontentloaded", timeout: 45_000 })
    await page.evaluate(async ({ deployment, meetingRoom, jwt }) => {
      window.__jitsiLoadJoined = false
      await new Promise((resolve, reject) => {
        const script = document.createElement("script")
        script.src = `${deployment}/external_api.js`
        script.onload = resolve
        script.onerror = reject
        document.head.appendChild(script)
      })
      document.body.innerHTML = '<main id="jitsi-load-root" style="width:100vw;height:100vh"></main>'
      const api = new window.JitsiMeetExternalAPI(new URL(deployment).hostname, {
        roomName: meetingRoom,
        jwt,
        parentNode: document.querySelector("#jitsi-load-root"),
        configOverwrite: {
          prejoinConfig: { enabled: false },
          startWithAudioMuted: false,
          startWithVideoMuted: false,
        },
      })
      api.addListener("videoConferenceJoined", () => { window.__jitsiLoadJoined = true })
      window.__jitsiLoadAPI = api
    }, { deployment: baseURL, meetingRoom: room, jwt: token(index) })
    await page.waitForFunction(() => window.__jitsiLoadJoined === true, undefined, { timeout: 45_000 })
    return { index, joinedAtMs: Date.now() - startedAt }
  }))

  const joined = settled.filter((result) => result.status === "fulfilled").map((result) => result.value)
  const failed = settled.length - joined.length
  const maxJoinMs = joined.length > 0 ? Math.max(...joined.map((result) => result.joinedAtMs)) : null
  process.stdout.write(JSON.stringify({ room, participants, joined: joined.length, failed, maxJoinMs }) + "\n")
  if (joined.length === 0) throw new Error("no load-test participant joined")
  await new Promise((resolve) => setTimeout(resolve, holdSeconds * 1000))
} finally {
  await Promise.allSettled(sessions.map((context) => context.close()))
  await browser.close()
}
