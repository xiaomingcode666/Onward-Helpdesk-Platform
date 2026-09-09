package services

import (
	"fmt"
	"testing"

	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestWebhookServiceScopesUpdatesToTenant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:webhook-scope-test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.WebhookEndpoint{}); err != nil {
		t.Fatalf("migrate webhook endpoint: %v", err)
	}
	sqls.SetDB(db)

	endpoint, err := WebhookService.RegisterWebhook(nil, "11", "Tenant 11", "https://example.test/webhook", `["ticket.updated"]`, "secret")
	if err != nil {
		t.Fatalf("register webhook: %v", err)
	}
	if err := WebhookService.SetWebhookActive("12", stringID(endpoint.ID), false); err == nil {
		t.Fatal("cross-tenant webhook update should be rejected")
	}
	var unchanged models.WebhookEndpoint
	if err := db.First(&unchanged, endpoint.ID).Error; err != nil {
		t.Fatalf("reload webhook: %v", err)
	}
	if !unchanged.Active {
		t.Fatal("cross-tenant update changed webhook state")
	}

	if err := WebhookService.SetWebhookActive("11", stringID(endpoint.ID), false); err != nil {
		t.Fatalf("same-tenant webhook update: %v", err)
	}
	items, err := WebhookService.ListWebhooks("11")
	if err != nil {
		t.Fatalf("list webhooks: %v", err)
	}
	if len(items) != 1 || items[0].Active {
		t.Fatalf("webhooks = %#v, want one inactive endpoint", items)
	}
}

func stringID(value int64) string {
	return fmt.Sprintf("%d", value)
}
