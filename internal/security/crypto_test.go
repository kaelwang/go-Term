package security

import (
	"encoding/base64"
	"strings"
	"testing"
)

// TestDeriveKey verifies the key is a deterministic 32-byte AES-256 key.
func TestDeriveKey(t *testing.T) {
	k1 := DeriveKey("secret-a")
	k2 := DeriveKey("secret-a")
	k3 := DeriveKey("secret-b")

	if len(k1) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(k1))
	}
	if string(k1) != string(k2) {
		t.Fatal("DeriveKey is not deterministic for the same secret")
	}
	if string(k1) == string(k3) {
		t.Fatal("DeriveKey collided for different secrets")
	}
}

// TestEncryptDecryptRoundtrip verifies ciphertext is not plaintext, and that
// decryption restores the original value across inputs.
func TestEncryptDecryptRoundtrip(t *testing.T) {
	secret := "unit-test-app-secret"
	cases := []string{
		"",
		"hello world",
		"password-with-special-chars-!@#$%^&*()",
		strings.Repeat("x", 4096), // larger than one GCM block
		"{\"username\":\"alice\",\"password\":\"s3cr3t\"}",
	}
	for _, plain := range cases {
		t.Run(encodeName(plain), func(t *testing.T) {
			ct, err := Encrypt(plain, secret)
			if err != nil {
				t.Fatalf("Encrypt error: %v", err)
			}
			// Ciphertext must not leak the plaintext.
			if plain != "" && strings.Contains(ct, plain) {
				t.Fatal("ciphertext contains readable plaintext")
			}
			// Base64 should decode without error.
			raw, derr := base64.StdEncoding.DecodeString(ct)
			if derr != nil {
				t.Fatalf("ciphertext is not valid base64: %v", derr)
			}
			if len(raw) <= 12 {
				t.Fatal("ciphertext too short to carry nonce+ciphertext")
			}
			got, err := Decrypt(ct, secret)
			if err != nil {
				t.Fatalf("Decrypt error: %v", err)
			}
			if got != plain {
				t.Fatalf("roundtrip mismatch: got %q want %q", got, plain)
			}
		})
	}
}

// TestEncryptUniqueNonce verifies each encryption produces a different
// ciphertext even for identical plaintext (random nonce).
func TestEncryptUniqueNonce(t *testing.T) {
	secret := "secret"
	a, _ := Encrypt("same", secret)
	b, _ := Encrypt("same", secret)
	if a == b {
		t.Fatal("two encryptions of the same plaintext produced identical ciphertext (nonce not random)")
	}
}

// TestDecryptTamperDetection verifies that any modification of the ciphertext
// (or wrong key) is detected by GCM and rejected.
func TestDecryptTamperDetection(t *testing.T) {
	secret := "secret"
	ct, err := Encrypt("top-secret", secret)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	raw, _ := base64.StdEncoding.DecodeString(ct)
	// Flip the last byte (which is part of the GCM tag).
	raw[len(raw)-1] ^= 0xff
	tampered := base64.StdEncoding.EncodeToString(raw)

	if _, err := Decrypt(tampered, secret); err == nil {
		t.Fatal("expected error decrypting tampered ciphertext, got nil")
	}

	// Wrong key must also fail.
	if _, err := Decrypt(ct, "different-secret"); err == nil {
		t.Fatal("expected error decrypting with wrong key, got nil")
	}
}

// TestDecryptInvalidInput verifies malformed input is rejected gracefully.
func TestDecryptInvalidInput(t *testing.T) {
	cases := []string{"", "not-base64-!!!", base64.StdEncoding.EncodeToString([]byte("tooshort"))}
	for _, c := range cases {
		if _, err := Decrypt(c, "secret"); err == nil {
			t.Fatalf("expected error for input %q, got nil", c)
		}
	}
}

// encodeName produces a safe subtest name for arbitrary/plaintext inputs.
func encodeName(s string) string {
	if s == "" {
		return "empty"
	}
	if len(s) > 24 {
		return "len-" + string(rune(len(s)))
	}
	return s
}
