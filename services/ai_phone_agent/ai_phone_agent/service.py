from __future__ import annotations

from dataclasses import dataclass, field

from .decision import DecisionEngine
from .go_client import DecisionClientError, GoTicketClient
from .go_client import TicketClientError
from .models import AgentAction, AgentDecision, AgentState, TicketResult, TicketTaskState, VoiceSession
from .task_store import InMemoryTicketTaskStore, TicketTaskStore


@dataclass(slots=True)
class VoiceAgentService:
    engine: DecisionEngine
    ticket_client: GoTicketClient | None = None
    task_store: TicketTaskStore | None = None
    sessions: dict[str, VoiceSession] = field(default_factory=dict)

    def start(self, session: VoiceSession) -> VoiceSession:
        if session.call_id in self.sessions:
            return self.sessions[session.call_id]
        session.state = AgentState.LISTENING
        if self.task_store is None:
            self.task_store = InMemoryTicketTaskStore()
        self.sessions[session.call_id] = session
        return session

    async def handle_text(self, call_id: str, text: str) -> tuple[AgentDecision, TicketResult | None]:
        session = self.sessions[call_id]
        if session.state in {AgentState.CANCELLED, AgentState.COMPLETED}:
            raise RuntimeError("voice session is already closed")
        session.state = AgentState.RECOGNIZING
        try:
            decision = await self.engine.decide(session, text)
        except DecisionClientError as exc:
            session.state = AgentState.LISTENING
            session.last_error = str(exc)
            session.transcript_segments.append({"text": text, "language": session.language})
            session.history.append({"role": "customer", "text": text})
            return AgentDecision(
                action=AgentAction.FALLBACK,
                intent_id=session.intent_id,
                reply_text="抱歉，系统有点忙，请您再说一遍。",
            ), None
        session.transcript_segments.append({"text": text, "language": session.language})
        session.history.append({"role": "customer", "text": text})
        session.fields.update(decision.collected_fields)
        if decision.action == AgentAction.PREPARE_TICKET and decision.ticket_draft is not None:
            session.pending_ticket = decision.ticket_draft
        if decision.action == AgentAction.CONFIRM_TICKET and session.pending_ticket is None:
            decision = AgentDecision(
                action=AgentAction.CLARIFY,
                intent_id=session.intent_id,
                reply_text="请先说明需要处理的问题，我再为您准备工单。",
                collected_fields=session.fields,
                missing_fields=["issue_summary"],
            )
        if decision.reply_text:
            session.history.append({"role": "agent", "text": decision.reply_text})
        session.clarify_rounds += int(decision.action == AgentAction.CLARIFY)
        if session.clarify_rounds > session.max_clarify_rounds:
            decision.action = AgentAction.FALLBACK
            decision.reply_text = "暂时无法确认完整信息，稍后请通过其他渠道联系支持团队。"
            session.state = AgentState.FAILED
            return decision, None
        if decision.action == AgentAction.CANCEL:
            session.state = AgentState.CANCELLED
            return decision, None
        if decision.action == AgentAction.END:
            session.state = AgentState.COMPLETED
            return decision, None
        if decision.action != AgentAction.CONFIRM_TICKET:
            session.state = AgentState.CONFIRMING if decision.action == AgentAction.PREPARE_TICKET else AgentState.LISTENING
            return decision, None
        if self.ticket_client is None or decision.ticket_draft is None:
            raise RuntimeError("ticket client is required to create a ticket")
        if self.task_store is None:
            self.task_store = InMemoryTicketTaskStore()
        task = self.task_store.put_pending(session, decision.ticket_draft)
        self.task_store.update(task.key, state=TicketTaskState.SUBMITTING)
        session.state = AgentState.SUBMITTING
        try:
            result = await self.ticket_client.create_ticket(session, decision.ticket_draft)
        except TicketClientError as exc:
            state = TicketTaskState.RESULT_UNKNOWN if exc.result_unknown else TicketTaskState.NEEDS_ATTENTION
            self.task_store.update(task.key, state=state, error=str(exc))
            session.state = AgentState.RECOVERING if exc.result_unknown else AgentState.FAILED
            session.last_error = str(exc)
            raise
        if result.success:
            self.task_store.update(task.key, state=TicketTaskState.CREATED, result=result)
            session.pending_ticket = None
            session.state = AgentState.COMPLETED
        return decision, result
