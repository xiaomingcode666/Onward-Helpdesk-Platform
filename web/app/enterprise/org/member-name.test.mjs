import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import ts from "typescript"
import vm from "node:vm"

async function loadModule() {
  const source = await readFile(new URL("./member-name.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2017,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "member-name.ts",
  })
  const sandbox = {
    exports: {},
    module: { exports: {} },
    require: (specifier) => {
      if (specifier === "@/i18n/messages") {
        return {
          translateCurrentMessage: (key, values) =>
            key.endsWith("memberNameFallback") ? `成员 #${String(values.id)}` : key,
        }
      }
      throw new Error(`Unexpected require: ${specifier}`)
    },
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

const organizationSource = await readFile(new URL("./product-support-organization-page.tsx", import.meta.url), "utf8")
const membersSource = await readFile(new URL("./members/page.tsx", import.meta.url), "utf8")
const schedulesSource = await readFile(new URL("./schedules/page.tsx", import.meta.url), "utf8")
const optionComboboxSource = await readFile(new URL("../../../components/option-combobox.tsx", import.meta.url), "utf8")
const platformIAMSource = await readFile(new URL("../../../lib/api/platform-iam.ts", import.meta.url), "utf8")

test("enterprise organization prefers the current personnel name over a stale account alias", async () => {
  const { agentMemberName } = await loadModule()
  assert.equal(agentMemberName({
    userId: 25,
    nickname: "旧账号昵称",
    displayName: "人员表当前姓名",
    username: "e2e.product1.engineer",
  }), "人员表当前姓名")
})

test("member-name policy falls back through agent alias and account identity", async () => {
  const { agentMemberName } = await loadModule()
  assert.equal(agentMemberName({ displayName: "派单显示名", username: "engineer" }), "派单显示名")
  assert.equal(agentMemberName({ username: "engineer" }), "engineer")
  assert.equal(agentMemberName({ userId: 25 }), "成员 #25")
})

test("all enterprise organization views use the shared member-name policy", () => {
  assert.match(organizationSource, /agentMemberName/)
  assert.match(membersSource, /agentMemberName/)
  assert.match(schedulesSource, /agentMemberName as memberName/)
})

test("organization leader picker keeps option values unique", () => {
  assert.match(organizationSource, /!members\.some\(\(member\) => member\.userId === selectedTeam\.leaderUserId\)/)
  assert.match(organizationSource, /label: member\.userId === selectedTeam\.leaderUserId/)
})

test("organization page shows member availability as a compact timeline", () => {
  assert.match(organizationSource, /data-testid="team-availability-timeline"/)
  assert.match(organizationSource, /ops\(t, "dutyPanel\.title"\)/)
  assert.match(organizationSource, /ops\(t, "dutyPanel\.hint"\)/)
  assert.match(organizationSource, /todayLeaveStyle/)
  assert.match(organizationSource, /dayWindowStyle/)
  assert.doesNotMatch(organizationSource, /排班草稿/)
  assert.doesNotMatch(membersSource, /七天排班草稿/)
})

test("organization page avoids explanatory helper copy", () => {
  assert.doesNotMatch(organizationSource, /未分配工单优先处理/)
  assert.doesNotMatch(organizationSource, /可以在这里直接分配给组内工程师/)
  assert.doesNotMatch(organizationSource, /创建产品后会自动生成对应的产品维修组/)
  assert.doesNotMatch(organizationSource, /未指定工程师时/)
  assert.doesNotMatch(organizationSource, /按当前会话和工单负载/)
  assert.doesNotMatch(organizationSource, /按负载 \/ 权重/)
  assert.doesNotMatch(organizationSource, /普通组织仅展示；售后节点只读/)
  assert.doesNotMatch(organizationSource, /当前产品暂无待处理工单/)
  assert.match(organizationSource, /ops\(t, "empty\.noSupportGroups"\)/)
  assert.match(organizationSource, /ops\(t, "ticketsPanel\.empty"\)/)
})

test("organization views use the default technical maintenance team when no product group exists", () => {
  assert.match(organizationSource, /productTeams\.length > 0 \? productTeams : technicalTeam \? \[technicalTeam\] : \[\]/)
  assert.match(organizationSource, /\? \{ team_id: selectedTeam\.id \}/)
  assert.match(membersSource, /productTeams\.length > 0 \? productTeams : technicalTeam \? \[technicalTeam\] : \[\]/)
  assert.match(membersSource, /header\.technicalGroup/)
})

test("product team member page keeps empty and destructive copy terse", () => {
  assert.doesNotMatch(membersSource, /加入后默认启用自动派单/)
  assert.doesNotMatch(membersSource, /接单时间来自个人规则和请假/)
  assert.doesNotMatch(membersSource, /只解除其与/)
  assert.doesNotMatch(membersSource, /选择右上角企业成员加入即可/)
  assert.doesNotMatch(membersSource, /没有成员可选时/)
  assert.match(membersSource, /om\(t, "tableModule\.empty"\)/)
})

test("product team member picker uses paged remote member search", () => {
  assert.match(membersSource, /ENTERPRISE_MEMBER_OPTION_PAGE_SIZE = 50/)
  assert.match(membersSource, /searchValue=\{memberSearch\}/)
  assert.match(membersSource, /onSearchChange=\{setMemberSearch\}/)
  assert.match(membersSource, /shouldFilter=\{false\}/)
  assert.doesNotMatch(membersSource, /fetchEnterpriseIAMMembers\(\{ page: 1, limit: 500 \}/)
  assert.match(optionComboboxSource, /onSearchChange\?: \(value: string\) => void/)
  assert.match(optionComboboxSource, /<Command shouldFilter=\{shouldFilter\}>/)
})

test("organization department catalog is loaded by small pages", () => {
  assert.match(organizationSource, /fetchAllEnterpriseIAMDepartments\(\)/)
  assert.doesNotMatch(organizationSource, /fetchEnterpriseIAMDepartments\(\{ page: 1, limit: 500 \}/)
  assert.match(platformIAMSource, /ENTERPRISE_IAM_CATALOG_PAGE_SIZE = 100/)
  assert.match(platformIAMSource, /export async function fetchAllEnterpriseIAMDepartments/)
})
