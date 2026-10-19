import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from ai_phone_agent.local_voice_mic import (
    HandsFreeMicrophoneRecorder,
    build_parser,
    extract_chat_text,
    load_env_file,
    sub2api_chat_endpoint,
)


class LocalVoiceMicCLIParserTest(unittest.TestCase):
    def test_defaults_to_push_to_talk_chinese_loop(self) -> None:
        args = build_parser().parse_args([])

        self.assertEqual(args.language, "zh-CN")
        self.assertTrue(args.env_file.endswith(".env.local"))
        self.assertFalse(args.smart_reply)
        self.assertFalse(args.once)
        self.assertEqual(args.sample_rate, 16000)

    def test_accepts_smart_reply_and_one_turn_mode(self) -> None:
        args = build_parser().parse_args(
            [
                "--smart-reply",
                "--once",
                "--max-seconds",
                "12",
                "--input-device",
                "1",
                "--output-device",
                "3",
            ]
        )

        self.assertTrue(args.smart_reply)
        self.assertTrue(args.once)
        self.assertEqual(args.max_seconds, 12.0)
        self.assertEqual(args.input_device, "1")
        self.assertEqual(args.output_device, "3")

    def test_accepts_helpdesk_reply_and_connection_options(self) -> None:
        args = build_parser().parse_args(
            [
                "--helpdesk-reply",
                "--line-id",
                "line-1",
                "--project-key",
                "support",
                "--call-id",
                "mic-test-1",
                "--decision-timeout",
                "20",
            ]
        )

        self.assertTrue(args.helpdesk_reply)
        self.assertFalse(args.smart_reply)
        self.assertEqual(args.line_id, "line-1")
        self.assertEqual(args.project_key, "support")
        self.assertEqual(args.call_id, "mic-test-1")
        self.assertEqual(args.decision_timeout, 20.0)

    def test_accepts_hands_free_recording_options(self) -> None:
        args = build_parser().parse_args(
            [
                "--helpdesk-reply",
                "--hands-free",
                "--idle-timeout",
                "12",
                "--silence-duration-ms",
                "850",
                "--speech-pad-ms",
                "150",
            ]
        )

        self.assertTrue(args.hands_free)
        self.assertEqual(args.idle_timeout, 12.0)
        self.assertEqual(args.silence_duration_ms, 850)
        self.assertEqual(args.speech_pad_ms, 150)

    def test_hands_free_requires_16khz_before_loading_audio_dependencies(self) -> None:
        recorder = HandsFreeMicrophoneRecorder(sample_rate=8000)

        with self.assertRaisesRegex(ValueError, "16000 Hz"):
            recorder.record_turn()

    def test_loads_local_env_file_without_overwriting_process_values(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            env_file = Path(directory) / ".env.local"
            env_file.write_text(
                'SUB2API_BASE_URL="https://example.test"\n'
                "SUB2API_DEFAULT_MODEL=demo-model\n",
                encoding="utf-8",
            )
            with patch.dict(os.environ, {"SUB2API_BASE_URL": "process-value"}, clear=False):
                load_env_file(env_file)
                self.assertEqual(os.environ["SUB2API_BASE_URL"], "process-value")
                self.assertEqual(os.environ["SUB2API_DEFAULT_MODEL"], "demo-model")


class Sub2APIResponseTest(unittest.TestCase):
    def test_extracts_openai_compatible_text(self) -> None:
        payload = {
            "choices": [{"message": {"role": "assistant", "content": "请先重启设备试试。"}}]
        }

        self.assertEqual(extract_chat_text(payload), "请先重启设备试试。")

    def test_builds_v1_endpoint_without_duplicate_version(self) -> None:
        self.assertEqual(
            sub2api_chat_endpoint("https://sub2api.example.com/v1/"),
            "https://sub2api.example.com/v1/chat/completions",
        )
