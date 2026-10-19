from __future__ import annotations

import argparse
import asyncio
import io
import json
import os
import queue
import sys
import time
import wave
from collections import deque
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Mapping
from uuid import uuid4

from .local_models import (
    KOKORO_MODEL_ID,
    WHISPER_MODEL_ID,
    KokoroTTSAdapter,
    SileroVADAdapter,
    WhisperASRAdapter,
)
from .local_voice import LocalVoicePipeline
from .go_client import GoTicketClient, HelpdeskDecisionClient, TicketClientError
from .models import AgentState, VoiceSession
from .service import VoiceAgentService
from .task_store import InMemoryTicketTaskStore, PostgresTicketTaskStore


class LocalMicDependencyError(RuntimeError):
    """Raised when the optional microphone/audio-output dependency is absent."""


DEFAULT_ENV_FILE = Path(__file__).resolve().parents[1] / ".env.local"


def load_env_file(path: str | Path | None) -> None:
    """Load simple ``KEY=value`` settings without overwriting the process environment.

    This intentionally supports only the small dotenv subset needed by the local
    microphone runner. Secrets stay in the local file and are never logged.
    """

    if path is None:
        return
    env_path = Path(path)
    if not env_path.is_file():
        return

    for raw_line in env_path.read_text(encoding="utf-8-sig").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        key = key.strip()
        value = value.strip()
        if not key:
            continue
        if len(value) >= 2 and value[0] == value[-1] and value[0] in {"'", '"'}:
            value = value[1:-1]
        os.environ.setdefault(key, value)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Talk to the local VAD/ASR/TTS voice agent with a microphone."
    )
    parser.add_argument("--language", default="zh-CN", help="zh-CN, en-US or en-GB")
    reply_group = parser.add_mutually_exclusive_group()
    reply_group.add_argument(
        "--smart-reply",
        action="store_true",
        help="ask the configured OpenAI-compatible sub2api model for each reply",
    )
    reply_group.add_argument(
        "--helpdesk-reply",
        action="store_true",
        help="send each turn to the Go Helpdesk decision and ticket APIs",
    )
    parser.add_argument("--once", action="store_true", help="record and answer one turn, then exit")
    parser.add_argument(
        "--hands-free",
        action="store_true",
        help="automatically detect speech and silence; no Enter key is required",
    )
    parser.add_argument("--sample-rate", type=int, default=16000)
    parser.add_argument("--channels", type=int, default=1)
    parser.add_argument("--max-seconds", type=float, default=30.0)
    parser.add_argument(
        "--idle-timeout",
        type=float,
        default=20.0,
        help="hands-free seconds to wait for speech before starting another turn",
    )
    parser.add_argument(
        "--silence-duration-ms",
        type=int,
        default=700,
        help="hands-free trailing silence needed to finish a turn",
    )
    parser.add_argument(
        "--speech-pad-ms",
        type=int,
        default=120,
        help="hands-free padding added around the detected speech",
    )
    parser.add_argument(
        "--input-device",
        help="sounddevice microphone device index or name; omit for the system default",
    )
    parser.add_argument(
        "--output-device",
        help="sounddevice speaker/headphone device index or name; omit for the system default",
    )
    parser.add_argument("--asr-model", default=WHISPER_MODEL_ID)
    parser.add_argument("--tts-model", default=KOKORO_MODEL_ID)
    parser.add_argument("--device", default="auto", help="Whisper device: auto, cpu or cuda")
    parser.add_argument("--compute-type", default="auto", help="Whisper compute type: auto, int8 or float16")
    parser.add_argument("--speed", type=float, default=1.0)
    parser.add_argument("--sub2api-base-url", help="overrides SUB2API_BASE_URL")
    parser.add_argument("--sub2api-api-key", help="overrides SUB2API_API_KEY/SUB2API_DEFAULT_KEY")
    parser.add_argument("--sub2api-model", help="overrides SUB2API_DEFAULT_MODEL/SUB2API_LLM_MODEL")
    parser.add_argument("--sub2api-timeout", type=float, default=40.0)
    parser.add_argument("--helpdesk-go-url", help="overrides HELPDESK_GO_URL")
    parser.add_argument("--helpdesk-token", help="overrides AI_PHONE_SERVICE_TOKEN")
    parser.add_argument("--line-id", help="phone line bound to the Helpdesk tenant")
    parser.add_argument("--provider", default="local-mic", help="provider value sent to Helpdesk")
    parser.add_argument("--caller-phone", default="", help="optional caller phone sent to Helpdesk")
    parser.add_argument("--project-key", help="overrides AI_PHONE_PROJECT_KEY")
    parser.add_argument("--call-id", help="stable call id; defaults to a generated local id")
    parser.add_argument("--intent-id", default="intent_001")
    parser.add_argument("--decision-timeout", type=float, default=40.0)
    parser.add_argument(
        "--task-store",
        choices=("auto", "memory", "postgres"),
        default=None,
        help="Helpdesk ticket task storage; auto uses PostgreSQL when configured",
    )
    parser.add_argument(
        "--env-file",
        default=str(DEFAULT_ENV_FILE),
        help="local dotenv file loaded before reading sub2api settings",
    )
    parser.add_argument("--list-devices", action="store_true", help="list microphone/output devices and exit")
    return parser


