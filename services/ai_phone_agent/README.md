# AI Phone Agent

这是电话接入的 Python Agent，按 `work/AI-PHONE-SDD-SPEC.md` 实现。它只负责通话状态、字段收集和 AI 决策；工单写入必须调用 Go 的 `/api/third/ai-phone/tickets`，不能直接访问业务数据库。

## 本地运行

```powershell
cd services/ai_phone_agent
python -m unittest discover -s tests -v
python -m uvicorn ai_phone_agent.app:app --reload --port 8090 --no-access-log
# 单独运行建单恢复 Worker
python -m ai_phone_agent.recovery
```

最小配置：

```text
HELPDESK_GO_URL=http://127.0.0.1:8083
AI_PHONE_SERVICE_TOKEN=<至少 32 位随机值>
AI_PHONE_LINE_ID=line-1
AI_PHONE_PROJECT_KEY=support
# app.py 文本接口需要时可设为 helpdesk；规则引擎仍可用作离线测试
AI_PHONE_DECISION_ENGINE=rule
AI_PHONE_TASK_DATABASE_URL=postgresql://...   # 可选；不设置时使用内存存储
```

如果当前 PowerShell 是由旧的 IDE 进程启动、没有继承新的用户级变量，执行一次：

```powershell
. D:\work\helpdesk\scripts\load-ai-phone-env.ps1
```

文本模拟接口：

1. `POST /v1/sessions/{call_id}` 创建会话，body 使用 `VoiceSession`。
2. `POST /v1/sessions/{call_id}/turn` 发送一轮识别文本，body 为 `{ "call_id": "...", "text": "..." }`。
3. 只有客户确认后，Agent 才会调用 Go 建单接口。

## 本地 VAD-ASR-TTS 模型测试

本地模型测试不接 SIP，使用 WAV 文件验证完整的语音处理链路：

```text
WAV → Silero VAD → openai/whisper-large-v3-turbo → hexgrad/Kokoro-82M → WAV
```

建议使用项目目录下的独立虚拟环境。这样不会依赖系统 Python 的包，也不会影响 Go 服务或其他 Python 项目：

```powershell
cd services/ai_phone_agent
python -m venv .venv-ai-phone
.\.venv-ai-phone\Scripts\python.exe -m pip install -e ".[local-voice]"
```

Windows CPU 测试直接使用 `--device cpu --compute-type int8`。如果机器有匹配版本的 NVIDIA CUDA，可以先按 PyTorch 官方说明安装 CUDA 版，再改为 `--device cuda --compute-type float16`。

模型首次运行会下载到本地缓存；下面的设置把缓存放在项目的 `.local` 目录：

```powershell
$env:HF_HOME = "D:\work\helpdesk\.local\hf-cache"
$env:HF_HUB_CACHE = "$env:HF_HOME\hub"
$env:PYTHONIOENCODING = "utf-8"
```

准备一个未压缩 PCM WAV（单声道或多声道、8/16/24/44.1/48 kHz 均可），然后运行中文测试：

```powershell
.\.venv-ai-phone\Scripts\python.exe -m ai_phone_agent.local_voice_cli `
  --input .\samples\customer.wav `
  --output .\outputs\customer-reply.wav `
  --language zh-CN `
  --reply-text "我已听到您的问题，请确认是否创建工单。" `
  --device cpu `
  --compute-type int8
```

命令会输出 JSON，包含语音片段数、转写文本和 TTS 输出路径。首次运行会下载模型，Whisper 使用
`faster-whisper` 对 `openai/whisper-large-v3-turbo` 权重的 CTranslate2 转换版本。

如果只想先验证安装，可使用本次已生成的合成输入
`D:\work\helpdesk\tmp\ai-phone-local-smoke\kokoro-input.wav`；该文件包含“设备无法启动，请帮我创建工单”的中文语音。

