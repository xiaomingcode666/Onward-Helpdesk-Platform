import assert from "node:assert/strict"
import test from "node:test"
import ts from "typescript"
import { readFile } from "node:fs/promises"
import vm from "node:vm"

async function loadModule() {
  const source = await readFile(new URL("./audit-i18n.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "audit-i18n.ts",
  })
  const sandbox = {
    exports: {},
    module: { exports: {} },
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

test("localizes IAM audit action codes for business-facing pages", async () => {
  const { getAuditActionLabel } = await loadModule()

  assert.equal(getAuditActionLabel("customer_user.authorized"), "客户联系人已授权")
  assert.equal(getAuditActionLabel("customer_user.support_session_started"), "客户门户代访已开始")
  assert.equal(getAuditActionLabel("tenant.support_session_started"), "平台协助访问已开始")
  assert.equal(getAuditActionLabel("tenant_member.invited"), "企业成员已邀请")
  assert.equal(getAuditActionLabel("tenant_member.support_session_started"), "员工门户代登录已开始")
  assert.equal(getAuditActionLabel("partner_admin.invited"), "供应商管理员已邀请")
  assert.equal(getAuditActionLabel("tenant.created"), "租户已创建")
  assert.equal(getAuditActionLabel("auth_policy.saved"), "权限策略已保存")
  assert.equal(getAuditActionLabel("tenant_administrator.password_reset"), "企业管理员密码已重置")
  assert.equal(getAuditActionLabel("department.created"), "部门已创建")
  assert.equal(getAuditActionLabel("platform_integration.connection_tested"), "外部服务连接已测试")
  assert.equal(getAuditActionLabel("speech_runtime.updated"), "语音服务配置已更新")
  assert.equal(getAuditActionLabel("ai_agent_release.rolled_back"), "接待配置上线包已回滚")
})

test("localizes audit labels to Spanish", async () => {
  const {
    getAuditActionLabel,
    getAuditRiskLabel,
    getAuditStatusLabel,
    getAuditSubjectLabel,
    getAuditTargetLabel,
  } = await loadModule()

  assert.equal(getAuditActionLabel("tenant.created", "es-ES"), "Tenant creado")
  assert.equal(getAuditActionLabel("tenant.updated", "es-ES"), "Tenant actualizado")
  assert.equal(getAuditActionLabel("customer_user.authorized", "es-ES"), "Contacto de cliente autorizado")
  assert.equal(getAuditTargetLabel("customer_user", "es-ES"), "Contacto de cliente")
  assert.equal(getAuditTargetLabel("platform_integration", "es-ES"), "Servicio externo")
  assert.equal(getAuditSubjectLabel("platform_staff", "es-ES"), "Platform staff")
  assert.equal(getAuditRiskLabel("critical", "es-ES"), "Critico")
  assert.equal(getAuditRiskLabel("medium", "es-ES"), "Riesgo medio")
  assert.equal(getAuditStatusLabel("success", "es-ES"), "Correcto")
  assert.equal(getAuditStatusLabel("blocked", "es-ES"), "Bloqueado")
})

test("localizes audit risk, status, actor and target labels", async () => {
  const {
    getAuditRiskLabel,
    getAuditStatusLabel,
    getAuditSubjectLabel,
    getAuditTargetLabel,
  } = await loadModule()

  assert.equal(getAuditRiskLabel("critical"), "严重")
  assert.equal(getAuditRiskLabel("high"), "高风险")
  assert.equal(getAuditStatusLabel("success"), "成功")
  assert.equal(getAuditStatusLabel("blocked"), "已阻断")
  assert.equal(getAuditSubjectLabel("platform_staff"), "平台人员")
  assert.equal(getAuditTargetLabel("customer_user"), "客户联系人")
  assert.equal(getAuditTargetLabel("platform_integration"), "外部服务")
  assert.equal(getAuditTargetLabel("speech_runtime"), "语音服务配置")
})

test("keeps successful IAM audit records out of alert events", async () => {
  const { isAuditAlertEvent } = await loadModule()

  assert.equal(isAuditAlertEvent({
    action: "tenant_member.invited",
    riskLevel: "high",
    status: "success",
  }), false)
  assert.equal(isAuditAlertEvent({
    action: "auth_policy.saved",
    riskLevel: "high",
    status: "success",
  }), false)
  assert.equal(isAuditAlertEvent({
    action: "customer_user.authorized",
    riskLevel: "high",
    status: "failure",
  }), true)
  assert.equal(isAuditAlertEvent({
    action: "security.alert",
    riskLevel: "low",
    status: "success",
  }), true)
})

test("normalizes legacy successful routine IAM audit risk for display", async () => {
  const { getAuditDisplayRiskLevel } = await loadModule()

  assert.equal(getAuditDisplayRiskLevel({
    action: "tenant_member.invited",
    riskLevel: "high",
    status: "success",
  }), "medium")
  assert.equal(getAuditDisplayRiskLevel({
    action: "auth_policy.saved",
    riskLevel: "high",
    status: "success",
  }), "medium")
  assert.equal(getAuditDisplayRiskLevel({
    action: "platform_staff.tenant_granted",
    riskLevel: "critical",
    status: "success",
  }), "critical")
  assert.equal(getAuditDisplayRiskLevel({
    action: "tenant_member.invited",
    riskLevel: "high",
    status: "failure",
  }), "high")
})
