package crypto

import (
	"strings"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := "test-key-do-not-use-in-prod"
	cases := []string{"sk-abc123xyz", "短密钥", "a", strings.Repeat("x", 100)}
	for _, plain := range cases {
		ct, err := Encrypt(plain, key)
		if err != nil {
			t.Fatalf("encrypt %q: %v", plain, err)
		}
		got, err := Decrypt(ct, key)
		if err != nil {
			t.Fatalf("decrypt %q: %v", plain, err)
		}
		if got != plain {
			t.Errorf("round trip mismatch: got %q want %q", got, plain)
		}
	}
}

func TestEncryptEmptyStaysEmpty(t *testing.T) {
	ct, err := Encrypt("", "key")
	if err != nil || ct != "" {
		t.Errorf("empty plaintext should stay empty, got %q err=%v", ct, err)
	}
	got, err := Decrypt("", "key")
	if err != nil || got != "" {
		t.Errorf("empty ciphertext should stay empty, got %q err=%v", got, err)
	}
}

func TestSamePlaintextDifferentCiphertext(t *testing.T) {
	key := "k"
	a, _ := Encrypt("secret", key)
	b, _ := Encrypt("secret", key)
	if a == b {
		t.Error("same plaintext must produce different ciphertext (random nonce)")
	}
}

func TestWrongKeyFails(t *testing.T) {
	ct, _ := Encrypt("secret", "key-a")
	if _, err := Decrypt(ct, "key-b"); err == nil {
		t.Error("decrypt with wrong key must fail")
	}
}

func TestEmptyKeyFails(t *testing.T) {
	if _, err := Encrypt("x", ""); err == nil {
		t.Error("encrypt with empty key must fail")
	}
}

func TestMask(t *testing.T) {
	// "sk-" + a..p = 19 字符，去首尾各 4 → 11 个星
	if got := Mask("sk-abcdefghijklmnop"); got != "sk-a***********mnop" {
		t.Errorf("mask mismatch: %q", got)
	}
	if got := Mask("short"); got != "*****" {
		t.Errorf("short key should be all stars: %q", got)
	}
	if got := Mask(""); got != "" {
		t.Errorf("empty mask: %q", got)
	}
}
