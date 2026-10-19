from __future__ import annotations

from dataclasses import dataclass, replace
from datetime import datetime, timedelta, timezone
from typing import Protocol
from uuid import uuid4

from .models import TicketDraft, TicketResult, TicketTaskState, VoiceSession


@dataclass(slots=True)
class TicketTask:
    key: str
    call_id: str
    intent_id: str
    state: TicketTaskState
    draft: TicketDraft
    session: VoiceSession
    created_at: datetime
    updated_at: datetime
    next_attempt_at: datetime
    attempts: int = 0
    lease_token: str = ""
    result: TicketResult | None = None
    error: str = ""


class TicketTaskStore(Protocol):
    def put_pending(self, session: VoiceSession, draft: TicketDraft) -> TicketTask: ...

    def update(self, key: str, **changes: object) -> TicketTask: ...

    def get(self, key: str) -> TicketTask | None: ...

    def claim_due(self, *, now: datetime, lease_seconds: int, limit: int) -> list[TicketTask]: ...

    def close(self) -> None: ...


class InMemoryTicketTaskStore:
    """Local store for the simulator; production can replace it with the PostgreSQL table."""

    def __init__(self) -> None:
        self._items: dict[str, TicketTask] = {}

    @staticmethod
    def key(session: VoiceSession) -> str:
        return f"{session.provider}:{session.call_id}:{session.intent_id}"

    def put_pending(self, session: VoiceSession, draft: TicketDraft) -> TicketTask:
        now = datetime.now(timezone.utc)
        task = self._items.get(self.key(session))
        if task is None:
            task = TicketTask(self.key(session), session.call_id, session.intent_id, TicketTaskState.PENDING, draft, session.model_copy(deep=True), now, now, now)
            self._items[task.key] = task
        return task

    def update(self, key: str, **changes: object) -> TicketTask:
        task = self._items[key]
        lease_token = str(changes.get("lease_token", ""))
        if lease_token and task.lease_token and lease_token != task.lease_token:
            raise RuntimeError("ticket task lease was lost")
        task = replace(task, updated_at=datetime.now(timezone.utc), **changes)
        self._items[key] = task
        return task

    def get(self, key: str) -> TicketTask | None:
        return self._items.get(key)

    def claim_due(self, *, now: datetime, lease_seconds: int, limit: int) -> list[TicketTask]:
        del lease_seconds
        due_states = {TicketTaskState.PENDING, TicketTaskState.RESULT_UNKNOWN}
        tasks = [item for item in self._items.values() if item.state in due_states and item.next_attempt_at <= now]
        claimed: list[TicketTask] = []
        for task in sorted(tasks, key=lambda item: item.updated_at)[:limit]:
            claimed_task = replace(
                task,
                state=TicketTaskState.SUBMITTING,
                updated_at=datetime.now(timezone.utc),
                lease_token=uuid4().hex,
            )
            self._items[task.key] = claimed_task
            claimed.append(claimed_task)
        return claimed

    def close(self) -> None:
        return None


