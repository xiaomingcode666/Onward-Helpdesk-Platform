package dsl

import (
	"encoding/json"
	"strings"
)

type Definition struct {
	SchemaVersion    int               `json:"schemaVersion"`
	EntryNodeID      string            `json:"entryNodeId"`
	ModelPolicy      *ModelPolicy      `json:"modelPolicy,omitempty"`
	RuntimeVariables []RuntimeVariable `json:"runtimeVariables,omitempty"`
	Nodes            []Node            `json:"nodes"`
	Edges            []Edge            `json:"edges"`
}

const (
	ModelCredentialScopeTenantDefault = "tenant_default"
	ModelCredentialScopeProduct       = "product"
)

// ModelPolicy controls portable model-account behavior without persisting
// tenant-owned credential identifiers in a workflow definition. Sources are
// attempted in order, so fallback behavior remains workflow data instead of
// being coupled to the runtime router.
type ModelPolicy struct {
	CredentialChain []string `json:"credentialChain,omitempty"`
}

// UnmarshalJSON keeps immutable workflow versions published with the original
// credentialScope field executable while all newly marshalled definitions use
// credentialChain.
func (p *ModelPolicy) UnmarshalJSON(data []byte) error {
	var raw struct {
		CredentialChain []string `json:"credentialChain"`
		CredentialScope string   `json:"credentialScope"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw.CredentialChain) > 0 {
		p.CredentialChain = cloneCredentialChain(raw.CredentialChain)
		return nil
	}
	legacyScope := strings.TrimSpace(raw.CredentialScope)
	switch legacyScope {
	case ModelCredentialScopeProduct:
		p.CredentialChain = []string{ModelCredentialScopeProduct, ModelCredentialScopeTenantDefault}
	case ModelCredentialScopeTenantDefault:
		p.CredentialChain = []string{ModelCredentialScopeTenantDefault}
	case "":
		p.CredentialChain = nil
	default:
		p.CredentialChain = []string{legacyScope}
	}
	return nil
}

func (d Definition) EffectiveModelCredentialChain(productID int64) []string {
	if d.ModelPolicy != nil && len(d.ModelPolicy.CredentialChain) > 0 {
		return cloneCredentialChain(d.ModelPolicy.CredentialChain)
	}
	return DefaultModelCredentialChain(productID)
}

func DefaultModelCredentialChain(productID int64) []string {
	if productID > 0 {
		return []string{ModelCredentialScopeProduct, ModelCredentialScopeTenantDefault}
	}
	return []string{ModelCredentialScopeTenantDefault}
}

func IsSupportedModelCredentialScope(scope string) bool {
	switch strings.TrimSpace(scope) {
	case ModelCredentialScopeTenantDefault, ModelCredentialScopeProduct:
		return true
	default:
		return false
	}
}

func cloneCredentialChain(chain []string) []string {
	ret := make([]string, len(chain))
	for index, scope := range chain {
		ret[index] = strings.TrimSpace(scope)
	}
	return ret
}

func (d Definition) HasNodeType(nodeType string) bool {
	for _, node := range d.Nodes {
		if node.Type == nodeType {
			return true
		}
	}
	return false
}

// RuntimeVariable declares a portable value supplied by the conversation or
// the immutable Agent Release. Workflow templates never persist tenant-owned
// resource identifiers directly.
type RuntimeVariable struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	Source      string `json:"source"`
	Required    bool   `json:"required,omitempty"`
	Description string `json:"description,omitempty"`
}

type Node struct {
	ID                string                      `json:"id"`
	Type              string                      `json:"type"`
	Name              string                      `json:"name"`
	Position          Position                    `json:"position"`
	Config            json.RawMessage             `json:"config"`
	Inputs            map[string]VariableSelector `json:"inputs,omitempty"`
	ErrorTargetNodeID string                      `json:"errorTargetNodeId,omitempty"`
}

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Edge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
}

type ConditionConfig struct {
	Branches []ConditionBranch `json:"branches,omitempty"`
}

type ConditionBranch struct {
	ID           string     `json:"id"`
	Name         string     `json:"name,omitempty"`
	TargetNodeID string     `json:"targetNodeId"`
	Condition    *Condition `json:"condition,omitempty"`
	Default      bool       `json:"default,omitempty"`
}

type Condition struct {
	Expression string            `json:"expression,omitempty"`
	Left       *VariableSelector `json:"left,omitempty"`
	Operator   string            `json:"operator,omitempty"`
	Right      any               `json:"right,omitempty"`
}

type VariableSelector struct {
	NodeID string `json:"nodeId"`
	Field  string `json:"field"`
}

// KnowledgeMergeConfig joins results from parallel knowledge retrieval nodes.
type KnowledgeMergeConfig struct {
	Sources  []VariableSelector `json:"sources"`
	MaxItems int                `json:"maxItems,omitempty"`
}

// AnswerabilityGateConfig keeps answerability policy portable. It evaluates
// retrieved evidence without binding the workflow to a product or service domain.
type AnswerabilityGateConfig struct {
	MinScore        float64 `json:"minScore,omitempty"`
	StrongScore     float64 `json:"strongScore,omitempty"`
	MinMatchedTerms int     `json:"minMatchedTerms,omitempty"`
}

// EntryContextConfig controls how an unbound conversation enters the service
// workflow. It contains no tenant or product identifiers and remains portable.
type EntryContextConfig struct {
	UnboundMode string `json:"unboundMode,omitempty"`
}

// ServiceAccessPolicyConfig controls customer-facing service capabilities.
// Rules are evaluated against runtime context and the capabilities present in
// the workflow definition instead of binding to a concrete product service.
type ServiceAccessPolicyConfig struct {
	HumanHandoffMode string `json:"humanHandoffMode,omitempty"`
	TicketAccessMode string `json:"ticketAccessMode,omitempty"`
}

// KnowledgeRetrieveConfig contains only portable logical scope and tuning
// parameters. Concrete knowledge base identifiers are resolved at runtime.
type KnowledgeRetrieveConfig struct {
	BindingMethod            string  `json:"bindingMethod,omitempty"`
	Scope                    string  `json:"scope,omitempty"` // Legacy alias for BindingMethod.
	KnowledgeBaseIDsVariable string  `json:"knowledgeBaseIdsVariable,omitempty"`
	Audience                 string  `json:"audience,omitempty"`
	TopK                     int     `json:"topK,omitempty"`
	ScoreThreshold           float64 `json:"scoreThreshold,omitempty"`
	ContextMaxTokens         int     `json:"contextMaxTokens,omitempty"`
	MaxContextItems          int     `json:"maxContextItems,omitempty"`
}

func (c KnowledgeRetrieveConfig) EffectiveBindingMethod() string {
	if c.BindingMethod != "" {
		return c.BindingMethod
	}
	return c.Scope
}

// SubflowConfig references an immutable published workflow version.
type SubflowConfig struct {
	WorkflowVersionID int64 `json:"workflowVersionId"`
}

// LoopConfig executes one immutable subflow repeatedly with a hard bound.
// Until evaluates against the loop node outputs after each iteration.
type LoopConfig struct {
	WorkflowVersionID int64      `json:"workflowVersionId"`
	MaxIterations     int        `json:"maxIterations"`
	Until             *Condition `json:"until,omitempty"`
	FailurePolicy     string     `json:"failurePolicy,omitempty"`
}
