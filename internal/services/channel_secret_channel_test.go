package services

import (
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
)

const waIdentifiersOnly = `{"projectKey":"p1","metaPhoneNumberId":"phone-1","metaBusinessAccountId":"waba-1"}`

func allWhatsAppSecrets() *request.WhatsAppSecretsInput {
	return &request.WhatsAppSecretsInput{MetaAccessToken: "access-1", MetaAppSecret: "app-1", WebhookVerifyToken: "verify-1"}
}

func createWhatsAppAgentForChannelTest(t *testing.T) models.AIAgent {
	t.Helper()
	agent := models.AIAgent{TenantID: 5, Name: "wa", Status: enums.StatusOk, WorkflowVersionID: 1}
	if err := sqls.DB().Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	return agent
}

func TestChannelCreateAcceptsDatabaseSecretsWithoutReferences(t *testing.T) {
	setupChannelSecretTestDB(t)
	agent := createWhatsAppAgentForChannelTest(t)
	item, err := ChannelService.CreateChannel(request.CreateChannelRequest{
		ChannelType: enums.ChannelTypeWhatsApp, AIAgentID: agent.ID, Name: "WA", Status: int(enums.StatusOk),
		ConfigJSON: waIdentifiersOnly, WhatsAppSecrets: allWhatsAppSecrets(),
	}, channelSecretTestOperator())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got := ChannelSecretService.ConfiguredNames(item.ID); len(got) != 3 {
		t.Fatalf("configured = %v", got)
	}
	if err := ChannelService.UpdateStatus(item.ID, int(enums.StatusOk), channelSecretTestOperator()); err != nil {
		t.Fatalf("enable with database secrets: %v", err)
	}
}

func TestChannelCreateRejectsMissingSecretAndWritesNothing(t *testing.T) {
	db := setupChannelSecretTestDB(t)
	agent := createWhatsAppAgentForChannelTest(t)
	_, err := ChannelService.CreateChannel(request.CreateChannelRequest{
		ChannelType: enums.ChannelTypeWhatsApp, AIAgentID: agent.ID, Name: "WA", Status: int(enums.StatusOk),
		ConfigJSON: waIdentifiersOnly, WhatsAppSecrets: &request.WhatsAppSecretsInput{MetaAccessToken: "access-1", MetaAppSecret: "app-1"},
	}, channelSecretTestOperator())
	if err == nil {
		t.Fatal("channel without verify token accepted")
	}
	var count int64
	db.Model(&models.ChannelSecret{}).Count(&count)
	if count != 0 {
		t.Fatalf("secrets written for rejected channel: %d", count)
	}
}

func TestChannelSecretsRejectedForNonWhatsAppChannel(t *testing.T) {
	setupChannelSecretTestDB(t)
	agent := createWhatsAppAgentForChannelTest(t)
	_, err := ChannelService.CreateChannel(request.CreateChannelRequest{
		ChannelType: enums.ChannelTypeWeb, AIAgentID: agent.ID, Name: "Web", Status: int(enums.StatusOk), WhatsAppSecrets: allWhatsAppSecrets(),
	}, channelSecretTestOperator())
	if err == nil {
		t.Fatal("web channel accepted WhatsApp secrets")
	}
}

func TestChannelSecretsKeptWhenSavingOtherFields(t *testing.T) {
	setupChannelSecretTestDB(t)
	agent := createWhatsAppAgentForChannelTest(t)
	item, err := ChannelService.CreateChannel(request.CreateChannelRequest{
		ChannelType: enums.ChannelTypeWhatsApp, AIAgentID: agent.ID, Name: "WA", Status: int(enums.StatusOk),
		ConfigJSON: waIdentifiersOnly, WhatsAppSecrets: allWhatsAppSecrets(),
	}, channelSecretTestOperator())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := ChannelService.UpdateChannel(request.UpdateChannelRequest{ID: item.ID, CreateChannelRequest: request.CreateChannelRequest{
		ChannelType: enums.ChannelTypeWhatsApp, AIAgentID: agent.ID, Name: "WA renamed", Status: int(enums.StatusOk), ConfigJSON: waIdentifiersOnly,
	}}, channelSecretTestOperator()); err != nil {
		t.Fatalf("update without secrets: %v", err)
	}
	if got, _ := ChannelSecretService.Resolve(item, ChannelSecretMetaAppSecret, ""); got != "app-1" {
		t.Fatalf("app secret changed to %q", got)
	}
	if err := ChannelService.UpdateChannel(request.UpdateChannelRequest{ID: item.ID, CreateChannelRequest: request.CreateChannelRequest{
		ChannelType: enums.ChannelTypeWhatsApp, AIAgentID: agent.ID, Name: "WA", Status: int(enums.StatusOk), ConfigJSON: waIdentifiersOnly,
		WhatsAppSecrets: &request.WhatsAppSecretsInput{Clear: []string{"metaAppSecret"}},
	}}, channelSecretTestOperator()); err == nil {
		t.Fatal("clearing the only app secret source was accepted")
	}
	if got, _ := ChannelSecretService.Resolve(item, ChannelSecretMetaAppSecret, ""); got != "app-1" {
		t.Fatalf("rejected clear still removed secret: %q", got)
	}
}

