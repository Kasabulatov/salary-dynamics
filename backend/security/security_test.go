package security

import (
	"testing"
	"time"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("hash must not equal plaintext")
	}
	if !CheckPasswordHash("correct horse battery staple", hash) {
		t.Error("correct password rejected")
	}
	if CheckPasswordHash("wrong password", hash) {
		t.Error("wrong password accepted")
	}
}

func TestIssueAndParseToken(t *testing.T) {
	secret := []byte("test-secret")
	token, err := IssueToken(secret, 42, time.Hour)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	id, err := ParseToken(secret, token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if id != 42 {
		t.Errorf("got user ID %d, want 42", id)
	}
}

func TestParseTokenRejectsBadInput(t *testing.T) {
	secret := []byte("test-secret")

	if _, err := ParseToken(secret, "not-a-token"); err == nil {
		t.Error("garbage token accepted")
	}

	// Token signed with a different secret must be rejected.
	other, _ := IssueToken([]byte("other-secret"), 1, time.Hour)
	if _, err := ParseToken(secret, other); err == nil {
		t.Error("token with wrong signature accepted")
	}

	// Expired token must be rejected.
	expired, _ := IssueToken(secret, 1, -time.Minute)
	if _, err := ParseToken(secret, expired); err == nil {
		t.Error("expired token accepted")
	}
}

func TestNewCSRFTokenIsRandom(t *testing.T) {
	a, err := NewCSRFToken()
	if err != nil {
		t.Fatalf("NewCSRFToken: %v", err)
	}
	b, _ := NewCSRFToken()
	if a == b {
		t.Error("two CSRF tokens are identical")
	}
	if len(a) != 64 {
		t.Errorf("token length = %d, want 64 hex chars", len(a))
	}
}
