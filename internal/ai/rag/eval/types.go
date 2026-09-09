package eval

type Suite struct {
	Name        string   `yaml:"name" json:"name"`
	Version     int      `yaml:"version" json:"version"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Defaults    Defaults `yaml:"defaults,omitempty" json:"defaults,omitempty"`
	Cases       []Case   `yaml:"cases" json:"cases"`
}

type Defaults struct {
	Enabled              *bool                `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Scope                Scope                `yaml:"scope,omitempty" json:"scope,omitempty"`
	RetrievalExpectation RetrievalExpectation `yaml:"retrievalExpectation,omitempty" json:"retrievalExpectation,omitempty"`
	AnswerExpectation    AnswerExpectation    `yaml:"answerExpectation,omitempty" json:"answerExpectation,omitempty"`
}

type Case struct {
	ID                   string               `yaml:"id" json:"id"`
	Name                 string               `yaml:"name" json:"name"`
	Enabled              *bool                `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Tags                 []string             `yaml:"tags,omitempty" json:"tags,omitempty"`
	IntentTags           []string             `yaml:"intentTags,omitempty" json:"intentTags,omitempty"`
	FaultCodes           []string             `yaml:"faultCodes,omitempty" json:"faultCodes,omitempty"`
	Scope                Scope                `yaml:"scope,omitempty" json:"scope,omitempty"`
	Input                Input                `yaml:"input" json:"input"`
	RetrievalExpectation RetrievalExpectation `yaml:"retrievalExpectation,omitempty" json:"retrievalExpectation,omitempty"`
	AnswerExpectation    AnswerExpectation    `yaml:"answerExpectation,omitempty" json:"answerExpectation,omitempty"`
	Notes                string               `yaml:"notes,omitempty" json:"notes,omitempty"`
}

type Scope struct {
	TenantRef         string   `yaml:"tenantRef,omitempty" json:"tenantRef,omitempty"`
	ProductRef        string   `yaml:"productRef,omitempty" json:"productRef,omitempty"`
	ProductModelRef   string   `yaml:"productModelRef,omitempty" json:"productModelRef,omitempty"`
	KnowledgeBaseRefs []string `yaml:"knowledgeBaseRefs,omitempty" json:"knowledgeBaseRefs,omitempty"`
	Locale            string   `yaml:"locale,omitempty" json:"locale,omitempty"`
	RegionCode        string   `yaml:"regionCode,omitempty" json:"regionCode,omitempty"`
	Audience          string   `yaml:"audience,omitempty" json:"audience,omitempty"`
}

type Input struct {
	Query         string   `yaml:"query" json:"query"`
	QueryVariants []string `yaml:"queryVariants,omitempty" json:"queryVariants,omitempty"`
}

type RetrievalExpectation struct {
	ExpectedEntryRefs   []string `yaml:"expectedEntryRefs,omitempty" json:"expectedEntryRefs,omitempty"`
	ExpectedEntryKeys   []string `yaml:"expectedEntryKeys,omitempty" json:"expectedEntryKeys,omitempty"`
	ForbiddenEntryRefs  []string `yaml:"forbiddenEntryRefs,omitempty" json:"forbiddenEntryRefs,omitempty"`
	ForbiddenEntryKeys  []string `yaml:"forbiddenEntryKeys,omitempty" json:"forbiddenEntryKeys,omitempty"`
	MinExpectedHitsAt5  int      `yaml:"minExpectedHitsAt5,omitempty" json:"minExpectedHitsAt5,omitempty"`
	MinExpectedHitsAt10 int      `yaml:"minExpectedHitsAt10,omitempty" json:"minExpectedHitsAt10,omitempty"`
}

type AnswerExpectation struct {
	ShouldAnswer           bool     `yaml:"shouldAnswer" json:"shouldAnswer"`
	ExpectHandoff          bool     `yaml:"expectHandoff,omitempty" json:"expectHandoff,omitempty"`
	CitationRequired       *bool    `yaml:"citationRequired,omitempty" json:"citationRequired,omitempty"`
	MinCitationCount       int      `yaml:"minCitationCount,omitempty" json:"minCitationCount,omitempty"`
	MustCiteAnyOfEntryRefs []string `yaml:"mustCiteAnyOfEntryRefs,omitempty" json:"mustCiteAnyOfEntryRefs,omitempty"`
	MustContain            []string `yaml:"mustContain,omitempty" json:"mustContain,omitempty"`
	MustNotContain         []string `yaml:"mustNotContain,omitempty" json:"mustNotContain,omitempty"`
	AllowedNoAnswerReasons []string `yaml:"allowedNoAnswerReasons,omitempty" json:"allowedNoAnswerReasons,omitempty"`
}

type Refs struct {
	TenantRefs        map[string]int64    `yaml:"tenantRefs,omitempty" json:"tenantRefs,omitempty"`
	ProductRefs       map[string]int64    `yaml:"productRefs,omitempty" json:"productRefs,omitempty"`
	ProductModelRefs  map[string]int64    `yaml:"productModelRefs,omitempty" json:"productModelRefs,omitempty"`
	KnowledgeBaseRefs map[string]int64    `yaml:"knowledgeBaseRefs,omitempty" json:"knowledgeBaseRefs,omitempty"`
	EntryRefs         map[string]EntryRef `yaml:"entryRefs,omitempty" json:"entryRefs,omitempty"`
}

type EntryRef struct {
	EntryKey string `yaml:"entryKey" json:"entryKey"`
	Kind     string `yaml:"kind,omitempty" json:"kind,omitempty"`
	Note     string `yaml:"note,omitempty" json:"note,omitempty"`
}

func (s Suite) NormalizedCases() []Case {
	enabledDefault := true
	if s.Defaults.Enabled != nil {
		enabledDefault = *s.Defaults.Enabled
	}

	out := make([]Case, 0, len(s.Cases))
	for _, item := range s.Cases {
		normalized := item
		if normalized.Enabled == nil {
			normalized.Enabled = boolPtr(enabledDefault)
		}
		normalized.Scope = mergeScope(s.Defaults.Scope, normalized.Scope)
		normalized.RetrievalExpectation = mergeRetrievalExpectation(s.Defaults.RetrievalExpectation, normalized.RetrievalExpectation)
		normalized.AnswerExpectation = mergeAnswerExpectation(s.Defaults.AnswerExpectation, normalized.AnswerExpectation)
		out = append(out, normalized)
	}
	return out
}

func mergeScope(defaults Scope, item Scope) Scope {
	if item.TenantRef == "" {
		item.TenantRef = defaults.TenantRef
	}
	if item.ProductRef == "" {
		item.ProductRef = defaults.ProductRef
	}
	if item.ProductModelRef == "" {
		item.ProductModelRef = defaults.ProductModelRef
	}
	if len(item.KnowledgeBaseRefs) == 0 {
		item.KnowledgeBaseRefs = cloneStrings(defaults.KnowledgeBaseRefs)
	}
	if item.Locale == "" {
		item.Locale = defaults.Locale
	}
	if item.RegionCode == "" {
		item.RegionCode = defaults.RegionCode
	}
	if item.Audience == "" {
		item.Audience = defaults.Audience
	}
	return item
}

func mergeRetrievalExpectation(defaults RetrievalExpectation, item RetrievalExpectation) RetrievalExpectation {
	if len(item.ExpectedEntryRefs) == 0 {
		item.ExpectedEntryRefs = cloneStrings(defaults.ExpectedEntryRefs)
	}
	if len(item.ExpectedEntryKeys) == 0 {
		item.ExpectedEntryKeys = cloneStrings(defaults.ExpectedEntryKeys)
	}
	if len(item.ForbiddenEntryRefs) == 0 {
		item.ForbiddenEntryRefs = cloneStrings(defaults.ForbiddenEntryRefs)
	}
	if len(item.ForbiddenEntryKeys) == 0 {
		item.ForbiddenEntryKeys = cloneStrings(defaults.ForbiddenEntryKeys)
	}
	if item.MinExpectedHitsAt5 == 0 {
		item.MinExpectedHitsAt5 = defaults.MinExpectedHitsAt5
	}
	if item.MinExpectedHitsAt10 == 0 {
		item.MinExpectedHitsAt10 = defaults.MinExpectedHitsAt10
	}
	return item
}

func mergeAnswerExpectation(defaults AnswerExpectation, item AnswerExpectation) AnswerExpectation {
	if item.CitationRequired == nil && defaults.CitationRequired != nil {
		item.CitationRequired = boolPtr(*defaults.CitationRequired)
	}
	if item.MinCitationCount == 0 {
		item.MinCitationCount = defaults.MinCitationCount
	}
	if len(item.MustCiteAnyOfEntryRefs) == 0 {
		item.MustCiteAnyOfEntryRefs = cloneStrings(defaults.MustCiteAnyOfEntryRefs)
	}
	if len(item.MustContain) == 0 {
		item.MustContain = cloneStrings(defaults.MustContain)
	}
	if len(item.MustNotContain) == 0 {
		item.MustNotContain = cloneStrings(defaults.MustNotContain)
	}
	if len(item.AllowedNoAnswerReasons) == 0 {
		item.AllowedNoAnswerReasons = cloneStrings(defaults.AllowedNoAnswerReasons)
	}
	return item
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func boolPtr(v bool) *bool {
	return &v
}
