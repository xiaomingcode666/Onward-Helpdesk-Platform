import assert from "node:assert/strict"
import test from "node:test"
import ts from "typescript"
import { readFile } from "node:fs/promises"
import vm from "node:vm"

async function loadModule() {
  const source = await readFile(new URL("./account-context-i18n.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2017,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "account-context-i18n.ts",
  })
  const sandbox = {
    exports: {},
    module: { exports: {} },
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

test("maps internal enterprise account context to Chinese labels", async () => {
  const {
    getAccountDomainLabel,
    getAccountRoleLabel,
    getAccountSubjectLabel,
  } = await loadModule()

  assert.equal(getAccountDomainLabel("enterprise", "zh-CN"), "企业端")
  assert.equal(getAccountSubjectLabel("enterprise_member", 13, "zh-CN"), "企业成员 #13")
  assert.equal(getAccountRoleLabel("tenant_admin", "zh-CN"), "企业管理员")
  assert.equal(getAccountRoleLabel("tenant_owner", "zh-CN"), "租户所有者")
})

test("maps every built-in portal and role family", async () => {
  const { getAccountDomainLabel, getAccountRoleLabel } = await loadModule()

  assert.deepEqual(
    ["platform", "enterprise", "customer", "partner", "service_account"].map((value) =>
      getAccountDomainLabel(value, "zh-CN")
    ),
    ["平台端", "企业端", "客户端", "供应商端", "系统服务"]
  )
  assert.deepEqual(
    ["platform_admin", "service_engineer", "partner_admin", "customer_admin"].map((value) =>
      getAccountRoleLabel(value, "zh-CN")
    ),
    ["平台管理员", "服务工程师", "供应商管理员", "客户管理员"]
  )
})

test("uses English labels and preserves unknown custom values", async () => {
  const {
    getAccountDomainLabel,
    getAccountRoleLabel,
    getAccountSubjectLabel,
  } = await loadModule()

  assert.equal(getAccountDomainLabel("enterprise", "en-US"), "Enterprise portal")
  assert.equal(getAccountSubjectLabel("partner_account", 8, "en-US"), "Partner account #8")
  assert.equal(getAccountRoleLabel("tenant_admin", "en-US"), "Enterprise administrator")
  assert.equal(getAccountRoleLabel("regional_reviewer", "zh-CN"), "regional_reviewer")
})

test("maps internal enterprise account context to Spanish labels", async () => {
  const {
    getAccountDomainLabel,
    getAccountRoleLabel,
    getAccountSubjectLabel,
  } = await loadModule()

  assert.equal(getAccountDomainLabel("enterprise", "es-ES"), "Portal de empresa")
  assert.equal(getAccountSubjectLabel("partner_account", 8, "es-ES"), "Cuenta de proveedor #8")
  assert.equal(getAccountRoleLabel("tenant_admin", "es-ES"), "Administrador de empresa")
  assert.equal(getAccountRoleLabel("tenant_owner", "es-ES"), "Propietario del tenant")
  assert.equal(getAccountRoleLabel("regional_reviewer", "es-ES"), "regional_reviewer")
})
