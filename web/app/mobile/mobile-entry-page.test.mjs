import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./mobile-entry-page.tsx", import.meta.url), "utf8")
const pushRegistrationSource = await readFile(new URL("../../lib/mobile/push-registration.ts", import.meta.url), "utf8")
const androidManifest = await readFile(new URL("../../android/app/src/main/AndroidManifest.xml", import.meta.url), "utf8")
const androidMainActivity = await readFile(new URL("../../android/app/src/main/java/com/digintelspace/remotehelpdesk/MainActivity.java", import.meta.url), "utf8")

test("mobile entry makes the customer account gateway primary", () => {
  assert.match(source, /!signedInCustomer && !serviceCode/)
  assert.match(source, /initialMode=\{initialState === "register" \? "register" : "account"\}/)
  assert.match(source, /initialInviteCode=\{searchParams\.get\("invite"\)/)
})

test("mobile entry uses H5 camera scanning for service codes", () => {
  assert.match(source, /import\("@zxing\/browser"\)/)
  assert.match(source, /decodeFromConstraints/)
  assert.match(source, /navigator\.mediaDevices\?\.getUserMedia/)
  assert.match(source, /initialState === "scan"/)
  assert.match(source, /onScan=\{\(\) => router\.replace\("\/mobile\?state=scan"\)\}/)
  assert.doesNotMatch(source, /@capacitor-mlkit\/barcode-scanning/)
  assert.doesNotMatch(source, /scanServiceCode/)
})

test("mobile entry keeps customer authentication and the signed-in portal inside the app", () => {
  assert.match(source, /<MobileCustomerLogin/)
  assert.match(source, /<MobileCustomerPortal/)
})

test("native root URL is normalized before authentication reads its portal", () => {
  assert.match(source, /useLayoutEffect\(\(\) => \{/)
  assert.match(source, /window\.location\.pathname !== "\/"/)
  assert.match(source, /window\.history\.replaceState\(window\.history\.state, "", normalizedUrl\)/)
})

test("native mobile deep links can open signed-in customer tabs", () => {
  assert.match(source, /url\.protocol !== "remotehelpdesk:" \|\| url\.hostname !== "mobile"/)
  assert.match(source, /mobileStates\.has\(state\)/)
  assert.match(source, /router\.replace\(mobilePath\)/)
  assert.match(androidManifest, /android:scheme="remotehelpdesk" android:host="mobile"/)
})

test("native mobile deep links retain safe page-specific detail targets", () => {
  assert.match(source, /chat: \["conversationId", "deviceId"\]/)
  assert.match(source, /tickets: \["ticketNo"\]/)
  assert.match(source, /video: \["id"\]/)
  assert.match(source, /register: \["invite"\]/)
  assert.match(source, /parameter\.length <= 128/)
})

test("native deep links survive cold and warm app starts", () => {
  assert.match(source, /App\.getLaunchUrl\(\)/)
  assert.match(androidMainActivity, /protected void onNewIntent\(Intent intent\)/)
  assert.match(androidMainActivity, /setIntent\(intent\)/)
  assert.match(androidMainActivity, /bridgeBuilder\.addWebViewListener/)
  assert.match(androidMainActivity, /public void onPageLoaded\(WebView webView\)/)
  assert.match(androidMainActivity, /pendingUrl = intentUrl\(getIntent\(\)\)/)
  assert.match(androidMainActivity, /getBridge\(\)\.getWebView\(\)\.loadUrl\(getBridge\(\)\.getLocalUrl\(\) \+ localPath\)/)
  assert.match(androidMainActivity, /"mobile"\.equals\(url\.getHost\(\)\)/)
  assert.match(androidMainActivity, /"c"\.equals\(url\.getHost\(\)\)/)
})

test("native push actions reuse the mobile deep-link allowlist", () => {
  assert.match(pushRegistrationSource, /pushNotificationActionPerformed/)
  assert.match(pushRegistrationSource, /MOBILE_PUSH_ACTION_EVENT/)
  assert.match(source, /mobilePathFromPushData/)
  assert.match(source, /for \(const key of \["deepLink", "url", "actionUrl"\]\)/)
  assert.match(source, /parsed\.pathname !== "\/mobile"/)
  assert.match(source, /mobileStates\.has\(state\)/)
  assert.match(source, /mobileStateParams\[state\]/)
  assert.match(source, /window\.addEventListener\(MOBILE_PUSH_ACTION_EVENT/)
  assert.match(source, /router\.replace\(path\)/)
})
