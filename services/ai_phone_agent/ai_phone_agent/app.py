from __future__ import annotations

import asyncio
import json
import logging
import os

from fastapi import FastAPI, HTTPException, Request, Response

from .decision import RuleDecisionEngine
from .go_client import GoTicketClient, HelpdeskDecisionClient
from .models import TurnRequest, TurnResponse, VoiceSession
from .service import VoiceAgentService
from .task_store import InMemoryTicketTaskStore, PostgresTicketTaskStore
from .recovery import TicketRecoveryWorker
from .telnyx import (
    TelnyxCallBridge,
    TelnyxCallControlClient,
    TelnyxSignatureError,
    TelnyxSignatureVerifier,
    TelnyxWebhookAuthenticator,
    parse_number_lines,
)

logger = logging.getLogger(__name__)


def build_service() -> VoiceAgentService:
    base_url = os.getenv("HELPDESK_GO_URL", "http://127.0.0.1:8083")
    token = os.getenv("AI_PHONE_SERVICE_TOKEN", "")
    task_database_url = os.getenv("AI_PHONE_TASK_DATABASE_URL", "").strip()
    task_store = PostgresTicketTaskStore(task_database_url) if task_database_url else InMemoryTicketTaskStore()
    engine_name = os.getenv("AI_PHONE_DECISION_ENGINE", "rule").strip().lower()
    if engine_name in {"helpdesk", "llm"}:
        timeout = float(os.getenv("AI_PHONE_DECISION_TIMEOUT", "40"))
        engine = HelpdeskDecisionClient(base_url, token, timeout=timeout)
    else:
        engine = RuleDecisionEngine()
    return VoiceAgentService(engine, GoTicketClient(base_url, token), task_store=task_store)


service = build_service()
app = FastAPI(title="Onward AI Phone Agent", version="0.1.0")
recovery_worker = TicketRecoveryWorker(service.task_store, service.ticket_client) if service.task_store and service.ticket_client else None
telnyx_auth = TelnyxWebhookAuthenticator(
    TelnyxSignatureVerifier(os.getenv("TELNYX_PUBLIC_KEY", "")),
    os.getenv("TELNYX_WEBHOOK_TOKEN", ""),
)
telnyx_bridge = TelnyxCallBridge(
    service,
    TelnyxCallControlClient(os.getenv("TELNYX_API_KEY", "")),
    parse_number_lines(os.getenv("TELNYX_NUMBER_LINES", "")),
    voices=json.loads(os.getenv("TELNYX_TTS_VOICES", "") or "{}"),
)
_telnyx_tasks: set[asyncio.Task] = set()


@app.get("/healthz")
async def healthz() -> dict[str, bool]:
    return {"ok": True}


@app.post("/v1/sessions/{call_id}")
async def start_session(call_id: str, session: VoiceSession) -> VoiceSession:
    if session.call_id != call_id:
        raise HTTPException(status_code=400, detail="call_id mismatch")
    return service.start(session)


@app.post("/v1/sessions/{call_id}/turn", response_model=TurnResponse)
async def turn(call_id: str, request: TurnRequest) -> TurnResponse:
    if request.call_id != call_id or call_id not in service.sessions:
        raise HTTPException(status_code=404, detail="voice session not found")
    decision, ticket = await service.handle_text(call_id, request.text)
    return TurnResponse(call_id=call_id, decision=decision, ticket=ticket)



@app.post("/v1/telnyx/webhook")
async def telnyx_webhook(request: Request) -> Response:
    body = await request.body()
    try:
        telnyx_auth.authenticate(body, request.headers, request.query_params.get("token"))
    except TelnyxSignatureError as exc:
        logger.warning("telnyx webhook rejected: %s", exc)
        raise HTTPException(status_code=403, detail="invalid webhook signature") from exc
    try:
        event = json.loads(body)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail="invalid JSON") from exc
    # Acknowledge immediately; Call Control commands and Go ticket calls run in the background.
    task = asyncio.create_task(telnyx_bridge.handle_event(event))
    _telnyx_tasks.add(task)
    task.add_done_callback(_telnyx_tasks.discard)
    return Response(status_code=200)
