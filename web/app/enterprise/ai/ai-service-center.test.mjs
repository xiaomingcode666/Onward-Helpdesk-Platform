import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./ai-service-center.tsx", import.meta.url), "utf8")
const workbenchSource = await readFile(new URL("./[agentId]/workbench-page.tsx", import.meta.url), "utf8")

test("model statuses only render capabilities returned by the backend", () => {
  assert.match(source, /capability:\s*EnterpriseAICapability/)
  assert.match(source, /const hasCapabilities = Boolean\(llmCapability \|\| embeddingCapability\)/)
  assert.match(source, /\{capabilitiesLoading \? \(/)
  assert.match(source, /: hasCapabilities \? \(/)
  assert.match(source, /\{llmCapability \? \(/)
  assert.match(source, /\{embeddingCapability \? <CapabilityStatus/)
  assert.equal(source.match(/<CapabilityStatus capability=\{embeddingCapability\}/g)?.length, 1)
})

test("model settings are a right-side gear popover instead of a full-width row", () => {
  assert.match(source, /SettingsIcon/)
  assert.match(source, /aria-label="模型设置"/)
  assert.match(source, /<PopoverContent align="end"/)
  assert.match(source, /<ModelSettingsPopover/)
  assert.doesNotMatch(source, /aria-label="机器人资产摘要"/)
  assert.doesNotMatch(source, /aria-label="机器人状态"/)
  assert.doesNotMatch(source, /<details className="group overflow-hidden rounded-md border bg-card">/)
})

test("tenant default reception is a first-class scope beside product agents", () => {
  assert.match(source, /type AgentScopeSelection = "tenant" \| "all" \| number/)
  assert.match(source, /<section className="rhd-railops-ai-workspace">/)
  assert.match(source, /<ProductTree/)
  assert.match(source, /buildProductTreeGroups\(filteredProducts\)/)
  assert.match(source, /onSelectProduct=\{\(product\) => setSelectedScope\(product\.id\)\}/)
  assert.match(source, /source:\s*selectedScope === "tenant" \? "tenant_default" : undefined/)
  assert.match(source, /label: "企业默认接待"/)
  assert.match(source, /aiServiceCenter\.text054/)
  assert.match(source, /isTenantDefault \? ee\(hasProductConcept \? "aiServiceCenter\.text061" : "aiServiceCenter\.text085"\) : handoffModeSummary\(agent\)/)
  assert.match(source, /isTenantDefault \? ee\(hasProductConcept \? "aiServiceCenter\.text062" : "aiServiceCenter\.text086"\) : handoffTeamSummary\(agent\)/)
  assert.match(source, /fetchEnterpriseAIAgentSummary/)
  assert.match(source, /fetchEnterpriseAIAgents\(query\)/)
  assert.match(source, /productScoped:\s*selectedScope === "all" \? 0 : undefined/)
  assert.doesNotMatch(source, /fetchAIAgents\(\{ page: 1, limit: 1000 \}\)/)
  assert.match(source, /listAllProducts\(\)/)
  assert.doesNotMatch(source, /page_size: 500/)
  assert.doesNotMatch(source, /className="grid gap-1\.5 xl:hidden"/)
  assert.doesNotMatch(source, /className="hidden xl:block"/)
})

test("knowledge support tenants use tenant reception without loading a product scope", () => {
  assert.match(source, /const hasProductConcept = session\?\.featureFlags\?\.product !== false/)
  assert.match(source, /const showProductTree = authReady && hasProductConcept/)
  assert.match(source, /if \(!hasProductConcept\) \{[\s\S]*?setSelectedScope\("tenant"\)[\s\S]*?return/)
  assert.match(source, /style=\{showProductTree \? undefined : \{ gridTemplateColumns: "minmax\(0, 1fr\)" \}\}/)
  assert.match(source, /\{showProductTree \? \([\s\S]*?<ProductTree/)
})

test("tenant default reception uses knowledge-support wording when product capability is disabled", () => {
  assert.match(source, /hasProductConcept \? "aiServiceCenter\.text061" : "aiServiceCenter\.text085"/)
  assert.match(source, /hasProductConcept \? "aiServiceCenter\.text062" : "aiServiceCenter\.text086"/)
  assert.match(workbenchSource, /ee\(hasProductConcept \? "aiWorkbench\.text074" : "aiWorkbench\.text125"\)/)
  assert.match(workbenchSource, /ee\(hasProductConcept \? "aiWorkbench\.text078" : "aiWorkbench\.text126"\)/)
  assert.doesNotMatch(workbenchSource, /平台统一 AI 能力|仅 AI、不转人工|仅 AI 模板/)
})

test("service center uses AntD table loading and pagination", () => {
  assert.match(source, /summaryLoading/)
  assert.match(source, /productsLoading/)
  assert.match(source, /agentsLoading/)
  assert.match(source, /capabilitiesLoading/)
  assert.match(source, /<AntTable<AIAgent>/)
  assert.match(source, /<ModuleLoading label="模型设置加载中"/)
  assert.match(source, /<AntPagination/)
  assert.match(source, /productAgentProductIds/)
  assert.match(source, /售后接待配置/)
  assert.doesNotMatch(source, /<ListPagination/)
  assert.doesNotMatch(source, /<ModuleLoading label="接待配置加载中"/)
  assert.doesNotMatch(source, /正在加载产品机器人/)
  assert.doesNotMatch(source, /产品机器人|搜索机器人|刷新机器人|机器人列表加载中/)
  assert.doesNotMatch(source, /loadingSummary/)
})

test("agent detail uses a single header model selector instead of a duplicate model card", () => {
  assert.match(workbenchSource, /placeholder=\{capabilitiesLoading \? "模型加载中" : llmCapability\?\.available \? "选择对话模型" : "模型未就绪"\}/)
  assert.match(workbenchSource, /当前路由 ·/)
  assert.match(workbenchSource, /updateAgentModelDraft/)
  assert.doesNotMatch(workbenchSource, /保存模型草稿/)
  assert.doesNotMatch(workbenchSource, /<div className="font-medium">模型设置<\/div>/)
})

test("agent detail keeps release and binding copy terse", () => {
  assert.match(workbenchSource, /"候选版本已生成"/)
  assert.match(workbenchSource, /"工作流已保存"/)
  assert.match(workbenchSource, /confirmText: "确认部署"/)
  assert.match(workbenchSource, /confirmText: "确认回滚"/)
  assert.doesNotMatch(workbenchSource, /上线该版本，现有版本转为历史。/)
  assert.doesNotMatch(workbenchSource, /启用该版本，现有版本转为历史。/)
  assert.doesNotMatch(workbenchSource, /候选版本已生成，线上不变/)
  assert.doesNotMatch(workbenchSource, /工作流已保存到草稿，线上不变/)
  assert.doesNotMatch(workbenchSource, /notice="保存草稿；部署后生效。"/)
  assert.doesNotMatch(workbenchSource, /模型草稿可生成候选版本。|固定工作流、模型、知识和指纹。/)
})

test("tenant default agent exposes key readiness without exposing key material", () => {
  assert.match(source, /ensureTenantDefaultAIAgent/)
  assert.doesNotMatch(source, /defaultKeyCiphertext|defaultKeyFingerprint/)
  assert.match(source, /!summary\.tenantDefaultReady \|\| !defaultCredential\?\.ready/)
  assert.doesNotMatch(source, /defaultCredential\?\.keyName/)
})
