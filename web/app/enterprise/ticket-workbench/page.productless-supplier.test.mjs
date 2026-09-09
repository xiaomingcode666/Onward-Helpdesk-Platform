import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./page.tsx", import.meta.url), "utf8")

test("knowledge-support tickets can invite a tenant supplier without product context", () => {
  assert.match(source, /fetchTicketSupplierOptions\(contextTicketId\)/)
  assert.match(source, /const supportsProductlessSupplier = !hasDeviceConcept && productId <= 0/)
  assert.match(source, /partner\.partner_company_id === current/)
  assert.match(source, /\{ partner_company_id: selectedPartnerCompanyId \}/)
  assert.match(source, /\(aggregate\?\.ticket\.product_id \?\? 0\) > 0 \|\| !hasDeviceConcept/)
})

test("product-backed supplier invitation keeps the product-module route", () => {
  assert.match(source, /\{ product_module_id: selectedProductModuleId \}/)
  assert.match(source, /getProductModules\(productId\)/)
})
