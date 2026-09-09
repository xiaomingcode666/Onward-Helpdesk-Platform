package vectordb

import (
	"testing"

	"github.com/qdrant/go-client/qdrant"
)

func TestQdrantCompatiblePayloadEncodesTypedSlices(t *testing.T) {
	payload := ChunkPayload{
		ScopeVersion:    1,
		ScopeKeys:       []string{"tenant:7", "product:11"},
		ProductIDs:      []int64{11, 12},
		ProductModelIDs: []int64{21},
		FaultCodes:      []string{"E01", "E02"},
	}

	encoded, err := qdrant.TryValueMap(qdrantCompatiblePayload(payload.ToMap()))
	if err != nil {
		t.Fatalf("TryValueMap() error = %v", err)
	}
	if got := len(encoded["product_ids"].GetListValue().GetValues()); got != 2 {
		t.Fatalf("product_ids length = %d, want 2", got)
	}
	if got := encoded["product_ids"].GetListValue().GetValues()[0].GetIntegerValue(); got != 11 {
		t.Fatalf("product_ids[0] = %d, want 11", got)
	}
	if got := encoded["scope_keys"].GetListValue().GetValues()[1].GetStringValue(); got != "product:11" {
		t.Fatalf("scope_keys[1] = %q, want product:11", got)
	}
	if got := encoded["scope_version"].GetIntegerValue(); got != 1 {
		t.Fatalf("scope_version = %d, want 1", got)
	}
	if got := encoded["fault_codes"].GetListValue().GetValues()[1].GetStringValue(); got != "E02" {
		t.Fatalf("fault_codes[1] = %q, want E02", got)
	}
}

func TestCloneQdrantPointForMigrationPreservesDataAndOverridesGeneration(t *testing.T) {
	point := &qdrant.RetrievedPoint{
		Id: qdrant.NewID("295e96aa-6b74-5c5b-973c-a46ec163a38b"),
		Payload: map[string]*qdrant.Value{
			"entry_key":           qdrant.NewValueString("document:2"),
			"index_generation_id": qdrant.NewValueInt(0),
		},
		Vectors: &qdrant.VectorsOutput{VectorsOptions: &qdrant.VectorsOutput_Vector{
			Vector: &qdrant.VectorOutput{Vector: &qdrant.VectorOutput_Dense{
				Dense: &qdrant.DenseVector{Data: []float32{0.1, 0.2, 0.3}},
			}},
		}},
	}

	cloned, err := cloneQdrantPointForMigration(point, map[string]any{"index_generation_id": int64(7)})
	if err != nil {
		t.Fatalf("cloneQdrantPointForMigration() error = %v", err)
	}
	if got := cloned.Payload["entry_key"].GetStringValue(); got != "document:2" {
		t.Fatalf("entry_key = %q, want document:2", got)
	}
	if got := cloned.Payload["index_generation_id"].GetIntegerValue(); got != 7 {
		t.Fatalf("index_generation_id = %d, want 7", got)
	}
	if got := cloned.Vectors.GetVector().GetDense().GetData(); len(got) != 3 || got[2] != 0.3 {
		t.Fatalf("cloned vector = %#v, want preserved dense vector", got)
	}
}
