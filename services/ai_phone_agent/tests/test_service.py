import unittest

from ai_phone_agent.decision import RuleDecisionEngine
from ai_phone_agent.models import AgentAction, VoiceSession
from ai_phone_agent.service import VoiceAgentService
from ai_phone_agent.go_client import TicketClientError
from ai_phone_agent.models import AgentState, TicketResult


class VoiceAgentServiceTest(unittest.IsolatedAsyncioTestCase):
    async def test_collects_device_number_before_prepare(self) -> None:
        service = VoiceAgentService(RuleDecisionEngine())
        service.start(VoiceSession(call_id="call-1", provider="test", line_id="line-1", project_key="support"))

        decision, ticket = await service.handle_text("call-1", "设备无法启动")

        self.assertEqual(decision.action, AgentAction.CLARIFY)
        self.assertEqual(decision.missing_fields, ["device_no"])
        self.assertIsNone(ticket)

    async def test_confirmation_requires_go_client(self) -> None:
        service = VoiceAgentService(RuleDecisionEngine())
        session = VoiceSession(call_id="call-2", provider="test", line_id="line-1", project_key="support")
        session.fields["device_no"] = "DEV-001"
        service.start(session)

        decision, ticket = await service.handle_text("call-2", "设备无法启动")

        self.assertEqual(decision.action, AgentAction.PREPARE_TICKET)
        self.assertIsNone(ticket)
        self.assertIsNone(service.task_store.get("test:call-2:intent_001"))

        with self.assertRaises(RuntimeError):
            await service.handle_text("call-2", "确认")

    async def test_device_number_is_collected_and_confirmation_creates_ticket(self) -> None:
        class FakeClient:
            async def create_ticket(self, session, draft):
                return TicketResult(success=True, ticket_id=7, ticket_no="TK-7", status="pending")

        service = VoiceAgentService(RuleDecisionEngine(), FakeClient())
        service.start(VoiceSession(call_id="call-3", provider="test", line_id="line-1", project_key="support"))
        await service.handle_text("call-3", "设备无法启动")
        decision, _ = await service.handle_text("call-3", "DEV-001")
        self.assertEqual(decision.action, AgentAction.PREPARE_TICKET)
        decision, ticket = await service.handle_text("call-3", "确认")
        self.assertEqual(decision.action, AgentAction.CONFIRM_TICKET)
        self.assertEqual(ticket.ticket_id, 7)
        self.assertEqual(service.sessions["call-3"].state, AgentState.COMPLETED)

    async def test_rule_engine_ignores_asr_punctuation(self) -> None:
        service = VoiceAgentService(RuleDecisionEngine())
        session = VoiceSession(call_id="call-5", provider="test", line_id="line-1", project_key="support")
        service.start(session)
        await service.handle_text("call-5", "设备无法启动。")
        decision, _ = await service.handle_text("call-5", "没有。")
        self.assertEqual(decision.action, AgentAction.PREPARE_TICKET)
        self.assertEqual(session.fields["issue_summary"], "设备无法启动")
        decision, _ = await service.handle_text("call-5", "取消！")
        self.assertEqual(decision.action, AgentAction.CANCEL)

    async def test_transport_failure_marks_result_unknown(self) -> None:
        class FailingClient:
            async def create_ticket(self, session, draft):
                raise TicketClientError("timeout", result_unknown=True)

        service = VoiceAgentService(RuleDecisionEngine(), FailingClient())
        session = VoiceSession(call_id="call-4", provider="test", line_id="line-1", project_key="support")
        session.fields["device_no"] = "DEV-004"
        service.start(session)
        await service.handle_text("call-4", "设备无法启动")
        with self.assertRaises(TicketClientError):
            await service.handle_text("call-4", "确认")
        self.assertEqual(session.state, AgentState.RECOVERING)
        task = service.task_store.get("test:call-4:intent_001")
        self.assertEqual(task.state.value, "result_unknown")
        self.assertEqual(task.session.call_id, "call-4")
