package security

import (
	"bytes"
	"crypto/rand"
	"strings"
	"testing"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return key
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := testKey(t)
	secret := "y0_AgAAAAA-example-oauth-token"

	enc, err := EncryptString(key, secret)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if strings.Contains(enc, secret) {
		t.Fatal("ciphertext contains plaintext")
	}
	dec, err := DecryptString(key, enc)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if dec != secret {
		t.Errorf("round trip = %q, want original", dec)
	}
}

func TestEncryptIsNonDeterministic(t *testing.T) {
	key := testKey(t)
	a, _ := EncryptString(key, "same input")
	b, _ := EncryptString(key, "same input")
	if a == b {
		t.Error("two encryptions identical — nonce not random")
	}
}

func TestDecryptRejectsWrongKeyAndGarbage(t *testing.T) {
	key := testKey(t)
	enc, _ := EncryptString(key, "secret")

	other := testKey(t)
	if !bytes.Equal(key, other) {
		if _, err := DecryptString(other, enc); err == nil {
			t.Error("wrong key accepted")
		}
	}
	if _, err := DecryptString(key, "not-base64!!!"); err == nil {
		t.Error("garbage accepted")
	}
	if _, err := DecryptString(key, "aGVsbG8="); err == nil {
		t.Error("too-short ciphertext accepted")
	}
}
