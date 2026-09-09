import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./page.tsx", import.meta.url), "utf8")
const locales = ["zh-CN", "en-US", "es-ES"]
const messagesByLocale = await Promise.all(
  locales.map(async (locale) => [
    locale,
    JSON.parse(await readFile(new URL(`../../../messages/${locale}.json`, import.meta.url), "utf8")),
  ]),
)

test("tenant default knowledge base is a first-class scope above products", () => {
  assert.match(source, /const tenantDefaultKnowledgeBaseRemark = "builtin-tenant-default-service"/)
  assert.match(source, /<section[\s\S]{0,180}className="kb-knowledge-layout rhd-railops-knowledge-workspace"[\s\S]{0,180}gridTemplateColumns/)
  assert.match(source, /knowledgeBase\.remark === tenantDefaultKnowledgeBaseRemark/)
  assert.match(source, /buildProductTreeGroups\(filteredProducts\)/)
  assert.match(source, /<ProductTree/)
  assert.match(source, /title=\{t\("enterpriseKnowledge\.productTreeTitle"\)\}/)
  assert.match(source, /label: t\("enterpriseKnowledge\.tenantPanel\.title"\)/)
  assert.match(source, /rootLabel=\{t\("enterpriseKnowledge\.productTreeRootLabel"\)\}/)
  assert.match(source, /rootSelected=\{false\}/)
  assert.doesNotMatch(source, /rhd-railops-knowledge-product-chip/)
  assert.doesNotMatch(source, /onSelectRoot=\{handleSelectTenantKnowledge\}/)
})

test("tenant knowledge creation immediately binds the default agent", () => {
  assert.match(source, /createKnowledgeBase\(buildKnowledgeBasePayload\(name, description\)\)/)
  assert.match(source, /await ensureTenantDefaultAIAgent\(\)/)
})

test("tenant default knowledge base supports document upload and management", () => {
  assert.match(source, /fetchTenantKnowledgeDocuments\(knowledgeBaseId,\s*\{[^}]*page_size: PER_PAGE[^}]*\}\)/)
  assert.match(source, /uploadTenantKnowledgeDocument\(tenantKnowledgeBaseID!, file\)/)
  assert.match(source, /deleteTenantKnowledgeDocument\(tenantKnowledgeBaseID!, item\.id\)/)
  assert.match(source, /reprocessTenantKnowledgeDocument\(tenantKnowledgeBaseID!, item\.id\)/)
  assert.match(source, /useConfirm\(\)/)
  assert.match(source, /getKnowledgeDocumentDeleteTitle\(t, item\)/)
  assert.match(source, /getKnowledgeDocumentDeleteDescription\(t, item\)/)
  assert.match(source, /showEntries=\{false\}/)
  assert.match(source, /emptyText=\{t\("enterpriseKnowledge\.rightPanel\.tenantEmptyText"\)\}/)
  assert.match(source, /uploadEnabled && hasKnowledgeMounted\(state\)/)
  assert.doesNotMatch(source, /productId && hasKnowledgeMounted\(state\)/)
  assert.match(source, /tenantKnowledgeSelected && tenantKnowledgeBaseID \? \(\s*<KnowledgeRightPanel/)
  assert.match(source, /className="ent-panel-head kb-tab-panel-head rhd-railops-knowledge-panel-head"/)
  assert.doesNotMatch(source, /ent-product-editor space-y-4/)
})

test("knowledge content column uses stable title and filename text styles", () => {
  assert.match(source, /const contentTitle = item\.title \|\| item\.filename \|\| t\("enterpriseKnowledge\.documentFallback", \{ id: item\.id \}\)/)
  assert.match(source, /const showFilename = Boolean\(item\.filename && item\.filename !== contentTitle\)/)
  assert.match(source, /<AntTable<ProductKnowledgeDocumentFile>/)
  assert.match(source, /className="rhd-railops-table-cell"/)
  assert.match(source, /<strong title=\{contentTitle\}>\{contentTitle\}<\/strong>/)
  assert.match(source, /<StatusTag tone=\{getKnowledgeDocumentSourceTone\(item\.source_type\)\}>/)
  assert.doesNotMatch(source, /className="kb-doc-content-title"/)
  assert.doesNotMatch(source, /className="text-xs font-normal text-muted-foreground">\{item\.filename\}/)
})

