package vectordb

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/qdrant/go-client/qdrant"

	"remotehelpdesk/internal/pkg/config"
)

type QdrantProvider struct {
	client *qdrant.Client
}

func NewQdrantProvider(cfg *config.QdrantVectorDBConfig) (*QdrantProvider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("vectordb config is nil")
	}

	host := cfg.Host
	if host == "" {
		host = "localhost"
	}

	port := cfg.GrpcPort
	if port <= 0 {
		port = 6334
	}

	client, err := qdrant.NewClient(&qdrant.Config{
		Host:                   host,
		Port:                   port,
		APIKey:                 cfg.APIKey,
		UseTLS:                 cfg.UseTLS,
		SkipCompatibilityCheck: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create qdrant client: %w", err)
	}

	return &QdrantProvider{client: client}, nil
}

func (p *QdrantProvider) Close() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}

func (p *QdrantProvider) CreateCollection(ctx context.Context, name string, dimension int) error {
	err := p.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: name,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     uint64(dimension),
			Distance: qdrant.Distance_Cosine,
		}),
	})
	if err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return fmt.Errorf("failed to create collection %s: %w", name, err)
		}
	}
	return p.EnsurePayloadIndexes(ctx, name)
}

func (p *QdrantProvider) DeleteCollection(ctx context.Context, name string) error {
	err := p.client.DeleteCollection(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to delete collection %s: %w", name, err)
	}
	return nil
}

func (p *QdrantProvider) GetCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	info, err := p.client.GetCollectionInfo(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection %s: %w", name, err)
	}

	status := info.GetStatus().String()
	pointCount := int(info.GetPointsCount())

	dimension := 0
	if info.Config != nil && info.Config.Params != nil {
		vectorsConfig := info.Config.Params.VectorsConfig
		if vectorsConfig != nil {
			params := vectorsConfig.GetParams()
			if params != nil {
				dimension = int(params.Size)
			}
		}
	}

	return &CollectionInfo{
		Name:       name,
		Dimension:  dimension,
		PointCount: pointCount,
		Status:     status,
	}, nil
}

func (p *QdrantProvider) ListCollections(ctx context.Context) ([]string, error) {
	collections, err := p.client.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	return collections, nil
}

func (p *QdrantProvider) UpsertVectors(ctx context.Context, collectionName string, vectors []Vector) error {
	if len(vectors) == 0 {
		return nil
	}

	points := make([]*qdrant.PointStruct, 0, len(vectors))
	for _, v := range vectors {
		payload, err := qdrant.TryValueMap(qdrantCompatiblePayload(v.Payload.ToMap()))
		if err != nil {
			return fmt.Errorf("failed to encode payload for vector %s: %w", v.ID, err)
		}
		points = append(points, &qdrant.PointStruct{
			Id:      qdrant.NewID(v.ID),
			Vectors: qdrant.NewVectors(v.Vector...),
			Payload: payload,
		})
	}

	_, err := p.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collectionName,
		Points:         points,
	})
	if err != nil {
		return fmt.Errorf("failed to upsert vectors to collection %s: %w", collectionName, err)
	}
	return nil
}

func qdrantCompatiblePayload(payload map[string]any) map[string]any {
	ret := make(map[string]any, len(payload))
	for key, value := range payload {
		switch items := value.(type) {
		case []int64:
			values := make([]any, len(items))
			for i, item := range items {
				values[i] = item
			}
			ret[key] = values
		case []string:
			values := make([]any, len(items))
			for i, item := range items {
				values[i] = item
			}
			ret[key] = values
		default:
			ret[key] = value
		}
	}
	return ret
}

