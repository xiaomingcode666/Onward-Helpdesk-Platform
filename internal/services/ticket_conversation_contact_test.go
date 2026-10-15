package services

import (
	"testing"

	"github.com/stretchr/testify/require"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/projectconfig"
)

func TestConversationTicketCopiesWhatsAppNumberAsContactPhone(t *testing.T) {
	db, op, doc := setupProjectRuntime(t)
	enabled := false
	for i := range doc.Runtime.Channels {
		if doc.Runtime.Channels[i].Name == "whatsapp" {
			doc.Runtime.Channels[i].Enabled, enabled = true, true
		}
	}
	if !enabled {
		doc.Runtime.Channels = append(doc.Runtime.Channels, projectconfig.Channel{Name: "whatsapp", Enabled: true})
	}
	activateRuntime(t, doc, op)
	require.NoError(t, db.AutoMigrate(&models.Channel{}, &models.CustomerIdentity{}))
	customer := models.Customer{Name: "WhatsApp Customer", Status: enums.StatusOk}
	require.NoError(t, db.Create(&customer).Error)
	// Another source's identity must not be mistaken for the phone number.
	require.NoError(t, db.Create(&models.CustomerIdentity{CustomerID: customer.ID, ExternalSource: enums.ExternalSourceGuest, ExternalID: "guest-1", Status: enums.StatusOk}).Error)
	require.NoError(t, db.Create(&models.CustomerIdentity{CustomerID: customer.ID, ExternalSource: enums.ExternalSourceWhatsApp, ExternalID: "8613800000000", Status: enums.StatusOk}).Error)

	for _, tc := range []struct {
		channelType string
		want        string
	}{
		{enums.ChannelTypeWhatsApp, "+8613800000000"},
		{enums.ChannelTypeWeb, ""},
	} {
		channel := models.Channel{ChannelType: tc.channelType, ChannelID: "contact-" + tc.channelType, Name: tc.channelType, Status: enums.StatusOk}
		require.NoError(t, db.Create(&channel).Error)
		conversation := models.Conversation{TenantID: doc.TenantID, ChannelID: channel.ID, CustomerID: customer.ID, Status: enums.IMConversationStatusAIServing}
		require.NoError(t, db.Create(&conversation).Error)

		ticket, err := TicketService.CreateFromConversation(request.CreateTicketFromConversationRequest{ConversationID: conversation.ID, Title: "Need help"}, op)
		require.NoError(t, err)
		require.Equal(t, tc.want, ticket.CallerPhone, tc.channelType)
		var stored models.Ticket
		require.NoError(t, db.First(&stored, ticket.ID).Error)
		require.Equal(t, tc.want, stored.CallerPhone, tc.channelType)
	}
}
