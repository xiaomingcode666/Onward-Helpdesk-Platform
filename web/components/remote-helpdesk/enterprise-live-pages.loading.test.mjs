import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./enterprise-live-pages.tsx", import.meta.url), "utf8")

test("enterprise live pages do not keep the unused legacy workbench shell", () => {
  assert.doesNotMatch(source, /EnterpriseWorkbenchLivePage/)
  assert.doesNotMatch(source, /fetchEnterpriseWorkbenchOverview/)
  assert.doesNotMatch(source, /fetchEnterpriseWorkbenchCore/)
  assert.doesNotMatch(source, /fetchEnterpriseWorkbenchQueue/)
  assert.doesNotMatch(source, /工作台指标加载中/)
  assert.doesNotMatch(source, /待处理队列加载中/)
})

test("enterprise live pages do not keep legacy whole-page workbench loading", () => {
  assert.doesNotMatch(source, /function LivePageSkeleton/)
  assert.doesNotMatch(source, /return <LivePageSkeleton/)
  assert.doesNotMatch(source, /if \(loading\)\s*\{[\s\S]*return <LivePageSkeleton/)
  assert.doesNotMatch(source, /暂无工作台数据/)
  assert.doesNotMatch(source, /后端暂未返回企业工作台概览/)
})

test("enterprise live lists only block on their first api response", () => {
  assert.match(source, /const \[meetingListLoaded, setMeetingListLoaded\] = useState\(false\)/)
  assert.match(source, /const meetingListInitialLoading = loading && !meetingListLoaded/)
  assert.match(source, /const meetingListBlockingError = Boolean\(error\) && !meetingListLoaded/)
  assert.match(source, /const \[notificationListLoaded, setNotificationListLoaded\] = useState\(false\)/)
  assert.match(source, /const notificationListInitialLoading = loading && !notificationListLoaded/)
  assert.match(source, /const notificationListBlockingError = Boolean\(error\) && !notificationListLoaded/)
  assert.doesNotMatch(source, /loading && !data/)
  assert.doesNotMatch(source, /Boolean\(error\) && !data/)
  assert.match(source, /meetingListInitialLoading \? <ListSkeleton \/>/)
  assert.match(source, /notificationListInitialLoading \? <ListSkeleton \/>/)
  assert.doesNotMatch(source, /loading \? <ListSkeleton/)
})

test("enterprise live empty notification states stay concise", () => {
  assert.doesNotMatch(source, /当前筛选下没有通知记录/)
  assert.doesNotMatch(source, /当前范围和筛选条件下没有消息/)
  assert.doesNotMatch(source, /当前范围共/)
  assert.doesNotMatch(source, /当前账号/)
  assert.doesNotMatch(source, /当前租户/)
  assert.doesNotMatch(source, /<EmptyState title="暂无通知"[^>]+description=/)
  assert.doesNotMatch(source, /PopoverDescription/)
  assert.match(source, /title="全部标为已读"/)
  assert.match(source, /租户范围/)
  assert.match(source, /个人范围/)
  assert.match(source, /<EmptyState title="暂无通知" className="rounded-none border-0" \/>/)
})
