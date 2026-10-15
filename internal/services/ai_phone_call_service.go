package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/telnyx"
)

// AIPhoneCallService runs inbound Telnyx Voice API calls: Telnyx handles SIP,
// speech recognition and synthesis; this service keeps the conversation state,
// asks the tenant-scoped LLM for each reply and creates the ticket in-process
// once the caller confirms. See work/AI-PHONE-SDD-SPEC.md section 15.
var AIPhoneCallService = newAIPhoneCallService()

const (
	aiPhoneProvider               = "telnyx"
	aiPhoneIntentID               = "intent_001"
	aiPhoneMaxCustomerTurns       = 20
	aiPhoneMaxClarifyStreak       = 3
	aiPhoneMaxFailureStreak       = 3
	aiPhoneMaxReplyRunes          = 300
	aiPhoneDecisionTimeoutDefault = 40 * time.Second
	aiPhoneEventTimeout           = 60 * time.Second
	aiPhoneHistoryTurns           = 20
	aiPhoneSeenEventCapacity      = 10000

	aiPhoneActionAnswer        = "answer"
	aiPhoneActionClarify       = "clarify"
	aiPhoneActionPrepareTicket = "prepare_ticket"
	aiPhoneActionConfirmTicket = "confirm_ticket"
	aiPhoneActionCancel        = "cancel"
	aiPhoneActionEnd           = "end"
)

var aiPhoneHangupState = base64.StdEncoding.EncodeToString([]byte("hangup"))

type aiPhoneLanguage struct {
	transcription string
	speak         string
	voice         string
	greeting      string
	holdOn        string
	busy          string
	giveUp        string
	ticketCreated string
	ticketFailed  string
	needIssue     string
}

var aiPhoneLanguages = map[string]aiPhoneLanguage{
	"zh-CN": {
		transcription: "zh", speak: "cmn-CN", voice: "AWS.Polly.Zhiyu-Neural",
		greeting:      "您好，这里是智能客服。请简单描述您遇到的问题。",
		holdOn:        "好的，请稍等。",
		busy:          "抱歉，系统有点忙，请您再说一遍。",
		giveUp:        "抱歉，暂时无法继续处理，请稍后再联系我们。再见。",
		ticketCreated: "工单已创建，工单号 %s。我们会尽快处理，感谢来电，再见。",
		ticketFailed:  "系统暂时无法确认工单结果，我们已记录您的问题，稍后会联系您。再见。",
		needIssue:     "请先简单说明您遇到的问题，我再帮您创建工单。",
	},
	"en-US": {
		transcription: "en", speak: "en-US", voice: "AWS.Polly.Joanna-Neural",
		greeting:      "Hello, this is the AI support line. Please describe your issue.",
		holdOn:        "One moment, please.",
		busy:          "Sorry, the system is busy. Could you say that again?",
		giveUp:        "Sorry, we cannot continue right now. Please contact us later. Goodbye.",
		ticketCreated: "Your ticket has been created. The ticket number is %s. Thank you for calling, goodbye.",
		ticketFailed:  "We could not confirm the ticket right now. We have recorded your issue and will contact you. Goodbye.",
		needIssue:     "Please describe your issue first, then I can create a ticket.",
	},
	"ar": {
		transcription: "ar", speak: "arb", voice: "AWS.Polly.Zeina",
		greeting:      "مرحباً، هذا خط الدعم الذكي. يرجى وصف مشكلتك.",
		holdOn:        "لحظة من فضلك.",
		busy:          "عذراً، النظام مشغول. هل يمكنك الإعادة؟",
		giveUp:        "عذراً، لا يمكننا المتابعة الآن. يرجى التواصل معنا لاحقاً. مع السلامة.",
		ticketCreated: "تم إنشاء التذكرة برقم %s. شكراً لاتصالك، مع السلامة.",
		ticketFailed:  "تعذر تأكيد التذكرة الآن. سجلنا مشكلتك وسنتواصل معك. مع السلامة.",
		needIssue:     "يرجى وصف مشكلتك أولاً ثم سأنشئ التذكرة.",
	},
}

