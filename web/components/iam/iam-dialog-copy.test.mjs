import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const iamFiles = [
  "iam-invite-dialog.tsx",
  "iam-platform-staff-dialog.tsx",
  "iam-role-dialog.tsx",
  "iam-enterprise-edit-dialog.tsx",
]

async function readSource(path) {
  return readFile(new URL(path, import.meta.url), "utf8")
}

test("IAM management dialogs avoid repeated header descriptions", async () => {
  const sources = await Promise.all(iamFiles.map((file) => readSource(file)))
  for (const source of sources) {
    assert.doesNotMatch(source, /DialogDescription/)
    assert.doesNotMatch(source, /dialogCopy\.description/)
    assert.doesNotMatch(source, /roleDescription/)
  }
  assert.doesNotMatch(sources.join("\n"), /targetDescription\(/)
})

test("IAM detail sheets avoid description components", async () => {
  const source = await readSource("iam-detail-sheet.tsx")

  assert.doesNotMatch(source, /SheetDescription/)
  assert.doesNotMatch(source, /\{role\.description \|\| "暂无说明"\}/)
  assert.match(source, /业务审计 #/)
  assert.match(source, /认证审计 #/)
})

test("legacy dashboard create drawers avoid repeated header descriptions", async () => {
  const userCreate = await readSource("../../app/dashboard/users/_components/create.tsx")
  const userEdit = await readSource("../../app/dashboard/users/_components/edit.tsx")
  const roleCreate = await readSource("../../app/dashboard/roles/_components/create.tsx")
  const roleAssign = await readSource("../../app/dashboard/roles/_components/assign-permissions.tsx")

  assert.doesNotMatch(userCreate, /DrawerDescription/)
  assert.doesNotMatch(userCreate, /user\.createDescription/)
  assert.doesNotMatch(userEdit, /DrawerDescription/)
  assert.doesNotMatch(roleCreate, /DrawerDescription/)
  assert.doesNotMatch(roleCreate, /role\.createDescription/)
  assert.doesNotMatch(roleAssign, /DrawerDescription/)
})

test("unused management description locale keys are removed", async () => {
  for (const locale of ["zh-CN", "en-US", "es-ES"]) {
    const messages = JSON.parse(await readSource(`../../messages/${locale}.json`))
    assert.equal(Object.hasOwn(messages.user, "createDescription"), false, `${locale} should not keep user createDescription`)
    assert.equal(Object.hasOwn(messages.role, "customDescription"), false, `${locale} should not keep role customDescription`)
    assert.equal(Object.hasOwn(messages.role, "customDomainPlatform"), false, `${locale} should not keep role customDomainPlatform`)
    assert.equal(Object.hasOwn(messages.role, "customDomainEnterprise"), false, `${locale} should not keep role customDomainEnterprise`)
    assert.equal(Object.hasOwn(messages.role, "createDescription"), false, `${locale} should not keep role createDescription`)
  }
})
