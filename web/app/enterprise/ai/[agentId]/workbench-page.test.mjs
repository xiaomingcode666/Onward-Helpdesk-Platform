import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./workbench-page.tsx", import.meta.url), "utf8")

test("agent workbench uses paged enterprise option APIs", () => {
  assert.match(source, /fetchEnterpriseAIAgents/)
  assert.match(source, /const OPTION_PAGE_SIZE = 50/)
  assert.match(source, /fetchEnterpriseAIAgents\(\{ page: 1, limit: OPTION_PAGE_SIZE \}\)/)
  assert.match(source, /fetchEnterpriseAIWorkflows\(\{ page: 1, limit: OPTION_PAGE_SIZE \}\)/)
  assert.match(source, /fetchEnterpriseAIWorkflowVersions\(workflowId, \{\s*page: pageNumber,\s*limit: OPTION_PAGE_SIZE,\s*releaseChannel: "stable",\s*status: Status\.Ok,/)
  assert.match(source, /function OptionPager/)
  assert.doesNotMatch(source, /fetchAIAgents/)
  assert.doesNotMatch(source, /limit:\s*1000/)
  assert.doesNotMatch(source, /fetchEnterpriseAIWorkflowVersions\(workflowId, \{ page: 1, limit: 100 \}\)/)
})

test("agent workbench keeps selected options during paged loading", () => {
  assert.match(source, /mergeById\(filterSwitchableAgents\(nextPage\.results\), currentAgent\)/)
  assert.match(source, /mergeById\(filterWorkflowTemplates\(nextPage\.results\), currentWorkflow\)/)
  assert.match(source, /const \[bindingVersionsPage, setBindingVersionsPage\] = useState<OptionPageState>\(emptyOptionPage\)/)
  assert.match(source, /setBindingVersionsPage\(versionPage\.page\)/)
  assert.match(source, /mergeById\(pagedStableVersions, currentVersion\)/)
  assert.match(source, /page=\{bindingVersionsPage\}/)
  assert.match(source, /接待配置选项加载失败/)
  assert.match(source, /工作流选项加载失败/)
})

test("agent workbench renders primary data before secondary modules finish", () => {
  assert.match(source, /const agentData = await fetchAIAgent\(agentId\)/)
  assert.match(source, /setAgent\(agentData\)[\s\S]*setAgentLoaded\(true\)[\s\S]*setLoading\(false\)[\s\S]*setAgentOptionsLoading\(true\)/)
  assert.match(source, /const \[releasesLoading, setReleasesLoading\] = useState\(false\)/)
  assert.match(source, /const \[capabilitiesLoading, setCapabilitiesLoading\] = useState\(false\)/)
  assert.match(source, /const \[workflowLoading, setWorkflowLoading\] = useState\(false\)/)
  assert.match(source, /<ModuleLoading variant="list" count=\{3\} label="发布记录加载中" \/>/)
  assert.match(source, /placeholder=\{capabilitiesLoading \? "模型加载中" : llmCapability\?\.available \? "选择对话模型" : "模型未就绪"\}/)
  assert.doesNotMatch(source, /const \[agentData, agentPage, releaseData, capabilityResponse, templatePage\] = await Promise\.all/)
})

test("agent workbench trims explanatory release copy", () => {
  assert.match(source, /"候选版本已生成"/)
  assert.match(source, /"工作流已保存"/)
  assert.match(source, /confirmText: "确认部署"/)
  assert.match(source, /confirmText: "确认回滚"/)
  assert.doesNotMatch(source, /候选版本已生成，线上不变/)
  assert.doesNotMatch(source, /上线该版本，现有版本转为历史。/)
  assert.doesNotMatch(source, /启用该版本，现有版本转为历史。/)
  assert.doesNotMatch(source, /工作流已保存到草稿，线上不变/)
  assert.doesNotMatch(source, /notice="保存草稿；部署后生效。"/)
  assert.doesNotMatch(source, /固定工作流、模型、知识和指纹。/)
  assert.doesNotMatch(source, /退回原因会进入发布记录/)
  assert.doesNotMatch(source, /正在读取机器人配置、发布版本和工作流绑定/)
  assert.doesNotMatch(source, /机器人选项加载失败|机器人配置|机器人不存在|返回机器人资产|搜索机器人/)
  assert.doesNotMatch(source, /自动接待模板不会转人工/)
  assert.doesNotMatch(source, /用户端会一次性切到新配置/)
  assert.doesNotMatch(source, /当前生产版本不受影响/)
  assert.doesNotMatch(source, /完成机器人配置和工作流发布后生成第一个候选版本/)
  assert.doesNotMatch(source, /平台模型能力|模型能力未就绪/)
})
