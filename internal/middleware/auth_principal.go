package middleware

import "remotehelpdesk/internal/pkg/dto"

type AuthPrincipal = dto.AuthPrincipal
type DataScope = dto.DataScope

const (
	UserTypePlatform       = "platform"
	UserTypeEnterprise     = "enterprise"
	UserTypeCustomer       = "customer"
	UserTypePartner        = "partner"
	UserTypeServiceAccount = "service_account"
)

const SystemTenantID int64 = 0
