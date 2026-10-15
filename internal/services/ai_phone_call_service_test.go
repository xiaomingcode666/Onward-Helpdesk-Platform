package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/telnyx"
)

const testPhoneNumber = "+18573940272"

type recordedCommand struct {
	ccid   string
	action string
	body   map[string]any
}

type fakeTelnyxCommander struct {
	mu       sync.Mutex
	commands []recordedCommand
	err      error
}

func (f *fakeTelnyxCommander) Command(_ context.Context, ccid, action string, body map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commands = append(f.commands, recordedCommand{ccid, action, body})
	return f.err
}

func (f *fakeTelnyxCommander) actions() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.commands))
	for _, c := range f.commands {
		out = append(out, c.action)
	}
	return out
}

func (f *fakeTelnyxCommander) last(action string) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.commands) - 1; i >= 0; i-- {
		if f.commands[i].action == action {
			return f.commands[i].body
		}
	}
	return nil
}

type scriptedLLM struct {
	mu        sync.Mutex
	responses []string
	prompts   []string
	err       error
}

func (s *scriptedLLM) chat(ctx context.Context, system, user string) (*ai.ChatCompletionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prompts = append(s.prompts, user)
	if s.err != nil {
		return nil, s.err
	}
	if ai.CapabilityScopeFromContext(ctx).TenantID != 14 {
		return nil, errors.New("LLM call is not scoped to the line tenant")
	}
	content := s.responses[0]
	s.responses = s.responses[1:]
	return &ai.ChatCompletionResult{Content: content, ModelName: "test-model"}, nil
}

type fakeTicketCreator struct {
	requests  []request.CreateTicketRequest
	operators []*dto.AuthPrincipal
	err       error
}

func (f *fakeTicketCreator) create(req request.CreateTicketRequest, operator *dto.AuthPrincipal) (*models.Ticket, error) {
	f.requests = append(f.requests, req)
	f.operators = append(f.operators, operator)
	if f.err != nil {
		return nil, f.err
	}
	ticket := &models.Ticket{TicketNo: "TK7", Status: "pending_dispatch"}
	ticket.ID = 7
	return ticket, nil
}

func newTestCallService(llm *scriptedLLM, tickets *fakeTicketCreator) (*aiPhoneCallService, *fakeTelnyxCommander) {
	commander := &fakeTelnyxCommander{}
	svc := newAIPhoneCallService()
	svc.commander = func() aiPhoneCommander { return commander }
	svc.chat = llm.chat
	svc.createTicket = tickets.create
	svc.lines = func() map[string]aiPhoneLine {
		return map[string]aiPhoneLine{testPhoneNumber: {LineID: "line-1", ProjectKey: "ai-phone", Language: "zh-CN", TenantID: 14}}
	}
	return svc, commander
}

func telnyxEvent(id, eventType string, payload map[string]any) []byte {
	base := map[string]any{"call_control_id": "v3:ccid-1", "call_leg_id": "leg-1"}
	for k, v := range payload {
		base[k] = v
	}
	raw, _ := json.Marshal(map[string]any{"data": map[string]any{"id": id, "event_type": eventType, "payload": base}})
	return raw
}

func transcriptEvent(id, text string, final bool) []byte {
	return telnyxEvent(id, "call.transcription", map[string]any{
		"transcription_data": map[string]any{"transcript": text, "is_final": final, "confidence": 0.9},
	})
}

func process(t *testing.T, svc *aiPhoneCallService, raw []byte) {
	t.Helper()
	event, err := parseTelnyxEvent(raw)
	if err != nil {
		t.Fatalf("parse event: %v", err)
	}
	if svc.markSeen(event.ID) {
		return
	}
	svc.processEvent(context.Background(), event)
}

func connectCall(t *testing.T, svc *aiPhoneCallService) {
	t.Helper()
	process(t, svc, telnyxEvent("e1", "call.initiated", map[string]any{"direction": "incoming", "to": testPhoneNumber, "from": "+8613800000000"}))
	process(t, svc, telnyxEvent("e2", "call.answered", nil))
}

