import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const apiSource = await readFile(new URL("../lib/api/im.ts", import.meta.url), "utf8")
const customerEntryApiSource = await readFile(new URL("../lib/api/customer-entry.ts", import.meta.url), "utf8")
const componentSource = await readFile(new URL("./im-message-html.tsx", import.meta.url), "utf8")
const messageSource = await readFile(new URL("../lib/im-message.ts", import.meta.url), "utf8")
const timelineSource = await readFile(new URL("./remote-helpdesk/customer-conversation-timeline.tsx", import.meta.url), "utf8")
const mobileConversationSource = await readFile(new URL("./remote-helpdesk/mobile-customer-conversation-workbench.tsx", import.meta.url), "utf8")

test("native customer media URLs preserve entry-session authorization", () => {
  assert.match(apiSource, /const isCustomerPortal = !session \|\| session\.domainType === "customer"/)
  assert.match(apiSource, /const customerSessionToken = getCustomerSessionToken\(\)/)
  assert.match(apiSource, /customerPath\}\?customerSessionToken=/)
  assert.match(apiSource, /const operatorPath = `\/api\/media\//)
  assert.match(apiSource, /session\.channelId && session\.channelId !== channelId/)
  assert.match(customerEntryApiSource, /persistCustomerSession\(result\)/)
})

test("media playback falls back to authenticated download with progress and retry", () => {
  assert.match(componentSource, /const loadAuthenticatedBlob = async \(\) =>/)
  assert.match(componentSource, /fetchImMediaAsset\(assetId, controller\.signal, \(loadedBytes, totalBytes\) =>/)
  assert.match(componentSource, /const percent = \(loadedBytes \/ totalBytes\) \* 100/)
  assert.match(componentSource, /if \(!fallbackAttempted\) \{\s*void loadAuthenticatedBlob\(\)/)
  assert.match(componentSource, /fallbackAttempted = false/)
  assert.match(componentSource, /media\.addEventListener\("progress", handleProgress\)/)
  assert.match(componentSource, /const hasPlayableMediaData = \(\) => media instanceof HTMLMediaElement && media\.readyState >= 2/)
  assert.match(componentSource, /media\.addEventListener\("loadedmetadata", handleMediaMetadata\)/)
  assert.match(componentSource, /media\.addEventListener\("loadeddata", handleMediaReady\)/)
  assert.match(componentSource, /media\.addEventListener\("canplay", handleMediaReady\)/)
  assert.match(componentSource, /if \(hasPlayableMediaData\(\)\) handleReady\(\)/)
  assert.doesNotMatch(componentSource, /const readyEvent = media instanceof HTMLImageElement \? "load" : "loadedmetadata"/)
  assert.match(componentSource, /media\.classList\.remove\("hidden"\)/)
  assert.match(componentSource, /media\.controls = true/)
  assert.match(componentSource, /state === "error" \? "supportChat\.mediaLoadFailed"/)
  assert.match(componentSource, /retry\?\.addEventListener\("click", handleRetry\)/)
  assert.match(componentSource, /media\.removeEventListener\("loadedmetadata", handleMediaMetadata\)/)
  assert.match(componentSource, /media\.removeEventListener\("loadeddata", handleMediaReady\)/)
  assert.match(componentSource, /media\.removeEventListener\("canplay", handleMediaReady\)/)
  assert.match(componentSource, /activeController\?\.abort\(\)/)
  assert.match(componentSource, /objectUrls\.forEach\(\(objectUrl\) => URL\.revokeObjectURL\(objectUrl\)\)/)
})

test("knowledge answer payloads render source citations in both customer portals", () => {
  assert.match(messageSource, /parsed\?\.kind !== "knowledge_answer"/)
  assert.match(messageSource, /Array\.isArray\(parsed\.knowledgeCitations\)/)
  assert.match(timelineSource, /<KnowledgeMessageCitations payload=\{message\.payload\} \/>/)
  assert.match(mobileConversationSource, /<KnowledgeMessageCitations payload=\{message\.payload\} \/>/)
})
