import os
import unittest
from datetime import datetime, timedelta, timezone

from ai_phone_agent.models import TicketDraft, TicketTaskState, VoiceSession
from ai_phone_agent.task_store import PostgresTicketTaskStore


class PostgresTaskStoreTest(unittest.TestCase):
    @unittest.skipUnless(os.getenv("AI_PHONE_TEST_DSN"), "AI_PHONE_TEST_DSN is not configured")
    def test_persists_snapshot_and_claims_due_task(self) -> None:
        store = PostgresTicketTaskStore(os.environ["AI_PHONE_TEST_DSN"])
        try:
            session = VoiceSession(
                call_id="postgres-store-test", provider="test", line_id="line-1", project_key="support",
            )
            draft = TicketDraft(title="恢复测试", description="恢复测试", project_key="support")
            task = store.put_pending(session, draft)
            self.assertEqual(store.get(task.key).session.call_id, session.call_id)
            now = datetime.now(timezone.utc)
            store.update(task.key, state=TicketTaskState.RESULT_UNKNOWN, next_attempt_at=now - timedelta(seconds=1))
            claimed = store.claim_due(now=now, lease_seconds=60, limit=10)
            self.assertEqual([item.key for item in claimed], [task.key])
            self.assertEqual(claimed[0].state, TicketTaskState.SUBMITTING)
        finally:
            store.close()
