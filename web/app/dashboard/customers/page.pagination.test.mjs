import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const customersSource = await readFile(new URL("./page.tsx", import.meta.url), "utf8")
const companyApiSource = await readFile(new URL("../../../lib/api/company.ts", import.meta.url), "utf8")

test("customer company filter options are loaded by small pages", () => {
  assert.match(customersSource, /fetchCompanyCatalog\(\{ status: 0 \}\)/)
  assert.doesNotMatch(customersSource, /fetchCompanies\(\{ status: 0, page: 1, limit: 500 \}/)
  assert.match(companyApiSource, /COMPANY_CATALOG_PAGE_SIZE = 100/)
  assert.match(companyApiSource, /export async function fetchCompanyCatalog/)
  assert.match(companyApiSource, /results\.length >= total/)
})