class PostgresTicketTaskStore:
    """Durable task store used when AI_PHONE_TASK_DATABASE_URL is configured.

    psycopg is imported lazily so the text simulator remains dependency-light.
    The SQL schema is kept in sql/001_ticket_tasks.sql for deployment migrations.
    """

    def __init__(self, dsn: str) -> None:
        try:
            import psycopg
        except ImportError as exc:  # pragma: no cover - exercised in deployment
            raise RuntimeError("psycopg is required for PostgreSQL task storage") from exc
        self._conn = psycopg.connect(dsn, autocommit=True)
        with self._conn.cursor() as cursor:
            cursor.execute(
                """
                CREATE TABLE IF NOT EXISTS ai_phone_ticket_tasks (
                    task_key TEXT PRIMARY KEY,
                    call_id TEXT NOT NULL,
                    intent_id TEXT NOT NULL,
                    state TEXT NOT NULL,
                    draft_json JSONB NOT NULL,
                    session_json JSONB NOT NULL,
                    result_json JSONB,
                    error TEXT NOT NULL DEFAULT '',
                    created_at TIMESTAMPTZ NOT NULL,
                    updated_at TIMESTAMPTZ NOT NULL,
                    next_attempt_at TIMESTAMPTZ NOT NULL,
                    attempts INTEGER NOT NULL DEFAULT 0,
                    lease_token TEXT NOT NULL DEFAULT ''
                )
                """
            )
            cursor.execute("ALTER TABLE ai_phone_ticket_tasks ADD COLUMN IF NOT EXISTS session_json JSONB")
            cursor.execute("ALTER TABLE ai_phone_ticket_tasks ADD COLUMN IF NOT EXISTS next_attempt_at TIMESTAMPTZ")
            cursor.execute("ALTER TABLE ai_phone_ticket_tasks ADD COLUMN IF NOT EXISTS attempts INTEGER NOT NULL DEFAULT 0")
            cursor.execute("ALTER TABLE ai_phone_ticket_tasks ADD COLUMN IF NOT EXISTS lease_token TEXT NOT NULL DEFAULT ''")
            cursor.execute("UPDATE ai_phone_ticket_tasks SET next_attempt_at = updated_at WHERE next_attempt_at IS NULL")
            cursor.execute(
                "UPDATE ai_phone_ticket_tasks SET state = 'needs_attention', error = 'legacy task has no frozen request snapshot' "
                "WHERE session_json IS NULL AND state IN ('pending', 'submitting', 'result_unknown')"
            )

    def claim_due(self, *, now: datetime, lease_seconds: int, limit: int) -> list[TicketTask]:
        import json

        lease_until = now + timedelta(seconds=lease_seconds)
        with self._conn.transaction():
            with self._conn.cursor() as cursor:
                cursor.execute(
                    """
                    SELECT task_key
                        FROM ai_phone_ticket_tasks
                        WHERE session_json IS NOT NULL AND ((state IN ('pending', 'result_unknown') AND next_attempt_at <= %s)
                           OR (state = 'submitting' AND updated_at <= %s))
                        ORDER BY next_attempt_at
                        FOR UPDATE SKIP LOCKED
                        LIMIT %s
                    """,
                    (now, now, limit),
                )
                keys = [row[0] for row in cursor.fetchall()]
                claimed: list[tuple[str, str]] = []
                for key in keys:
                    token = uuid4().hex
                    cursor.execute(
                        "UPDATE ai_phone_ticket_tasks SET state = 'submitting', updated_at = %s, lease_token = %s "
                        "WHERE task_key = %s RETURNING task_key",
                        (lease_until, token, key),
                    )
                    if cursor.fetchone() is not None:
                        claimed.append((key, token))
        tasks: list[TicketTask] = []
        for key, token in claimed:
            task = self.get(key)
            if task is not None:
                task.lease_token = token
                tasks.append(task)
        return tasks

    def close(self) -> None:
        self._conn.close()

    def put_pending(self, session: VoiceSession, draft: TicketDraft) -> TicketTask:
        import json

        now = datetime.now(timezone.utc)
        key = InMemoryTicketTaskStore.key(session)
        with self._conn.cursor() as cursor:
            cursor.execute(
                """
                INSERT INTO ai_phone_ticket_tasks
                    (task_key, call_id, intent_id, state, draft_json, session_json, created_at, updated_at, next_attempt_at)
                VALUES (%s, %s, %s, %s, %s::jsonb, %s::jsonb, %s, %s, %s)
                ON CONFLICT (task_key) DO NOTHING
                """,
                (key, session.call_id, session.intent_id, TicketTaskState.PENDING.value,
                 json.dumps(draft.model_dump(), ensure_ascii=False),
                 json.dumps(session.model_dump(mode="json"), ensure_ascii=False), now, now, now),
            )
        task = self.get(key)
        if task is None:  # pragma: no cover - defensive database guard
            raise RuntimeError("ticket task was not persisted")
        return task

    def update(self, key: str, **changes: object) -> TicketTask:
        import json

        allowed = {"state", "result", "error", "next_attempt_at", "attempts", "lease_token"}
        unknown = set(changes) - allowed
        if unknown:
            raise ValueError(f"unsupported task fields: {sorted(unknown)}")
        assignments: list[str] = []
        values: list[object] = []
        if "state" in changes:
            assignments.append("state = %s")
            values.append(getattr(changes["state"], "value", changes["state"]))
        if "result" in changes:
            assignments.append("result_json = %s::jsonb")
            result = changes["result"]
            values.append(json.dumps(result.model_dump() if isinstance(result, TicketResult) else result, ensure_ascii=False))
        if "error" in changes:
            assignments.append("error = %s")
            values.append(str(changes["error"]))
        if "next_attempt_at" in changes:
            assignments.append("next_attempt_at = %s")
            values.append(changes["next_attempt_at"])
        if "attempts" in changes:
            assignments.append("attempts = %s")
            values.append(int(changes["attempts"]))
        lease_token = str(changes.get("lease_token", ""))
        if lease_token:
            assignments.append("lease_token = %s")
            values.append(lease_token)
        assignments.append("updated_at = %s")
        values.append(datetime.now(timezone.utc))
        values.append(key)
        where = "task_key = %s"
        if lease_token:
            values.append(lease_token)
            where += " AND lease_token = %s"
        with self._conn.cursor() as cursor:
            cursor.execute(f"UPDATE ai_phone_ticket_tasks SET {', '.join(assignments)} WHERE {where}", values)
            if cursor.rowcount != 1:
                raise RuntimeError("ticket task lease was lost")
        task = self.get(key)
        if task is None:
            raise KeyError(key)
        return task

    def get(self, key: str) -> TicketTask | None:
        import json

        with self._conn.cursor() as cursor:
            cursor.execute(
                "SELECT task_key, call_id, intent_id, state, draft_json, session_json, result_json, error, created_at, updated_at, next_attempt_at, attempts, lease_token "
                "FROM ai_phone_ticket_tasks WHERE task_key = %s", (key,)
            )
            row = cursor.fetchone()
        if row is None:
            return None
        result = TicketResult(**row[6]) if row[6] else None
        return TicketTask(
            key=row[0], call_id=row[1], intent_id=row[2], state=TicketTaskState(row[3]),
            draft=TicketDraft(**(row[4] if isinstance(row[4], dict) else json.loads(row[4]))),
            session=VoiceSession(**(row[5] if isinstance(row[5], dict) else json.loads(row[5]))),
            created_at=row[8], updated_at=row[9], next_attempt_at=row[10] or row[9], attempts=row[11] or 0, lease_token=row[12] or "",
            result=result, error=row[7] or "",
        )
