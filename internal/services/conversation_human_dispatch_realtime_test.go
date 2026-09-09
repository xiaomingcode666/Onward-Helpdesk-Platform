package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestAIHandoffPublishesFinalAssignedConversationEvent(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	WsService = newWsService()
	session := captureHumanDispatchRealtimeSession(t, "admin:101", "admin:all")
	aiAgent := createHumanDispatchRealtimeAIAgent(t, db, "1")
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeActiveSchedule(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	conversation := createHumanDispatchRealtimeConversation(t, db, aiAgent.ID)

	result, err := ConversationHumanDispatchService.HandoffByAI(conversation.ID, aiAgent, "用户要求转人工")
	if err != nil {
		t.Fatalf("HandoffByAI() error = %v", err)
	}
	if result == nil || result.Decision != HandoffDecisionAssigned {
		t.Fatalf("expected assigned decision, got %+v", result)
	}

	event := findHumanDispatchRealtimeEvent(t, session, enums.IMRealtimeEventConversationAssigned)
	if event.Data["conversationId"] != float64(conversation.ID) {
		t.Fatalf("unexpected conversation id in event: %+v", event.Data)
	}
	if event.Data["status"] != float64(enums.IMConversationStatusPending) {
		t.Fatalf("expected assigned conversation to wait for acceptance, got %+v", event.Data["status"])
	}
	if event.Data["currentAssigneeId"] != float64(101) {
		t.Fatalf("expected assignee 101 in assigned event, got %+v", event.Data["currentAssigneeId"])
	}
}

func TestAIHandoffPublishesFinalTeamPoolConversationEvent(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	WsService = newWsService()
	session := captureHumanDispatchRealtimeSession(t, "admin:tenant:1", "admin:team:1")
	aiAgent := createHumanDispatchRealtimeAIAgent(t, db, "1")
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeActiveSchedule(t, db, 1)
	conversation := createHumanDispatchRealtimeConversation(t, db, aiAgent.ID)

	result, err := ConversationHumanDispatchService.HandoffByAI(conversation.ID, aiAgent, "用户要求转人工")
	if err != nil {
		t.Fatalf("HandoffByAI() error = %v", err)
	}
	if result == nil || result.Decision != HandoffDecisionTeamPool {
		t.Fatalf("expected team_pool decision, got %+v", result)
	}

	event := findHumanDispatchRealtimeEvent(t, session, enums.IMRealtimeEventConversationUpdated, func(event humanDispatchRealtimeEvent) bool {
		return event.Data["currentTeamId"] == float64(1)
	})
	if event.Data["conversationId"] != float64(conversation.ID) {
		t.Fatalf("unexpected conversation id in event: %+v", event.Data)
	}
	if event.Data["status"] != float64(enums.IMConversationStatusPending) {
		t.Fatalf("expected pending status in updated event, got %+v", event.Data["status"])
	}
	if value, ok := event.Data["currentAssigneeId"]; ok && value != float64(0) {
		t.Fatalf("expected no assignee in updated event, got %+v", event.Data["currentAssigneeId"])
	}
}

type humanDispatchRealtimeEvent struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

func findHumanDispatchRealtimeEvent(t *testing.T, session *ClientSession, eventType string, matchers ...func(humanDispatchRealtimeEvent) bool) humanDispatchRealtimeEvent {
	t.Helper()
	timeout := time.After(500 * time.Millisecond)
	for {
		select {
		case raw := <-session.Send:
			var event humanDispatchRealtimeEvent
			if err := json.Unmarshal(raw, &event); err != nil {
				t.Fatalf("decode realtime event: %v", err)
			}
			if event.Type != eventType {
				continue
			}
			matched := true
			for _, matcher := range matchers {
				if !matcher(event) {
					matched = false
					break
				}
			}
			if matched {
				return event
			}
		case <-timeout:
			t.Fatalf("expected realtime event %q", eventType)
		}
	}
}

func captureHumanDispatchRealtimeSession(t *testing.T, topics ...string) *ClientSession {
	t.Helper()
	session := &ClientSession{
		ID:     "test-session",
		Role:   realtimeRoleAdmin,
		Topics: map[string]struct{}{},
		Send:   make(chan []byte, 32),
	}
	WsService.manager.Register(session, topics)
	return session
}

func setupHumanDispatchRealtimeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "t_",
			SingularTable: true,
		},
	})
	if err != nil {
		t.Fatalf("open sqlite error = %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(
		&models.Tenant{},
		&models.User{},
		&models.Notification{},
		&models.Customer{},
		&models.CustomerIdentity{},
		&models.CustomerDeviceBinding{},
		&models.Channel{},
		&models.AIAgent{},
		&models.Product{},
		&models.ProductModule{},
		&models.AgentTeam{},
		&models.AgentTeamMember{},
		&models.AgentTeamSchedule{},
		&models.AgentTeamScheduleTemplate{},
		&models.AgentTeamHoliday{},
		&models.AgentScheduleException{},
		&models.AgentProfile{},
		&models.AgentWorkStatus{},
		&models.Conversation{},
		&models.ConversationParticipant{},
		&models.ConversationAssignment{},
		&models.ConversationEventLog{},
		&models.ConversationReadState{},
		&models.Message{},
		&models.Ticket{},
		&models.TicketTag{},
		&models.TicketProgress{},
		&models.TicketContextSnapshot{},
		&models.TicketDispatchAttempt{},
		&models.TicketNoSequence{},
		&models.ChannelMessageOutbox{},
		&models.DomainEvent{},
		&models.OutboxRecord{},
	); err != nil {
		t.Fatalf("auto migrate error = %v", err)
	}
	sqls.SetDB(db)
	now := time.Now()
	if err := db.Where("id = ?", int64(1)).FirstOrCreate(&models.Tenant{
		ID: 1, Name: "测试租户", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create tenant error = %v", err)
	}
	ensureEnterpriseTemplateCoversTime(t, db, 1, now)
	return db
}

func createHumanDispatchRealtimeAIAgent(t *testing.T, db *gorm.DB, teamIDs string) models.AIAgent {
	t.Helper()
	item := models.AIAgent{
		TenantID:    1,
		Name:        "测试AI",
		ServiceMode: enums.IMConversationServiceModeAIFirst,
		TeamIDs:     teamIDs,
		HandoffMode: enums.AIAgentHandoffModeDefaultTeamPool,
		Status:      enums.StatusOk,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create ai agent error = %v", err)
	}
	return item
}

func createHumanDispatchRealtimeTeam(t *testing.T, db *gorm.DB, id int64) {
	t.Helper()
	if err := db.Create(&models.AgentTeam{ID: id, TenantID: 1, Name: "售后支持组", Status: enums.StatusOk}).Error; err != nil {
		t.Fatalf("create team error = %v", err)
	}
}

func createHumanDispatchRealtimeActiveSchedule(t *testing.T, db *gorm.DB, teamID int64) {
	t.Helper()
	now := time.Now()
	minute := realtimeScheduleMinute(now)
	if err := db.Create(&models.AgentTeamSchedule{
		TenantID:      1,
		TeamID:        teamID,
		RepeatType:    AgentTeamScheduleRepeatOnce,
		DayType:       AgentTeamScheduleDayTypeWork,
		Weekday:       realtimeScheduleWeekday(now),
		StartMinute:   max(0, minute-1),
		EndMinute:     min(24*60, minute+30),
		Timezone:      EngineerScheduleTimezone,
		PublishStatus: AgentTeamSchedulePublishPublished,
		StartAt:       now.Add(-time.Hour),
		EndAt:         now.Add(time.Hour),
		Status:        enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create schedule error = %v", err)
	}
}

func realtimeScheduleWeekday(at time.Time) int {
	weekday := int(at.In(time.Local).Weekday())
	if weekday == 0 {
		return 7
	}
	return weekday
}

func realtimeScheduleMinute(at time.Time) int {
	local := at.In(time.Local)
	return local.Hour()*60 + local.Minute()
}

func createHumanDispatchRealtimeAgentProfile(t *testing.T, db *gorm.DB, userID, teamID int64) {
	t.Helper()
	now := time.Now()
	if err := db.Create(&models.User{
		ID:       userID,
		Username: fmt.Sprintf("agent-%d", userID),
		Nickname: "客服",
		Status:   enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create user error = %v", err)
	}
	if err := db.Create(&models.AgentProfile{
		TenantID:           1,
		UserID:             userID,
		TeamID:             teamID,
		AgentCode:          fmt.Sprintf("A%03d", userID),
		DisplayName:        "客服",
		ServiceStatus:      enums.ServiceStatusIdle,
		MaxConcurrentCount: 3,
		AutoAssignEnabled:  true,
		LastOnlineAt:       &now,
		Status:             enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create profile error = %v", err)
	}
	if err := db.Create(&models.AgentTeamMember{
		TenantID:        1,
		TeamID:          teamID,
		UserID:          userID,
		DispatchEnabled: true,
		DispatchWeight:  1,
		Status:          enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create team member error = %v", err)
	}
	if err := db.Create(&models.AgentWorkStatus{
		TenantID: 1, UserID: userID, Status: AgentWorkStatusAvailable,
		ConfirmedAt: now, StatusChangedAt: now,
	}).Error; err != nil {
		t.Fatalf("create work status error = %v", err)
	}
}

func createHumanDispatchRealtimeConversation(t *testing.T, db *gorm.DB, aiAgentID int64) models.Conversation {
	t.Helper()
	now := time.Now()
	if err := db.Where("id = ?", int64(1)).FirstOrCreate(&models.Customer{
		ID: 1, Name: "测试访客", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create customer error = %v", err)
	}
	item := models.Conversation{
		TenantID:      1,
		AIAgentID:     aiAgentID,
		ChannelID:     1,
		CustomerID:    1,
		CustomerName:  "测试访客",
		Status:        enums.IMConversationStatusAIServing,
		ServiceMode:   enums.IMConversationServiceModeAIFirst,
		LastMessageAt: now,
		LastActiveAt:  now,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create conversation error = %v", err)
	}
	return item
}
