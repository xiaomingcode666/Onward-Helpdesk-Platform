from __future__ import annotations

import asyncio
import io
import wave
from dataclasses import dataclass, field
from typing import Any, Mapping

from .local_voice import SpeechSpan


WHISPER_MODEL_ID = "openai/whisper-large-v3-turbo"
KOKORO_MODEL_ID = "hexgrad/Kokoro-82M"


class LocalVoiceDependencyError(RuntimeError):
    """Raised when optional local voice dependencies are not installed."""


class UnsupportedLanguageError(ValueError):
    """Raised when a selected local model has no voice for a language."""


def _language_prefix(language: str) -> str:
    value = language.strip().lower().replace("_", "-")
    if not value:
        raise UnsupportedLanguageError("language is required")
    return value.split("-", 1)[0]


def whisper_language_code(language: str) -> str:
    code = _language_prefix(language)
    if code in {"zh", "en", "ar"}:
        return code
    raise UnsupportedLanguageError(f"Whisper language is not configured: {language}")


def kokoro_language_code(language: str) -> str:
    code = _language_prefix(language)
    if code == "zh":
        return "z"
    if code == "en":
        return "b" if language.lower().replace("_", "-").startswith("en-gb") else "a"
    raise UnsupportedLanguageError(
        "Kokoro-82M does not provide the configured local voice for "
        f"{language}; use a fallback TTS for Arabic"
    )


def kokoro_voice(language: str) -> str:
    code = _language_prefix(language)
    if code == "zh":
        return "zf_xiaobei"
    if code == "en":
        return "bf_emma" if language.lower().replace("_", "-").startswith("en-gb") else "af_heart"
    raise UnsupportedLanguageError(f"Kokoro voice is not configured: {language}")


@dataclass(slots=True)
class SileroVADAdapter:
    threshold: float = 0.5
    min_speech_duration_ms: int = 250
    min_silence_duration_ms: int = 100
    model: Any | None = None

    def _load_model(self) -> Any:
        if self.model is not None:
            return self.model
        try:
            from silero_vad import load_silero_vad
        except ImportError as exc:
            raise LocalVoiceDependencyError(
                "Silero VAD is not installed; install the local-voice dependencies"
            ) from exc
        self.model = load_silero_vad(onnx=False)
        return self.model

    def detect(
        self,
        pcm: bytes,
        *,
        sample_rate: int,
        channels: int,
        sample_width: int,
    ) -> list[SpeechSpan]:
        try:
            import numpy as np
            import torch
            from silero_vad import get_speech_timestamps
        except ImportError as exc:
            raise LocalVoiceDependencyError(
                "Silero VAD requires numpy, torch and silero-vad"
            ) from exc
        samples = _pcm_to_float32(pcm, channels=channels, sample_width=sample_width, np=np)
        vad_rate = sample_rate
        if sample_rate not in {8000, 16000}:
            # Silero only accepts 8 kHz or 16 kHz. Keep the public span
            # indexes in the source WAV's frame space so LocalVoicePipeline
            # can slice the original PCM without losing synchronization.
            samples = _resample(samples, sample_rate, 16000, np=np)
            vad_rate = 16000
        timestamps = get_speech_timestamps(
            torch.from_numpy(samples),
            self._load_model(),
            sampling_rate=vad_rate,
            threshold=self.threshold,
            min_speech_duration_ms=self.min_speech_duration_ms,
            min_silence_duration_ms=self.min_silence_duration_ms,
        )
        if vad_rate == sample_rate:
            return [SpeechSpan(int(item["start"]), int(item["end"])) for item in timestamps]
        scale = sample_rate / vad_rate
        source_frame_count = len(pcm) // (channels * sample_width)
        spans: list[SpeechSpan] = []
        for item in timestamps:
            start = min(source_frame_count, round(int(item["start"]) * scale))
            end = min(source_frame_count, round(int(item["end"]) * scale))
            if end > start:
                spans.append(SpeechSpan(start, end))
        return spans


@dataclass(slots=True)
class WhisperASRAdapter:
    """Whisper large-v3-turbo through the faster-whisper runtime."""

    model_id: str = WHISPER_MODEL_ID
    device: str = "auto"
    compute_type: str = "auto"
    model: Any | None = None

    def _load_model(self) -> Any:
        if self.model is not None:
            return self.model
        try:
            from faster_whisper import WhisperModel
        except ImportError as exc:
            raise LocalVoiceDependencyError(
                "faster-whisper is not installed; install the local-voice dependencies"
            ) from exc
        device, compute_type = _whisper_runtime_options(self.device, self.compute_type)
        # faster-whisper consumes the CTranslate2 conversion of the exact
        # openai/whisper-large-v3-turbo weights.
        runtime_model_id = "large-v3-turbo" if self.model_id == WHISPER_MODEL_ID else self.model_id
        self.model = WhisperModel(runtime_model_id, device=device, compute_type=compute_type)
        return self.model

    async def transcribe(
        self,
        pcm: bytes,
        *,
        sample_rate: int,
        channels: int,
        sample_width: int,
        language: str,
    ) -> str:
        return await asyncio.to_thread(
            self._transcribe_sync,
            pcm,
            sample_rate,
            channels,
            sample_width,
            language,
        )

    def _transcribe_sync(
        self,
        pcm: bytes,
        sample_rate: int,
        channels: int,
        sample_width: int,
        language: str,
    ) -> str:
        try:
            import numpy as np
        except ImportError as exc:
            raise LocalVoiceDependencyError("Whisper requires numpy") from exc
        samples = _pcm_to_float32(pcm, channels=channels, sample_width=sample_width, np=np)
        samples = _resample(samples, sample_rate, 16000, np=np)
        segments, _info = self._load_model().transcribe(
            samples,
            language=whisper_language_code(language),
            task="transcribe",
            beam_size=5,
            vad_filter=False,
            condition_on_previous_text=False,
        )
        return " ".join(segment.text.strip() for segment in segments if segment.text.strip())


