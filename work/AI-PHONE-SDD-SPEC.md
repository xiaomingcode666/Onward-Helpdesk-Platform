# AI 电话智能客服与自动建单规格

状态：In progress
版本：0.3
日期：2026-10-10

> 2026-10-10 用户确认：电话接入全部改为 Go 实现（见第 15 节），第 3、5、8、11、12、13、14 节中由 Python Agent 承担的职责全部移入 Go 后端；Python 服务 `services/ai_phone_agent/` 在 Go 版本真机验证通过前保留作对照，之后删除。

## 1. 目标

客户通过中国大陆 SIP 电话呼入后，Python 语音 Agent 使用中文、英文或阿拉伯语完成实时对话，识别客户诉求并调用 Go 工单服务创建工单。Go 服务继续负责租户校验、幂等、工单写入和现有派单流程。

## 2. 范围

### 2.1 本期包含

- SIP 呼入、双向语音和通话结束处理
- 语言识别、ASR、翻译、字段提取和 TTS
- 最多限定轮次的字段追问
- 工单草稿生成和客户确认
- Python 到 Go 的受控建单调用
- 建单超时、结果未知和幂等重试
- 通话、转写、模型版本和工单关联记录

### 2.2 本期不包含

- 浏览器 WebRTC 坐席
- 人工坐席接听电话
- 外呼回访
- Python 直接访问业务数据库
- Python 直接写入 `tickets` 表

## 3. 架构约束

```text
SIP 运营商 → FreeSWITCH → Python Voice Agent
                              ├─ ASR / LLM / TTS
                              └─ Go Ticket API → 工单与派单
```

- FreeSWITCH 负责 SIP、RTP、录音、DTMF、挂断和媒体回注
- Python 负责实时语音、对话状态和 AI 决策
- Go 负责业务权限、租户、字段校验、幂等和工单创建
- Python 只能通过内部 REST 或 gRPC 调用 Go
- Python 不接受请求体中的可覆盖租户 ID

## 4. 业务规则

1. 来电建立稳定的 `call_id`，每个独立诉求建立 `intent_id`
2. 号码国家码只设置默认语言，实际发言和 DTMF 可以改变语言
3. AI 先确认有效诉求，再收集必填字段
4. 关键字段由 AI 复述，客户可以确认或修改
5. 信息不完整但诉求有效时，按业务规则创建带缺失字段的工单
6. 静音、误拨和无有效诉求不创建工单
7. Go 未返回成功结果时，Python 不播报工单号
8. Go 超时属于结果未知，使用相同幂等键查询和重试
9. 一通电话的多个诉求分别建单，但关联同一 `call_id`

## 5. Python Agent 状态

```text
initializing → listening → recognizing → deciding → speaking
                                      ↓
                                  confirming
                                      ↓
                                  submitting
                                      ↓
                                  completed
```

任意状态可进入 `failed`、`cancelled` 或 `recovering`。客户在 TTS 播放时讲话，Agent 立即停止当前 TTS，取消当前回复任务并回到 `listening`。

## 6. Agent 决策契约

每轮决策必须返回结构化结果，禁止使用自由文本决定是否建单：

```json
{
  "action": "clarify",
  "intent_id": "intent_001",
  "reply_text": "请提供设备编号。",
  "collected_fields": {
    "issue_summary": "设备无法启动"
  },
  "missing_fields": ["device_no"],
  "ready_to_create": false
}
```

`action` 只允许：`answer`、`clarify`、`prepare_ticket`、`confirm_ticket`、`cancel`、`end`、`fallback`。

## 7. Go 建单接口契约

```http
POST /api/third/ai-phone/tickets
Authorization: Bearer <service credential>
X-AI-Phone-Line-ID: <configured line id>
Idempotency-Key: provider:call_id:intent_id
Content-Type: application/json
```

请求字段：

