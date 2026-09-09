package services

import (
	"log/slog"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/mlogclub/simple/sqls"
)

var AgentTeamService = newAgentTeamService()

func newAgentTeamService() *agentTeamService {
	return &agentTeamService{}
}

type agentTeamService struct {
}

func (s *agentTeamService) Get(id int64) *models.AgentTeam {
	return repositories.AgentTeamRepository.Get(sqls.DB(), id)
}

func (s *agentTeamService) GetForTenant(id, tenantID int64) *models.AgentTeam {
	if id <= 0 {
		return nil
	}
	item := repositories.AgentTeamRepository.Get(sqls.DB(), id)
	if item == nil || (tenantID > 0 && item.TenantID != tenantID) {
		return nil
	}
	return item
}

func (s *agentTeamService) Take(where ...interface{}) *models.AgentTeam {
	return repositories.AgentTeamRepository.Take(sqls.DB(), where...)
}

func (s *agentTeamService) Find(cnd *sqls.Cnd) []models.AgentTeam {
	return repositories.AgentTeamRepository.Find(sqls.DB(), cnd)
}

func (s *agentTeamService) FindOne(cnd *sqls.Cnd) *models.AgentTeam {
	return repositories.AgentTeamRepository.FindOne(sqls.DB(), cnd)
}

func (s *agentTeamService) FindPageByParams(params *params.QueryParams) (list []models.AgentTeam, paging *sqls.Paging) {
	return repositories.AgentTeamRepository.FindPageByParams(sqls.DB(), params)
}

func (s *agentTeamService) FindPageByCnd(cnd *sqls.Cnd) (list []models.AgentTeam, paging *sqls.Paging) {
	return repositories.AgentTeamRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *agentTeamService) Count(cnd *sqls.Cnd) int64 {
	return repositories.AgentTeamRepository.Count(sqls.DB(), cnd)
}

func (s *agentTeamService) FindByIds(ids []int64) []models.AgentTeam {
	return repositories.AgentTeamRepository.FindByIds(sqls.DB(), ids)
}

func (s *agentTeamService) Create(t *models.AgentTeam) error {
	return repositories.AgentTeamRepository.Create(sqls.DB(), t)
}

func (s *agentTeamService) Update(t *models.AgentTeam) error {
	return repositories.AgentTeamRepository.Update(sqls.DB(), t)
}

func (s *agentTeamService) Updates(id int64, columns map[string]interface{}) error {
	return repositories.AgentTeamRepository.Updates(sqls.DB(), id, columns)
}

func (s *agentTeamService) UpdateColumn(id int64, name string, value interface{}) error {
	return repositories.AgentTeamRepository.UpdateColumn(sqls.DB(), id, name, value)
}

func (s *agentTeamService) Delete(id int64) {
	repositories.AgentTeamRepository.Delete(sqls.DB(), id)
}

func (s *agentTeamService) CreateAgentTeam(req request.CreateAgentTeamRequest, operator *dto.AuthPrincipal) (*models.AgentTeam, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item, err := s.buildTeamModel(0, operator.EffectiveTenantID(), req.Name, req.LeaderUserID, req.AssignmentMode, req.Status, req.Description, req.Remark)
	if err != nil {
		return nil, err
	}
	item.AuditFields = utils.BuildAuditFields(operator)
	if err := repositories.AgentTeamRepository.Create(sqls.DB(), item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *agentTeamService) UpdateAgentTeam(req request.UpdateAgentTeamRequest, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	current := s.GetForTenant(req.ID, operator.EffectiveTenantID())
	if current == nil || current.Status == enums.StatusDeleted {
		return errorsx.InvalidParamI18n("error.e0169")
	}
	if current.SystemManaged {
		return s.updateSystemManagedAgentTeam(current, req, operator)
	}
	item, err := s.buildTeamModel(req.ID, current.TenantID, req.Name, req.LeaderUserID, req.AssignmentMode, req.Status, req.Description, req.Remark)
	if err != nil {
		return err
	}
	now := time.Now()
	if err := repositories.AgentTeamRepository.Updates(sqls.DB(), req.ID, map[string]any{
		"name":             item.Name,
		"leader_user_id":   item.LeaderUserID,
		"assignment_mode":  item.AssignmentMode,
		"status":           item.Status,
		"description":      item.Description,
		"remark":           item.Remark,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       now,
	}); err != nil {
		return err
	}
	if item.Status == enums.StatusOk {
		_, _ = ConversationDispatchService.DispatchPendingConversations(0)
		_, _ = TicketDispatchService.DispatchPendingTickets(0)
		return nil
	}
	s.recoverWorkAfterTeamDisabled(current.TenantID, current.ID, now, "disabled")
	return nil
}

func (s *agentTeamService) updateSystemManagedAgentTeam(current *models.AgentTeam, req request.UpdateAgentTeamRequest, operator *dto.AuthPrincipal) error {
	assignmentMode := req.AssignmentMode
	if strings.TrimSpace(assignmentMode) == "" {
		assignmentMode = current.AssignmentMode
	}
	now := time.Now()
	updates := map[string]any{
		"assignment_mode":  NormalizeAgentTeamAssignmentMode(assignmentMode),
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       now,
	}
	if current.TeamType != AgentTeamTypeProductRepair || current.ProductID <= 0 || req.LeaderUserID <= 0 {
		return repositories.AgentTeamRepository.Updates(sqls.DB(), current.ID, updates)
	}
	member := repositories.EnterpriseIAMRepository.FindTenantMemberByUserID(sqls.DB(), current.TenantID, req.LeaderUserID)
	if member == nil || member.Status != enums.StatusOk {
		return errorsx.InvalidParam("product supervisor must be an active enterprise member")
	}
	user := repositories.UserRepository.Get(sqls.DB(), req.LeaderUserID)
	if user == nil || user.Status != enums.StatusOk {
		return errorsx.InvalidParam("product supervisor user is not active")
	}
	updates["leader_user_id"] = req.LeaderUserID
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if current.Status == enums.StatusOk {
			if _, err := AgentTeamMemberService.EnsureMemberPreservingDispatchDB(
				ctx.Tx,
				current.TenantID,
				current.ID,
				req.LeaderUserID,
				member.ID,
				defaultAgentTeamMemberWeight,
				false,
				operator,
			); err != nil {
				return err
			}
		}
		if err := repositories.AgentTeamRepository.Updates(ctx.Tx, current.ID, updates); err != nil {
			return err
		}
		return repositories.ProductRepository.UpdatesByTenant(ctx.Tx, current.ProductID, current.TenantID, map[string]any{
			"owner_member_id":  member.ID,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
			"updated_at":       now,
		})
	})
}

