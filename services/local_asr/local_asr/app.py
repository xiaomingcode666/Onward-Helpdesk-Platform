"""Loopback-only HTTP API that transcribes PCM16 WAV with a local Whisper model.

Run:
    HF_HOME=<repo>/.local/hf-cache HF_HUB_OFFLINE=1 \
        uvicorn local_asr.app:app --host 127.0.0.1 --port 8791
"""

from __future__ import annotations

import asyncio
import io
import os
import wave
from contextlib import asynccontextmanager
from typing import Protocol

from fastapi import FastAPI, HTTPException, Request

from .engine import LocalASRDependencyError, WhisperEngine

MAX_AUDIO_BYTES = 25 << 20
DEFAULT_LANGUAGE = "zh-CN"


class Engine(Protocol):
    device: str

    def load(self) -> None: ...

    def transcribe(self, pcm: bytes, *, sample_rate: int, channels: int, sample_width: int, language: str) -> str: ...


def default_engine() -> WhisperEngine:
    return WhisperEngine(
        model=os.getenv("RHD_ASR_MODEL", "large-v3-turbo"),
        device=os.getenv("RHD_ASR_DEVICE", "cuda"),
        compute_type=os.getenv("RHD_ASR_COMPUTE_TYPE", "float16"),
    )


def create_app(engine: Engine | None = None) -> FastAPI:
    @asynccontextmanager
    async def lifespan(app: FastAPI):
        if app.state.engine is None:
            app.state.engine = default_engine()
        # Preload so the first customer request does not wait for model load.
        try:
            await asyncio.to_thread(app.state.engine.load)
        except LocalASRDependencyError:
            pass
        yield

    app = FastAPI(title="Onward Local ASR", version="0.1.0", lifespan=lifespan)
    app.state.engine = engine

    @app.get("/healthz")
    async def healthz() -> dict:
        return {"ok": True, "device": app.state.engine.device}

    @app.post("/v1/asr/transcribe")
    async def transcribe(request: Request, language: str = DEFAULT_LANGUAGE) -> dict:
        body = await request.body()
        if len(body) > MAX_AUDIO_BYTES:
            raise HTTPException(status_code=413, detail="audio is too large")
        try:
            with wave.open(io.BytesIO(body), "rb") as reader:
                channels = reader.getnchannels()
                sample_width = reader.getsampwidth()
                sample_rate = reader.getframerate()
                frames = reader.getnframes()
                pcm = reader.readframes(frames)
        except (wave.Error, EOFError) as exc:
            raise HTTPException(status_code=400, detail="audio must be a PCM WAV file") from exc
        if frames <= 0 or not pcm:
            raise HTTPException(status_code=400, detail="audio is empty")
        if sample_width != 2:
            raise HTTPException(status_code=400, detail="audio must be 16-bit PCM")
        try:
            text = await asyncio.to_thread(
                app.state.engine.transcribe,
                pcm,
                sample_rate=sample_rate,
                channels=channels,
                sample_width=sample_width,
                language=language,
            )
        except LocalASRDependencyError as exc:
            raise HTTPException(status_code=503, detail=str(exc)) from exc
        return {
            "text": text.strip(),
            "language": language,
            "durationSeconds": round(frames / sample_rate, 3),
        }

    return app


app = create_app()
