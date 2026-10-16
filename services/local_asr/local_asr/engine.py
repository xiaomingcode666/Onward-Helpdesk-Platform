"""faster-whisper engine used by the local ASR service."""

from __future__ import annotations

import logging
import threading
from typing import Any

logger = logging.getLogger(__name__)

# CTranslate2 conversion of openai/whisper-large-v3-turbo, resolved from the
# local Hugging Face cache (HF_HOME) when HF_HUB_OFFLINE=1.
DEFAULT_MODEL = "large-v3-turbo"


class LocalASRDependencyError(RuntimeError):
    """Raised when faster-whisper or numpy is not installed."""


def whisper_language(language: str) -> str:
    value = language.strip().lower().replace("_", "-")
    return value.split("-", 1)[0] or "zh"


class WhisperEngine:
    def __init__(self, model: str = DEFAULT_MODEL, device: str = "cuda", compute_type: str = "float16") -> None:
        self.model_name = model
        self.device = device
        self.compute_type = compute_type
        self._model: Any | None = None
        self._lock = threading.Lock()

    def load(self) -> None:
        with self._lock:
            if self._model is not None:
                return
            try:
                from faster_whisper import WhisperModel
            except ImportError as exc:
                raise LocalASRDependencyError("faster-whisper is not installed") from exc
            try:
                self._model = WhisperModel(self.model_name, device=self.device, compute_type=self.compute_type)
            except Exception as exc:  # noqa: BLE001 - CUDA/runtime errors vary by driver
                if self.device == "cpu":
                    raise
                logger.warning("CUDA unavailable for local ASR (%s); falling back to CPU int8", exc)
                self.device, self.compute_type = "cpu", "int8"
                self._model = WhisperModel(self.model_name, device="cpu", compute_type="int8")

    def transcribe(self, pcm: bytes, *, sample_rate: int, channels: int, sample_width: int, language: str) -> str:
        try:
            import numpy as np
        except ImportError as exc:
            raise LocalASRDependencyError("numpy is not installed") from exc
        self.load()
        samples = _pcm_to_mono_float32(pcm, channels=channels, sample_width=sample_width, np=np)
        if sample_rate != 16000:
            samples = _resample(samples, sample_rate, 16000, np=np)
        segments, _info = self._model.transcribe(
            samples,
            language=whisper_language(language),
            task="transcribe",
            beam_size=5,
            vad_filter=True,
            condition_on_previous_text=False,
        )
        return "".join(segment.text.strip() for segment in segments if segment.text.strip())


def _pcm_to_mono_float32(pcm: bytes, *, channels: int, sample_width: int, np: Any) -> Any:
    if sample_width != 2:
        raise ValueError("only 16-bit PCM is supported")
    samples = np.frombuffer(pcm, dtype="<i2").astype(np.float32) / 32768.0
    if channels > 1:
        samples = samples[: len(samples) - len(samples) % channels].reshape(-1, channels).mean(axis=1)
    return samples


def _resample(samples: Any, source_rate: int, target_rate: int, *, np: Any) -> Any:
    if source_rate == target_rate or len(samples) == 0:
        return samples
    target_length = max(1, round(len(samples) * target_rate / source_rate))
    positions = np.linspace(0, len(samples) - 1, target_length)
    return np.interp(positions, np.arange(len(samples)), samples).astype(np.float32)
