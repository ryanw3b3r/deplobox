package security

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math"
	"strings"
)

const (
	// MinSecretLength is the minimum allowed length for webhook secrets.
	// Increased from 32 to 48 for better security.
	MinSecretLength = 48

	// MinEntropy is the minimum Shannon entropy threshold for secrets.
	// This ensures secrets have sufficient randomness.
	MinEntropy = 3.5
)

var forbiddenSecrets = map[string]bool{
	"replace-with-secret":                          true,
	"replace-with-secret-must-be-at-least-32-chars-long": true,
	"another-secret-must-be-at-least-32-chars-long":     true,
	"github-webhook-password":                           true,
	"topsecret":                                         true,
	"secret":                                            true,
	"password":                                          true,
	"changeme":                                          true,
	"your-webhook-secret-min-32-chars-long":            true,
	"min-32-char-webhook-secret":                       true,
}

// ValidateSecret ensures webhook secret meets security requirements.
// Checks:
// - Minimum length (48 characters)
// - Not a placeholder value
// - Sufficient Shannon entropy (minimum 3.5)
func ValidateSecret(secret string) error {
	if len(secret) < MinSecretLength {
		return fmt.Errorf("secret too short (minimum %d characters, got %d)", MinSecretLength, len(secret))
	}

	// Check against forbidden list (case-insensitive)
	secretLower := strings.ToLower(secret)
	if forbiddenSecrets[secretLower] {
		return fmt.Errorf("secret appears to be a placeholder value, please use a real secret")
	}

	// Check for common placeholder patterns
	if strings.Contains(secretLower, "replace") ||
	   strings.Contains(secretLower, "changeme") ||
	   strings.Contains(secretLower, "topsecret") ||
	   strings.Contains(secretLower, "password") {
		return fmt.Errorf("secret appears to be a placeholder value")
	}

	// Calculate Shannon entropy
	entropy := calculateEntropy(secret)
	if entropy < MinEntropy {
		return fmt.Errorf("secret has insufficient entropy (%.2f < %.2f) - use a more random secret", entropy, MinEntropy)
	}

	return nil
}

// GenerateSecret creates a cryptographically secure random secret.
// Returns a 48-character base64-encoded string.
func GenerateSecret() (string, error) {
	// Generate 36 bytes which will encode to 48 characters in base64
	bytes := make([]byte, 36)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random secret: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// calculateEntropy computes the Shannon entropy of a string.
// Higher entropy indicates more randomness/unpredictability.
// Returns a value between 0 (completely predictable) and ~8 (maximum entropy for byte strings).
func calculateEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}

	// Count frequency of each character
	freq := make(map[rune]int)
	for _, c := range s {
		freq[c]++
	}

	// Calculate Shannon entropy: H = -Σ(p(x) * log2(p(x)))
	var entropy float64
	length := float64(len(s))

	for _, count := range freq {
		p := float64(count) / length
		entropy -= p * math.Log2(p)
	}

	return entropy
}

// IsWeakSecret performs a quick check if a secret is obviously weak.
// This can be used for warning messages without failing validation.
func IsWeakSecret(secret string) bool {
	// Too short
	if len(secret) < 32 {
		return true
	}

	// All same character
	if len(strings.Trim(secret, string(secret[0]))) == 0 {
		return true
	}

	// Sequential characters (e.g., "12345678...")
	if isSequential(secret) {
		return true
	}

	// Low entropy
	if calculateEntropy(secret) < 2.5 {
		return true
	}

	return false
}

// isSequential checks if a string consists of sequential characters.
func isSequential(s string) bool {
	if len(s) < 4 {
		return false
	}

	sequential := 0
	for i := 1; i < len(s); i++ {
		if s[i] == s[i-1]+1 || s[i] == s[i-1]-1 {
			sequential++
		}
	}

	// If more than 70% of characters are sequential, it's weak
	return float64(sequential) > float64(len(s))*0.7
}
