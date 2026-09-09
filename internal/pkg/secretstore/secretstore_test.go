package secretstore

import (
	"testing"

	"remotehelpdesk/internal/pkg/config"
)

func TestEncryptionKeyIsIndependentFromCustomerSessionSecret(t *testing.T) {
	previous := config.CurrentOrDefault()
	t.Cleanup(func() { config.SetCurrent(&previous) })

	config.SetCurrent(&config.Config{
		EncryptionKey:   "new-encryption-key-0123456789abcdef",
		CustomerSession: config.CustomerSessionConfig{Secret: "customer-session-secret-0123456789"},
	})
	ciphertext, err := Encrypt("provider-api-key")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	config.SetCurrent(&config.Config{
		EncryptionKey:   "new-encryption-key-0123456789abcdef",
		CustomerSession: config.CustomerSessionConfig{Secret: "rotated-customer-secret-0123456789"},
	})
	plain, err := Decrypt(ciphertext)
	if err != nil || plain != "provider-api-key" {
		t.Fatalf("Decrypt() = %q, %v", plain, err)
	}
}

func TestDecryptFallsBackToLegacySessionDerivedKey(t *testing.T) {
	previous := config.CurrentOrDefault()
	t.Cleanup(func() { config.SetCurrent(&previous) })

	legacySecret := "legacy-customer-session-secret-012345"
	config.SetCurrent(&config.Config{CustomerSession: config.CustomerSessionConfig{Secret: legacySecret}})
	legacyCiphertext, err := Encrypt("legacy-connector-secret")
	if err != nil {
		t.Fatalf("Encrypt legacy value: %v", err)
	}

	config.SetCurrent(&config.Config{
		EncryptionKey:   "new-encryption-key-0123456789abcdef",
		CustomerSession: config.CustomerSessionConfig{Secret: legacySecret},
	})
	plain, err := Decrypt(legacyCiphertext)
	if err != nil || plain != "legacy-connector-secret" {
		t.Fatalf("legacy Decrypt() = %q, %v", plain, err)
	}
}

func TestDecryptUsesConfiguredFallbackKey(t *testing.T) {
	previous := config.CurrentOrDefault()
	t.Cleanup(func() { config.SetCurrent(&previous) })

	legacySecret := "local-dev-secret"
	config.SetCurrent(&config.Config{CustomerSession: config.CustomerSessionConfig{Secret: legacySecret}})
	legacyCiphertext, err := Encrypt("tenant-sub2api-secret")
	if err != nil {
		t.Fatalf("Encrypt legacy value: %v", err)
	}

	config.SetCurrent(&config.Config{
		EncryptionKey:          "rotated-production-key-0123456789abcdef",
		EncryptionKeyFallbacks: []string{"unused-fallback", legacySecret},
		CustomerSession:        config.CustomerSessionConfig{Secret: "production-session-secret-0123456789"},
	})
	plain, err := Decrypt(legacyCiphertext)
	if err != nil || plain != "tenant-sub2api-secret" {
		t.Fatalf("fallback Decrypt() = %q, %v", plain, err)
	}
}
