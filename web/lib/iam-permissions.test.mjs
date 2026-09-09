import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import ts from "typescript"
import vm from "node:vm"

async function loadIAMPermissionsModule() {
  const source = await readFile(new URL("./iam-permissions.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2017,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "iam-permissions.ts",
  })
  const i18nSource = await readFile(new URL("./permission-i18n.ts", import.meta.url), "utf8")
  const i18nCompiled = ts.transpileModule(i18nSource, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2017,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "permission-i18n.ts",
  })
  const i18nSandbox = { exports: {}, module: { exports: {} } }
  i18nSandbox.exports = i18nSandbox.module.exports
  vm.runInNewContext(i18nCompiled.outputText, i18nSandbox)
  const sandbox = { exports: {}, module: { exports: {} } }
  sandbox.exports = sandbox.module.exports
  sandbox.require = (specifier) => {
    if (specifier === "@/lib/permission-i18n") return i18nSandbox.module.exports
    throw new Error(`Unexpected import: ${specifier}`)
  }
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

test("platform compatibility permissions use menu-aligned Chinese labels", async () => {
  const { groupIAMPermissions } = await loadIAMPermissionsModule()
  const groups = groupIAMPermissions(
    ["platform.audit.view", "platform.staff.manage", "platform.tenant.view"],
    [],
  )

  assert.equal(groups.length, 1)
  assert.equal(groups[0].label, "平台端")
  assert.deepEqual(
    Array.from(groups[0].permissions, ({ code, name }) => ({ code, name })),
    [
      { code: "platform.staff.manage", name: "平台人员" },
      { code: "platform.tenant.view", name: "租户管理" },
      { code: "platform.audit.view", name: "审计日志" },
    ],
  )
})

test("permission groups follow the supplied menu order", async () => {
  const { groupIAMPermissions } = await loadIAMPermissionsModule()
  const groups = groupIAMPermissions(
    ["tenant.view", "session.view", "user.view", "role.view"],
    [],
    ["user", "tenant", "role", "session"],
  )

  assert.deepEqual(
    Array.from(groups, ({ key }) => key),
    ["user", "tenant", "role", "session"],
  )
})

test("permission catalog names remain the fallback for regular permissions", async () => {
  const { groupIAMPermissions } = await loadIAMPermissionsModule()
  const [group] = groupIAMPermissions(
    ["ticket.view"],
    [{
      apiPath: "/api/enterprise/tickets/list",
      code: "ticket.view",
      groupName: "ticket",
      method: "GET",
      name: "查看工单",
      type: "api",
    }],
  )

  assert.equal(group.label, "工单与维修")
  assert.equal(group.permissions[0].name, "查看工单")
})

test("English catalog names are localized for the Chinese IAM workspace", async () => {
  const { groupIAMPermissions } = await loadIAMPermissionsModule()
  const [group] = groupIAMPermissions(
    ["role.create", "role.delete", "role.update", "role.view"],
    [
      ["role.create", "Create roles"],
      ["role.delete", "Delete roles"],
      ["role.update", "Update roles"],
      ["role.view", "View roles"],
    ].map(([code, name]) => ({ apiPath: "", code, groupName: "role", method: "POST", name, type: "api" })),
  )

  assert.deepEqual(
    Array.from(group.permissions, ({ name }) => name),
    ["创建角色", "删除角色", "更新角色", "查看角色"],
  )
})

test("legacy privacy resource groups also use Chinese labels", async () => {
  const { getIAMPermissionGroupLabel } = await loadIAMPermissionsModule()

  assert.equal(getIAMPermissionGroupLabel("privacyRequest"), "隐私请求")
  assert.equal(getIAMPermissionGroupLabel("dataBreach"), "数据泄露事件")
  assert.equal(getIAMPermissionGroupLabel("dataRetention"), "数据保留策略")
})

test("permission groups use Spanish labels for the es-ES locale", async () => {
  const { getIAMPermissionGroupLabel, groupIAMPermissions } = await loadIAMPermissionsModule()

  assert.equal(getIAMPermissionGroupLabel("ticket", "es-ES"), "Tickets y reparaciones")
  assert.equal(getIAMPermissionGroupLabel("privacyRequest", "es-ES"), "Solicitudes de privacidad")
  assert.equal(getIAMPermissionGroupLabel("unknownGroup", "es-ES"), "unknownGroup")

  const [group] = groupIAMPermissions(
    ["role.create"],
    [{
      apiPath: "/api/enterprise/roles/create",
      code: "role.create",
      groupName: "role",
      method: "POST",
      name: "Crear roles",
      type: "api",
    }],
    [],
    "es-ES",
  )
  assert.equal(group.label, "Gestion de roles")
  assert.equal(group.permissions[0].name, "Crear roles")
})
