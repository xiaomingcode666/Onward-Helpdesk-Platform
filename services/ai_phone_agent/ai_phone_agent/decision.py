from __future__ import annotations

from dataclasses import dataclass
import re
from typing import Protocol

from .models import AgentAction, AgentDecision, TicketDraft, VoiceSession


class DecisionEngine(Protocol):
    async def decide(self, session: VoiceSession, text: str) -> AgentDecision: ...


@dataclass(slots=True)
class RuleDecisionEngine:
    """Deterministic engine for local SDD tests.

    Replace this adapter with the approved LLM/translation provider after the
    state and Go ticket contract pass. The rule engine never creates tickets.
    """

    async def decide(self, session: VoiceSession, text: str) -> AgentDecision:
        # ASR engines append sentence punctuation ("确认。"); keywords must still match.
        content = text.strip().strip("。.！!？?，,、；;：: ")
        lowered = content.lower()
        if not content:
            return AgentDecision(
                action=AgentAction.FALLBACK,
                intent_id=session.intent_id,
                reply_text="没有听清您的问题，请重新说明。",
            )
        if lowered in {"取消", "cancel", "不要了"}:
            return AgentDecision(
                action=AgentAction.CANCEL,
                intent_id=session.intent_id,
                reply_text="已取消本次工单创建。",
            )
        if lowered in {"确认", "confirm", "是", "好的"} and session.pending_ticket:
            return AgentDecision(
                action=AgentAction.CONFIRM_TICKET,
                intent_id=session.intent_id,
                reply_text="正在创建工单。",
                collected_fields=session.fields,
                ready_to_create=True,
                ticket_draft=session.pending_ticket,
            )

        if not session.fields.get("issue_summary"):
            session.fields["issue_summary"] = content
        if not session.fields.get("device_no"):
            if lowered in {"没有", "没有设备编号", "无", "none", "no"}:
                session.fields["device_no"] = "未提供"
            elif re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_-]{2,31}", content):
                session.fields["device_no"] = content
        if not session.fields.get("device_no"):
            return AgentDecision(
                action=AgentAction.CLARIFY,
                intent_id=session.intent_id,
                reply_text="请提供设备编号；如果没有设备编号，请说没有。",
                collected_fields=session.fields,
                missing_fields=["device_no"],
            )

        issue_summary = str(session.fields.get("issue_summary") or content)
        device_no = str(session.fields.get("device_no") or "")
        description = issue_summary
        if device_no:
            description = f"{issue_summary}\n设备编号：{device_no}"
        draft = TicketDraft(
            title=issue_summary[:80],
            description=description,
            project_key=session.project_key,
            missing_fields=[],
            field_evidence={"issue_summary": "turn_text"},
        )
        session.pending_ticket = draft
        return AgentDecision(
            action=AgentAction.PREPARE_TICKET,
            intent_id=session.intent_id,
            reply_text="我已整理问题。请说确认创建工单，或说取消。",
            collected_fields=session.fields,
            missing_fields=[],
            ready_to_create=True,
            ticket_draft=draft,
        )
