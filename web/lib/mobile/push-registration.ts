import type { PluginListenerHandle } from "@capacitor/core"

import { translateCurrentMessage } from "@/i18n/messages"
import { registerMobilePushToken, revokeMobilePushToken } from "@/lib/api/mobile-push"

let registrationSetup: Promise<void> | null = null
let activeRegistrationKey = ""
let listenerHandles: PluginListenerHandle[] = []
const DEVICE_ID_KEY = "remotehelpdesk-mobile-device-id"
const TOKEN_ID_KEY_PREFIX = "remotehelpdesk-mobile-push-token-id:"
const ANDROID_CONVERSATION_CHANNEL_ID = "conversation_updates"

export const MOBILE_PUSH_ACTION_EVENT = "remotehelpdesk:mobile-push-action"

function isNativePushConfigured(platform: string) {
  return platform !== "android" || process.env.NEXT_PUBLIC_ANDROID_PUSH_CONFIGURED === "true"
}

function getDeviceId() {
  const existing = window.localStorage.getItem(DEVICE_ID_KEY)?.trim()
  if (existing) return existing
  const value = globalThis.crypto?.randomUUID?.() || `device-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
  window.localStorage.setItem(DEVICE_ID_KEY, value)
  return value
}

async function removePushListeners() {
  const handles = listenerHandles
  listenerHandles = []
  await Promise.all(handles.map((handle) => handle.remove().catch(() => undefined)))
}

function tokenIDStorageKey(registrationKey: string) {
  return `${TOKEN_ID_KEY_PREFIX}${registrationKey}`
}

export function registerNativeMobilePushNotifications(registrationKey = "default") {
  const normalizedKey = registrationKey.trim() || "default"
  if (registrationSetup && activeRegistrationKey === normalizedKey) return registrationSetup

  const previousSetup = registrationSetup
  activeRegistrationKey = normalizedKey

  registrationSetup = (async () => {
    await previousSetup?.catch(() => undefined)
    const [{ Capacitor }, { PushNotifications }] = await Promise.all([
      import("@capacitor/core"),
      import("@capacitor/push-notifications"),
    ])
    if (!Capacitor.isNativePlatform()) return
    const platform = Capacitor.getPlatform()
    if (!isNativePushConfigured(platform)) return
    await removePushListeners()

    let permission = await PushNotifications.checkPermissions()
    if (permission.receive === "prompt") {
      permission = await PushNotifications.requestPermissions()
    }
    if (permission.receive !== "granted") return

    if (platform === "android") {
      await PushNotifications.createChannel({
        id: ANDROID_CONVERSATION_CHANNEL_ID,
        name: translateCurrentMessage("miscTailExtract.libMisc.push.conversationChannelName"),
        description: translateCurrentMessage("miscTailExtract.libMisc.push.conversationChannelDescription"),
        importance: 5,
        visibility: 1,
        vibration: true,
      })
    }

    listenerHandles.push(await PushNotifications.addListener("registration", ({ value }) => {
      void registerMobilePushToken({
        platform: platform === "ios" ? "ios" : "android",
        token: value,
        deviceId: getDeviceId(),
        appVersion: process.env.NEXT_PUBLIC_APP_VERSION?.trim() || undefined,
      }).then((item) => {
        window.localStorage.setItem(tokenIDStorageKey(normalizedKey), String(item.id))
      }).catch((error) => {
        console.warn("mobile push token registration failed", error)
      })
    }))
    listenerHandles.push(await PushNotifications.addListener("registrationError", (error) => {
      console.warn("mobile push registration failed", error)
    }))
    listenerHandles.push(await PushNotifications.addListener("pushNotificationActionPerformed", ({ notification }) => {
      window.dispatchEvent(new CustomEvent(MOBILE_PUSH_ACTION_EVENT, { detail: notification.data }))
    }))
    await PushNotifications.register()
  })().catch((error) => {
    if (activeRegistrationKey === normalizedKey) registrationSetup = null
    console.warn("mobile push setup failed", error)
  })

  return registrationSetup
}

export async function unregisterNativeMobilePushNotifications(registrationKey = "default") {
  const normalizedKey = registrationKey.trim() || "default"
  await registrationSetup?.catch(() => undefined)
  const storedTokenID = Number(window.localStorage.getItem(tokenIDStorageKey(normalizedKey)) || "0")
  if (storedTokenID > 0) {
    await revokeMobilePushToken(storedTokenID).catch((error) => {
      console.warn("mobile push token revoke failed", error)
    })
  }
  window.localStorage.removeItem(tokenIDStorageKey(normalizedKey))

  const { Capacitor } = await import("@capacitor/core")
  const platform = Capacitor.getPlatform()
  if (Capacitor.isNativePlatform() && isNativePushConfigured(platform)) {
    const { PushNotifications } = await import("@capacitor/push-notifications")
    await PushNotifications.unregister().catch(() => undefined)
  }
  await removePushListeners()
  if (activeRegistrationKey === normalizedKey) {
    activeRegistrationKey = ""
    registrationSetup = null
  }
}
