# AI 电话 Agent SDD 验证记录

日期：2026-10-12（持续记录）

## 已验证

| 范围 | 命令 | 结果 |
|---|---|---|
| Go AI 电话 handler、路由、来源枚举 | `go test ./internal/handlers/third ./internal/bootstrap ./internal/pkg/enums -count=1` | 通过 |
| Python Agent、恢复 Worker 和模拟 Go HTTP 接口 | `python -m unittest discover -s tests -v` | 8 个测试通过；无 PostgreSQL 环境时 1 个集成测试跳过 |
| PostgreSQL 任务快照和租约领取 | `AI_PHONE_TEST_DSN=... python -m unittest tests.test_postgres_store -v` | 通过（临时 PostgreSQL 16 容器） |
| 客户确认前不入建单队列 | `python -m unittest discover -s tests -v` | 通过 |
| Python 语法 | `python -m compileall -q ai_phone_agent` | 通过 |
| `pyproject.toml` | `tomllib.load(...)` | 通过 |

| Telnyx Webhook 签名、来电流程、去重、建单失败播报（2026-10-10） | `python -m unittest discover -s tests -v` | 28 个测试通过（含 URL 令牌认证），1 个 PostgreSQL 集成测试跳过；仅使用模拟 Telnyx 命令客户端，未连接真实 Telnyx |

| Go 版 Telnyx 认证与命令客户端（2026-10-10） | `go test ./internal/pkg/telnyx` | 通过 |
| Go 版通话服务（流程、确认前置草稿、建单失败、LLM 失败、连续追问、去重、挂断后事件、90018、FIFO、等待提示） | `go test ./internal/services -run AIPhone` | 13 个通过，联调测试默认跳过 |
| Go Webhook 处理器与路由 | `go test ./internal/handlers/third ./internal/bootstrap` | 通过 |
| `ai.WithReasoningEffort` | `go test ./internal/ai -run ReasoningEffort` | 通过 |
| 租户 14 真实 LLM（sub2api gpt-5.5，生产路径，40 秒上限） | `RUN_AI_PHONE_LIVE_TEST=1 AI_PHONE_LIVE_TENANT_ID=14 go test ./internal/services -run TestLiveAIPhoneDecision -v` | 3 次运行：1 次超过 40 秒失败，2 次通过；单轮 2.5～39 秒，多为 11～20 秒；6 次决策动作均正确 |
| 本地 Go Webhook | curl 无令牌 / 有令牌 / 非法 JSON | 403 / 200 / 400；访问日志只记录路径，不含令牌 |
| 本地模型依赖安装（2026-10-12） | `services/ai_phone_agent/.venv-ai-phone`，`python -m pip install -e ".[local-voice]"` | 通过；CPU PyTorch、Silero VAD、faster-whisper、Kokoro 和中文 Misaki 依赖已安装 |
| 本地模型和麦克风单元测试（2026-10-12） | `python -m unittest discover -s tests -p 'test_local*.py' -v`（项目虚拟环境） | 11 个通过，包含 24 kHz WAV 重采样、麦克风入口和 sub2api 响应解析测试 |
| 本地 VAD-ASR-TTS 真实推理（2026-10-12） | 在 `services/ai_phone_agent` 下运行 `python -m ai_phone_agent.local_voice_cli --input D:\work\helpdesk\tmp\ai-phone-local-smoke\kokoro-input.wav --output D:\work\helpdesk\tmp\ai-phone-local-smoke\reply-utf8-2.wav --language zh-CN --reply-text "我已听到您的问题，正在为您准备工单。" --device cpu --compute-type int8` | 退出码 0；Silero 检出 3 段，Whisper 返回中文转写，Kokoro 生成 24 kHz 单声道 PCM16 WAV（约 5.22 秒） |
| 本地麦克风入口（2026-10-12） | `python -m ai_phone_agent.local_voice_mic --help`、`--list-devices`、`python -m unittest tests.test_local_voice_mic -v` | 通过；已安装 `sounddevice`，本机可列出麦克风和扬声器设备；5 个入口、环境文件和响应解析测试通过。`.env.local` 自动加载，真实说话和 sub2api 账号联调待用户执行 |
| Python Helpdesk 决策桥接（2026-10-12） | `python -m unittest discover -s tests -v`；Go `go test ./internal/handlers/third ./internal/bootstrap ./internal/services -run 'AIPhone|Telnyx|Third' -count=1` | 通过；43 个 Python 测试（1 个 PostgreSQL 条件跳过），Go 相关包通过；`HelpdeskDecisionClient` 复用 Go 端租户大模型决策并把确认后的请求交给幂等建单接口 |
| 真实 8083 文本联调（2026-10-12） | `POST /api/third/ai-phone/decisions` + Python `VoiceAgentService` 连续发送“设备无法启动 / 设备编号是 DEV-001，地点在办公室 / 确认创建工单” | 通过；真实 Helpdesk LLM 返回结构化 `answer`、`prepare_ticket`、`confirm_ticket`，Go 建单返回测试工单 `TK2026101200001`（ID 169）；未接入真实麦克风音频 |

本机限制（非本次改动引起）：`go test ./...` 会编译未跟踪的 `work/rag-complex-manual-20261008/`（多个 `main`）而失败；服务层集成测试需 `RHD_BOOTSTRAP_ADMIN_PASSWORD`；`data_retention_service_test` 需要 cgo sqlite；本机 Go 为 1.27.1（CI 为 1.26）。推送前须按 AGENTS.md 在 Linux 环境完整验证。

## 当前未验收

- Go 版真实号码来电与真实建单
- sub2api 延迟：单轮 LLM 决策 2.5～40 秒以上，电话体验不可接受，需要 sub2api 侧更快的模型或通道
- Telnyx 真实号码来电、Google 中文转写和 Polly 中文播报效果；`call.speak.ended` 是否回传 `client_state`（影响播报结束后自动挂机）
- 真实 SIP 线路、FreeSWITCH RTP/AudioSocket 和双向音频（已改为可选路径）
- 真实 ASR、翻译、LLM、TTS 服务质量
- 本地模型在用户真实录音上的识别准确率、CPU 延迟和长音频稳定性
- 生产 PostgreSQL 集群和 Go 服务部署环境下的任务恢复 Worker
- 中文、英文、阿拉伯语真实通话及 5 路并发
- Go 全量测试和仓库 CI 所需的完整依赖环境

这些项目不能标记为已完成；完成前规格状态保持 `In progress`。
