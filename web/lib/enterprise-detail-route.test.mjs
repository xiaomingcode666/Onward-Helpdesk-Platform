import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import ts from "typescript"
import vm from "node:vm"

async function loadModule() {
  const source = await readFile(new URL("./enterprise-detail-route.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2017,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "enterprise-detail-route.ts",
  })
  const sandbox = { exports: {}, module: { exports: {} }, URLSearchParams }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

test("enterprise detail links use static-export-safe query routes", async () => {
  const {
    buildEnterpriseAIPath,
    buildEnterpriseProductPath,
    buildEnterpriseWorkflowPath,
  } = await loadModule()

  assert.equal(buildEnterpriseWorkflowPath(8), "/enterprise/workflow?workflowId=8")
  assert.equal(buildEnterpriseAIPath("12", "view=releases"), "/enterprise/ai?view=releases&agentId=12")
  assert.equal(
    buildEnterpriseProductPath(31, new URLSearchParams("tab=repairHistory")),
    "/enterprise/products?tab=repairHistory&product_id=31",
  )
})

test("enterprise detail links reject route labels and invalid identifiers", async () => {
  const {
    buildEnterpriseAIPath,
    buildEnterpriseProductPath,
    buildEnterpriseWorkflowPath,
    normalizeEnterpriseDetailId,
  } = await loadModule()

  assert.equal(normalizeEnterpriseDetailId("enterprise"), 0)
  assert.equal(normalizeEnterpriseDetailId("8.5"), 0)
  assert.equal(buildEnterpriseWorkflowPath("enterprise"), "/enterprise/workflow")
  assert.equal(buildEnterpriseAIPath(0), "/enterprise/ai")
  assert.equal(buildEnterpriseProductPath(-1), "/enterprise/products")
})

test("enterprise portal sources no longer navigate to runtime dynamic detail segments", async () => {
  const sources = await Promise.all([
    readFile(new URL("../app/enterprise/workflow/page.tsx", import.meta.url), "utf8"),
    readFile(new URL("../app/enterprise/workflow/[workflowId]/template-page.tsx", import.meta.url), "utf8"),
    readFile(new URL("../app/enterprise/ai/ai-service-center.tsx", import.meta.url), "utf8"),
    readFile(new URL("../app/enterprise/ai/[agentId]/workbench-page.tsx", import.meta.url), "utf8"),
    readFile(new URL("../app/enterprise/_components/service-workbench-page.tsx", import.meta.url), "utf8"),
  ])

  const dynamicDetailSegment = /`\/enterprise\/(?:workflow|ai|products)\/\$\{/
  for (const source of sources) assert.doesNotMatch(source, dynamicDetailSegment)
})