type aiPhoneCommander interface {
	Command(ctx context.Context, callControlID, action string, body map[string]any) error
}

type aiPhoneLine struct {
	LineID     string
	ProjectKey string
	Language   string
	TenantID   int64
}

type aiPhoneTurn struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

type aiPhoneTicketDraft struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type aiPhoneDecision struct {
	Action          string
	ReplyText       string
	CollectedFields map[string]string
	MissingFields   []string
	Ticket          *aiPhoneTicketDraft
}

type aiPhoneCall struct {
	callControlID  string
	callID         string
	line           aiPhoneLine
	callerPhone    string
	answered       bool
	history        []aiPhoneTurn
	fields         map[string]string
	pendingTicket  *aiPhoneTicketDraft
	customerTurns  int
	clarifyStreak  int
	failureStreak  int
	endAfterSpeech bool
}

type aiPhoneQueue struct {
	events  []telnyxVoiceEvent
	running bool
}

type telnyxVoiceEvent struct {
	ID        string
	EventType string
	Payload   telnyxVoicePayload
}

type telnyxVoicePayload struct {
	CallControlID     string `json:"call_control_id"`
	CallLegID         string `json:"call_leg_id"`
	From              string `json:"from"`
	To                string `json:"to"`
	Direction         string `json:"direction"`
	ClientState       string `json:"client_state"`
	TranscriptionData *struct {
		Transcript string  `json:"transcript"`
		IsFinal    bool    `json:"is_final"`
		Confidence float64 `json:"confidence"`
	} `json:"transcription_data"`
}

type aiPhoneCallService struct {
	commander    func() aiPhoneCommander
	chat         func(ctx context.Context, systemPrompt, userPrompt string) (*ai.ChatCompletionResult, error)
	createTicket func(req request.CreateTicketRequest, operator *dto.AuthPrincipal) (*models.Ticket, error)
	lines        func() map[string]aiPhoneLine
	// decisionTimeout bounds one LLM turn; the caller hears silence meanwhile.
	decisionTimeout time.Duration
	// reasoningEffort lowers reasoning-model latency; empty keeps the provider default.
	reasoningEffort string

	mu        sync.Mutex
	calls     map[string]*aiPhoneCall
	queues    map[string]*aiPhoneQueue
	seen      map[string]struct{}
	seenOrder []string
}

func newAIPhoneCallService() *aiPhoneCallService {
	return &aiPhoneCallService{
		commander: func() aiPhoneCommander {
			return &telnyx.Client{APIKey: strings.TrimSpace(os.Getenv("TELNYX_API_KEY"))}
		},
		chat:            aiPhoneChat,
		createTicket:    TicketService.CreateTicket,
		lines:           aiPhoneLinesFromEnv,
		decisionTimeout: aiPhoneDecisionTimeoutDefault,
		calls:           map[string]*aiPhoneCall{},
		queues:          map[string]*aiPhoneQueue{},
		seen:            map[string]struct{}{},
	}
}

// AIPhoneWebhookAuthenticator is rebuilt per request so key rotation in the
// environment takes effect without a restart, matching the ticket endpoint.
func AIPhoneWebhookAuthenticator() *telnyx.WebhookAuthenticator {
	return telnyx.NewWebhookAuthenticator(os.Getenv("TELNYX_PUBLIC_KEY"), os.Getenv("TELNYX_WEBHOOK_TOKEN"))
}

// AIPhoneTenantForLine resolves AI_PHONE_LINE_TENANTS; the tenant is never
// taken from a request body.
func AIPhoneTenantForLine(lineID string) (int64, bool) {
	if lineID == "" {
		return 0, false
	}
	var bindings map[string]int64
	if err := json.Unmarshal([]byte(os.Getenv("AI_PHONE_LINE_TENANTS")), &bindings); err != nil {
		return 0, false
	}
	tenantID, ok := bindings[lineID]
	return tenantID, ok && tenantID > 0
}