Kokoro 当前配置了中文 `zf_xiaobei`、美式英语 `af_heart` 和英式英语 `bf_emma` 音色；阿拉伯语不能静默降级到错误音色，会明确报错。当前 Telnyx Voice API 电话链路仍由 Telnyx 承担 ASR/TTS；要让本地模型进入真实电话，后续需要接入原始媒体流。这个 CLI 只验证本地 WAV，不创建工单，也不调用 SIP、Telnyx 或 sub2api。

## 本地麦克风对话测试

安装麦克风采集和播放依赖：

```powershell
cd D:\work\helpdesk\services\ai_phone_agent
.\.venv-ai-phone\Scripts\python.exe -m pip install -e ".[local-voice,local-mic]"
```

先查看设备编号：

```powershell
.\.venv-ai-phone\Scripts\python.exe -m ai_phone_agent.local_voice_mic --list-devices
```

将示例复制为本地配置文件，填写 sub2api 的 OpenAI 兼容地址、普通用户 API Key 和可用模型（不要使用管理 API Key）。程序启动时会自动读取 `.env.local`；命令行参数和 PowerShell 已存在的环境变量优先级更高：

```powershell
Copy-Item .env.local.example .env.local
notepad .env.local

$env:HF_HOME = "D:\work\helpdesk\.local\hf-cache"
$env:HF_HUB_CACHE = "$env:HF_HOME\hub"
$env:PYTHONIOENCODING = "utf-8"
```

也可以使用其他配置文件：`--env-file D:\path\to\another.env`。配置文件只支持简单的
`KEY=value` 格式，缺失文件会被忽略；`.env.local` 已加入 Git 忽略规则。

启动智能对话（麦克风和扬声器可以使用不同设备编号；下面使用列表中的麦克风 1、扬声器 3）：

```powershell
chcp 65001 > $null
.\.venv-ai-phone\Scripts\python.exe -m ai_phone_agent.local_voice_mic `
  --smart-reply `
  --input-device 1 `
  --output-device 3 `
  --language zh-CN `
  --device cpu `
  --compute-type int8
```

程序提示后按回车开始说话，再按回车结束本轮；它会执行 VAD、Whisper 转写、sub2api 智能回答、Kokoro 合成并从扬声器播放，然后进入下一轮。加 `--once` 只测试一轮，按 `Ctrl+C` 退出。省略 `--smart-reply` 时只会把转写文本复述出来，不会请求云端大模型；`--smart-reply` 模式不接 SIP，也不创建工单。

## 直接启动本地语音客服（免按键）

如果不想每轮按回车，可在 Windows 资源管理器中双击仓库根目录的
`scripts\start-ai-phone-voice.cmd`。脚本会检查 8083 Go 服务，然后自动启动
“麦克风 → Silero VAD → Whisper → Helpdesk 智能客服 → Kokoro TTS → 扬声器”链路。
如果 8083 尚未运行，脚本会自动打开一个 Go 服务窗口并等待健康检查；如果已经运行，
会直接复用现有进程，不会重复占用端口。
启动后直接说话，检测到停顿会自动提交当前轮次；说“确认创建工单”后才会调用 Go 建单接口。
按 `Ctrl+C` 退出。

脚本默认使用系统输入/输出设备。如果需要固定设备，可在
`services\ai_phone_agent\.env.local` 设置 `AI_PHONE_INPUT_DEVICE` 和
`AI_PHONE_OUTPUT_DEVICE`，例如 `1` 和 `3`。这条链路使用本地麦克风，不代表 SIP
电话已经接通；真实电话仍需另外配置 Telnyx/FreeSWITCH。若配置了
`AI_PHONE_TASK_DATABASE_URL` 但虚拟环境尚未安装 `psycopg`，启动脚本会为本地验收
自动使用内存任务存储；工单仍会真实调用 Go，但结果未知任务不会持久化。安装
`psycopg[binary]` 后可通过 `AI_PHONE_TASK_STORE=auto` 切回 PostgreSQL。

## 本地麦克风接入 Helpdesk 智能客服

要让本地麦克风调用 Helpdesk 已配置的租户大模型，并在客户确认后真实调用 Go 建单接口，先确认 8083 服务已启动，并在 Go 进程中配置 `AI_PHONE_SERVICE_TOKEN`、`AI_PHONE_LINE_TENANTS`（例如 `{"line-1":14}`）。Python 不保存大模型密钥，只携带线路服务令牌：

