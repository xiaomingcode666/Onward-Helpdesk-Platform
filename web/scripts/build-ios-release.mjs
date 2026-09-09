import { spawnSync } from "node:child_process"
import { existsSync, mkdirSync, readdirSync, writeFileSync } from "node:fs"
import { resolve } from "node:path"

const teamId = process.env.RHD_IOS_TEAM_ID?.trim()
if (!teamId) {
  console.error("RHD_IOS_TEAM_ID is required for an iOS release build")
  process.exit(1)
}
if (!/^[A-Z0-9]{10}$/.test(teamId)) {
  console.error("RHD_IOS_TEAM_ID must be a 10-character Apple Team ID")
  process.exit(1)
}

const bundleId = (process.env.RHD_IOS_BUNDLE_ID || "com.digintelspace.remotehelpdesk").trim()
if (!/^[a-zA-Z][\w-]*(?:\.[a-zA-Z][\w-]*)+$/.test(bundleId)) {
  console.error(`Invalid iOS bundle ID: ${bundleId}`)
  process.exit(1)
}

const buildNumber = (process.env.RHD_IOS_BUILD_NUMBER || "1").trim()
if (!/^[1-9]\d*$/.test(buildNumber)) {
  console.error("RHD_IOS_BUILD_NUMBER must be a positive integer")
  process.exit(1)
}

const marketingVersion = (process.env.RHD_IOS_MARKETING_VERSION || "1.0.0").trim()
if (!/^\d+(?:\.\d+){1,2}$/.test(marketingVersion)) {
  console.error("RHD_IOS_MARKETING_VERSION must contain two or three numeric components")
  process.exit(1)
}

const exportMethod = (process.env.RHD_IOS_EXPORT_METHOD || "app-store-connect").trim()
const allowedExportMethods = new Set(["app-store-connect", "ad-hoc", "development", "enterprise"])
if (!allowedExportMethods.has(exportMethod)) {
  console.error(`Unsupported RHD_IOS_EXPORT_METHOD: ${exportMethod}`)
  process.exit(1)
}

const xcodebuildCheck = spawnSync("xcrun", ["--find", "xcodebuild"], { stdio: "ignore" })
if (xcodebuildCheck.status !== 0) {
  console.error("Full Xcode is required; Apple Command Line Tools alone cannot archive an iOS app")
  process.exit(1)
}

const command = process.platform === "win32" ? "pnpm.cmd" : "pnpm"
const releaseEnvironment = {
  ...process.env,
  CAPACITOR_APP_ID: bundleId,
}
const iosRoot = resolve("ios/App")
const iosProjectPath = resolve(iosRoot, "App.xcodeproj")
const iosEntitlementsPath = resolve(iosRoot, "App/App.entitlements")
const requiredNativeSources = [
  iosProjectPath,
  resolve(iosRoot, "Podfile"),
  resolve(iosRoot, "App/AppDelegate.swift"),
  resolve(iosRoot, "App/SceneDelegate.swift"),
  resolve(iosRoot, "App/Info.plist"),
  resolve(iosRoot, "App/App.Debug.entitlements"),
]
const outputRoot = resolve("ios/build/release")
const archivePath = resolve(outputRoot, "RemoteHelpDesk.xcarchive")
const exportPath = resolve(outputRoot, "export")
const exportOptionsPath = resolve(outputRoot, "ExportOptions.plist")

function run(executable, args, options = {}) {
  const result = spawnSync(executable, args, {
    stdio: "inherit",
    env: releaseEnvironment,
    ...options,
  })
  if (result.status !== 0) process.exit(result.status ?? 1)
}

function readBundleIdentifier(infoPlistPath) {
  const result = spawnSync("plutil", ["-extract", "CFBundleIdentifier", "raw", infoPlistPath], {
    encoding: "utf8",
    env: releaseEnvironment,
  })
  if (result.status !== 0) {
    console.error(`Unable to read bundle identifier from ${infoPlistPath}`)
    process.exit(result.status ?? 1)
  }
  return result.stdout.trim()
}

function validateArchiveBundleIdentifiers() {
  const appPath = resolve(archivePath, "Products/Applications/App.app")
  const frameworksPath = resolve(appPath, "Frameworks")
  const appBundleId = readBundleIdentifier(resolve(appPath, "Info.plist"))
  if (appBundleId !== bundleId) {
    console.error(`Archived app bundle ID ${appBundleId} does not match ${bundleId}`)
    process.exit(1)
  }
  if (!existsSync(frameworksPath)) {
    console.error(`Archived app frameworks are missing: ${frameworksPath}`)
    process.exit(1)
  }

  const identifiers = new Map([[appBundleId, appPath]])
  for (const entry of readdirSync(frameworksPath, { withFileTypes: true })) {
    if (!entry.isDirectory() || !entry.name.endsWith(".framework")) continue
    const frameworkPath = resolve(frameworksPath, entry.name)
    const frameworkBundleId = readBundleIdentifier(resolve(frameworkPath, "Info.plist"))
    const existingPath = identifiers.get(frameworkBundleId)
    if (existingPath) {
      console.error(`Duplicate bundle identifier ${frameworkBundleId}: ${existingPath} and ${frameworkPath}`)
      process.exit(1)
    }
    identifiers.set(frameworkBundleId, frameworkPath)
  }
}

const missingNativeSources = requiredNativeSources.filter((path) => !existsSync(path))
if (missingNativeSources.length > 0) {
  console.error("The checked-in iOS native project is incomplete:")
  missingNativeSources.forEach((path) => console.error(`- ${path}`))
  console.error("Restore the tracked iOS sources before building; generating a default Capacitor project would omit RemoteHelpDesk native capabilities")
  process.exit(1)
}

mkdirSync(outputRoot, { recursive: true })
writeFileSync(
  exportOptionsPath,
  `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>method</key>
  <string>${exportMethod}</string>
  <key>signingStyle</key>
  <string>automatic</string>
  <key>teamID</key>
  <string>${teamId}</string>
</dict>
</plist>
`
)

run(command, ["mobile:build"])
run(command, ["exec", "cap", "sync", "ios"])
run("pod", ["install"], { cwd: iosRoot })
writeFileSync(
  iosEntitlementsPath,
  `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>aps-environment</key>
  <string>production</string>
</dict>
</plist>
`
)
run("xcodebuild", [
  "-workspace", "App.xcworkspace",
  "-scheme", "App",
  "-configuration", "Release",
  "-destination", "generic/platform=iOS",
  "-archivePath", archivePath,
  `RHD_IOS_BUNDLE_ID=${bundleId}`,
  `RHD_IOS_TEAM_ID=${teamId}`,
  `RHD_IOS_MARKETING_VERSION=${marketingVersion}`,
  `RHD_IOS_BUILD_NUMBER=${buildNumber}`,
  "-allowProvisioningUpdates",
  "archive",
], { cwd: iosRoot })
validateArchiveBundleIdentifiers()
run("xcodebuild", [
  "-exportArchive",
  "-archivePath", archivePath,
  "-exportPath", exportPath,
  "-exportOptionsPlist", exportOptionsPath,
  "-allowProvisioningUpdates",
], { cwd: iosRoot })

if (!existsSync(exportPath)) process.exit(1)
console.log(`Signed iOS export: ${exportPath}`)
