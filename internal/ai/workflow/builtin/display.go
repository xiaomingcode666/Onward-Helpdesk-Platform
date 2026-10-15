package builtin

import (
	"encoding/json"
	"strings"
	"sync"

	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/pkg/i18nx"
)

var (
	builtinCodesOnce sync.Once
	builtinCodes     []string
)

func manifestCodes() []string {
	builtinCodesOnce.Do(func() {
		items, err := Load()
		if err != nil {
			return
		}
		for _, item := range items {
			builtinCodes = append(builtinCodes, item.Code)
		}
	})
	return builtinCodes
}

// NodeNameKey 返回内置流程节点名称的资源键。
func NodeNameKey(code, nodeID string) string {
	return "builtin.aiWorkflow." + code + ".node." + nodeID + ".name"
}

// BranchNameKey 返回内置流程条件分支名称的资源键。
func BranchNameKey(code, nodeID, branchID string) string {
	return "builtin.aiWorkflow." + code + ".node." + nodeID + ".branch." + branchID + ".name"
}

// LocalizeDefinition 返回按语言渲染内置节点和分支名称后的流程定义副本。
// 租户流程复制自平台内置流程，节点 ID 保持不变；只有名称仍等于某个内置资源在任一语言下的文本时才替换，
// 用户改过的名称和自建节点原样返回。传入的定义不会被修改。
func LocalizeDefinition(definition dsl.Definition, locale string) dsl.Definition {
	if len(definition.Nodes) == 0 {
		return definition
	}
	codes := manifestCodes()
	nodes := make([]dsl.Node, len(definition.Nodes))
	copy(nodes, definition.Nodes)
	for index := range nodes {
		node := &nodes[index]
		node.Name = localizeBuiltinName(codes, node.Name, locale, func(code string) string {
			return NodeNameKey(code, node.ID)
		})
		node.Config = localizeBranchNames(codes, node.ID, node.Config, locale)
	}
	definition.Nodes = nodes
	return definition
}

// LocalizeNodeName 按语言渲染单个内置节点名称，供运行记录等只有节点 ID 和名称的场景使用。
func LocalizeNodeName(nodeID, name, locale string) string {
	return localizeBuiltinName(manifestCodes(), name, locale, func(code string) string {
		return NodeNameKey(code, nodeID)
	})
}

func localizeBuiltinName(codes []string, name, locale string, keyFor func(code string) string) string {
	if strings.TrimSpace(name) == "" {
		return name
	}
	for _, code := range codes {
		key := keyFor(code)
		if !i18nx.HasKey(key) {
			continue
		}
		text := i18nx.NewText(key, nil)
		if text.MatchesAnyLocale(name) {
			return text.Render(locale)
		}
	}
	return name
}

func localizeBranchNames(codes []string, nodeID string, config json.RawMessage, locale string) json.RawMessage {
	if len(config) == 0 || !strings.Contains(string(config), `"branches"`) {
		return config
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(config, &fields); err != nil {
		return config
	}
	var branches []map[string]json.RawMessage
	if err := json.Unmarshal(fields["branches"], &branches); err != nil || len(branches) == 0 {
		return config
	}
	changed := false
	for _, branch := range branches {
		var branchID, name string
		if json.Unmarshal(branch["id"], &branchID) != nil || json.Unmarshal(branch["name"], &name) != nil {
			continue
		}
		localized := localizeBuiltinName(codes, name, locale, func(code string) string {
			return BranchNameKey(code, nodeID, branchID)
		})
		if localized == name {
			continue
		}
		encoded, err := json.Marshal(localized)
		if err != nil {
			continue
		}
		branch["name"] = encoded
		changed = true
	}
	if !changed {
		return config
	}
	encodedBranches, err := json.Marshal(branches)
	if err != nil {
		return config
	}
	fields["branches"] = encodedBranches
	encoded, err := json.Marshal(fields)
	if err != nil {
		return config
	}
	return encoded
}

// CanonicalizeDefinition 把以任一语言保存的内置节点和分支名称还原为清单中的规范值，
// 保证不同语言界面保存同一流程时存储内容和定义哈希保持一致。
func CanonicalizeDefinition(definition dsl.Definition) dsl.Definition {
	return LocalizeDefinition(definition, i18nx.LocaleZhCN)
}
