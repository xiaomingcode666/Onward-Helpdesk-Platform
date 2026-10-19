import unittest
from datetime import datetime, timezone

import httpx
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

from ai_phone_agent.go_client import GoTicketClient, TicketClientError
from ai_phone_agent.models import TicketDraft, TicketResult, TicketTaskState, VoiceSession
from ai_phone_agent.recovery import TicketRecoveryWorker
from ai_phone_agent.task_store import InMemoryTicketTaskStore


class RecoveryWorkerE2ETest(unittest.IsolatedAsyncioTestCase):
    async def test_retries_against_simulated_go_http_api_with_same_idempotency_key(self) -> None:
        go_app = FastAPI()
        requests: list[tuple[dict, dict]] = []

        @go_app.post("/api/third/ai-phone/tickets")
        async def create_ticket(request: Request) -> JSONResponse:
            body = await request.json()
            headers = {
                "authorization": request.headers.get("authorization", ""),
                "line": request.headers.get("x-ai-phone-line-id", ""),
                "idempotency": request.headers.get("idempotency-key", ""),
            }
            requests.append((body, headers))
            return JSONResponse({"success": True, "ticket_id": 9001, "ticket_no": "TK9001", "status": "pending"})

        transport = httpx.ASGITransport(app=go_app)
        client = GoTicketClient("http://go-simulated", "test-service-token", transport=transport)
        store = InMemoryTicketTaskStore()
        session = VoiceSession(
            call_id="call-recovery-1", provider="china-telecom", line_id="line-1", project_key="support",
            caller_phone="13800000000", pending_ticket=TicketDraft(
                title="设备无法启动", description="客户反馈设备无法启动", project_key="support",
            ),
        )
        draft = session.pending_ticket
        assert draft is not None
        task = store.put_pending(session, draft)
        now = datetime.now(timezone.utc)
        store.update(task.key, state=TicketTaskState.RESULT_UNKNOWN, next_attempt_at=now)

        worker = TicketRecoveryWorker(store, client, base_delay_seconds=1)
        self.assertEqual(await worker.run_once(now=now), 1)
        saved = store.get(task.key)
        self.assertIsNotNone(saved)
        self.assertEqual(saved.state, TicketTaskState.CREATED)
        self.assertEqual(saved.result, TicketResult(success=True, ticket_id=9001, ticket_no="TK9001", status="pending"))
        self.assertEqual(len(requests), 1)
        self.assertEqual(requests[0][1]["line"], "line-1")
        self.assertEqual(requests[0][1]["idempotency"], "china-telecom:call-recovery-1:intent_001")
        self.assertEqual(requests[0][0]["call_id"], "call-recovery-1")

        # A duplicate HTTP delivery receives the same Go result and key.
        duplicate = await client.create_ticket(session, draft)
        self.assertEqual(duplicate.ticket_id, 9001)
        self.assertEqual(requests[0][1]["idempotency"], requests[1][1]["idempotency"])

    async def test_transport_failure_uses_backoff_then_needs_attention(self) -> None:
        class FailingClient:
            async def create_ticket(self, session, draft):
                raise TicketClientError("network unavailable", result_unknown=True)

        store = InMemoryTicketTaskStore()
        session = VoiceSession(call_id="call-retry", provider="test", line_id="line-1", project_key="support")
        draft = TicketDraft(title="重试测试", description="重试测试", project_key="support")
        task = store.put_pending(session, draft)
        now = datetime.now(timezone.utc)
        store.update(task.key, state=TicketTaskState.RESULT_UNKNOWN, next_attempt_at=now)
        worker = TicketRecoveryWorker(store, FailingClient(), base_delay_seconds=10, max_attempts=2)

        await worker.run_once(now=now)
        retry_task = store.get(task.key)
        self.assertEqual(retry_task.state, TicketTaskState.RESULT_UNKNOWN)
        self.assertEqual(retry_task.attempts, 1)
        self.assertGreaterEqual(retry_task.next_attempt_at, now)

        await worker.run_once(now=retry_task.next_attempt_at)
        self.assertEqual(store.get(task.key).state, TicketTaskState.NEEDS_ATTENTION)

    async def test_stale_lease_cannot_overwrite_new_claim(self) -> None:
        store = InMemoryTicketTaskStore()
        session = VoiceSession(call_id="call-lease", provider="test", line_id="line-1", project_key="support")
        task = store.put_pending(session, TicketDraft(title="租约", description="租约", project_key="support"))
        now = datetime.now(timezone.utc)
        first = store.claim_due(now=now, lease_seconds=0, limit=1)[0]
        store.update(task.key, state=TicketTaskState.RESULT_UNKNOWN, next_attempt_at=now)
        second = store.claim_due(now=now, lease_seconds=0, limit=1)[0]
        self.assertNotEqual(first.lease_token, second.lease_token)
        with self.assertRaises(RuntimeError):
            store.update(task.key, state=TicketTaskState.CREATED, lease_token=first.lease_token)
