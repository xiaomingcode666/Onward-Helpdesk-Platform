package providers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"remotehelpdesk/internal/pkg/config"
)

func TestSub2APIAdminListUserKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("method = %s", req.Method)
		}
		if req.URL.Path != "/api/v1/admin/users/15/api-keys" {
			t.Fatalf("path = %s", req.URL.Path)
		}
		if req.URL.Query().Get("timezone") != "Asia/Shanghai" {
			t.Fatalf("timezone = %s", req.URL.Query().Get("timezone"))
		}
		if req.Header.Get("X-Api-Key") != "admin-key" {
			t.Fatalf("X-Api-Key = %s", req.Header.Get("X-Api-Key"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":0,"message":"success","data":{"items":[{"id":42,"user_id":15,"key":"sk-secret","name":"Work","group_id":3,"status":"active","quota":0,"quota_used":0,"created_at":"2026-08-03T17:39:18+08:00"}],"total":1,"page":1,"page_size":20,"pages":1}}`)
	}))
	defer server.Close()

	provider := NewSub2APIProvider(&config.Sub2APIConfig{
		BaseURL:    server.URL,
		Timeout:    time.Second,
		MaxRetries: 1,
	})
	resp, err := provider.AdminListUserKeys(context.Background(), "admin-key", 15, "Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Data.Total != 1 || len(resp.Data.Items) != 1 {
		t.Fatalf("unexpected response: %+v", resp.Data)
	}
	if resp.Data.Items[0].Name != "Work" || resp.Data.Items[0].UserID != 15 {
		t.Fatalf("unexpected key: %+v", resp.Data.Items[0])
	}
}
