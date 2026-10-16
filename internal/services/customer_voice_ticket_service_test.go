package services

import (
	"context"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/openidentity"
)

type voiceTicketFixture struct {
	db       *gorm.DB
	tenantID int64
	customer models.Customer
	external openidentity.ExternalUser
	operator *dto.AuthPrincipal
	uploads  int
}

func setupCustomerVoiceTicket(t *testing.T) *voiceTicketFixture {
	t.Helper()
	db, op, doc := setupProjectRuntime(t)
	activateRuntime(t, doc, op)
	require.NoError(t, db.AutoMigrate(&models.CustomerIdentity{}, &models.CustomerUser{}, &models.CustomerDeviceBinding{}, &models.Device{}, &models.Asset{}))
	stubVoiceTicketTitleLLM(t, func(context.Context, string, string) (string, error) {
		return "打印机无法开机", nil
	})

	customer := models.Customer{Name: "Voice Customer", Status: enums.StatusOk}
	require.NoError(t, db.Create(&customer).Error)
	external := openidentity.ExternalUser{ExternalSource: enums.ExternalSourceUser, ExternalID: "8101", ExternalName: "Voice Customer"}
	require.NoError(t, db.Create(&models.CustomerIdentity{CustomerID: customer.ID, ExternalSource: external.ExternalSource, ExternalID: external.ExternalID, Status: enums.StatusOk}).Error)
	require.NoError(t, db.Create(&models.CustomerUser{TenantID: doc.TenantID, CustomerOrgID: 1, UserID: 8101, DisplayName: "Voice Customer", Status: enums.StatusOk}).Error)

	f := &voiceTicketFixture{db: db, tenantID: doc.TenantID, customer: customer, external: external, operator: op}
	previous := uploadCustomerVoiceAudio
	uploadCustomerVoiceAudio = func(file *multipart.FileHeader, principal *dto.AuthPrincipal) (*models.Asset, error) {
		f.uploads++
		asset := &models.Asset{TenantID: principal.TenantID, AssetID: "voice-asset-" + string(rune('0'+f.uploads)), StorageKey: "voice/" + file.Filename + string(rune('0'+f.uploads)), Filename: file.Filename, MimeType: "audio/webm", Status: enums.AssetStatusSuccess, ScanStatus: models.AssetScanClean}
		return asset, db.Create(asset).Error
	}
	t.Cleanup(func() { uploadCustomerVoiceAudio = previous })
	return f
}

func (f *voiceTicketFixture) input(t *testing.T, transcript, requestKey string) CustomerVoiceTicketInput {
	return CustomerVoiceTicketInput{
		Transcript: transcript,
		RequestKey: requestKey,
		Locale:     "zh-CN",
		Audio:      makeAudioFileHeader(t, makePCM16WAV(16_000, 1)),
	}
}

func (f *voiceTicketFixture) countTickets(t *testing.T) int64 {
	var count int64
	require.NoError(t, f.db.Model(&models.Ticket{}).Where("customer_id = ?", f.customer.ID).Count(&count).Error)
	return count
}

const voiceTranscript = "你好，我家的打印机今天早上突然无法开机，指示灯一直闪红色。"

func TestCustomerVoiceTicketCreatesStandaloneTicket(t *testing.T) {
	f := setupCustomerVoiceTicket(t)

	result, err := CustomerVoiceTicketService.Submit(context.Background(), f.external, f.tenantID, f.input(t, voiceTranscript, "key-1"))
	require.NoError(t, err)

	var ticket models.Ticket
	require.NoError(t, f.db.First(&ticket, result.TicketID).Error)
	require.Equal(t, ticket.TicketNo, result.TicketNo)
	require.Equal(t, "打印机无法开机", ticket.Title)
	require.Equal(t, voiceTranscript, ticket.Description)
	require.Equal(t, enums.TicketSourceCustomerVoice, ticket.Source)
	require.Equal(t, f.customer.ID, ticket.CustomerID)
	require.Equal(t, f.tenantID, ticket.TenantID)
	require.Zero(t, ticket.ConversationID)

	var progress models.TicketProgress
	require.NoError(t, f.db.Where("ticket_id = ? AND metadata_json LIKE ?", ticket.ID, "%customer_voice%").First(&progress).Error)
	require.True(t, progress.VisibleToCustomer)
	require.Contains(t, progress.MetadataJSON, `"audioAssetId":"voice-asset-1"`)
	require.Equal(t, 1, f.uploads)

	visible, err := CustomerPortalService.ListTickets(f.external)
	require.NoError(t, err)
	require.Len(t, visible, 1)
	require.Equal(t, ticket.ID, visible[0].ID)
}

