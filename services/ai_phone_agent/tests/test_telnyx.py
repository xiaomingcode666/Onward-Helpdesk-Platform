import base64
import json
import unittest

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

from ai_phone_agent.decision import RuleDecisionEngine
from ai_phone_agent.go_client import TicketClientError
from ai_phone_agent.models import TicketResult
from ai_phone_agent.service import VoiceAgentService
from ai_phone_agent.telnyx import (
    TelnyxCallBridge,
    TelnyxLine,
    TelnyxSignatureError,
    TelnyxSignatureVerifier,
    parse_number_lines,
)

NUMBER = "+12025550100"


class FakeCommands:
    def __init__(self) -> None:
        self.calls: list[tuple[str, str, dict]] = []

    async def command(self, call_control_id: str, action: str, body: dict) -> None:
        self.calls.append((call_control_id, action, body))

    def actions(self) -> list[str]:
        return [action for _, action, _ in self.calls]

    def last(self, action: str) -> dict:
        return [body for _, name, body in self.calls if name == action][-1]


class FakeTicketClient:
    def __init__(self, error: Exception | None = None) -> None:
        self.error = error
        self.created = 0

    async def create_ticket(self, session, draft):
        if self.error:
            raise self.error
        self.created += 1
        return TicketResult(success=True, ticket_id=7, ticket_no="TK7", status="pending_dispatch")


def event(event_id: str, event_type: str, **payload) -> dict:
    base = {"call_control_id": "v3:ccid-1", "call_leg_id": "leg-1", "call_session_id": "sess-1"}
    base.update(payload)
    return {"data": {"id": event_id, "event_type": event_type, "payload": base}}


def transcript(event_id: str, text: str, *, final: bool = True) -> dict:
    return event(event_id, "call.transcription", transcription_data={"transcript": text, "is_final": final, "confidence": 0.9})


def state(value: str) -> str:
    return base64.b64encode(value.encode()).decode()


class TelnyxSignatureVerifierTest(unittest.TestCase):
    def setUp(self) -> None:
        self.private_key = Ed25519PrivateKey.generate()
        raw = self.private_key.public_key().public_bytes(serialization.Encoding.Raw, serialization.PublicFormat.Raw)
        self.public_key = base64.b64encode(raw).decode()
        self.body = b'{"data":{"id":"evt"}}'

    def sign(self, timestamp: str, body: bytes) -> str:
        return base64.b64encode(self.private_key.sign(timestamp.encode() + b"|" + body)).decode()

    def test_accepts_valid_signature(self) -> None:
        verifier = TelnyxSignatureVerifier(self.public_key, clock=lambda: 1000.0)
        verifier.verify(self.body, self.sign("1000", self.body), "1000")

    def test_accepts_spki_public_key(self) -> None:
        der = self.private_key.public_key().public_bytes(serialization.Encoding.DER, serialization.PublicFormat.SubjectPublicKeyInfo)
        verifier = TelnyxSignatureVerifier(base64.b64encode(der).decode(), clock=lambda: 1000.0)
        verifier.verify(self.body, self.sign("1000", self.body), "1000")

    def test_rejects_tampered_body(self) -> None:
        verifier = TelnyxSignatureVerifier(self.public_key, clock=lambda: 1000.0)
        with self.assertRaises(TelnyxSignatureError):
            verifier.verify(b'{"data":{"id":"other"}}', self.sign("1000", self.body), "1000")

    def test_rejects_stale_timestamp(self) -> None:
        verifier = TelnyxSignatureVerifier(self.public_key, clock=lambda: 1400.0)
        with self.assertRaises(TelnyxSignatureError):
            verifier.verify(self.body, self.sign("1000", self.body), "1000")

    def test_rejects_missing_headers(self) -> None:
        verifier = TelnyxSignatureVerifier(self.public_key, clock=lambda: 1000.0)
        with self.assertRaises(TelnyxSignatureError):
            verifier.verify(self.body, None, "1000")

    def test_fails_closed_without_public_key(self) -> None:
        verifier = TelnyxSignatureVerifier("", clock=lambda: 1000.0)
        with self.assertRaises(TelnyxSignatureError):
            verifier.verify(self.body, self.sign("1000", self.body), "1000")


class ParseNumberLinesTest(unittest.TestCase):
    def test_parses_lines_and_defaults_language(self) -> None:
        lines = parse_number_lines(json.dumps({NUMBER: {"line_id": "line-1", "project_key": "support"}}))
        self.assertEqual(lines[NUMBER], TelnyxLine(line_id="line-1", project_key="support", language="zh-CN"))

    def test_empty_or_invalid_config_has_no_lines(self) -> None:
        self.assertEqual(parse_number_lines(""), {})
        self.assertEqual(parse_number_lines("not json"), {})


