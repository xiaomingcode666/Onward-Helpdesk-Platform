"""Telnyx Voice API (Call Control) adapter.

Telnyx answers the PSTN call and reports call events by webhook; this module
turns those events into text turns for ``VoiceAgentService`` and replies with
Call Control commands. See section 13 of work/AI-PHONE-SDD-SPEC.md.
"""

from __future__ import annotations

import asyncio
import base64
import binascii
import hmac
import json
import logging
import time
from collections import OrderedDict
from dataclasses import dataclass, field
from typing import Any, Callable, Mapping, Protocol

import httpx
from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey
from cryptography.hazmat.primitives.serialization import load_der_public_key

from .go_client import TicketClientError
from .models import AgentAction, AgentState, VoiceSession
from .service import VoiceAgentService

logger = logging.getLogger(__name__)

PROVIDER = "telnyx"
HANGUP_STATE = base64.b64encode(b"hangup").decode()

# session language -> (transcription language, speak language, default voice)
LANGUAGES: dict[str, tuple[str, str, str]] = {
    "zh-CN": ("zh", "cmn-CN", "AWS.Polly.Zhiyu-Neural"),
    "en-US": ("en", "en-US", "AWS.Polly.Joanna-Neural"),
    "ar": ("ar", "arb", "AWS.Polly.Zeina"),
}

GREETINGS = {
    "zh-CN": "您好，这里是智能客服。请简单描述您遇到的问题。",
    "en-US": "Hello, this is the AI support line. Please describe your issue.",
    "ar": "مرحباً، هذا خط الدعم الذكي. يرجى وصف مشكلتك.",
}
TICKET_CREATED = "工单已创建，工单号 {ticket_no}。我们会尽快处理，感谢来电，再见。"
TICKET_UNKNOWN = "系统暂时无法确认工单结果，我们已记录您的问题，稍后会联系您。再见。"


class TelnyxSignatureError(Exception):
    pass


class TelnyxSignatureVerifier:
    """Verifies ``telnyx-signature-ed25519`` over ``{timestamp}|{raw body}``.

    Fails closed: a missing or unreadable public key rejects every webhook.
    """

    def __init__(self, public_key: str, *, tolerance_seconds: int = 300, clock: Callable[[], float] = time.time) -> None:
        self.tolerance_seconds = tolerance_seconds
        self.clock = clock
        self._key = self._load_key(public_key)

    @property
    def configured(self) -> bool:
        return self._key is not None

    @staticmethod
    def _load_key(value: str) -> Ed25519PublicKey | None:
        try:
            raw = base64.b64decode(value.strip(), validate=True)
        except (binascii.Error, ValueError):
            return None
        if len(raw) == 32:
            return Ed25519PublicKey.from_public_bytes(raw)
        try:
            key = load_der_public_key(raw)
        except ValueError:
            return None
        return key if isinstance(key, Ed25519PublicKey) else None

    def verify(self, body: bytes, signature: str | None, timestamp: str | None) -> None:
        if self._key is None:
            raise TelnyxSignatureError("TELNYX_PUBLIC_KEY is not configured or invalid")
        if not signature or not timestamp:
            raise TelnyxSignatureError("missing signature headers")
        try:
            sent_at = int(timestamp)
            signature_bytes = base64.b64decode(signature, validate=True)
        except (binascii.Error, ValueError) as exc:
            raise TelnyxSignatureError("malformed signature headers") from exc
        if abs(self.clock() - sent_at) > self.tolerance_seconds:
            raise TelnyxSignatureError("webhook timestamp outside tolerance")
        try:
            self._key.verify(signature_bytes, timestamp.encode() + b"|" + body)
        except InvalidSignature as exc:
            raise TelnyxSignatureError("invalid signature") from exc


