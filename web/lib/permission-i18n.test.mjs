import assert from "node:assert/strict"
import test from "node:test"
import ts from "typescript"
import { readFile } from "node:fs/promises"
import vm from "node:vm"

async function loadModule() {
  const source = await readFile(new URL("./permission-i18n.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2017,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "permission-i18n.ts",
  })
  const sandbox = {
    exports: {},
    module: { exports: {} },
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

test("localizes seeded permission display names to English", async () => {
  const { getPermissionDisplayName, getPermissionGroupName } = await loadModule()

  assert.equal(getPermissionDisplayName("user.assignRole", "\u5206\u914d\u7528\u6237\u89d2\u8272", "en-US"), "Assign employee roles")
  assert.equal(getPermissionDisplayName("agentTeamSchedule.update", "\u66f4\u65b0\u5ba2\u670d\u7ec4\u6392\u73ed", "en-US"), "Update engineer schedules")
  assert.equal(getPermissionGroupName("agentTeamSchedule", "en-US"), "Engineer schedules")
})

test("localizes permission names and groups to Chinese", async () => {
  const { getPermissionDisplayName, getPermissionGroupName } = await loadModule()

  assert.equal(getPermissionDisplayName("user.view", "\u67e5\u770b\u7528\u6237", "zh-CN"), "\u67e5\u770b\u5458\u5de5\u8d26\u53f7")
  assert.equal(getPermissionDisplayName("role.delete", "Delete roles", "zh-CN"), "\u5220\u9664\u89d2\u8272")
  assert.equal(getPermissionDisplayName("role.update", "Update roles", "zh-CN"), "\u66f4\u65b0\u89d2\u8272")
  assert.equal(getPermissionDisplayName("notification.channel.manage", "Manage notification channels", "zh-CN"), "\u7ba1\u7406\u6d88\u606f\u901a\u77e5\u6e20\u9053")
  assert.equal(getPermissionGroupName("agentTeam", "zh-CN"), "\u5de5\u7a0b\u5e08\u56e2\u961f")
})

test("localizes permission names and groups to Spanish", async () => {
  const { getPermissionDisplayName, getPermissionGroupName } = await loadModule()

  assert.equal(getPermissionDisplayName("user.assignRole", "Asignar roles de usuarios", "es-ES"), "Asignar roles de empleados")
  assert.equal(getPermissionDisplayName("tenant.delete", "Retirar tenants", "es-ES"), "Retirar tenants")
  assert.equal(getPermissionDisplayName("user.view", "Ver usuarios", "es-ES"), "Ver cuentas de empleado")
  assert.equal(getPermissionDisplayName("role.create", "Crear roles", "es-ES"), "Crear roles")
  assert.equal(getPermissionGroupName("agentTeamSchedule", "es-ES"), "Horarios de ingenieros")
  assert.equal(getPermissionGroupName("partnerMember", "es-ES"), "Cuentas de proveedor")
})

test("localizes enterprise custom-role permissions from seeded catalog names", async () => {
  const { getPermissionDisplayName, getPermissionGroupName } = await loadModule()

  assert.equal(getPermissionDisplayName("customerMember.update", "Update customer members", "zh-CN"), "\u66f4\u65b0\u5ba2\u6237\u8d26\u53f7")
  assert.equal(getPermissionDisplayName("partnerMember.view", "View partner members", "zh-CN"), "\u67e5\u770b\u4f9b\u5e94\u5546\u8d26\u53f7")
  assert.equal(getPermissionDisplayName("agentTeamSchedule.batchGenerate", "Batch generate agent team schedules", "zh-CN"), "\u6279\u91cf\u751f\u6210\u5de5\u7a0b\u5e08\u6392\u73ed")
  assert.equal(getPermissionDisplayName("aiAgent.update", "Update AI Agents", "zh-CN"), "\u66f4\u65b0\u63a5\u5f85\u914d\u7f6e")
  assert.equal(getPermissionDisplayName("skillDefinition.delete", "Delete Skill definitions", "en-US"), "Delete skill definitions")
  assert.equal(getPermissionDisplayName("aiAgent.update", "Update AI Agents", "en-US"), "Update reception configurations")
  assert.equal(getPermissionDisplayName("tenant.delete", "Delete tenants", "en-US"), "Decommission tenants")
  assert.equal(getPermissionGroupName("partnerMember", "en-US"), "Supplier accounts")
})
