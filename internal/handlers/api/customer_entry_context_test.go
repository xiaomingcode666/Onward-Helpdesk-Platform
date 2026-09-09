package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func setupCustomerEntryContextDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	modelsToMigrate := []any{
		&models.Tenant{},
		&models.Product{},
		&models.ProductServiceProfile{},
		&models.ProductModel{},
		&models.Device{},
		&models.ServiceCode{},
		&models.CustomerEntrySession{},
		&models.CustomerPrivacyConsent{},
		&models.Conversation{},
		&models.Ticket{},
		&models.TicketRepairRecord{},
		&models.TicketFeedback{},
		&models.TicketProgress{},
		&models.KnowledgeCandidate{},
		&models.TicketQualityClue{},
		&models.ProductFaultStatsDaily{},
		&models.FaultStatsEventInbox{},
		&models.DeviceServiceRecord{},
		&models.KnowledgeBase{},
		&models.KnowledgeDocument{},
		&models.KnowledgeFAQ{},
		&models.ProductKnowledgeLink{},
		&models.Asset{},
		&models.ProductManualFile{},
		&models.MeetingRoomJitsi{},
		&models.MeetingParticipant{},
		&models.TicketSupplierCollaboration{},
		&models.TicketSupplierCollaborationParticipant{},
		&models.PartnerAuthorizationScope{},
		&models.DomainEvent{},
		&models.OutboxRecord{},
	}
	for _, model := range modelsToMigrate {
		if err := db.AutoMigrate(model); err != nil {
			if strings.Contains(err.Error(), "index idx_tenant_id already exists") {
				continue
			}
			t.Fatalf("auto migrate %T: %v", model, err)
		}
	}

	return db
}

