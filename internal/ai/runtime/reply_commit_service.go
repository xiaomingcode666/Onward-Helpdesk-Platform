package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"
	svc "remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

type replyCommitService struct{}

type replyCommitInput struct {
	Conversation   models.Conversation
	Message        models.Message
	AIAgent        models.AIAgent
	ReplyText      string
	Citations      []dto.KnowledgeCitation
	ClientPrefix   string
	WorkflowRunID  int64
	IncrementRound bool
}

func newReplyCommitService() *replyCommitService {
	return &replyCommitService{}
}

func (s *replyCommitService) SendAIReply(input replyCommitInput) (*models.Message, error) {
	replyText := strings.TrimSpace(input.ReplyText)
	if replyText == "" {
		return nil, nil
	}
	replyMessage, err := svc.MessageService.SendAIMessageWithRequestIDAndWorkflowRunID(
		input.Conversation.ID,
		input.AIAgent.ID,
		fmt.Sprintf("%s_%d", strings.TrimSpace(input.ClientPrefix), input.Message.ID),
		enums.IMMessageTypeText,
		replyText,
		buildKnowledgeAnswerPayload(input.Citations),
		s.buildAIPrincipal(input.AIAgent),
		input.Message.RequestID,
		input.WorkflowRunID,
	)
	if err != nil || !input.IncrementRound {
		return replyMessage, err
	}
	if err := s.IncrementAIReplyRounds(input.Conversation.ID, input.Conversation.AIReplyRounds+1, input.AIAgent.Name); err != nil {
		return nil, err
	}
	return replyMessage, err
}

func buildKnowledgeAnswerPayload(citations []dto.KnowledgeCitation) string {
	if len(citations) == 0 {
		return ""
	}
	if len(citations) > 3 {
		citations = citations[:3]
	}
	payload, err := json.Marshal(map[string]any{
		"kind":               "knowledge_answer",
		"knowledgeCitations": citations,
	})
	if err != nil {
		return ""
	}
	return string(payload)
}

func (s *replyCommitService) CommitAIReply(input replyCommitInput) (*models.Message, error) {
	input.IncrementRound = true
	replyMessage, err := s.SendAIReply(input)
	if err == nil {
		return replyMessage, nil
	}
	if !shouldCommitLateAIReplyAsSystem(err) {
		return replyMessage, err
	}
	lateMessage, lateErr := s.CommitLateAIReplySummary(input)
	if lateErr != nil {
		return replyMessage, err
	}
	return lateMessage, nil
}

func (s *replyCommitService) IncrementAIReplyRounds(conversationID int64, nextRounds int, aiAgentName string) error {
	return repositories.ConversationRepository.Updates(sqls.DB(), conversationID, map[string]any{
		"ai_reply_rounds":  nextRounds,
		"update_user_id":   0,
		"update_user_name": strings.TrimSpace(aiAgentName),
		"updated_at":       time.Now(),
	})
}

func (s *replyCommitService) buildAIPrincipal(aiAgent models.AIAgent) *dto.AuthPrincipal {
	username := "AI"
	if strings.TrimSpace(aiAgent.Name) != "" {
		username = aiAgent.Name
	}
	return &dto.AuthPrincipal{
		UserID:   0,
		Username: username,
		Nickname: username,
		TenantID: aiAgent.TenantID,
	}
}

func (s *replyCommitService) CommitLateAIReplySummary(input replyCommitInput) (*models.Message, error) {
	conversation := svc.ConversationService.Get(input.Conversation.ID)
	if conversation == nil {
		return nil, nil
	}
	if conversation.Status == enums.IMConversationStatusClosed {
		return nil, nil
	}
	content := "以下是转人工前生成的 AI 诊断摘要，供你和工程师参考：\n\n" + strings.TrimSpace(input.ReplyText)
	message, err := svc.MessageService.SendSystemMessageWithRequestIDAndWorkflowRunID(
		input.Conversation.ID,
		fmt.Sprintf("%s_late_%d", strings.TrimSpace(input.ClientPrefix), input.Message.ID),
		content,
		"",
		input.Message.RequestID,
		input.WorkflowRunID,
	)
	if err != nil {
		return nil, err
	}
	nextRounds := conversation.AIReplyRounds + 1
	if input.Conversation.AIReplyRounds+1 > nextRounds {
		nextRounds = input.Conversation.AIReplyRounds + 1
	}
	if err := s.IncrementAIReplyRounds(input.Conversation.ID, nextRounds, input.AIAgent.Name); err != nil {
		return nil, err
	}
	return message, nil
}

func shouldCommitLateAIReplyAsSystem(err error) bool {
	var i18nErr *errorsx.I18nError
	if !errors.As(err, &i18nErr) {
		return false
	}
	return i18nErr.Key == "error.e0119" || i18nErr.Key == "error.e0189" || i18nErr.Key == "error.e0192"
}
