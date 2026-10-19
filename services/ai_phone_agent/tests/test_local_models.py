import importlib.util
import unittest
from unittest.mock import patch

from ai_phone_agent.local_models import (
    KOKORO_MODEL_ID,
    SileroVADAdapter,
    WHISPER_MODEL_ID,
    UnsupportedLanguageError,
    kokoro_language_code,
    kokoro_voice,
    whisper_language_code,
)
from ai_phone_agent.local_voice import SpeechSpan


class LocalModelConfigurationTest(unittest.TestCase):
    def test_uses_the_selected_asr_and_tts_models(self) -> None:
        self.assertEqual(WHISPER_MODEL_ID, "openai/whisper-large-v3-turbo")
        self.assertEqual(KOKORO_MODEL_ID, "hexgrad/Kokoro-82M")

    def test_maps_supported_languages_to_whisper_and_kokoro(self) -> None:
        self.assertEqual(whisper_language_code("zh-CN"), "zh")
        self.assertEqual(whisper_language_code("en-US"), "en")
        self.assertEqual(whisper_language_code("ar"), "ar")
        self.assertEqual(kokoro_language_code("zh-CN"), "z")
        self.assertEqual(kokoro_language_code("en-US"), "a")
        self.assertEqual(kokoro_voice("zh-CN"), "zf_xiaobei")
        self.assertEqual(kokoro_voice("en-US"), "af_heart")

    def test_rejects_arabic_for_kokoro_instead_of_silently_using_wrong_voice(self) -> None:
        with self.assertRaises(UnsupportedLanguageError):
            kokoro_language_code("ar")

    @unittest.skipUnless(importlib.util.find_spec("numpy"), "local-voice dependencies are not installed")
    def test_resamples_common_wav_rate_before_silero_and_maps_spans_back(self) -> None:
        adapter = SileroVADAdapter(model=object())
        pcm = b"\x00\x00" * 24000
        with patch(
            "silero_vad.get_speech_timestamps",
            return_value=[{"start": 0, "end": 16000}],
        ) as get_timestamps:
            spans = adapter.detect(
                pcm,
                sample_rate=24000,
                channels=1,
                sample_width=2,
            )

        self.assertEqual(spans, [SpeechSpan(0, 24000)])
        self.assertEqual(get_timestamps.call_args.kwargs["sampling_rate"], 16000)
        self.assertEqual(get_timestamps.call_args.args[0].shape[0], 16000)
