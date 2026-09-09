package eval

import (
	"fmt"
	"strings"
)

type ResolvedCase struct {
	Case               Case
	TenantID           int64
	ProductID          int64
	ProductModelID     int64
	KnowledgeBaseIDs   []int64
	ExpectedEntryKeys  []string
	ForbiddenEntryKeys []string
	MustCiteEntryKeys  []string
}

func (r Refs) ResolveCase(item Case) (*ResolvedCase, error) {
	ret := &ResolvedCase{Case: item}

	var err error
	if item.Scope.TenantRef != "" {
		ret.TenantID, err = resolveInt64Ref("tenant", item.Scope.TenantRef, r.TenantRefs)
		if err != nil {
			return nil, err
		}
	}
	if item.Scope.ProductRef != "" {
		ret.ProductID, err = resolveInt64Ref("product", item.Scope.ProductRef, r.ProductRefs)
		if err != nil {
			return nil, err
		}
	}
	if item.Scope.ProductModelRef != "" {
		ret.ProductModelID, err = resolveInt64Ref("product model", item.Scope.ProductModelRef, r.ProductModelRefs)
		if err != nil {
			return nil, err
		}
	}
	if len(item.Scope.KnowledgeBaseRefs) > 0 {
		ret.KnowledgeBaseIDs = make([]int64, 0, len(item.Scope.KnowledgeBaseRefs))
		for _, ref := range item.Scope.KnowledgeBaseRefs {
			value, err := resolveInt64Ref("knowledge base", ref, r.KnowledgeBaseRefs)
			if err != nil {
				return nil, err
			}
			ret.KnowledgeBaseIDs = append(ret.KnowledgeBaseIDs, value)
		}
	}

	ret.ExpectedEntryKeys = mergeStringLists(item.RetrievalExpectation.ExpectedEntryKeys)
	if len(item.RetrievalExpectation.ExpectedEntryRefs) > 0 {
		keys, err := r.ResolveEntryKeys(item.RetrievalExpectation.ExpectedEntryRefs)
		if err != nil {
			return nil, err
		}
		ret.ExpectedEntryKeys = mergeStringLists(ret.ExpectedEntryKeys, keys)
	}

	ret.ForbiddenEntryKeys = mergeStringLists(item.RetrievalExpectation.ForbiddenEntryKeys)
	if len(item.RetrievalExpectation.ForbiddenEntryRefs) > 0 {
		keys, err := r.ResolveEntryKeys(item.RetrievalExpectation.ForbiddenEntryRefs)
		if err != nil {
			return nil, err
		}
		ret.ForbiddenEntryKeys = mergeStringLists(ret.ForbiddenEntryKeys, keys)
	}

	if len(item.AnswerExpectation.MustCiteAnyOfEntryRefs) > 0 {
		keys, err := r.ResolveEntryKeys(item.AnswerExpectation.MustCiteAnyOfEntryRefs)
		if err != nil {
			return nil, err
		}
		ret.MustCiteEntryKeys = mergeStringLists(keys)
	}

	return ret, nil
}

func resolveInt64Ref(kind string, ref string, values map[string]int64) (int64, error) {
	value, ok := values[strings.TrimSpace(ref)]
	if !ok || value <= 0 {
		return 0, fmt.Errorf("%s ref not found: %s", kind, ref)
	}
	return value, nil
}

func mergeStringLists(groups ...[]string) []string {
	if len(groups) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	ret := make([]string, 0)
	for _, group := range groups {
		for _, item := range group {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			ret = append(ret, item)
		}
	}
	if len(ret) == 0 {
		return nil
	}
	return ret
}
