package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
)

var (
	mu              sync.RWMutex
	keyVersions     map[string][]byte // version → raw key bytes
	currentVersion  string
	versionedPrefix = regexp.MustCompile(`^v(\d+):(.+)$`)
)

// Init sets up the encryption key store. keys is a map of version → base64-encoded AES-256 key.
// currentKey identifies which version to use for new encryptions.
func Init(keys map[string]string, currentKey string) error {
	mu.Lock()
	defer mu.Unlock()

	kv := make(map[string][]byte, len(keys))
	for ver, b64key := range keys {
		raw, err := base64.StdEncoding.DecodeString(b64key)
		if err != nil {
			return fmt.Errorf("invalid key for version %s: %w", ver, err)
		}
		if len(raw) != 32 {
			return fmt.Errorf("key for version %s is %d bytes, expected 32 (AES-256)", ver, len(raw))
		}
		kv[ver] = raw
	}

	if len(kv) == 0 {
		return fmt.Errorf("at least one encryption key is required")
	}
	if _, ok := kv[currentKey]; !ok {
		return fmt.Errorf("current_key_version %q not found in keys", currentKey)
	}

	keyVersions = kv
	currentVersion = currentKey
	return nil
}

// Encrypt encrypts plaintext using AES-256-GCM with a random nonce.
// Returns a versioned string: "v{version}:{base64(nonce+ciphertext)}"
func Encrypt(plaintext string) (string, error) {
	mu.RLock()
	key := keyVersions[currentVersion]
	ver := currentVersion
	mu.RUnlock()

	if key == nil {
		return "", fmt.Errorf("crypto not initialized")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	return fmt.Sprintf("%s:%s", ver, encoded), nil
}

// Decrypt decrypts a versioned ciphertext produced by Encrypt.
// Returns the plaintext or an error if the key version is unknown or decryption fails.
func Decrypt(ciphertext string) (string, error) {
	matches := versionedPrefix.FindStringSubmatch(ciphertext)
	if len(matches) != 3 {
		return ciphertext, nil // plaintext / legacy data
	}

	ver := "v" + matches[1]
	encoded := matches[2]

	mu.RLock()
	key := keyVersions[ver]
	mu.RUnlock()

	if key == nil {
		return "", fmt.Errorf("unknown key version: %s", ver)
	}

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ct := raw[:nonceSize], raw[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}

// TryDecrypt attempts to decrypt; returns the original string on failure (for graceful degradation).
func TryDecrypt(s string) string {
	if !versionedPrefix.MatchString(s) {
		return s
	}
	v, err := Decrypt(s)
	if err != nil {
		return s
	}
	return v
}

// MaskPhone masks a phone number for display. 13800001111 → 138****1111
func MaskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if len(phone) < 7 {
		return strings.Repeat("*", len(phone))
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}

// IsEncrypted returns true if the string appears to be an encrypted (versioned) value.
func IsEncrypted(s string) bool {
	return versionedPrefix.MatchString(s)
}
