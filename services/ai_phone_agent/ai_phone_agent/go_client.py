from __future__ import annotations

import hashlib
import json
from typing import Any, Mapping

import httpx

from .models import AgentAction, AgentDecision, TicketDraft, TicketResult, VoiceSession


class TicketClientError(RuntimeError):
    def __init__(self, message: str, *, result_unknown: bool = False, conflict: bool = False) -> None:
        super().__init__(message)
        self.result_unknown = result_unknown
        self.conflict = conflict


class DecisionClientError(RuntimeError):
    """Raised when Helpdesk cannot return a usable structured decision."""


class HelpdeskDecisionClient:
    """DecisionEngine implementation backed by the Go Helpdesk API.

    The Python process owns the conversation state, while Go resolves the
    tenant-scoped LLM configuration and returns a validated action. No model
    credential is held by the voice agent.
    """

    def __init__(
        self,
        base_url: str,
        service_token: str,
        timeout: float = 40.0,
        transport: httpx.AsyncBaseTransport | None = None,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.service_token = service_token
        self.timeout = timeout
        self.transport = transport

    async def decide(self, session: VoiceSession, text: str) -> AgentDecision:
        payload = {
            "schema_version": 1,
            "provider": session.provider,
            "call_id": session.call_id,
            "intent_id": session.intent_id,
            "language": session.language,
            "work_language": session.work_language,
            "project_key": session.project_key,
            "caller_phone": session.caller_phone,
            "text": text,
            "collected_fields": {
                str(key): str(value)
                for key, value in session.fields.items()
                if value is not None and str(value).strip()
            },
            "pending_ticket": (
                {
                    "title": session.pending_ticket.title,
                    "description": session.pending_ticket.description,
                }
                if session.pending_ticket is not None
                else None
            ),
            "history": session.history[-20:],
        }
        try:
            async with httpx.AsyncClient(timeout=self.timeout, transport=self.transport) as client:
                response = await client.post(
                    f"{self.base_url}/api/third/ai-phone/decisions",
                    headers={
                        "Authorization": f"Bearer {self.service_token}",
                        "X-AI-Phone-Line-ID": session.line_id,
                    },
                    json=payload,
                )
                response.raise_for_status()
                data = response.json()
        except (httpx.TimeoutException, httpx.NetworkError) as exc:
            raise DecisionClientError("Helpdesk decision request timed out or was unreachable") from exc
        except httpx.HTTPStatusError as exc:
            status = exc.response.status_code if exc.response is not None else 0
            raise DecisionClientError(
                f"Helpdesk decision request failed with HTTP {status}"
            ) from exc
        except ValueError as exc:
            raise DecisionClientError("Helpdesk decision response is not valid JSON") from exc

        try:
            return self._parse_decision(data, session)
        except (TypeError, ValueError, KeyError) as exc:
            raise DecisionClientError("Helpdesk returned an invalid structured decision") from exc

    @staticmethod
    def _parse_decision(data: Any, session: VoiceSession) -> AgentDecision:
        def as_text(value: Any) -> str:
            return "" if value is None else str(value).strip()

        if not isinstance(data, Mapping):
            raise TypeError("decision must be an object")
        action = AgentAction(as_text(data.get("action", "")))
        reply_text = as_text(data.get("reply_text", ""))
        if not reply_text:
            raise ValueError("reply_text is required")
        raw_fields = data.get("collected_fields") or {}
        if not isinstance(raw_fields, Mapping):
            raise TypeError("collected_fields must be an object")
        collected_fields = {
            str(key): str(value)
            for key, value in raw_fields.items()
            if value is not None and str(value).strip()
        }
        raw_missing = data.get("missing_fields") or []
        if not isinstance(raw_missing, list):
            raise TypeError("missing_fields must be an array")
        missing_fields = [as_text(value) for value in raw_missing if as_text(value)]
        ticket_draft: TicketDraft | None = None
        raw_ticket = data.get("ticket")
        if raw_ticket is not None:
            if not isinstance(raw_ticket, Mapping):
                raise TypeError("ticket must be an object")
            ticket_draft = TicketDraft(
                title=as_text(raw_ticket.get("title", "")),
                description=as_text(raw_ticket.get("description", "")),
                project_key=session.project_key,
                missing_fields=missing_fields,
            )
        if action == AgentAction.PREPARE_TICKET and ticket_draft is None:
            raise ValueError("prepare_ticket requires a ticket draft")
        if action == AgentAction.CONFIRM_TICKET:
            # The Go side also enforces this rule in the prompt, but the
            # Python state machine is the final guard before建单.
            if session.pending_ticket is None:
                return AgentDecision(
                    action=AgentAction.CLARIFY,
                    intent_id=session.intent_id,
                    reply_text="请先说明需要处理的问题，我再为您准备工单。",
                    collected_fields=collected_fields,
                    missing_fields=list(raw_missing),
                )
            ticket_draft = session.pending_ticket
        return AgentDecision(
            action=action,
            intent_id=as_text(data.get("intent_id")) or session.intent_id,
            reply_text=reply_text,
            collected_fields=collected_fields,
            missing_fields=missing_fields,
            ready_to_create=bool(data.get("ready_to_create", False)),
            ticket_draft=ticket_draft,
        )


class GoTicketClient:
    def __init__(
        self,
        base_url: str,
        service_token: str,
        timeout: float = 8.0,
        transport: httpx.AsyncBaseTransport | None = None,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.service_token = service_token
        self.timeout = timeout
        self.transport = transport

    async def create_ticket(self, session: VoiceSession, draft: TicketDraft) -> TicketResult:
        key = f"{session.provider}:{session.call_id}:{session.intent_id}"
        payload = {
            "schema_version": 1,
            "provider": session.provider,
            "call_id": session.call_id,
            "intent_id": session.intent_id,
            "language": session.language,
            "work_language": session.work_language,
            "title": draft.title,
            "description": draft.description,
            "project_key": draft.project_key,
            "ticket_type": draft.ticket_type,
            "priority_code": draft.priority_code,
            "caller_phone": session.caller_phone,
            "missing_fields": draft.missing_fields,
            "recording_ref": session.recording_ref,
            "transcript_segments": session.transcript_segments,
            "field_evidence": draft.field_evidence,
        }
        try:
            async with httpx.AsyncClient(timeout=self.timeout, transport=self.transport) as client:
                response = await client.post(
                    f"{self.base_url}/api/third/ai-phone/tickets",
                    headers={
                        "Authorization": f"Bearer {self.service_token}",
                        "X-AI-Phone-Line-ID": session.line_id,
                        "Idempotency-Key": key,
                    },
                    json=payload,
                )
        except (httpx.TimeoutException, httpx.NetworkError) as exc:
            raise TicketClientError("ticket result is unknown after transport failure", result_unknown=True) from exc
        if response.status_code == 409:
            raise TicketClientError("ticket idempotency conflict", conflict=True)
        try:
            response.raise_for_status()
        except httpx.HTTPStatusError as exc:
            raise TicketClientError(f"ticket creation failed with HTTP {response.status_code}") from exc
        try:
            data = response.json()
        except ValueError as exc:
            raise TicketClientError("ticket response is not valid JSON", result_unknown=True) from exc
        if not data.get("success") or not data.get("ticket_id"):
            raise TicketClientError("Go service did not confirm ticket creation")
        return TicketResult(
            success=bool(data.get("success")),
            ticket_id=data.get("ticket_id"),
            ticket_no=data.get("ticket_no"),
            status=data.get("status"),
        )

    @staticmethod
    def payload_fingerprint(payload: dict[str, object]) -> str:
        encoded = json.dumps(payload, ensure_ascii=False, sort_keys=True).encode()
        return hashlib.sha256(encoded).hexdigest()
