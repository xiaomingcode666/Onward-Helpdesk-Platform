import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

import { resolvePartnerMeetingAccess } from "./partner-meeting-access.ts"

const portalSource = await readFile(new URL("./partner-portal-pages.tsx", import.meta.url), "utf8")

const collaboration = {
  id: 21,
  ticket_id: 108,
  status: "processing",
  authorization_active: true,
  visibility: ["ticket", "meeting"],
}

function meeting(overrides = {}) {
  return {
    id: "meeting-1",
    collaboration_id: collaboration.id,
    ticket_id: collaboration.ticket_id,
    status: "active",
    ...overrides,
  }
}

test("shows only a joinable meeting belonging to the authorized collaboration", () => {
  const access = resolvePartnerMeetingAccess({
    collaboration,
    meetings: [
      meeting({ id: "other-collaboration", collaboration_id: 99 }),
      meeting({ id: "finished", status: "finished" }),
      meeting({ id: "ended", status: "ended" }),
      meeting({ id: "cancelled", status: "cancelled" }),
      meeting({ id: "scheduled", status: "scheduled" }),
      meeting({ id: "active" }),
    ],
  })

  assert.equal(access.state, "joinable")
  assert.equal(access.meeting?.id, "active")
})

test("waiting and scheduled meetings remain joinable when no active meeting exists", () => {
  const waiting = resolvePartnerMeetingAccess({
    collaboration,
    meetings: [meeting({ status: "waiting" })],
  })
  const scheduled = resolvePartnerMeetingAccess({
    collaboration,
    meetings: [meeting({ status: "scheduled" })],
  })

  assert.equal(waiting.state, "joinable")
  assert.equal(scheduled.state, "joinable")
})

test("finished and ended meetings show an ended state without a join target", () => {
  for (const terminalStatus of ["finished", "ended"]) {
    const access = resolvePartnerMeetingAccess({
      collaboration,
      meetings: [meeting({ status: terminalStatus })],
    })

    assert.deepEqual(access, { meeting: null, state: "ended", message: "视频协作已结束" })
  }
})

test("cancelled and canceled meetings show a cancelled state without a join target", () => {
  for (const terminalStatus of ["cancelled", "canceled"]) {
    const access = resolvePartnerMeetingAccess({
      collaboration,
      meetings: [meeting({ status: terminalStatus })],
    })

    assert.deepEqual(access, { meeting: null, state: "cancelled", message: "视频协作已取消" })
  }
})

test("missing and unrelated meetings do not expose a join target", () => {
  const missing = resolvePartnerMeetingAccess({ collaboration, meetings: [] })
  const unrelated = resolvePartnerMeetingAccess({
    collaboration,
    meetings: [
      meeting({ collaboration_id: 99 }),
      meeting({ id: "missing-collaboration-id", collaboration_id: undefined }),
    ],
  })

  assert.deepEqual(missing, { meeting: null, state: "none", message: "暂无可加入的视频协作" })
  assert.equal(unrelated.state, "none")
  assert.equal(unrelated.meeting, null)
})

test("invalid collaboration authorization always hides meeting access", () => {
  for (const invalidCollaboration of [
    { ...collaboration, authorization_active: false },
    { ...collaboration, status: "resolved" },
    { ...collaboration, visibility: ["ticket"] },
  ]) {
    const access = resolvePartnerMeetingAccess({
      collaboration: invalidCollaboration,
      meetings: [meeting()],
    })
    assert.deepEqual(access, { meeting: null, state: "hidden", message: "" })
  }
})

test("loading and failed meeting requests never expose a join target", () => {
  assert.equal(resolvePartnerMeetingAccess({ collaboration, meetings: null }).state, "loading")
  assert.deepEqual(
    resolvePartnerMeetingAccess({ collaboration, meetings: null, loadError: "network error" }),
    { meeting: null, state: "unavailable", message: "视频协作状态暂不可用" },
  )
})

test("ticket detail renders the meeting entry only from resolved access state", () => {
  assert.match(portalSource, /const meetingAccess = resolvePartnerMeetingAccess\(\{/)
  assert.match(portalSource, /\{meetingAccess\.meeting \? \(\s*<Button/)
  assert.match(portalSource, /meetingAccess\.state !== "hidden"/)
  assert.doesNotMatch(
    portalSource,
    /ticket\.visibility\?\.includes\("meeting"\) && ticket\.status !== "resolved" && ticket\.authorization_active \? \(/,
  )
})
