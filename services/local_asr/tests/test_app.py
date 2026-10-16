from __future__ import annotations

import io
import wave

from fastapi.testclient import TestClient

from local_asr.app import create_app
from local_asr.engine import LocalASRDependencyError


def wav_bytes(seconds: float = 1.0, sample_rate: int = 16000) -> bytes:
    buffer = io.BytesIO()
    with wave.open(buffer, "wb") as writer:
        writer.setnchannels(1)
        writer.setsampwidth(2)
        writer.setframerate(sample_rate)
        writer.writeframes(b"\x00\x00" * int(seconds * sample_rate))
    return buffer.getvalue()


class FakeEngine:
    device = "cuda"

    def __init__(self, text: str = "替身文本", error: Exception | None = None) -> None:
        self.text = text
        self.error = error
        self.calls: list[dict] = []

    def load(self) -> None:
        pass

    def transcribe(self, pcm: bytes, *, sample_rate: int, channels: int, sample_width: int, language: str) -> str:
        self.calls.append({
            "bytes": len(pcm),
            "sample_rate": sample_rate,
            "channels": channels,
            "sample_width": sample_width,
            "language": language,
        })
        if self.error is not None:
            raise self.error
        return self.text


def post_wav(client: TestClient, body: bytes, language: str | None = None):
    params = {"language": language} if language else None
    return client.post("/v1/asr/transcribe", content=body, params=params, headers={"Content-Type": "audio/wav"})


def test_transcribe_returns_text() -> None:
    engine = FakeEngine()
    with TestClient(create_app(engine)) as client:
        response = post_wav(client, wav_bytes(1.0))
    assert response.status_code == 200
    assert response.json() == {"text": "替身文本", "language": "zh-CN", "durationSeconds": 1.0}
    assert engine.calls == [{
        "bytes": 32000,
        "sample_rate": 16000,
        "channels": 1,
        "sample_width": 2,
        "language": "zh-CN",
    }]


def test_transcribe_passes_language() -> None:
    engine = FakeEngine()
    with TestClient(create_app(engine)) as client:
        response = post_wav(client, wav_bytes(0.5), language="en-US")
    assert response.status_code == 200
    assert engine.calls[0]["language"] == "en-US"


def test_rejects_non_wav() -> None:
    with TestClient(create_app(FakeEngine())) as client:
        response = post_wav(client, b"not wav")
    assert response.status_code == 400


def test_rejects_empty_audio() -> None:
    with TestClient(create_app(FakeEngine())) as client:
        response = post_wav(client, wav_bytes(0.0))
    assert response.status_code == 400


def test_dependency_error_maps_to_503() -> None:
    engine = FakeEngine(error=LocalASRDependencyError("faster-whisper missing"))
    with TestClient(create_app(engine)) as client:
        response = post_wav(client, wav_bytes(1.0))
    assert response.status_code == 503


def test_healthz_reports_device() -> None:
    with TestClient(create_app(FakeEngine())) as client:
        response = client.get("/healthz")
    assert response.json() == {"ok": True, "device": "cuda"}
