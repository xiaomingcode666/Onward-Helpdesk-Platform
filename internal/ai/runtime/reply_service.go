package runtime

import (
	"context"
	"strings"
	"sync"

	applicationruntime "remotehelpdesk/internal/ai/application/runtime"
	svc "remotehelpdesk/internal/services"
)

var AIReplyService = newAIReplyService()

func init() {
	svc.EnqueueAIReplyJobHook = AIReplyService.EnqueueReplyJob
	svc.TriggerAIReplyAsyncHook = AIReplyService.TriggerReplyAsync
	svc.ProcessDueAIReplyJobsHook = AIReplyService.ProcessDueReplyJobs
	svc.RecoverTimedOutAIReplyJobsHook = AIReplyService.RecoverTimedOutReplyJobs
	svc.StartAIReplyJobWorkerHook = AIReplyService.StartReplyJobWorker
}

func newAIReplyService() *aiReplyService {
	return &aiReplyService{
		eligibility: newReplyEligibility(),
		executor:    newRuntimeReplyExecutor(),
		interrupts:  newReplyInterruptService(),
		commit:      newReplyCommitService(),
		wake:        make(chan struct{}, 1),
	}
}

type aiReplyService struct {
	eligibility *replyEligibility
	executor    *runtimeReplyExecutor
	interrupts  *replyInterruptService
	commit      *replyCommitService
	wake        chan struct{}
	startWorker sync.Once
}

func (s *aiReplyService) StartReplyJobWorker(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	s.startWorker.Do(func() {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-s.wake:
					s.ProcessDueReplyJobs(ctx, aiReplyJobBatchSize)
				}
			}
		}()
	})
}

func firstInvokedToolCode(summary *applicationruntime.Summary) string {
	if summary == nil {
		return ""
	}
	if len(summary.InvokedToolCodes) > 0 {
		return strings.TrimSpace(summary.InvokedToolCodes[0])
	}
	return ""
}