func (s *agentTeamService) DeleteAgentTeam(id int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	current := s.GetForTenant(id, operator.EffectiveTenantID())
	if current == nil || current.Status == enums.StatusDeleted {
		return errorsx.InvalidParamI18n("error.e0169")
	}
	if current.SystemManaged {
		return errorsx.Forbidden("system managed product repair teams cannot be deleted")
	}
	if AgentProfileService.Take("team_id = ?", id) != nil {
		return errorsx.ForbiddenI18n("error.e0167")
	}
	if AgentTeamMemberService.tableReady(sqls.DB()) {
		if member := repositories.AgentTeamMemberRepository.FindOne(sqls.DB(), sqls.NewCnd().
			Eq("team_id", id).
			Eq("status", enums.StatusOk)); member != nil {
			return errorsx.Forbidden("agent team still has product repair members")
		}
	}
	if AIAgentService.Take(
		"(team_ids = ? OR team_ids LIKE ? OR team_ids LIKE ? OR team_ids LIKE ?) AND status <> ?",
		utils.JoinInt64s([]int64{id}),
		utils.JoinInt64s([]int64{id})+",%",
		"%,"+utils.JoinInt64s([]int64{id}),
		"%,"+utils.JoinInt64s([]int64{id})+",%",
		enums.StatusDeleted,
	) != nil {
		return errorsx.ForbiddenI18n("error.e0166")
	}
	now := time.Now()
	if err := repositories.AgentTeamRepository.Updates(sqls.DB(), id, map[string]any{
		"status":           enums.StatusDeleted,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       now,
	}); err != nil {
		return err
	}
	s.recoverWorkAfterTeamDisabled(current.TenantID, current.ID, now, "deleted")
	return nil
}

func (s *agentTeamService) recoverWorkAfterTeamDisabled(tenantID, teamID int64, now time.Time, action string) {
	if recovered, recoverErr := TicketDispatchService.RecoverDisabledTeamAssignments(tenantID, teamID, now); recoverErr != nil {
		slog.Warn("recover pending ticket assignments after agent team unavailable failed",
			"tenant_id", tenantID,
			"team_id", teamID,
			"action", action,
			"recovered", recovered,
			"error", recoverErr,
		)
	}
	if recovered, recoverErr := ConversationDispatchService.RecoverDisabledTeamAssignments(tenantID, teamID, now); recoverErr != nil {
		slog.Warn("recover pending conversation assignments after agent team unavailable failed",
			"tenant_id", tenantID,
			"team_id", teamID,
			"action", action,
			"recovered", recovered,
			"error", recoverErr,
		)
	}
}

func (s *agentTeamService) buildTeamModel(id, tenantID int64, name string, leaderUserID int64, assignmentMode string, status int, description, remark string) (*models.AgentTeam, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errorsx.InvalidParamI18n("error.e0170")
	}
	if exists := s.Take("tenant_id = ? AND name = ? AND status <> ? AND id <> ?", tenantID, name, enums.StatusDeleted, id); exists != nil {
		return nil, errorsx.InvalidParamI18n("error.e0171")
	}
	if leaderUserID > 0 && UserService.Get(leaderUserID) == nil {
		return nil, errorsx.InvalidParamI18n("error.e0294")
	}
	if status != 0 && status != 1 {
		return nil, errorsx.InvalidParamI18n("error.e0174")
	}
	return &models.AgentTeam{
		TenantID:       tenantID,
		TeamType:       "custom",
		Name:           name,
		LeaderUserID:   leaderUserID,
		AssignmentMode: NormalizeAgentTeamAssignmentMode(assignmentMode),
		Status:         enums.Status(status),
		Description:    strings.TrimSpace(description),
		Remark:         strings.TrimSpace(remark),
	}, nil
}
