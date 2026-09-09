package bootstrap

import (
	"os"
	"regexp"
	"testing"
)

func TestEnterpriseProductManualFileRoutesUseProductPermissions(t *testing.T) {
	source, err := os.ReadFile("routes.go")
	if err != nil {
		t.Fatalf("read routes.go: %v", err)
	}
	text := string(source)
	for _, pattern := range []string{
		`group\.GET\("/products/:id/manual-files", require\(constants\.PermissionProductView\), enterprise\.ProductManualFiles\)`,
		`group\.GET\("/products/:id/manual-files/:manualFileId/content", require\(constants\.PermissionProductView\), enterprise\.ProductManualFileContent\)`,
		`group\.POST\("/products/:id/manual-files/_upload", require\(constants\.PermissionProductUpdate\), enterprise\.ProductManualFileUpload\)`,
		`group\.PATCH\("/products/:id/manual-files/:manualFileId", require\(constants\.PermissionProductUpdate\), enterprise\.ProductManualFileUpdate\)`,
		`group\.DELETE\("/products/:id/manual-files/:manualFileId", require\(constants\.PermissionProductUpdate\), enterprise\.ProductManualFileDelete\)`,
	} {
		if !regexp.MustCompile(pattern).MatchString(text) {
			t.Fatalf("routes.go missing manual permission pattern: %s", pattern)
		}
	}
	for _, forbidden := range []string{
		`/products/:id/manual-files", require\(constants\.PermissionKnowledgeDocument`,
		`/products/:id/manual-files/:manualFileId/content", require\(constants\.PermissionKnowledgeDocument`,
		`/products/:id/manual-files/_upload", require\(constants\.PermissionKnowledgeDocument`,
		`/products/:id/manual-files/:manualFileId", require\(constants\.PermissionKnowledgeDocument`,
	} {
		if regexp.MustCompile(forbidden).MatchString(text) {
			t.Fatalf("manual file route still uses knowledge document permission: %s", forbidden)
		}
	}
}
