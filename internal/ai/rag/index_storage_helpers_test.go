package rag

import (
	"context"
	"testing"

	"remotehelpdesk/internal/ai/rag/vectordb"
)

type collectionDimensionTestProvider struct {
	info        *vectordb.CollectionInfo
	getErr      error
	createCalls int
}

func (p *collectionDimensionTestProvider) CreateCollection(_ context.Context, name string, dimension int) error {
	p.createCalls++
	p.info = &vectordb.CollectionInfo{Name: name, Dimension: dimension}
	p.getErr = nil
	return nil
}

func (p *collectionDimensionTestProvider) DeleteCollection(context.Context, string) error { return nil }
func (p *collectionDimensionTestProvider) GetCollection(context.Context, string) (*vectordb.CollectionInfo, error) {
	return p.info, p.getErr
}
func (p *collectionDimensionTestProvider) ListCollections(context.Context) ([]string, error) {
	return nil, nil
}
func (p *collectionDimensionTestProvider) UpsertVectors(context.Context, string, []vectordb.Vector) error {
	return nil
}
func (p *collectionDimensionTestProvider) DeleteVectors(context.Context, string, []string) error {
	return nil
}
func (p *collectionDimensionTestProvider) Search(context.Context, *vectordb.SearchRequest) ([]vectordb.SearchResult, error) {
	return nil, nil
}
func (p *collectionDimensionTestProvider) Close() error { return nil }

func TestEnsureCollectionRejectsExistingDimensionMismatch(t *testing.T) {
	provider := &collectionDimensionTestProvider{
		info: &vectordb.CollectionInfo{Name: "knowledge", Dimension: 8},
	}

	err := Index.ensureCollection(context.Background(), provider, "knowledge", 1024)
	if err == nil {
		t.Fatal("ensureCollection() error = nil, want dimension mismatch")
	}
	if provider.createCalls != 0 {
		t.Fatalf("create calls = %d, want 0", provider.createCalls)
	}
}

func TestEnsureCollectionCreatesAndVerifiesDimension(t *testing.T) {
	provider := &collectionDimensionTestProvider{getErr: context.Canceled}

	if err := Index.ensureCollection(context.Background(), provider, "knowledge", 1024); err != nil {
		t.Fatalf("ensureCollection() error = %v", err)
	}
	if provider.createCalls != 1 || provider.info == nil || provider.info.Dimension != 1024 {
		t.Fatalf("provider state = %#v, create calls = %d", provider.info, provider.createCalls)
	}
}
