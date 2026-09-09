import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const rolesSource = await readFile(new URL("./page.tsx", import.meta.url), "utf8")
const adminApiSource = await readFile(new URL("../../../lib/api/admin.ts", import.meta.url), "utf8")
const iamWorkspaceSource = await readFile(new URL("../../../components/iam/iam-workspace-page.tsx", import.meta.url), "utf8")

test("role permission catalogs are loaded by small pages", () => {
  assert.match(rolesSource, /fetchPermissionCatalog\(\)/)
  assert.match(iamWorkspaceSource, /fetchPermissionCatalog\(\{ status: 0 \}\)/)
  assert.doesNotMatch(`${rolesSource}\n${iamWorkspaceSource}`, /fetchPermissions\(\{[^}]*limit:\s*500/)
  assert.match(adminApiSource, /PERMISSION_CATALOG_PAGE_SIZE = 100/)
  assert.match(adminApiSource, /export async function fetchPermissionCatalog/)
  assert.match(adminApiSource, /results\.length >= total/)
})