class TelnyxWebhookAuthenticator:
    """Signature first; a URL token only when the account has no public key.

    Telnyx accounts below a certain level cannot read the webhook public key.
    For them ``TELNYX_WEBHOOK_TOKEN`` (at least 32 characters) must appear as
    ``?token=`` in the configured webhook URL. With neither, every webhook fails.
    """

    MIN_TOKEN_LENGTH = 32

    def __init__(self, verifier: TelnyxSignatureVerifier, token: str) -> None:
        self.verifier = verifier
        token = token.strip()
        self.token = token if len(token) >= self.MIN_TOKEN_LENGTH else ""

    def authenticate(self, body: bytes, headers: Mapping[str, str], query_token: str | None) -> None:
        if self.verifier.configured or not self.token:
            self.verifier.verify(body, headers.get("telnyx-signature-ed25519"), headers.get("telnyx-timestamp"))
            return
        if not query_token or not hmac.compare_digest(query_token.encode(), self.token.encode()):
            raise TelnyxSignatureError("invalid webhook token")


@dataclass(frozen=True, slots=True)
class TelnyxLine:
    line_id: str
    project_key: str
    language: str = "zh-CN"


def parse_number_lines(raw: str) -> dict[str, TelnyxLine]:
    """Parses ``TELNYX_NUMBER_LINES``: {"+E164": {"line_id", "project_key", "language"?}}."""
    try:
        data = json.loads(raw) if raw.strip() else {}
    except ValueError:
        logger.error("TELNYX_NUMBER_LINES is not valid JSON")
        return {}
    lines: dict[str, TelnyxLine] = {}
    for number, item in data.items() if isinstance(data, dict) else ():
        if isinstance(item, dict) and item.get("line_id") and item.get("project_key"):
            lines[number.strip()] = TelnyxLine(str(item["line_id"]), str(item["project_key"]), str(item.get("language") or "zh-CN"))
    return lines


class TelnyxCommands(Protocol):
    async def command(self, call_control_id: str, action: str, body: dict[str, Any]) -> None: ...


class TelnyxCallControlClient:
    def __init__(
        self,
        api_key: str,
        base_url: str = "https://api.telnyx.com/v2",
        timeout: float = 10.0,
        transport: httpx.AsyncBaseTransport | None = None,
    ) -> None:
        self.api_key = api_key
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self.transport = transport

    async def command(self, call_control_id: str, action: str, body: dict[str, Any]) -> None:
        async with httpx.AsyncClient(timeout=self.timeout, transport=self.transport) as client:
            response = await client.post(
                f"{self.base_url}/calls/{call_control_id}/actions/{action}",
                headers={"Authorization": f"Bearer {self.api_key}"},
                json=body,
            )
        if response.status_code >= 400:
            logger.warning("telnyx %s failed with HTTP %s: %s", action, response.status_code, response.text[:500])
            response.raise_for_status()


@dataclass(slots=True)
class _Call:
    call_id: str
    line: TelnyxLine
    caller_phone: str
    end_after_speech: bool = False


