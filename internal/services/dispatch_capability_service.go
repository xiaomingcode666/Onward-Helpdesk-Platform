package services

import (
	"encoding/json"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"gorm.io/gorm"
)

type dispatchTicketCapabilityRequirement struct {
	SkillTags     []string
	ServiceRegion string
}

func buildDispatchTicketCapabilityRequirementDB(db *gorm.DB, ticket *models.Ticket) dispatchTicketCapabilityRequirement {
	req := dispatchTicketCapabilityRequirement{}
	if ticket == nil {
		return req
	}
	req.ServiceRegion = normalizeDispatchCapabilityValue(ticket.ServiceRegion)
	// Fault codes describe the observed failure, not a configured engineer skill.
	// Product-team membership remains the fallback until a product module provides
	// an explicit capability mapping.
	if db != nil && ticket.ProductModuleID > 0 && db.Migrator().HasTable(&models.ProductModule{}) {
		if module := repositories.ProductModuleRepository.Get(db, ticket.ProductModuleID); module != nil &&
			module.TenantID == ticket.TenantID &&
			(ticket.ProductID <= 0 || module.ProductID == ticket.ProductID) {
			req.SkillTags = appendNormalizedDispatchCapabilityValues(req.SkillTags, module.ModuleCode, module.Name)
		}
	}
	req.SkillTags = uniqueNormalizedDispatchCapabilityValues(req.SkillTags)
	return req
}

func filterDispatchCandidatesByTicketCapabilityDB(db *gorm.DB, ticket *models.Ticket, candidates []dispatchCandidate) ([]dispatchCandidate, string, error) {
	return filterDispatchCandidatesByTicketCapabilityWithOptionsDB(db, ticket, candidates, false)
}

func filterDispatchCandidatesByTicketCapabilityWithOptionsDB(db *gorm.DB, ticket *models.Ticket, candidates []dispatchCandidate, allowManualDisabled bool) ([]dispatchCandidate, string, error) {
	if len(candidates) == 0 || ticket == nil || db == nil {
		return candidates, "", nil
	}
	requirement := buildDispatchTicketCapabilityRequirementDB(db, ticket)
	if requirement.empty() || !db.Migrator().HasTable(&models.TenantMember{}) || !db.Migrator().HasTable(&models.EngineerProfile{}) {
		return candidates, "", nil
	}
	userIDs := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.profile.UserID > 0 {
			userIDs = append(userIDs, candidate.profile.UserID)
		}
	}
	membersByUserID, err := repositories.EnterpriseIAMRepository.FindTenantMembersByUserIDs(db, ticket.TenantID, userIDs)
	if err != nil {
		return nil, "", err
	}
	memberIDs := make([]int64, 0, len(membersByUserID))
	for _, member := range membersByUserID {
		if member != nil && member.ID > 0 {
			memberIDs = append(memberIDs, member.ID)
		}
	}
	engineersByMemberID, err := repositories.EnterpriseIAMRepository.FindEngineerProfilesByMemberIDs(db, ticket.TenantID, memberIDs)
	if err != nil {
		return nil, "", err
	}

	filtered := make([]dispatchCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if dispatchCandidateMatchesRequirementWithOptions(candidate.profile.UserID, requirement, membersByUserID, engineersByMemberID, allowManualDisabled) {
			filtered = append(filtered, candidate)
		}
	}
	if len(filtered) == 0 {
		return nil, "no_capability_match", nil
	}
	return filtered, "", nil
}

func validateTicketDispatchCapabilityDB(db *gorm.DB, ticket *models.Ticket, userID int64) error {
	return validateTicketDispatchCapabilityWithOptionsDB(db, ticket, userID, false)
}

func validateManualTicketDispatchCapabilityDB(db *gorm.DB, ticket *models.Ticket, userID int64) error {
	return validateTicketDispatchCapabilityWithOptionsDB(db, ticket, userID, true)
}

