package services

import (
	"fmt"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

const (
	AgentTeamTypeTechnicalRepair = "technical_repair"
	AgentTeamTypeProductRepair   = "product_repair"

	TechnicalAfterSalesTeamName  = "技术售后组"
	TechnicalMaintenanceTeamName = "技术维护组"
	TechnicalAfterSalesTeamPath  = "/" + TechnicalAfterSalesTeamName
	TechnicalMaintenanceTeamPath = "/" + TechnicalMaintenanceTeamName

	technicalRepairDepartmentCode = "technical-repair"
	technicalRepairTeamSystemKey  = "technical-repair"
)

var ProductSupportOrganizationService = &productSupportOrganizationService{}

type productSupportOrganizationService struct{}

// EnsureTenantTechnicalRepairTeamDB creates the tenant-owned service organization root.
// The matching Department keeps it visible in the enterprise organization tree while
// AgentTeam remains the executable queue and schedule boundary used by dispatch.
func (s *productSupportOrganizationService) EnsureTenantTechnicalRepairTeamDB(
	db *gorm.DB,
	tenantID int64,
	operator *dto.AuthPrincipal,
) (*models.AgentTeam, error) {
	if tenantID <= 0 {
		return nil, fmt.Errorf("tenant is required")
	}
	technicalTeamName := tenantTechnicalRepairTeamNameDB(db, tenantID)
	technicalTeamPath := "/" + supportPathSegment(technicalTeamName)
	technicalTeamDescription := "接收产品售后会话、维修工单和工程师排班。"
	if technicalTeamName == TechnicalMaintenanceTeamName {
		technicalTeamDescription = "接收知识服务会话、人工工单和工程师排班。"
	}
	rootDepartment, err := s.ensureRootDepartmentDB(db, tenantID, operator)
	if err != nil {
		return nil, err
	}
	technicalDepartment, err := s.ensureDepartmentDB(db, models.Department{
		TenantID:       tenantID,
		ParentID:       rootDepartment.ID,
		DepartmentCode: technicalRepairDepartmentCode,
		Name:           technicalTeamName,
		Path:           technicalTeamPath,
		Depth:          1,
		Status:         enums.StatusOk,
	}, operator)
	if err != nil {
		return nil, err
	}

	key := technicalRepairTeamSystemKey
	team := repositories.AgentTeamRepository.FindOne(db, sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("system_key", key))
	if team == nil {
		team = repositories.AgentTeamRepository.FindOne(db, sqls.NewCnd().
			Eq("tenant_id", tenantID).
			Eq("team_type", AgentTeamTypeTechnicalRepair).
			NotEq("status", enums.StatusDeleted).
			Asc("id"))
	}
	if team == nil {
		team = &models.AgentTeam{
			TenantID:       tenantID,
			DepartmentID:   technicalDepartment.ID,
			TeamType:       AgentTeamTypeTechnicalRepair,
			SystemKey:      &key,
			SystemManaged:  true,
			Name:           technicalTeamName,
			AssignmentMode: AgentTeamAssignmentModeBalanced,
			Status:         enums.StatusOk,
			Description:    technicalTeamDescription,
			AuditFields:    utils.BuildAuditFields(operator),
		}
		if err := repositories.AgentTeamRepository.Create(db, team); err != nil {
			return nil, err
		}
		return team, nil
	}
	if err := repositories.AgentTeamRepository.Updates(db, team.ID, map[string]any{
		"tenant_id":        tenantID,
		"parent_id":        0,
		"product_id":       0,
		"department_id":    technicalDepartment.ID,
		"team_type":        AgentTeamTypeTechnicalRepair,
		"system_key":       key,
		"system_managed":   true,
		"name":             technicalTeamName,
		"assignment_mode":  AgentTeamAssignmentModeBalanced,
		"status":           enums.StatusOk,
		"description":      technicalTeamDescription,
		"update_user_id":   auditOperatorID(operator),
		"update_user_name": auditOperatorName(operator),
		"updated_at":       time.Now(),
	}); err != nil {
		return nil, err
	}
	return repositories.AgentTeamRepository.Get(db, team.ID), nil
}

func (s *productSupportOrganizationService) EnsureProductRepairTeamDB(
	db *gorm.DB,
	product *models.Product,
	operator *dto.AuthPrincipal,
) (*models.AgentTeam, error) {
	if product == nil || product.ID <= 0 || product.TenantID <= 0 {
		return nil, fmt.Errorf("product is required")
	}
	rootTeam, err := s.EnsureTenantTechnicalRepairTeamDB(db, product.TenantID, operator)
	if err != nil {
		return nil, err
	}
	productDepartment, err := s.ensureDepartmentDB(db, models.Department{
		TenantID:       product.TenantID,
		ParentID:       rootTeam.DepartmentID,
		DepartmentCode: fmt.Sprintf("product-%d", product.ID),
		Name:           strings.TrimSpace(product.Name),
		Path:           "/" + supportPathSegment(rootTeam.Name) + "/" + supportPathSegment(product.Name),
		Depth:          2,
		Status:         product.Status,
	}, operator)
	if err != nil {
		return nil, err
	}
	ownerMember := s.productOwnerMemberDB(db, product)
	leaderUserID := int64(0)
	if ownerMember != nil {
		leaderUserID = ownerMember.UserID
	}

	key := fmt.Sprintf("product:%d", product.ID)
	team := repositories.AgentTeamRepository.FindOne(db, sqls.NewCnd().
		Eq("tenant_id", product.TenantID).
		Eq("system_key", key))
	if team == nil {
		team = &models.AgentTeam{
			TenantID:       product.TenantID,
			ParentID:       rootTeam.ID,
			ProductID:      product.ID,
			DepartmentID:   productDepartment.ID,
			TeamType:       AgentTeamTypeProductRepair,
			SystemKey:      &key,
			SystemManaged:  true,
			Name:           strings.TrimSpace(product.Name),
			LeaderUserID:   leaderUserID,
			AssignmentMode: AgentTeamAssignmentModeBalanced,
			Status:         product.Status,
			Description:    fmt.Sprintf("%s 产品维修组", strings.TrimSpace(product.Name)),
			AuditFields:    utils.BuildAuditFields(operator),
		}
		if err := repositories.AgentTeamRepository.Create(db, team); err != nil {
			return nil, err
		}
		if err := s.ensureProductRepairSupervisorDB(db, product.TenantID, team, ownerMember, operator); err != nil {
			return nil, err
		}
		return team, nil
	}
	updates := map[string]any{
		"tenant_id":        product.TenantID,
		"parent_id":        rootTeam.ID,
		"product_id":       product.ID,
		"department_id":    productDepartment.ID,
		"team_type":        AgentTeamTypeProductRepair,
		"system_managed":   true,
		"name":             strings.TrimSpace(product.Name),
		"assignment_mode":  NormalizeAgentTeamAssignmentMode(team.AssignmentMode),
		"status":           product.Status,
		"description":      fmt.Sprintf("%s 产品维修组", strings.TrimSpace(product.Name)),
		"update_user_id":   auditOperatorID(operator),
		"update_user_name": auditOperatorName(operator),
		"updated_at":       time.Now(),
	}
	if leaderUserID > 0 {
		updates["leader_user_id"] = leaderUserID
	}
	if err := repositories.AgentTeamRepository.Updates(db, team.ID, updates); err != nil {
		return nil, err
	}
	updated := repositories.AgentTeamRepository.Get(db, team.ID)
	if err := s.ensureProductRepairSupervisorDB(db, product.TenantID, updated, ownerMember, operator); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *productSupportOrganizationService) productOwnerMemberDB(db *gorm.DB, product *models.Product) *models.TenantMember {
	if db == nil || product == nil || product.TenantID <= 0 || product.OwnerMemberID <= 0 {
		return nil
	}
	return repositories.EnterpriseIAMRepository.GetTenantMember(db, product.TenantID, product.OwnerMemberID)
}

func (s *productSupportOrganizationService) ensureProductRepairSupervisorDB(
	db *gorm.DB,
	tenantID int64,
	team *models.AgentTeam,
	member *models.TenantMember,
	operator *dto.AuthPrincipal,
) error {
	if db == nil || team == nil || member == nil || tenantID <= 0 || team.ID <= 0 || member.UserID <= 0 {
		return nil
	}
	if team.TenantID != tenantID {
		return fmt.Errorf("product repair team tenant mismatch")
	}
	if team.Status != enums.StatusOk {
		return nil
	}
	_, err := AgentTeamMemberService.EnsureMemberPreservingDispatchDB(
		db,
		tenantID,
		team.ID,
		member.UserID,
		member.ID,
		defaultAgentTeamMemberWeight,
		false,
		operator,
	)
	return err
}

func (s *productSupportOrganizationService) FindProductRepairTeam(db *gorm.DB, tenantID, productID int64) *models.AgentTeam {
	if tenantID <= 0 || productID <= 0 {
		return nil
	}
	return repositories.AgentTeamRepository.FindOne(db, sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Eq("team_type", AgentTeamTypeProductRepair).
		Eq("status", enums.StatusOk))
}

func (s *productSupportOrganizationService) EnsureProductAgentTeamBindingDB(db *gorm.DB, agent *models.AIAgent, teamID int64) error {
	if agent == nil || teamID <= 0 {
		return nil
	}
	team := repositories.AgentTeamRepository.Get(db, teamID)
	if team == nil || team.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("agent team not found")
	}
	if team.Status != enums.StatusOk {
		return errorsx.InvalidParam("agent team is not active")
	}
	if agent.TenantID > 0 && team.TenantID != agent.TenantID {
		return errorsx.InvalidParam("agent team scope mismatch")
	}
	if agent.ProductID > 0 && (team.TeamType != AgentTeamTypeProductRepair || team.ProductID != agent.ProductID) {
		return errorsx.InvalidParam("agent team does not belong to product")
	}
	teamIDs := utils.SplitInt64s(agent.TeamIDs)
	for _, existingID := range teamIDs {
		if existingID == teamID {
			return nil
		}
	}
	teamIDs = append([]int64{teamID}, teamIDs...)
	return repositories.AIAgentRepository.Updates(db, agent.ID, map[string]any{
		"team_ids":     utils.JoinInt64s(teamIDs),
		"handoff_mode": enums.AIAgentHandoffModeDefaultTeamPool,
		"updated_at":   time.Now(),
	})
}

