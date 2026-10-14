package i18nx

import (
	"sort"
	"strings"
	"testing"
)

// 内置数据名称直接展示给用户，每种语言都必须提供，且占位符一致。
func TestBuiltinResourcesExistInEveryLocale(t *testing.T) {
	loadMessages()
	reference := builtinKeys(messages[LocaleZhCN])
	if len(reference) == 0 {
		t.Fatal("no builtin resources found in zh-CN")
	}
	for _, locale := range []string{LocaleEnUS, LocaleEsES} {
		keys := builtinKeys(messages[locale])
		if strings.Join(keys, ",") != strings.Join(reference, ",") {
			t.Fatalf("%s builtin keys = %v, want %v", locale, keys, reference)
		}
		for _, key := range reference {
			if got, want := placeholders(messages[locale][key]), placeholders(messages[LocaleZhCN][key]); got != want {
				t.Fatalf("%s %s placeholders = %q, want %q", locale, key, got, want)
			}
		}
	}
}

func builtinKeys(values map[string]string) []string {
	keys := make([]string, 0)
	for key := range values {
		if strings.HasPrefix(key, "builtin.") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func placeholders(value string) string {
	names := make([]string, 0)
	for {
		start := strings.IndexByte(value, '{')
		if start < 0 {
			break
		}
		end := strings.IndexByte(value[start:], '}')
		if end < 0 {
			break
		}
		names = append(names, value[start+1:start+end])
		value = value[start+end+1:]
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}
