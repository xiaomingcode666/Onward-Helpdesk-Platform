package repositories

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupAIReplyJobRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := "ai_reply_job_" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.AIReplyJob{}); err != nil {
		t.Fatalf("migrate ai reply job: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestAIReplyJobRepositoryEnqueueAndClaimAreIdempotent(t *testing.T) {
	db := setupAIReplyJobRepositoryTestDB(t)
	now := time.Now()
	job := &models.AIReplyJob{
		TenantID: 1, ProductID: 2, ConversationID: 3, MessageID: 4,
		Status: models.AIReplyJobStatusPending, MaxRetries: 5, CreatedAt: now, UpdatedAt: now,
	}
	if err := AIReplyJobRepository.Create(db, job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	if err := AIReplyJobRepository.Create(db, &models.AIReplyJob{
		TenantID: 1, ProductID: 2, ConversationID: 3, MessageID: 4,
		Status: models.AIReplyJobStatusPending, MaxRetries: 5, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("idempotent create: %v", err)
	}
	var count int64
	if err := db.Model(&models.AIReplyJob{}).Count(&count).Error; err != nil {
		t.Fatalf("count jobs: %v", err)
	}
	if count != 1 {
		t.Fatalf("job count = %d, want 1", count)
	}

	claimed, err := AIReplyJobRepository.ClaimByMessageID(db, 4, now, "worker-a")
	if err != nil || claimed == nil {
		t.Fatalf("claim job: job=%v err=%v", claimed, err)
	}
	claimedAgain, err := AIReplyJobRepository.ClaimByMessageID(db, 4, now, "worker-b")
	if err != nil {
		t.Fatalf("claim job again: %v", err)
	}
	if claimedAgain != nil {
		t.Fatalf("second worker claimed running job: %+v", claimedAgain)
	}
}

func TestAIReplyJobRepositoryRecoversExpiredLease(t *testing.T) {
	db := setupAIReplyJobRepositoryTestDB(t)
	lockedAt := time.Now().Add(-10 * time.Minute)
	job := models.AIReplyJob{
		TenantID: 1, ProductID: 2, ConversationID: 3, MessageID: 5,
		Status: models.AIReplyJobStatusRunning, MaxRetries: 5, LockedAt: &lockedAt, LockOwner: "dead-worker",
		CreatedAt: lockedAt, UpdatedAt: lockedAt,
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatalf("create running job: %v", err)
	}
	now := time.Now()
	recovered, err := AIReplyJobRepository.RecoverExpiredRunningJobs(db, now.Add(-5*time.Minute), now)
	if err != nil {
		t.Fatalf("recover jobs: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("recovered = %d, want 1", recovered)
	}
	got := AIReplyJobRepository.GetByMessageID(db, 5)
	if got == nil || got.Status != models.AIReplyJobStatusWaitingRetry || got.LockOwner != "" {
		t.Fatalf("unexpected recovered job: %+v", got)
	}
}

func TestAIReplyJobRepositorySerializesTurnsWithinConversation(t *testing.T) {
	db := setupAIReplyJobRepositoryTestDB(t)
	now := time.Now()
	first := models.AIReplyJob{
		TenantID: 1, ProductID: 2, ConversationID: 3, MessageID: 10,
		Status: models.AIReplyJobStatusRunning, MaxRetries: 5, LockOwner: "worker-a",
		CreatedAt: now, UpdatedAt: now,
	}
	second := models.AIReplyJob{
		TenantID: 1, ProductID: 2, ConversationID: 3, MessageID: 11,
		Status: models.AIReplyJobStatusRunning, MaxRetries: 5, LockOwner: "worker-b",
		CreatedAt: now.Add(time.Millisecond), UpdatedAt: now.Add(time.Millisecond),
	}
	otherConversation := models.AIReplyJob{
		TenantID: 1, ProductID: 2, ConversationID: 4, MessageID: 12,
		Status: models.AIReplyJobStatusRunning, MaxRetries: 5, LockOwner: "worker-c",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first job: %v", err)
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create second job: %v", err)
	}
	if err := db.Create(&otherConversation).Error; err != nil {
		t.Fatalf("create other conversation job: %v", err)
	}

	waiting, err := AIReplyJobRepository.HasEarlierUnfinishedJob(db, second.ConversationID, second.ID)
	if err != nil || !waiting {
		t.Fatalf("second turn should wait for first: waiting=%v err=%v", waiting, err)
	}
	waiting, err = AIReplyJobRepository.HasEarlierUnfinishedJob(db, otherConversation.ConversationID, otherConversation.ID)
	if err != nil || waiting {
		t.Fatalf("different conversation must not be blocked: waiting=%v err=%v", waiting, err)
	}
	if err := db.Model(&models.AIReplyJob{}).Where("id = ?", first.ID).Update("status", models.AIReplyJobStatusSucceeded).Error; err != nil {
		t.Fatalf("finish first job: %v", err)
	}
	waiting, err = AIReplyJobRepository.HasEarlierUnfinishedJob(db, second.ConversationID, second.ID)
	if err != nil || waiting {
		t.Fatalf("second turn should proceed after first completes: waiting=%v err=%v", waiting, err)
	}
}
