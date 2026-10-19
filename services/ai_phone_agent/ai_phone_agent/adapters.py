from __future__ import annotations

from dataclasses import dataclass
from typing import Protocol


class ASRAdapter(Protocol):
    async def transcribe(self, audio: bytes, *, language: str) -> str: ...


class TTSAdapter(Protocol):
    async def synthesize(self, text: str, *, language: str) -> bytes: ...


class TranslationAdapter(Protocol):
    async def translate(self, text: str, *, source_language: str, target_language: str) -> str: ...


@dataclass(slots=True)
class TextASRAdapter:
    """Adapter used by the text simulator and local tests."""

    async def transcribe(self, audio: bytes, *, language: str) -> str:
        return audio.decode("utf-8")


@dataclass(slots=True)
class NoopTTSAdapter:
    """Deterministic TTS substitute until a provider is approved."""

    async def synthesize(self, text: str, *, language: str) -> bytes:
        return text.encode("utf-8")


@dataclass(slots=True)
class PassthroughTranslationAdapter:
    async def translate(self, text: str, *, source_language: str, target_language: str) -> str:
        return text
