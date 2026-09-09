package eval

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func LoadSuite(path string) (*Suite, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read eval suite: %w", err)
	}
	var suite Suite
	if err := yaml.Unmarshal(raw, &suite); err != nil {
		return nil, fmt.Errorf("unmarshal eval suite: %w", err)
	}
	if err := suite.Validate(); err != nil {
		return nil, err
	}
	return &suite, nil
}

func LoadRefs(path string) (*Refs, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read eval refs: %w", err)
	}
	var refs Refs
	if err := yaml.Unmarshal(raw, &refs); err != nil {
		return nil, fmt.Errorf("unmarshal eval refs: %w", err)
	}
	return &refs, nil
}

func (s Suite) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return errors.New("eval suite name is required")
	}
	if s.Version <= 0 {
		return errors.New("eval suite version must be greater than 0")
	}
	if len(s.Cases) == 0 {
		return errors.New("eval suite must contain at least one case")
	}

	seen := make(map[string]struct{}, len(s.Cases))
	for _, item := range s.NormalizedCases() {
		if strings.TrimSpace(item.ID) == "" {
			return errors.New("eval case id is required")
		}
		if _, ok := seen[item.ID]; ok {
			return fmt.Errorf("duplicate eval case id: %s", item.ID)
		}
		seen[item.ID] = struct{}{}
		if strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("eval case %s name is required", item.ID)
		}
		if strings.TrimSpace(item.Input.Query) == "" {
			return fmt.Errorf("eval case %s query is required", item.ID)
		}
		if item.AnswerExpectation.ShouldAnswer == false && boolValue(item.AnswerExpectation.CitationRequired) {
			return fmt.Errorf("eval case %s cannot require citations for a no-answer result", item.ID)
		}
		if item.AnswerExpectation.ShouldAnswer == false && item.AnswerExpectation.MinCitationCount > 0 {
			return fmt.Errorf("eval case %s cannot require citations for a no-answer result", item.ID)
		}
		if item.AnswerExpectation.ExpectHandoff && item.AnswerExpectation.ShouldAnswer {
			return fmt.Errorf("eval case %s handoff cases must set shouldAnswer=false", item.ID)
		}
	}

	return nil
}

func (r Refs) ResolveEntryKey(ref string) (string, error) {
	item, ok := r.EntryRefs[ref]
	if !ok {
		return "", fmt.Errorf("entry ref not found: %s", ref)
	}
	if strings.TrimSpace(item.EntryKey) == "" {
		return "", fmt.Errorf("entry ref %s has empty entryKey", ref)
	}
	return item.EntryKey, nil
}

func (r Refs) ResolveEntryKeys(refs []string) ([]string, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		key, err := r.ResolveEntryKey(ref)
		if err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	return out, nil
}

func boolValue(v *bool) bool {
	return v != nil && *v
}