// EnsureEngineerAgentProfileDB bridges enterprise IAM engineers into the
// executable dispatch model. Existing profiles keep their product-team
// assignment; newly invited engineers start in the tenant repair pool.
func (s *productSupportOrganizationService) EnsureEngineerAgentProfileDB(
	db *gorm.DB,
	member *models.TenantMember,
	engineer *models.EngineerProfile,
	operator *dto.AuthPrincipal,
) (*models.AgentProfile, error) {
	if member == nil || engineer == nil || member.ID <= 0 || member.UserID <= 0 || member.TenantID <= 0 {
		return nil, fmt.Errorf("engineer member is required")
	}
	if engineer.MemberID != member.ID || engineer.TenantID != member.TenantID {
		return nil, fmt.Errorf("engineer profile does not belong to member")
	}
	if err := EnsureTenantDefaultIAMRolesDB(db, member.TenantID, operator); err != nil {
		return nil, err
	}
	if err := ensureIAMRoleBindingDB(
		db,
		member.TenantID,
		models.DomainTypeEnterprise,
		models.SubjectTypeTenantMember,
		member.ID,
		EnterpriseRoleEngineer,
		operator,
	); err != nil {
		return nil, err
	}
	profile := repositories.AgentProfileRepository.FindOne(db, sqls.NewCnd().Eq("user_id", member.UserID))
	if profile != nil {
		if profile.TenantID != member.TenantID {
			return nil, fmt.Errorf("engineer already has a dispatch profile in another tenant")
		}
		if profile.TeamID > 0 {
			if team := repositories.AgentTeamRepository.Get(db, profile.TeamID); team != nil && team.TenantID == member.TenantID && team.Status == enums.StatusOk {
				if _, err := AgentTeamMemberService.EnsureMemberPreservingDispatchDB(db, member.TenantID, profile.TeamID, member.UserID, member.ID, defaultAgentTeamMemberWeight, engineer.DispatchEnabled, operator); err != nil {
					return nil, err
				}
			}
		}
		return profile, nil
	}

	team, err := s.EnsureTenantTechnicalRepairTeamDB(db, member.TenantID, operator)
	if err != nil {
		return nil, err
	}
	maxConcurrent := engineer.MaxTicketLoad
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}
	displayName := strings.TrimSpace(member.DisplayName)
	if displayName == "" {
		displayName = fmt.Sprintf("工程师 %d", member.UserID)
	}
	profile = &models.AgentProfile{
		TenantID:              member.TenantID,
		UserID:                member.UserID,
		TeamID:                team.ID,
		AgentCode:             fmt.Sprintf("ENG-%d-%d", member.TenantID, member.UserID),
		DisplayName:           displayName,
		ServiceStatus:         enums.ServiceStatusIdle,
		MaxConcurrentCount:    maxConcurrent,
		PriorityLevel:         50,
		AutoAssignEnabled:     engineer.DispatchEnabled,
		ReceiveOfflineMessage: true,
		Status:                enums.StatusOk,
		Remark:                "企业维修工程师派单档案",
		AuditFields:           utils.BuildAuditFields(operator),
	}
	if err := repositories.AgentProfileRepository.Create(db, profile); err != nil {
		return nil, err
	}
	if _, err := AgentTeamMemberService.EnsureMemberPreservingDispatchDB(db, member.TenantID, team.ID, member.UserID, member.ID, defaultAgentTeamMemberWeight, engineer.DispatchEnabled, operator); err != nil {
		return nil, err
	}

	// A member explicitly assigned to another department keeps that structure.
	// Root/unassigned members enter the technical repair organization by default.
	currentDepartment := repositories.EnterpriseIAMRepository.GetDepartment(db, member.TenantID, member.DepartmentID)
	if currentDepartment == nil || currentDepartment.DepartmentCode == "root" {
		if err := repositories.EnterpriseIAMRepository.UpdateTenantMemberDepartmentByUserID(db, member.TenantID, member.UserID, team.DepartmentID); err != nil {
			return nil, err
		}
		member.DepartmentID = team.DepartmentID
	}
	return profile, nil
}