```json
{
  "schema_version": 1,
  "provider": "china-telecom",
  "call_id": "call_001",
  "intent_id": "intent_001",
  "language": "zh-CN",
  "work_language": "zh-CN",
  "title": "设备无法启动",
  "description": "客户反馈设备无法启动",
  "project_key": "project_a",
  "ticket_type": "incident",
  "caller_phone": "13800000000",
  "missing_fields": ["device_no"],
  "recording_ref": "phone/call_001.wav",
  "transcript_segments": [],
  "field_evidence": {}
}
```

Go 返回：

```json
{
  "success": true,
  "ticket_id": 12345,
  "ticket_no": "TK202610080001",
  "status": "pending_dispatch"
}
```

必须携带 `Idempotency-Key`，且必须等于 `provider:call_id:intent_id`。相同幂等键和不同载荷必须返回冲突；重复请求返回同一工单结果。租户由 `X-AI-Phone-Line-ID` 的服务端配置映射推导，不能由请求体覆盖。

## 8. 数据和恢复要求

Python 至少记录 `call_id`、`intent_id`、语言、Agent Release、模型版本、转写片段、录音引用和建单请求摘要。建单前先保存任务；任务状态包括 `pending`、`submitting`、`result_unknown`、`created` 和 `needs_attention`。

Python 进程重启后可以继续处理未完成任务。ASR、LLM、TTS 或 Go 暂时不可用时，通话播放固定提示，并保留已经获得的有效信息。

## 9. 验收标准

- 三种语言均能完成真实或仿真的完整通话
- 支持 DTMF 切换语言、客户打断、静音和挂机
- 有效诉求能创建工单，静音和误拨不创建工单
- 关键字段、原文、译文和模型版本可追溯
- Go 服务超时重试不会创建重复工单
- Python 不能跨租户创建工单
- 工单进入现有派单和主管兜底流程
- 5 路并发时不丢失通话任务
- AI、Go 或数据库短暂故障后任务可恢复

## 10. 开发前假设

- 首期接入中国大陆企业 SIP 中继
- FreeSWITCH 采用 G.711 `PCMA/PCMU`
- Python 使用 FastAPI 和 `asyncio`
- 首期通过 PostgreSQL 任务表恢复任务，不引入消息集群
- SIP 供应商、ASR、LLM、TTS 和服务区域在 PoC 前确定

## 11. 当前实现边界

- 已实现 Go 建单边界：`/api/third/ai-phone/tickets`、Bearer 服务凭证、线路到租户绑定、强制幂等键和 `ai_phone` 来源。
- 已实现 Python 文本 Agent：结构化动作、字段追问、客户确认/取消、状态和建单失败状态。
- 已实现 ASR/TTS/翻译的适配器协议、PostgreSQL 任务表和结果未知扫描 Worker；真实供应商和 FreeSWITCH 仍在后续阶段。

### 11.1 本地模型验证工具（不作为当前电话生产路径）

- `services/ai_phone_agent/ai_phone_agent/local_voice_cli.py` 提供 WAV 文件级验证：`Silero VAD → openai/whisper-large-v3-turbo → hexgrad/Kokoro-82M`。
- Whisper 通过 `faster-whisper` 运行；Silero 对非 8/16 kHz 的 PCM WAV 先重采样到 16 kHz，再将语音区间映射回原始帧位置。
- Kokoro 当前提供中文 `zf_xiaobei`、美式英语 `af_heart` 和英式英语 `bf_emma`；阿拉伯语本地 TTS 明确报不支持，不使用错误音色降级。
- 该工具只用于模型和音频链路测试，不接 SIP/Telnyx、不调用 sub2api、不创建工单。真实电话媒体接入仍按第 15 节的 Go/Telnyx 路径或后续原始媒体适配方案验收。

### 11.2 本地麦克风测试入口

