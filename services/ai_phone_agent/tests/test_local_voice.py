import io
import wave
import unittest

from ai_phone_agent.local_voice import LocalVoicePipeline, SpeechSpan


def make_wav(frame_count: int = 20, sample_rate: int = 16000) -> bytes:
    pcm = b"".join((index.to_bytes(2, "little", signed=True) for index in range(frame_count)))
    output = io.BytesIO()
    with wave.open(output, "wb") as wav:
        wav.setnchannels(1)
        wav.setsampwidth(2)
        wav.setframerate(sample_rate)
        wav.writeframes(pcm)
    return output.getvalue()


class FakeVAD:
    def __init__(self, spans):
        self.spans = spans

    def detect(self, pcm, *, sample_rate, channels, sample_width):
        return self.spans


class FakeASR:
    def __init__(self):
        self.calls = []

    async def transcribe(self, pcm, *, sample_rate, channels, sample_width, language):
        self.calls.append((pcm, sample_rate, channels, sample_width, language))
        return f"片段{len(self.calls)}"


class FakeTTS:
    def __init__(self):
        self.calls = []

    async def synthesize(self, text, *, language):
        self.calls.append((text, language))
        return b"tts-wav"


class LocalVoicePipelineTest(unittest.IsolatedAsyncioTestCase):
    async def test_runs_vad_asr_and_tts_for_each_speech_span(self) -> None:
        asr = FakeASR()
        tts = FakeTTS()
        pipeline = LocalVoicePipeline(
            FakeVAD([SpeechSpan(2, 6), SpeechSpan(10, 14)]),
            asr,
            tts,
        )

        result = await pipeline.run(make_wav(), language="zh-CN")

        self.assertEqual(result.transcript, "片段1 片段2")
        self.assertEqual(result.reply_text, "片段1 片段2")
        self.assertEqual(result.speech_segments, 2)
        self.assertEqual(result.tts_audio, b"tts-wav")
        self.assertEqual([call[0] for call in tts.calls], ["片段1 片段2"])
        self.assertEqual([len(call[0]) for call in asr.calls], [8, 8])

    async def test_does_not_call_asr_or_tts_when_vad_finds_no_speech(self) -> None:
        asr = FakeASR()
        tts = FakeTTS()
        pipeline = LocalVoicePipeline(FakeVAD([]), asr, tts)

        result = await pipeline.run(make_wav(), language="en-US")

        self.assertEqual(result.transcript, "")
        self.assertEqual(result.reply_text, "")
        self.assertEqual(result.speech_segments, 0)
        self.assertEqual(result.tts_audio, b"")
        self.assertEqual(asr.calls, [])
        self.assertEqual(tts.calls, [])