func AIPhoneServiceAccountID() int64 {
	value, err := strconv.ParseInt(strings.TrimSpace(os.Getenv("AI_PHONE_SERVICE_ACCOUNT_ID")), 10, 64)
	if err != nil || value < 0 {
		return 0
	}
	return value
}

// NewAIPhoneOperator is the service principal that creates AI phone tickets.
func NewAIPhoneOperator(tenantID int64) *dto.AuthPrincipal {
	return &dto.AuthPrincipal{
		TenantID:         tenantID,
		Username:         "ai-phone-agent",
		DomainType:       "service_account",
		Domain:           "service_account",
		SubjectType:      "service_account",
		AuthMethod:       "ai_phone_service_token",
		Permissions:      []string{constants.PermissionTicketCreate.Code},
		ServiceAccountID: AIPhoneServiceAccountID(),
	}
}

// aiPhoneLinesFromEnv maps called numbers (TELNYX_NUMBER_LINES) to lines and
// their tenants (AI_PHONE_LINE_TENANTS). Numbers without a tenant are dropped.
func aiPhoneLinesFromEnv() map[string]aiPhoneLine {
	var raw map[string]struct {
		LineID     string `json:"line_id"`
		ProjectKey string `json:"project_key"`
		Language   string `json:"language"`
	}
	if err := json.Unmarshal([]byte(os.Getenv("TELNYX_NUMBER_LINES")), &raw); err != nil {
		return nil
	}
	lines := make(map[string]aiPhoneLine, len(raw))
	for number, item := range raw {
		tenantID, ok := AIPhoneTenantForLine(strings.TrimSpace(item.LineID))
		if !ok || strings.TrimSpace(item.ProjectKey) == "" {
			continue
		}
		lines[strings.TrimSpace(number)] = aiPhoneLine{
			LineID:     strings.TrimSpace(item.LineID),
			ProjectKey: strings.TrimSpace(item.ProjectKey),
			Language:   strings.TrimSpace(item.Language),
			TenantID:   tenantID,
		}
	}
	return lines
}