func (s *productSupportOrganizationService) SyncEngineerAgentProfilesDB(db *gorm.DB, operator *dto.AuthPrincipal) error {
	engineers := make([]models.EngineerProfile, 0)
	if err := db.Where("status <> ?", enums.StatusDeleted).Order("tenant_id ASC, id ASC").Find(&engineers).Error; err != nil {
		return err
	}
	for i := range engineers {
		member := &models.TenantMember{}
		if err := db.Where("id = ? AND tenant_id = ? AND status = ?", engineers[i].MemberID, engineers[i].TenantID, enums.StatusOk).
			First(member).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			return err
		}
		if _, err := s.EnsureEngineerAgentProfileDB(db, member, &engineers[i], operator); err != nil {
			return err
		}
	}
	return nil
}

func (s *productSupportOrganizationService) BackfillAllDB(db *gorm.DB, operator *dto.AuthPrincipal) error {
	tenants := make([]models.Tenant, 0)
	if err := db.Where("status <> ?", enums.StatusDeleted).Order("id ASC").Find(&tenants).Error; err != nil {
		return err
	}
	for i := range tenants {
		if _, err := s.EnsureTenantTechnicalRepairTeamDB(db, tenants[i].ID, operator); err != nil {
			return err
		}
	}
	if err := s.SyncEngineerAgentProfilesDB(db, operator); err != nil {
		return err
	}
	products := make([]models.Product, 0)
	if err := db.Where("status <> ?", enums.StatusDeleted).Order("tenant_id ASC, id ASC").Find(&products).Error; err != nil {
		return err
	}
	for i := range products {
		team, err := s.EnsureProductRepairTeamDB(db, &products[i], operator)
		if err != nil {
			return err
		}
		if products[i].Status != enums.StatusOk || team == nil || team.Status != enums.StatusOk {
			continue
		}
		agents := repositories.AIAgentRepository.Find(db, sqls.NewCnd().
			Eq("tenant_id", products[i].TenantID).
			Eq("product_id", products[i].ID).
			NotEq("status", enums.StatusDeleted))
		for j := range agents {
			if err := s.EnsureProductAgentTeamBindingDB(db, &agents[j], team.ID); err != nil {
				return err
			}
		}
	}
	profiles := make([]models.AgentProfile, 0)
	if err := db.Where("tenant_id = 0 AND team_id > 0").Find(&profiles).Error; err != nil {
		return err
	}
	for i := range profiles {
		if team := repositories.AgentTeamRepository.Get(db, profiles[i].TeamID); team != nil && team.TenantID > 0 {
			if err := repositories.AgentProfileRepository.Updates(db, profiles[i].ID, map[string]any{"tenant_id": team.TenantID}); err != nil {
				return err
			}
			profiles[i].TenantID = team.TenantID
		}
		if profiles[i].TenantID > 0 && profiles[i].TeamID > 0 && profiles[i].UserID > 0 {
			if team := repositories.AgentTeamRepository.Get(db, profiles[i].TeamID); team != nil && team.TenantID == profiles[i].TenantID && team.Status == enums.StatusOk {
				if _, err := AgentTeamMemberService.EnsureMemberPreservingDispatchDB(db, profiles[i].TenantID, profiles[i].TeamID, profiles[i].UserID, 0, defaultAgentTeamMemberWeight, profiles[i].AutoAssignEnabled, operator); err != nil {
					return err
				}
			}
		}
	}
	pendingConversations := make([]models.Conversation, 0)
	if err := db.Where("current_team_id = 0 AND product_id > 0 AND status = ?", enums.IMConversationStatusPending).
		Find(&pendingConversations).Error; err != nil {
		return err
	}
	for i := range pendingConversations {
		team := s.FindProductRepairTeam(db, pendingConversations[i].TenantID, pendingConversations[i].ProductID)
		if team != nil {
			if err := repositories.ConversationRepository.Updates(db, pendingConversations[i].ID, map[string]any{"current_team_id": team.ID}); err != nil {
				return err
			}
		}
	}
	tickets := make([]models.Ticket, 0)
	if err := db.Where("current_team_id = 0").Find(&tickets).Error; err != nil {
		return err
	}
	for i := range tickets {
		teamID := int64(0)
		if tickets[i].ConversationID > 0 {
			if conversation := repositories.ConversationRepository.Get(db, tickets[i].ConversationID); conversation != nil {
				teamID = conversation.CurrentTeamID
			}
		}
		if teamID <= 0 && tickets[i].ProductID > 0 {
			if team := s.FindProductRepairTeam(db, tickets[i].TenantID, tickets[i].ProductID); team != nil {
				teamID = team.ID
			}
		}
		if teamID > 0 {
			if err := repositories.TicketRepository.Updates(db, tickets[i].ID, map[string]any{"current_team_id": teamID}); err != nil {
				return err
			}
		}
	}
	return nil
}

