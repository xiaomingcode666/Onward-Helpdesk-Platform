import unittest

from ai_phone_agent.local_voice_cli import build_parser


class LocalVoiceCLIParserTest(unittest.TestCase):
    def test_requires_input_and_output_and_defaults_to_chinese(self) -> None:
        args = build_parser().parse_args(["--input", "in.wav", "--output", "out.wav"])

        self.assertEqual(args.input, "in.wav")
        self.assertEqual(args.output, "out.wav")
        self.assertEqual(args.language, "zh-CN")
        self.assertEqual(args.asr_model, "openai/whisper-large-v3-turbo")
        self.assertEqual(args.tts_model, "hexgrad/Kokoro-82M")
