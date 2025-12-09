package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const (
	SignaturePrefix = "sha256="
)

// VerifySignature verifies the HMAC-SHA256 signature from GitHub webhook
func VerifySignature(payload []byte, signature, secret string) bool {
	// Signature must be present
	if signature == "" {
		return false
	}

	// Signature format: "sha256=<hex_digest>"
	if !strings.HasPrefix(signature, SignaturePrefix) {
		return false
	}

	// Extract the hex digest by removing prefix
	receivedMAC := strings.TrimPrefix(signature, SignaturePrefix)

	// Compute expected HMAC
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	// Constant-time comparison to prevent timing attacks
	return hmac.Equal([]byte(expectedMAC), []byte(receivedMAC))
}
