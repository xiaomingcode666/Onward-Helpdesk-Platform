package projectconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testDocument() Document {
	return Document{SchemaVersion: 1, TenantID: 7, Environment: "staging", Projects: []Project{{Key: "support", Name: "知识服务"}}, SecretRefs: []string{}}
}

func TestConfigurationSchemaPolicyAndSecrets(t *testing.T) {
	doc := testDocument()
	if r := Validate(doc, 7, "staging", nil); !r.Valid {
		t.Fatalf("valid document rejected: %+v", r)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Document)
		kind   string
	}{
		{"schema version", func(d *Document) { d.SchemaVersion = 2 }, "schema"},
		{"blank project name", func(d *Document) { d.Projects = []Project{{Key: "support", Name: " "}} }, "policy"},
		{"duplicate project", func(d *Document) { d.Projects = append(d.Projects, d.Projects[0]) }, "policy"},
		{"tenant isolation", func(d *Document) { d.TenantID = 8 }, "policy"},
		{"environment isolation", func(d *Document) { d.Environment = "production" }, "policy"},
		{"missing resolver", func(d *Document) { d.SecretRefs = []string{"secret://mail"} }, "secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := testDocument()
			tc.mutate(&d)
			r := Validate(d, 7, "staging", nil)
			if r.Valid || r.Issues[0].Kind != tc.kind {
				t.Fatalf("bad result %+v", r)
			}
		})
	}
	root := t.TempDir()
	dir := filepath.Join(root, "7", "staging")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	secretPath := filepath.Join(dir, "mail")
	if err := os.WriteFile(secretPath, []byte("synthetic-test-secret-only"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := CheckSecret(root, 7, "staging", "secret://mail"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		tenant   int64
		env, ref string
	}{{8, "staging", "secret://mail"}, {7, "production", "secret://mail"}, {7, "staging", "secret://../mail"}, {7, "staging", "secret://missing"}} {
		err := CheckSecret(root, tc.tenant, tc.env, tc.ref)
		if err == nil || strings.Contains(err.Error(), "synthetic-test-secret-only") {
			t.Fatalf("unsafe secret validation: %v", err)
		}
	}
	if err := os.WriteFile(secretPath, []byte(" \n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := CheckSecret(root, 7, "staging", "secret://mail"); err == nil {
		t.Fatal("empty secret accepted")
	}
}

func TestDeploymentRejectsTamperingScopeAndTrailingData(t *testing.T) {
	doc := testDocument()
	bundle := Deployment{VersionID: 3, Digest: Digest(doc), Document: doc}
	path := filepath.Join(t.TempDir(), "config.json")
	write := func(b Deployment) {
		t.Helper()
		data, _ := json.Marshal(b)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(bundle)
	if _, err := ReadDeployment(path, 7, "staging", bundle.Digest, ""); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		tenant    int64
		env, hash string
	}{{8, "staging", bundle.Digest}, {7, "production", bundle.Digest}, {7, "staging", ""}} {
		if _, err := ReadDeployment(path, tc.tenant, tc.env, tc.hash, ""); err == nil {
			t.Fatal("incorrect deployment identity accepted")
		}
	}
	bundle.Document.Projects = append(bundle.Document.Projects, Project{Key: "tampered", Name: "Tampered"})
	write(bundle)
	if _, err := ReadDeployment(path, 7, "staging", bundle.Digest, ""); err == nil {
		t.Fatal("tampered document accepted")
	}
	for _, raw := range []string{`{"schema_version":1,"password":"do-not-store"}`, `{} {}`, strings.Repeat(" ", 256*1024+1)} {
		if _, err := Decode([]byte(raw)); err == nil {
			t.Fatal("invalid raw document accepted")
		}
	}
}