func TestCustomerEntryTicketFeedbackConfirmAndReopen(t *testing.T) {
	db := setupCustomerEntryContextDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	expiresAt := now.Add(24 * time.Hour)
	visitorToken := "customer-entry-action-secret"
	visitorTokenHash := sha256.Sum256([]byte(visitorToken))

	tenant := models.Tenant{Name: "Entry Action Tenant", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatal(err)
	}
	product := models.Product{TenantID: tenant.ID, Code: "ENTRY-ACTION", Name: "Entry Action Product", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&product).Error; err != nil {
		t.Fatal(err)
	}
	session := models.CustomerEntrySession{
		TenantID: tenant.ID, ProductID: product.ID, CustomerUserID: 42, VisitorID: "visitor-action",
		VisitorTokenHash: hex.EncodeToString(visitorTokenHash[:]), State: "active", EntryContextJSON: "{}",
		ExpiresAt: &expiresAt, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	createCustomerEntryConsent(t, db, session, now, "action")
	ticket := models.Ticket{
		TenantID: tenant.ID, ProductID: product.ID, CustomerEntrySessionID: session.ID,
		TicketNo: "TK-ENTRY-ACTION", Title: "远程产品咨询", Status: enums.TicketStatusResolved,
		FaultCode: "configuration", ResolvedAt: &now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TicketRepairRecord{
		TenantID: tenant.ID, TicketID: ticket.ID, ProductID: product.ID, Conclusion: "参数已恢复",
		Solution: "恢复推荐配置", VisibleToCustomer: true, FinishedAt: &now,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatal(err)
	}

	requestJSON := func(method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(method, path, strings.NewReader(body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Request.Header.Set("X-Customer-Entry-Visitor-Id", session.VisitorID)
		ctx.Request.Header.Set("X-Customer-Entry-Visitor-Token", visitorToken)
		ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(ticket.ID)}}
		return ctx, rec
	}

	ctx, rec := requestJSON(http.MethodPost, fmt.Sprintf("/api/customer/entry-session/tickets/%d/feedback?entrySessionId=%d", ticket.ID, session.ID), `{"rating":5,"tags":["专业"],"comment":"已解决"}`)
	CustomerPostEntry_session_ticket_feedback(ctx)
	if !strings.Contains(rec.Body.String(), `"success":true`) {
		t.Fatalf("feedback failed: %s", rec.Body.String())
	}
	ctx, rec = requestJSON(http.MethodPost, fmt.Sprintf("/api/customer/entry-session/tickets/%d/_confirm?entrySessionId=%d", ticket.ID, session.ID), `{}`)
	CustomerPostEntry_session_ticket_confirm(ctx)
	if !strings.Contains(rec.Body.String(), `"success":true`) {
		t.Fatalf("confirm failed: %s", rec.Body.String())
	}
	if stored := db.First(&models.Ticket{}, ticket.ID); stored.Error != nil {
		t.Fatal(stored.Error)
	}
	var closed models.Ticket
	if err := db.First(&closed, ticket.ID).Error; err != nil || closed.Status != enums.TicketStatusClosed {
		t.Fatalf("ticket not closed: %+v err=%v", closed, err)
	}
	ctx, rec = requestJSON(http.MethodPost, fmt.Sprintf("/api/customer/entry-session/tickets/%d/_reopen?entrySessionId=%d", ticket.ID, session.ID), `{"reason":"问题再次出现"}`)
	CustomerPostEntry_session_ticket_reopen(ctx)
	if !strings.Contains(rec.Body.String(), `"success":true`) {
		t.Fatalf("reopen failed: %s", rec.Body.String())
	}
	var reopened models.Ticket
	if err := db.First(&reopened, ticket.ID).Error; err != nil || reopened.Status != enums.TicketStatusReopened {
		t.Fatalf("ticket not reopened: %+v err=%v", reopened, err)
	}
}

func newCustomerEntryContextRequest(path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, path, nil)
	return ctx, rec
}

func decodeCustomerEntryData(t *testing.T, rec *httptest.ResponseRecorder, target any) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Message string          `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v\nbody=%s", err, rec.Body.String())
	}
	if !envelope.Success {
		t.Fatalf("success = false, message = %q, body = %s", envelope.Message, rec.Body.String())
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		t.Fatalf("decode data: %v\nbody=%s", err, rec.Body.String())
	}
}

func TestCustomerEntryContextReturnsStoredServiceData(t *testing.T) {
	db := setupCustomerEntryContextDB(t)
	config.SetCurrent(&config.Config{Storage: config.StorageConfig{
		Default: enums.AssetProviderLocal,
		Local: config.LocalStorageConfig{
			Root:    t.TempDir(),
			BaseURL: "/storage",
		},
	}})
	now := time.Now().UTC().Truncate(time.Second)
	expiresAt := now.Add(24 * time.Hour)
	visitorToken := "customer-entry-context-secret"
	visitorTokenHash := sha256.Sum256([]byte(visitorToken))

	tenant := models.Tenant{Name: "Acme Overseas", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	product := models.Product{TenantID: tenant.ID, Code: "HP-3000", Name: "Hydraulic Power Unit", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	model := models.ProductModel{TenantID: tenant.ID, ProductID: product.ID, ModelCode: "HP-A", Name: "HP Model A", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	device := models.Device{TenantID: tenant.ID, DeviceNo: "DEV-CUSTOMER-01", SerialNo: "SN-CUSTOMER-01", ProductID: product.ID, ProductModelID: model.ID, RegionCode: "US-WEST", Status: enums.StatusOk, DeviceStatus: string(enums.DeviceStatusOperational), AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&device).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	code := models.ServiceCode{TenantID: tenant.ID, ServiceCode: "SC-CUSTOMER-CTX", ProductID: product.ID, ProductModelID: model.ID, DeviceID: device.ID, Status: enums.ServiceCodeStatusBound, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&code).Error; err != nil {
		t.Fatalf("create service code: %v", err)
	}
	session := models.CustomerEntrySession{TenantID: tenant.ID, ProductID: product.ID, ProductModelID: model.ID, EntryType: "qr", ServiceCodeID: code.ID, DeviceID: device.ID, CustomerUserID: 42, VisitorID: "visitor-test", VisitorTokenHash: hex.EncodeToString(visitorTokenHash[:]), Locale: "en-US", State: "active", EntryContextJSON: "{}", ExpiresAt: &expiresAt, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create entry session: %v", err)
	}
	createCustomerEntryConsent(t, db, session, now, "context")
	conversation := models.Conversation{TenantID: tenant.ID, ProductID: product.ID, ProductModelID: model.ID, DeviceID: device.ID, ServiceCodeID: code.ID, CustomerEntrySessionID: session.ID, Status: enums.IMConversationStatusActive, ServiceMode: enums.IMConversationServiceModeAIOnly, LastMessageSummary: "Pump pressure is unstable", LastMessageAt: now, LastActiveAt: now, AIReplyRounds: 2, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	ticket := models.Ticket{TenantID: tenant.ID, ProductID: product.ID, ProductModelID: model.ID, DeviceID: device.ID, ServiceCodeID: code.ID, CustomerEntrySessionID: session.ID, ConversationID: conversation.ID, TicketNo: "TK-CUSTOMER-01", Title: "Pump pressure unstable", Source: enums.TicketSourceConversation, Status: enums.TicketStatusInProgress, FaultCode: "P-101", AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if err := db.Create(&models.DeviceServiceRecord{TenantID: tenant.ID, DeviceID: device.ID, ProductID: product.ID, ProductModelID: model.ID, SourceType: "ticket", SourceID: fmt.Sprint(ticket.ID), TicketID: ticket.ID, ServiceType: "repair", Summary: "Replaced hydraulic filter", RootCause: "Filter blockage", Solution: "Replace filter", VisibleToCustomer: true, OccurredAt: now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}).Error; err != nil {
		t.Fatalf("create service record: %v", err)
	}
	kb := models.KnowledgeBase{TenantID: tenant.ID, Name: "Customer Manuals", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&kb).Error; err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}
	doc := models.KnowledgeDocument{TenantID: tenant.ID, KnowledgeBaseID: kb.ID, Title: "HP-3000 Operation Manual", ContentType: enums.KnowledgeDocumentContentTypeMarkdown, Content: "Check pressure gauge before restart.", Status: enums.StatusOk, IndexStatus: enums.KnowledgeDocumentIndexStatusIndexed, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&doc).Error; err != nil {
		t.Fatalf("create document: %v", err)
	}
	if err := db.Create(&models.ProductKnowledgeLink{TenantID: tenant.ID, ProductID: product.ID, ProductModelID: model.ID, KnowledgeBaseID: kb.ID, KnowledgeEntryID: doc.ID, LinkType: "manual", Language: "en-US", Visibility: "customer", PublishStatus: "published", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}).Error; err != nil {
		t.Fatalf("create knowledge link: %v", err)
	}
	asset := models.Asset{TenantID: tenant.ID, AssetID: "customer-manual-asset", Provider: enums.AssetProviderLocal, StorageKey: "product-manuals/manual.pdf", Filename: "manual.pdf", FileSize: 1024, MimeType: "application/pdf", Status: enums.AssetStatusSuccess, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("create manual asset: %v", err)
	}
	if err := db.Create(&models.ProductManualFile{TenantID: tenant.ID, ProductID: product.ID, AssetID: asset.ID, Title: "HP-3000 Product Manual", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}).Error; err != nil {
		t.Fatalf("create product manual file: %v", err)
	}
	if err := db.Create(&models.MeetingRoomJitsi{ID: "meet-customer-01", TenantID: tenant.ID, TicketID: fmt.Sprint(ticket.ID), RoomName: "support-room", Status: "active", CreatedBy: "engineer", StartedAt: &now, BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now}}).Error; err != nil {
		t.Fatalf("create meeting: %v", err)
	}

	unauthorizedCtx, unauthorizedRec := newCustomerEntryContextRequest(fmt.Sprintf("/api/customer/entry-session/context?entrySessionId=%d", session.ID))
	CustomerGetEntry_session_context(unauthorizedCtx)
	if !strings.Contains(unauthorizedRec.Body.String(), `"success":false`) {
		t.Fatalf("context without visitor credential should be rejected: %s", unauthorizedRec.Body.String())
	}

	ctx, rec := newCustomerEntryContextRequest(fmt.Sprintf("/api/customer/entry-session/context?entrySessionId=%d", session.ID))
	ctx.Request.Header.Set("X-Customer-Entry-Visitor-Id", session.VisitorID)
	ctx.Request.Header.Set("X-Customer-Entry-Visitor-Token", visitorToken)
	CustomerGetEntry_session_context(ctx)

	var body struct {
		EntrySessionID int64 `json:"entrySessionId"`
		Device         struct {
			DeviceNo string `json:"deviceNo"`
		} `json:"device"`
		Tickets []struct {
			TicketNo       string `json:"ticketNo"`
			Status         string `json:"status"`
			ConversationID int64  `json:"conversationId"`
		} `json:"tickets"`
		Conversations []struct {
			Summary string `json:"summary"`
		} `json:"conversations"`
		RepairHistory []struct {
			Summary string `json:"summary"`
		} `json:"repairHistory"`
		KnowledgeEntries []struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		} `json:"knowledgeEntries"`
		ManualFiles []struct {
			Title string `json:"title"`
			URL   string `json:"url"`
		} `json:"manualFiles"`
		Meetings []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"meetings"`
	}
	decodeCustomerEntryData(t, rec, &body)
	if body.EntrySessionID != session.ID || body.Device.DeviceNo != "DEV-CUSTOMER-01" {
		t.Fatalf("context did not include entry device: %#v", body)
	}
	if len(body.Tickets) != 1 || body.Tickets[0].TicketNo != "TK-CUSTOMER-01" || body.Tickets[0].Status != "processing" || body.Tickets[0].ConversationID != conversation.ID {
		t.Fatalf("tickets not mapped from stored data: %#v", body.Tickets)
	}
	if len(body.Conversations) != 1 || body.Conversations[0].Summary != "Pump pressure is unstable" {
		t.Fatalf("conversations not mapped from stored data: %#v", body.Conversations)
	}
	if len(body.RepairHistory) != 1 || body.RepairHistory[0].Summary != "Replaced hydraulic filter" {
		t.Fatalf("repair history not mapped from stored data: %#v", body.RepairHistory)
	}
	if len(body.KnowledgeEntries) != 1 || body.KnowledgeEntries[0].Title != "HP-3000 Operation Manual" || body.KnowledgeEntries[0].Content == "" {
		t.Fatalf("knowledge entries not mapped from stored data: %#v", body.KnowledgeEntries)
	}
	if len(body.ManualFiles) != 1 || body.ManualFiles[0].Title != "HP-3000 Product Manual" || !strings.Contains(body.ManualFiles[0].URL, "product-manuals/manual.pdf") {
		t.Fatalf("manual files not mapped from product storage: %#v", body.ManualFiles)
	}
	if len(body.Meetings) != 1 || body.Meetings[0].ID != "meet-customer-01" || body.Meetings[0].Status != "active" {
		t.Fatalf("meetings not mapped from stored data: %#v", body.Meetings)
	}

	joinCtx, joinRec := newCustomerEntryContextRequest(fmt.Sprintf("/api/customer/entry-session/meetings/meet-customer-01/join?entrySessionId=%d", session.ID))
	joinCtx.Params = gin.Params{{Key: "id", Value: "meet-customer-01"}}
	joinCtx.Request.Header.Set("X-Customer-Entry-Visitor-Id", session.VisitorID)
	joinCtx.Request.Header.Set("X-Customer-Entry-Visitor-Token", visitorToken)
	CustomerGetEntry_session_meeting_join(joinCtx)
	var joinBody struct {
		MeetingID string `json:"meetingId"`
		RoomName  string `json:"roomName"`
	}
	decodeCustomerEntryData(t, joinRec, &joinBody)
	if joinBody.MeetingID != "meet-customer-01" || joinBody.RoomName != "support-room" {
		t.Fatalf("unexpected customer entry join config: %#v", joinBody)
	}
	var participant models.MeetingParticipant
	if err := db.Where("meeting_id = ? AND user_id = ? AND user_type = ?", "meet-customer-01", "customer-user-42", "customer").First(&participant).Error; err != nil {
		t.Fatalf("customer entry participant was not recorded: %v", err)
	}

	foreignSession := models.CustomerEntrySession{
		TenantID: tenant.ID, ProductID: product.ID, VisitorID: "visitor-foreign",
		VisitorTokenHash: hashCustomerEntryTestToken("foreign-entry-token"), State: "active", EntryContextJSON: "{}",
		ExpiresAt: &expiresAt, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&foreignSession).Error; err != nil {
		t.Fatalf("create foreign entry session: %v", err)
	}
	foreignCtx, foreignRec := newCustomerEntryContextRequest(fmt.Sprintf("/api/customer/entry-session/meetings/meet-customer-01/join?entrySessionId=%d", foreignSession.ID))
	foreignCtx.Params = gin.Params{{Key: "id", Value: "meet-customer-01"}}
	foreignCtx.Request.Header.Set("X-Customer-Entry-Visitor-Id", foreignSession.VisitorID)
	foreignCtx.Request.Header.Set("X-Customer-Entry-Visitor-Token", "foreign-entry-token")
	CustomerGetEntry_session_meeting_join(foreignCtx)
	if !strings.Contains(foreignRec.Body.String(), `"success":false`) {
		t.Fatalf("foreign entry session should not join meeting: %s", foreignRec.Body.String())
	}
}

func createCustomerEntryConsent(t *testing.T, db *gorm.DB, session models.CustomerEntrySession, now time.Time, suffix string) {
	t.Helper()
	item := models.CustomerPrivacyConsent{
		TenantID: session.TenantID, ProductID: session.ProductID, EntrySessionID: session.ID,
		VisitorID: session.VisitorID, PolicyVersion: constants.CustomerPrivacyPolicyVersion,
		RequiredAccepted: true, ReceiptHash: fmt.Sprintf("%064s", suffix),
		ConsentedAt: now, CreatedAt: now,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create privacy consent: %v", err)
	}
}

func hashCustomerEntryTestToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