- `local_voice_mic.py` 使用本机输入/输出设备做按键分段测试：回车开始录音，回车结束本轮，然后执行 VAD、ASR 和 TTS 播放。
- 加 `--smart-reply` 时，转写文本发送到配置的 OpenAI 兼容 sub2api `/v1/chat/completions`；对话测试使用普通用户 API Key，不能使用管理 Key。
- 麦克风测试只验证音频和回答体验，不接 SIP，不创建工单；默认不请求 LLM，省略 `--smart-reply` 时仅复述转写文本。

## 12. 建单恢复 Worker

- 只有客户明确确认后才持久化建单任务；未确认草稿、取消和静音不入任务队列。
- Worker 扫描到期的 `pending` / `result_unknown`，并恢复领取租约已过期的 `submitting`。进程在发送请求前后崩溃都可以恢复。
- 建单前冻结完整 JSON 请求、线路 ID 和幂等键；所有重试原样发送，不能重新读取通话上下文拼接载荷。
- PostgreSQL 通过 `FOR UPDATE SKIP LOCKED` 原子领取任务；每次领取使用新租约标识，旧 Worker 不能覆盖新 Worker 的结果。
- Go 返回完整成功结果时任务变为 `created`；传输异常、5xx、429 或不可解析的成功响应进入指数退避重试；4xx、幂等冲突和重试耗尽进入 `needs_attention`。
- 默认轮询 2 秒、领取租约 60 秒、最多发送 5 次、首次重试等待 5 秒、最长退避 300 秒。租约必须大于单次 HTTP 超时。
- 旧版本任务没有冻结载荷，迁移时进入 `needs_attention`，不自动猜测并重放。
- Worker 不向客户播报结果，客户播报只由当前通话会话负责。
- 模拟 Go 接口测试必须验证请求路径、线路头、幂等键和重复请求只得到同一工单结果。

## 13. Telnyx Voice API 接入（替代自建 FreeSWITCH 的首期方案）

2026-10-10 用户确认：首期电话接入改用 Telnyx Voice API Application（Call Control），不自建 FreeSWITCH。原 FreeSWITCH 方案保留为后续可选路径，Python 决策层和 Go 建单接口不变。

```text
手机 → Telnyx 号码 → Voice API Application ──Webhook──→ Python /v1/telnyx/webhook
                                     ←──Call Control 命令（answer / speak / transcription_start / hangup）
Python → Go /api/third/ai-phone/tickets
```

### 13.1 安全

- 每个 Webhook 必须校验 `telnyx-signature-ed25519` 与 `telnyx-timestamp`，签名内容为 `{timestamp}|{原始请求体}`，公钥来自 `TELNYX_PUBLIC_KEY`。
- 时间戳偏差超过 300 秒、签名错误或未配置公钥时一律拒绝（403），不得放行。
- 2026-10-10 实测：当前 Telnyx 账户等级无法读取 Public Key（"This feature is not permitted at this account level"）。未配置公钥时，改用 `TELNYX_WEBHOOK_TOKEN`（至少 32 位随机值）作为 Webhook URL 的 `?token=` 参数，常量时间比较；公钥和令牌都未配置时仍一律拒绝。令牌只能证明请求来自知道 URL 的一方，无法防重放，账户升级后应改回签名校验。
- Agent 以 `--no-access-log` 启动，避免访问日志记录带令牌的 URL。
- 被叫号码必须在 `TELNYX_NUMBER_LINES` 中配置，映射到 `line_id`、`project_key`、`language`；未配置号码直接挂断，不建会话。租户仍由 Go 端的 `line_id` 绑定推导。
- `TELNYX_API_KEY` 只用于调用 Telnyx 命令接口，不写日志。

### 13.2 通话流程

| Telnyx 事件 | Agent 动作 |
|---|---|
| `call.initiated`（`direction=incoming`） | 校验被叫号码 → `answer`；未配置号码 → `hangup` |
| `call.answered` | 建立会话（`call_id` = `call_leg_id`，`provider=telnyx`），播报开场白，`transcription_start`（仅 inbound 轨道，只取最终结果） |
| `call.transcription`（`is_final=true`，文本非空） | 交给 `VoiceAgentService.handle_text`，播报 `reply_text`；建单成功后播报工单号并在播报结束后挂断 |
| `call.speak.ended` | 若会话已标记结束（建单完成、取消、结束、失败），执行 `hangup` |
| `call.hangup` | 释放通话映射与会话；已确认的建单任务仍由任务表和恢复 Worker 负责 |

