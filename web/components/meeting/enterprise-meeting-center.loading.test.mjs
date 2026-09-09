import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./enterprise-meeting-center.tsx", import.meta.url), "utf8")

test("enterprise meeting center keeps existing meetings visible while refreshing", () => {
  assert.match(source, /const \[meetingsLoaded, setMeetingsLoaded\] = useState\(false\)/)
  assert.match(source, /setMeetingsLoaded\(true\)/)
  assert.match(source, /const hasMeetings = filteredMeetings\.length > 0/)
  assert.match(source, /const meetingCenterInitialLoading = loading && !meetingsLoaded/)
  assert.match(source, /const meetingCenterRefreshing = loading && meetingsLoaded/)
  assert.match(source, /const meetingCenterBlockingError = Boolean\(error\) && !loading && !hasMeetings/)
  assert.match(source, /const meetingCenterRefreshError = Boolean\(error\) && !loading && hasMeetings/)
  assert.match(source, /meetingCenterRefreshing \? \([\s\S]*role="status" aria-busy="true"/)
  assert.match(source, /meetingCenterInitialLoading \? \(\s*<MeetingCenterSkeleton \/>/)
  assert.match(source, /aria-busy=\{meetingCenterRefreshing\}/)
  assert.doesNotMatch(source, /\{loading \? \(\s*<MeetingCenterSkeleton \/>/)
  assert.doesNotMatch(source, /ErrorState title="视频协作加载失败" description=\{error\} action=\{\{ label: "重试", onClick: load \}\}/)
})

test("enterprise meeting review keeps rendered records visible while refreshing", () => {
  assert.match(source, /const \[recordLoaded, setRecordLoaded\] = useState\(false\)/)
  assert.match(source, /setRecordLoaded\(true\)/)
  assert.match(source, /const recordInitialLoading = loading && !recordLoaded/)
  assert.match(source, /const recordRefreshing = loading && recordLoaded/)
  assert.match(source, /recordRefreshing \? \([\s\S]*role="status" aria-busy="true"/)
  assert.match(source, /recordInitialLoading \? \(\s*<div className="space-y-3 p-5" role="status" aria-busy="true">/)
  assert.doesNotMatch(source, /\{loading \? \(\s*<div className="space-y-3 p-5">/)
})
