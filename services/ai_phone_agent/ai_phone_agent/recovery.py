from __future__ import annotations

import asyncio
import os
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone

from .go_client import GoTicketClient, TicketClientError
from .models import TicketTaskState
from .task_store import TicketTaskStore


@dataclass(slots=True)
class TicketRecoveryWorker:
    """Retries only persisted, confirmed ticket submissions.

    The Go endpoint is idempotent, so a transport failure can safely be retried
    with the exact same session and draft snapshot.
    """

    store: TicketTaskStore
    client: GoTicketClient
    lease_seconds: int = 60
    max_attempts: int = 5
    base_delay_seconds: int = 5
    max_delay_seconds: int = 300

    async def run_once(self, *, now: datetime | None = None, limit: int = 20) -> int:
        current = now or datetime.now(timezone.utc)
        tasks = self.store.claim_due(now=current, lease_seconds=self.lease_seconds, limit=limit)
        for task in tasks:
            try:
                result = await self.client.create_ticket(task.session, task.draft)
            except TicketClientError as exc:
                attempt = task.attempts + 1
                retryable = exc.result_unknown and attempt < self.max_attempts
                if retryable:
                    delay = min(self.max_delay_seconds, self.base_delay_seconds * (2 ** max(0, attempt - 1)))
                    self.store.update(
                        task.key,
                        state=TicketTaskState.RESULT_UNKNOWN,
                        attempts=attempt,
                        next_attempt_at=current + timedelta(seconds=delay),
                        error=str(exc),
                        lease_token=task.lease_token,
                    )
                else:
                    self.store.update(
                        task.key,
                        state=TicketTaskState.NEEDS_ATTENTION,
                        attempts=attempt,
                        error=str(exc),
                        lease_token=task.lease_token,
                    )
                continue
            self.store.update(
                task.key,
                state=TicketTaskState.CREATED,
                attempts=task.attempts + 1,
                result=result,
                error="",
                lease_token=task.lease_token,
            )
        return len(tasks)


async def run_forever(worker: TicketRecoveryWorker, *, poll_seconds: float = 2.0) -> None:
    while True:
        await worker.run_once()
        await asyncio.sleep(poll_seconds)


async def _main() -> None:
    from .go_client import GoTicketClient
    from .task_store import PostgresTicketTaskStore

    dsn = os.getenv("AI_PHONE_TASK_DATABASE_URL", "").strip()
    if not dsn:
        raise RuntimeError(
            "AI_PHONE_TASK_DATABASE_URL is required. Set it in this PowerShell session, "
            "for example: $env:AI_PHONE_TASK_DATABASE_URL='postgresql://user:password@host:5432/database'"
        )
    store = PostgresTicketTaskStore(dsn)
    client = GoTicketClient(
        os.getenv("HELPDESK_GO_URL", "http://127.0.0.1:8083"),
        os.getenv("AI_PHONE_SERVICE_TOKEN", ""),
    )
    try:
        print("AI phone recovery worker started; polling every 2 seconds.", flush=True)
        await run_forever(TicketRecoveryWorker(store, client))
    finally:
        store.close()


if __name__ == "__main__":  # pragma: no cover - process entrypoint
    asyncio.run(_main())