- 每次播报使用 `stop=all`，客户新的发言会打断尚未播完的回复。
- 建单结果未知或失败时，不播报工单号，只播报固定提示后挂断。
- 同一事件重复投递按 `data.id` 去重；Telnyx 命令的 `command_id` 由事件 ID 和动作派生，重放时 Telnyx 侧也会忽略重复命令。
- 同一通话的事件按到达顺序串行处理；Webhook 先返回 200，再在后台处理。

### 13.3 语言映射

| 会话语言 | 转写 `language`（Google 引擎） | 播报 `language` | 默认音色（可用 `TELNYX_TTS_VOICES` 覆盖） |
|---|---|---|---|
| `zh-CN` | `zh` | `cmn-CN` | `AWS.Polly.Zhiyu-Neural` |
| `en-US` | `en` | `en-US` | `AWS.Polly.Joanna-Neural` |
| `ar` | `ar` | `arb` | `AWS.Polly.Zeina` |

### 13.4 本阶段不包含

- DTMF 切换语言、静音超时提示、最长通话时长控制
- LLM/知识库回答（仍使用规则决策引擎）
- 多进程部署下的通话状态共享（通话映射保存在单进程内存中）

## 14. 大模型决策（复用现有 AI 配置）

2026-10-10 用户确认：电话 Agent 的对话决策改用大模型，复用 helpdesk 现有 AI 配置。

### 14.1 架构

Python 不直接持有大模型凭证。现有 LLM 路由按租户解析（sub2api 租户账号、租户默认模型、数据库或环境配置），只能在 Go 内完成，因此新增 Go 决策接口：

```http
POST /api/third/ai-phone/decisions
Authorization: Bearer <service credential>
X-AI-Phone-Line-ID: <configured line id>
```

- 鉴权与建单接口相同：服务令牌 + 线路绑定推导租户；请求体中不接受租户 ID。
- Go 以 `ai.WithCapabilityScope({TenantID})` 调用 `ai.LLM.Chat`，沿用现有模型路由与故障切换。
- Go 只返回校验后的结构化决策，不创建工单；建单仍由 Python 在客户确认后调用 `/api/third/ai-phone/tickets`。

请求：

```json
{
  "schema_version": 1,
  "call_id": "leg-1",
  "language": "zh-CN",
  "text": "最新一句客户转写",
  "history": [{"role": "agent", "text": "..."}, {"role": "customer", "text": "..."}],
  "collected_fields": {"issue_summary": "..."},
  "pending_ticket": {"title": "...", "description": "..."}
}
```

响应：

```json
{
  "action": "prepare_ticket",
  "reply_text": "我帮您记录为……，确认创建工单吗？",
  "collected_fields": {"issue_summary": "设备无法启动", "device_no": "DEV001"},
  "missing_fields": [],
  "ticket": {"title": "设备无法启动", "description": "..."},
  "model": "qwen-plus"
}
```

### 14.2 决策规则

- `action` 仍限于第 6 节的七个值；不合法 JSON、未知动作、空回复或回复超过 300 字时 Go 返回 502，不把原始模型输出交给 Python。
- 回复为口语、简短（建议 60 字内）、无 Markdown，使用客户语言。
- 转写明显不通顺或无法理解时使用 `clarify` 请客户重说，不得把无意义文本写入工单。
- 只有在客户明确同意后才可返回 `confirm_ticket`；Python 另外要求会话里已有 `prepare_ticket` 生成的草稿，否则降级为 `clarify`。
- 大模型不得编造工单号、处理时限或承诺；本阶段不接知识库，只给通用排查建议，无法解决时引导建单。
- Python 连续 3 次 `clarify` 后结束通话（原为累计 3 次）。
- Go 决策接口失败或超时（默认 15 秒）时，Python 播报"抱歉，系统有点忙，请您再说一遍"，会话保持在 listening。

