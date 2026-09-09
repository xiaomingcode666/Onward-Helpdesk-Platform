package builders

import (
	"encoding/json"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/services"
)

func BuildIndustrySolutionPack(item *models.IndustrySolutionPack) dto.IndustrySolutionPackDTO {
	if item == nil {
		return dto.IndustrySolutionPackDTO{}
	}
	return dto.IndustrySolutionPackDTO{
		ID: item.ID, PackCode: item.PackCode, Name: item.Name, IndustryCode: item.IndustryCode,
		ProductFamilyCode: item.ProductFamilyCode, Version: item.Version, SchemaVersion: item.SchemaVersion,
		Status: item.Status, DefaultLocale: item.DefaultLocale, Description: item.Description,
		ManifestHash: item.ManifestHash, Metadata: industrySolutionPackMetadata(item.MetadataJSON),
		PublishedAt: item.PublishedAt, PublishedByID: item.PublishedByID, PublishedByName: item.PublishedByName,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		CreateUserName: item.CreateUserName, UpdateUserName: item.UpdateUserName,
	}
}

func BuildIndustrySolutionPackResource(item *models.IndustrySolutionPackResource) dto.IndustrySolutionPackResourceDTO {
	if item == nil {
		return dto.IndustrySolutionPackResourceDTO{}
	}
	return dto.IndustrySolutionPackResourceDTO{
		ID: item.ID, PackID: item.PackID, ResourceType: item.ResourceType, ResourceKey: item.ResourceKey,
		Version: item.Version, Status: item.Status, Required: item.Required, ApplyOrder: item.ApplyOrder,
		Dependencies:    industrySolutionJSON(item.DependenciesJSON, `[]`),
		SourceReference: industrySolutionJSON(item.SourceReferenceJSON, `{}`),
		TargetSelector:  industrySolutionJSON(item.TargetSelectorJSON, `{}`),
		Payload:         industrySolutionJSON(item.PayloadJSON, `{}`), Checksum: item.Checksum,
		Metadata:  industrySolutionJSON(item.MetadataJSON, `{}`),
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		CreateUserName: item.CreateUserName, UpdateUserName: item.UpdateUserName,
	}
}

func BuildIndustrySolutionPackResources(items []models.IndustrySolutionPackResource) []dto.IndustrySolutionPackResourceDTO {
	result := make([]dto.IndustrySolutionPackResourceDTO, 0, len(items))
	for i := range items {
		result = append(result, BuildIndustrySolutionPackResource(&items[i]))
	}
	return result
}

func BuildIndustrySolutionPackManifest(pack *models.IndustrySolutionPack, resources []models.IndustrySolutionPackResource, reused bool) dto.IndustrySolutionPackManifestDTO {
	return dto.IndustrySolutionPackManifestDTO{
		Pack: BuildIndustrySolutionPack(pack), Resources: BuildIndustrySolutionPackResources(resources), Reused: reused,
	}
}

func BuildIndustrySolutionPackList(items []models.IndustrySolutionPack, total int64, page, pageSize int) dto.IndustrySolutionPackListDTO {
	result := make([]dto.IndustrySolutionPackDTO, 0, len(items))
	for i := range items {
		result = append(result, BuildIndustrySolutionPack(&items[i]))
	}
	return dto.IndustrySolutionPackListDTO{Items: result, Total: total, Page: page, PageSize: pageSize}
}

func BuildIndustrySolutionPackValidation(item services.IndustrySolutionPackValidationResult) dto.IndustrySolutionPackValidationDTO {
	issues := make([]dto.IndustrySolutionPackValidationIssueDTO, 0, len(item.Issues))
	for _, issue := range item.Issues {
		issues = append(issues, dto.IndustrySolutionPackValidationIssueDTO{
			Severity: issue.Severity, Code: issue.Code, Message: issue.Message, ResourceID: issue.ResourceID,
			ResourceType: issue.ResourceType, ResourceKey: issue.ResourceKey,
		})
	}
	return dto.IndustrySolutionPackValidationDTO{
		Valid: item.Valid, Pack: BuildIndustrySolutionPack(&item.Pack), Resources: BuildIndustrySolutionPackResources(item.Resources),
		CalculatedManifestHash: item.CalculatedManifestHash, Issues: issues,
	}
}

func BuildIndustrySolutionPackExecution(item *services.IndustrySolutionPackExecutionResult) dto.IndustrySolutionPackExecutionDTO {
	if item == nil {
		return dto.IndustrySolutionPackExecutionDTO{}
	}
	application := item.Application
	applicationDTO := dto.IndustrySolutionPackApplicationDTO{
		ID: application.ID, PackID: application.PackID, PackCode: application.PackCode, PackVersion: application.PackVersion,
		Version: application.Version, TargetProductID: application.TargetProductID,
		TargetProductModelID: application.TargetProductModelID,
		TargetContext:        industrySolutionJSON(application.TargetContextJSON, `{}`), Mode: application.Mode,
		ConflictStrategy: application.ConflictStrategy, IdempotencyKey: application.IdempotencyKey,
		RequestHash: application.RequestHash, Status: application.Status, ResourceCount: application.ResourceCount,
		SucceededCount: application.SucceededCount, SkippedCount: application.SkippedCount, FailedCount: application.FailedCount,
		Summary: industrySolutionJSON(application.SummaryJSON, `{}`), LastError: application.LastError,
		StartedAt: application.StartedAt, CompletedAt: application.CompletedAt,
		CreatedAt: application.CreatedAt, UpdatedAt: application.UpdatedAt,
		CreateUserName: application.CreateUserName, UpdateUserName: application.UpdateUserName,
	}
	items := make([]dto.IndustrySolutionPackApplicationItemDTO, 0, len(item.Items))
	for _, entry := range item.Items {
		items = append(items, dto.IndustrySolutionPackApplicationItemDTO{
			ID: entry.ID, ApplicationID: entry.ApplicationID, PackResourceID: entry.PackResourceID,
			ResourceType: entry.ResourceType, ResourceKey: entry.ResourceKey, Version: entry.Version,
			Status: entry.Status, Required: entry.Required, ApplyOrder: entry.ApplyOrder,
			Dependencies: industrySolutionJSON(entry.DependenciesJSON, `[]`), PlannedAction: entry.PlannedAction,
			ConflictReason:    entry.ConflictReason,
			ExistingReference: industrySolutionJSON(entry.ExistingReferenceJSON, `{}`),
			AppliedReference:  industrySolutionJSON(entry.AppliedReferenceJSON, `{}`),
			Result:            industrySolutionJSON(entry.ResultJSON, `{}`), AttemptCount: entry.AttemptCount,
			LastError: entry.LastError, StartedAt: entry.StartedAt, CompletedAt: entry.CompletedAt,
			CreatedAt: entry.CreatedAt, UpdatedAt: entry.UpdatedAt,
		})
	}
	return dto.IndustrySolutionPackExecutionDTO{
		Application: applicationDTO, Items: items, Validation: BuildIndustrySolutionPackValidation(item.Validation), Reused: item.Reused,
	}
}

func industrySolutionJSON(value, fallback string) json.RawMessage {
	if !json.Valid([]byte(value)) {
		value = fallback
	}
	return json.RawMessage(value)
}

func industrySolutionPackMetadata(value string) json.RawMessage {
	metadata := map[string]any{}
	if err := json.Unmarshal([]byte(value), &metadata); err != nil {
		return json.RawMessage(`{}`)
	}
	delete(metadata, "_rhd_import_requests")
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}