def sub2api_chat_endpoint(base_url: str) -> str:
    base = base_url.strip().rstrip("/")
    if not base:
        raise ValueError("sub2api base URL is required")
    if base.lower().endswith("/v1"):
        return f"{base}/chat/completions"
    return f"{base}/v1/chat/completions"


def extract_chat_text(payload: Mapping[str, Any]) -> str:
    choices = payload.get("choices")
    if not isinstance(choices, list) or not choices:
        raise ValueError("sub2api response has no choices")
    message = choices[0].get("message") if isinstance(choices[0], Mapping) else None
    content = message.get("content") if isinstance(message, Mapping) else None
    if isinstance(content, list):
        content = "".join(
            item.get("text", "") for item in content if isinstance(item, Mapping)
        )
    if not isinstance(content, str) or not content.strip():
        raise ValueError("sub2api response has empty assistant content")
    return content.strip()


def _language_instruction(language: str) -> str:
    code = language.strip().lower().replace("_", "-")
    if code.startswith("zh"):
        return "请使用简体中文"
    if code.startswith("en"):
        return "Please answer in English"
    raise ValueError(f"smart reply language is not configured: {language}")


@dataclass(slots=True)
class Sub2APIChatResponder:
    base_url: str
    api_key: str
    model: str
    timeout_seconds: float = 40.0
    history: list[dict[str, str]] = field(default_factory=list)

    async def reply(self, transcript: str, *, language: str) -> str:
        text = transcript.strip()
        if not text:
            return ""
        try:
            import httpx
        except ImportError as exc:  # pragma: no cover - httpx is a base dependency
            raise LocalMicDependencyError("sub2api replies require httpx") from exc

        system = (
            "你是本地麦克风测试用的智能客服。"
            f"{_language_instruction(language)}，用自然、简短的口语回答。"
            "只回答用户当前问题；信息不足时先澄清。"
            "不要输出 Markdown、JSON、工单号，也不要声称已经执行了外部操作。"
            "回答控制在 80 个字以内。"
        )
        messages = [{"role": "system", "content": system}]
        messages.extend(self.history[-10:])
        messages.append({"role": "user", "content": text})
        headers = {
            "Accept": "application/json",
            "Content-Type": "application/json",
            "X-Api-Key": self.api_key,
            "Authorization": f"Bearer {self.api_key}",
        }
        payload = {
            "model": self.model,
            "messages": messages,
            "stream": False,
        }
        async with httpx.AsyncClient(timeout=self.timeout_seconds) as client:
            response = await client.post(
                sub2api_chat_endpoint(self.base_url),
                headers=headers,
                json=payload,
            )
            response.raise_for_status()
            answer = extract_chat_text(response.json())
        self.history.extend(
            [
                {"role": "user", "content": text},
                {"role": "assistant", "content": answer},
            ]
        )
        return answer


