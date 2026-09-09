import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./page.tsx", import.meta.url), "utf8")

test("platform ops keeps the page shell visible while loading", () => {
  assert.doesNotMatch(source, /function OpsInitialLoadingLayout/)
  assert.doesNotMatch(source, /\{isInitialLoading \? \(\s*<OpsInitialLoadingLayout/)
  assert.match(source, /const \[hasLoadedOps, setHasLoadedOps\] = useState\(false\)/)
  assert.match(source, /const isInitialLoading = loading && !hasLoadedOps/)
  assert.match(source, /<section className="overflow-hidden rounded-lg border border-border bg-card shadow-sm" aria-busy=\{isInitialLoading \|\| undefined\} aria-label="运维监控">/)
})

test("platform ops splits initial loading across visible panels", () => {
  assert.match(source, /\{isInitialLoading \? \([\s\S]*label="运维健康加载中"/)
  assert.match(source, /<DashboardPanel title="并发 \/ 队列"[\s\S]*\{isInitialLoading \? \([\s\S]*label="队列状态加载中"/)
  assert.match(source, /<DashboardPanel title="依赖响应"[\s\S]*\{isInitialLoading \? \([\s\S]*label="依赖响应加载中"/)
  assert.match(source, /<DashboardPanel title="存储分布"[\s\S]*\{isInitialLoading \? \([\s\S]*label="存储分布加载中"/)
  assert.match(source, /<DashboardPanel title="依赖时延分布"[\s\S]*\{isInitialLoading \? \([\s\S]*label="时延分布加载中"/)
  assert.match(source, /<DashboardPanel title="数据库热点"[\s\S]*\{isInitialLoading \? \([\s\S]*label="数据库热点加载中"/)
  assert.match(source, /<DashboardPanel title="模型与接入状态"[\s\S]*\{isInitialLoading \? \([\s\S]*label="模型与接入状态加载中"/)
  assert.match(source, /<DashboardPanel title="告警事件"[\s\S]*\{isInitialLoading \? \([\s\S]*label="告警事件加载中"/)
})
