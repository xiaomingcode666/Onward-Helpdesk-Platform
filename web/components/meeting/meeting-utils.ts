/**
 * Jitsi 视频协作工具函数
 */

export interface JitsiConfig {
  domain: string
  roomName: string
  jwt: string
  jitsiUrl: string
  meetingId: string
}

/**
 * 生成 Jitsi 协作房间名称
 */
export function generateJitsiRoomName(tenantId: string, ticketId: string): string {
  const random = Math.random().toString(36).substring(2, 10)
  return `rhd-${tenantId}-${ticketId}-${random}`
}

/**
 * 动态加载 Jitsi External API script
 */
export function normalizeJitsiDomain(value: string): string {
  const trimmed = value.trim().replace(/\/+$/, "")
  if (!trimmed) return "meet.jit.si"
  try {
    return new URL(trimmed.includes("://") ? trimmed : `https://${trimmed}`).host
  } catch {
    return trimmed.replace(/^https?:\/\//, "").split("/")[0]
  }
}

export function jitsiExternalApiScriptUrl(value: string): string {
  const trimmed = value.trim().replace(/\/+$/, "")
  if (trimmed.includes("://")) return `${trimmed}/external_api.js`
  const protocol = trimmed.startsWith("localhost") || trimmed.startsWith("127.0.0.1") ? "http" : "https"
  return `${protocol}://${trimmed}/external_api.js`
}

export function loadJitsiExternalApi(deployment = "meet.jit.si"): Promise<JitsiMeetExternalAPIConstructor> {
  return new Promise((resolve, reject) => {
    if (typeof window !== "undefined" && window.JitsiMeetExternalAPI) {
      resolve(window.JitsiMeetExternalAPI)
      return
    }

    const script = document.createElement("script")
    script.src = jitsiExternalApiScriptUrl(deployment)
    script.async = true
    script.onload = () => {
      if (window.JitsiMeetExternalAPI) {
        resolve(window.JitsiMeetExternalAPI)
      } else {
        reject(new Error("Jitsi Meet External API failed to load"))
      }
    }
    script.onerror = () => reject(new Error("Failed to load Jitsi Meet External API script"))
    document.head.appendChild(script)
  })
}

/**
 * 获取 Jitsi 配置
 * 生产环境应从后端 API 获取
 */
export function getJitsiConfig(): Pick<JitsiConfig, "domain" | "jitsiUrl"> {
  const domain = process.env.NEXT_PUBLIC_JITSI_DOMAIN || "meet.jit.si"
  const jitsiUrl = `https://${domain}`
  return { domain, jitsiUrl }
}

/**
 * Jitsi iframe 配置选项
 */
export function getJitsiInterfaceConfig() {
  return {
    TOOLBAR_ALWAYS_VISIBLE: false,
    DEFAULT_REMOTE_DISPLAY_NAME: "Participant",
    FILM_STRIP_MAX_HEIGHT: 120,
    SHOW_JITSI_WATERMARK: false,
    SHOW_WATERMARK_FOR_GUESTS: false,
    SHOW_BRAND_WATERMARK: false,
    SHOW_POWERED_BY: false,
    DISABLE_JOIN_LEAVE_NOTIFICATIONS: false,
    DISABLE_PRESENCE_STATUS: false,
    DISABLE_TRANSCRIPTION_SUBTITLES: true,
    DISABLE_RECORDING_BUTTON: true,
    DISABLE_LIVESTREAMING_BUTTON: true,
    HIDE_INVITE_MORE_HEADER: true,
    TOOLBAR_BUTTONS: [
      "microphone",
      "camera",
      "desktop",
      "fullscreen",
      "fodeviceselection",
      "hangup",
      "profile",
      "chat",
      "recording",
      "livestreaming",
      "etherpad",
      "sharedvideo",
      "settings",
      "raisehand",
      "videoquality",
      "filmstrip",
      "invite",
      "feedback",
      "stats",
      "shortcuts",
      "tileview",
      "videobackgroundblur",
      "download",
      "help",
      "mute-everyone",
      "mute-video-everyone",
    ],
  }
}

/**
 * Jitsi 配置覆盖
 */
export function getJitsiConfigOverrides() {
  return {
    startWithAudioMuted: false,
    startWithVideoMuted: false,
    disableDeepLinking: true,
    disableSimulcast: false,
    enableWelcomePage: false,
    enableClosePage: false,
    prejoinPageEnabled: false,
    resolution: 720,
    constraints: {
      video: {
        height: { ideal: 720, max: 720, min: 240 },
      },
    },
  }
}