func (p *QdrantProvider) DeleteVectors(ctx context.Context, collectionName string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	pointIDs := make([]*qdrant.PointId, 0, len(ids))
	for _, id := range ids {
		pointIDs = append(pointIDs, qdrant.NewID(id))
	}

	_, err := p.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: collectionName,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Points{
				Points: &qdrant.PointsIdsList{
					Ids: pointIDs,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete vectors from collection %s: %w", collectionName, err)
	}
	return nil
}

func (p *QdrantProvider) DeleteByFilter(ctx context.Context, collectionName string, filter *SearchFilter) error {
	compiled, err := p.buildFilter(filter)
	if err != nil {
		return err
	}
	if compiled == nil {
		return nil
	}
	_, err = p.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: collectionName,
		Points:         qdrant.NewPointsSelectorFilter(compiled),
	})
	if err != nil {
		return fmt.Errorf("failed to delete vectors by filter from collection %s: %w", collectionName, err)
	}
	return nil
}

func (p *QdrantProvider) Search(ctx context.Context, req *SearchRequest) ([]SearchResult, error) {
	if req == nil {
		return nil, fmt.Errorf("search request is required")
	}
	if err := validateRuntimeSearchFilter(req.Filter); err != nil {
		return nil, err
	}
	filter, err := p.buildFilter(req.Filter)
	if err != nil {
		return nil, err
	}

	results, err := p.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: req.CollectionName,
		Query:          qdrant.NewQuery(req.Vector...),
		Limit:          qdrant.PtrOf(uint64(req.TopK)),
		ScoreThreshold: &req.ScoreThreshold,
		Filter:         filter,
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search collection %s: %w", req.CollectionName, err)
	}

	searchResults := make([]SearchResult, 0, len(results))
	for _, r := range results {
		payload := make(map[string]any)
		if r.Payload != nil {
			for k, v := range r.Payload {
				payload[k] = p.extractPayloadValue(v)
			}
		}

		id := ""
		if r.Id != nil {
			id = r.Id.GetUuid()
		}

		searchResults = append(searchResults, SearchResult{
			ID:      id,
			Score:   r.Score,
			Payload: ChunkPayloadFromMap(payload),
		})
	}

	return searchResults, nil
}

func (p *QdrantProvider) buildFilter(filter *SearchFilter) (*qdrant.Filter, error) {
	if filter == nil {
		return nil, nil
	}
	if err := validateKnowledgeEntryFilter(filter); err != nil {
		return nil, err
	}

	must := make([]*qdrant.Condition, 0, 8)
	must = append(must, qdrant.NewMatchKeyword("tenant_id", tenantKey(filter.TenantID)))
	must = append(must, qdrant.NewMatchKeyword("review_status", strings.TrimSpace(filter.ReviewStatus)))
	must = append(must, qdrant.NewMatchKeywords("entry_key", filter.EntryKeys...))
	must = append(must, qdrant.NewMatchInts("revision_id", filter.RevisionIDs...))
	if len(filter.ScopeKeys) > 0 {
		scopeConditions := []*qdrant.Condition{qdrant.NewMatchKeywords("scope_keys", filter.ScopeKeys...)}
		if filter.AllowLegacyScope {
			scopeConditions = append(scopeConditions, qdrant.NewIsEmpty("scope_version"))
		}
		must = append(must, qdrant.NewFilterAsCondition(&qdrant.Filter{Should: scopeConditions}))
	}
	if len(filter.KnowledgeBaseIDs) > 0 {
		must = append(must, qdrant.NewMatchInts("knowledge_base_id", filter.KnowledgeBaseIDs...))
	}
	if len(filter.Languages) > 0 {
		must = append(must, qdrant.NewMatchKeywords("language", filter.Languages...))
	}
	if len(filter.Visibilities) > 0 {
		must = append(must, qdrant.NewMatchKeywords("visibility", filter.Visibilities...))
	}
	if filter.IndexGenerationID > 0 {
		must = append(must, qdrant.NewMatchInt("index_generation_id", filter.IndexGenerationID))
	}
	if len(filter.DocumentIDs) > 0 {
		must = append(must, qdrant.NewMatchInts("document_id", filter.DocumentIDs...))
	}
	if len(must) == 0 {
		return nil, nil
	}

	return &qdrant.Filter{Must: must}, nil
}

