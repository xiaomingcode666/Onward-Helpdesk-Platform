from __future__ import annotations

from enum import StrEnum
from typing import Any

from pydantic import BaseModel, Field


class AgentAction(StrEnum):
    ANSWER = "answer"
    CLARIFY = "clarify"
    PREPARE_TICKET = "prepare_ticket"
    CONFIRM_TICKET = "confirm_ticket"
    CANCEL = "cancel"
    END = "end"
    FALLBACK = "fallback"


class AgentState(StrEnum):
    INITIALIZING = "initializing"
    LISTENING = "listening"
    RECOGNIZING = "recognizing"
    DECIDING = "deciding"
    SPEAKING = "speaking"
    CONFIRMING = "confirming"
    SUBMITTING = "submitting"
    COMPLETED = "completed"
    RECOVERING = "recovering"
    FAILED = "failed"
    CANCELLED = "cancelled"


class TicketTaskState(StrEnum):
    PENDING = "pending"
    SUBMITTING = "submitting"
    RESULT_UNKNOWN = "result_unknown"
    CREATED = "created"
    NEEDS_ATTENTION = "needs_attention"


class TicketDraft(BaseModel):
    title: str
    description: str
    project_key: str
    ticket_type: str = "incident"
    priority_code: str = "p2"
    missing_fields: list[str] = Field(default_factory=list)
    field_evidence: dict[str, Any] = Field(default_factory=dict)


class AgentDecision(BaseModel):
    action: AgentAction
    intent_id: str
    reply_text: str = ""
    collected_fields: dict[str, Any] = Field(default_factory=dict)
    missing_fields: list[str] = Field(default_factory=list)
    ready_to_create: bool = False
    ticket_draft: TicketDraft | None = None


class TurnRequest(BaseModel):
    call_id: str
    text: str
    language: str = "zh-CN"


class TurnResponse(BaseModel):
    call_id: str
    decision: AgentDecision
    ticket: "TicketResult | None" = None


class TicketResult(BaseModel):
    success: bool
    ticket_id: int | None = None
    ticket_no: str | None = None
    status: str | None = None


class VoiceSession(BaseModel):
    call_id: str
    provider: str
    line_id: str
    caller_phone: str = ""
    language: str = "zh-CN"
    work_language: str = "zh-CN"
    project_key: str
    intent_id: str = "intent_001"
    clarify_rounds: int = 0
    max_clarify_rounds: int = 3
    fields: dict[str, Any] = Field(default_factory=dict)
    transcript_segments: list[dict[str, Any]] = Field(default_factory=list)
    # Structured conversation history sent to the Helpdesk decision endpoint.
    # ``transcript_segments`` remains the audit-friendly customer transcript;
    # this list also includes the agent's previous spoken replies.
    history: list[dict[str, str]] = Field(default_factory=list)
    recording_ref: str = ""
    pending_ticket: TicketDraft | None = None
    state: AgentState = AgentState.INITIALIZING
    last_error: str = ""


class TicketTaskSnapshot(BaseModel):
    """Persisted input needed to retry a Go request after process restart."""

    session: VoiceSession
    draft: TicketDraft