@dataclass(slots=True)
class MicrophoneRecorder:
    sample_rate: int = 16000
    channels: int = 1
    max_seconds: float = 30.0
    device: int | str | None = None

    def record_turn(self) -> bytes:
        np, sd = _audio_modules()
        print("按回车开始录音。")
        input()
        chunks: list[Any] = []

        def callback(indata: Any, _frames: int, _time_info: Any, status: Any) -> None:
            if status:
                print(f"麦克风状态：{status}", file=sys.stderr)
            chunks.append(indata.copy())

        print("请说话；说完后按回车结束本轮。")
        with sd.InputStream(
            samplerate=self.sample_rate,
            channels=self.channels,
            dtype="int16",
            device=self.device,
            callback=callback,
        ):
            _wait_for_enter_or_timeout(self.max_seconds)

        if not chunks:
            return _pcm_to_wav(b"", sample_rate=self.sample_rate, channels=self.channels)
        samples = np.concatenate(chunks, axis=0)
        max_frames = max(1, round(self.sample_rate * self.max_seconds))
        samples = samples[:max_frames]
        return _pcm_to_wav(
            samples.astype(np.int16, copy=False).tobytes(),
            sample_rate=self.sample_rate,
            channels=self.channels,
        )


@dataclass(slots=True)
class HandsFreeMicrophoneRecorder:
    """Capture one utterance without a keyboard using Silero VAD.

    The stream is kept open only for one utterance.  A new VAD iterator is
    created for every turn, so state from a previous speaker turn cannot leak
    into the next one.  The resulting WAV is still passed through the normal
    pipeline, which runs Silero VAD once more before Whisper transcription.
    """

    sample_rate: int = 16000
    channels: int = 1
    max_seconds: float = 30.0
    idle_timeout: float = 20.0
    min_silence_duration_ms: int = 700
    speech_pad_ms: int = 120
    device: int | str | None = None
    vad: SileroVADAdapter | None = None

    def record_turn(self) -> bytes:
        if self.sample_rate != 16000:
            raise ValueError("hands-free microphone mode currently requires a 16000 Hz sample rate")
        if self.channels <= 0:
            raise ValueError("channels must be positive")
        if self.max_seconds <= 0 or self.idle_timeout <= 0:
            raise ValueError("max seconds and idle timeout must be positive")
        if self.min_silence_duration_ms <= 0 or self.speech_pad_ms < 0:
            raise ValueError("silence duration must be positive and speech padding cannot be negative")

        np, sd = _audio_modules()
        try:
            import torch
            from silero_vad import VADIterator
        except ImportError as exc:
            raise LocalMicDependencyError(
                "hands-free microphone mode requires torch and silero-vad"
            ) from exc

        vad_adapter = self.vad or SileroVADAdapter()
        iterator = VADIterator(
            vad_adapter._load_model(),
            threshold=vad_adapter.threshold,
            sampling_rate=self.sample_rate,
            min_silence_duration_ms=self.min_silence_duration_ms,
            speech_pad_ms=self.speech_pad_ms,
        )
        block_frames = 512
        audio_queue: queue.Queue[Any] = queue.Queue()

        def callback(indata: Any, _frames: int, _time_info: Any, status: Any) -> None:
            if status:
                print(f"麦克风状态：{status}", file=sys.stderr)
            audio_queue.put(indata.copy())

        print("正在监听，请直接说话；检测到停顿后会自动回答。")
        started_at = time.monotonic()
        speech_started = False
        speech_start_sample: int | None = None
        blocks: list[Any] = []

        with sd.InputStream(
            samplerate=self.sample_rate,
            channels=self.channels,
            dtype="int16",
            blocksize=block_frames,
            device=self.device,
            callback=callback,
        ):
            while time.monotonic() - started_at < self.max_seconds:
                if not speech_started and time.monotonic() - started_at >= self.idle_timeout:
                    print("等待语音超时，继续监听。")
                    return _pcm_to_wav(b"", sample_rate=self.sample_rate, channels=self.channels)
                try:
                    block = audio_queue.get(timeout=0.25)
                except queue.Empty:
                    continue

                block_array = np.asarray(block, dtype=np.int16)
                if block_array.ndim == 1:
                    block_array = block_array.reshape(-1, 1)
                if block_array.shape[1] != self.channels:
                    raise ValueError("microphone returned an unexpected channel count")
                blocks.append(block_array.copy())
                mono = block_array.astype(np.float32)
                if self.channels > 1:
                    mono = mono.mean(axis=1)
                mono = mono.reshape(-1) / 32768.0
                event = iterator(torch.from_numpy(mono))
                if not speech_started and event and "start" in event:
                    speech_started = True
                    speech_start_sample = int(event["start"])
                    print("检测到语音。")
                if speech_started and event and "end" in event:
                    print("检测到停顿，正在识别。")
                    break

        if not speech_started or speech_start_sample is None or not blocks:
            print("未检测到有效语音，继续监听。")
            return _pcm_to_wav(b"", sample_rate=self.sample_rate, channels=self.channels)

        samples = np.concatenate(blocks, axis=0)
        pre_roll_frames = round(self.sample_rate * self.speech_pad_ms / 1000)
        start_frame = max(0, speech_start_sample - pre_roll_frames)
        pcm = samples[start_frame:].astype(np.int16, copy=False).tobytes()
        return _pcm_to_wav(pcm, sample_rate=self.sample_rate, channels=self.channels)


