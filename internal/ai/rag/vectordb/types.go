package vectordb

import "context"

type Vector struct {
	ID      string       `json:"id"`
	Vector  []float32    `json:"vector"`
	Payload ChunkPayload `json:"payload"`
}

type SearchRequest struct {
	CollectionName string        `json:"collectionName"`
	Vector         []float32     `json:"vector"`
	TopK           int           `json:"topK"`
	ScoreThreshold float32       `json:"scoreThreshold"`
	Filter         *SearchFilter `json:"filter,omitempty"`
}

type SearchFilter struct {
	TenantID          int64    `json:"tenantId,omitempty"`
	EntryKeys         []string `json:"entryKeys,omitempty"`
	KnowledgeBaseIDs  []int64  `json:"knowledgeBaseIds,omitempty"`
	Languages         []string `json:"languages,omitempty"`
	Visibilities      []string `json:"visibilities,omitempty"`
	ReviewStatus      string   `json:"reviewStatus,omitempty"`
	RevisionIDs       []int64  `json:"revisionIds,omitempty"`
	ScopeKeys         []string `json:"scopeKeys,omitempty"`
	AllowLegacyScope  bool     `json:"allowLegacyScope,omitempty"`
	IndexGenerationID int64    `json:"indexGenerationId,omitempty"`
	DocumentIDs       []int64  `json:"documentIds,omitempty"`
}

type SearchResult struct {
	ID      string       `json:"id"`
	Score   float32      `json:"score"`
	Payload ChunkPayload `json:"payload"`
}

type CollectionInfo struct {
	Name       string `json:"name"`
	Dimension  int    `json:"dimension"`
	PointCount int    `json:"pointCount"`
	Status     string `json:"status"`
}

type Provider interface {
	CreateCollection(ctx context.Context, name string, dimension int) error
	DeleteCollection(ctx context.Context, name string) error
	GetCollection(ctx context.Context, name string) (*CollectionInfo, error)
	ListCollections(ctx context.Context) ([]string, error)

	UpsertVectors(ctx context.Context, collectionName string, vectors []Vector) error
	DeleteVectors(ctx context.Context, collectionName string, ids []string) error

	Search(ctx context.Context, req *SearchRequest) ([]SearchResult, error)
	Close() error
}

type AliasProvider interface {
	GetAliasTarget(ctx context.Context, aliasName string) (string, error)
	SwitchAlias(ctx context.Context, aliasName, collectionName string) error
	CollectionExists(ctx context.Context, collectionName string) (bool, error)
}

// PayloadIndexEnsurer upgrades filter indexes on an existing collection.
// Providers without separate payload indexes do not need to implement it.
type PayloadIndexEnsurer interface {
	EnsurePayloadIndexes(ctx context.Context, collectionName string) error
}

// LegacyCollectionAliasMigrator converts a collection that occupies an alias
// name into an alias without dropping the existing points.
type LegacyCollectionAliasMigrator interface {
	MigrateLegacyCollectionToAlias(ctx context.Context, legacyCollectionName, targetCollectionName string, payloadOverrides map[string]any) (int, error)
}

type FilterDeleteProvider interface {
	DeleteByFilter(ctx context.Context, collectionName string, filter *SearchFilter) error
}