func TestChannelSecretUpdateRollsBackWhenAuditFails(t *testing.T) {
	db := setupChannelSecretTestDB(t)
	agent := createWhatsAppAgentForChannelTest(t)
	item, err := ChannelService.CreateChannel(request.CreateChannelRequest{
		ChannelType: enums.ChannelTypeWhatsApp, AIAgentID: agent.ID, Name: "WA", Status: int(enums.StatusOk),
		ConfigJSON: waIdentifiersOnly, WhatsAppSecrets: allWhatsAppSecrets(),
	}, channelSecretTestOperator())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Dropping the durable-event table makes RecordAuditTx fail inside the transaction.
	if err := db.Migrator().DropTable(&models.DomainEvent{}); err != nil {
		t.Fatalf("drop domain events: %v", err)
	}
	err = ChannelService.UpdateChannel(request.UpdateChannelRequest{ID: item.ID, CreateChannelRequest: request.CreateChannelRequest{
		ChannelType: enums.ChannelTypeWhatsApp, AIAgentID: agent.ID, Name: "WA renamed", Status: int(enums.StatusOk), ConfigJSON: waIdentifiersOnly,
		WhatsAppSecrets: &request.WhatsAppSecretsInput{MetaAppSecret: "app-2"},
	}}, channelSecretTestOperator())
	if err == nil {
		t.Fatal("update succeeded although audit failed")
	}
	reloaded := ChannelService.Get(item.ID)
	if reloaded.Name != "WA" {
		t.Fatalf("channel update not rolled back: %q", reloaded.Name)
	}
	if got, _ := ChannelSecretService.Resolve(item, ChannelSecretMetaAppSecret, ""); got != "app-1" {
		t.Fatalf("secret not rolled back: %q", got)
	}
}

func TestChannelSecretsDeletedWhenTypeChangesOrChannelDeleted(t *testing.T) {
	setupChannelSecretTestDB(t)
	agent := createWhatsAppAgentForChannelTest(t)
	create := func(name string) *models.Channel {
		item, err := ChannelService.CreateChannel(request.CreateChannelRequest{
			ChannelType: enums.ChannelTypeWhatsApp, AIAgentID: agent.ID, Name: name, Status: int(enums.StatusOk),
			ConfigJSON: waIdentifiersOnly, WhatsAppSecrets: allWhatsAppSecrets(),
		}, channelSecretTestOperator())
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		return item
	}
	switched := create("switch")
	if err := ChannelService.UpdateChannel(request.UpdateChannelRequest{ID: switched.ID, CreateChannelRequest: request.CreateChannelRequest{
		ChannelType: enums.ChannelTypeWeb, AIAgentID: agent.ID, Name: "now web", Status: int(enums.StatusOk),
	}}, channelSecretTestOperator()); err != nil {
		t.Fatalf("switch type: %v", err)
	}
	if got := ChannelSecretService.ConfiguredNames(switched.ID); len(got) != 0 {
		t.Fatalf("secrets kept after type change: %v", got)
	}
	deleted := create("delete")
	if err := ChannelService.DeleteChannel(deleted.ID, channelSecretTestOperator()); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := ChannelSecretService.ConfiguredNames(deleted.ID); len(got) != 0 {
		t.Fatalf("secrets kept after delete: %v", got)
	}
}