@dataclass(slots=True)
class KokoroTTSAdapter:
    """Kokoro-82M TTS adapter returning a PCM16 WAV byte string."""

    model_id: str = KOKORO_MODEL_ID
    speed: float = 1.0
    sample_rate: int = 24000
    voices: Mapping[str, str] = field(default_factory=dict)
    pipelines: dict[str, Any] = field(default_factory=dict)

    def __post_init__(self) -> None:
        if not self.voices:
            self.voices = {
                "zh-CN": "zf_xiaobei",
                "en-US": "af_heart",
                "en-GB": "bf_emma",
            }

    async def synthesize(self, text: str, *, language: str) -> bytes:
        if not text.strip():
            return b""
        return await asyncio.to_thread(self._synthesize_sync, text, language)

    def _synthesize_sync(self, text: str, language: str) -> bytes:
        try:
            import numpy as np
            from kokoro import KPipeline
        except ImportError as exc:
            raise LocalVoiceDependencyError(
                "Kokoro TTS requires numpy and the kokoro package"
            ) from exc
        lang_code = kokoro_language_code(language)
        voice = self.voices.get(language) or kokoro_voice(language)
        pipeline = self.pipelines.get(lang_code)
        if pipeline is None:
            try:
                pipeline = KPipeline(lang_code=lang_code, repo_id=self.model_id)
            except TypeError:
                # Older kokoro releases use the default Hugging Face repo.
                pipeline = KPipeline(lang_code=lang_code)
            self.pipelines[lang_code] = pipeline
        chunks = []
        for item in pipeline(text, voice=voice, speed=self.speed, split_pattern=r"\n+"):
            audio = item[2]
            if hasattr(audio, "detach"):
                audio = audio.detach().cpu().numpy()
            chunks.append(np.asarray(audio, dtype=np.float32).reshape(-1))
        if not chunks:
            return b""
        waveform = np.concatenate(chunks)
        waveform = np.clip(waveform, -1.0, 1.0)
        pcm = (waveform * 32767.0).astype(np.int16).tobytes()
        output = io.BytesIO()
        with wave.open(output, "wb") as wav:
            wav.setnchannels(1)
            wav.setsampwidth(2)
            wav.setframerate(self.sample_rate)
            wav.writeframes(pcm)
        return output.getvalue()


def _whisper_runtime_options(device: str, compute_type: str) -> tuple[str, str]:
    if device != "auto":
        resolved_device = device
    else:
        try:
            import torch

            resolved_device = "cuda" if torch.cuda.is_available() else "cpu"
        except ImportError:
            resolved_device = "cpu"
    if compute_type != "auto":
        return resolved_device, compute_type
    return resolved_device, "float16" if resolved_device == "cuda" else "int8"


def _pcm_to_float32(pcm: bytes, *, channels: int, sample_width: int, np: Any) -> Any:
    if channels <= 0:
        raise ValueError("channels must be positive")
    if sample_width == 1:
        samples = (np.frombuffer(pcm, dtype=np.uint8).astype(np.float32) - 128.0) / 128.0
    elif sample_width == 2:
        samples = np.frombuffer(pcm, dtype=np.int16).astype(np.float32) / 32768.0
    elif sample_width == 4:
        samples = np.frombuffer(pcm, dtype=np.int32).astype(np.float32) / 2147483648.0
    else:
        raise ValueError("only 8-bit, 16-bit and 32-bit PCM are supported")
    if channels > 1:
        samples = samples[: len(samples) - (len(samples) % channels)]
        samples = samples.reshape(-1, channels).mean(axis=1)
    return samples


def _resample(samples: Any, source_rate: int, target_rate: int, *, np: Any) -> Any:
    if source_rate == target_rate or len(samples) == 0:
        return samples
    target_length = max(1, round(len(samples) * target_rate / source_rate))
    source_positions = np.linspace(0, len(samples) - 1, num=len(samples), endpoint=True)
    target_positions = np.linspace(0, len(samples) - 1, num=target_length, endpoint=True)
    return np.interp(target_positions, source_positions, samples).astype(np.float32)
