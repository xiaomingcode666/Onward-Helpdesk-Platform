package services_test

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/openidentity"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestEnsureExternalCustomerWithoutNameUsesCustomerIdentity(t *testing.T) {
	db := setupCustomerServiceTestDB(t)

	var customerID int64
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		id, err := services.CustomerService.EnsureExternalCustomer(ctx, openidentity.ExternalUser{
			ExternalSource: enums.ExternalSourceUser,
			ExternalID:     "customer-without-name",
		})
		customerID = id
		return err
	}); err != nil {
		t.Fatalf("EnsureExternalCustomer() error = %v", err)
	}
	var customer models.Customer
	if err := db.First(&customer, customerID).Error; err != nil {
		t.Fatalf("load customer error = %v", err)
	}
	if !strings.HasPrefix(customer.Name, "客户") || strings.Contains(customer.Name, "访客") {
		t.Fatalf("unsupported visitor label leaked into customer name: %q", customer.Name)
	}
}

func TestEnsureExternalCustomerUpdatesNameFromExternalIdentity(t *testing.T) {
	db := setupCustomerServiceTestDB(t)

	var firstID int64
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		id, err := services.CustomerService.EnsureExternalCustomer(ctx, openidentity.ExternalUser{
			ExternalSource: enums.ExternalSourceUser,
			ExternalID:     "user-1",
			ExternalName:   "张三",
		})
		firstID = id
		return err
	}); err != nil {
		t.Fatalf("EnsureExternalCustomer() first error = %v", err)
	}

	conversation := &models.Conversation{
		CustomerID:   firstID,
		CustomerName: "张三",
		Status:       enums.IMConversationStatusActive,
		AuditFields:  models.AuditFields{CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatalf("create conversation error = %v", err)
	}

	var secondID int64
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		id, err := services.CustomerService.EnsureExternalCustomer(ctx, openidentity.ExternalUser{
			ExternalSource: enums.ExternalSourceUser,
			ExternalID:     "user-1",
			ExternalName:   "李四",
		})
		secondID = id
		return err
	}); err != nil {
		t.Fatalf("EnsureExternalCustomer() second error = %v", err)
	}
	if secondID != firstID {
		t.Fatalf("expected same customer id, got %d and %d", firstID, secondID)
	}

	customer := services.CustomerService.Get(firstID)
	if customer == nil {
		t.Fatalf("expected customer to exist")
	}
	if customer.Name != "李四" {
		t.Fatalf("expected customer name updated, got %q", customer.Name)
	}

	var updatedConversation models.Conversation
	if err := db.First(&updatedConversation, conversation.ID).Error; err != nil {
		t.Fatalf("get conversation error = %v", err)
	}
	if updatedConversation.CustomerName != "李四" {
		t.Fatalf("expected conversation customer name updated, got %q", updatedConversation.CustomerName)
	}
}

func TestCustomerServiceProfilePersistsAndNormalizes(t *testing.T) {
	setupCustomerServiceTestDB(t)
	operator := &dto.AuthPrincipal{UserID: 1, Username: "fixture"}
	customer, err := services.CustomerService.CreateCustomer(request.CreateCustomerRequest{
		Name:           "Enhanced Customer",
		ServiceProfile: "enhanced",
	}, operator)
	if err != nil {
		t.Fatalf("CreateCustomer() error = %v", err)
	}
	if customer.ServiceProfile != "enhanced" {
		t.Fatalf("service profile = %q, want enhanced", customer.ServiceProfile)
	}

	if err := services.CustomerService.UpdateCustomer(request.UpdateCustomerRequest{
		ID: customer.ID,
		CreateCustomerRequest: request.CreateCustomerRequest{
			Name:           customer.Name,
			ServiceProfile: "unsupported",
		},
	}, operator); err != nil {
		t.Fatalf("UpdateCustomer() error = %v", err)
	}
	updated := services.CustomerService.Get(customer.ID)
	if updated == nil || updated.ServiceProfile != "standard" {
		t.Fatalf("service profile = %+v, want standard fallback", updated)
	}
}

func setupCustomerServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "t_",
			SingularTable: true,
		},
	})
	if err != nil {
		t.Fatalf("open sqlite error = %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&models.Customer{}, &models.CustomerIdentity{}, &models.Conversation{}); err != nil {
		t.Fatalf("auto migrate error = %v", err)
	}
	sqls.SetDB(db)
	return db
}
