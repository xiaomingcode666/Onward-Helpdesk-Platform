package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"gorm.io/gorm"
)

const industrySolutionImportLedgerKey = "_rhd_import_requests"

type IndustrySolutionPackManifestResult struct {
	Pack      models.IndustrySolutionPack
	Resources []models.IndustrySolutionPackResource
	Reused    bool
}

type IndustrySolutionPackListResult struct {
	Items    []models.IndustrySolutionPack
	Total    int64
	Page     int
	PageSize int
}

func (s *industrySolutionPackService) ListPacks(
	ctx context.Context,
	tenantID int64,
	status, industryCode string,
	page, pageSize int,
	operator *dto.AuthPrincipal,
) (*IndustrySolutionPackListResult, error) {
	if err := s.authorize(tenantID, operator); err != nil {
		return nil, err
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	status = strings.TrimSpace(status)
	if status != "" && !isIndustrySolutionPackStatus(status) {
		return nil, errorsx.InvalidParam("invalid solution pack status")
	}
	items, total, err := repositories.IndustrySolutionPackRepository.ListPacks(s.db.WithContext(ctx), tenantID, repositories.IndustrySolutionPackListFilter{
		Status: status, IndustryCode: strings.TrimSpace(industryCode), Page: page, PageSize: pageSize,
	})
	if err != nil {
		return nil, err
	}
	return &IndustrySolutionPackListResult{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *industrySolutionPackService) ImportDraftManifest(
	ctx context.Context,
	tenantID int64,
	req dto.IndustrySolutionPackManifestRequest,
	operator *dto.AuthPrincipal,
) (*IndustrySolutionPackManifestResult, error) {
	return s.saveDraftManifest(ctx, tenantID, 0, req, operator)
}

func (s *industrySolutionPackService) UpdateDraftManifest(
	ctx context.Context,
	tenantID, packID int64,
	req dto.IndustrySolutionPackManifestRequest,
	operator *dto.AuthPrincipal,
) (*IndustrySolutionPackManifestResult, error) {
	if packID <= 0 {
		return nil, errorsx.InvalidParam("packId is required")
	}
	return s.saveDraftManifest(ctx, tenantID, packID, req, operator)
}

func (s *industrySolutionPackService) saveDraftManifest(
	ctx context.Context,
	tenantID, expectedPackID int64,
	req dto.IndustrySolutionPackManifestRequest,
	operator *dto.AuthPrincipal,
) (*IndustrySolutionPackManifestResult, error) {
	if err := s.authorize(tenantID, operator); err != nil {
		return nil, err
	}
	pack, resources, userMetadata, requestHash, err := s.buildDraftManifest(tenantID, req, operator)
	if err != nil {
		return nil, err
	}

	var result IndustrySolutionPackManifestResult
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing, findErr := repositories.IndustrySolutionPackRepository.FindPackByCodeVersion(tx, tenantID, pack.PackCode, pack.Version)
		if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		if expectedPackID > 0 {
			byID, getErr := repositories.IndustrySolutionPackRepository.LockPack(tx, tenantID, expectedPackID)
			if getErr != nil {
				if errors.Is(getErr, gorm.ErrRecordNotFound) {
					return errorsx.InvalidParam("industry solution pack not found")
				}
				return getErr
			}
			if existing == nil || existing.ID != byID.ID {
				return errorsx.InvalidParam("draft pack code and version cannot be changed")
			}
			existing = byID
		}

		if existing == nil {
			if expectedPackID > 0 {
				return errorsx.InvalidParam("industry solution pack not found")
			}
			pack.MetadataJSON, err = encodeIndustrySolutionImportMetadata(userMetadata, map[string]string{req.IdempotencyKey: requestHash})
			if err != nil {
				return err
			}
			if err := repositories.IndustrySolutionPackRepository.CreatePack(tx, &pack); err != nil {
				return err
			}
			for i := range resources {
				resources[i].PackID = pack.ID
			}
			if err := repositories.IndustrySolutionPackRepository.ReplaceDraftResources(tx, tenantID, pack.ID, resources); err != nil {
				return err
			}
			result = IndustrySolutionPackManifestResult{Pack: pack, Resources: resources}
			return nil
		}

		currentResources, err := repositories.IndustrySolutionPackRepository.ListResources(tx, tenantID, existing.ID)
		if err != nil {
			return err
		}
		currentMetadata, ledger, err := decodeIndustrySolutionImportMetadata(existing.MetadataJSON)
		if err != nil {
			return err
		}
		if previousHash, exists := ledger[req.IdempotencyKey]; exists && previousHash != requestHash {
			return errorsx.InvalidParam("idempotency key was already used for a different draft manifest")
		}
		currentHash := industrySolutionDraftManifestRequestHash(*existing, currentResources, currentMetadata)
		if currentHash == requestHash {
			result = IndustrySolutionPackManifestResult{Pack: *existing, Resources: currentResources, Reused: true}
			return nil
		}
		if existing.Status != models.IndustrySolutionPackStatusDraft {
			return errorsx.InvalidParam("published industry solution packs are immutable; import a new version")
		}
		applicationCount, err := repositories.IndustrySolutionPackRepository.CountApplicationsForPack(tx, tenantID, existing.ID)
		if err != nil {
			return err
		}
		if applicationCount > 0 {
			return errorsx.InvalidParam("draft pack already has application audit records; import a new version instead of overwriting it")
		}
		ledger[req.IdempotencyKey] = requestHash
		storedMetadata, err := encodeIndustrySolutionImportMetadata(userMetadata, ledger)
		if err != nil {
			return err
		}
		now := s.now().UTC()
		if err := repositories.IndustrySolutionPackRepository.UpdateDraftPack(tx, tenantID, existing.ID, map[string]any{
			"name": pack.Name, "industry_code": pack.IndustryCode, "product_family_code": pack.ProductFamilyCode,
			"schema_version": pack.SchemaVersion, "default_locale": pack.DefaultLocale, "description": pack.Description,
			"manifest_hash": pack.ManifestHash, "metadata_json": storedMetadata,
			"update_user_id": operator.UserID, "update_user_name": industrySolutionOperatorName(operator), "updated_at": now,
		}); err != nil {
			return err
		}
		for i := range resources {
			resources[i].PackID = existing.ID
			resources[i].AuditFields = industrySolutionAuditFields(operator, now)
		}
		if err := repositories.IndustrySolutionPackRepository.ReplaceDraftResources(tx, tenantID, existing.ID, resources); err != nil {
			return err
		}
		updated, err := repositories.IndustrySolutionPackRepository.GetPack(tx, tenantID, existing.ID)
		if err != nil {
			return err
		}
		result = IndustrySolutionPackManifestResult{Pack: *updated, Resources: resources}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *industrySolutionPackService) Publish(
	ctx context.Context,
	tenantID, packID int64,
	operator *dto.AuthPrincipal,
) (*IndustrySolutionPackManifestResult, error) {
	if err := s.authorize(tenantID, operator); err != nil {
		return nil, err
	}
	if packID <= 0 {
		return nil, errorsx.InvalidParam("packId is required")
	}
	var reused bool
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		pack, err := repositories.IndustrySolutionPackRepository.LockPack(tx, tenantID, packID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errorsx.InvalidParam("industry solution pack not found")
			}
			return err
		}
		if pack.Status == models.IndustrySolutionPackStatusPublished {
			reused = true
			return nil
		}
		if pack.Status != models.IndustrySolutionPackStatusDraft {
			return errorsx.InvalidParam("only draft industry solution packs can be published")
		}
		resources, err := repositories.IndustrySolutionPackRepository.ListResources(tx, tenantID, packID)
		if err != nil {
			return err
		}
		candidatePack := *pack
		candidatePack.Status = models.IndustrySolutionPackStatusPublished
		candidateResources := append([]models.IndustrySolutionPackResource(nil), resources...)
		for i := range candidateResources {
			candidateResources[i].Status = models.IndustrySolutionResourceStatusReady
		}
		manifestHash, err := ComputeIndustrySolutionPackManifestHash(candidateResources)
		if err != nil {
			return err
		}
		candidatePack.ManifestHash = manifestHash
		validation := s.validatePack(candidatePack, candidateResources)
		if !validation.Valid {
			return errorsx.InvalidParam(firstIndustrySolutionValidationError(validation))
		}
		now := s.now().UTC()
		return repositories.IndustrySolutionPackRepository.PublishPack(
			tx, tenantID, packID, manifestHash, operator.UserID, industrySolutionOperatorName(operator), now,
		)
	})
	if err != nil {
		return nil, err
	}
	pack, resources, err := s.loadPack(ctx, tenantID, packID)
	if err != nil {
		return nil, err
	}
	return &IndustrySolutionPackManifestResult{Pack: *pack, Resources: resources, Reused: reused}, nil
}

