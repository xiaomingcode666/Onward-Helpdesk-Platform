package i18nx

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var literalTextKeyPattern = regexp.MustCompile(`(?:NewText|systemText)\("([A-Za-z0-9_.]+)"|I18n\((?:[A-Za-z0-9_.]+, )?"(error\.[A-Za-z0-9_.]+)"`)

// 代码中以字面量引用的系统文本资源键必须在每种语言中存在，避免页面显示资源键本身。
func TestLiteralTextKeysExistInEveryLocale(t *testing.T) {
	loadMessages()
	root := filepath.Join("..", "..")
	missing := map[string][]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "migration" || entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range literalTextKeyPattern.FindAllStringSubmatch(string(data), -1) {
			key := match[1]
			if key == "" {
				key = match[2]
			}
			if strings.HasSuffix(key, ".") {
				continue
			}
			for _, locale := range SupportedLocales {
				if _, ok := messages[locale][key]; !ok {
					missing[key] = append(missing[key], locale+" ("+filepath.ToSlash(path)+")")
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for key, where := range missing {
		t.Errorf("resource key %s missing in %s", key, strings.Join(where, ", "))
	}
}
