import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./mobile-service-client.tsx", import.meta.url), "utf8")

test("a signed-in customer with an existing binding enters the device conversation directly", () => {
  assert.match(source, /fetchCustomerDeviceAccess\(resolvedDeviceId\)/)
  assert.match(source, /if \(access\.accessible\)/)
  assert.match(source, /openDeviceConversation\(resolvedDeviceId, loadRevision\)/)
  assert.match(source, /loadRevision !== serviceCodeLoadRevisionRef\.current/)
  assert.match(source, /router\.replace\(`\/customer\/chat\?conversationId=\$\{conversation\.id\}`\)/)
  assert.doesNotMatch(source, /fetchCustomerDevices\(\)/)
})

test("new device binding enters the conversation without a device-page detour", () => {
  assert.match(source, /openDeviceConversation\(binding\.deviceId\)/)
  assert.doesNotMatch(source, /router\.replace\(`\/customer\/devices\?deviceId=/)
})

test("access checks and post-binding conversation failures use a retry-only state", () => {
  assert.match(source, /setEntryState\(buildDeviceAccessErrorState\(nextState, reason\)\)/)
  assert.match(source, /setEntryState\(buildDeviceAccessErrorState\(entryState, reason\)\)/)
  assert.doesNotMatch(source, /当前账号/)
  assert.match(source, /服务权限暂不可用/)
})
