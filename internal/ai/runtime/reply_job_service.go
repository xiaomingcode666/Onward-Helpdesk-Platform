package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
	svc "remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

const (
	aiReplyJobBatchSize         = 20
	aiReplyJobMaxRetries        = 5
	aiReplyJobLeaseTTL          = 45 * time.Second
	aiReplyJobHeartbeatInterval = 10 * time.Second
)

func (s *aiReplyService) EnqueueReplyJob(db *gorm.DB, conversation models.Conversation, message models.Message) error {
	if db == nil || message.ID <= 0 || conversation.ID <= 0 || message.SenderType != enums.IMSenderTypeCustomer {
		return nil
	}
	now := time.Now()
	return repositories.AIReplyJobRepository.Create(db, &models.AIReplyJob{
		TenantID:       conversation.TenantID,
		ProductID:      conversation.ProductID,
		ConversationID: conversation.ID,
		MessageID:      message.ID,
		AIAgentID:      conversation.AIAgentID,
		RequestID:      message.RequestID,
		Status:         models.AIReplyJobStatusPending,
		MaxRetries:     aiReplyJobMaxRetries,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
}

func (s *aiReplyService) TriggerReplyAsync(_ models.Conversation, message models.Message) {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *aiReplyService) ProcessReplyJobByMessageID(ctx context.Context, messageID int64) error {
	now := time.Now()
	lockOwner := fmt.Sprintf("ai-reply-immediate-%d-%d", messageID, now.UnixNano())
	job, err := repositories.AIReplyJobRepository.ClaimByMessageID(sqls.DB(), messageID, now, lockOwner)
	if err != nil || job == nil {
		return err
	}
	return s.processClaimedReplyJob(ctx, job)
}

func (s *aiReplyService) ProcessDueReplyJobs(ctx context.Context, limit int) int {
	if limit <= 0 {
		limit = aiReplyJobBatchSize
	}
	now := time.Now()
	lockOwner := fmt.Sprintf("ai-reply-worker-%d", now.UnixNano())
	jobs, err := repositories.AIReplyJobRepository.ClaimDueJobs(sqls.DB(), now, limit, lockOwner)
	if err != nil {
		slog.Warn("claim ai reply jobs failed", "error", err)
		return 0
	}
	for i := range jobs {
		if err := s.processClaimedReplyJob(ctx, &jobs[i]); err != nil {
			slog.Warn("process ai reply job failed", "job_id", jobs[i].ID, "message_id", jobs[i].MessageID, "error", err)
		}
	}
	return len(jobs)
}

func (s *aiReplyService) RecoverTimedOutReplyJobs() int64 {
	now := time.Now()
	recovered, err := repositories.AIReplyJobRepository.RecoverExpiredRunningJobs(sqls.DB(), now.Add(-aiReplyJobLeaseTTL), now)
	if err != nil {
		slog.Warn("recover timed out ai reply jobs failed", "error", err)
		return 0
	}
	return recovered
}

func (s *aiReplyService) processClaimedReplyJob(parent context.Context, job *models.AIReplyJob) (retErr error) {
	if job == nil {
		return nil
	}
	stopHeartbeat := s.startReplyJobLeaseHeartbeat(parent, job)
	defer stopHeartbeat()
	defer func() {
		if recovered := recover(); recovered != nil {
			retErr = fmt.Errorf("panic while processing ai reply job: %v", recovered)
			slog.Error("panic while processing ai reply job", "job_id", job.ID, "message_id", job.MessageID, "panic", recovered, "stack", string(debug.Stack()))
			_ = s.retryOrFailReplyJob(job, retErr)
		}
	}()

	conversation := svc.ConversationService.Get(job.ConversationID)
	message := svc.MessageService.Get(job.MessageID)
	if conversation == nil || message == nil || message.ConversationID != job.ConversationID || message.SenderType != enums.IMSenderTypeCustomer {
		return s.finishReplyJob(job, models.AIReplyJobStatusCancelled, "conversation or customer message no longer exists", job.RetryCount, nil)
	}
	if conversation.TenantID != job.TenantID || conversation.ProductID != job.ProductID {
		return s.finishReplyJob(job, models.AIReplyJobStatusFailed, "job scope does not match conversation scope", job.RetryCount, nil)
	}
	waitingForEarlierTurn, err := repositories.AIReplyJobRepository.HasEarlierUnfinishedJob(
		sqls.DB(),
		job.ConversationID,
		job.ID,
	)
	if err != nil {
		if retryErr := s.retryOrFailReplyJob(job, err); retryErr != nil {
			return retryErr
		}
		return err
	}
	if waitingForEarlierTurn {
		nextAttemptAt := time.Now().Add(time.Second)
		return s.finishReplyJob(
			job,
			models.AIReplyJobStatusWaitingRetry,
			"waiting for an earlier customer turn",
			job.RetryCount,
			&nextAttemptAt,
		)
	}
	aiAgent := svc.AIAgentService.Get(conversation.AIAgentID)
	if aiAgent == nil || aiAgent.Status != enums.StatusOk {
		return s.finishReplyJob(job, models.AIReplyJobStatusCancelled, "ai agent is not enabled", job.RetryCount, nil)
	}
	if s.eligibility != nil && !s.eligibility.CanReply(*conversation, *message, *aiAgent) {
		return s.finishReplyJob(job, models.AIReplyJobStatusSucceeded, "", job.RetryCount, nil)
	}
	quotaExceeded, err := s.isAIReplyQuotaExceeded(conversation.TenantID, conversation.ProductID)
	if err != nil {
		if retryErr := s.retryOrFailReplyJob(job, err); retryErr != nil {
			return retryErr
		}
		return err
	}
	if quotaExceeded {
		_, commitErr := s.commit.SendAIReply(replyCommitInput{
			Conversation: *conversation,
			Message:      *message,
			AIAgent:      *aiAgent,
			ReplyText:    buildAIReplyFailureText(*conversation, *message, *aiAgent, aiReplyFailureKindQuota),
			ClientPrefix: "ai_reply_quota",
		})
		if commitErr != nil {
			if retryErr := s.retryOrFailReplyJob(job, commitErr); retryErr != nil {
				return retryErr
			}
			return commitErr
		}
		return s.finishReplyJob(job, models.AIReplyJobStatusSucceeded, "quota exceeded", job.RetryCount, nil)
	}

	timeout := s.resolveReplyTimeout(*aiAgent)
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	if err := s.TriggerReply(ctx, *conversation, *message, *aiAgent); err != nil {
		if retryErr := s.retryOrFailReplyJob(job, err); retryErr != nil {
			return retryErr
		}
		return err
	}
	return s.finishReplyJob(job, models.AIReplyJobStatusSucceeded, "", job.RetryCount, nil)
}

func (s *aiReplyService) startReplyJobLeaseHeartbeat(parent context.Context, job *models.AIReplyJob) context.CancelFunc {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	if job == nil || job.ID <= 0 || strings.TrimSpace(job.LockOwner) == "" {
		return cancel
	}
	go func() {
		ticker := time.NewTicker(aiReplyJobHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				updated, err := repositories.AIReplyJobRepository.Heartbeat(sqls.DB(), job.ID, job.LockOwner, now)
				if err != nil {
					slog.Warn("heartbeat ai reply job lease failed", "job_id", job.ID, "message_id", job.MessageID, "error", err)
					continue
				}
				if !updated {
					return
				}
			}
		}
	}()
	return cancel
}

func (s *aiReplyService) isAIReplyQuotaExceeded(tenantID, productID int64) (bool, error) {
	for _, resourceType := range []string{models.QuotaResourceRequests, models.QuotaResourceTokens, models.QuotaResourceCost} {
		_, exceeded, _, err := svc.MeteringService.CheckQuotaWithLimit(tenantID, productID, resourceType)
		if err != nil {
			return false, err
		}
		if exceeded {
			return true, nil
		}
	}
	return false, nil
}

func (s *aiReplyService) retryOrFailReplyJob(job *models.AIReplyJob, cause error) error {
	nextRetry := job.RetryCount + 1
	message := "ai reply failed"
	if cause != nil {
		message = strings.TrimSpace(cause.Error())
	}
	if len(message) > 2000 {
		message = message[:2000]
	}
	maxRetries := job.MaxRetries
	if maxRetries <= 0 {
		maxRetries = aiReplyJobMaxRetries
	}
	if nextRetry >= maxRetries {
		conversation := svc.ConversationService.Get(job.ConversationID)
		customerMessage := svc.MessageService.Get(job.MessageID)
		aiAgent := svc.AIAgentService.Get(job.AIAgentID)
		if conversation != nil && customerMessage != nil && aiAgent != nil {
			s.commitAsyncFailure(*conversation, *customerMessage, *aiAgent)
		}
		return s.finishReplyJob(job, models.AIReplyJobStatusFailed, message, nextRetry, nil)
	}
	delay := time.Duration(1<<min(nextRetry, 6)) * time.Second
	nextAttemptAt := time.Now().Add(delay)
	return s.finishReplyJob(job, models.AIReplyJobStatusWaitingRetry, message, nextRetry, &nextAttemptAt)
}

func (s *aiReplyService) finishReplyJob(job *models.AIReplyJob, status, lastError string, retryCount int, nextAttemptAt *time.Time) error {
	updated, err := repositories.AIReplyJobRepository.Finish(sqls.DB(), job.ID, job.LockOwner, status, lastError, retryCount, nextAttemptAt)
	if err != nil {
		return err
	}
	if !updated {
		return fmt.Errorf("ai reply job %d lease is stale", job.ID)
	}
	return nil
}
