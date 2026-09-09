package services

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestCustomerSessionTokenAcceptsWebSocketProtocolCredential(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/api/ws/open", nil)
	ctx.Request.Header.Set("Sec-WebSocket-Protocol", webSocketCustomerSessionProtocolPrefix+"jwt.test.token")

	if got := newCustomerSessionService().getCustomerSessionToken(ctx); got != "jwt.test.token" {
		t.Fatalf("customer websocket session token = %q, want %q", got, "jwt.test.token")
	}
}

func setupCustomerEntrySessionExchangeTest(t *testing.T) (*gorm.DB, models.CustomerEntrySession, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.Customer{}, &models.CustomerIdentity{}, &models.CustomerEntrySession{}, &models.CustomerPrivacyConsent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqls.SetDB(db)
	previous := config.CurrentOrDefault()
	config.SetCurrent(&config.Config{CustomerSession: config.CustomerSessionConfig{
		Secret:                  "customer-entry-session-test-secret",
		TTLMinutes:              120,
		RefreshThresholdMinutes: 30,
	}})
	t.Cleanup(func() {
		config.SetCurrent(&previous)
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	visitorToken := "entry-visitor-secret"
	expiresAt := time.Now().Add(time.Hour)
	session := models.CustomerEntrySession{
		TenantID:         11,
		ProductID:        22,
		EntryType:        "qr",
		VisitorID:        "visitor-session-test",
		VisitorTokenHash: hashCustomerEntrySecret(visitorToken),
		State:            "active",
		EntryContextJSON: `{"tenantId":11,"productId":22}`,
		ExpiresAt:        &expiresAt,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create entry session: %v", err)
	}
	consent := models.CustomerPrivacyConsent{
		TenantID: session.TenantID, ProductID: session.ProductID, EntrySessionID: session.ID,
		VisitorID: session.VisitorID, PolicyVersion: constants.CustomerPrivacyPolicyVersion,
		RequiredAccepted: true, ReceiptHash: strings.Repeat("a", 63) + "1",
		ConsentedAt: time.Now(), CreatedAt: time.Now(),
	}
	if err := db.Create(&consent).Error; err != nil {
		t.Fatalf("create privacy consent: %v", err)
	}
	return db, session, visitorToken
}

func TestCustomerSessionExchangeRejectsGuestEntryWithoutPrivacyConsent(t *testing.T) {
	db, session, visitorToken := setupCustomerEntrySessionExchangeTest(t)
	if err := db.Where("entry_session_id = ?", session.ID).Delete(&models.CustomerPrivacyConsent{}).Error; err != nil {
		t.Fatal(err)
	}
	_, err := CustomerSessionService.ExchangeEntrySession(request.ExchangeCustomerEntrySessionRequest{
		EntrySessionID: session.ID,
		VisitorID:      session.VisitorID,
		VisitorToken:   visitorToken,
	})
	if err == nil || !strings.Contains(err.Error(), "guest customer sessions are no longer supported") {
		t.Fatalf("expected guest session rejection, got %v", err)
	}
}

func TestCustomerSessionExchangeRejectsProtectedGuestEntrySession(t *testing.T) {
	_, session, visitorToken := setupCustomerEntrySessionExchangeTest(t)
	_, err := CustomerSessionService.ExchangeEntrySession(request.ExchangeCustomerEntrySessionRequest{
		EntrySessionID: session.ID,
		VisitorID:      session.VisitorID,
		VisitorToken:   visitorToken,
	})
	if err == nil || !strings.Contains(err.Error(), "guest customer sessions are no longer supported") {
		t.Fatalf("expected guest session rejection, got %v", err)
	}
}

func TestCustomerSessionExchangeRejectsWrongVisitorToken(t *testing.T) {
	_, session, _ := setupCustomerEntrySessionExchangeTest(t)
	_, err := CustomerSessionService.ExchangeEntrySession(request.ExchangeCustomerEntrySessionRequest{
		EntrySessionID: session.ID,
		VisitorID:      session.VisitorID,
		VisitorToken:   "wrong-token",
	})
	if err == nil {
		t.Fatal("expected invalid visitor token to be rejected")
	}
}

func TestCustomerSessionGuestIdentityCannotBeIssuedForAnyEntrySession(t *testing.T) {
	db, firstSession, visitorToken := setupCustomerEntrySessionExchangeTest(t)
	secondSession := firstSession
	secondSession.ID = 0
	secondVisitorToken := visitorToken + "-second-session"
	secondSession.VisitorTokenHash = hashCustomerEntrySecret(secondVisitorToken)
	if err := db.Create(&secondSession).Error; err != nil {
		t.Fatalf("create second entry session: %v", err)
	}
	secondConsent := models.CustomerPrivacyConsent{
		TenantID: secondSession.TenantID, ProductID: secondSession.ProductID, EntrySessionID: secondSession.ID,
		VisitorID: secondSession.VisitorID, PolicyVersion: constants.CustomerPrivacyPolicyVersion,
		RequiredAccepted: true, ReceiptHash: strings.Repeat("b", 64),
		ConsentedAt: time.Now(), CreatedAt: time.Now(),
	}
	if err := db.Create(&secondConsent).Error; err != nil {
		t.Fatalf("create second privacy consent: %v", err)
	}

	_, err := CustomerSessionService.ExchangeEntrySession(request.ExchangeCustomerEntrySessionRequest{
		EntrySessionID: firstSession.ID,
		VisitorID:      firstSession.VisitorID,
		VisitorToken:   visitorToken,
	})
	if err == nil {
		t.Fatal("expected first guest entry session to be rejected")
	}
	_, err = CustomerSessionService.ExchangeEntrySession(request.ExchangeCustomerEntrySessionRequest{
		EntrySessionID: secondSession.ID,
		VisitorID:      secondSession.VisitorID,
		VisitorToken:   secondVisitorToken,
	})
	if err == nil {
		t.Fatal("expected second guest entry session to be rejected")
	}
}