def list_audio_devices() -> Any:
    _np, sd = _audio_modules()
    return sd.query_devices()


def play_wav(wav_audio: bytes, *, device: int | str | None = None) -> None:
    np, sd = _audio_modules()
    try:
        reader = wave.open(io.BytesIO(wav_audio), "rb")
    except (EOFError, wave.Error) as exc:
        raise ValueError("TTS output is not a valid WAV") from exc
    with reader:
        if reader.getcomptype() != "NONE" or reader.getsampwidth() != 2:
            raise ValueError("TTS playback requires uncompressed PCM16 WAV")
        channels = reader.getnchannels()
        samples = np.frombuffer(reader.readframes(reader.getnframes()), dtype=np.int16)
        if channels > 1:
            samples = samples.reshape(-1, channels)
        sd.play(samples, samplerate=reader.getframerate(), device=device)
        sd.wait()


def _audio_modules() -> tuple[Any, Any]:
    try:
        import numpy as np
        import sounddevice as sd
    except ImportError as exc:
        raise LocalMicDependencyError(
            "microphone mode requires sounddevice; install with "
            "python -m pip install -e \".[local-voice,local-mic]\""
        ) from exc
    return np, sd


def _pcm_to_wav(pcm: bytes, *, sample_rate: int, channels: int) -> bytes:
    output = io.BytesIO()
    with wave.open(output, "wb") as wav:
        wav.setnchannels(channels)
        wav.setsampwidth(2)
        wav.setframerate(sample_rate)
        wav.writeframes(pcm)
    return output.getvalue()


def _wait_for_enter_or_timeout(max_seconds: float) -> None:
    if os.name != "nt":
        input()
        return
    import msvcrt

    deadline = time.monotonic() + max_seconds
    while time.monotonic() < deadline:
        if msvcrt.kbhit():
            key = msvcrt.getwch()
            if key in {"\r", "\n"}:
                print()
                return
        time.sleep(0.05)
    print("\n已达到录音时长上限。")


def _device_value(value: str | None) -> int | str | None:
    if value is None:
        return None
    return int(value) if value.isdigit() else value


