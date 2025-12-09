package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// MakeTestSignature generates an HMAC-SHA256 signature for testing
// This is a test helper shared across multiple test files
func MakeTestSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return SignaturePrefix + hex.EncodeToString(mac.Sum(nil))
}

// makeTestSignature is the internal version for backward compatibility
func makeTestSignature(payload []byte, secret string) string {
	return MakeTestSignature(payload, secret)
}
