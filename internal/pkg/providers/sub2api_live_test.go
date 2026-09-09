package providers_test

import (
	"context"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mlogclub/simple/sqls"

	"remotehelpdesk/internal/bootstrap"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/pkg/secretstore"
	"remotehelpdesk/internal/repositories"
)

func TestLiveSub2APIModelList(t *testing.T) {
	if os.Getenv("RUN_SUB2API_LIVE_TEST") != "1" {
		t.Skip("set RUN_SUB2API_LIVE_TEST=1 to call the configured Sub2API")
	}
	cfg, err := config.Load("../../../config/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	config.SetCurrent(cfg)
	if _, err := bootstrap.InitDB(cfg.DB); err != nil {
		t.Fatal(err)
	}
	hostItem := repositories.SystemConfigRepository.FindByKey(sqls.DB(), "platform.sub2api.host")
	account := repositories.PlatformIAMRepository.FindSub2APIAccountByTenantID(sqls.DB(), 1)
	if hostItem == nil || account == nil {
		t.Fatal("Sub2API host or tenant account is not configured")
	}
	key, err := secretstore.Decrypt(account.DefaultKeyCiphertext)
	if err != nil {
		t.Fatal(err)
	}
	provider := providers.NewSub2APIProvider(&config.Sub2APIConfig{
		BaseURL:      strings.TrimSpace(hostItem.ConfigValue),
		Timeout:      20 * time.Second,
		MaxRetries:   1,
		RetryBackoff: time.Second,
	})
	resp, err := provider.ListModels(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	modelIDs := make([]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		modelIDs = append(modelIDs, item.ID)
	}
	sort.Strings(modelIDs)
	t.Logf("Sub2API models (%d): %s", len(modelIDs), strings.Join(modelIDs, ", "))
}
