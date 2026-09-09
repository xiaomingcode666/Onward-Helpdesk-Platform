package services

import (
	"errors"
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

func TestKnowledgeIndexTaskRetriesEmbeddingFailureBeforeTerminalState(t *testing.T) {
	db := setupKnowledgeIndexSyncTestDB(t)
	baseTime := time.Date(2026, 7, 29, 4, 0, 0, 0, time.UTC)
	service := newKnowledgeIndexSyncService()
	service.now = func() time.Time { return baseTime }
	document := models.KnowledgeDocument{
		ID:              11,
		TenantID:        1,
		KnowledgeBaseID: 7,
		Title:           "维修手册",
		ReviewStatus:    "published",
		IndexStatus:     enums.KnowledgeDocumentIndexStatusPending,
		Status:          enums.StatusOk,
	}
	if err := db.Create(&document).Error; err != nil {
		t.Fatalf("create document: %v", err)
	}
	task := models.KnowledgeIndexSyncTask{
		TenantID:        1,
		SubjectType:     "knowledge_document",
		SubjectID:       document.ID,
		KnowledgeBaseID: document.KnowledgeBaseID,
		Status:          "running",
		MaxRetries:      3,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	cause := errors.New("failed to call embedding api: provider unavailable")

	if err := service.failTask(&task, cause); !errors.Is(err, cause) {
		t.Fatalf("first failTask() error = %v", err)
	}
	first := loadKnowledgeIndexSyncTask(t, db, task.ID)
	if first.Status != "waiting_retry" || first.RetryCount != 1 || first.ErrorCode != "embedding_error" {
		t.Fatalf("first retry state = %#v", first)
	}
	if first.NextAttemptAt == nil || !first.NextAttemptAt.Equal(baseTime.Add(time.Minute)) || first.FinishedAt != nil {
		t.Fatalf("first retry timing = next:%v finished:%v", first.NextAttemptAt, first.FinishedAt)
	}
	assertKnowledgeDocumentIndexState(t, db, document.ID, enums.KnowledgeDocumentIndexStatusPending, "embedding_error")

	service.now = func() time.Time { return baseTime.Add(time.Minute) }
	if err := service.failTask(&first, cause); !errors.Is(err, cause) {
		t.Fatalf("second failTask() error = %v", err)
	}
	second := loadKnowledgeIndexSyncTask(t, db, task.ID)
	if second.Status != "waiting_retry" || second.RetryCount != 2 || second.NextAttemptAt == nil || !second.NextAttemptAt.Equal(baseTime.Add(3*time.Minute)) {
		t.Fatalf("second retry state = %#v", second)
	}

	service.now = func() time.Time { return baseTime.Add(3 * time.Minute) }
	if err := service.failTask(&second, cause); !errors.Is(err, cause) {
		t.Fatalf("terminal failTask() error = %v", err)
	}
	terminal := loadKnowledgeIndexSyncTask(t, db, task.ID)
	if terminal.Status != "failed" || terminal.RetryCount != 3 || terminal.NextAttemptAt != nil || terminal.FinishedAt == nil {
		t.Fatalf("terminal retry state = %#v", terminal)
	}
	assertKnowledgeDocumentIndexState(t, db, document.ID, enums.KnowledgeDocumentIndexStatusFailed, "embedding_error")
}

func TestKnowledgeIndexTerminalTaskRequeueMarksEntryPending(t *testing.T) {
	db := setupKnowledgeIndexSyncTestDB(t)
	now := time.Now().Add(-2 * time.Hour)
	document := models.KnowledgeDocument{
		TenantID: 1, KnowledgeBaseID: 7, Title: "维修手册", ReviewStatus: "published",
		IndexStatus: enums.KnowledgeDocumentIndexStatusFailed, IndexError: "embedding_error: unavailable", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&document).Error; err != nil {
		t.Fatalf("create failed document: %v", err)
	}
	task := models.KnowledgeIndexSyncTask{
		TenantID: 1, IdempotencyKey: "terminal-requeue", SubjectType: "knowledge_document", SubjectID: document.ID,
		KnowledgeBaseID: 7, Status: "failed", RetryCount: 3, MaxRetries: 3, ErrorCode: "embedding_error",
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create terminal task: %v", err)
	}
	service := newKnowledgeIndexSyncService()
	if !service.requeueTerminalTask(&task) {
		t.Fatal("terminal task was not requeued")
	}
	requeued := loadKnowledgeIndexSyncTask(t, db, task.ID)
	if requeued.Status != "pending" || requeued.RetryCount != 0 || requeued.NextAttemptAt == nil || requeued.FinishedAt != nil {
		t.Fatalf("requeued task = %+v", requeued)
	}
	var reloaded models.KnowledgeDocument
	if err := db.First(&reloaded, document.ID).Error; err != nil {
		t.Fatalf("reload document: %v", err)
	}
	if reloaded.IndexStatus != enums.KnowledgeDocumentIndexStatusPending || reloaded.IndexError != "" || reloaded.IndexedAt != nil {
		t.Fatalf("requeued document index state = %+v", reloaded)
	}
	recentFailure := requeued
	recentFailure.Status = "failed"
	recentFailure.UpdatedAt = service.now()
	if service.requeueTerminalTask(&recentFailure) {
		t.Fatal("recent terminal failure bypassed the automatic recovery cooldown")
	}
	recentLocalEmbeddingFailure := recentFailure
	recentLocalEmbeddingFailure.ErrorCode = "embedding_error"
	recentLocalEmbeddingFailure.ErrorSummary = `Post "http://127.0.0.1:18099/v1/embeddings": connect: connection refused`
	if !service.requeueTerminalTask(&recentLocalEmbeddingFailure) {
		t.Fatal("recent local embedding failure was not requeued")
	}
	requeuedLocalFailure := loadKnowledgeIndexSyncTask(t, db, task.ID)
	if requeuedLocalFailure.Status != "pending" || requeuedLocalFailure.RetryCount != 0 || requeuedLocalFailure.ErrorSummary != "" {
		t.Fatalf("requeued local embedding failure = %+v", requeuedLocalFailure)
	}
}

func TestKnowledgeIndexSucceededTaskIsRequeuedForNewerEntryState(t *testing.T) {
	db := setupKnowledgeIndexSyncTestDB(t)
	previousSyncAt := time.Date(2026, 7, 29, 4, 0, 0, 0, time.UTC)
	republishedAt := previousSyncAt.Add(time.Hour)
	document := models.KnowledgeDocument{
		TenantID: 1, KnowledgeBaseID: 7, Title: "维修手册", ReviewStatus: "published",
		IndexStatus: enums.KnowledgeDocumentIndexStatusPending, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: previousSyncAt, UpdatedAt: republishedAt},
	}
	if err := db.Create(&document).Error; err != nil {
		t.Fatalf("create republished document: %v", err)
	}
	existing := models.KnowledgeIndexSyncTask{
		TenantID: 1, IdempotencyKey: "republished-same-revision", SubjectType: "knowledge_document", SubjectID: document.ID,
		KnowledgeBaseID: 7, RevisionID: 3, Action: "upsert", Status: "succeeded", MaxRetries: 3,
		AuditFields: models.AuditFields{CreatedAt: previousSyncAt, UpdatedAt: previousSyncAt},
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create prior succeeded task: %v", err)
	}
	service := newKnowledgeIndexSyncService()
	service.now = func() time.Time { return republishedAt.Add(time.Minute) }
	task, err := service.enqueue(db, &models.KnowledgeIndexSyncTask{
		TenantID: 1, IdempotencyKey: existing.IdempotencyKey, SubjectType: existing.SubjectType, SubjectID: document.ID,
		KnowledgeBaseID: 7, RevisionID: 3, Action: "upsert",
	}, nil)
	if err != nil {
		t.Fatalf("enqueue newer entry state: %v", err)
	}
	if task == nil || task.ID != existing.ID || task.Status != "pending" || task.NextAttemptAt == nil || task.FinishedAt != nil {
		t.Fatalf("requeued succeeded task = %+v", task)
	}
}

func TestKnowledgeIndexSucceededDeleteIsRequeuedForNewerDraftState(t *testing.T) {
	db := setupKnowledgeIndexSyncTestDB(t)
	previousSyncAt := time.Date(2026, 7, 29, 4, 0, 0, 0, time.UTC)
	editedAt := previousSyncAt.Add(time.Hour)
	document := models.KnowledgeDocument{
		TenantID: 1, KnowledgeBaseID: 7, Title: "Edited repair draft", ReviewStatus: "draft",
		PublishedRevisionID: 3, IndexStatus: enums.KnowledgeDocumentIndexStatusPending, Status: enums.StatusDisabled,
		AuditFields: models.AuditFields{CreatedAt: previousSyncAt, UpdatedAt: editedAt},
	}
	if err := db.Create(&document).Error; err != nil {
		t.Fatalf("create edited draft: %v", err)
	}
	existing := models.KnowledgeIndexSyncTask{
		TenantID: 1, IdempotencyKey: "draft-delete-same-revision", SubjectType: "knowledge_document", SubjectID: document.ID,
		KnowledgeBaseID: 7, RevisionID: 3, Action: "delete", Status: "succeeded", MaxRetries: 3,
		AuditFields: models.AuditFields{CreatedAt: previousSyncAt, UpdatedAt: previousSyncAt},
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create prior delete task: %v", err)
	}
	service := newKnowledgeIndexSyncService()
	service.now = func() time.Time { return editedAt.Add(time.Minute) }
	task, err := service.enqueue(db, &models.KnowledgeIndexSyncTask{IdempotencyKey: existing.IdempotencyKey}, nil)
	if err != nil {
		t.Fatalf("enqueue newer draft cleanup: %v", err)
	}
	if task == nil || task.ID != existing.ID || task.Status != "pending" {
		t.Fatalf("requeued draft delete task = %+v", task)
	}
}

func TestKnowledgeIndexTaskConvergesOnlyItsCapturedRevision(t *testing.T) {
	db := setupKnowledgeIndexSyncTestDB(t)
	now := time.Now()
	document := models.KnowledgeDocument{
		TenantID: 1, KnowledgeBaseID: 7, Title: "维修手册", ReviewStatus: "published",
		PublishedRevisionID: 3, IndexStatus: enums.KnowledgeDocumentIndexStatusPending, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&document).Error; err != nil {
		t.Fatalf("create document: %v", err)
	}
	task := &models.KnowledgeIndexSyncTask{TenantID: 1, SubjectType: "knowledge_document", SubjectID: document.ID, RevisionID: 3}
	service := newKnowledgeIndexSyncService()
	if desiredUpsert, markEntry := service.taskRevisionDesiredState(task); !desiredUpsert || !markEntry {
		t.Fatalf("published current revision state = upsert:%v mark:%v", desiredUpsert, markEntry)
	}
	if err := db.Model(&document).Updates(map[string]any{"review_status": "deprecated", "status": enums.StatusDisabled}).Error; err != nil {
		t.Fatalf("deprecate document: %v", err)
	}
	if desiredUpsert, markEntry := service.taskRevisionDesiredState(task); desiredUpsert || !markEntry {
		t.Fatalf("deprecated current revision state = upsert:%v mark:%v", desiredUpsert, markEntry)
	}
	if err := db.Model(&document).Updates(map[string]any{"review_status": "published", "published_revision_id": 4, "status": enums.StatusOk}).Error; err != nil {
		t.Fatalf("publish newer revision: %v", err)
	}
	if desiredUpsert, markEntry := service.taskRevisionDesiredState(task); desiredUpsert || markEntry {
		t.Fatalf("superseded revision state = upsert:%v mark:%v", desiredUpsert, markEntry)
	}
}

func TestKnowledgeIndexRevisionBaseUsesCapturedRevision(t *testing.T) {
	db := setupKnowledgeIndexSyncTestDB(t)
	now := time.Now()
	revision := models.KnowledgeRevision{
		TenantID: 1, KnowledgeBaseID: 9, EntryType: "document", EntryID: 11,
		VersionNo: 1, ReviewStatus: "published", AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&revision).Error; err != nil {
		t.Fatalf("create revision: %v", err)
	}
	if got := resolveKnowledgeIndexRevisionBaseID(revision.ID, 7); got != 9 {
		t.Fatalf("revision knowledge base = %d, want 9", got)
	}
}

func setupKnowledgeIndexSyncTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.KnowledgeIndexSyncTask{}, &models.KnowledgeDocument{}, &models.KnowledgeRevision{}, &models.ProductManualFile{}); err != nil {
		t.Fatal(err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if raw, closeErr := db.DB(); closeErr == nil {
			_ = raw.Close()
		}
	})
	return db
}

func loadKnowledgeIndexSyncTask(t *testing.T, db *gorm.DB, taskID int64) models.KnowledgeIndexSyncTask {
	t.Helper()
	var task models.KnowledgeIndexSyncTask
	if err := db.First(&task, taskID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	return task
}

func assertKnowledgeDocumentIndexState(t *testing.T, db *gorm.DB, documentID int64, status enums.KnowledgeDocumentIndexStatus, errorCode string) {
	t.Helper()
	var document models.KnowledgeDocument
	if err := db.First(&document, documentID).Error; err != nil {
		t.Fatalf("load document: %v", err)
	}
	if document.IndexStatus != status || !strings.Contains(document.IndexError, errorCode) {
		t.Fatalf("document index state = status:%q error:%q", document.IndexStatus, document.IndexError)
	}
}
