import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const listSource = await readFile(new URL("./page.tsx", import.meta.url), "utf8")
const detailSource = await readFile(new URL("./[workflowId]/template-page.tsx", import.meta.url), "utf8")
const utilsSource = await readFile(new URL("./[workflowId]/workflow-product-utils.ts", import.meta.url), "utf8")
const messagesSource = await readFile(new URL("../../../i18n/extracted/workflow.ts", import.meta.url), "utf8")

test("knowledge-support workflows use tenant knowledge and reception labels", () => {
  assert.match(utilsSource, /isKnowledgeSupportWorkflowDefinition/)
  assert.match(utilsSource, /journeyStages\.tenantKnowledge/)
  assert.match(utilsSource, /capabilities\.tenantKnowledge\.title/)
  assert.match(utilsSource, /serviceModes\.knowledgeHuman/)
  assert.match(detailSource, /releaseSettings\.enabledReception/)
  assert.match(detailSource, /orchestration\.knowledgeStageIntent\.receive/)
  assert.match(detailSource, /nodeLabels\.types\.tenant_knowledge_retrieve/)
  assert.match(detailSource, /runtime\.byTenant/)
  assert.match(listSource, /createIntro\.knowledgeDescription/)
  assert.match(listSource, /launch\.enableReception/)
})

test("knowledge-support workflow labels exist in every supported locale", () => {
  for (const key of [
    "knowledgeSupport",
    "enableReception",
    "tenantKnowledgeTitle",
    "tenant_knowledge_retrieve",
    "configurationsToUpgrade",
    "enabledReception",
    "knowledgeStageIntent",
    "knowledgeHuman",
    "tenantKnowledgeBasis",
    "serviceConfig",
  ]) {
    assert.equal((messagesSource.match(new RegExp(`\"${key}\"`, "g")) ?? []).length, 3, `${key} should exist in three locales`)
  }
})
