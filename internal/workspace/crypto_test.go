package workspace

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	c, err := NewCipher("demo-passphrase-for-tests-only")
	if err != nil {
		t.Fatal(err)
	}
	ct, err := c.Encrypt("sk-test-secret-value")
	if err != nil {
		t.Fatal(err)
	}
	pt, err := c.Decrypt(ct)
	if err != nil {
		t.Fatal(err)
	}
	if pt != "sk-test-secret-value" {
		t.Fatalf("got %q", pt)
	}
}

func TestHexKey(t *testing.T) {
	hexKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	c, err := NewCipherStrict(hexKey)
	if err != nil {
		t.Fatal(err)
	}
	ct, _ := c.Encrypt("hello")
	pt, err := c.Decrypt(ct)
	if err != nil || pt != "hello" {
		t.Fatalf("hex key roundtrip failed: %v %q", err, pt)
	}
}

func TestStrictRejectsEmpty(t *testing.T) {
	if _, err := NewCipherStrict(""); err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestStrictRejectsDevDefault(t *testing.T) {
	if _, err := NewCipherStrict(DevOnlyFallbackKey); err == nil {
		t.Fatal("expected error for dev default in strict mode")
	}
}

func TestTamperFails(t *testing.T) {
	c, _ := NewCipher("demo-passphrase-for-tests")
	ct, _ := c.Encrypt("secret")
	raw, err := base64.StdEncoding.DecodeString(ct)
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 0xff
	bad := base64.StdEncoding.EncodeToString(raw)
	if _, err := c.Decrypt(bad); err == nil {
		t.Fatal("expected decrypt failure on tamper")
	}
}

func TestMaskSecretNoPrefix(t *testing.T) {
	m := MaskSecret("sk-abcdefghijklmnopqrstuvwxyz")
	if strings.Contains(m, "sk-a") || strings.HasPrefix(m, "sk-") {
		t.Fatalf("mask leaked prefix: %s", m)
	}
	if !strings.HasSuffix(m, "wxyz") {
		t.Fatalf("expected last-4: %s", m)
	}
}