func tenantTechnicalRepairTeamNameDB(db *gorm.DB, tenantID int64) string {
	tenant := repositories.PlatformIAMRepository.GetTenant(db, tenantID)
	if tenant != nil && tenant.IsKnowledgeSupportScene() {
		return TechnicalMaintenanceTeamName
	}
	return TechnicalAfterSalesTeamName
}

func (s *productSupportOrganizationService) ensureRootDepartmentDB(db *gorm.DB, tenantID int64, operator *dto.AuthPrincipal) (*models.Department, error) {
	return s.ensureDepartmentDB(db, models.Department{
		TenantID:       tenantID,
		DepartmentCode: "root",
		Name:           "企业组织",
		Path:           "/",
		Depth:          0,
		Status:         enums.StatusOk,
	}, operator)
}

func (s *productSupportOrganizationService) ensureDepartmentDB(db *gorm.DB, desired models.Department, operator *dto.AuthPrincipal) (*models.Department, error) {
	department := repositories.EnterpriseIAMRepository.GetDepartmentByCode(db, desired.TenantID, desired.DepartmentCode)
	if department == nil {
		desired.AuditFields = utils.BuildAuditFields(operator)
		if err := db.Create(&desired).Error; err != nil {
			return nil, err
		}
		return &desired, nil
	}
	if err := db.Model(&models.Department{}).
		Where("id = ? AND tenant_id = ?", department.ID, desired.TenantID).
		Updates(map[string]any{
			"parent_id":        desired.ParentID,
			"name":             desired.Name,
			"path":             desired.Path,
			"depth":            desired.Depth,
			"status":           desired.Status,
			"update_user_id":   auditOperatorID(operator),
			"update_user_name": auditOperatorName(operator),
			"updated_at":       time.Now(),
		}).Error; err != nil {
		return nil, err
	}
	return repositories.EnterpriseIAMRepository.GetDepartment(db, desired.TenantID, department.ID), nil
}

func supportPathSegment(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "/", "-"))
	if value == "" {
		return "未命名产品"
	}
	return value
}

func auditOperatorID(operator *dto.AuthPrincipal) int64 {
	if operator == nil {
		return 0
	}
	return operator.UserID
}

func auditOperatorName(operator *dto.AuthPrincipal) string {
	if operator == nil || strings.TrimSpace(operator.Username) == "" {
		return "system"
	}
	return operator.Username
}