func (s *industrySolutionPackService) GetApplicationDetail(
	ctx context.Context,
	tenantID, applicationID int64,
	operator *dto.AuthPrincipal,
) (*IndustrySolutionPackExecutionResult, error) {
	if err := s.authorize(tenantID, operator); err != nil {
		return nil, err
	}
	if applicationID <= 0 {
		return nil, errorsx.InvalidParam("applicationId is required")
	}
	application, err := repositories.IndustrySolutionPackRepository.GetApplication(s.db.WithContext(ctx), tenantID, applicationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorsx.InvalidParam("industry solution pack application not found")
		}
		return nil, err
	}
	pack, resources, err := s.loadPack(ctx, tenantID, application.PackID)
	if err != nil {
		return nil, err
	}
	return s.loadExecutionResult(ctx, tenantID, applicationID, s.validatePack(*pack, resources), false)
}

func (s *industrySolutionPackService) buildDraftManifest(
	tenantID int64,
	req dto.IndustrySolutionPackManifestRequest,
	operator *dto.AuthPrincipal,
) (models.IndustrySolutionPack, []models.IndustrySolutionPackResource, string, string, error) {
	req.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	if !industrySolutionKeyPattern.MatchString(req.IdempotencyKey) {
		return models.IndustrySolutionPack{}, nil, "", "", errorsx.InvalidParam("idempotency_key must be a stable identifier up to 128 characters")
	}
	metadata, err := canonicalJSONObject(string(req.Metadata), "{}")
	if err != nil {
		return models.IndustrySolutionPack{}, nil, "", "", errorsx.InvalidParam("metadata must be a JSON object")
	}
	var metadataObject map[string]any
	if err := json.Unmarshal([]byte(metadata), &metadataObject); err != nil {
		return models.IndustrySolutionPack{}, nil, "", "", err
	}
	if _, reserved := metadataObject[industrySolutionImportLedgerKey]; reserved {
		return models.IndustrySolutionPack{}, nil, "", "", errorsx.InvalidParam(industrySolutionImportLedgerKey + " is reserved for server audit metadata")
	}
	now := s.now().UTC()
	pack := models.IndustrySolutionPack{
		TenantID: tenantID, PackCode: strings.TrimSpace(req.PackCode), Name: strings.TrimSpace(req.Name),
		IndustryCode: strings.TrimSpace(req.IndustryCode), ProductFamilyCode: strings.TrimSpace(req.ProductFamilyCode),
		Version: strings.TrimSpace(req.Version), SchemaVersion: req.SchemaVersion,
		Status: models.IndustrySolutionPackStatusDraft, DefaultLocale: strings.TrimSpace(req.DefaultLocale),
		Description: strings.TrimSpace(req.Description), MetadataJSON: metadata,
		AuditFields: industrySolutionAuditFields(operator, now),
	}
	if pack.SchemaVersion == 0 {
		pack.SchemaVersion = 1
	}
	if pack.DefaultLocale == "" {
		pack.DefaultLocale = "en"
	}
	resources := make([]models.IndustrySolutionPackResource, 0, len(req.Resources))
	for _, input := range req.Resources {
		required := true
		if input.Required != nil {
			required = *input.Required
		}
		dependencies, err := json.Marshal(input.Dependencies)
		if err != nil {
			return models.IndustrySolutionPack{}, nil, "", "", err
		}
		resource := models.IndustrySolutionPackResource{
			TenantID: tenantID, ResourceType: strings.TrimSpace(input.ResourceType), ResourceKey: strings.TrimSpace(input.ResourceKey),
			Version: strings.TrimSpace(input.Version), Status: models.IndustrySolutionResourceStatusDraft,
			Required: required, ApplyOrder: input.ApplyOrder, DependenciesJSON: string(dependencies),
			AuditFields: industrySolutionAuditFields(operator, now),
		}
		if resource.SourceReferenceJSON, err = canonicalJSONObject(string(input.SourceReference), "{}"); err != nil {
			return models.IndustrySolutionPack{}, nil, "", "", errorsx.InvalidParam("source_reference must be a JSON object")
		}
		if resource.TargetSelectorJSON, err = canonicalJSONObject(string(input.TargetSelector), "{}"); err != nil {
			return models.IndustrySolutionPack{}, nil, "", "", errorsx.InvalidParam("target_selector must be a JSON object")
		}
		if resource.PayloadJSON, err = canonicalJSONObject(string(input.Payload), "{}"); err != nil {
			return models.IndustrySolutionPack{}, nil, "", "", errorsx.InvalidParam("payload must be a JSON object")
		}
		if resource.MetadataJSON, err = canonicalJSONObject(string(input.Metadata), "{}"); err != nil {
			return models.IndustrySolutionPack{}, nil, "", "", errorsx.InvalidParam("resource metadata must be a JSON object")
		}
		resource.Checksum, err = ComputeIndustrySolutionResourceChecksum(resource)
		if err != nil {
			return models.IndustrySolutionPack{}, nil, "", "", err
		}
		resources = append(resources, resource)
	}
	pack.ManifestHash, err = ComputeIndustrySolutionPackManifestHash(resources)
	if err != nil {
		return models.IndustrySolutionPack{}, nil, "", "", err
	}
	validation := s.validatePack(pack, resources)
	if !validation.Valid {
		return models.IndustrySolutionPack{}, nil, "", "", errorsx.InvalidParam(firstIndustrySolutionValidationError(validation))
	}
	requestHash := industrySolutionDraftManifestRequestHash(pack, resources, metadata)
	return pack, resources, metadata, requestHash, nil
}