func TestCustomerVoiceTicketIsIdempotentPerRequestKey(t *testing.T) {
	f := setupCustomerVoiceTicket(t)
	first, err := CustomerVoiceTicketService.Submit(context.Background(), f.external, f.tenantID, f.input(t, voiceTranscript, "key-1"))
	require.NoError(t, err)
	again, err := CustomerVoiceTicketService.Submit(context.Background(), f.external, f.tenantID, f.input(t, voiceTranscript, "key-1"))
	require.NoError(t, err)
	require.Equal(t, first.TicketID, again.TicketID)
	require.Equal(t, int64(1), f.countTickets(t))
	require.Equal(t, 1, f.uploads, "a repeated submit must not upload the recording again")

	second, err := CustomerVoiceTicketService.Submit(context.Background(), f.external, f.tenantID, f.input(t, "第二个问题：扫描仪也坏了。", "key-2"))
	require.NoError(t, err)
	require.NotEqual(t, first.TicketID, second.TicketID)
	require.Equal(t, int64(2), f.countTickets(t))
}

func TestCustomerVoiceTicketRejectsEmptyTranscript(t *testing.T) {
	f := setupCustomerVoiceTicket(t)
	_, err := CustomerVoiceTicketService.Submit(context.Background(), f.external, f.tenantID, f.input(t, "  \n ", "key-1"))
	requireI18nKey(t, err, "voiceTicket.transcriptRequired")
	require.Zero(t, f.countTickets(t))
}

func TestCustomerVoiceTicketRejectsTooLong(t *testing.T) {
	f := setupCustomerVoiceTicket(t)
	_, err := CustomerVoiceTicketService.Submit(context.Background(), f.external, f.tenantID, f.input(t, strings.Repeat("长", 4001), "key-1"))
	requireI18nKey(t, err, "voiceTicket.transcriptTooLong")
}

func TestCustomerVoiceTicketRequiresRecording(t *testing.T) {
	f := setupCustomerVoiceTicket(t)
	input := f.input(t, voiceTranscript, "key-1")
	input.Audio = nil
	_, err := CustomerVoiceTicketService.Submit(context.Background(), f.external, f.tenantID, input)
	requireI18nKey(t, err, "voiceTicket.audioRequired")
	require.Zero(t, f.countTickets(t))
}

func TestCustomerVoiceTicketRequiresRequestKey(t *testing.T) {
	f := setupCustomerVoiceTicket(t)
	_, err := CustomerVoiceTicketService.Submit(context.Background(), f.external, f.tenantID, f.input(t, voiceTranscript, " "))
	require.Error(t, err)
	require.Zero(t, f.countTickets(t))
}

func TestCustomerVoiceTicketRejectsTenantOutsideCustomerScope(t *testing.T) {
	f := setupCustomerVoiceTicket(t)
	_, err := CustomerVoiceTicketService.Submit(context.Background(), f.external, f.tenantID+99, f.input(t, voiceTranscript, "key-1"))
	require.Error(t, err)
	require.Zero(t, f.countTickets(t))
	require.Zero(t, f.uploads)
}

func TestCustomerVoiceTicketRejectsGuestIdentity(t *testing.T) {
	f := setupCustomerVoiceTicket(t)
	guest := openidentity.ExternalUser{ExternalSource: enums.ExternalSourceGuest, ExternalID: "guest-1"}
	_, err := CustomerVoiceTicketService.Submit(context.Background(), guest, f.tenantID, f.input(t, voiceTranscript, "key-1"))
	require.Error(t, err)
	require.Zero(t, f.countTickets(t))
}

func TestCustomerVoiceTicketRecordingIsListedAsTicketAsset(t *testing.T) {
	f := setupCustomerVoiceTicket(t)
	result, err := CustomerVoiceTicketService.Submit(context.Background(), f.external, f.tenantID, f.input(t, voiceTranscript, "key-1"))
	require.NoError(t, err)

	var ticket models.Ticket
	require.NoError(t, f.db.First(&ticket, result.TicketID).Error)
	assets := EnterpriseTicketService.buildAssets(&ticket)
	require.Len(t, assets, 1)
	require.Equal(t, "voice.wav", assets[0].FileName)
	require.Equal(t, "voice-asset-1", assets[0].AssetID, "the frontend needs the asset id to play the recording")
}

func TestOperatorCanOpenVoiceTicketRecording(t *testing.T) {
	f := setupCustomerVoiceTicket(t)
	_, err := CustomerVoiceTicketService.Submit(context.Background(), f.external, f.tenantID, f.input(t, voiceTranscript, "key-1"))
	require.NoError(t, err)

	asset, err := ConversationMediaService.AuthorizeForOperator("voice-asset-1", f.operator)
	require.NoError(t, err)
	require.Equal(t, "voice-asset-1", asset.AssetID)

	outsider := *f.operator
	outsider.TenantID = f.tenantID + 99
	_, err = ConversationMediaService.AuthorizeForOperator("voice-asset-1", &outsider)
	require.Error(t, err)
}