### 14.3 配置

- `AI_PHONE_DECISION_ENGINE=llm|rule`，默认 `llm`；`rule` 保留原规则引擎用于离线测试。

## 15. 全部改为 Go（2026-10-10，取代第 13、14 节中的 Python 部分）

### 15.1 理由

采用 Telnyx Voice API 后，SIP、媒体、ASR、TTS 均由 Telnyx 承担，Agent 只剩 Webhook 处理、Call Control REST 调用和对话状态，不再需要 Python 的音频生态；大模型路由、知识库和建单都在 Go 内，且 Python 代码不在仓库 CI 覆盖范围内。

### 15.2 架构

```text
手机 → Telnyx 号码 → Voice API Application ──Webhook──→ Go POST /api/third/telnyx/voice/webhook
                                   ←── Call Control（answer / speak / transcription_start / hangup）
Go AIPhoneCallService ──→ ai.LLM.Chat（租户作用域）──→ TicketService.CreateTicket（进程内，幂等键）
```

- `internal/pkg/telnyx`：Webhook 认证（Ed25519 签名优先；账户无公钥时用 `TELNYX_WEBHOOK_TOKEN` URL 令牌）和 Call Control HTTP 客户端。
- `internal/services/ai_phone_call_service.go`：通话状态、事件串行处理、大模型决策、确认后建单。
- `internal/handlers/third/telnyx_voice_handler.go`：读取原始请求体、认证、入队后立即返回 200。
- 第 14 节的 `/api/third/ai-phone/decisions` 外部接口不再实现，决策在进程内完成；`/api/third/ai-phone/tickets` 保留给外部接入方。

### 15.3 行为

- 第 13.1、13.2、13.3、14.2 节的安全、通话流程、语言映射和决策规则保持不变，执行方由 Python 改为 Go。
- 同一通话事件按到达顺序串行处理（每通电话一个 FIFO 队列）；事件按 `data.id` 去重。
- 建单在进程内调用 `TicketService.CreateTicket`，幂等键 `telnyx:{call_leg_id}:intent_001`，来源 `ai_phone`、渠道 `phone`；工单描述附带通话转写。建单失败只播报固定提示，不播报工单号。
- 不再需要 Python 的 PostgreSQL 任务表和恢复 Worker：建单与幂等记录在同一数据库内完成，不存在跨进程"结果未知"。
- Telnyx 返回 90018（通话已结束）时静默清理通话，不记错误。
- 每个事件、大模型调用和 Telnyx 命令记录耗时日志（不记录转写原文）。
- 单通电话最多 20 轮客户发言；连续 3 次 `clarify` 或连续 3 次决策失败后播报结束语并挂机。
- 通话状态保存在单个 Go 进程内存中；多实例部署需要会话亲和或共享状态，本期不包含。

### 15.4 配置（环境变量）

| 变量 | 说明 |
|---|---|
| `TELNYX_API_KEY` | Call Control 命令 |
| `TELNYX_PUBLIC_KEY` / `TELNYX_WEBHOOK_TOKEN` | Webhook 认证，二者都未配置时全部拒绝 |
| `TELNYX_NUMBER_LINES` | `{"+E164":{"line_id","project_key","language"}}` |
| `AI_PHONE_LINE_TENANTS` | `{"line_id": tenant_id}`，与建单接口共用 |
| `AI_PHONE_SERVICE_ACCOUNT_ID` | 建单操作人服务账号 ID，可为 0 |
| `TELNYX_TTS_VOICES` | 可选，按语言覆盖音色 |
| `TELNYX_TRANSCRIPTION_ENGINE` | 可选，默认 `Google`；支持 `Telnyx`（whisper-large-v3-turbo） |
