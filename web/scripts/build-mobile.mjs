import { spawnSync } from "node:child_process"
import { copyFileSync } from "node:fs"
import { resolve } from "node:path"

const apiBaseUrl = (process.env.MOBILE_API_BASE_URL || process.env.NEXT_PUBLIC_API_BASE_URL || "").trim()
if (!apiBaseUrl) {
  console.error("MOBILE_API_BASE_URL is required, for example https://api.example.com")
  process.exit(1)
}

if (!/^https:\/\//i.test(apiBaseUrl)) {
  console.error("MOBILE_API_BASE_URL must use HTTPS for a release build")
  process.exit(1)
}

const legalEntityName = (process.env.MOBILE_LEGAL_ENTITY_NAME || process.env.NEXT_PUBLIC_LEGAL_ENTITY_NAME || "").trim()
const privacyContactEmail = (process.env.MOBILE_PRIVACY_CONTACT_EMAIL || process.env.NEXT_PUBLIC_PRIVACY_CONTACT_EMAIL || "").trim()
const dataStorageRegion = (process.env.MOBILE_DATA_STORAGE_REGION || process.env.NEXT_PUBLIC_DATA_STORAGE_REGION || "").trim()
const missingPrivacyDisclosure = [
  ["MOBILE_LEGAL_ENTITY_NAME", legalEntityName],
  ["MOBILE_PRIVACY_CONTACT_EMAIL", privacyContactEmail],
  ["MOBILE_DATA_STORAGE_REGION", dataStorageRegion],
].filter(([, value]) => !value).map(([name]) => name)
if (missingPrivacyDisclosure.length > 0) {
  console.error(`Mobile release privacy disclosure is incomplete: ${missingPrivacyDisclosure.join(", ")}`)
  process.exit(1)
}
if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(privacyContactEmail)) {
  console.error("MOBILE_PRIVACY_CONTACT_EMAIL must be a valid email address")
  process.exit(1)
}

const command = process.platform === "win32" ? "pnpm.cmd" : "pnpm"
const androidPushConfigured = process.env.NEXT_PUBLIC_ANDROID_PUSH_CONFIGURED?.trim().toLowerCase() === "true"
const result = spawnSync(command, ["exec", "next", "build"], {
  stdio: "inherit",
  env: {
    ...process.env,
    NEXT_STATIC_EXPORT: "1",
    NEXT_PUBLIC_API_BASE_URL: apiBaseUrl,
    NEXT_PUBLIC_LEGAL_ENTITY_NAME: legalEntityName,
    NEXT_PUBLIC_PRIVACY_CONTACT_EMAIL: privacyContactEmail,
    NEXT_PUBLIC_DATA_STORAGE_REGION: dataStorageRegion,
    NEXT_PUBLIC_ANDROID_PUSH_CONFIGURED: androidPushConfigured ? "true" : "false",
    NEXT_PUBLIC_ENABLE_LOCAL_LOGIN_DEFAULTS: "false",
    NEXT_PUBLIC_LOCAL_LOGIN_PLATFORM_USERNAME: "",
    NEXT_PUBLIC_LOCAL_LOGIN_ENTERPRISE_USERNAME: "",
    NEXT_PUBLIC_LOCAL_LOGIN_PARTNER_USERNAME: "",
    NEXT_PUBLIC_LOCAL_LOGIN_CUSTOMER_USERNAME: "",
    NEXT_PUBLIC_LOCAL_LOGIN_PLATFORM_PASSWORD: "",
    NEXT_PUBLIC_LOCAL_LOGIN_ENTERPRISE_PASSWORD: "",
    NEXT_PUBLIC_LOCAL_LOGIN_PARTNER_PASSWORD: "",
    NEXT_PUBLIC_LOCAL_LOGIN_CUSTOMER_PASSWORD: "",
  },
})

if (result.status !== 0) {
  process.exit(result.status ?? 1)
}

// Capacitor opens index.html. The native package is a customer service app,
// so its root must render the dedicated mobile entry instead of the web portal.
copyFileSync(resolve("out/mobile.html"), resolve("out/index.html"))
