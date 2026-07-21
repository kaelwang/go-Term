package security

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TestGenerateAndParseToken verifies a freshly issued token round-trips and
// carries the expected subject/user claim.
func TestGenerateAndParseToken(t *testing.T) {
	secret := "jwt-unit-secret"
	token, err := GenerateToken("alice", secret, 60)
	if err != nil {
		t.Fatalf("GenerateToken error: %v", err)
	}
	if token == "" {
		t.Fatal("GenerateToken returned empty token")
	}
	claims, err := ParseToken(token, secret)
	if err != nil {
		t.Fatalf("ParseToken error: %v", err)
	}
	if claims.User != "alice" {
		t.Fatalf("expected user 'alice', got %q", claims.User)
	}
	if claims.Subject != "alice" {
		t.Fatalf("expected subject 'alice', got %q", claims.Subject)
	}
	if claims.Issuer != "go-Term" {
		t.Fatalf("expected issuer 'go-Term', got %q", claims.Issuer)
	}
	if claims.ExpiresAt == nil || claims.ExpiresAt.Before(time.Now()) {
		t.Fatal("expected a future expiry")
	}
}

// TestGenerateTokenDefaultExpiry verifies a non-positive expiry falls back to
// the 24h default instead of producing an already-expired token.
func TestGenerateTokenDefaultExpiry(t *testing.T) {
	secret := "s"
	token, err := GenerateToken("bob", secret, 0)
	if err != nil {
		t.Fatalf("GenerateToken error: %v", err)
	}
	claims, err := ParseToken(token, secret)
	if err != nil {
		t.Fatalf("ParseToken error: %v", err)
	}
	if claims.ExpiresAt.Before(time.Now()) {
		t.Fatal("default expiry produced an already-expired token")
	}
}

// TestParseExpiredToken verifies an expired token is rejected.
func TestParseExpiredToken(t *testing.T) {
	secret := "s"
	// Build an expired token manually to guarantee the expiry condition.
	claims := Claims{
		User: "carol",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "carol",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign error: %v", err)
	}
	if _, err := ParseToken(signed, secret); err == nil {
		t.Fatal("expected error parsing expired token, got nil")
	}
}

// TestParseWrongSecret verifies a token signed with a different secret is
// rejected (signature verification failure).
func TestParseWrongSecret(t *testing.T) {
	token, _ := GenerateToken("dave", "right-secret", 60)
	if _, err := ParseToken(token, "wrong-secret"); err == nil {
		t.Fatal("expected error parsing token signed with wrong secret, got nil")
	}
}

// TestParseGarbage verifies malformed tokens are rejected.
func TestParseGarbage(t *testing.T) {
	cases := []string{"", "not.a.jwt", "eyJhbGciOiJIUzI1NiJ9.invalid", "header.payload"}
	for _, c := range cases {
		if _, err := ParseToken(c, "secret"); err == nil {
			t.Fatalf("expected error parsing %q, got nil", c)
		}
	}
}