func (p *QdrantProvider) extractPayloadValue(v *qdrant.Value) interface{} {
	if v == nil {
		return nil
	}

	switch val := v.Kind.(type) {
	case *qdrant.Value_StringValue:
		return val.StringValue
	case *qdrant.Value_IntegerValue:
		return val.IntegerValue
	case *qdrant.Value_DoubleValue:
		return val.DoubleValue
	case *qdrant.Value_BoolValue:
		return val.BoolValue
	case *qdrant.Value_ListValue:
		list := make([]interface{}, 0, len(val.ListValue.Values))
		for _, item := range val.ListValue.Values {
			list = append(list, p.extractPayloadValue(item))
		}
		return list
	case *qdrant.Value_StructValue:
		m := make(map[string]interface{})
		for k, v := range val.StructValue.Fields {
			m[k] = p.extractPayloadValue(v)
		}
		return m
	default:
		return nil
	}
}

func validateKnowledgeEntryFilter(filter *SearchFilter) error {
	if filter == nil {
		return fmt.Errorf("search filter is required")
	}
	if filter.TenantID <= 0 {
		return fmt.Errorf("search filter tenant id is required")
	}
	if len(filter.EntryKeys) == 0 {
		return fmt.Errorf("search filter entry keys are required")
	}
	if len(filter.RevisionIDs) == 0 {
		return fmt.Errorf("search filter revision ids are required")
	}
	if strings.TrimSpace(filter.ReviewStatus) != "published" {
		return fmt.Errorf("runtime search filter review status must be published")
	}
	return nil
}

func validateRuntimeSearchFilter(filter *SearchFilter) error {
	if err := validateKnowledgeEntryFilter(filter); err != nil {
		return err
	}
	if len(filter.ScopeKeys) == 0 {
		return fmt.Errorf("search filter product scope keys are required")
	}
	return nil
}

func tenantKey(tenantID int64) string {
	if tenantID <= 0 {
		return ""
	}
	return strconv.FormatInt(tenantID, 10)
}

func (p *QdrantProvider) CollectionExists(ctx context.Context, collectionName string) (bool, error) {
	return p.client.CollectionExists(ctx, collectionName)
}

func (p *QdrantProvider) GetAliasTarget(ctx context.Context, aliasName string) (string, error) {
	aliases, err := p.client.ListAliases(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list aliases: %w", err)
	}
	for _, alias := range aliases {
		if alias.GetAliasName() == aliasName {
			return alias.GetCollectionName(), nil
		}
	}
	return "", nil
}

func (p *QdrantProvider) SwitchAlias(ctx context.Context, aliasName, collectionName string) error {
	aliasName = strings.TrimSpace(aliasName)
	collectionName = strings.TrimSpace(collectionName)
	if aliasName == "" || collectionName == "" {
		return fmt.Errorf("alias name and collection name are required")
	}
	currentTarget, err := p.GetAliasTarget(ctx, aliasName)
	if err != nil {
		return err
	}
	if currentTarget == collectionName {
		return nil
	}
	if currentTarget == "" {
		return p.client.CreateAlias(ctx, aliasName, collectionName)
	}
	return p.client.UpdateAliases(ctx, []*qdrant.AliasOperations{
		{
			Action: &qdrant.AliasOperations_DeleteAlias{
				DeleteAlias: &qdrant.DeleteAlias{AliasName: aliasName},
			},
		},
		{
			Action: &qdrant.AliasOperations_CreateAlias{
				CreateAlias: &qdrant.CreateAlias{
					CollectionName: collectionName,
					AliasName:      aliasName,
				},
			},
		},
	})
}