func TestParseAIPhoneDecisionValidatesModelOutput(t *testing.T) {
	decision, err := parseAIPhoneDecision("```json\n{\"action\":\"prepare_ticket\",\"reply_text\":\"确认创建吗？\",\"collected_fields\":{\"device_no\":1001},\"ticket\":{\"title\":\"设备无法启动\",\"description\":\"客户设备无法启动\"}}\n```")
	if err != nil {
		t.Fatalf("valid decision rejected: %v", err)
	}
	if decision.Action != aiPhoneActionPrepareTicket || decision.CollectedFields["device_no"] != "1001" || decision.Ticket.Title != "设备无法启动" {
		t.Fatalf("decision = %+v", decision)
	}

	for name, content := range map[string]string{
		"not json":          "好的，我来帮您",
		"unknown action":    `{"action":"transfer","reply_text":"好的"}`,
		"internal fallback": `{"action":"fallback","reply_text":"好的"}`,
		"empty reply":       `{"action":"answer","reply_text":"  "}`,
		"long reply":        `{"action":"answer","reply_text":"` + strings.Repeat("长", 301) + `"}`,
		"draft missing":     `{"action":"prepare_ticket","reply_text":"确认吗？"}`,
	} {
		if _, err := parseAIPhoneDecision(content); err == nil {
			t.Fatalf("%s: invalid model output accepted", name)
		}
	}
}

func TestAIPhoneDecisionEndpointUsesBoundTenantAndReturnsStructuredDecision(t *testing.T) {
	t.Setenv("AI_PHONE_LINE_TENANTS", `{"line-1":14}`)
	llm := &scriptedLLM{responses: []string{
		`{"action":"answer","reply_text":"请先检查电源和插座。","collected_fields":{"issue_summary":"设备无法启动"}}`,
	}}
	svc, _ := newTestCallService(llm, &fakeTicketCreator{})
	response, err := svc.DecideText(context.Background(), AIPhoneDecisionRequest{
		SchemaVersion: 1,
		Provider:      "local-mic",
		CallID:        "mic-call-1",
		IntentID:      "intent_001",
		LineID:        "line-1",
		Language:      "zh-CN",
		WorkLanguage:  "zh-CN",
		ProjectKey:    "support",
		Text:          "设备无法启动",
		History: []AIPhoneDecisionHistoryTurn{
			{Role: "customer", Text: "设备无法启动"},
		},
	})
	if err != nil {
		t.Fatalf("decide text: %v", err)
	}
	if response.Action != aiPhoneActionAnswer || response.ReplyText != "请先检查电源和插座。" {
		t.Fatalf("response = %+v", response)
	}
	if response.CollectedFields["issue_summary"] != "设备无法启动" {
		t.Fatalf("collected fields = %+v", response.CollectedFields)
	}
}

func TestAIPhoneDecisionRequestValidation(t *testing.T) {
	valid := AIPhoneDecisionRequest{
		SchemaVersion: 1,
		Provider:      "local-mic",
		CallID:        "mic-call-1",
		IntentID:      "intent_001",
		Language:      "zh-CN",
		WorkLanguage:  "zh-CN",
		ProjectKey:    "support",
		Text:          "设备无法启动",
	}
	if err := valid.ValidateForHTTP(); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	valid.SchemaVersion = 2
	if err := valid.ValidateForHTTP(); err == nil {
		t.Fatal("unsupported schema version must be rejected")
	}
}

func TestAIPhoneCallAnswersGreetsAndStartsTranscription(t *testing.T) {
	svc, commander := newTestCallService(&scriptedLLM{}, &fakeTicketCreator{})
	connectCall(t, svc)

	if got := strings.Join(commander.actions(), ","); got != "answer,speak,transcription_start" {
		t.Fatalf("commands = %s", got)
	}
	start := commander.last("transcription_start")
	config := start["transcription_engine_config"].(map[string]any)
	if config["language"] != "zh" || start["transcription_tracks"] != "inbound" || start["command_id"] != "e2:transcription_start" {
		t.Fatalf("transcription_start = %v", start)
	}
}

func TestAIPhoneCallRejectsUnconfiguredNumber(t *testing.T) {
	svc, commander := newTestCallService(&scriptedLLM{}, &fakeTicketCreator{})
	process(t, svc, telnyxEvent("e1", "call.initiated", map[string]any{"direction": "incoming", "to": "+19990000000"}))
	process(t, svc, telnyxEvent("e2", "call.answered", nil))

	if got := strings.Join(commander.actions(), ","); got != "hangup,hangup" {
		t.Fatalf("commands = %s, want hangup for the unknown number and its answered event", got)
	}
}

