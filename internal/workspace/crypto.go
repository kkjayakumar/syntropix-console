package workspace

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

const (
	// DevOnlyFallbackKey is used only when AllowDevFallback is true (local demos).
	// Production must set WORKSPACE_SECRETS_KEY — never rely on this string.
	DevOnlyFallbackKey = "aegisops-dev-workspace-key-change-me"
	hkdfInfo           = "aegisops/workspace-secrets/v1"
)

// Cipher encrypts workspace secret values at rest with AES-256-GCM.
type Cipher struct {
	gcm cipher.AEAD
}

type Options struct {
	// AllowDevFallback permits a well-known local key when material is empty.
	// Must be false in production.
	AllowDevFallback bool
}

// NewCipher builds an AES-256-GCM cipher from WORKSPACE_SECRETS_KEY material.
// Preferred formats: 64-char hex (32 bytes) or standard base64 of 32 bytes.
// Any other non-empty string is stretched with HKDF-SHA256 (industry KDF).
func NewCipher(key string) (*Cipher, error) {
	return NewCipherWithOptions(key, Options{AllowDevFallback: true})
}

func NewCipherStrict(key string) (*Cipher, error) {
	return NewCipherWithOptions(key, Options{AllowDevFallback: false})
}

func NewCipherWithOptions(key string, opts Options) (*Cipher, error) {
	rawKey, err := DeriveKeyMaterial(strings.TrimSpace(key), opts.AllowDevFallback)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(rawKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{gcm: gcm}, nil
}

// DeriveKeyMaterial returns a 32-byte AES key.
func DeriveKeyMaterial(material string, allowDevFallback bool) ([]byte, error) {
	if material == "" {
		if !allowDevFallback {
			return nil, fmt.Errorf("WORKSPACE_SECRETS_KEY is required (openssl rand -hex 32)")
		}
		material = DevOnlyFallbackKey
	}
	if material == DevOnlyFallbackKey && !allowDevFallback {
		return nil, fmt.Errorf("WORKSPACE_SECRETS_KEY must not use the development default in production")
	}
	if b, err := hex.DecodeString(material); err == nil && len(b) == 32 {
		return b, nil
	}
	if b, err := base64.StdEncoding.DecodeString(material); err == nil && len(b) == 32 {
		return b, nil
	}
	// HKDF-SHA256: passphrase → 32-byte key (NIST-recommended KDF pattern)
	return hkdf.Key(sha256.New, []byte(material), nil, hkdfInfo, 32)
}

func (c *Cipher) Encrypt(plaintext string) (string, error) {
	if c == nil || c.gcm == nil {
		return "", fmt.Errorf("cipher not initialized")
	}
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := c.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

func (c *Cipher) Decrypt(ciphertext string) (string, error) {
	if c == nil || c.gcm == nil {
		return "", fmt.Errorf("cipher not initialized")
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	nonceSize := c.gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	plain, err := c.gcm.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// MaskSecret never reveals key prefixes (SOC2). Shows only a fixed mask + optional last-4.
func MaskSecret(value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return "••••••••"
	}
	if len(v) <= 4 {
		return "••••••••"
	}
	return "••••••••…" + v[len(v)-4:]
}

// IsInsecureDevKey reports whether the configured material is the known local default.
func IsInsecureDevKey(key string) bool {
	k := strings.TrimSpace(key)
	return k == "" || k == DevOnlyFallbackKey
}