func (p *QdrantProvider) MigrateLegacyCollectionToAlias(ctx context.Context, legacyCollectionName, targetCollectionName string, payloadOverrides map[string]any) (int, error) {
	legacyCollectionName = strings.TrimSpace(legacyCollectionName)
	targetCollectionName = strings.TrimSpace(targetCollectionName)
	if legacyCollectionName == "" || targetCollectionName == "" {
		return 0, fmt.Errorf("legacy and target collection names are required")
	}
	if legacyCollectionName == targetCollectionName {
		return 0, fmt.Errorf("legacy and target collection names must differ")
	}

	currentTarget, err := p.GetAliasTarget(ctx, legacyCollectionName)
	if err != nil {
		return 0, err
	}
	if currentTarget != "" {
		if currentTarget == targetCollectionName {
			info, infoErr := p.GetCollection(ctx, targetCollectionName)
			if infoErr != nil {
				return 0, infoErr
			}
			return info.PointCount, nil
		}
		return 0, fmt.Errorf("alias %s already points to collection %s", legacyCollectionName, currentTarget)
	}

	legacyExists, err := p.CollectionExists(ctx, legacyCollectionName)
	if err != nil {
		return 0, err
	}
	if !legacyExists {
		if err := p.SwitchAlias(ctx, legacyCollectionName, targetCollectionName); err != nil {
			return 0, err
		}
		return 0, nil
	}

	legacyInfo, err := p.GetCollection(ctx, legacyCollectionName)
	if err != nil {
		return 0, err
	}
	targetInfo, err := p.GetCollection(ctx, targetCollectionName)
	if err != nil {
		return 0, err
	}
	if legacyInfo.Dimension != targetInfo.Dimension {
		return 0, fmt.Errorf("cannot migrate collection %s dimension %d to %s dimension %d", legacyCollectionName, legacyInfo.Dimension, targetCollectionName, targetInfo.Dimension)
	}

	migratedCount, err := p.copyCollectionPoints(ctx, legacyCollectionName, targetCollectionName, payloadOverrides)
	if err != nil {
		return 0, err
	}
	targetInfo, err = p.GetCollection(ctx, targetCollectionName)
	if err != nil {
		return 0, err
	}
	if targetInfo.PointCount < legacyInfo.PointCount {
		return 0, fmt.Errorf("legacy collection copy incomplete: source has %d points, target has %d", legacyInfo.PointCount, targetInfo.PointCount)
	}
	if migratedCount < legacyInfo.PointCount {
		return 0, fmt.Errorf("legacy collection scroll incomplete: source has %d points, copied %d", legacyInfo.PointCount, migratedCount)
	}

	if err := p.DeleteCollection(ctx, legacyCollectionName); err != nil {
		return 0, err
	}
	if err := p.SwitchAlias(ctx, legacyCollectionName, targetCollectionName); err != nil {
		return 0, fmt.Errorf("legacy collection data is preserved in %s but alias creation failed: %w", targetCollectionName, err)
	}
	return migratedCount, nil
}

func (p *QdrantProvider) copyCollectionPoints(ctx context.Context, sourceCollection, targetCollection string, payloadOverrides map[string]any) (int, error) {
	const batchSize = 256
	var offset *qdrant.PointId
	total := 0
	wait := true

	for {
		points, nextOffset, err := p.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
			CollectionName: sourceCollection,
			Offset:         offset,
			Limit:          qdrant.PtrOf(uint32(batchSize)),
			WithPayload:    qdrant.NewWithPayload(true),
			WithVectors:    qdrant.NewWithVectors(true),
		})
		if err != nil {
			return total, fmt.Errorf("failed to scroll legacy collection %s: %w", sourceCollection, err)
		}
		if len(points) == 0 {
			break
		}

		upserts := make([]*qdrant.PointStruct, 0, len(points))
		for _, point := range points {
			cloned, cloneErr := cloneQdrantPointForMigration(point, payloadOverrides)
			if cloneErr != nil {
				return total, cloneErr
			}
			upserts = append(upserts, cloned)
		}
		if _, err := p.client.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: targetCollection,
			Points:         upserts,
			Wait:           &wait,
		}); err != nil {
			return total, fmt.Errorf("failed to copy points to collection %s: %w", targetCollection, err)
		}
		total += len(upserts)
		if nextOffset == nil {
			break
		}
		offset = nextOffset
	}
	return total, nil
}

