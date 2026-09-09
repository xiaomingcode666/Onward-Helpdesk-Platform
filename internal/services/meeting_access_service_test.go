package services

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/providers"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestEnterpriseMeetingAccessFollowsTicketProductTeamScope(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.AgentTeam{},
		&models.AgentProfile{},
		&models.AgentTeamMember{},
		&models.Ticket{},
		&models.MeetingRoomJitsi{},
		&models.MeetingParticipant{},
		&models.TicketProgress{},
		&models.DomainEvent{},
		&models.OutboxRecord{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	previousProvider := providers.DefaultJitsiProvider
	providers.DefaultJitsiProvider = providers.NewJitsiClient(&config.JitsiConfig{AppID: "test-app", AppSecret: "test-secret"})
	t.Cleanup(func() {
		providers.DefaultJitsiProvider = previousProvider
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	teamA := models.AgentTeam{TenantID: 1, Name: "产品 A", TeamType: AgentTeamTypeProductRepair, Status: enums.StatusOk}
	teamB := models.AgentTeam{TenantID: 1, Name: "产品 B", TeamType: AgentTeamTypeProductRepair, Status: enums.StatusOk}
	if err := db.Create(&teamA).Error; err != nil {
		t.Fatalf("create product team A: %v", err)
	}
	if err := db.Create(&teamB).Error; err != nil {
		t.Fatalf("create product team B: %v", err)
	}

	assignee := createMeetingAccessEngineer(t, db, 1, teamA.ID, "meeting-assignee")
	peer := createMeetingAccessEngineer(t, db, 1, teamA.ID, "meeting-peer")
	outsider := createMeetingAccessEngineer(t, db, 1, teamB.ID, "meeting-outsider")
	manager := &dto.AuthPrincipal{TenantID: 1, UserID: 9001, Username: "service-manager", Roles: []string{EnterpriseRoleServiceManager}}
	otherTenantManager := &dto.AuthPrincipal{TenantID: 2, UserID: 9002, Username: "other-manager", Roles: []string{EnterpriseRoleServiceManager}}

	now := time.Now()
	ticket := models.Ticket{
		TenantID:          1,
		TicketNo:          "MEETING-ABAC-1",
		Title:             "产品 A 视频协作",
		Status:            enums.TicketStatusAccepted,
		CurrentTeamID:     teamA.ID,
		CurrentAssigneeID: assignee.UserID,
		AuditFields:       models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	meeting := models.MeetingRoomJitsi{
		ID:        "meeting-abac-1",
		TenantID:  1,
		TicketID:  strconv.FormatInt(ticket.ID, 10),
		RoomName:  "meeting-abac-room-1",
		Status:    "waiting",
		CreatedBy: strconv.FormatInt(assignee.UserID, 10),
		BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&meeting).Error; err != nil {
		t.Fatalf("create meeting: %v", err)
	}

	if _, err := MeetingService.ListTicketMeetingsForOperator(nil, ticket.ID, outsider); err == nil {
		t.Fatal("engineer outside the product team must not list ticket meetings")
	}
	if _, err := MeetingService.ListMeetingsByTicketForOperator(nil, strconv.FormatInt(ticket.ID, 10), outsider); err == nil {
		t.Fatal("legacy meeting list must enforce the same product-team scope")
	}
	if _, err := MeetingService.GetMeetingStatusForOperator(nil, meeting.ID, outsider); err == nil {
		t.Fatal("engineer outside the product team must not view meeting status")
	}
	if _, err := MeetingService.JoinMeetingForOperator(nil, meeting.ID, outsider); err == nil {
		t.Fatal("engineer outside the product team must not join the meeting")
	}
	if err := MeetingService.EndMeetingForOperator(nil, meeting.ID, outsider); err == nil {
		t.Fatal("engineer outside the product team must not end the meeting")
	}
	if _, err := MeetingService.JoinMeetingForOperator(nil, meeting.ID, otherTenantManager); err == nil {
		t.Fatal("cross-tenant manager must not join the meeting")
	}

	if items, err := MeetingService.ListTicketMeetingsForOperator(nil, ticket.ID, peer); err != nil || len(items) != 1 {
		t.Fatalf("same product-team engineer meeting list = %+v, %v", items, err)
	}
	if _, err := MeetingService.GetMeetingStatusForOperator(nil, meeting.ID, peer); err != nil {
		t.Fatalf("same product-team engineer should view meeting status: %v", err)
	}
	peerJoin, err := MeetingService.JoinMeetingForOperator(nil, meeting.ID, peer)
	if err != nil {
		t.Fatalf("same product-team engineer should join as a participant: %v", err)
	}
	if peerJoin.Role != "participant" || peerJoin.CanEnd {
		t.Fatalf("peer join permissions = role %q canEnd %v", peerJoin.Role, peerJoin.CanEnd)
	}
	if peerJoin.TicketID != ticket.ID || peerJoin.TicketNo != ticket.TicketNo || peerJoin.Subject != ticket.TicketNo+" · "+ticket.Title {
		t.Fatalf("peer join business context = %+v", peerJoin)
	}
	assertMeetingParticipantRole(t, db, meeting.ID, peer.UserID, "participant")
	assertMeetingParticipantNotOnline(t, db, meeting.ID, peer.UserID)
	if err := MeetingService.ConfirmJoinForOperator(nil, meeting.ID, peer); err != nil {
		t.Fatalf("confirmed Jitsi join should update attendance: %v", err)
	}
	if err := MeetingService.HeartbeatForOperator(nil, meeting.ID, peer); err != nil {
		t.Fatalf("active participant heartbeat should succeed: %v", err)
	}
	var joinedMeeting models.MeetingRoomJitsi
	if err := db.First(&joinedMeeting, "id = ?", meeting.ID).Error; err != nil || joinedMeeting.Status != "active" || joinedMeeting.StartedAt == nil {
		t.Fatalf("confirmed Jitsi join did not activate meeting: meeting=%+v err=%v", joinedMeeting, err)
	}
	if err := requireTicketMeetingEndAccess(&ticket, peer); err != nil {
		t.Fatalf("same product-team engineer should be authorized to end an orphaned meeting: %v", err)
	}
	if err := MeetingService.ConfirmLeaveForOperator(nil, meeting.ID, peer); err != nil {
		t.Fatalf("confirmed Jitsi leave should close attendance: %v", err)
	}
	if err := MeetingService.ConfirmLeaveForOperator(nil, meeting.ID, peer); err != nil {
		t.Fatalf("repeated Jitsi leave should be idempotent: %v", err)
	}
	assertMeetingParticipantLeft(t, db, meeting.ID, peer.UserID)

	assigneeJoin, err := MeetingService.JoinMeetingForOperator(nil, meeting.ID, assignee)
	if err != nil {
		t.Fatalf("ticket assignee should join as moderator: %v", err)
	}
	if assigneeJoin.Role != "moderator" || !assigneeJoin.CanEnd {
		t.Fatalf("assignee join permissions = role %q canEnd %v", assigneeJoin.Role, assigneeJoin.CanEnd)
	}
	assertMeetingParticipantRole(t, db, meeting.ID, assignee.UserID, "moderator")
	assertMeetingParticipantNotOnline(t, db, meeting.ID, assignee.UserID)
	customerJoin, err := MeetingService.JoinMeeting(nil, meeting.ID, strconv.FormatInt(assignee.UserID, 10), "Customer", "customer", ticket.TenantID)
	if err != nil {
		t.Fatalf("customer with colliding numeric id should join: %v", err)
	}
	if customerJoin.Role != "participant" || customerJoin.CanEnd {
		t.Fatalf("customer id collision join permissions = role %q canEnd %v", customerJoin.Role, customerJoin.CanEnd)
	}
	var customerParticipant models.MeetingParticipant
	if err := db.Where("meeting_id = ? AND user_id = ? AND user_type = ?", meeting.ID, strconv.FormatInt(assignee.UserID, 10), "customer").First(&customerParticipant).Error; err != nil {
		t.Fatalf("find customer meeting participant: %v", err)
	}
	if customerParticipant.Role != "participant" {
		t.Fatalf("customer participant role = %q, want participant", customerParticipant.Role)
	}
	managerJoin, err := MeetingService.JoinMeetingForOperator(nil, meeting.ID, manager)
	if err != nil {
		t.Fatalf("service manager should join the meeting: %v", err)
	}
	if managerJoin.Role != "participant" || managerJoin.CanEnd {
		t.Fatalf("non-creator manager join permissions = role %q canEnd %v", managerJoin.Role, managerJoin.CanEnd)
	}
	assertMeetingParticipantRoleForType(t, db, meeting.ID, manager.UserID, meetingParticipantTypeTenantAdmin, "participant")
	changed, err := MeetingService.EndMeetingForOperatorWithResult(nil, meeting.ID, manager)
	if err != nil {
		t.Fatalf("service manager should end the meeting: %v", err)
	}
	if !changed {
		t.Fatal("first meeting end must report a state change")
	}
	changed, err = MeetingService.EndMeetingForOperatorWithResult(nil, meeting.ID, manager)
	if err != nil || changed {
		t.Fatalf("repeated meeting end = changed %v, err %v", changed, err)
	}
	var persisted models.MeetingRoomJitsi
	if err := db.First(&persisted, "id = ?", meeting.ID).Error; err != nil || persisted.Status != "ended" {
		t.Fatalf("meeting was not ended: meeting=%+v err=%v", persisted, err)
	}

	staleAt := time.Now().Add(-meetingParticipantStaleAfter - time.Minute)
	staleMeeting := models.MeetingRoomJitsi{
		ID: "meeting-stale-presence", TenantID: 1, TicketID: strconv.FormatInt(ticket.ID, 10),
		RoomName: "meeting-stale-presence-room", Status: "active", CreatedBy: strconv.FormatInt(assignee.UserID, 10),
		StartedAt: &staleAt, BaseModel: models.BaseModel{CreatedAt: staleAt, UpdatedAt: staleAt},
	}
	staleParticipant := models.MeetingParticipant{
		ID: "participant-stale-presence", MeetingID: staleMeeting.ID, UserID: strconv.FormatInt(assignee.UserID, 10),
		UserType: "enterprise", ParticipantName: assignee.Username, Role: "moderator", JoinedAt: &staleAt,
		BaseModel: models.BaseModel{CreatedAt: staleAt, UpdatedAt: staleAt},
	}
	if err := db.Create(&staleMeeting).Error; err != nil {
		t.Fatalf("create stale meeting: %v", err)
	}
	if err := db.Create(&staleParticipant).Error; err != nil {
		t.Fatalf("create stale participant: %v", err)
	}
	if ended := MeetingService.ReconcileStaleMeetingPresenceForTenant(nil, 1, 100); ended != 1 {
		t.Fatalf("reconciled stale meetings = %d, want 1", ended)
	}
	var reconciledMeeting models.MeetingRoomJitsi
	if err := db.First(&reconciledMeeting, "id = ?", staleMeeting.ID).Error; err != nil || reconciledMeeting.Status != "ended" {
		t.Fatalf("stale meeting was not ended: meeting=%+v err=%v", reconciledMeeting, err)
	}
}

func createMeetingAccessEngineer(t *testing.T, db *gorm.DB, tenantID, teamID int64, username string) *dto.AuthPrincipal {
	t.Helper()
	user := models.User{Username: username, Nickname: username, Status: enums.StatusOk}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create engineer %s: %v", username, err)
	}
	profile := models.AgentProfile{
		TenantID: tenantID, UserID: user.ID, TeamID: teamID, AgentCode: strings.ToUpper(username),
		DisplayName: username, Status: enums.StatusOk,
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create engineer profile %s: %v", username, err)
	}
	if err := db.Create(&models.AgentTeamMember{
		TenantID:        tenantID,
		TeamID:          teamID,
		UserID:          user.ID,
		DispatchEnabled: true,
		DispatchWeight:  1,
		Status:          enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create engineer team membership %s: %v", username, err)
	}
	return &dto.AuthPrincipal{
		TenantID: tenantID, UserID: user.ID, Username: username, Nickname: username,
		DomainType: models.DomainTypeEnterprise, Roles: []string{EnterpriseRoleEngineer},
	}
}

func assertMeetingParticipantRole(t *testing.T, db *gorm.DB, meetingID string, userID int64, want string) {
	assertMeetingParticipantRoleForType(t, db, meetingID, userID, meetingParticipantTypeEnterprise, want)
}

func assertMeetingParticipantRoleForType(t *testing.T, db *gorm.DB, meetingID string, userID int64, userType, want string) {
	t.Helper()
	var participant models.MeetingParticipant
	if err := db.Where("meeting_id = ? AND user_id = ? AND user_type = ?", meetingID, strconv.FormatInt(userID, 10), userType).First(&participant).Error; err != nil {
		t.Fatalf("find meeting participant: %v", err)
	}
	if participant.Role != want {
		t.Fatalf("participant role = %s, want %s", participant.Role, want)
	}
}

func assertMeetingParticipantNotOnline(t *testing.T, db *gorm.DB, meetingID string, userID int64) {
	t.Helper()
	var participant models.MeetingParticipant
	if err := db.Where("meeting_id = ? AND user_id = ? AND user_type = ?", meetingID, strconv.FormatInt(userID, 10), "enterprise").First(&participant).Error; err != nil {
		t.Fatalf("find meeting participant: %v", err)
	}
	if participant.JoinedAt != nil {
		t.Fatalf("issuing a join token must not mark participant %d online", userID)
	}
}

func assertMeetingParticipantLeft(t *testing.T, db *gorm.DB, meetingID string, userID int64) {
	t.Helper()
	var participant models.MeetingParticipant
	if err := db.Where("meeting_id = ? AND user_id = ? AND user_type = ?", meetingID, strconv.FormatInt(userID, 10), "enterprise").First(&participant).Error; err != nil {
		t.Fatalf("find meeting participant: %v", err)
	}
	if participant.JoinedAt == nil || participant.LeftAt == nil || participant.LeftAt.Before(*participant.JoinedAt) {
		t.Fatalf("participant attendance was not closed: %+v", participant)
	}
}