def _sub2api_responder(args: argparse.Namespace) -> Sub2APIChatResponder:
    base_url = args.sub2api_base_url or os.getenv("SUB2API_BASE_URL", "")
    api_key = args.sub2api_api_key or os.getenv("SUB2API_API_KEY") or os.getenv("SUB2API_DEFAULT_KEY", "")
    model = (
        args.sub2api_model
        or os.getenv("SUB2API_DEFAULT_MODEL")
        or os.getenv("SUB2API_LLM_MODEL")
        or ""
    )
    if not base_url or not api_key or not model:
        raise RuntimeError(
            "--smart-reply requires SUB2API_BASE_URL, a user API key "
            "(SUB2API_API_KEY or SUB2API_DEFAULT_KEY), and SUB2API_DEFAULT_MODEL "
            "(or the corresponding command-line options)"
        )
    if args.sub2api_timeout <= 0:
        raise ValueError("sub2api timeout must be positive")
    return Sub2APIChatResponder(base_url, api_key, model, args.sub2api_timeout)


def _helpdesk_agent(args: argparse.Namespace) -> tuple[VoiceAgentService, VoiceSession, Any]:
    base_url = args.helpdesk_go_url or os.getenv("HELPDESK_GO_URL", "http://127.0.0.1:8083")
    token = args.helpdesk_token or os.getenv("AI_PHONE_SERVICE_TOKEN", "")
    line_id = args.line_id or os.getenv("AI_PHONE_LINE_ID", "line-1")
    project_key = args.project_key or os.getenv("AI_PHONE_PROJECT_KEY", "support")
    call_id = args.call_id or os.getenv("AI_PHONE_CALL_ID") or f"local-mic-{uuid4().hex}"
    if not token:
        raise RuntimeError(
            "--helpdesk-reply requires AI_PHONE_SERVICE_TOKEN (or --helpdesk-token)"
        )
    if not line_id or not project_key:
        raise RuntimeError("--helpdesk-reply requires a line id and project key")
    if args.decision_timeout <= 0:
        raise ValueError("decision timeout must be positive")
    task_dsn = os.getenv("AI_PHONE_TASK_DATABASE_URL", "").strip()
    task_store_mode = args.task_store or os.getenv("AI_PHONE_TASK_STORE", "auto")
    if task_store_mode == "memory":
        task_store: Any = InMemoryTicketTaskStore()
    elif task_store_mode == "postgres":
        if not task_dsn:
            raise RuntimeError("--task-store postgres requires AI_PHONE_TASK_DATABASE_URL")
        task_store = PostgresTicketTaskStore(task_dsn)
    else:
        task_store = PostgresTicketTaskStore(task_dsn) if task_dsn else InMemoryTicketTaskStore()
    agent = VoiceAgentService(
        HelpdeskDecisionClient(base_url, token, timeout=args.decision_timeout),
        GoTicketClient(base_url, token),
        task_store=task_store,
    )
    session = VoiceSession(
        call_id=call_id,
        provider=args.provider,
        line_id=line_id,
        caller_phone=args.caller_phone or os.getenv("AI_PHONE_CALLER_PHONE", ""),
        language=args.language,
        work_language=os.getenv("AI_PHONE_WORK_LANGUAGE", args.language),
        project_key=project_key,
        intent_id=args.intent_id,
    )
    agent.start(session)
    return agent, session, task_store


