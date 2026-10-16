package services

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/openidentity"
)

type voiceTicketFixture struct {
	db           *gorm.DB
	conversation models.Conversation
	external     openidentity.ExternalUser
	assetID      string
}

func setupCustomerVoiceTicket(t *testing.T, status enums.IMConversationStatus) voiceTicketFixture {
	t.Helper()
	db, op, doc := setupProjectRuntime(t)
	activateRuntime(t, doc, op)
	require.NoError(t, db.AutoMigrate(&models.Channel{}, &models.CustomerIdentity{}, &models.ConversationParticipant{}, &models.Asset{}, &models.AIAgent{}))
	stubVoiceTicketTitleLLM(t, func(context.Context, string, string) (string, error) {
		return "打印机无法开机", nil
	})

	customer := models.Customer{Name: "Voice Customer", Status: enums.StatusOk}
	require.NoError(t, db.Create(&customer).Error)
	external := openidentity.ExternalUser{ExternalSource: enums.ExternalSourceUser, ExternalID: "voice-user"}
	require.NoError(t, db.Create(&models.CustomerIdentity{CustomerID: customer.ID, ExternalSource: external.ExternalSource, ExternalID: external.ExternalID, Status: enums.StatusOk}).Error)
	channel := models.Channel{ChannelType: enums.ChannelTypeWeb, ChannelID: "voice-web", Name: "web", Status: enums.StatusOk}
	require.NoError(t, db.Create(&channel).Error)
	conversation := models.Conversation{TenantID: doc.TenantID, ChannelID: channel.ID, CustomerID: customer.ID, Status: status}
	require.NoError(t, db.Create(&conversation).Error)
	asset := models.Asset{TenantID: doc.TenantID, ConversationID: conversation.ID, AssetID: "voice-asset-1", StorageKey: "voice/asset-1.webm", Filename: "voice.webm"}
	require.NoError(t, db.Create(&asset).Error)
	return voiceTicketFixture{db: db, conversation: conversation, external: external, assetID: asset.AssetID}
}

func (f voiceTicketFixture) request(transcript string) request.CustomerVoiceTicketRequest {
	return request.CustomerVoiceTicketRequest{ConversationID: f.conversation.ID, Transcript: transcript, AudioAssetID: f.assetID, Locale: "zh-CN"}
}

func (f voiceTicketFixture) countTickets(t *testing.T) int64 {
	var count int64
	require.NoError(t, f.db.Model(&models.Ticket{}).Where("conversation_id = ?", f.conversation.ID).Count(&count).Error)
	return count
}

func (f voiceTicketFixture) systemMessages(t *testing.T) []models.Message {
	var messages []models.Message
	require.NoError(t, f.db.Where("conversation_id = ? AND sender_type = ?", f.conversation.ID, enums.IMSenderTypeSystem).Find(&messages).Error)
	return messages
}

const voiceTranscript = "你好，我家的打印机今天早上突然无法开机，指示灯一直闪红色。"

func TestCustomerVoiceTicketCreatesTicket(t *testing.T) {
	f := setupCustomerVoiceTicket(t, enums.IMConversationStatusAIServing)

	result, err := CustomerVoiceTicketService.Submit(context.Background(), f.request(voiceTranscript), f.external, "req-1")
	require.NoError(t, err)
	require.True(t, result.Created)

	var ticket models.Ticket
	require.NoError(t, f.db.First(&ticket, result.TicketID).Error)
	require.Equal(t, "打印机无法开机", ticket.Title)
	require.Equal(t, voiceTranscript, ticket.Description)
	require.Equal(t, f.conversation.ID, ticket.ConversationID)
	require.Equal(t, ticket.TicketNo, result.TicketNo)

	messages := f.systemMessages(t)
	require.Len(t, messages, 1)
	require.Contains(t, messages[0].Content, ticket.TicketNo)
}