test("knowledge page loads every product page through the shared API helper", () => {
  assert.match(source, /listAllProducts\(\)/)
  assert.doesNotMatch(source, /listProducts\(\{ page_size: 200 \}\)/)
})

test("enterprise knowledge page avoids explanatory empty and callout copy", () => {
  assert.doesNotMatch(source, /DialogDescription/)
  assert.doesNotMatch(source, /enterpriseKnowledge\.pageEyebrow/)
  assert.doesNotMatch(source, /enterpriseKnowledge\.productPanel\.emptyDescription/)
  assert.doesNotMatch(source, /enterpriseKnowledge\.productPanel\.calloutDescription/)
  assert.doesNotMatch(source, /enterpriseKnowledge\.productPanel\.calloutLead/)
  assert.doesNotMatch(source, /enterpriseKnowledge\.tenantPanel\.dataset(Ready|Empty)Description/)
  assert.doesNotMatch(source, /enterpriseKnowledge\.tenantPanel\.readyBanner/)
  assert.doesNotMatch(source, /<EmptyState[^>]+description=\{t\("enterpriseKnowledge\.productPanel\.emptyDescription"\)\}/)
  assert.doesNotMatch(source, /保存 .*服务团队从这里查/)
  assert.doesNotMatch(source, /保存企业通用的服务政策/)

  for (const [locale, messages] of messagesByLocale) {
    const enterpriseKnowledge = messages.enterpriseKnowledge
    assert.ok(enterpriseKnowledge, `${locale} should define enterpriseKnowledge copy`)
    assert.equal(Object.hasOwn(enterpriseKnowledge, "pageEyebrow"), false, `${locale} should not keep enterpriseKnowledge.pageEyebrow`)
    assert.equal(Object.hasOwn(enterpriseKnowledge.productPanel, "emptyDescription"), false, `${locale} should not keep product panel empty description`)
    assert.equal(Object.hasOwn(enterpriseKnowledge.productPanel, "calloutDescription"), false, `${locale} should not keep product panel callout description`)
    assert.equal(Object.hasOwn(enterpriseKnowledge.productPanel, "calloutLead"), false, `${locale} should not keep product panel callout lead`)
    assert.equal(Object.hasOwn(enterpriseKnowledge.tenantPanel, "datasetReadyDescription"), false, `${locale} should not keep tenant ready description`)
    assert.equal(Object.hasOwn(enterpriseKnowledge.tenantPanel, "datasetEmptyDescription"), false, `${locale} should not keep tenant empty description`)
    assert.equal(Object.hasOwn(enterpriseKnowledge.tenantPanel, "readyBanner"), false, `${locale} should not keep tenant ready banner`)
    assert.equal(Object.hasOwn(enterpriseKnowledge.tenantPanel, "formDescriptionDefault"), false, `${locale} should not keep tenant form description defaults`)
    assert.equal(Object.hasOwn(enterpriseKnowledge.productPanel, "formDescriptionDefault"), false, `${locale} should not keep product form description defaults`)
    assert.ok(
      ["选填", "Optional"].includes(enterpriseKnowledge.tenantPanel.formDescriptionPlaceholder),
      `${locale} tenant form placeholder should stay short`,
    )
    assert.ok(
      ["选填", "Optional"].includes(enterpriseKnowledge.productPanel.formDescriptionPlaceholder),
      `${locale} product form placeholder should stay short`,
    )
    assert.ok(
      ["恢复默认", "Reset defaults"].includes(enterpriseKnowledge.tenantPanel.resetButton),
      `${locale} tenant reset copy should avoid recommendations`,
    )
    assert.ok(
      ["恢复默认", "Reset defaults"].includes(enterpriseKnowledge.productPanel.resetButton),
      `${locale} product reset copy should avoid recommendations`,
    )
  }
})
