package secretstore

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"remotehelpdesk/internal/pkg/config"
)

const prefix = "enc:v1:"

func Fingerprint(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func Encrypt(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	block, err := aes.NewCipher(primaryKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(value), nil)
	payload := append(nonce, ciphertext...)
	return prefix + base64.StdEncoding.EncodeToString(payload), nil
}

func Decrypt(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, prefix) {
		return value, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return "", err
	}
	var decryptErr error
	for _, key := range decryptionKeys() {
		block, err := aes.NewCipher(key)
		if err != nil {
			decryptErr = err
			continue
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			decryptErr = err
			continue
		}
		if len(raw) < gcm.NonceSize() {
			return "", errors.New("secret payload is too short")
		}
		nonce := raw[:gcm.NonceSize()]
		ciphertext := raw[gcm.NonceSize():]
		plain, err := gcm.Open(nil, nonce, ciphertext, nil)
		if err == nil {
			return string(plain), nil
		}
		decryptErr = err
	}
	return "", decryptErr
}

func primaryKey() []byte {
	cfg := config.CurrentOrDefault()
	seed := strings.TrimSpace(cfg.EncryptionKey)
	if seed == "" {
		seed = legacySeed(cfg)
	}
	return keyFromSeed(seed)
}

func decryptionKeys() [][]byte {
	cfg := config.CurrentOrDefault()
	keys := [][]byte{primaryKey()}
	for _, fallback := range cfg.EncryptionKeyFallbacks {
		if fallback = strings.TrimSpace(fallback); fallback != "" {
			keys = appendUniqueKey(keys, keyFromSeed(fallback))
		}
	}
	legacy := legacySeed(cfg)
	if explicit := strings.TrimSpace(cfg.EncryptionKey); explicit != "" && legacy != "" && explicit != legacy {
		keys = appendUniqueKey(keys, keyFromSeed(legacy))
	}
	return keys
}

func appendUniqueKey(keys [][]byte, candidate []byte) [][]byte {
	for _, key := range keys {
		if bytes.Equal(key, candidate) {
			return keys
		}
	}
	return append(keys, candidate)
}

func legacySeed(cfg config.Config) string {
	seed := strings.TrimSpace(cfg.CustomerSession.Secret)
	if seed == "" {
		seed = strings.TrimSpace(cfg.WxWork.StateSecret)
	}
	if seed == "" {
		seed = strings.TrimSpace(cfg.Jitsi.AppSecret)
	}
	if seed == "" {
		return "remotehelpdesk-development-secret"
	}
	return seed
}

func keyFromSeed(seed string) []byte {
	sum := sha256.Sum256([]byte(seed))
	key := make([]byte, len(sum))
	copy(key, sum[:])
	return key
}

func MustEncrypt(value string) string {
	enc, err := Encrypt(value)
	if err != nil {
		panic(fmt.Errorf("secretstore encrypt failed: %w", err))
	}
	return enc
}