func industrySolutionDraftManifestRequestHash(pack models.IndustrySolutionPack, resources []models.IndustrySolutionPackResource, userMetadata string) string {
	input := struct {
		PackCode          string   `json:"packCode"`
		Name              string   `json:"name"`
		IndustryCode      string   `json:"industryCode"`
		ProductFamilyCode string   `json:"productFamilyCode"`
		Version           string   `json:"version"`
		SchemaVersion     int      `json:"schemaVersion"`
		DefaultLocale     string   `json:"defaultLocale"`
		Description       string   `json:"description"`
		Metadata          string   `json:"metadata"`
		ManifestHash      string   `json:"manifestHash"`
		ResourceChecksums []string `json:"resourceChecksums"`
	}{
		PackCode: pack.PackCode, Name: pack.Name, IndustryCode: pack.IndustryCode,
		ProductFamilyCode: pack.ProductFamilyCode, Version: pack.Version, SchemaVersion: pack.SchemaVersion,
		DefaultLocale: pack.DefaultLocale, Description: pack.Description, Metadata: userMetadata, ManifestHash: pack.ManifestHash,
	}
	for _, resource := range resources {
		input.ResourceChecksums = append(input.ResourceChecksums, resource.ResourceType+":"+resource.ResourceKey+":"+resource.Checksum)
	}
	encoded, _ := json.Marshal(input)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func encodeIndustrySolutionImportMetadata(userMetadata string, ledger map[string]string) (string, error) {
	metadata := map[string]any{}
	if err := json.Unmarshal([]byte(userMetadata), &metadata); err != nil {
		return "", err
	}
	metadata[industrySolutionImportLedgerKey] = ledger
	encoded, err := json.Marshal(metadata)
	return string(encoded), err
}

func decodeIndustrySolutionImportMetadata(value string) (string, map[string]string, error) {
	metadata := map[string]any{}
	if err := json.Unmarshal([]byte(firstNonBlank(strings.TrimSpace(value), "{}")), &metadata); err != nil {
		return "", nil, fmt.Errorf("invalid pack audit metadata: %w", err)
	}
	ledger := map[string]string{}
	if raw, exists := metadata[industrySolutionImportLedgerKey]; exists {
		encoded, _ := json.Marshal(raw)
		if err := json.Unmarshal(encoded, &ledger); err != nil {
			return "", nil, fmt.Errorf("invalid pack import audit ledger: %w", err)
		}
		delete(metadata, industrySolutionImportLedgerKey)
	}
	encoded, err := json.Marshal(metadata)
	return string(encoded), ledger, err
}

func industrySolutionOperatorName(operator *dto.AuthPrincipal) string {
	if operator == nil {
		return "system"
	}
	return firstNonBlank(strings.TrimSpace(operator.Username), "system")
}
