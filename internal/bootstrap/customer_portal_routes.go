package bootstrap

import (
	"remotehelpdesk/internal/handlers/api"

	"github.com/gin-gonic/gin"
)

func registerApiCustomerPortalRoutes(group *gin.RouterGroup) {
	group.GET("/me", api.CustomerPortalGetMe)
	group.POST("/me/update", api.CustomerPortalPostMeUpdate)
	group.POST("/presence/_heartbeat", api.CustomerPortalPostPresenceHeartbeat)
	group.GET("/account-deletion", api.CustomerPortalGetAccountDeletion)
	group.POST("/account-deletion", api.CustomerPortalPostAccountDeletion)
	group.GET("/home", api.CustomerPortalGetHome)
	group.GET("/devices/page", api.CustomerPortalGetDevicePage)
	group.GET("/devices", api.CustomerPortalGetDevices)
	group.POST("/devices/bind", api.CustomerPortalPostDeviceBind)
	group.GET("/devices/:id/access", api.CustomerPortalGetDeviceAccess)
	group.GET("/devices/:id/manuals", api.CustomerPortalGetDeviceManuals)
	group.GET("/system-intro-docs", api.CustomerPortalGetSystemIntroDocs)
	group.GET("/conversations/page", api.CustomerPortalGetConversationPage)
	group.GET("/conversations", api.CustomerPortalGetConversations)
	group.POST("/conversations/:id/_translate", api.CustomerPortalPostConversationTranslate)
	group.POST("/notifications/push-tokens", api.CustomerPortalPostMobilePushToken)
	group.POST("/notifications/push-tokens/:id/_revoke", api.CustomerPortalPostMobilePushTokenRevoke)
	group.GET("/tickets/page", api.CustomerPortalGetTicketPage)
	group.GET("/tickets", api.CustomerPortalGetTickets)
	group.GET("/tickets/:id", api.CustomerPortalGetTicketDetail)
	group.POST("/tickets/:id/feedback", api.CustomerPortalPostTicketFeedback)
	group.POST("/tickets/:id/_confirm", api.CustomerPortalPostTicketConfirm)
	group.POST("/tickets/:id/_reopen", api.CustomerPortalPostTicketReopen)
	group.GET("/meetings/page", api.CustomerPortalGetMeetingPage)
	group.GET("/meetings", api.CustomerPortalGetMeetings)
	group.GET("/meetings/:id/join", api.CustomerPortalGetMeetingJoin)
	group.POST("/meetings/:id/_joined", api.CustomerPortalPostMeetingJoined)
	group.POST("/meetings/:id/_left", api.CustomerPortalPostMeetingLeft)
	group.POST("/meetings/:id/_heartbeat", api.CustomerPortalPostMeetingHeartbeat)
	group.GET("/meetings/:id/status", api.CustomerPortalGetMeetingStatus)
	group.GET("/meetings/:id/transcripts", api.CustomerPortalGetMeetingTranscripts)
	group.GET("/meetings/:id/transcripts/page", api.CustomerPortalGetMeetingTranscriptPage)
	group.POST("/meetings/:id/transcripts", api.CustomerPortalPostMeetingTranscript)
}
