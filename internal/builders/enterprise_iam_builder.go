package builders

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
)

func BuildEnterpriseIAMMemberList(items []models.TenantMember, users map[int64]*models.User, departments map[int64]*models.Department, engineers map[int64]*models.EngineerProfile, productGroups map[int64][]dto.EnterpriseIAMProductGroupDTO, rolesBySubject map[int64][]string) []*dto.EnterpriseIAMMemberDTO {
	ret := make([]*dto.EnterpriseIAMMemberDTO, 0, len(items))
	for i := range items {
		ret = append(ret, BuildEnterpriseIAMMember(&items[i], users[items[i].UserID], departments[items[i].DepartmentID], engineers[items[i].ID], productGroups[items[i].ID], rolesBySubject[items[i].ID]))
	}
	return ret
}

func BuildEnterpriseIAMMember(item *models.TenantMember, user *models.User, department *models.Department, engineer *models.EngineerProfile, productGroups []dto.EnterpriseIAMProductGroupDTO, roles []string) *dto.EnterpriseIAMMemberDTO {
	if item == nil {
		return nil
	}
	ret := &dto.EnterpriseIAMMemberDTO{
		ID:              item.ID,
		TenantID:        item.TenantID,
		UserID:          item.UserID,
		DisplayName:     item.DisplayName,
		MemberNo:        item.MemberNo,
		DepartmentID:    item.DepartmentID,
		JobTitle:        item.JobTitle,
		MemberType:      item.MemberType,
		Roles:           roles,
		Status:          int(item.Status),
		JoinedAt:        formatTimePtr(item.JoinedAt),
		UpdatedAt:       formatTime(item.UpdatedAt),
		DispatchEnabled: false,
		ProductGroups:   productGroups,
	}
	if user != nil {
		ret.Username = user.Username
		if user.Email != nil {
			ret.Email = *user.Email
		}
		if user.Mobile != nil {
			ret.Mobile = *user.Mobile
		}
		if ret.DisplayName == "" {
			ret.DisplayName = user.Nickname
		}
	}
	if department != nil {
		ret.DepartmentName = department.Name
	}
	if engineer != nil {
		ret.SkillTagsJSON = engineer.SkillTagsJSON
		ret.LanguagesJSON = engineer.LanguagesJSON
		ret.ServiceRegionsJSON = engineer.ServiceRegionsJSON
		ret.DispatchEnabled = engineer.DispatchEnabled
	}
	if ret.Roles == nil {
		ret.Roles = []string{}
	}
	if ret.ProductGroups == nil {
		ret.ProductGroups = []dto.EnterpriseIAMProductGroupDTO{}
	}
	return ret
}

func BuildEnterpriseIAMCustomerUserList(items []models.CustomerUser, orgs map[int64]*models.CustomerOrg, rolesBySubject map[int64][]string) []*dto.EnterpriseIAMCustomerUserDTO {
	ret := make([]*dto.EnterpriseIAMCustomerUserDTO, 0, len(items))
	for i := range items {
		ret = append(ret, BuildEnterpriseIAMCustomerUser(&items[i], orgs[items[i].CustomerOrgID], rolesBySubject[items[i].ID]))
	}
	return ret
}

func BuildEnterpriseIAMCustomerUser(item *models.CustomerUser, org *models.CustomerOrg, roles []string) *dto.EnterpriseIAMCustomerUserDTO {
	if item == nil {
		return nil
	}
	ret := &dto.EnterpriseIAMCustomerUserDTO{
		ID:            item.ID,
		TenantID:      item.TenantID,
		CustomerOrgID: item.CustomerOrgID,
		UserID:        item.UserID,
		DisplayName:   item.DisplayName,
		Email:         item.Email,
		Phone:         item.Phone,
		Locale:        item.Locale,
		Timezone:      item.Timezone,
		Roles:         roles,
		Status:        int(item.Status),
		LastSeenAt:    formatTimePtr(item.LastSeenAt),
		UpdatedAt:     formatTime(item.UpdatedAt),
	}
	if org != nil {
		ret.CustomerOrgName = org.Name
	}
	if ret.Roles == nil {
		ret.Roles = []string{}
	}
	return ret
}

func BuildEnterpriseIAMPartnerList(items []models.PartnerCompany, accountCounts, contractCounts map[int64]int64) []*dto.EnterpriseIAMPartnerDTO {
	ret := make([]*dto.EnterpriseIAMPartnerDTO, 0, len(items))
	for i := range items {
		ret = append(ret, BuildEnterpriseIAMPartner(&items[i], accountCounts[items[i].ID], contractCounts[items[i].ID]))
	}
	return ret
}

func BuildEnterpriseIAMPartner(item *models.PartnerCompany, accountCount, contractCount int64) *dto.EnterpriseIAMPartnerDTO {
	if item == nil {
		return nil
	}
	return &dto.EnterpriseIAMPartnerDTO{
		ID:            item.ID,
		TenantID:      item.TenantID,
		PartnerNo:     item.PartnerNo,
		Name:          item.Name,
		PartnerType:   item.PartnerType,
		CountryRegion: item.CountryRegion,
		ContactName:   item.ContactName,
		AccountCount:  accountCount,
		ContractCount: contractCount,
		Status:        int(item.Status),
		UpdatedAt:     formatTime(item.UpdatedAt),
	}
}

func BuildEnterpriseIAMDepartmentList(items []models.Department, managers map[int64]*models.TenantMember, memberCounts map[int64]int64) []*dto.EnterpriseIAMDepartmentDTO {
	ret := make([]*dto.EnterpriseIAMDepartmentDTO, 0, len(items))
	for i := range items {
		ret = append(ret, BuildEnterpriseIAMDepartment(&items[i], managers[items[i].ManagerMemberID], memberCounts[items[i].ID]))
	}
	return ret
}

func BuildEnterpriseIAMDepartment(item *models.Department, manager *models.TenantMember, memberCount int64) *dto.EnterpriseIAMDepartmentDTO {
	if item == nil {
		return nil
	}
	ret := &dto.EnterpriseIAMDepartmentDTO{
		ID:              item.ID,
		TenantID:        item.TenantID,
		ParentID:        item.ParentID,
		DepartmentCode:  item.DepartmentCode,
		Name:            item.Name,
		Path:            item.Path,
		Depth:           item.Depth,
		ManagerMemberID: item.ManagerMemberID,
		RegionCode:      item.RegionCode,
		MemberCount:     memberCount,
		Status:          int(item.Status),
		UpdatedAt:       formatTime(item.UpdatedAt),
	}
	if manager != nil {
		ret.ManagerName = manager.DisplayName
	}
	return ret
}
