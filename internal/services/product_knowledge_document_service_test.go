package services

import (
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
)

func TestBuildEnterpriseProductDocumentDTOUsesInlineContentMetadata(t *testing.T) {
	content := "首次压接后端子受热回弹。"
	document := models.KnowledgeDocument{
		ID:            34,
		Title:         "端子受热回弹",
		ContentType:   enums.KnowledgeDocumentContentTypeMarkdown,
		Content:       content,
		SourceAssetID: 0,
	}

	result := KnowledgeDocumentService.buildEnterpriseProductDocumentDTO(1, document, 1, "knowledge_entry", document.ID)

	if result.FileSize != int64(len([]byte(content))) {
		t.Fatalf("FileSize = %d, want %d", result.FileSize, len([]byte(content)))
	}
	if result.MimeType != "text/markdown" {
		t.Fatalf("MimeType = %q, want text/markdown", result.MimeType)
	}
}

func TestKnowledgeDocumentContentMimeType(t *testing.T) {
	tests := []struct {
		name        string
		contentType enums.KnowledgeDocumentContentType
		want        string
	}{
		{name: "markdown", contentType: enums.KnowledgeDocumentContentTypeMarkdown, want: "text/markdown"},
		{name: "html", contentType: enums.KnowledgeDocumentContentTypeHTML, want: "text/html"},
		{name: "unknown", contentType: enums.KnowledgeDocumentContentType("unknown"), want: "text/plain"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := knowledgeDocumentContentMimeType(tt.contentType); got != tt.want {
				t.Fatalf("knowledgeDocumentContentMimeType() = %q, want %q", got, tt.want)
			}
		})
	}
}