async def run(args: argparse.Namespace) -> int:
    load_env_file(args.env_file)
    if args.list_devices:
        print(list_audio_devices())
        return 0
    if args.sample_rate <= 0 or args.channels <= 0 or args.max_seconds <= 0:
        raise ValueError("sample rate, channels and max seconds must be positive")

    input_device = _device_value(args.input_device or os.getenv("AI_PHONE_INPUT_DEVICE"))
    output_device = _device_value(args.output_device or os.getenv("AI_PHONE_OUTPUT_DEVICE"))
    vad = SileroVADAdapter()
    if args.hands_free:
        recorder: MicrophoneRecorder | HandsFreeMicrophoneRecorder = HandsFreeMicrophoneRecorder(
            sample_rate=args.sample_rate,
            channels=args.channels,
            max_seconds=args.max_seconds,
            idle_timeout=args.idle_timeout,
            min_silence_duration_ms=args.silence_duration_ms,
            speech_pad_ms=args.speech_pad_ms,
            device=input_device,
            vad=vad,
        )
    else:
        recorder = MicrophoneRecorder(
            sample_rate=args.sample_rate,
            channels=args.channels,
            max_seconds=args.max_seconds,
            device=input_device,
        )
    tts = KokoroTTSAdapter(model_id=args.tts_model, speed=args.speed)
    pipeline = LocalVoicePipeline(
        vad,
        WhisperASRAdapter(
            model_id=args.asr_model,
            device=args.device,
            compute_type=args.compute_type,
        ),
        tts,
    )
    responder = _sub2api_responder(args) if args.smart_reply else None
    helpdesk_agent: VoiceAgentService | None = None
    helpdesk_session: VoiceSession | None = None
    task_store: Any | None = None
    if args.helpdesk_reply:
        helpdesk_agent, helpdesk_session, task_store = _helpdesk_agent(args)
    print("麦克风测试已启动。按 Ctrl+C 退出。")
    try:
        while True:
            wav_audio = await asyncio.to_thread(recorder.record_turn)
            terminal = False
            ticket = None
            decision = None
            if responder is None and helpdesk_agent is None:
                result = await pipeline.run(wav_audio, language=args.language)
                reply = result.reply_text
                tts_audio = result.tts_audio
            else:
                result = await pipeline.run(wav_audio, language=args.language, reply_text="")
                if args.hands_free and not result.transcript:
                    # An idle timeout is normal for an always-listening runner;
                    # do not turn it into a spoken "please repeat" response.
                    continue
                if not result.transcript:
                    reply = "没有听清您的问题，请再说一次。"
                elif helpdesk_agent is not None and helpdesk_session is not None:
                    print("正在请求 Helpdesk 智能客服……")
                    try:
                        decision, ticket = await helpdesk_agent.handle_text(
                            helpdesk_session.call_id, result.transcript
                        )
                        reply = decision.reply_text
                        if ticket is not None and ticket.ticket_no:
                            reply = f"{reply} 工单号是 {ticket.ticket_no}。"
                    except TicketClientError as exc:
                        reply = (
                            "系统暂时无法确认工单结果，我们已记录您的问题，稍后会联系您。"
                            if exc.result_unknown
                            else "工单暂时创建失败，请稍后再试。"
                        )
                    terminal = helpdesk_session.state in {
                        AgentState.COMPLETED,
                        AgentState.CANCELLED,
                        AgentState.FAILED,
                    }
                else:
                    print("正在请求 sub2api 智能回答……")
                    reply = await responder.reply(result.transcript, language=args.language)
                tts_audio = await tts.synthesize(reply, language=args.language)
            output: dict[str, object] = {
                "transcript": result.transcript,
                "reply_text": reply,
                "speech_segments": result.speech_segments,
            }
            if decision is not None:
                output["action"] = decision.action.value
                output["collected_fields"] = decision.collected_fields
                output["missing_fields"] = decision.missing_fields
            if ticket is not None:
                output["ticket"] = ticket.model_dump()
            print(json.dumps(output, ensure_ascii=False))
            if tts_audio:
                print("正在播放回答……")
                await asyncio.to_thread(play_wav, tts_audio, device=output_device)
            if args.once or terminal:
                return 0
    finally:
        if task_store is not None:
            task_store.close()


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        return asyncio.run(run(args))
    except KeyboardInterrupt:
        print("\n麦克风测试已退出。")
        return 0
    except (LocalMicDependencyError, OSError, RuntimeError, ValueError) as exc:
        print(f"local microphone pipeline failed: {exc}", file=sys.stderr)
        return 2
    except Exception as exc:
        # sounddevice keeps PortAudioError optional, so avoid importing it at
        # module load time.  Turn the common Windows device-open failure into
        # an actionable message instead of a long traceback.
        if exc.__class__.__name__ == "PortAudioError":
            print(
                "无法打开音频设备。请先运行 --list-devices，确认麦克风/扬声器编号，"
                "并关闭占用麦克风的其他程序后重试。",
                file=sys.stderr,
            )
            return 2
        raise


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
