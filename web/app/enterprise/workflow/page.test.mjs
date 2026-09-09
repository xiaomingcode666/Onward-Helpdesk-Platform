import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./page.tsx", import.meta.url), "utf8")
const detailSource = await readFile(new URL("./[workflowId]/template-page.tsx", import.meta.url), "utf8")

test("enterprise workflow list uses split summary and page adoption APIs", () => {
  assert.match(source, /fetchEnterpriseAIWorkflowSummary/)
  assert.match(source, /fetchEnterpriseAIWorkflowAdoption\(workflowIds\)/)
  assert.match(source, /const fetchWorkflowList = useCallback/)
  assert.match(source, /fetchList=\{fetchWorkflowList\}/)
  assert.doesNotMatch(source, /fetchAIAgents/)
  assert.doesNotMatch(source, /fetchEnterpriseAIWorkflows\(\{ page: 1, limit: 1000 \}\)/)
})

test("enterprise workflow list keeps local loading and pagination copy", () => {
  assert.match(source, /summaryLoading/)
  assert.match(source, /<Skeleton className="mt-2 h-7 w-14" \/>/)
  assert.match(source, /loading: "工作流加载中"/)
  assert.doesNotMatch(source, /loadingSummary/)
  assert.doesNotMatch(source, /正在加载工作流模板/)
})

test("enterprise workflow visible wording avoids AI labels", () => {
  assert.match(source, /自动接待 \+ 人工协同/)
  assert.match(source, /诊断服务/)
  assert.match(source, /基础问答/)
  assert.match(source, /功能派发/)
  assert.match(detailSource, /wd\(t, "serviceModes\.auto"\)/)
  assert.match(detailSource, /wd\(t, "serviceModes\.dispatchOnly"\)/)
  assert.match(detailSource, /wd\(t, workflowBlueprintLabels\[blueprint\]\)/)
  assert.doesNotMatch(source, /基础 AI 问答|AI 智能问诊|智能问诊|AI \+ 人工协同/)
  assert.doesNotMatch(source, /机器人/)
  assert.doesNotMatch(detailSource, /基础 AI 知识问答|AI 智能问诊|智能问诊|会话内闭环|AI 优先处理|仅 AI 流程|>仅 AI<|AI \+ 人工/)
})

test("enterprise workflow create dialog avoids explanatory descriptions", () => {
  assert.doesNotMatch(source, /DialogDescription/)
  assert.doesNotMatch(source, /选择稳定方案作为起点/)
  assert.doesNotMatch(source, /创建后只生成草稿/)
  assert.doesNotMatch(source, /有设备完整诊断/)
  assert.doesNotMatch(source, /可开放人工接管/)
  assert.doesNotMatch(source, /标准产品问答/)
  assert.doesNotMatch(source, /setCreateDescription/)
  assert.doesNotMatch(source, /new-workflow-description/)
  assert.match(source, /description: ""/)
})
