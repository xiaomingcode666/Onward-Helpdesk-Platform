package services

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestChannelServiceRejectsAgentWithoutPublishedWorkflow(t *testing.T) {
	db := setupChannelServiceTestDB(t)
	agent := createChannelServiceTestAgent(t, db, 0)

	_, err := ChannelService.CreateChannel(request.CreateChannelRequest{
		ChannelType: enums.ChannelTypeWeb,
		AIAgentID:   agent.ID,
		Name:        "官网客服",
		Status:      int(enums.StatusOk),
	}, channelServiceTestOperator())
	if err == nil {
		t.Fatalf("expected channel creation to reject unpublished ai agent")
	}
}

func TestChannelServiceAllowsAgentWithPublishedWorkflow(t *testing.T) {
	db := setupChannelServiceTestDB(t)
	agent := createChannelServiceTestAgent(t, db, 1001)

	item, err := ChannelService.CreateChannel(request.CreateChannelRequest{
		ChannelType: enums.ChannelTypeWeb,
		AIAgentID:   agent.ID,
		Name:        "官网客服",
		Status:      int(enums.StatusOk),
	}, channelServiceTestOperator())
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if item == nil || item.AIAgentID != agent.ID {
		t.Fatalf("unexpected channel: %#v", item)
	}
}

func TestChannelServiceGetEnabledChannelFallsBackToSingleEnabledWebChannel(t *testing.T) {
	db := setupChannelServiceTestDB(t)
	agent := createChannelServiceTestAgent(t, db, 1001)
	channel := createChannelServiceTestChannel(t, db, agent.ID, "web-only", enums.ChannelTypeWeb, enums.StatusOk)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("GET", "/api/channel/config", nil)

	got := ChannelService.GetEnabledChannel(ctx)
	if got == nil {
		t.Fatalf("GetEnabledChannel() = nil, want channel")
	}
	if got.ID != channel.ID {
		t.Fatalf("GetEnabledChannel().ID = %d, want %d", got.ID, channel.ID)
	}
}

func TestChannelServiceGetEnabledChannelDoesNotFallbackWhenMultipleEnabledWebChannels(t *testing.T) {
	db := setupChannelServiceTestDB(t)
	agent := createChannelServiceTestAgent(t, db, 1001)
	createChannelServiceTestChannel(t, db, agent.ID, "web-1", enums.ChannelTypeWeb, enums.StatusOk)
	createChannelServiceTestChannel(t, db, agent.ID, "web-2", enums.ChannelTypeWeb, enums.StatusOk)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("GET", "/api/channel/config", nil)

	if got := ChannelService.GetEnabledChannel(ctx); got != nil {
		t.Fatalf("GetEnabledChannel() = %#v, want nil for ambiguous default channel", got)
	}
}

func TestChannelServiceGetEnabledChannelRespectsExplicitHeader(t *testing.T) {
	db := setupChannelServiceTestDB(t)
	agent := createChannelServiceTestAgent(t, db, 1001)
	expected := createChannelServiceTestChannel(t, db, agent.ID, "web-target", enums.ChannelTypeWeb, enums.StatusOk)
	createChannelServiceTestChannel(t, db, agent.ID, "web-other", enums.ChannelTypeWeb, enums.StatusOk)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("GET", "/api/channel/config", nil)
	req.Header.Set("X-Channel-Id", expected.ChannelID)
	ctx.Request = req

	got := ChannelService.GetEnabledChannel(ctx)
	if got == nil {
		t.Fatalf("GetEnabledChannel() = nil, want channel")
	}
	if got.ID != expected.ID {
		t.Fatalf("GetEnabledChannel().ID = %d, want %d", got.ID, expected.ID)
	}
}

func setupChannelServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "t_",
			SingularTable: true,
		},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&models.AIAgent{}, &models.Channel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	return db
}

func createChannelServiceTestAgent(t *testing.T, db *gorm.DB, workflowVersionID int64) models.AIAgent {
	t.Helper()
	item := models.AIAgent{
		Name:              "测试 AI",
		Status:            enums.StatusOk,
		WorkflowVersionID: workflowVersionID,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create ai agent: %v", err)
	}
	return item
}

func createChannelServiceTestChannel(
	t *testing.T,
	db *gorm.DB,
	aiAgentID int64,
	channelID string,
	channelType string,
	status enums.Status,
) *models.Channel {
	t.Helper()
	now := time.Now()
	item := &models.Channel{
		Name:        channelID,
		ChannelType: channelType,
		ChannelID:   channelID,
		AIAgentID:   aiAgentID,
		Status:      status,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	return item
}

func channelServiceTestOperator() *dto.AuthPrincipal {
	return &dto.AuthPrincipal{UserID: 1, Username: "admin"}
}
