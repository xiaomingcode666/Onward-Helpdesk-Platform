package services

import (
	"context"

	"remotehelpdesk/internal/models"

	"gorm.io/gorm"
)

var TriggerAIReplyAsyncHook func(conversation models.Conversation, message models.Message)
var EnqueueAIReplyJobHook func(db *gorm.DB, conversation models.Conversation, message models.Message) error
var ProcessDueAIReplyJobsHook func(ctx context.Context, limit int) int
var RecoverTimedOutAIReplyJobsHook func() int64
var StartAIReplyJobWorkerHook func(ctx context.Context)

func StartAIReplyJobWorker(ctx context.Context) {
	if StartAIReplyJobWorkerHook != nil {
		StartAIReplyJobWorkerHook(ctx)
	}
}

func ProcessDueAIReplyJobs(ctx context.Context, limit int) int {
	if ProcessDueAIReplyJobsHook == nil {
		return 0
	}
	return ProcessDueAIReplyJobsHook(ctx, limit)
}

func RecoverTimedOutAIReplyJobs() int64 {
	if RecoverTimedOutAIReplyJobsHook == nil {
		return 0
	}
	return RecoverTimedOutAIReplyJobsHook()
}