func TestCustomerVoiceTicketAppendsToActiveTicket(t *testing.T) {
	f := setupCustomerVoiceTicket(t, enums.IMConversationStatusAIServing)
	first, err := CustomerVoiceTicketService.Submit(context.Background(), f.request(voiceTranscript), f.external, "req-1")
	require.NoError(t, err)

	second := f.request("补充：重启之后还是一样。")
	second.AudioAssetID = "voice-asset-2"
	require.NoError(t, f.db.Create(&models.Asset{TenantID: f.conversation.TenantID, ConversationID: f.conversation.ID, AssetID: "voice-asset-2", StorageKey: "voice/asset-2.webm"}).Error)
	result, err := CustomerVoiceTicketService.Submit(context.Background(), second, f.external, "req-2")
	require.NoError(t, err)
	require.False(t, result.Created)
	require.Equal(t, first.TicketID, result.TicketID)
	require.Equal(t, int64(1), f.countTickets(t))

	var progress models.TicketProgress
	require.NoError(t, f.db.Where("ticket_id = ? AND content = ?", first.TicketID, "补充：重启之后还是一样。").First(&progress).Error)
	require.True(t, progress.VisibleToCustomer)
	require.Contains(t, progress.MetadataJSON, `"audioAssetId":"voice-asset-2"`)
}

func TestCustomerVoiceTicketIsIdempotent(t *testing.T) {
	f := setupCustomerVoiceTicket(t, enums.IMConversationStatusAIServing)
	first, err := CustomerVoiceTicketService.Submit(context.Background(), f.request(voiceTranscript), f.external, "req-1")
	require.NoError(t, err)
	again, err := CustomerVoiceTicketService.Submit(context.Background(), f.request(voiceTranscript), f.external, "req-1b")
	require.NoError(t, err)

	require.Equal(t, first.TicketID, again.TicketID)
	require.True(t, again.Created)
	require.Equal(t, int64(1), f.countTickets(t))
	require.Len(t, f.systemMessages(t), 1)
	var progressCount int64
	require.NoError(t, f.db.Model(&models.TicketProgress{}).Where("ticket_id = ? AND metadata_json LIKE ?", first.TicketID, "%customer_voice%").Count(&progressCount).Error)
	require.Zero(t, progressCount)
}

func TestCustomerVoiceTicketRejectsClosedConversation(t *testing.T) {
	f := setupCustomerVoiceTicket(t, enums.IMConversationStatusClosed)
	_, err := CustomerVoiceTicketService.Submit(context.Background(), f.request(voiceTranscript), f.external, "req-1")
	requireI18nKey(t, err, "voiceTicket.conversationClosed")
	require.Zero(t, f.countTickets(t))
}

func TestCustomerVoiceTicketRejectsEmptyTranscript(t *testing.T) {
	f := setupCustomerVoiceTicket(t, enums.IMConversationStatusAIServing)
	_, err := CustomerVoiceTicketService.Submit(context.Background(), f.request("  \n "), f.external, "req-1")
	requireI18nKey(t, err, "voiceTicket.transcriptRequired")
}

func TestCustomerVoiceTicketRejectsTooLong(t *testing.T) {
	f := setupCustomerVoiceTicket(t, enums.IMConversationStatusAIServing)
	_, err := CustomerVoiceTicketService.Submit(context.Background(), f.request(strings.Repeat("长", 4001)), f.external, "req-1")
	requireI18nKey(t, err, "voiceTicket.transcriptTooLong")
}

func TestCustomerVoiceTicketRejectsOtherCustomer(t *testing.T) {
	f := setupCustomerVoiceTicket(t, enums.IMConversationStatusAIServing)
	other := openidentity.ExternalUser{ExternalSource: enums.ExternalSourceUser, ExternalID: "someone-else"}
	_, err := CustomerVoiceTicketService.Submit(context.Background(), f.request(voiceTranscript), other, "req-1")
	requireI18nKey(t, err, "error.e0222")
	require.Zero(t, f.countTickets(t))
}

func TestCustomerVoiceTicketRejectsForeignAsset(t *testing.T) {
	f := setupCustomerVoiceTicket(t, enums.IMConversationStatusAIServing)
	require.NoError(t, f.db.Create(&models.Asset{TenantID: f.conversation.TenantID, ConversationID: f.conversation.ID + 99, AssetID: "foreign-asset", StorageKey: "voice/foreign.webm"}).Error)
	req := f.request(voiceTranscript)
	req.AudioAssetID = "foreign-asset"
	_, err := CustomerVoiceTicketService.Submit(context.Background(), req, f.external, "req-1")
	requireI18nKey(t, err, "error.e0222")
	require.Zero(t, f.countTickets(t))
}
