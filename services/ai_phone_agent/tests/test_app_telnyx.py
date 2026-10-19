import base64
import os
import time
import unittest
from unittest import mock

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from fastapi.testclient import TestClient

with mock.patch.dict(os.environ, {"AI_PHONE_TASK_DATABASE_URL": ""}):
    from ai_phone_agent import app as app_module

from ai_phone_agent.telnyx import TelnyxSignatureVerifier, TelnyxWebhookAuthenticator


class TelnyxWebhookEndpointTest(unittest.TestCase):
    def setUp(self) -> None:
        self.private_key = Ed25519PrivateKey.generate()
        raw = self.private_key.public_key().public_bytes(serialization.Encoding.Raw, serialization.PublicFormat.Raw)
        verifier = TelnyxSignatureVerifier(base64.b64encode(raw).decode())
        patcher = mock.patch.object(app_module, "telnyx_auth", TelnyxWebhookAuthenticator(verifier, ""))
        patcher.start()
        self.addCleanup(patcher.stop)
        self.handled: list[dict] = []

        handled = self.handled

        class RecordingBridge:
            async def handle_event(self, event: dict) -> None:
                handled.append(event)

        bridge_patcher = mock.patch.object(app_module, "telnyx_bridge", RecordingBridge())
        bridge_patcher.start()
        self.addCleanup(bridge_patcher.stop)
        self.client = TestClient(app_module.app)

    def headers(self, body: bytes, timestamp: str | None = None) -> dict[str, str]:
        timestamp = timestamp or str(int(time.time()))
        signature = base64.b64encode(self.private_key.sign(timestamp.encode() + b"|" + body)).decode()
        return {"telnyx-signature-ed25519": signature, "telnyx-timestamp": timestamp, "content-type": "application/json"}

    def test_signed_webhook_is_acknowledged_and_dispatched(self) -> None:
        body = b'{"data":{"id":"e1","event_type":"call.hangup","payload":{"call_control_id":"c1"}}}'
        response = self.client.post("/v1/telnyx/webhook", content=body, headers=self.headers(body))
        self.assertEqual(response.status_code, 200)
        self.assertEqual(self.handled[0]["data"]["id"], "e1")

    def test_unsigned_or_forged_webhook_is_rejected(self) -> None:
        body = b'{"data":{"id":"e1"}}'
        self.assertEqual(self.client.post("/v1/telnyx/webhook", content=body).status_code, 403)
        forged = self.headers(b'{"data":{"id":"other"}}')
        self.assertEqual(self.client.post("/v1/telnyx/webhook", content=body, headers=forged).status_code, 403)
        self.assertEqual(self.handled, [])


class TelnyxWebhookTokenTest(unittest.TestCase):
    """Accounts without a Telnyx public key authenticate with a URL token."""

    TOKEN = "t" * 40

    def setUp(self) -> None:
        patcher = mock.patch.object(app_module, "telnyx_auth", TelnyxWebhookAuthenticator(TelnyxSignatureVerifier(""), self.TOKEN))
        patcher.start()
        self.addCleanup(patcher.stop)
        self.handled: list[dict] = []
        handled = self.handled

        class RecordingBridge:
            async def handle_event(self, event: dict) -> None:
                handled.append(event)

        bridge_patcher = mock.patch.object(app_module, "telnyx_bridge", RecordingBridge())
        bridge_patcher.start()
        self.addCleanup(bridge_patcher.stop)
        self.client = TestClient(app_module.app)

    def test_matching_token_is_accepted(self) -> None:
        body = b'{"data":{"id":"e1"}}'
        response = self.client.post(f"/v1/telnyx/webhook?token={self.TOKEN}", content=body)
        self.assertEqual(response.status_code, 200)
        self.assertEqual(len(self.handled), 1)

    def test_missing_or_wrong_token_is_rejected(self) -> None:
        body = b'{"data":{"id":"e1"}}'
        self.assertEqual(self.client.post("/v1/telnyx/webhook", content=body).status_code, 403)
        self.assertEqual(self.client.post("/v1/telnyx/webhook?token=" + "x" * 40, content=body).status_code, 403)
        self.assertEqual(self.handled, [])


class TelnyxWebhookAuthenticatorTest(unittest.TestCase):
    def test_short_token_is_not_accepted_as_configuration(self) -> None:
        auth = TelnyxWebhookAuthenticator(TelnyxSignatureVerifier(""), "short")
        from ai_phone_agent.telnyx import TelnyxSignatureError
        with self.assertRaises(TelnyxSignatureError):
            auth.authenticate(b"{}", {}, "short")


if __name__ == "__main__":
    unittest.main()
