package server

import (
	"testing"
)

func TestVerifySignature_Valid(t *testing.T) {
	payload := []byte(`{"ref":"refs/heads/main"}`)
	secret := "test-secret-at-least-32-chars-long-here"
	signature := makeTestSignature(payload, secret)

	if !VerifySignature(payload, signature, secret) {
		t.Error("Expected valid signature to be accepted")
	}
}

func TestVerifySignature_Invalid(t *testing.T) {
	payload := []byte(`{"ref":"refs/heads/main"}`)
	secret := "test-secret-at-least-32-chars-long-here"
	wrongSecret := "wrong-secret-at-least-32-chars-long-x"
	signature := makeTestSignature(payload, wrongSecret)

	if VerifySignature(payload, signature, secret) {
		t.Error("Expected invalid signature to be rejected")
	}
}

func TestVerifySignature_MissingHeader(t *testing.T) {
	payload := []byte(`{"ref":"refs/heads/main"}`)
	secret := "test-secret-at-least-32-chars-long-here"

	if VerifySignature(payload, "", secret) {
		t.Error("Expected missing signature to be rejected")
	}
}

func TestVerifySignature_MalformedSignature(t *testing.T) {
	payload := []byte(`{"ref":"refs/heads/main"}`)
	secret := "test-secret-at-least-32-chars-long-here"

	testCases := []struct {
		name      string
		signature string
	}{
		{"no prefix", "abc123def456"},
		{"wrong prefix", "sha1=abc123def456"},
		{"no equals", "sha256abc123def456"},
		{"empty after prefix", "sha256="},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if VerifySignature(payload, tc.signature, secret) {
				t.Errorf("Expected malformed signature '%s' to be rejected", tc.signature)
			}
		})
	}
}
