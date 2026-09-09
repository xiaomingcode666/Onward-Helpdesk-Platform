import assert from "node:assert/strict"
import test from "node:test"
import { readFile } from "node:fs/promises"
import vm from "node:vm"
import ts from "typescript"

async function loadModule() {
  const source = await readFile(new URL("./im-realtime-state.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "im-realtime-state.ts",
  })
  const sandbox = {
    exports: {},
    module: { exports: {} },
    require(id) {
      if (id === "@/lib/im-message-merge") {
        return { mergeImMessagesByIdAsc: () => [] }
      }
      if (id === "@/lib/im-message") {
        return { summarizeIMMessage: () => "" }
      }
      throw new Error(`Unexpected dependency: ${id}`)
    },
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

const engineerTyping = {
  conversationId: 8,
  actorId: "user:101",
  participantType: "agent",
  displayName: "维修工程师",
  typing: true,
  expiresAt: "2026-07-28T22:20:05Z",
}

const supplierTyping = {
  conversationId: 8,
  actorId: "user:202",
  participantType: "partner",
  displayName: "供应商",
  typing: true,
}

test("stopping one participant does not clear another participant's typing state", async () => {
  const { updateRealtimeTypingState } = await loadModule()
  let state = updateRealtimeTypingState({}, engineerTyping)
  state = updateRealtimeTypingState(state, supplierTyping)
  state = updateRealtimeTypingState(state, { ...engineerTyping, typing: false })

  assert.equal(state[engineerTyping.actorId], undefined)
  assert.equal(state[supplierTyping.actorId].displayName, "供应商")
})

test("typing expiry removes only the expired participant", async () => {
  const { expireRealtimeTypingActor, updateRealtimeTypingState } = await loadModule()
  let state = updateRealtimeTypingState({}, engineerTyping)
  state = updateRealtimeTypingState(state, supplierTyping)
  state = expireRealtimeTypingActor(state, supplierTyping.actorId)

  assert.equal(state[supplierTyping.actorId], undefined)
  assert.equal(state[engineerTyping.actorId].displayName, "维修工程师")
})

test("active typing is also an authoritative online signal", async () => {
  const { presenceFromRealtimeTyping } = await loadModule()
  const presence = presenceFromRealtimeTyping(engineerTyping, "2026-07-28T22:20:00Z")

  assert.equal(presence.actorId, engineerTyping.actorId)
  assert.equal(presence.online, true)
  assert.equal(presence.changedAt, "2026-07-28T22:20:00Z")
  assert.equal(presence.expiresAt, engineerTyping.expiresAt)
})

test("typing stop does not imply participant offline", async () => {
  const { presenceFromRealtimeTyping } = await loadModule()
  assert.equal(presenceFromRealtimeTyping({ ...engineerTyping, typing: false }), null)
})

test("short typing lease does not shorten an existing presence lease", async () => {
  const { mergeRealtimePresenceActor, presenceFromRealtimeTyping } = await loadModule()
  const current = {
    [engineerTyping.actorId]: {
      conversationId: 8,
      actorId: engineerTyping.actorId,
      participantType: "agent",
      online: true,
      expiresAt: "2026-07-28T22:21:15Z",
    },
  }
  const typingPresence = presenceFromRealtimeTyping(engineerTyping)
  const merged = mergeRealtimePresenceActor(current, typingPresence)

  assert.equal(merged[engineerTyping.actorId].expiresAt, "2026-07-28T22:21:15Z")
})

test("presence expiry ignores a stale timer and removes the matching lease", async () => {
  const { expireRealtimePresenceActor } = await loadModule()
  const current = {
    [engineerTyping.actorId]: {
      conversationId: 8,
      actorId: engineerTyping.actorId,
      participantType: "agent",
      online: true,
      expiresAt: "2026-07-28T22:21:15Z",
    },
  }

  assert.equal(
    expireRealtimePresenceActor(current, engineerTyping.actorId, "2026-07-28T22:20:05Z"),
    current,
  )
  assert.equal(
    expireRealtimePresenceActor(current, engineerTyping.actorId, "2026-07-28T22:21:15Z")[engineerTyping.actorId],
    undefined,
  )
})

test("presence expiry delay tolerates clock and timer jitter", async () => {
  const { realtimePresenceExpiryDelay } = await loadModule()

  assert.equal(
    realtimePresenceExpiryDelay("2026-07-28T22:20:05Z", Date.parse("2026-07-28T22:20:00Z")),
    5_250,
  )
  assert.equal(realtimePresenceExpiryDelay("invalid"), null)
  assert.equal(realtimePresenceExpiryDelay(undefined), null)
})
