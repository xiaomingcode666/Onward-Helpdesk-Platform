package ticketpolicy

import "testing"

func TestCaseWorkflowTransitions(t *testing.T) {
	if !CanTransitionCase("new", "acknowledged") || CanTransitionCase("new", "resolved") {
		t.Fatal("new state has invalid transitions")
	}
	if !CanTransitionCase("resolved", "closure_pending") || !CanTransitionCase("closed", "in_triage") {
		t.Fatal("resolution and reopen transitions are missing")
	}
	if len(AllowedCaseTransitions("cancelled")) != 0 {
		t.Fatal("cancelled must be terminal")
	}
}
