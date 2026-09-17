# 时间与日历最小 MVP · 交付说明

日期：2026-09-17。状态：**开发完成，已在本地真实环境验证，等待用户验收**。

需求原文：`[MUST] 支持 24x7、Business Hours、Holiday、Timezone 和 DST，时间统一存 UTC`。

## 一、MVP 选了什么

先说明为什么是这三项。上一轮核对结论是：工作日历、节假日、时区、DST 在代码里本来就有（`projectconfig.Calendar` + `AddMinutes/WorkingMinutes`，含纽约夏令时用例），真正缺的是"用起来"和"配置得动"。

- **24x7 没有入口**：只能手工把 7 个工作日勾满、把时间填成 `00:00`–`24:00`，没人知道这么填就是全天候。
- **节假日只能一天一条**：国庆、春节这种长假要写 7 条、15 条日期，实际没人愿意维护。
- **排班时区写死**：`EngineerScheduleTimezone = "Asia/Shanghai"` 是常量，排班、请假、值班可用性全按上海时间算，项目时区改了它不跟着走。

"时间统一存 UTC"没有放进这次 MVP，原因见第五节。

## 二、这次改了什么

1. **24×7 与工作日一键预设**（前端）
   `web/app/enterprise/tickets/_components/project-runtime-fields.tsx` 的工作日历里多了两个按钮：
   - 「24×7 全天候」→ 工作日勾满 7 天、上班 `00:00`、下班 `24:00`；
   - 「工作日 09:00-18:00」→ 周一到周五、`09:00`–`18:00`。
   按钮只改表单值，仍然要保存草稿、校验、应用才生效。

2. **节假日支持区间写法**（后端）
   `internal/pkg/projectconfig/runtime.go` 新增 `NormalizeHolidayEntry` 与 `Calendar.IsHoliday`，休息日期现在支持两种写法：
   - `2026-10-01`（原有单日）
   - `2026-10-01..2026-10-07`（闭区间，一个长假写一条）
   校验会拒绝写反的区间（`2026-10-07..2026-10-01`）和格式错误的日期。

   > 2026-09-17 用户反馈「休息日这个没必要」，已把「休息日期」输入行从工作日历表单里去掉。后端能力保留：配置文件或接口仍然可以下发单日和区间，`holidays` 字段照常参与 SLA 计算。也就是说界面上暂时没有配置入口，要恢复成可编辑只需把那一行加回来。

4. **时限规则改为按服务档次配置**（前后端）
   用户反馈「服务档次就可以确定首次回复、接单、解决时间，没必要有工单优先级」。改动：
   - 表单里去掉「工单优先级」和「适用服务项目」，一条时限规则 = 一个**服务档次** + 一条工作日历 + 三个分钟数；
   - 服务项目增加「服务档次」下拉（标准 / 增强 / 关键服务），留空按标准档；
   - 工单按所属服务项目的档次取时限：项目没声明档次、或该档次没配规则、或工单没有项目，都回退标准档；
   - `targets[].project_key` 和 `targets[].priority` 降级为兼容字段（都可省略），schema、Go 校验、界面提示同步调整；每个档次只能配置一组时限；
   - 旧的按优先级存的 SLA 策略在 `升级配置` 时折算成三档：**业务 P1/P2 → 关键服务、P3 → 增强、P4 → 标准**，同一档次取更紧的一组。注意旧 `sla_policies` 表存的是兼容键 `p0`–`p4`，比业务优先级 `P1`–`P4` 整体低一档（`ticketpolicy.LegacyCode`：p1→p0、p2→p1、p3→p2、p4→p3），代码里统一走 `ticketpolicy.FromLegacy` 转换，不手工拼字符串。

3. **排班时区可配置**（后端）
   - 新增配置 `server.timezone`（IANA 名称，默认 `Asia/Shanghai`），可用环境变量 `RHD_SERVER_TIMEZONE` 覆盖，`config/config.example.yaml` 已加示例。
   - `EngineerScheduleTimezone` 从常量改成启动时注入的值，新增 `ConfigureEngineerScheduleTimezone` 做校验；空值回落默认，非法值让启动直接失败，避免带着错时区跑。
   - 启动日志新增一行 `engineer schedule timezone configured timezone=...`，运维能直接确认生效值。

## 三、怎么验收

**24×7 与工作日预设**：进 企业端 → 工单中心 → 受理规则 → 配置版本，展开「工作日历」，点「24×7 全天候」，应看到 7 个工作日全部勾选、上班 `00:00`、下班 `24:00`；点「工作日 09:00-18:00」应变回周一到周五。

**节假日区间**：界面上已没有「休息日期」输入行，改用接口验证——跑 `pwsh -NoProfile -File tmp\verify-mvp.ps1`，会依次看到单日 `2026-10-01` 通过、区间 `2026-10-01..2026-10-07` 通过、写反的区间和错误格式被拒。

**排班时区**：用 `RHD_SERVER_TIMEZONE=America/New_York` 启动一次后端，看启动日志和工程师「我的排班」接口返回的时区；不设该变量时仍是 `Asia/Shanghai`。

**日历真的影响 SLA**：你现在的工单都没有服务项目，所以一律按标准档取时限。配一条 `09:00`–`12:00`、周一至周五的日历，给标准档选它、解决问题分钟填 120，保存草稿 → 检查配置 → 应用。挑一个 12:00 之后的时间建工单，工单详情里的 SLA 应落在下一个工作日 11:00 左右，而不是「当前时间 + 120 分钟」；接单截止同理落在下一个工作日 09:30。

