CREATE TABLE IF NOT EXISTS ai_phone_ticket_tasks (
    task_key TEXT PRIMARY KEY,
    call_id TEXT NOT NULL,
    intent_id TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('pending', 'submitting', 'result_unknown', 'created', 'needs_attention')),
    draft_json JSONB NOT NULL,
    session_json JSONB NOT NULL,
    result_json JSONB,
    error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
    ,next_attempt_at TIMESTAMPTZ NOT NULL
    ,attempts INTEGER NOT NULL DEFAULT 0
    ,lease_token TEXT NOT NULL DEFAULT ''
);

ALTER TABLE ai_phone_ticket_tasks ADD COLUMN IF NOT EXISTS session_json JSONB;
ALTER TABLE ai_phone_ticket_tasks ADD COLUMN IF NOT EXISTS next_attempt_at TIMESTAMPTZ;
ALTER TABLE ai_phone_ticket_tasks ADD COLUMN IF NOT EXISTS attempts INTEGER NOT NULL DEFAULT 0;
ALTER TABLE ai_phone_ticket_tasks ADD COLUMN IF NOT EXISTS lease_token TEXT NOT NULL DEFAULT '';
UPDATE ai_phone_ticket_tasks SET next_attempt_at = updated_at WHERE next_attempt_at IS NULL;
UPDATE ai_phone_ticket_tasks SET state = 'needs_attention', error = 'legacy task has no frozen request snapshot'
    WHERE session_json IS NULL AND state IN ('pending', 'submitting', 'result_unknown');

CREATE INDEX IF NOT EXISTS idx_ai_phone_ticket_tasks_state_updated
    ON ai_phone_ticket_tasks (state, updated_at);