func parseTelnyxEvent(raw []byte) (telnyxVoiceEvent, error) {
	var envelope struct {
		Data struct {
			ID        string             `json:"id"`
			EventType string             `json:"event_type"`
			Payload   telnyxVoicePayload `json:"payload"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return telnyxVoiceEvent{}, fmt.Errorf("invalid telnyx event: %w", err)
	}
	return telnyxVoiceEvent{ID: envelope.Data.ID, EventType: envelope.Data.EventType, Payload: envelope.Data.Payload}, nil
}

// Enqueue accepts an authenticated webhook body and processes it in the
// background. Events of one call run strictly in arrival order.
func (s *aiPhoneCallService) Enqueue(raw []byte) error {
	event, err := parseTelnyxEvent(raw)
	if err != nil {
		return err
	}
	ccid := event.Payload.CallControlID
	if event.ID == "" || ccid == "" || s.markSeen(event.ID) {
		return nil
	}
	s.mu.Lock()
	queue := s.queues[ccid]
	if queue == nil {
		queue = &aiPhoneQueue{}
		s.queues[ccid] = queue
	}
	queue.events = append(queue.events, event)
	start := !queue.running
	queue.running = true
	s.mu.Unlock()
	if start {
		go s.drain(ccid, queue)
	}
	return nil
}

func (s *aiPhoneCallService) drain(ccid string, queue *aiPhoneQueue) {
	for {
		s.mu.Lock()
		if len(queue.events) == 0 {
			queue.running = false
			if s.calls[ccid] == nil {
				delete(s.queues, ccid)
			}
			s.mu.Unlock()
			return
		}
		event := queue.events[0]
		queue.events = queue.events[1:]
		s.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), aiPhoneEventTimeout)
		s.processEvent(ctx, event)
		cancel()
	}
}

func (s *aiPhoneCallService) markSeen(eventID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.seen[eventID]; ok {
		return true
	}
	s.seen[eventID] = struct{}{}
	s.seenOrder = append(s.seenOrder, eventID)
	if len(s.seenOrder) > aiPhoneSeenEventCapacity {
		delete(s.seen, s.seenOrder[0])
		s.seenOrder = s.seenOrder[1:]
	}
	return false
}

func (s *aiPhoneCallService) activeCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func (s *aiPhoneCallService) call(ccid string) *aiPhoneCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[ccid]
}

func (s *aiPhoneCallService) release(ccid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.calls, ccid)
}

// processEvent must only run on the call's queue goroutine (or in tests).
func (s *aiPhoneCallService) processEvent(ctx context.Context, event telnyxVoiceEvent) {
	started := time.Now()
	ccid := event.Payload.CallControlID
	call := s.call(ccid)
	var err error
	switch {
	case event.EventType == "call.initiated":
		err = s.onInitiated(ctx, event)
	case event.EventType == "call.hangup":
		s.release(ccid)
	case call == nil:
		if event.EventType == "call.answered" {
			err = s.command(ctx, ccid, "hangup", event.ID, map[string]any{})
		}
	case event.EventType == "call.answered":
		err = s.onAnswered(ctx, event, call)
	case event.EventType == "call.transcription":
		err = s.onTranscription(ctx, event, call)
	case event.EventType == "call.speak.ended":
		if call.endAfterSpeech && event.Payload.ClientState == aiPhoneHangupState {
			err = s.command(ctx, ccid, "hangup", event.ID, map[string]any{})
		}
	}
	if errors.Is(err, telnyx.ErrCallEnded) {
		s.release(ccid)
		err = nil
	}
	if err != nil {
		slog.Error("telnyx voice event failed", "event", event.EventType, "event_id", event.ID, "error", err)
		return
	}
	slog.Info("telnyx voice event handled", "event", event.EventType, "event_id", event.ID, "elapsed_ms", time.Since(started).Milliseconds())
}

func (s *aiPhoneCallService) onInitiated(ctx context.Context, event telnyxVoiceEvent) error {
	if event.Payload.Direction != "incoming" {
		return nil
	}
	ccid := event.Payload.CallControlID
	line, ok := s.lines()[strings.TrimSpace(event.Payload.To)]
	if !ok {
		slog.Warn("telnyx call to unconfigured number rejected", "to", event.Payload.To)
		return s.command(ctx, ccid, "hangup", event.ID, map[string]any{})
	}
	if _, supported := aiPhoneLanguages[line.Language]; !supported {
		line.Language = "zh-CN"
	}
	callID := strings.TrimSpace(event.Payload.CallLegID)
	if callID == "" {
		callID = ccid
	}
	s.mu.Lock()
	s.calls[ccid] = &aiPhoneCall{
		callControlID: ccid,
		callID:        callID,
		line:          line,
		callerPhone:   strings.TrimSpace(event.Payload.From),
		fields:        map[string]string{},
	}
	s.mu.Unlock()
	return s.command(ctx, ccid, "answer", event.ID, map[string]any{})
}

func (s *aiPhoneCallService) onAnswered(ctx context.Context, event telnyxVoiceEvent, call *aiPhoneCall) error {
	if call.answered {
		return nil
	}
	call.answered = true
	lang := aiPhoneLanguages[call.line.Language]
	call.history = append(call.history, aiPhoneTurn{Role: "agent", Text: lang.greeting})
	if err := s.speak(ctx, call, event.ID, lang.greeting, false); err != nil {
		return err
	}
	engine := strings.TrimSpace(os.Getenv("TELNYX_TRANSCRIPTION_ENGINE"))
	if engine == "" {
		engine = "Google"
	}
	config := map[string]any{"transcription_engine": engine, "language": lang.transcription}
	switch engine {
	case "Google":
		config["interim_results"] = false
	case "Telnyx":
		config["transcription_model"] = "openai/whisper-large-v3-turbo"
	}
	return s.command(ctx, call.callControlID, "transcription_start", event.ID, map[string]any{
		"transcription_engine":        engine,
		"transcription_engine_config": config,
		"transcription_tracks":        "inbound",
	})
}

func (s *aiPhoneCallService) onTranscription(ctx context.Context, event telnyxVoiceEvent, call *aiPhoneCall) error {
	data := event.Payload.TranscriptionData
	if data == nil || !data.IsFinal || call.endAfterSpeech {
		return nil
	}
	text := strings.TrimSpace(data.Transcript)
	if text == "" {
		return nil
	}
	lang := aiPhoneLanguages[call.line.Language]
	call.customerTurns++
	call.history = append(call.history, aiPhoneTurn{Role: "customer", Text: text})
	if call.customerTurns > aiPhoneMaxCustomerTurns {
		return s.speak(ctx, call, event.ID, lang.giveUp, true)
	}

	// The tenant LLM route can take well over ten seconds; acknowledge first so
	// the caller does not hang up on silence. Not part of the LLM history.
	if err := s.command(ctx, call.callControlID, "speak", event.ID+":hold", s.speakBody(call, lang.holdOn, false)); err != nil {
		return err
	}
	started := time.Now()
	decision, err := s.decide(ctx, call, text)
	slog.Info("ai phone decision", "call_id", call.callID, "elapsed_ms", time.Since(started).Milliseconds(), "ok", err == nil, "transcript_chars", utf8.RuneCountInString(text))
	if err != nil {
		slog.Warn("ai phone decision failed", "call_id", call.callID, "error", err)
		call.failureStreak++
		if call.failureStreak >= aiPhoneMaxFailureStreak {
			return s.speak(ctx, call, event.ID, lang.giveUp, true)
		}
		return s.speak(ctx, call, event.ID, lang.busy, false)
	}
	call.failureStreak = 0
	for key, value := range decision.CollectedFields {
		call.fields[key] = value
	}
	if decision.Action == aiPhoneActionClarify {
		call.clarifyStreak++
	} else {
		call.clarifyStreak = 0
	}

	switch decision.Action {
	case aiPhoneActionClarify:
		if call.clarifyStreak >= aiPhoneMaxClarifyStreak {
			return s.speak(ctx, call, event.ID, lang.giveUp, true)
		}
	case aiPhoneActionPrepareTicket:
		call.pendingTicket = decision.Ticket
	case aiPhoneActionConfirmTicket:
		if call.pendingTicket == nil {
			return s.speak(ctx, call, event.ID, lang.needIssue, false)
		}
		return s.submitTicket(ctx, call, event.ID)
	case aiPhoneActionCancel:
		call.pendingTicket = nil
		return s.speak(ctx, call, event.ID, decision.ReplyText, true)
	case aiPhoneActionEnd:
		return s.speak(ctx, call, event.ID, decision.ReplyText, true)
	}
	return s.speak(ctx, call, event.ID, decision.ReplyText, false)
}

func (s *aiPhoneCallService) submitTicket(ctx context.Context, call *aiPhoneCall, eventID string) error {
	lang := aiPhoneLanguages[call.line.Language]
	identity := aiPhoneProvider + ":" + call.callID + ":" + aiPhoneIntentID
	draft := call.pendingTicket
	ticket, err := s.createTicket(request.CreateTicketRequest{
		TicketIntakeInput: dto.TicketIntakeInput{
			SourceRecordID: identity,
			ProjectKey:     call.line.ProjectKey,
			TicketType:     "incident",
			CallerPhone:    call.callerPhone,
		},
		IdempotencyKey: identity,
		Title:          draft.Title,
		Description:    aiPhoneTicketDescription(draft.Description, call.history),
		Source:         string(enums.TicketSourceAIPhone),
		Channel:        "phone",
		TenantID:       call.line.TenantID,
	}, NewAIPhoneOperator(call.line.TenantID))
	if err != nil || ticket == nil {
		slog.Error("ai phone ticket creation failed", "call_id", call.callID, "tenant_id", call.line.TenantID, "error", err)
		return s.speak(ctx, call, eventID, lang.ticketFailed, true)
	}
	call.pendingTicket = nil
	slog.Info("ai phone ticket created", "call_id", call.callID, "tenant_id", call.line.TenantID, "ticket_id", ticket.ID)
	number := ticket.TicketNo
	if number == "" {
		number = strconv.FormatInt(ticket.ID, 10)
	}
	spoken := strings.Join(strings.Split(number, ""), " ")
	return s.speak(ctx, call, eventID, fmt.Sprintf(lang.ticketCreated, spoken), true)
}

func aiPhoneTicketDescription(summary string, history []aiPhoneTurn) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(summary))
	b.WriteString("\n\n通话记录：")
	for _, turn := range history {
		role := "客服"
		if turn.Role == "customer" {
			role = "客户"
		}
		b.WriteString("\n" + role + "：" + turn.Text)
	}
	return b.String()
}

func (s *aiPhoneCallService) speak(ctx context.Context, call *aiPhoneCall, eventID, text string, hangup bool) error {
	if hangup {
		call.endAfterSpeech = true
	}
	if len(call.history) == 0 || call.history[len(call.history)-1].Text != text {
		call.history = append(call.history, aiPhoneTurn{Role: "agent", Text: text})
	}
	return s.command(ctx, call.callControlID, "speak", eventID, s.speakBody(call, text, hangup))
}

func (s *aiPhoneCallService) speakBody(call *aiPhoneCall, text string, hangup bool) map[string]any {
	lang := aiPhoneLanguages[call.line.Language]
	voice := lang.voice
	var overrides map[string]string
	if json.Unmarshal([]byte(os.Getenv("TELNYX_TTS_VOICES")), &overrides) == nil && overrides[call.line.Language] != "" {
		voice = overrides[call.line.Language]
	}
	body := map[string]any{"payload": text, "voice": voice, "language": lang.speak, "stop": "all"}
	if hangup {
		body["client_state"] = aiPhoneHangupState
	}
	return body
}

// command sends one Call Control action. commandKey is the event ID, plus a
// suffix when one event issues the same action twice; the derived command_id
// lets Telnyx drop commands replayed after a webhook retry.
func (s *aiPhoneCallService) command(ctx context.Context, ccid, action, commandKey string, body map[string]any) error {
	body["command_id"] = commandKey + ":" + action
	started := time.Now()
	err := s.commander().Command(ctx, ccid, action, body)
	slog.Info("telnyx command", "action", action, "command_id", body["command_id"], "elapsed_ms", time.Since(started).Milliseconds(), "ok", err == nil)
	return err
}

// aiPhoneChat uses the tenant's configured LLM route like ai.LLM.Chat, but is
// bounded by the phone decision timeout instead of the global failover attempt
// timeout (15s), which the tenant sub2api route regularly exceeds.
func aiPhoneChat(ctx context.Context, systemPrompt, userPrompt string) (*ai.ChatCompletionResult, error) {
	config, err := ai.GetEnabledAIConfigWithContext(ctx, enums.AIModelTypeLLM)
	if err != nil {
		return nil, err
	}
	return ai.LLM.ChatWithConfig(ctx, *config, systemPrompt, userPrompt)
}

const aiPhoneSystemPrompt = `你是企业技术支持热线的电话智能客服，正在与客户实时通话。客户的话来自语音识别，可能有错字；你的回复会被语音合成直接播报。
目标：理解客户诉求；能简单回答的给出简短的通用建议；需要人工处理时收集信息，并在客户同意后创建工单。
规则：
1. 只输出一个 JSON 对象，不要输出 Markdown、代码块或其他文字。
2. reply_text 使用客户的语言（会话语言代码见 language），口语化、简短（中文不超过 60 字），不要使用列表、符号或链接。
3. 识别文本明显不通顺、无法理解时，action 为 clarify，礼貌请客户再说一遍，不要猜测，也不要把无意义的文字写入工单。
4. 需要建单时先返回 prepare_ticket：ticket.title 不超过 20 字，ticket.description 写清问题现象和设备编号等已知信息；reply_text 复述要点并询问客户是否确认创建。
5. 只有 pending_ticket 不为空且客户明确同意时，才返回 confirm_ticket。客户要修改内容时重新返回 prepare_ticket。
6. 客户不再需要帮助或要结束通话时返回 end；客户取消建单返回 cancel。普通问答返回 answer。
7. 不得编造工单号、处理时限、价格或承诺；不知道答案时说明会由人工跟进，并建议创建工单。
8. collected_fields 只填写客户明确提供的信息，可用键：issue_summary、device_no、location、contact_name。
输出格式：{"action":"answer|clarify|prepare_ticket|confirm_ticket|cancel|end","reply_text":"...","collected_fields":{},"missing_fields":[],"ticket":{"title":"...","description":"..."}}`

func (s *aiPhoneCallService) decide(ctx context.Context, call *aiPhoneCall, text string) (*aiPhoneDecision, error) {
	history := call.history
	if len(history) > aiPhoneHistoryTurns {
		history = history[len(history)-aiPhoneHistoryTurns:]
	}
	userPrompt, err := json.Marshal(map[string]any{
		"language":               call.line.Language,
		"pending_ticket":         call.pendingTicket,
		"collected_fields":       call.fields,
		"history":                history,
		"latest_customer_speech": text,
	})
	if err != nil {
		return nil, err
	}
	ctx = ai.WithCapabilityScope(ctx, ai.CapabilityScope{TenantID: call.line.TenantID})
	if s.reasoningEffort != "" {
		ctx = ai.WithReasoningEffort(ctx, s.reasoningEffort)
	}
	ctx, cancel := context.WithTimeout(ctx, s.decisionTimeout)
	defer cancel()
	result, err := s.chat(ctx, aiPhoneSystemPrompt, string(userPrompt))
	if err != nil {
		return nil, err
	}
	decision, err := parseAIPhoneDecision(result.Content)
	if err != nil {
		return nil, fmt.Errorf("model %s: %w", result.ModelName, err)
	}
	return decision, nil
}

// parseAIPhoneDecision accepts only a well-formed structured decision; raw
// model text is never spoken to the caller.
func parseAIPhoneDecision(content string) (*aiPhoneDecision, error) {
	start, end := strings.Index(content, "{"), strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return nil, errors.New("decision is not a JSON object")
	}
	var raw struct {
		Action          string              `json:"action"`
		ReplyText       string              `json:"reply_text"`
		CollectedFields map[string]any      `json:"collected_fields"`
		MissingFields   []string            `json:"missing_fields"`
		Ticket          *aiPhoneTicketDraft `json:"ticket"`
	}
	if err := json.Unmarshal([]byte(content[start:end+1]), &raw); err != nil {
		return nil, fmt.Errorf("decision JSON: %w", err)
	}
	decision := &aiPhoneDecision{
		Action:          strings.TrimSpace(raw.Action),
		ReplyText:       strings.TrimSpace(raw.ReplyText),
		CollectedFields: map[string]string{},
		MissingFields:   raw.MissingFields,
	}
	switch decision.Action {
	case aiPhoneActionAnswer, aiPhoneActionClarify, aiPhoneActionPrepareTicket, aiPhoneActionConfirmTicket, aiPhoneActionCancel, aiPhoneActionEnd:
	default:
		return nil, fmt.Errorf("unsupported action %q", decision.Action)
	}
	if decision.ReplyText == "" || utf8.RuneCountInString(decision.ReplyText) > aiPhoneMaxReplyRunes {
		return nil, errors.New("reply_text is empty or too long")
	}
	for key, value := range raw.CollectedFields {
		if text := strings.TrimSpace(fmt.Sprint(value)); value != nil && text != "" {
			decision.CollectedFields[key] = text
		}
	}
	if decision.Action == aiPhoneActionPrepareTicket {
		if raw.Ticket == nil || strings.TrimSpace(raw.Ticket.Title) == "" || strings.TrimSpace(raw.Ticket.Description) == "" {
			return nil, errors.New("prepare_ticket requires ticket title and description")
		}
		decision.Ticket = &aiPhoneTicketDraft{Title: strings.TrimSpace(raw.Ticket.Title), Description: strings.TrimSpace(raw.Ticket.Description)}
	}
	return decision, nil
}
