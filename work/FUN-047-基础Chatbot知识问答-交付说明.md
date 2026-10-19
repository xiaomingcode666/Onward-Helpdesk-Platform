# FUN-047 基础 Chatbot 已审批知识问答

日期：2026-09-22。状态：开发完成，已完成本地定向验证，待用户验收；在线需求表状态待同步。

## 本次实现

- 基础知识问答流程 `knowledge_customer_service_default` 设置为知识专用模式。
- 只有检索到的已审批且仍有效知识内容才允许生成业务事实回答。
- 无知识命中时，业务问题直接返回兜底，不调用模型自行补答。
- 普通问候、感谢、结束语和信息不完整的澄清请求使用固定话术完成基础接待。
- 知识专用流程不向模型暴露租户配置的动态 MCP 动作工具，避免执行交易、账户变更、退款、赔付、封禁或 Priority 最终裁决。
- 保留意图分流、语言跟随和人工转接路径。

## 本地验证

- `go test ./internal/ai/...` 通过。
- `go test ./internal/ai/runtime/workflow ./internal/ai/runtime/executor ./internal/ai/workflow/builtin ./internal/ai/runtime/internal/impl/factory ./internal/pkg/toolx -count=1` 通过。
- `go vet ./internal/ai/...` 通过。
- `git diff --check` 通过。

仓库级 `go test ./...` 仍受现有环境问题影响：部分 SQLite 测试在 `CGO_ENABLED=0` 下不可运行，另有项目配置测试受共享临时数据库状态影响；相关失败与本次 FUN-047 改动无关。
