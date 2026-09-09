package eval

import (
	"context"
	"testing"
	"time"

	"remotehelpdesk/internal/ai/rag"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/services"
)

func TestEvaluatorEvaluateRetrievalOnly(t *testing.T) {
	evaluator := &Evaluator{
		Retrieve: func(_ context.Context, req services.KnowledgeRetrievalRequest) (*services.KnowledgeRetrievalResult, error) {
			switch req.Query {
			case "startup":
				return &services.KnowledgeRetrievalResult{
					RerankedHits: []rag.RetrieveResult{
						{DocumentID: 101, Score: 0.91},
						{FaqID: 205, Score: 0.82},
					},
					ContextHits: []rag.RetrieveResult{
						{DocumentID: 101, Score: 0.91},
					},
					Citations: []dto.KnowledgeCitation{
						{DocumentID: 101, Score: 0.91},
					},
					TopScore: 0.91,
					Strategy: services.KnowledgeRetrievalStrategyTrace{
						RequestedMode: "auto",
						AppliedMode:   "dense_only",
					},
				}, nil
			case "unsafe":
				return &services.KnowledgeRetrievalResult{
					RerankedHits: []rag.RetrieveResult{
						{FaqID: 205, Score: 0.88},
						{DocumentID: 107, Score: 0.77},
					},
					ContextHits: []rag.RetrieveResult{
						{FaqID: 205, Score: 0.88},
					},
					Citations: []dto.KnowledgeCitation{
						{FaqID: 205, Score: 0.88},
					},
					TopScore: 0.88,
					Strategy: services.KnowledgeRetrievalStrategyTrace{
						RequestedMode: "auto",
						AppliedMode:   "hybrid_rrf",
					},
				}, nil
			default:
				return &services.KnowledgeRetrievalResult{}, nil
			}
		},
		Now: func() time.Time {
			return time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
		},
	}

	suite := &Suite{
		Name:    "test_suite",
		Version: 1,
		Cases: []Case{
			{
				ID:   "c1",
				Name: "startup",
				Scope: Scope{
					TenantRef:       "tenant_a",
					ProductRef:      "product_a",
					ProductModelRef: "model_a",
				},
				Input: Input{Query: "startup"},
				RetrievalExpectation: RetrievalExpectation{
					ExpectedEntryRefs:   []string{"manual.startup"},
					MinExpectedHitsAt5:  1,
					MinExpectedHitsAt10: 1,
				},
				AnswerExpectation: AnswerExpectation{
					ShouldAnswer:     true,
					CitationRequired: boolPtr(true),
				},
			},
			{
				ID:   "c2",
				Name: "unsafe",
				Scope: Scope{
					TenantRef:       "tenant_a",
					ProductRef:      "product_a",
					ProductModelRef: "model_a",
				},
				Input: Input{Query: "unsafe"},
				RetrievalExpectation: RetrievalExpectation{
					ExpectedEntryRefs:  []string{"faq.warning"},
					ForbiddenEntryRefs: []string{"internal.bypass"},
					MinExpectedHitsAt5: 1,
				},
				AnswerExpectation: AnswerExpectation{
					ShouldAnswer:     true,
					CitationRequired: boolPtr(true),
				},
			},
		},
	}
	refs := &Refs{
		TenantRefs:       map[string]int64{"tenant_a": 1},
		ProductRefs:      map[string]int64{"product_a": 10},
		ProductModelRefs: map[string]int64{"model_a": 20},
		EntryRefs: map[string]EntryRef{
			"manual.startup":  {EntryKey: "document:101"},
			"faq.warning":     {EntryKey: "faq:205"},
			"internal.bypass": {EntryKey: "document:107"},
		},
	}

	report, err := evaluator.Evaluate(context.Background(), suite, refs, RunOptions{})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if report.Summary.EnabledCases != 2 {
		t.Fatalf("EnabledCases = %d, want 2", report.Summary.EnabledCases)
	}
	if report.Cases[0].Passed != true {
		t.Fatalf("case1 passed = %v, want true", report.Cases[0].Passed)
	}
	if report.Cases[1].Passed != false {
		t.Fatalf("case2 passed = %v, want false", report.Cases[1].Passed)
	}
	if report.Summary.ForbiddenHitCount != 1 {
		t.Fatalf("ForbiddenHitCount = %d, want 1", report.Summary.ForbiddenHitCount)
	}
	if report.Summary.CitationCoverage != 1 {
		t.Fatalf("CitationCoverage = %v, want 1", report.Summary.CitationCoverage)
	}
}

func TestEvaluatorEvaluateWithAnswer(t *testing.T) {
	evaluator := &Evaluator{
		Retrieve: func(_ context.Context, req services.KnowledgeRetrievalRequest) (*services.KnowledgeRetrievalResult, error) {
			return &services.KnowledgeRetrievalResult{
				RerankedHits: []rag.RetrieveResult{
					{FaqID: 206, Score: 0.92},
				},
				ContextHits: []rag.RetrieveResult{
					{FaqID: 206, Score: 0.92},
				},
				Citations: []dto.KnowledgeCitation{
					{FaqID: 206, Score: 0.92},
				},
				ContextText: "Please contact support for further diagnosis.",
				TopScore:    0.92,
			}, nil
		},
		GenerateAnswer: func(_ context.Context, item Case, retrieval *services.KnowledgeRetrievalResult) (*AnswerResult, error) {
			return &AnswerResult{
				Generated: true,
				Content:   "Please contact support for further diagnosis.",
			}, nil
		},
		Now: func() time.Time {
			return time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
		},
	}

	suite := &Suite{
		Name:    "answer_suite",
		Version: 1,
		Cases: []Case{
			{
				ID:   "c3",
				Name: "handoff",
				Scope: Scope{
					TenantRef:       "tenant_a",
					ProductRef:      "product_a",
					ProductModelRef: "model_a",
				},
				Input: Input{Query: "storm noise"},
				RetrievalExpectation: RetrievalExpectation{
					ExpectedEntryRefs:  []string{"faq.contact_support"},
					MinExpectedHitsAt5: 1,
				},
				AnswerExpectation: AnswerExpectation{
					ShouldAnswer:     false,
					ExpectHandoff:    true,
					CitationRequired: boolPtr(false),
				},
			},
		},
	}
	refs := &Refs{
		TenantRefs:       map[string]int64{"tenant_a": 1},
		ProductRefs:      map[string]int64{"product_a": 10},
		ProductModelRefs: map[string]int64{"model_a": 20},
		EntryRefs: map[string]EntryRef{
			"faq.contact_support": {EntryKey: "faq:206"},
		},
	}

	report, err := evaluator.Evaluate(context.Background(), suite, refs, RunOptions{GenerateAnswer: true})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if !report.Summary.AnswerEvaluated {
		t.Fatalf("AnswerEvaluated = false, want true")
	}
	if report.Cases[0].Passed != true {
		t.Fatalf("case passed = %v, want true, failures=%v", report.Cases[0].Passed, report.Cases[0].Failures)
	}
	if report.Summary.HandoffAccuracy != 1 {
		t.Fatalf("HandoffAccuracy = %v, want 1", report.Summary.HandoffAccuracy)
	}
}