func TestAIPhoneCallCreatesTicketAfterLLMConfirmation(t *testing.T) {
	llm := &scriptedLLM{responses: []string{
		`{"action":"clarify","reply_text":"请问设备编号是多少？","collected_fields":{"issue_summary":"设备无法启动"}}`,
		`{"action":"prepare_ticket","reply_text":"我记录为设备无法启动，没有编号。确认创建工单吗？","ticket":{"title":"设备无法启动","description":"客户反馈设备无法启动，无设备编号"}}`,
		`{"action":"confirm_ticket","reply_text":"好的，正在创建。"}`,
	}}
	tickets := &fakeTicketCreator{}
	svc, commander := newTestCallService(llm, tickets)
	connectCall(t, svc)

	process(t, svc, transcriptEvent("e3", "我的设备无法启动", true))
	process(t, svc, transcriptEvent("e4", "没有编号", true))
	process(t, svc, transcriptEvent("e5", "确认", true))

	if len(tickets.requests) != 1 {
		t.Fatalf("tickets created = %d, want 1", len(tickets.requests))
	}
	req := tickets.requests[0]
	if req.TenantID != 14 || req.Source != "ai_phone" || req.Channel != "phone" || req.ProjectKey != "ai-phone" || req.CallerPhone != "+8613800000000" {
		t.Fatalf("ticket request = %+v", req)
	}
	if req.IdempotencyKey != "telnyx:leg-1:intent_001" || req.Title != "设备无法启动" || !strings.Contains(req.Description, "客户：我的设备无法启动") {
		t.Fatalf("ticket identity/content = %q %q %q", req.IdempotencyKey, req.Title, req.Description)
	}
	if tickets.operators[0].TenantID != 14 {
		t.Fatalf("operator tenant = %d", tickets.operators[0].TenantID)
	}
	final := commander.last("speak")
	if !strings.Contains(final["payload"].(string), "T K 7") || final["client_state"] != aiPhoneHangupState {
		t.Fatalf("final speech = %v", final)
	}
	if !strings.Contains(llm.prompts[2], "设备无法启动") {
		t.Fatal("later LLM turns must include the pending ticket and history")
	}

	process(t, svc, telnyxEvent("e6", "call.speak.ended", nil))
	if strings.Contains(strings.Join(commander.actions(), ","), "hangup") {
		t.Fatal("an interrupted earlier prompt must not end the call")
	}
	process(t, svc, telnyxEvent("e7", "call.speak.ended", map[string]any{"client_state": aiPhoneHangupState}))
	if actions := commander.actions(); actions[len(actions)-1] != "hangup" {
		t.Fatalf("call not hung up after final speech: %v", actions)
	}
	process(t, svc, telnyxEvent("e8", "call.hangup", nil))
	if svc.activeCalls() != 0 {
		t.Fatal("call state must be released on hangup")
	}
}

func TestAIPhoneCallRequiresDraftBeforeConfirmation(t *testing.T) {
	tickets := &fakeTicketCreator{}
	svc, commander := newTestCallService(&scriptedLLM{responses: []string{`{"action":"confirm_ticket","reply_text":"好的"}`}}, tickets)
	connectCall(t, svc)
	process(t, svc, transcriptEvent("e3", "确认", true))

	if len(tickets.requests) != 0 {
		t.Fatal("confirmation without a prepared draft must not create a ticket")
	}
	if speech := commander.last("speak")["payload"].(string); speech == "好的" {
		t.Fatalf("unexpected speech %q", speech)
	}
}

func TestAIPhoneCallTicketFailureNeverAnnouncesNumber(t *testing.T) {
	llm := &scriptedLLM{responses: []string{
		`{"action":"prepare_ticket","reply_text":"确认创建吗？","ticket":{"title":"设备无法启动","description":"设备无法启动"}}`,
		`{"action":"confirm_ticket","reply_text":"好的"}`,
	}}
	svc, commander := newTestCallService(llm, &fakeTicketCreator{err: errors.New("database down")})
	connectCall(t, svc)
	process(t, svc, transcriptEvent("e3", "设备无法启动", true))
	process(t, svc, transcriptEvent("e4", "确认", true))

	final := commander.last("speak")
	if strings.Contains(final["payload"].(string), "工单号") || final["client_state"] != aiPhoneHangupState {
		t.Fatalf("failure speech = %v", final)
	}
}

func TestAIPhoneCallLLMFailureAsksToRepeatThenGivesUp(t *testing.T) {
	svc, commander := newTestCallService(&scriptedLLM{err: errors.New("timeout")}, &fakeTicketCreator{})
	connectCall(t, svc)
	process(t, svc, transcriptEvent("e3", "你好", true))
	if speech := commander.last("speak"); speech["client_state"] != nil || !strings.Contains(speech["payload"].(string), "再说一遍") {
		t.Fatalf("first failure speech = %v", speech)
	}
	process(t, svc, transcriptEvent("e4", "你好", true))
	process(t, svc, transcriptEvent("e5", "你好", true))
	if speech := commander.last("speak"); speech["client_state"] != aiPhoneHangupState {
		t.Fatalf("third consecutive failure must end the call: %v", speech)
	}
}