class TelnyxCallBridgeTest(unittest.IsolatedAsyncioTestCase):
    def build(self, ticket_client=None) -> tuple[TelnyxCallBridge, FakeCommands, VoiceAgentService]:
        service = VoiceAgentService(RuleDecisionEngine(), ticket_client or FakeTicketClient())
        commands = FakeCommands()
        bridge = TelnyxCallBridge(service, commands, {NUMBER: TelnyxLine("line-1", "support")})
        return bridge, commands, service

    async def connect(self, bridge: TelnyxCallBridge) -> None:
        await bridge.handle_event(event("e1", "call.initiated", direction="incoming", to=NUMBER, **{"from": "+8613800000000"}))
        await bridge.handle_event(event("e2", "call.answered"))

    async def test_unknown_number_is_hung_up_without_session(self) -> None:
        bridge, commands, service = self.build()
        await bridge.handle_event(event("e1", "call.initiated", direction="incoming", to="+19990000000"))
        self.assertEqual(commands.actions(), ["hangup"])
        self.assertEqual(service.sessions, {})

    async def test_answer_greets_and_starts_transcription(self) -> None:
        bridge, commands, service = self.build()
        await self.connect(bridge)

        self.assertEqual(commands.actions(), ["answer", "speak", "transcription_start"])
        session = service.sessions["leg-1"]
        self.assertEqual((session.provider, session.line_id, session.project_key), ("telnyx", "line-1", "support"))
        self.assertEqual(session.caller_phone, "+8613800000000")
        start = commands.last("transcription_start")
        self.assertEqual(start["transcription_engine_config"]["language"], "zh")
        self.assertEqual(start["transcription_tracks"], "inbound")
        self.assertEqual(start["command_id"], "e2:transcription_start")

    async def test_full_call_creates_ticket_and_hangs_up_after_final_speech(self) -> None:
        ticket_client = FakeTicketClient()
        bridge, commands, service = self.build(ticket_client)
        await self.connect(bridge)

        await bridge.handle_event(transcript("e3", "设备无法启动。"))
        self.assertIn("设备编号", commands.last("speak")["payload"])
        await bridge.handle_event(transcript("e4", "没有。"))
        self.assertIn("确认", commands.last("speak")["payload"])
        await bridge.handle_event(transcript("e5", "确认。"))

        final = commands.last("speak")
        self.assertIn("T K 7", final["payload"])
        self.assertEqual(final["stop"], "all")
        self.assertEqual(ticket_client.created, 1)
        self.assertNotIn("hangup", commands.actions())

        # An earlier, interrupted prompt ending must not end the call.
        await bridge.handle_event(event("e6", "call.speak.ended"))
        self.assertNotIn("hangup", commands.actions())
        await bridge.handle_event(event("e7", "call.speak.ended", client_state=final["client_state"]))
        self.assertEqual(commands.actions()[-1], "hangup")

        await bridge.handle_event(event("e8", "call.hangup"))
        self.assertEqual(service.sessions, {})

    async def test_duplicate_and_interim_events_are_ignored(self) -> None:
        bridge, commands, _ = self.build()
        await self.connect(bridge)
        await bridge.handle_event(transcript("e3", "设备无法", final=False))
        await bridge.handle_event(transcript("e4", "设备无法启动"))
        await bridge.handle_event(transcript("e4", "设备无法启动"))
        self.assertEqual(commands.actions().count("speak"), 2)

    async def test_ticket_failure_never_announces_ticket_number(self) -> None:
        bridge, commands, service = self.build(FakeTicketClient(TicketClientError("timeout", result_unknown=True)))
        await self.connect(bridge)
        await bridge.handle_event(transcript("e3", "设备无法启动"))
        await bridge.handle_event(transcript("e4", "没有"))
        await bridge.handle_event(transcript("e5", "确认"))

        final = commands.last("speak")
        self.assertNotIn("工单号", final["payload"])
        self.assertEqual(final["client_state"], state("hangup"))
        task = service.task_store.get("telnyx:leg-1:intent_001")
        self.assertEqual(task.state.value, "result_unknown")

    async def test_events_for_unknown_call_are_hung_up(self) -> None:
        bridge, commands, _ = self.build()
        await bridge.handle_event(event("e1", "call.answered"))
        self.assertEqual(commands.actions(), ["hangup"])


if __name__ == "__main__":
    unittest.main()
