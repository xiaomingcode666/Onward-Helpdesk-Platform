package builders

import (
	"strings"

	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/services"
)

func BuildCustomerRegistrationContext(item *services.CustomerRegistrationContext) *response.CustomerRegistrationContextResponse {
	if item == nil {
		return nil
	}
	ret := &response.CustomerRegistrationContextResponse{
		Method: item.Method, DomainType: strings.TrimSpace(item.DomainType), Email: strings.TrimSpace(item.Email), DisplayName: strings.TrimSpace(item.DisplayName),
		Registered: item.Registered,
		Tenant:     BuildResolvedTenant(item.Tenant), Product: BuildResolvedProduct(item.Product), Device: BuildResolvedDevice(item.Device),
	}
	if item.ExpiresAt != nil {
		ret.ExpiresAt = utils.FormatTime(*item.ExpiresAt)
	}
	if item.CustomerOrg != nil {
		ret.CustomerOrg = &response.CustomerRegistrationOrg{ID: item.CustomerOrg.ID, Name: strings.TrimSpace(item.CustomerOrg.Name)}
	}
	if item.PartnerCompany != nil {
		ret.Partner = &response.CustomerRegistrationOrg{ID: item.PartnerCompany.ID, Name: strings.TrimSpace(item.PartnerCompany.Name)}
	}
	return ret
}