func validateTicketDispatchCapabilityWithOptionsDB(db *gorm.DB, ticket *models.Ticket, userID int64, allowManualDisabled bool) error {
	if db == nil || ticket == nil || userID <= 0 {
		return errConversationDispatchCandidateUnavailable
	}
	requirement := buildDispatchTicketCapabilityRequirementDB(db, ticket)
	if requirement.empty() || !db.Migrator().HasTable(&models.TenantMember{}) || !db.Migrator().HasTable(&models.EngineerProfile{}) {
		return nil
	}
	member := repositories.EnterpriseIAMRepository.FindTenantMemberByUserID(db, ticket.TenantID, userID)
	if member == nil {
		return nil
	}
	engineer, err := repositories.EnterpriseIAMRepository.FindEngineerProfileByMemberID(db, ticket.TenantID, member.ID)
	if err != nil {
		return err
	}
	if engineer == nil {
		return nil
	}
	if !engineerMatchesTicketCapabilityWithOptions(requirement, engineer, allowManualDisabled) {
		return errConversationDispatchCandidateUnavailable
	}
	return nil
}

func dispatchCandidateMatchesRequirement(
	userID int64,
	requirement dispatchTicketCapabilityRequirement,
	membersByUserID map[int64]*models.TenantMember,
	engineersByMemberID map[int64]*models.EngineerProfile,
) bool {
	return dispatchCandidateMatchesRequirementWithOptions(userID, requirement, membersByUserID, engineersByMemberID, false)
}

func dispatchCandidateMatchesRequirementWithOptions(
	userID int64,
	requirement dispatchTicketCapabilityRequirement,
	membersByUserID map[int64]*models.TenantMember,
	engineersByMemberID map[int64]*models.EngineerProfile,
	allowManualDisabled bool,
) bool {
	member := membersByUserID[userID]
	if member == nil {
		return true
	}
	engineer := engineersByMemberID[member.ID]
	if engineer == nil {
		return true
	}
	return engineerMatchesTicketCapabilityWithOptions(requirement, engineer, allowManualDisabled)
}

func engineerMatchesTicketCapability(requirement dispatchTicketCapabilityRequirement, engineer *models.EngineerProfile) bool {
	return engineerMatchesTicketCapabilityWithOptions(requirement, engineer, false)
}

func engineerMatchesTicketCapabilityWithOptions(requirement dispatchTicketCapabilityRequirement, engineer *models.EngineerProfile, allowManualDisabled bool) bool {
	if engineer == nil {
		return true
	}
	if engineer.Status != enums.StatusOk {
		return false
	}
	if !allowManualDisabled && !engineer.DispatchEnabled {
		return false
	}
	if requirement.ServiceRegion != "" {
		regions := normalizedDispatchCapabilityList(engineer.ServiceRegionsJSON)
		if len(regions) > 0 && !containsNormalizedDispatchCapabilityValue(regions, requirement.ServiceRegion) {
			return false
		}
	}
	if len(requirement.SkillTags) > 0 {
		skills := normalizedDispatchCapabilityList(engineer.SkillTagsJSON)
		if len(skills) == 0 || !intersectsNormalizedDispatchCapabilityValues(skills, requirement.SkillTags) {
			return false
		}
	}
	return true
}

func (r dispatchTicketCapabilityRequirement) empty() bool {
	return len(r.SkillTags) == 0 && strings.TrimSpace(r.ServiceRegion) == ""
}

func normalizedDispatchCapabilityList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return splitDispatchCapabilityCSV(raw)
	}
	return uniqueNormalizedDispatchCapabilityValues(values)
}

func splitDispatchCapabilityCSV(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '|' || r == '\n' || r == '\t'
	})
	return uniqueNormalizedDispatchCapabilityValues(parts)
}

func appendNormalizedDispatchCapabilityValues(values []string, candidates ...string) []string {
	for _, value := range candidates {
		normalized := normalizeDispatchCapabilityValue(value)
		if normalized == "" {
			continue
		}
		values = append(values, normalized)
	}
	return values
}

func uniqueNormalizedDispatchCapabilityValues(values []string) []string {
	ret := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := normalizeDispatchCapabilityValue(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		ret = append(ret, normalized)
	}
	return ret
}

func normalizeDispatchCapabilityValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func containsNormalizedDispatchCapabilityValue(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func intersectsNormalizedDispatchCapabilityValues(left, right []string) bool {
	rightSet := make(map[string]struct{}, len(right))
	for _, value := range right {
		rightSet[value] = struct{}{}
	}
	for _, value := range left {
		if _, ok := rightSet[value]; ok {
			return true
		}
	}
	return false
}