func TestAIPhoneCallEndsAfterThreeConsecutiveClarifications(t *testing.T) {
	clarify := `{"action":"clarify","reply_text":"抱歉没听清，请再说一遍。"}`
	svc, commander := newTestCallService(&scriptedLLM{responses: []string{clarify, clarify, clarify}}, &fakeTicketCreator{})
	connectCall(t, svc)
	process(t, svc, transcriptEvent("e3", "作为20名初中", true))
	process(t, svc, transcriptEvent("e4", "受冷遇", true))
	if speech := commander.last("speak"); speech["client_state"] != nil {
		t.Fatal("two clarifications must not end the call")
	}
	process(t, svc, transcriptEvent("e5", "嗯", true))
	if speech := commander.last("speak"); speech["client_state"] != aiPhoneHangupState {
		t.Fatalf("third consecutive clarification must end the call: %v", speech)
	}
}

func TestAIPhoneCallIgnoresInterimDuplicateAndPostHangupEvents(t *testing.T) {
	llm := &scriptedLLM{responses: []string{`{"action":"answer","reply_text":"请先重启设备试试。"}`}}
	svc, commander := newTestCallService(llm, &fakeTicketCreator{})
	connectCall(t, svc)
	process(t, svc, transcriptEvent("e3", "设备无法", false))
	process(t, svc, transcriptEvent("e4", "设备无法启动", true))
	process(t, svc, transcriptEvent("e4", "设备无法启动", true))
	if got := len(llm.prompts); got != 1 {
		t.Fatalf("LLM calls = %d, want 1", got)
	}
	process(t, svc, telnyxEvent("e5", "call.hangup", nil))
	before := len(commander.actions())
	process(t, svc, transcriptEvent("e6", "没有", true))
	if len(commander.actions()) != before {
		t.Fatal("transcripts after hangup must not trigger commands")
	}
}

func TestAIPhoneCallReleasesStateWhenTelnyxReportsEndedCall(t *testing.T) {
	svc, commander := newTestCallService(&scriptedLLM{responses: []string{`{"action":"answer","reply_text":"好的"}`}}, &fakeTicketCreator{})
	connectCall(t, svc)
	commander.err = telnyx.ErrCallEnded
	process(t, svc, transcriptEvent("e3", "你好", true))
	if svc.activeCalls() != 0 {
		t.Fatal("an ended call must be released")
	}
}

func TestAIPhoneCallEnqueueProcessesEventsInOrder(t *testing.T) {
	llm := &scriptedLLM{responses: []string{`{"action":"answer","reply_text":"第一句"}`, `{"action":"answer","reply_text":"第二句"}`}}
	svc, commander := newTestCallService(llm, &fakeTicketCreator{})
	for _, raw := range [][]byte{
		telnyxEvent("e1", "call.initiated", map[string]any{"direction": "incoming", "to": testPhoneNumber}),
		telnyxEvent("e2", "call.answered", nil),
		transcriptEvent("e3", "一", true),
		transcriptEvent("e4", "二", true),
	} {
		if err := svc.Enqueue(raw); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if body := commander.last("speak"); body != nil && body["payload"] == "第二句" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := commander.last("speak")["payload"]; got != "第二句" {
		t.Fatalf("last speech = %v, want 第二句; actions %v", got, commander.actions())
	}
	if err := svc.Enqueue([]byte("not json")); err == nil {
		t.Fatal("malformed events must be rejected")
	}
}

func TestAIPhoneCallAcknowledgesBeforeSlowDecision(t *testing.T) {
	svc, commander := newTestCallService(&scriptedLLM{responses: []string{`{"action":"answer","reply_text":"请先重启设备。"}`}}, &fakeTicketCreator{})
	connectCall(t, svc)
	process(t, svc, transcriptEvent("e3", "设备无法启动", true))

	commander.mu.Lock()
	defer commander.mu.Unlock()
	n := len(commander.commands)
	hold, reply := commander.commands[n-2], commander.commands[n-1]
	if hold.body["payload"] != "好的，请稍等。" || hold.body["command_id"] != "e3:hold:speak" {
		t.Fatalf("hold speech = %v", hold.body)
	}
	if reply.body["payload"] != "请先重启设备。" || reply.body["command_id"] != "e3:speak" {
		t.Fatalf("reply speech = %v", reply.body)
	}
}

func TestAIPhoneHangupStateIsBase64(t *testing.T) {
	if decoded, _ := base64.StdEncoding.DecodeString(aiPhoneHangupState); string(decoded) != "hangup" {
		t.Fatalf("hangup state = %q", aiPhoneHangupState)
	}
}
