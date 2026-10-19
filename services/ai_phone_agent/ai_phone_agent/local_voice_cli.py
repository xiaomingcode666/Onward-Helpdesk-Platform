from __future__ import annotations

import argparse
import asyncio
import json
from pathlib import Path

from .local_models import (
    KOKORO_MODEL_ID,
    WHISPER_MODEL_ID,
    KokoroTTSAdapter,
    SileroVADAdapter,
    WhisperASRAdapter,
)
from .local_voice import LocalVoicePipeline


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Run a local WAV through Silero VAD, Whisper ASR and Kokoro TTS."
    )
    parser.add_argument("--input", required=True, help="input PCM WAV path")
    parser.add_argument("--output", required=True, help="output Kokoro WAV path")
    parser.add_argument("--language", default="zh-CN", help="zh-CN, en-US or ar")
    parser.add_argument("--reply-text", help="optional TTS reply; defaults to the transcript")
    parser.add_argument("--asr-model", default=WHISPER_MODEL_ID)
    parser.add_argument("--tts-model", default=KOKORO_MODEL_ID)
    parser.add_argument("--device", default="auto", help="auto, cpu or cuda")
    parser.add_argument("--compute-type", default="auto", help="auto, int8 or float16")
    parser.add_argument("--speed", type=float, default=1.0)
    return parser


async def run(args: argparse.Namespace) -> dict[str, object]:
    input_path = Path(args.input)
    output_path = Path(args.output)
    if not input_path.is_file():
        raise FileNotFoundError(f"input WAV does not exist: {input_path}")
    pipeline = LocalVoicePipeline(
        SileroVADAdapter(),
        WhisperASRAdapter(
            model_id=args.asr_model,
            device=args.device,
            compute_type=args.compute_type,
        ),
        KokoroTTSAdapter(model_id=args.tts_model, speed=args.speed),
    )
    result = await pipeline.run(
        input_path.read_bytes(),
        language=args.language,
        reply_text=args.reply_text,
    )
    if result.tts_audio:
        output_path.parent.mkdir(parents=True, exist_ok=True)
        output_path.write_bytes(result.tts_audio)
    return {
        "input": str(input_path),
        "output": str(output_path) if result.tts_audio else None,
        "language": args.language,
        "speech_segments": result.speech_segments,
        "transcript": result.transcript,
        "reply_text": result.reply_text,
    }


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        result = asyncio.run(run(args))
    except (FileNotFoundError, ValueError, RuntimeError) as exc:
        print(f"local voice pipeline failed: {exc}")
        return 2
    print(json.dumps(result, ensure_ascii=False))
    return 0


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
