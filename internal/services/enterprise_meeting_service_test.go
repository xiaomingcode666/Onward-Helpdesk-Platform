package services

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/providers"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestListPersonalMeetingsTenantWideViewerScope(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.MeetingRoomJitsi{}, &models.MeetingParticipant{}, &models.MeetingTranscriptSegment{}, &models.MeetingARAnnotation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqls.SetDB(db)
	previousProvider := providers.DefaultJitsiProvider
	providers.DefaultJitsiProvider = providers.NewJitsiClient(&config.JitsiConfig{
		URL: "http://localhost:8000", AppID: "test-app", AppSecret: "test-secret",
	})
	t.Cleanup(func() {
		providers.DefaultJitsiProvider = previousProvider
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	creator := models.User{Username: "meeting-tenant-creator", Nickname: "Tenant Creator"}
	other := models.User{Username: "meeting-tenant-other", Nickname: "Tenant Other"}
	if err := db.Create(&creator).Error; err != nil {
		t.Fatalf("create creator: %v", err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("create other user: %v", err)
	}
	now := time.Now()
	meetings := []models.MeetingRoomJitsi{
		{
			ID: "tenant-meeting-mine", TenantID: 1, RoomName: "tenant-meeting-mine-room",
			Status: "ended", CreatedBy: strconv.FormatInt(creator.ID, 10),
			BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now},
		},
		{
			ID: "tenant-meeting-other", TenantID: 1, RoomName: "tenant-meeting-other-room",
			Status: "ended", CreatedBy: strconv.FormatInt(other.ID, 10),
			BaseModel: models.BaseModel{CreatedAt: now.Add(-time.Minute), UpdatedAt: now},
		},
		{
			ID: "other-tenant-meeting", TenantID: 2, RoomName: "other-tenant-meeting-room",
			Status: "ended", CreatedBy: strconv.FormatInt(other.ID, 10),
			BaseModel: models.BaseModel{CreatedAt: now.Add(-2 * time.Minute), UpdatedAt: now},
		},
	}
	if err := db.Create(&meetings).Error; err != nil {
		t.Fatalf("create meetings: %v", err)
	}

	tenantWideOperators := []*dto.AuthPrincipal{
		{TenantID: 1, UserID: creator.ID, DomainType: models.DomainTypeEnterprise, Roles: []string{EnterpriseRoleOwner}},
		{TenantID: 1, UserID: creator.ID, DomainType: models.DomainTypeEnterprise, Roles: []string{EnterpriseRoleAdmin}},
		{TenantID: 1, UserID: creator.ID, DomainType: models.DomainTypeEnterprise, Roles: []string{EnterpriseRoleServiceManager}},
		{
			TenantID: 1, TargetTenantID: 1, UserID: 999, DomainType: models.DomainTypeEnterprise,
			SupportGrantID: 85, ImpersonatedBy: "admin",
		},
		{TargetTenantID: 1, UserID: 999, DomainType: models.DomainTypePlatform},
	}
	for _, operator := range tenantWideOperators {
		result, listErr := MeetingService.ListPersonalMeetingsSearchPageForOperator(nil, 1, "all", "", 1, 50, operator)
		if listErr != nil {
			t.Fatalf("list tenant meetings for %+v: %v", operator.Roles, listErr)
		}
		if result.Total != 2 || result.Summary.Mine != 2 || len(result.Items) != 2 {
			t.Fatalf("tenant-wide meeting list for roles=%v domain=%s: total=%d summary=%+v items=%d", operator.Roles, operator.DomainType, result.Total, result.Summary, len(result.Items))
		}
		for _, item := range result.Items {
			if item.ID == "other-tenant-meeting" {
				t.Fatal("cross-tenant meeting leaked into tenant-wide meeting list")
			}
		}
	}

	engineer := &dto.AuthPrincipal{
		TenantID: 1, UserID: creator.ID, DomainType: models.DomainTypeEnterprise,
		Roles: []string{EnterpriseRoleEngineer},
	}
	personal, err := MeetingService.ListPersonalMeetingsSearchPageForOperator(nil, 1, "all", "", 1, 50, engineer)
	if err != nil {
		t.Fatalf("list engineer meetings: %v", err)
	}
	if personal.Total != 1 || len(personal.Items) != 1 || personal.Items[0].ID != "tenant-meeting-mine" {
		t.Fatalf("engineer meeting scope = total %d items %+v", personal.Total, personal.Items)
	}
}

func TestListPersonalMeetingsCountsOnlyProviderConfirmedOnlineParticipants(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.MeetingRoomJitsi{}, &models.MeetingParticipant{}, &models.MeetingTranscriptSegment{}, &models.MeetingARAnnotation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqls.SetDB(db)
	previousProvider := providers.DefaultJitsiProvider
	providers.DefaultJitsiProvider = providers.NewJitsiClient(&config.JitsiConfig{
		URL: "http://localhost:8000", AppID: "test-app", AppSecret: "test-secret",
	})
	t.Cleanup(func() {
		providers.DefaultJitsiProvider = previousProvider
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	user := models.User{Username: "meeting-owner", Nickname: "Meeting Owner"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	now := time.Now()
	active := models.MeetingRoomJitsi{
		ID: "meeting-active", TenantID: 1, TicketID: "0", RoomName: "meeting-active-room",
		Status: "active", CreatedBy: strconv.FormatInt(user.ID, 10), StartedAt: &now,
		BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}
	endedAt := now.Add(time.Minute)
	ended := models.MeetingRoomJitsi{
		ID: "meeting-ended", TenantID: 1, TicketID: "0", RoomName: "meeting-ended-room",
		Status: "ended", CreatedBy: strconv.FormatInt(user.ID, 10), StartedAt: &now, EndedAt: &endedAt,
		BaseModel: models.BaseModel{CreatedAt: now.Add(-time.Minute), UpdatedAt: endedAt},
	}
	waiting := models.MeetingRoomJitsi{
		ID: "meeting-waiting", TenantID: 1, TicketID: "0", RoomName: "meeting-waiting-room",
		Status: "waiting", CreatedBy: strconv.FormatInt(user.ID, 10),
		BaseModel: models.BaseModel{CreatedAt: now.Add(-2 * time.Minute), UpdatedAt: now.Add(-2 * time.Minute)},
	}
	identityCollision := models.MeetingRoomJitsi{
		ID: "meeting-customer-id-collision", TenantID: 1, TicketID: "0", RoomName: "meeting-customer-id-collision-room",
		Status: "ended", CreatedBy: "999",
		BaseModel: models.BaseModel{CreatedAt: now.Add(-3 * time.Minute), UpdatedAt: now.Add(-3 * time.Minute)},
	}
	if err := db.Create(&[]models.MeetingRoomJitsi{active, ended, waiting, identityCollision}).Error; err != nil {
		t.Fatalf("create meetings: %v", err)
	}
	joinedAt := now.Add(-time.Minute)
	leftAt := now.Add(-30 * time.Second)
	participants := []models.MeetingParticipant{
		{ID: "registered-only", MeetingID: active.ID, UserID: strconv.FormatInt(user.ID, 10), UserType: "enterprise", Role: "moderator", BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now}},
		{ID: "online", MeetingID: active.ID, UserID: "customer-1", UserType: "customer", ParticipantName: "现场客户", Role: "participant", JoinedAt: &joinedAt, BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now}},
		{ID: "left", MeetingID: active.ID, UserID: "engineer-1", UserType: "enterprise", ParticipantName: "值班工程师", Role: "moderator", JoinedAt: &joinedAt, LeftAt: &leftAt, BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now}},
		{ID: "stale-ended", MeetingID: ended.ID, UserID: "customer-3", UserType: "customer", Role: "participant", JoinedAt: &joinedAt, BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now}},
		{ID: "customer-id-collision", MeetingID: identityCollision.ID, UserID: strconv.FormatInt(user.ID, 10), UserType: "customer", Role: "participant", JoinedAt: &joinedAt, BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&participants).Error; err != nil {
		t.Fatalf("create participants: %v", err)
	}

	result, err := MeetingService.ListPersonalMeetings(nil, 1, user.ID, "all")
	if err != nil {
		t.Fatalf("list meetings: %v", err)
	}
	if result.Summary.Active != 1 || result.Summary.Waiting != 1 || result.Summary.Ended != 1 || result.Summary.Mine != 3 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
	if result.Summary.ParticipantsOnline != 1 {
		t.Fatalf("participants online = %d, want 1", result.Summary.ParticipantsOnline)
	}
	foundActive := false
	for _, item := range result.Items {
		if item.ID == identityCollision.ID {
			t.Fatal("customer participant identity collision leaked into enterprise personal meetings")
		}
		if item.ID == active.ID && item.ParticipantCount != 2 {
			t.Fatalf("attended participant count = %d, want 2", item.ParticipantCount)
		}
		if item.ID == active.ID {
			foundActive = true
			if len(item.Participants) != 2 {
				t.Fatalf("participants = %#v, want two attended participants", item.Participants)
			}
			rolesByName := make(map[string]string, len(item.Participants))
			for _, participant := range item.Participants {
				rolesByName[participant.Name] = participant.Role
			}
			if rolesByName["现场客户"] != "participant" || rolesByName["值班工程师"] != "moderator" {
				t.Fatalf("participant names and roles = %#v", rolesByName)
			}
			if item.DurationSeconds < 59 || item.DurationSeconds > 61 {
				t.Fatalf("duration = %d, want provider attendance duration near 60 seconds", item.DurationSeconds)
			}
		}
	}
	if !foundActive {
		t.Fatal("active meeting missing from personal list")
	}
}

func TestListPersonalMeetingsFiltersVisibilityBeforePaginationAndKeepsFullSummary(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.MeetingRoomJitsi{}, &models.MeetingParticipant{}, &models.MeetingTranscriptSegment{}, &models.MeetingARAnnotation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqls.SetDB(db)
	previousProvider := providers.DefaultJitsiProvider
	providers.DefaultJitsiProvider = providers.NewJitsiClient(&config.JitsiConfig{
		URL: "http://localhost:8000", AppID: "test-app", AppSecret: "test-secret",
	})
	t.Cleanup(func() {
		providers.DefaultJitsiProvider = previousProvider
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	owner := models.User{Username: "meeting-history-owner", Nickname: "History Owner"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	now := time.Now()
	meetings := make([]models.MeetingRoomJitsi, 0, 205)
	for i := 0; i < 100; i++ {
		createdAt := now.Add(time.Duration(i) * time.Second)
		meetings = append(meetings, models.MeetingRoomJitsi{
			ID: fmt.Sprintf("other-%03d", i), TenantID: 1, RoomName: fmt.Sprintf("other-room-%03d", i),
			Status: "ended", CreatedBy: "999", BaseModel: models.BaseModel{CreatedAt: createdAt, UpdatedAt: createdAt},
		})
	}
	for i := 0; i < 105; i++ {
		createdAt := now.Add(-time.Duration(i+1) * time.Second)
		meetings = append(meetings, models.MeetingRoomJitsi{
			ID: fmt.Sprintf("mine-%03d", i), TenantID: 1, RoomName: fmt.Sprintf("mine-room-%03d", i),
			Status: "ended", CreatedBy: strconv.FormatInt(owner.ID, 10), BaseModel: models.BaseModel{CreatedAt: createdAt, UpdatedAt: createdAt},
		})
	}
	if err := db.Create(&meetings).Error; err != nil {
		t.Fatalf("create meetings: %v", err)
	}

	result, err := MeetingService.ListPersonalMeetings(nil, 1, owner.ID, "all")
	if err != nil {
		t.Fatalf("list meetings: %v", err)
	}
	if result.Summary.Mine != 105 || result.Summary.Ended != 105 || result.Total != 105 {
		t.Fatalf("full summary = %+v total=%d, want 105 visible ended meetings", result.Summary, result.Total)
	}
	if len(result.Items) != 100 || !result.HasMore {
		t.Fatalf("page size=%d hasMore=%v, want 100 and true", len(result.Items), result.HasMore)
	}
	for _, item := range result.Items {
		if !strings.HasPrefix(item.ID, "mine-") {
			t.Fatalf("unrelated tenant meeting leaked into personal page: %s", item.ID)
		}
	}
}

func TestListPersonalMeetingsSearchesTicketContextBeforePagination(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{}, &models.MeetingRoomJitsi{}, &models.MeetingParticipant{},
		&models.MeetingTranscriptSegment{}, &models.MeetingARAnnotation{},
		&models.Ticket{}, &models.Product{}, &models.Device{}, &models.Customer{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqls.SetDB(db)
	previousProvider := providers.DefaultJitsiProvider
	providers.DefaultJitsiProvider = providers.NewJitsiClient(&config.JitsiConfig{
		URL: "http://localhost:8000", AppID: "test-app", AppSecret: "test-secret",
	})
	t.Cleanup(func() {
		providers.DefaultJitsiProvider = previousProvider
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	owner := models.User{Username: "meeting-search-owner", Nickname: "Search Owner"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatalf("create owner: %v", err)
	}
	product := models.Product{TenantID: 1, Code: "PUMP", Name: "Hydraulic Pump"}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	device := models.Device{TenantID: 1, DeviceNo: "PRESS-SEARCH-001", ProductID: product.ID}
	if err := db.Create(&device).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	otherProduct := models.Product{TenantID: 1, Code: "COOLING", Name: "Cooling Tower"}
	if err := db.Create(&otherProduct).Error; err != nil {
		t.Fatalf("create other product: %v", err)
	}
	otherDevice := models.Device{TenantID: 1, DeviceNo: "PRESS-SEARCH-OTHER", ProductID: otherProduct.ID}
	if err := db.Create(&otherDevice).Error; err != nil {
		t.Fatalf("create other device: %v", err)
	}
	customer := models.Customer{Name: "Hainan Test Plant"}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	endedTicket := models.Ticket{
		TenantID: 1, TicketNo: "TK-SEARCH-ENDED", Title: "Pressure drop investigation",
		ProductID: product.ID, DeviceID: device.ID, CustomerID: customer.ID,
	}
	waitingTicket := models.Ticket{TenantID: 1, TicketNo: "TK-SEARCH-WAITING", Title: "Backup inspection"}
	otherTicket := models.Ticket{
		TenantID: 1, TicketNo: "TK-SEARCH-OTHER", Title: "Other equipment check",
		ProductID: otherProduct.ID, DeviceID: otherDevice.ID,
	}
	if err := db.Create(&endedTicket).Error; err != nil {
		t.Fatalf("create ended ticket: %v", err)
	}
	if err := db.Create(&waitingTicket).Error; err != nil {
		t.Fatalf("create waiting ticket: %v", err)
	}
	if err := db.Create(&otherTicket).Error; err != nil {
		t.Fatalf("create other ticket: %v", err)
	}
	now := time.Now()
	endedAt := now.Add(time.Minute)
	meetings := []models.MeetingRoomJitsi{
		{
			ID: "meeting-search-ended", TenantID: 1, TicketID: strconv.FormatInt(endedTicket.ID, 10),
			RoomName: "meeting-search-ended-room", Status: "ended", CreatedBy: strconv.FormatInt(owner.ID, 10),
			StartedAt: &now, EndedAt: &endedAt, BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: endedAt},
		},
		{
			ID: "meeting-search-waiting", TenantID: 1, TicketID: strconv.FormatInt(waitingTicket.ID, 10),
			RoomName: "meeting-search-waiting-room", Status: "waiting", CreatedBy: strconv.FormatInt(owner.ID, 10),
			BaseModel: models.BaseModel{CreatedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-time.Minute)},
		},
		{
			ID: "meeting-search-other", TenantID: 1, TicketID: strconv.FormatInt(otherTicket.ID, 10),
			RoomName: "meeting-search-other-room", Status: "ended", CreatedBy: strconv.FormatInt(owner.ID, 10),
			StartedAt: &now, EndedAt: &endedAt, BaseModel: models.BaseModel{CreatedAt: now.Add(-2 * time.Minute), UpdatedAt: endedAt},
		},
	}
	if err := db.Create(&meetings).Error; err != nil {
		t.Fatalf("create meetings: %v", err)
	}

	for _, keyword := range []string{"pressure drop", "hydraulic pump", "press-search-001", "hainan test"} {
		result, listErr := MeetingService.listPersonalMeetings(nil, 1, owner.ID, "ended", keyword, 1, 50, nil)
		if listErr != nil {
			t.Fatalf("search %q: %v", keyword, listErr)
		}
		if result.Total != 1 || len(result.Items) != 1 || result.Items[0].ID != "meeting-search-ended" {
			t.Fatalf("search %q returned total=%d items=%+v", keyword, result.Total, result.Items)
		}
	}
	waitingResult, err := MeetingService.listPersonalMeetings(nil, 1, owner.ID, "waiting", "backup", 1, 50, nil)
	if err != nil {
		t.Fatalf("search waiting meeting: %v", err)
	}
	if waitingResult.Total != 1 || len(waitingResult.Items) != 1 || waitingResult.Items[0].ID != "meeting-search-waiting" {
		t.Fatalf("waiting search returned total=%d items=%+v", waitingResult.Total, waitingResult.Items)
	}
	deviceResult, err := MeetingService.listPersonalMeetings(nil, 1, owner.ID, "all", "", 1, 50, nil, device.ID)
	if err != nil {
		t.Fatalf("list device meetings: %v", err)
	}
	if deviceResult.Total != 1 || len(deviceResult.Items) != 1 || deviceResult.Items[0].ID != "meeting-search-ended" {
		t.Fatalf("device meeting filter returned total=%d items=%+v", deviceResult.Total, deviceResult.Items)
	}
	if deviceResult.Summary.Mine != 1 || deviceResult.Summary.Ended != 1 || deviceResult.Summary.Waiting != 0 {
		t.Fatalf("device meeting summary = %+v, want only the filtered device meeting", deviceResult.Summary)
	}
}

func TestFormatMeetingDurationTextTreatsSubsecondMeetingAsLessThanOneSecond(t *testing.T) {
	if got := formatMeetingDurationText(500); got != "不足 1 秒" {
		t.Fatalf("duration text = %q", got)
	}
}
