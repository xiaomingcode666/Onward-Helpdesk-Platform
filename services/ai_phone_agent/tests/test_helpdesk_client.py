import unittest

import httpx
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

from ai_phone_agent.go_client import HelpdeskDecisionClient, GoTicketClient
from ai_phone_agent.models import AgentAction, AgentState, VoiceSession
from ai_phone_agent.service import VoiceAgentService
from ai_phone_agent.task_store import InMemoryTicketTaskStore


class HelpdeskVoiceLinkTest(unittest.IsolatedAsyncioTestCase):
    async def test_decision_and_confirmation_use_the_same_go_service_contract(self) -> None:
        go_app = FastAPI()
        decision_requests: list[tuple[dict, dict]] = []
        ticket_requests: list[tuple[dict, dict]] = []

        @go_app.post("/api/third/ai-phone/decisions")
        async def decision(request: Request) -> JSONResponse:
            body = await request.json()
            decision_requests.append((body, dict(request.headers)))
            if len(decision_requests) == 1:
                return JSONResponse(
                    {
                        "action": "prepare_ticket",
                        "intent_id": "intent_001",
                        "reply_text": "我记录为设备无法启动，确认创建工单吗？",
                        "collected_fields": {
                            "issue_summary": "设备无法启动",
                            "device_no": "DEV-001",
                        },
                        "missing_fields": [],
                        "ready_to_create": True,
                        "ticket": {
                            "title": "设备无法启动",
                            "description": "客户反馈设备无法启动，设备编号 DEV-001",
                        },
                    }
                )
            return JSONResponse(
                {
                    "action": "confirm_ticket",
                    "intent_id": "intent_001",
                    "reply_text": "好的，正在创建工单。",
                    "collected_fields": {
                        "issue_summary": "设备无法启动",
                        "device_no": "DEV-001",
                    },
                    "missing_fields": [],
                    "ready_to_create": True,
                }
            )

        @go_app.post("/api/third/ai-phone/tickets")
        async def ticket(request: Request) -> JSONResponse:
            body = await request.json()
            ticket_requests.append((body, dict(request.headers)))
            return JSONResponse(
                {"success": True, "ticket_id": 42, "ticket_no": "TK-42", "status": "pending"}
            )

        transport = httpx.ASGITransport(app=go_app)
        base_url = "http://go-simulated"
        agent = VoiceAgentService(
            HelpdeskDecisionClient(base_url, "s" * 40, transport=transport),
            GoTicketClient(base_url, "s" * 40, transport=transport),
            task_store=InMemoryTicketTaskStore(),
        )
        session = agent.start(
            VoiceSession(
                call_id="mic-call-1",
                provider="local-mic",
                line_id="line-1",
                caller_phone="13800000000",
                project_key="support",
            )
        )

        first, no_ticket = await agent.handle_text(session.call_id, "设备无法启动")
        self.assertEqual(first.action, AgentAction.PREPARE_TICKET)
        self.assertIsNone(no_ticket)
        self.assertEqual(session.pending_ticket.title, "设备无法启动")

        second, created = await agent.handle_text(session.call_id, "确认")
        self.assertEqual(second.action, AgentAction.CONFIRM_TICKET)
        self.assertEqual(created.ticket_no, "TK-42")
        self.assertEqual(session.state, AgentState.COMPLETED)

        self.assertEqual(len(decision_requests), 2)
        first_body, first_headers = decision_requests[0]
        self.assertEqual(first_headers["x-ai-phone-line-id"], "line-1")
        self.assertEqual(first_body["call_id"], "mic-call-1")
        self.assertEqual(first_body["text"], "设备无法启动")
        second_body, _ = decision_requests[1]
        self.assertEqual(second_body["text"], "确认")
        self.assertEqual(second_body["pending_ticket"]["title"], "设备无法启动")
        self.assertEqual([item["role"] for item in second_body["history"]], ["customer", "agent"])

        self.assertEqual(len(ticket_requests), 1)
        ticket_body, ticket_headers = ticket_requests[0]
        self.assertEqual(ticket_headers["x-ai-phone-line-id"], "line-1")
        self.assertEqual(ticket_headers["idempotency-key"], "local-mic:mic-call-1:intent_001")
        self.assertEqual(ticket_body["title"], "设备无法启动")
        self.assertEqual(ticket_body["caller_phone"], "13800000000")

    async def test_decision_transport_failure_returns_retry_prompt(self) -> None:
        client = HelpdeskDecisionClient("http://unreachable", "s" * 40, timeout=0.001)
        agent = VoiceAgentService(client, task_store=InMemoryTicketTaskStore())
        session = agent.start(
            VoiceSession(call_id="mic-call-2", provider="local-mic", line_id="line-1", project_key="support")
        )

        decision, ticket = await agent.handle_text(session.call_id, "你好")

        self.assertIsNone(ticket)
        self.assertEqual(decision.action, AgentAction.FALLBACK)
        self.assertIn("再说一遍", decision.reply_text)
        self.assertEqual(session.state, AgentState.LISTENING)


if __name__ == "__main__":
    unittest.main()
