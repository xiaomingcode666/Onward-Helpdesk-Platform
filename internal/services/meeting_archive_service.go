package services

import (
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"gorm.io/gorm"
)

func loadMeetingArchiveData(
	db *gorm.DB,
	meetingIDs []string,
) (map[string][]dto.MeetingParticipantDTO, map[string]repositories.MeetingArchiveStats) {
	participants := repositories.MeetingRoomRepository.ListAttendedParticipantsByMeetingIDs(db, meetingIDs)
	tenantByMeeting := meetingTenantIDs(db, meetingIDs)
	participantsByMeeting := make(map[string][]dto.MeetingParticipantDTO, len(meetingIDs))
	for _, meetingID := range meetingIDs {
		participantsByMeeting[meetingID] = []dto.MeetingParticipantDTO{}
	}
	for i := range participants {
		item := participants[i]
		participantsByMeeting[item.MeetingID] = append(
			participantsByMeeting[item.MeetingID],
			buildMeetingParticipantDTO(db, tenantByMeeting[item.MeetingID], &item),
		)
	}
	return participantsByMeeting, repositories.MeetingIntelligenceRepository.ArchiveStatsByMeetingIDs(db, meetingIDs)
}

func buildMeetingParticipantDTO(db *gorm.DB, tenantID int64, item *models.MeetingParticipant) dto.MeetingParticipantDTO {
	if item == nil {
		return dto.MeetingParticipantDTO{}
	}
	duration := item.Duration
	if duration <= 0 && item.JoinedAt != nil && item.LeftAt != nil && item.LeftAt.After(*item.JoinedAt) {
		duration = int64(item.LeftAt.Sub(*item.JoinedAt).Round(time.Second).Seconds())
	}
	return dto.MeetingParticipantDTO{
		ID:              item.ID,
		Name:            item.ParticipantName,
		UserType:        item.UserType,
		IdentityType:    meetingParticipantIdentityType(db, tenantID, item),
		Role:            item.Role,
		JoinedAt:        formatEnterpriseTimePtr(item.JoinedAt),
		LeftAt:          formatEnterpriseTimePtr(item.LeftAt),
		DurationSeconds: duration,
	}
}

func meetingTenantIDs(db *gorm.DB, meetingIDs []string) map[string]int64 {
	ret := make(map[string]int64, len(meetingIDs))
	if db == nil || len(meetingIDs) == 0 {
		return ret
	}
	var meetings []models.MeetingRoomJitsi
	if err := db.Select("id", "tenant_id").Where("id IN ?", meetingIDs).Find(&meetings).Error; err != nil {
		return ret
	}
	for _, meeting := range meetings {
		ret[meeting.ID] = meeting.TenantID
	}
	return ret
}

func meetingEnterpriseParticipantTypes() []string {
	return []string{meetingParticipantTypeEnterprise, "enterprise_participant", meetingParticipantTypeAuthorizedSupport, meetingParticipantTypePlatformAdmin, meetingParticipantTypeTenantAdmin}
}

func meetingParticipantIdentityType(db *gorm.DB, tenantID int64, item *models.MeetingParticipant) string {
	if item == nil {
		return ""
	}
	switch strings.TrimSpace(item.UserType) {
	case meetingParticipantTypePlatformAdmin:
		return meetingParticipantTypePlatformAdmin
	case meetingParticipantTypeTenantAdmin:
		return meetingParticipantTypeTenantAdmin
	case meetingParticipantTypeAuthorizedSupport:
		return meetingParticipantTypeAuthorizedSupport
	case "customer":
		return "customer"
	case "partner", "supplier", "external":
		return "external"
	}

	userID, err := strconv.ParseInt(strings.TrimSpace(item.UserID), 10, 64)
	if err != nil || userID <= 0 || db == nil {
		return "repair_engineer"
	}
	if db.Migrator().HasTable(&models.PlatformStaffProfile{}) {
		if staff := repositories.PlatformIAMRepository.FindPlatformStaffByUserID(db, userID); staff != nil && staff.Status == enums.StatusOk {
			if tenantID > 0 && hasActivePlatformTenantGrant(db, staff.ID, tenantID) {
				return meetingParticipantTypeAuthorizedSupport
			}
			return meetingParticipantTypePlatformAdmin
		}
	}
	if hasLegacyMeetingRole(db, userID, 0, []string{
		constants.RoleCodeSuperAdmin,
		constants.RoleCodeAdmin,
		PlatformRoleAdmin,
		"platform_staff",
	}) {
		return meetingParticipantTypePlatformAdmin
	}
	if tenantID > 0 && db.Migrator().HasTable(&models.TenantMember{}) &&
		db.Migrator().HasTable(&models.AuthRoleBinding{}) && db.Migrator().HasTable(&models.AuthRole{}) {
		member := repositories.EnterpriseIAMRepository.FindTenantMemberByUserID(db, tenantID, userID)
		if member != nil && member.Status == enums.StatusOk {
			bindings, err := repositories.PlatformIAMRepository.FindRoleBindings(db, tenantID, models.DomainTypeEnterprise, models.SubjectTypeTenantMember, member.ID)
			if err == nil && len(bindings) > 0 {
				roleIDs := make([]int64, 0, len(bindings))
				for _, binding := range bindings {
					roleIDs = append(roleIDs, binding.RoleID)
				}
				roles, err := repositories.PlatformIAMRepository.FindAuthRolesByIDs(db, roleIDs)
				if err == nil {
					for _, role := range roles {
						if role != nil && (role.Code == EnterpriseRoleOwner || role.Code == EnterpriseRoleAdmin || role.Code == EnterpriseRoleServiceManager) {
							return meetingParticipantTypeTenantAdmin
						}
					}
				}
			}
		}
	}
	if tenantID > 0 && hasLegacyMeetingRole(db, userID, tenantID, []string{EnterpriseRoleOwner, EnterpriseRoleAdmin}) {
		return meetingParticipantTypeTenantAdmin
	}
	return "repair_engineer"
}

func hasLegacyMeetingRole(db *gorm.DB, userID, tenantID int64, roleCodes []string) bool {
	if db == nil || userID <= 0 || len(roleCodes) == 0 ||
		!db.Migrator().HasTable(&models.Role{}) || !db.Migrator().HasTable(&models.UserRole{}) {
		return false
	}
	query := db.Table("t_role AS r").
		Joins("JOIN t_user_role AS ur ON ur.role_id = r.id").
		Where("ur.user_id = ? AND r.code IN ? AND r.status = ?", userID, roleCodes, enums.StatusOk)
	if tenantID == 0 {
		query = query.Where("r.tenant_id = 0")
	} else {
		query = query.Where("(ur.tenant_id = ? OR (ur.tenant_id = 0 AND r.tenant_id = ?))", tenantID, tenantID)
	}
	var count int64
	return query.Count(&count).Error == nil && count > 0
}

func hasActivePlatformTenantGrant(db *gorm.DB, platformStaffID, tenantID int64) bool {
	if db == nil || platformStaffID <= 0 || tenantID <= 0 || !db.Migrator().HasTable(&models.PlatformTenantGrant{}) {
		return false
	}
	var grant models.PlatformTenantGrant
	return db.Where("platform_staff_id = ? AND tenant_id = ? AND status = ? AND revoked_at IS NULL AND expired_at > ?", platformStaffID, tenantID, models.AccessGrantStatusActive, time.Now()).First(&grant).Error == nil
}