@dataclass(slots=True)
class TelnyxCallBridge:
    service: VoiceAgentService
    commands: TelnyxCommands
    lines: dict[str, TelnyxLine]
    voices: dict[str, str] = field(default_factory=dict)
    calls: dict[str, _Call] = field(default_factory=dict)
    _locks: dict[str, asyncio.Lock] = field(default_factory=dict)
    _seen: OrderedDict[str, None] = field(default_factory=OrderedDict)

    async def handle_event(self, envelope: dict[str, Any]) -> None:
        data = envelope.get("data") or {}
        event_id = str(data.get("id") or "")
        event_type = str(data.get("event_type") or "")
        payload = data.get("payload") or {}
        ccid = str(payload.get("call_control_id") or "")
        if not event_id or not ccid or self._already_seen(event_id):
            return
        lock = self._locks.setdefault(ccid, asyncio.Lock())
        async with lock:
            try:
                await self._dispatch(event_id, event_type, ccid, payload)
            except Exception:
                logger.exception("telnyx event %s (%s) failed", event_id, event_type)
        if event_type == "call.hangup":
            self._locks.pop(ccid, None)

    def _already_seen(self, event_id: str) -> bool:
        if event_id in self._seen:
            return True
        self._seen[event_id] = None
        while len(self._seen) > 10000:
            self._seen.popitem(last=False)
        return False

    async def _dispatch(self, event_id: str, event_type: str, ccid: str, payload: dict[str, Any]) -> None:
        if event_type == "call.initiated":
            await self._on_initiated(event_id, ccid, payload)
        elif event_type == "call.hangup":
            call = self.calls.pop(ccid, None)
            if call:
                self.service.sessions.pop(call.call_id, None)
        elif ccid not in self.calls:
            if event_type == "call.answered":
                await self._command(ccid, "hangup", event_id, {})
        elif event_type == "call.answered":
            await self._on_answered(event_id, ccid)
        elif event_type == "call.transcription":
            await self._on_transcription(event_id, ccid, payload)
        elif event_type == "call.speak.ended":
            if self.calls[ccid].end_after_speech and payload.get("client_state") == HANGUP_STATE:
                await self._command(ccid, "hangup", event_id, {})

    async def _on_initiated(self, event_id: str, ccid: str, payload: dict[str, Any]) -> None:
        if payload.get("direction") != "incoming":
            return
        line = self.lines.get(str(payload.get("to") or "").strip())
        if line is None:
            logger.warning("telnyx call to unconfigured number %s rejected", payload.get("to"))
            await self._command(ccid, "hangup", event_id, {})
            return
        call_id = str(payload.get("call_leg_id") or ccid)
        self.calls[ccid] = _Call(call_id=call_id, line=line, caller_phone=str(payload.get("from") or ""))
        await self._command(ccid, "answer", event_id, {})

    async def _on_answered(self, event_id: str, ccid: str) -> None:
        call = self.calls[ccid]
        language = call.line.language if call.line.language in LANGUAGES else "zh-CN"
        self.service.start(
            VoiceSession(
                call_id=call.call_id,
                provider=PROVIDER,
                line_id=call.line.line_id,
                caller_phone=call.caller_phone,
                language=language,
                project_key=call.line.project_key,
            )
        )
        await self._speak(ccid, event_id, GREETINGS[language], language)
        await self._command(
            ccid,
            "transcription_start",
            event_id,
            {
                "transcription_engine": "Google",
                "transcription_engine_config": {
                    "transcription_engine": "Google",
                    "language": LANGUAGES[language][0],
                    "interim_results": False,
                },
                "transcription_tracks": "inbound",
            },
        )

    async def _on_transcription(self, event_id: str, ccid: str, payload: dict[str, Any]) -> None:
        data = payload.get("transcription_data") or {}
        text = str(data.get("transcript") or "").strip()
        if not data.get("is_final") or not text:
            return
        call = self.calls[ccid]
        session = self.service.sessions.get(call.call_id)
        if session is None or call.end_after_speech:
            return
        try:
            decision, ticket = await self.service.handle_text(call.call_id, text)
        except TicketClientError:
            call.end_after_speech = True
            await self._speak(ccid, event_id, TICKET_UNKNOWN, session.language, hangup=True)
            return
        if ticket and ticket.success:
            call.end_after_speech = True
            spoken_no = " ".join(ticket.ticket_no or str(ticket.ticket_id))
            await self._speak(ccid, event_id, TICKET_CREATED.format(ticket_no=spoken_no), session.language, hangup=True)
            return
        closing = decision.action in {AgentAction.CANCEL, AgentAction.END} or session.state == AgentState.FAILED
        call.end_after_speech = closing
        if decision.reply_text:
            await self._speak(ccid, event_id, decision.reply_text, session.language, hangup=closing)
        elif closing:
            await self._command(ccid, "hangup", event_id, {})

    async def _speak(self, ccid: str, event_id: str, text: str, language: str, *, hangup: bool = False) -> None:
        _, speak_language, default_voice = LANGUAGES.get(language, LANGUAGES["zh-CN"])
        body: dict[str, Any] = {
            "payload": text,
            "voice": self.voices.get(language, default_voice),
            "language": speak_language,
            "stop": "all",
        }
        if hangup:
            body["client_state"] = HANGUP_STATE
        await self._command(ccid, "speak", event_id, body)

    async def _command(self, ccid: str, action: str, event_id: str, body: dict[str, Any]) -> None:
        # Derived command_id lets Telnyx drop commands replayed after a webhook retry.
        await self.commands.command(ccid, action, {**body, "command_id": f"{event_id}:{action}"})
