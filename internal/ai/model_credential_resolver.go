package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mlogclub/simple/sqls"

	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/secretstore"
	"remotehelpdesk/internal/repositories"
)

type resolvedSub2APICredential struct {
	APIKey   string
	APIKeyID string
	Scope    string
	Fallback bool
}

const (
	ModelCredentialScopePlatformTranslation = "platform_translation"
	platformSub2APITranslationKeyConfigKey  = "platform.sub2api.translation_api_key"
)

type platformSecretValue struct {
	Ciphertext  string `json:"ciphertext"`
	Fingerprint string `json:"fingerprint"`
}

type modelCredentialSourceResolver func(CapabilityScope) (resolvedSub2APICredential, bool, error)

// The ordered resolver is provider-agnostic. Adding a new credential source
// only requires registering its resolver and exposing it in the workflow DSL;
// the selection and fallback algorithm does not change.
var sub2APICredentialSources = map[string]modelCredentialSourceResolver{
	dsl.ModelCredentialScopeProduct:         resolveProductSub2APICredential,
	dsl.ModelCredentialScopeTenantDefault:   resolveTenantDefaultSub2APICredential,
	ModelCredentialScopePlatformTranslation: resolvePlatformTranslationSub2APICredential,
}

func resolveSub2APICredential(scope CapabilityScope) (resolvedSub2APICredential, error) {
	chain := append([]string(nil), scope.CredentialChain...)
	if len(chain) == 0 {
		chain = dsl.DefaultModelCredentialChain(scope.ProductID)
	}
	seen := make(map[string]struct{}, len(chain))
	for index, rawSource := range chain {
		source := strings.TrimSpace(rawSource)
		resolver, exists := sub2APICredentialSources[source]
		if !exists {
			return resolvedSub2APICredential{}, fmt.Errorf("unsupported model credential source: %s", source)
		}
		if _, repeated := seen[source]; repeated {
			return resolvedSub2APICredential{}, fmt.Errorf("model credential source is repeated: %s", source)
		}
		seen[source] = struct{}{}
		credential, available, err := resolver(scope)
		if err != nil {
			return resolvedSub2APICredential{}, fmt.Errorf("resolve %s model credential: %w", source, err)
		}
		if !available {
			continue
		}
		credential.Scope = source
		credential.Fallback = index > 0
		return credential, nil
	}
	return resolvedSub2APICredential{}, nil
}

func resolveProductSub2APICredential(scope CapabilityScope) (resolvedSub2APICredential, bool, error) {
	if scope.TenantID <= 0 || scope.ProductID <= 0 {
		return resolvedSub2APICredential{}, false, nil
	}
	credential := repositories.ProductAIUsageCredentialRepository.GetByProduct(sqls.DB(), scope.TenantID, scope.ProductID)
	if credential == nil || credential.Status != enums.StatusOk || !strings.EqualFold(strings.TrimSpace(credential.ProvisionStatus), "ready") || strings.TrimSpace(credential.Sub2APIKeyID) == "" || strings.TrimSpace(credential.APIKeyRef) == "" {
		return resolvedSub2APICredential{}, false, nil
	}
	plain, err := secretstore.Decrypt(credential.APIKeyRef)
	if err != nil {
		return resolvedSub2APICredential{}, false, err
	}
	plain = strings.TrimSpace(plain)
	if plain == "" || strings.HasPrefix(plain, "secret://") {
		return resolvedSub2APICredential{}, false, nil
	}
	return resolvedSub2APICredential{
		APIKey:   plain,
		APIKeyID: strings.TrimSpace(credential.Sub2APIKeyID),
	}, true, nil
}

func resolveTenantDefaultSub2APICredential(scope CapabilityScope) (resolvedSub2APICredential, bool, error) {
	if scope.TenantID > 0 {
		account := repositories.PlatformIAMRepository.FindSub2APIAccountByTenantID(sqls.DB(), scope.TenantID)
		if account == nil || account.Status != enums.StatusOk || !strings.EqualFold(strings.TrimSpace(account.ProvisionStatus), "active") || !strings.EqualFold(strings.TrimSpace(account.DefaultKeyStatus), "active") || strings.TrimSpace(account.DefaultKeyID) == "" || strings.TrimSpace(account.DefaultKeyCiphertext) == "" {
			return resolvedSub2APICredential{}, false, nil
		}
		plain, err := secretstore.Decrypt(account.DefaultKeyCiphertext)
		if err != nil {
			return resolvedSub2APICredential{}, false, err
		}
		plain = strings.TrimSpace(plain)
		if plain == "" {
			return resolvedSub2APICredential{}, false, nil
		}
		return resolvedSub2APICredential{
			APIKey:   plain,
			APIKeyID: strings.TrimSpace(account.DefaultKeyID),
		}, true, nil
	}
	if value := strings.TrimSpace(config.CurrentOrDefault().Sub2API.DefaultKey); value != "" {
		return resolvedSub2APICredential{APIKey: value}, true, nil
	}
	value := strings.TrimSpace(os.Getenv(sub2APIDefaultKeyEnv))
	return resolvedSub2APICredential{APIKey: value}, value != "", nil
}

func resolvePlatformTranslationSub2APICredential(CapabilityScope) (resolvedSub2APICredential, bool, error) {
	item := repositories.SystemConfigRepository.FindByKey(sqls.DB(), platformSub2APITranslationKeyConfigKey)
	if item == nil || strings.TrimSpace(item.ConfigValue) == "" {
		return resolvedSub2APICredential{}, false, nil
	}
	apiKey, fingerprint, err := decryptPlatformCredentialValue(item.ConfigValue)
	if err != nil {
		return resolvedSub2APICredential{}, false, err
	}
	if apiKey == "" {
		return resolvedSub2APICredential{}, false, nil
	}
	if fingerprint == "" {
		fingerprint = secretstore.Fingerprint(apiKey)
	}
	return resolvedSub2APICredential{
		APIKey:   apiKey,
		APIKeyID: platformTranslationAPIKeyID(fingerprint),
	}, true, nil
}

func decryptPlatformCredentialValue(value string) (string, string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", nil
	}
	payload := platformSecretValue{}
	if err := json.Unmarshal([]byte(value), &payload); err == nil && strings.TrimSpace(payload.Ciphertext) != "" {
		plain, err := secretstore.Decrypt(payload.Ciphertext)
		if err != nil {
			return "", "", err
		}
		return strings.TrimSpace(plain), strings.TrimSpace(payload.Fingerprint), nil
	}
	plain, err := secretstore.Decrypt(value)
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(plain), "", nil
}

func platformTranslationAPIKeyID(fingerprint string) string {
	fingerprint = strings.TrimSpace(fingerprint)
	if fingerprint == "" {
		return "platform_translation"
	}
	return "platform_translation:" + fingerprint
}
