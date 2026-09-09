package services

import (
	"context"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

func TestProductResourceProvisioningJobRetriesWhenSub2APIUnavailable(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	if err := db.AutoMigrate(
		&models.ProductAIUsageCredential{},
		&models.ProductResourceProvisioningJob{},
		&models.TenantIntegrationConfig{},
		&models.SystemConfig{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	product, err := ProductService.CreateProduct(request.CreateProductRequest{
		TenantID:      fixture.Tenant.ID,
		Code:          "HP-RESOURCE-RETRY",
		Name:          "Hydraulic Pump Resource Retry",
		Category:      "hydraulic",
		DefaultLocale: "en-US",
	}, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateProduct() error = %v", err)
	}

	if _, err := ProductResourceService.EnqueueProductResourceProvisioning(context.Background(), fixture.Tenant.ID, product.ID, 2, fixture.Operator); err != nil {
		t.Fatalf("EnqueueProductResourceProvisioning() error = %v", err)
	}
	job := repositories.ProductResourceProvisioningJobRepository.LatestByProduct(sqls.DB(), fixture.Tenant.ID, product.ID)
	if job == nil {
		t.Fatalf("expected provisioning job")
	}

	if err := ProductResourceService.ProcessProvisioningJob(context.Background(), job); err == nil {
		t.Fatalf("ProcessProvisioningJob() error = nil, want Sub2API provisioning failure")
	}

	updatedJob := repositories.ProductResourceProvisioningJobRepository.Get(sqls.DB(), job.ID)
	if updatedJob == nil {
		t.Fatalf("expected updated provisioning job")
	}
	if updatedJob.Status != "waiting_retry" || updatedJob.AIKeyStatus != productResourceStatusFailed {
		t.Fatalf("job should wait for retry after ai key failure: %+v", updatedJob)
	}
	if updatedJob.RetryCount != 1 || updatedJob.NextAttemptAt == nil || updatedJob.ErrorSummary == "" {
		t.Fatalf("job retry metadata was not recorded: %+v", updatedJob)
	}

	credential := repositories.ProductAIUsageCredentialRepository.GetByProduct(sqls.DB(), fixture.Tenant.ID, product.ID)
	if credential == nil {
		t.Fatalf("expected product ai credential shell")
	}
	if credential.ProvisionStatus != productResourceStatusFailed || credential.ProvisionError == "" {
		t.Fatalf("credential should expose failed provisioning state: %+v", credential)
	}
}