func cloneQdrantPointForMigration(point *qdrant.RetrievedPoint, payloadOverrides map[string]any) (*qdrant.PointStruct, error) {
	if point == nil || point.Id == nil {
		return nil, fmt.Errorf("legacy collection contains a point without an id")
	}
	vectorOutput := point.GetVectors().GetVector()
	if vectorOutput == nil {
		return nil, fmt.Errorf("legacy point %s does not contain a single dense vector", point.Id.String())
	}
	dense := vectorOutput.GetDense()
	vectorData := vectorOutput.GetData()
	if dense != nil {
		vectorData = dense.GetData()
	}
	if len(vectorData) == 0 {
		return nil, fmt.Errorf("legacy point %s contains an empty vector", point.Id.String())
	}

	payload := make(map[string]*qdrant.Value, len(point.Payload)+len(payloadOverrides))
	for key, value := range point.Payload {
		payload[key] = value
	}
	for key, value := range payloadOverrides {
		encoded, err := qdrant.NewValue(value)
		if err != nil {
			return nil, fmt.Errorf("failed to encode payload override %s: %w", key, err)
		}
		payload[key] = encoded
	}

	return &qdrant.PointStruct{
		Id:      point.Id,
		Vectors: qdrant.NewVectorsDense(vectorData),
		Payload: payload,
	}, nil
}

func (p *QdrantProvider) EnsurePayloadIndexes(ctx context.Context, collectionName string) error {
	requests := []*qdrant.CreateFieldIndexCollection{
		{
			CollectionName:   collectionName,
			FieldName:        "tenant_id",
			FieldType:        qdrant.PtrOf(qdrant.FieldType_FieldTypeKeyword),
			FieldIndexParams: qdrant.NewPayloadIndexParamsKeyword(&qdrant.KeywordIndexParams{IsTenant: qdrant.PtrOf(true)}),
			Wait:             qdrant.PtrOf(true),
		},
		{CollectionName: collectionName, FieldName: "entry_key", FieldType: qdrant.PtrOf(qdrant.FieldType_FieldTypeKeyword), Wait: qdrant.PtrOf(true)},
		{CollectionName: collectionName, FieldName: "scope_version", FieldType: qdrant.PtrOf(qdrant.FieldType_FieldTypeInteger), Wait: qdrant.PtrOf(true)},
		{CollectionName: collectionName, FieldName: "scope_keys", FieldType: qdrant.PtrOf(qdrant.FieldType_FieldTypeKeyword), Wait: qdrant.PtrOf(true)},
		{CollectionName: collectionName, FieldName: "product_ids", FieldType: qdrant.PtrOf(qdrant.FieldType_FieldTypeInteger), Wait: qdrant.PtrOf(true)},
		{CollectionName: collectionName, FieldName: "product_model_ids", FieldType: qdrant.PtrOf(qdrant.FieldType_FieldTypeInteger), Wait: qdrant.PtrOf(true)},
		{CollectionName: collectionName, FieldName: "knowledge_base_id", FieldType: qdrant.PtrOf(qdrant.FieldType_FieldTypeInteger), Wait: qdrant.PtrOf(true)},
		{CollectionName: collectionName, FieldName: "revision_id", FieldType: qdrant.PtrOf(qdrant.FieldType_FieldTypeInteger), Wait: qdrant.PtrOf(true)},
		{CollectionName: collectionName, FieldName: "review_status", FieldType: qdrant.PtrOf(qdrant.FieldType_FieldTypeKeyword), Wait: qdrant.PtrOf(true)},
		{CollectionName: collectionName, FieldName: "language", FieldType: qdrant.PtrOf(qdrant.FieldType_FieldTypeKeyword), Wait: qdrant.PtrOf(true)},
		{CollectionName: collectionName, FieldName: "visibility", FieldType: qdrant.PtrOf(qdrant.FieldType_FieldTypeKeyword), Wait: qdrant.PtrOf(true)},
		{CollectionName: collectionName, FieldName: "fault_codes", FieldType: qdrant.PtrOf(qdrant.FieldType_FieldTypeKeyword), Wait: qdrant.PtrOf(true)},
	}
	for _, request := range requests {
		if _, err := p.client.CreateFieldIndex(ctx, request); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "already exists") {
				continue
			}
			return fmt.Errorf("failed to create payload index %s for collection %s: %w", request.FieldName, collectionName, err)
		}
	}
	return nil
}