**三档时限**：我留了一份草稿（`三档服务时限初始值：关键 15/15/240（7x24）、增强 30/30/480（工作日）、标准 120/240/1440（工作日）`），在弹窗的「历史与草稿」里点「载入为新草稿」就能看到三条按档次分好的规则。注意：应用任何运营配置前，环境需要先配好密钥目录（原因见第六节）。

## 四、已经跑过的验证（真实环境）

- `POST /api/enterprise/v1/ticket-settings/configuration/validate` 实测四种输入：

  | 输入 | 结果 |
  | --- | --- |
  | 24×7 日历（7 天 / 00:00-24:00）+ `2026-10-01..2026-10-07` | `valid=true` |
  | 单日 `2026-10-01`（旧写法） | `valid=true` |
  | 区间写反 `2026-10-07..2026-10-01` | `valid=false`，报休息日期格式错误 |
  | 格式错误 `2026/10/01` | `valid=false`，报休息日期格式错误 |

- 排班时区端到端：以 `RHD_SERVER_TIMEZONE=America/New_York` 启动后，启动日志打印 `timezone=America/New_York`，`GET /api/dashboard/agent/self/work-schedule` 返回 `timezone = America/New_York`；去掉变量重启后回到 `Asia/Shanghai`，接口同步回到 `Asia/Shanghai`。
- 单测：新增 `TestRuntimeCalendarHolidayRangeAndAllDay`（区间跳过 + 24×7 跨零点）、`TestConfigureEngineerScheduleTimezone`、`TestServerTimezoneDefaultsAndEnvironmentOverride`，并给现有校验用例补了「区间写反」「格式错误」两例；`./internal/pkg/config/`、`./internal/pkg/projectconfig/`、`./internal/bootstrap/`、`./internal/migration/`、`./internal/handlers/...` 相关包通过；前端 `tsc --noEmit` 与改动文件的 `eslint` 通过。

## 五、这次没做，以及为什么

- **时间统一存 UTC（没做）**：工单、通知等表用的是 Postgres `timestamp`（无时区），历史数据是按 `Asia/Shanghai` 墙钟写进去的。只把连接改成 UTC 会让所有按 RFC3339 输出的接口（通知、工单列表）整体偏移 8 小时；要正确做，必须同时做三件事——数据回填（历史值 -8h）、写入侧统一 UTC、读取侧按配置时区渲染后再输出。这是一次带停机窗口的数据迁移，不属于 MVP。目前部署配置本身也不一致：`docker-compose.prod.yml` 是 `TimeZone=UTC`，`deploy/1panel/docker-compose.yml` 与 `docker/remotehelpdesk.yaml` 是 `TimeZone=Asia/Shanghai`。
- **团队级节假日没接线**：`agent_team_holidays` 表和迁移都在，但既没有管理入口，也没有任何业务代码读它（`FindActiveHolidaysByTeamIDs` 只在测试里出现过）。要做成"节假日不派单"需要补 API + 界面 + 派单判断，属于下一批。
- **一条日历一天只能一个时间段**：午休要拆两段（如 08:00-12:00 + 13:30-17:30）目前表达不了，且一条时限规则只能挂一条日历。要支持得改 `Calendar` 结构并兼容已有配置。
- **界面不再暴露休息日期**（按用户 2026-09-17 要求删除）：需求里的 Holiday 能力后端仍在，但页面没有配置入口，节假日目前只能用配置包或接口下发。
- **「服务档次」目前还是标注**：工单身上没有档次字段，所以档次不参与命中计算，真正决定用哪组时限的是服务项目。要让「档次不同、时限不同」真正生效，需要给服务项目或工单落一个档次字段（项目侧更省事：在项目配置里选等级，时限规则按项目取到该等级）。这是下一步可以做的。
- **更新**：服务档次已经能决定时限（服务项目带档次，规则按档次配）。但**建单界面还没有「服务项目」字段**，手工建的工单项目为空，一律落到标准档；要用上关键/增强档，需要让建单时能选服务项目（接口 `project_key` 已支持，缺的是界面）。邮件入站建单已经能带项目。
- **应用任何运营配置的前置条件**：环境要先配好密钥目录（`RHD_PROJECT_SECRET_DIR`），目录下按 `{租户ID}/{环境}/{密钥名}` 放密钥文件。「升级配置」生成的草稿会带上 `secret://smtp-password`（取的是租户已有的邮件设置），没有密钥目录时检查配置会报「此环境尚未配置密钥目录」，应用会被挡住。

## 六、注意

本次改动需要重启后端才生效。本地 8083 端口已换成本次构建（`tmp/server-mvp.exe`）并已按默认时区 `Asia/Shanghai` 运行。

## 七、测试结果里的一个说明

`go test ./internal/services/` 在**当前工作区**有 7 个失败：3 个是 `CGO_ENABLED=0` 下 go-sqlite3 是 stub 的老问题（`data_retention` 三条），另外 4 条是 `TestTicketCanonicalDispatchAndReassignmentFollowTheHandlingEngineer`、`TestTicketDuplicateParentValidationAndAtomicRollback`、`TestTicketStatusWorkflowPublishRuntimeAndRestore`、`TestSupplierCollaborationTimeoutEscalatesToSupervisorAndAllowsReinvite`。

这 4 条不是本次改动引入的：失败信息都是工单归属/生命周期语义（例如 `acknowledge: 1000: 当前状态不允许此操作`），涉及的文件 `internal/services/ticket_case_lifecycle_service.go`、`ticket_lifecycle_service.go`、`ticket_dispatch_service_test.go`、`internal/pkg/dto/ticket_case_lifecycle_dto.go` 在工作区里都还是未提交的在改状态；把同一批测试放到干净检出（`tmp/ci-final-validation`，824d56d）跑是 `ok`。本次改动没有触碰这些文件，也没有改变任何状态机行为。