```powershell
$env:HELPDESK_GO_URL = "http://127.0.0.1:8083"
$env:AI_PHONE_SERVICE_TOKEN = "<与 Go 服务相同的至少 32 位令牌>"
$env:AI_PHONE_LINE_ID = "line-1"
$env:AI_PHONE_PROJECT_KEY = "support"

chcp 65001 > $null
.\.venv-ai-phone\Scripts\python.exe -m ai_phone_agent.local_voice_mic `
  --helpdesk-reply `
  --input-device 1 `
  --output-device 3 `
  --language zh-CN `
  --device cpu `
  --compute-type int8
```

每轮流程是“本地 VAD → Whisper 转写 → Go `/api/third/ai-phone/decisions` → Kokoro 播报”。当 Go 返回 `prepare_ticket` 后说“确认”，Python 才调用 `/api/third/ai-phone/tickets`；命令行会播报并打印返回的工单号。`--once` 只跑一轮，建单完成、取消或失败后会话结束。需要持久化结果未知任务时再配置 `AI_PHONE_TASK_DATABASE_URL` 并单独启动恢复 Worker。

文本 FastAPI 接口也可以切换到同一决策客户端：设置 `AI_PHONE_DECISION_ENGINE=helpdesk` 后启动 `uvicorn`；不设置或设为 `rule` 时使用离线规则引擎。

恢复 Worker 只处理已经确认但建单结果未知的任务。它读取任务表中冻结的会话和工单草稿，用同一个 `provider:call_id:intent_id` 幂等键重试。不要为每个任务重新生成幂等键或重新拼接描述。

## Telnyx 电话接入（规格第 13 节）

Telnyx 号码绑定到 Voice API Application，Webhook URL 指向 `https://<公网地址>/v1/telnyx/webhook`（API 版本 v2）。Telnyx 负责接电话、语音转写和播报，Agent 只处理文字轮次。

额外配置：

```text
TELNYX_API_KEY=<Telnyx API Key，用于 answer/speak/transcription_start/hangup>
TELNYX_PUBLIC_KEY=<Portal 中的 Webhook 公钥，Base64>
TELNYX_WEBHOOK_TOKEN=<账户无法读取公钥时使用，至少 32 位随机值，Webhook URL 末尾加 ?token=...>
# 公钥和令牌都未配置时，所有 Webhook 返回 403
TELNYX_NUMBER_LINES={"+12025550100":{"line_id":"line-1","project_key":"<项目key>","language":"zh-CN"}}
TELNYX_TTS_VOICES={"zh-CN":"AWS.Polly.Zhiyu-Neural"}   # 可选，覆盖默认音色
```

`line_id` 必须同时出现在 Go 端的 `AI_PHONE_LINE_TENANTS` 中。本机测试可用 `cloudflared tunnel --url http://127.0.0.1:8090` 获得临时公网 HTTPS 地址。

## 适配器边界

`adapters.py` 定义 ASR、TTS 和翻译协议。接入 FreeSWITCH 时新增 RTP/AudioSocket 适配器即可，核心状态机不需要知道 SIP 厂商。真实 LLM 也应实现 `DecisionEngine`，并保留结构化 `AgentDecision` 契约。

## 生产接入顺序

1. 配置中国大陆 SIP 中继和 FreeSWITCH，统一输出 G.711 PCMA/PCMU。
2. 将 FreeSWITCH 音频桥接到 ASR，TTS 音频回注；实现打断、DTMF、静音和挂机事件。
3. 使用 `postgres` extra 和 `sql/001_ticket_tasks.sql`，将建单任务从内存切换到 PostgreSQL。
4. 增加结果未知任务扫描 Worker，使用同一个 `provider:call_id:intent_id` 幂等键查询/重试。
5. 完成中文、英文、阿拉伯语和 5 路并发验收后，再将规格状态改为 Accepted。
