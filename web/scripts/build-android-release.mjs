import { spawnSync } from "node:child_process"
import { existsSync, readFileSync } from "node:fs"
import { delimiter, resolve } from "node:path"

const requiredSigningVariables = [
  "RHD_ANDROID_KEYSTORE_FILE",
  "RHD_ANDROID_KEYSTORE_PASSWORD",
  "RHD_ANDROID_KEY_ALIAS",
  "RHD_ANDROID_KEY_PASSWORD",
]

for (const name of requiredSigningVariables) {
  if (!process.env[name]?.trim()) {
    console.error(`${name} is required for a signed Android release build`)
    process.exit(1)
  }
}

const applicationId = (
  process.env.RHD_ANDROID_APPLICATION_ID ||
  "com.digintelspace.remotehelpdesk"
).trim()
if (!/^[a-zA-Z][\w]*(?:\.[a-zA-Z][\w]*)+$/.test(applicationId)) {
  console.error(`Invalid Android application ID: ${applicationId}`)
  process.exit(1)
}

const googleServicesPath = resolve("android/app/google-services.json")
if (existsSync(googleServicesPath)) {
  let googleServices
  try {
    googleServices = JSON.parse(readFileSync(googleServicesPath, "utf8"))
  } catch (error) {
    console.error(`Invalid Firebase configuration at ${googleServicesPath}: ${error.message}`)
    process.exit(1)
  }
  const configuredPackages = (googleServices.client || [])
    .map((client) => client?.client_info?.android_client_info?.package_name)
    .filter(Boolean)
  if (!configuredPackages.includes(applicationId)) {
    console.error(`Firebase configuration does not contain Android application ID ${applicationId}`)
    process.exit(1)
  }
} else if (process.env.RHD_ANDROID_REQUIRE_PUSH?.trim().toLowerCase() === "true") {
  console.error(`RHD_ANDROID_REQUIRE_PUSH=true but ${googleServicesPath} is missing`)
  process.exit(1)
} else {
  console.warn(`Warning: ${googleServicesPath} is missing; this build cannot register for FCM push notifications`)
}

const versionCode = (process.env.RHD_ANDROID_VERSION_CODE || "1").trim()
if (!/^[1-9]\d*$/.test(versionCode)) {
  console.error("RHD_ANDROID_VERSION_CODE must be a positive integer")
  process.exit(1)
}

const keystorePath = resolve(process.env.RHD_ANDROID_KEYSTORE_FILE)
if (!existsSync(keystorePath)) {
  console.error(`Android keystore does not exist: ${keystorePath}`)
  process.exit(1)
}

const gradleCommand = process.platform === "win32" ? "gradlew.bat" : "./gradlew"
const apiBaseUrl = (
  process.env.RHD_ANDROID_API_BASE_URL ||
  process.env.MOBILE_API_BASE_URL ||
  ""
).trim()
if (!apiBaseUrl) {
  console.error("RHD_ANDROID_API_BASE_URL is required for a native Android release build")
  process.exit(1)
}
if (!apiBaseUrl.startsWith("https://")) {
  console.error("RHD_ANDROID_API_BASE_URL must use HTTPS for a release build")
  process.exit(1)
}

function findJavaHome() {
  const executable = process.platform === "win32" ? "java.exe" : "java"
  const candidates = [process.env.JAVA_HOME?.trim()]

  if (process.platform === "darwin") {
    candidates.push(
      "/Applications/Android Studio.app/Contents/jbr/Contents/Home",
      "/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home",
      "/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home",
      "/usr/local/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home",
      "/usr/local/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home"
    )
  }

  return candidates.find((candidate) =>
    candidate && existsSync(resolve(candidate, "bin", executable))
  )
}

const javaHome = findJavaHome()
if (!javaHome) {
  console.error("JDK 17 or newer is required for an Android release build; set JAVA_HOME")
  process.exit(1)
}

function findAndroidSdk() {
  const candidates = [
    process.env.ANDROID_HOME?.trim(),
    process.env.ANDROID_SDK_ROOT?.trim(),
  ]

  if (process.platform === "darwin") {
    candidates.push(
      resolve(process.env.HOME || "", "Library", "Android", "sdk"),
      "/usr/local/share/android-commandlinetools"
    )
  }

  return candidates.find((candidate) =>
    candidate && existsSync(resolve(candidate, "platform-tools"))
  )
}

const androidSdk = findAndroidSdk()
if (!androidSdk) {
  console.error("Android SDK is required for an Android release build; set ANDROID_HOME")
  process.exit(1)
}

const releaseEnvironment = {
  ...process.env,
  ANDROID_HOME: androidSdk,
  ANDROID_SDK_ROOT: androidSdk,
  JAVA_HOME: javaHome,
  PATH: `${resolve(javaHome, "bin")}${delimiter}${process.env.PATH || ""}`,
  RHD_ANDROID_API_BASE_URL: apiBaseUrl,
  RHD_ANDROID_APPLICATION_ID: applicationId,
  RHD_ANDROID_KEYSTORE_FILE: keystorePath,
  RHD_ANDROID_VERSION_CODE: versionCode,
  RHD_ANDROID_VERSION_NAME: (process.env.RHD_ANDROID_VERSION_NAME || "1.0.0").trim(),
}

function run(executable, args, options = {}) {
  const result = spawnSync(executable, args, {
    stdio: "inherit",
    env: releaseEnvironment,
    ...options,
  })
  if (result.status !== 0) process.exit(result.status ?? 1)
}

run(gradleCommand, ["validateReleaseBundle", "assembleRelease"], { cwd: resolve("android") })

console.log("Signed Android release outputs:")
console.log("- android/app/build/outputs/bundle/release/app-release.aab")
console.log("- android/app/build/outputs/apk/release/app-release.apk")
