package services

import (
	"net/mail"
	"strings"
	"time"

	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/errorsx"
)

var MarketingDemoService = &marketingDemoService{}

type marketingDemoService struct{}

// Submit sends a public demo enquiry to the configured marketing inbox.
func (s *marketingDemoService) Submit(input request.DemoRequestRequest) error {
	// A filled honeypot indicates automated traffic. Return success without
	// delivering any mail, so the endpoint does not reveal the detection rule.
	if strings.TrimSpace(input.Website) != "" {
		return nil
	}

	company := strings.TrimSpace(input.Company)
	contactName := strings.TrimSpace(input.ContactName)
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if company == "" || contactName == "" || email == "" {
		return errorsx.InvalidParam("company, contact name and email are required")
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || !strings.EqualFold(parsed.Address, email) {
		return errorsx.InvalidParam("a valid email address is required")
	}

	recipient := strings.TrimSpace(config.CurrentOrDefault().Marketing.DemoRequestRecipient)
	if recipient == "" {
		return errorsx.BusinessError(90, "demo request delivery is not configured")
	}

	err = EmailNotificationService.SendEmail(0, recipient, "", "marketing_demo_request", map[string]interface{}{
		"Company":       company,
		"ContactName":   contactName,
		"Email":         email,
		"Mobile":        strings.TrimSpace(input.Mobile),
		"CountryRegion": strings.TrimSpace(input.CountryRegion),
		"Requirements":  strings.TrimSpace(input.Requirements),
		"SubmittedAt":   time.Now().Format("2006-01-02 15:04:05 MST"),
	})
	if err != nil {
		return errorsx.BusinessError(91, "demo request delivery is temporarily unavailable")
	}
	return nil
}
