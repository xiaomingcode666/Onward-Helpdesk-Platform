from __future__ import annotations

import io
import wave
from dataclasses import dataclass
from typing import Protocol, Sequence


@dataclass(frozen=True, slots=True)
class SpeechSpan:
    """A speech interval expressed as zero-based PCM frame indexes."""

    start_sample: int
    end_sample: int

    def __post_init__(self) -> None:
        if self.start_sample < 0 or self.end_sample <= self.start_sample:
            raise ValueError("speech span must have a positive, non-negative range")


class VoiceActivityDetector(Protocol):
    def detect(
        self,
        pcm: bytes,
        *,
        sample_rate: int,
        channels: int,
        sample_width: int,
    ) -> Sequence[SpeechSpan]: ...


class AudioASR(Protocol):
    async def transcribe(
        self,
        pcm: bytes,
        *,
        sample_rate: int,
        channels: int,
        sample_width: int,
        language: str,
    ) -> str: ...


class AudioTTS(Protocol):
    async def synthesize(self, text: str, *, language: str) -> bytes: ...


@dataclass(frozen=True, slots=True)
class LocalVoiceResult:
    transcript: str
    reply_text: str
    speech_segments: int
    tts_audio: bytes


@dataclass(slots=True)
class LocalVoicePipeline:
    """Run a local WAV through VAD, ASR and TTS.

    The pipeline deliberately accepts WAV bytes rather than a SIP/RTP stream.
    This keeps model validation independent from the later Telnyx media
    integration and makes the same adapters usable from a microphone runner.
    """

    vad: VoiceActivityDetector
    asr: AudioASR
    tts: AudioTTS

    async def run(
        self,
        wav_audio: bytes,
        *,
        language: str,
        reply_text: str | None = None,
    ) -> LocalVoiceResult:
        pcm, sample_rate, channels, sample_width = _decode_wav(wav_audio)
        frame_width = channels * sample_width
        frame_count = len(pcm) // frame_width
        spans = tuple(self.vad.detect(
            pcm,
            sample_rate=sample_rate,
            channels=channels,
            sample_width=sample_width,
        ))

        transcripts: list[str] = []
        for span in spans:
            if span.end_sample > frame_count:
                raise ValueError("speech span exceeds WAV frame count")
            start = span.start_sample * frame_width
            end = span.end_sample * frame_width
            text = await self.asr.transcribe(
                pcm[start:end],
                sample_rate=sample_rate,
                channels=channels,
                sample_width=sample_width,
                language=language,
            )
            if text.strip():
                transcripts.append(text.strip())

        transcript = " ".join(transcripts)
        response = transcript if reply_text is None else reply_text.strip()
        tts_audio = await self.tts.synthesize(response, language=language) if response else b""
        return LocalVoiceResult(
            transcript=transcript,
            reply_text=response,
            speech_segments=len(spans),
            tts_audio=tts_audio,
        )


def _decode_wav(wav_audio: bytes) -> tuple[bytes, int, int, int]:
    try:
        reader = wave.open(io.BytesIO(wav_audio), "rb")
    except (EOFError, wave.Error) as exc:
        raise ValueError("input must be a valid WAV file") from exc
    with reader:
        if reader.getcomptype() != "NONE":
            raise ValueError("input WAV must contain uncompressed PCM")
        channels = reader.getnchannels()
        sample_width = reader.getsampwidth()
        sample_rate = reader.getframerate()
        if channels <= 0 or sample_rate <= 0 or sample_width <= 0:
            raise ValueError("input WAV has invalid audio parameters")
        return reader.readframes(reader.getnframes()), sample_rate, channels, sample_width
