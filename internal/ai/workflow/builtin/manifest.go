package builtin

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"remotehelpdesk/internal/ai/workflow/dsl"
)

// Platform workflow manifests are code-owned deployment assets and travel with
// the server binary instead of depending on rows seeded in one environment.
//
//go:embed manifests/*.json
var manifestFiles embed.FS

type Manifest struct {
	ManifestVersion int            `json:"manifestVersion"`
	Code            string         `json:"code"`
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	InitialChange   string         `json:"initialChange"`
	UpgradeChange   string         `json:"upgradeChange"`
	Definition      dsl.Definition `json:"definition"`
}

func Load() ([]Manifest, error) {
	paths, err := fs.Glob(manifestFiles, "manifests/*.json")
	if err != nil {
		return nil, fmt.Errorf("list platform workflow manifests: %w", err)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("no platform workflow manifests are embedded")
	}

	seen := make(map[string]string, len(paths))
	ret := make([]Manifest, 0, len(paths))
	for _, path := range paths {
		raw, err := manifestFiles.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read platform workflow manifest %s: %w", path, err)
		}
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		decoder.DisallowUnknownFields()
		var item Manifest
		if err := decoder.Decode(&item); err != nil {
			return nil, fmt.Errorf("decode platform workflow manifest %s: %w", path, err)
		}
		item.Code = strings.TrimSpace(item.Code)
		item.Name = strings.TrimSpace(item.Name)
		item.Description = strings.TrimSpace(item.Description)
		item.InitialChange = strings.TrimSpace(item.InitialChange)
		item.UpgradeChange = strings.TrimSpace(item.UpgradeChange)
		if item.ManifestVersion <= 0 || item.Code == "" || item.Name == "" || item.InitialChange == "" || item.Definition.SchemaVersion <= 0 {
			return nil, fmt.Errorf("platform workflow manifest %s is missing required fields", path)
		}
		if previous, ok := seen[item.Code]; ok {
			return nil, fmt.Errorf("platform workflow code %s is duplicated in %s and %s", item.Code, previous, path)
		}
		seen[item.Code] = filepath.Base(path)
		ret = append(ret, item)
	}
	return ret, nil
}

func Get(code string) (Manifest, bool, error) {
	items, err := Load()
	if err != nil {
		return Manifest{}, false, err
	}
	code = strings.TrimSpace(code)
	for _, item := range items {
		if item.Code == code {
			return item, true, nil
		}
	}
	return Manifest{}, false, nil
}
