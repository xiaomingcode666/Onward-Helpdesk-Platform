package ticketpolicy

import (
	"fmt"
	"slices"
)

// CaseWorkflow is the small, deterministic workflow used by customer cases.
// Keeping this contract in one package lets the API and future workflow editor
// share the same transition rules.
var caseWorkflow = map[string][]string{
	// A new case may be picked up directly by its engineer; the historical
	// support-agent reception edge stays valid for configurations that use it.
	"new":             {"acknowledged", "in_triage", "assigned", "cancelled"},
	"acknowledged":    {"in_triage", "assigned", "waiting", "resolved", "cancelled"},
	"in_triage":       {"assigned", "waiting", "restored", "resolved", "cancelled"},
	"assigned":        {"in_triage", "waiting", "restored", "resolved", "cancelled"},
	"waiting":         {"acknowledged", "in_triage", "assigned", "restored", "resolved", "closure_pending", "cancelled"},
	"restored":        {"in_triage", "waiting", "resolved", "cancelled"},
	"resolved":        {"closure_pending", "closed", "in_triage"},
	"closure_pending": {"closed", "in_triage"},
	"closed":          {"in_triage"},
	"cancelled":       {},
}

func AllowedCaseTransitions(status string) []string {
	return slices.Clone(caseWorkflow[status])
}

func CanTransitionCase(from, to string) bool {
	return slices.Contains(caseWorkflow[from], to)
}

// Workflow is a deterministic transition policy, not an AI execution graph.
type Workflow struct {
	Transitions map[string][]string `json:"transitions"`
}

func CaseStates() []string {
	return []string{"new", "acknowledged", "in_triage", "assigned", "waiting", "restored", "resolved", "closure_pending", "closed", "cancelled"}
}

func DefaultWorkflow() Workflow {
	w := Workflow{Transitions: map[string][]string{}}
	for _, state := range CaseStates() {
		w.Transitions[state] = append([]string{}, caseWorkflow[state]...)
	}
	return w
}

// These edges preserve reception, dispatch, resuming work and a path to closure.
// Optional shortcuts and re-opening can be disabled without stranding a case.
func RequiredCaseTransitions() map[string][]string {
	return map[string][]string{
		"new": {"acknowledged", "cancelled"}, "acknowledged": {"in_triage", "assigned"},
		"in_triage": {"assigned", "restored"}, "assigned": {"in_triage", "restored"},
		"waiting":  {"acknowledged", "in_triage", "assigned", "restored"},
		"restored": {"resolved"}, "resolved": {"closure_pending"}, "closure_pending": {"closed"},
	}
}

func (w Workflow) Allows(from, to string) bool {
	return from == to || slices.Contains(w.Transitions[from], to)
}

func (w Workflow) Validate() error {
	if len(w.Transitions) != 10 {
		return fmt.Errorf("工单流程必须保留全部 10 种状态")
	}
	for _, state := range CaseStates() {
		targets, exists := w.Transitions[state]
		if !exists || targets == nil {
			return fmt.Errorf("状态 %s 缺少下一步列表", state)
		}
		seen := map[string]bool{}
		for _, target := range targets {
			if seen[target] || !CanTransitionCase(state, target) {
				return fmt.Errorf("不支持或重复的状态流转：%s → %s", state, target)
			}
			seen[target] = true
		}
		for _, required := range RequiredCaseTransitions()[state] {
			if !seen[required] {
				return fmt.Errorf("不能移除必要的状态流转：%s → %s", state, required)
			}
		}
	}
	return nil
}
