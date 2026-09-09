import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./workbench-page.tsx", import.meta.url), "utf8")

test("enterprise bot workbench keeps missing-agent errors behind the initial loading surface", () => {
  assert.match(source, /const \[agentLoaded, setAgentLoaded\] = useState\(false\)/)
  assert.match(source, /const agentInitialLoading = loading && !agentLoaded/)
  assert.match(source, /const displayAgent = agent \?\? \{/)
  assert.match(source, /if \(!agent && !agentInitialLoading\) \{/)
  assert.doesNotMatch(source, /if \(!agent\) return <EnterpriseAIWorkbenchLoading \/>/)
  assert.doesNotMatch(source, /if \(agentInitialLoading\) \{\s*return <EnterpriseAIWorkbenchLoading \/>/)
  assert.match(source, /setAgentLoaded\(true\)[\s\S]*setLoading\(false\)[\s\S]*setAgentOptionsLoading\(true\)/)
  assert.match(source, /RouteBreadcrumbs/)
  assert.match(source, /<RouteBreadcrumbs size="page" \/>/)
  assert.match(source, /<h1 className="sr-only">\{displayAgent\.name\}<\/h1>/)
  assert.match(source, /displayAgent\.workflowVersionId/)
  assert.match(source, /showReleaseSkeleton \|\| agentInitialLoading \? "发布状态加载中"/)
  assert.match(source, /workflowLoading \|\| agentInitialLoading \? "工作流加载中"/)
  assert.match(source, /<AIWorkflowRunsWorkspace agentId=\{displayAgent\.id\} layout="fragment" \/>/)
  assert.doesNotMatch(source, /机器人概览加载中|机器人配置加载中/)
  assert.doesNotMatch(source, /agent === null && loading/)
  assert.doesNotMatch(source, /loading\s+&&\s+agent === null/)
})
