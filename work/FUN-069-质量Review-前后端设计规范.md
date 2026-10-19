# 质量 Review 前后端设计规范

## 1. 简化需求

每次工单质量 Review 都必须形成一条可追溯记录，至少保存：评分、缺陷码、证据、审核人、争议状态、评估结果和辅导动作。记录必须绑定租户、工单和评分卡版本，并可在报表考评的质量抽检流程中查看。

## 2. 对应系统功能

- 报表考评 → 质量抽检：抽检队列、检查状态和完成入口。
- 质量评估弹窗：填写评分、缺陷码、证据、争议和辅导动作。
- 评分卡版本：保存评分项版本，Review 保存评分快照，避免后续评分卡修改影响历史记录。
- 工单工作台：只保留工单处理上下文，不再承载完整质量评估表。
- 质量分析接口：基于 Review 记录统计评分、平均分和通过率。

## 3. 后端规范

### Review 数据

`TicketQualityReview` 必须包含：

- `total_score` / `max_score`：评分和满分。
- `defect_codes_json`：缺陷码数组，去重、统一小写。
- `evidence`：质检证据文本，可引用对话、工单记录或附件依据。
- `reviewer_id`：审核人账号 ID。
- `dispute_status`：`none`、`open`、`resolved`。
- `dispute_note`：争议说明。
- `outcome`：`pass`、`needs_improvement`、`fail`。
- `coaching_action`：辅导动作或“无需辅导”。
- `score_snapshot_json`：提交时的评分卡版本和评分项快照。

### 接口

现有接口继续复用：

- `GET /api/enterprise/v1/ticket-quality/scorecard/active`
- `GET /api/enterprise/v1/tickets/:id/quality-reviews`
- `POST /api/enterprise/v1/tickets/:id/quality-reviews`
- `GET /api/enterprise/v1/ticket-quality/analysis`

Review 创建请求新增：

```json
{
  "answers": {},
  "remark": "",
  "defect_codes": ["communication_delay"],
  "evidence": "客户对话第 3 条，客服未确认设备序列号",
  "dispute_status": "none",
  "dispute_note": "",
  "outcome": "needs_improvement",
  "coaching_action": "补充身份确认和信息采集培训"
}
```

后端负责校验评分项、争议状态、结果枚举、文本长度和缺陷码格式；不能由前端直接决定审核人，审核人从登录主体读取。

## 4. 前端规范

- 质量 Review 入口只放在报表考评 → 质量抽检。
- 工单详情不显示完整评分表，避免处理页面信息过载。
- 评分完成后显示总分、结果和评分卡版本。
- 缺陷码使用逗号输入，提交前转换成数组；后续可替换为标准多选控件。
- 证据为必填项。
- 争议状态为下拉选择；状态不是 `none` 时显示争议说明。
- 辅导动作默认“无需辅导”，允许改为具体培训、复盘或跟进动作。
- 评估历史显示审核时间、审核结果、分数和备注；详情字段通过 Review API 返回。

## 5. 本次实现范围

- 已新增 Review 结构化字段及迁移 `000158`。
- 已扩展 Review 创建接口和前端提交模型。
- 已在质量评估弹窗增加缺陷码、证据、争议和辅导动作录入。
- 已保留原有评分卡版本和评分快照能力。
- 已保留租户隔离和审核权限校验。

## 6. 验收标准

- 创建 Review 后可查询到七类必需信息。
- 修改评分卡版本后，历史 Review 的评分快照不变。
- 缺陷码、证据、争议状态、结果和辅导动作可在前端提交并在接口响应中返回。
- 不同租户不能读取或写入其他租户的 Review。
- 没有证据时不能提交质量 Review。

## 7. 自动预评估与简化交互（2026-09-28）

- 新增只读自动预评估接口，按工单身份资料、上下文完整性、分类与优先级、客户可见进展、派单记录、跟进时间及解决结论生成九项建议分数。
- 自动结果必须返回逐项依据和置信度；系统无法可靠证明的资料保护、自定义评分项及证据不足项标记为“需人工确认”，不自动给满分。
- 评估弹窗默认只显示自动总分、结论和待确认数量；逐项调整与异常、争议、辅导字段按需展开。
- 确认评估后同时保存 Review 并完成抽检，避免重复执行“提交评估”和“完成抽检”。
- 抽检列表直接显示最近一次分数、结论和“查看结果”入口；历史评估仍保留并可追溯。
- 质量分析筛选控件使用统一尺寸和响应式网格，日期与普通输入框保持一致。


## 8. 七要素展示与异议复核（2026-10-12）

需求重新解读：Review 不只"保存"七要素，还要让被检客服看得懂哪里扣分、为什么、怎么改，并能对结果提出一次异议。

- Review 新增 `reviewed_agent_id`、`reviewed_team_id`（质检时快照）和 `final_outcome`（初始等于 `outcome`）；原始分数和 `outcome` 提交后不再修改。迁移 `000162` 回填旧记录并同步权限。
- 提交规则：问题类型只能从所用评分卡的评分项中选择；结论不是 `pass` 时至少一个问题类型，辅导措施不能为空或"无需辅导"；提交质检时不再接受争议字段。
- 异议：新表 `t_ticket_quality_dispute`。只有被检客服本人、质检后 7 天内、每条一次可提出；复核需 `ticketQuality.disputeReview`，且不能是原质检人或被检客服；改判只改 `final_outcome`。
- 权限：新增 `ticketQuality.viewAgent`（查看客服个人质检）和 `ticketQuality.disputeReview`（复核质检异议），默认授予所有者、管理员、服务负责人。详情只对被检客服、质检人和 `viewAgent` 可见，其他人只看分数和最终结论。质量分析的客服维度改用 `viewAgent`，无权限带 `agent_id` 直接拒绝；按快照归属，按 `final_outcome` 统计合格率。
- 接口：`GET /ticket-quality/my-reviews`、`POST /ticket-quality/reviews/:id/disputes`、`GET /ticket-quality/disputes`、`POST /ticket-quality/disputes/:id/_decide`。
- 页面：质检表单问题类型改为评分项勾选；历史显示评分明细（标出扣分项）、问题类型、证据、质检人、被检客服、辅导、异议过程和最终结论；新增"我的质检"页面；质量抽检页新增"异议复核"。
- 本轮不发通知；e2e 未补充。
