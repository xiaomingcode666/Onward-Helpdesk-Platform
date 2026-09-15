package services

import (
	"errors"
	"strconv"
	"testing"

	"github.com/mlogclub/simple/web"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/openidentity"
)

func TestCustomerMissingOrForeignTicketDoesNotExpireLogin(t *testing.T) {
	db := setupCustomerPortalProfileTestDB(t)
	user := models.User{Username: "stale-link-customer", Status: enums.StatusOk}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	customer := models.Customer{Name: "Fixture customer", Status: enums.StatusOk}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatal(err)
	}
	external := openidentity.ExternalUser{ExternalSource: enums.ExternalSourceUser, ExternalID: strconv.FormatInt(user.ID, 10)}
	identity := models.CustomerIdentity{CustomerID: customer.ID, ExternalSource: external.ExternalSource, ExternalID: external.ExternalID, Status: enums.StatusOk}
	if err := db.Create(&identity).Error; err != nil {
		t.Fatal(err)
	}
	foreign := models.Ticket{TenantID: 999, CustomerID: customer.ID + 100, Title: "Must not be exposed", TicketNo: "FOREIGN-TICKET"}
	if err := db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{foreign.ID, 987654} {
		_, _, err := CustomerPortalService.resolveVisibleTicket(external, id)
		var business *web.CodeError
		if !errors.As(err, &business) || business.Code != errorsx.CodeAuthForbidden {
			t.Fatalf("resource error invalidates login: %#v", err)
		}
		if _, err := CustomerPortalService.resolveScope(external); err != nil {
			t.Fatal("valid identity lost after stale ticket", err)
		}
	}
}
