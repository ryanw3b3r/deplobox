package security

import (
	"strings"
	"testing"
)

func TestValidateSecret(t *testing.T) {
	tests := []struct {
		name    string
		secret  string
		wantErr bool
	}{
		// Valid secrets (48+ chars, good entropy)
		{
			"strong random secret",
			"kJ8mN2pQ5tR7vX1zB4cE6gH9jL3nP8qS2uW5yA7bD0fG3hK6",
			false,
		},
		{
			"base64-like secret",
			"dGhpcyBpcyBhIHZlcnkgbG9uZyBzZWNyZXQgd2l0aCBnb29kIGVudHJvcHk=",
			false,
		},
		{
			"mixed characters",
			"MySecretKey123!@#WithGoodLength&EntropyMixedChars456",
			false,
		},

		// Too short
		{
			"too short 32 chars",
			"12345678901234567890123456789012",
			true,
		},
		{
			"too short 40 chars",
			"1234567890123456789012345678901234567890",
			true,
		},
		{
			"empty string",
			"",
			true,
		},

		// Forbidden placeholder values
		{
			"replace-with-secret",
			"replace-with-secret",
			true,
		},
		{
			"replace-with-secret (long enough)",
			"replace-with-secret-must-be-at-least-32-chars-long",
			true,
		},
		{
			"another placeholder",
			"another-secret-must-be-at-least-32-chars-long",
			true,
		},
		{
			"password placeholder",
			"password",
			true,
		},
		{
			"changeme placeholder",
			"changeme",
			true,
		},
		{
			"secret placeholder",
			"secret",
			true,
		},
		{
			"contains replace",
			"please-replace-this-with-a-real-secret-that-is-secure",
			true,
		},
		{
			"contains changeme",
			"changeme-to-something-secure-and-random-please-now",
			true,
		},

		// Low entropy (even if long enough)
		{
			"all same character",
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			true,
		},
		{
			"repeated pattern",
			"abcabcabcabcabcabcabcabcabcabcabcabcabcabcabcabc",
			true,
		},
		{
			"sequential numbers",
			"123456789012345678901234567890123456789012345678",
			true,
		},
		{
			"sequential letters",
			"abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvw",
			false, // Has good entropy (4.7) despite being sequential - 26 unique chars
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSecret(tt.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSecret() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateSecret(t *testing.T) {
	// Test that generated secrets are valid
	for i := 0; i < 10; i++ {
		secret, err := GenerateSecret()
		if err != nil {
			t.Fatalf("GenerateSecret() error = %v", err)
		}

		// Check length (should be 48 chars)
		if len(secret) != 48 {
			t.Errorf("GenerateSecret() length = %d, want 48", len(secret))
		}

		// Check that it passes validation
		if err := ValidateSecret(secret); err != nil {
			t.Errorf("Generated secret failed validation: %v (secret: %s)", err, secret)
		}

		// Check entropy is good
		entropy := calculateEntropy(secret)
		if entropy < MinEntropy {
			t.Errorf("Generated secret has low entropy: %.2f < %.2f", entropy, MinEntropy)
		}
	}

	// Test that generated secrets are unique
	secrets := make(map[string]bool)
	for i := 0; i < 100; i++ {
		secret, err := GenerateSecret()
		if err != nil {
			t.Fatalf("GenerateSecret() error = %v", err)
		}
		if secrets[secret] {
			t.Errorf("GenerateSecret() generated duplicate secret")
		}
		secrets[secret] = true
	}
}

func TestCalculateEntropy(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		minExpected float64
		maxExpected float64
	}{
		{
			"empty string",
			"",
			0.0,
			0.0,
		},
		{
			"single character repeated",
			"aaaaaaa",
			0.0,
			0.0,
		},
		{
			"two characters alternating",
			"ababababab",
			1.0,
			1.0,
		},
		{
			"all unique characters",
			"abcdefghij",
			3.0,
			4.0, // log2(10) ≈ 3.32
		},
		{
			"random-looking string",
			"kJ8mN2pQ5tR7vX1zB4cE6gH9jL3nP8qS",
			4.0,
			6.0,
		},
		{
			"base64 string",
			"dGhpcyBpcyBhIHRlc3Q=",
			3.5,
			5.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entropy := calculateEntropy(tt.input)
			if entropy < tt.minExpected || entropy > tt.maxExpected {
				t.Errorf("calculateEntropy(%q) = %.2f, want between %.2f and %.2f",
					tt.input, entropy, tt.minExpected, tt.maxExpected)
			}
		})
	}
}

func TestIsWeakSecret(t *testing.T) {
	tests := []struct {
		name   string
		secret string
		want   bool
	}{
		// Weak secrets
		{"too short", "short", true},
		{"all same character", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true},
		{"sequential numbers", "12345678901234567890123456789012", true},
		{"sequential letters", "abcdefghijklmnopqrstuvwxyzabcdef", true},
		{"low entropy repeated", "abcabcabcabcabcabcabcabcabcabcab", true},

		// Strong secrets
		{
			"strong random",
			"kJ8mN2pQ5tR7vX1zB4cE6gH9jL3nP8qS2uW5yA7bD0fG3hK6",
			false,
		},
		{
			"good mixed",
			"MySecretKey123WithGoodEntropyAndLength456",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsWeakSecret(tt.secret)
			if got != tt.want {
				t.Errorf("IsWeakSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSequential(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"sequential ascending", "123456789", true},
		{"sequential descending", "987654321", true},
		{"sequential letters", "abcdefghij", true},
		{"mixed sequential", "12345abcde", true},
		{"non-sequential", "1a2b3c4d5e", false},
		{"random", "kJ8mN2pQ5t", false},
		{"too short", "123", false},
		{"repeated", "11111111", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSequential(tt.input)
			if got != tt.want {
				t.Errorf("isSequential() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSecretValidationEdgeCases(t *testing.T) {
	// Test minimum length boundary
	t.Run("exactly 48 chars with good entropy", func(t *testing.T) {
		secret := "abcdefghij1234567890ABCDEFGHIJ!@#$%^&*()KLMNOPQR"
		if len(secret) != 48 {
			t.Fatalf("Test secret has wrong length: %d", len(secret))
		}
		err := ValidateSecret(secret)
		if err != nil {
			t.Errorf("ValidateSecret() with exactly 48 chars failed: %v", err)
		}
	})

	// Test 47 chars (just under minimum)
	t.Run("47 chars should fail", func(t *testing.T) {
		secret := "abcdefghij1234567890ABCDEFGHIJ!@#$%^&*()KLMNOPQ"
		if len(secret) != 47 {
			t.Fatalf("Test secret has wrong length: %d", len(secret))
		}
		err := ValidateSecret(secret)
		if err == nil {
			t.Errorf("ValidateSecret() with 47 chars should fail")
		}
	})

	// Test case-insensitive forbidden check
	t.Run("forbidden secret uppercase", func(t *testing.T) {
		err := ValidateSecret("REPLACE-WITH-SECRET")
		if err == nil {
			t.Errorf("ValidateSecret() should reject uppercase forbidden secret")
		}
	})

	// Test very long secret with low entropy
	t.Run("very long but low entropy", func(t *testing.T) {
		secret := strings.Repeat("ab", 100) // 200 chars but low entropy
		err := ValidateSecret(secret)
		if err == nil {
			t.Errorf("ValidateSecret() should reject long secret with low entropy")
		}
	})
}

// Benchmark tests
func BenchmarkValidateSecret(b *testing.B) {
	secret := "kJ8mN2pQ5tR7vX1zB4cE6gH9jL3nP8qS2uW5yA7bD0fG3hK6"
	for i := 0; i < b.N; i++ {
		_ = ValidateSecret(secret)
	}
}

func BenchmarkGenerateSecret(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = GenerateSecret()
	}
}

func BenchmarkCalculateEntropy(b *testing.B) {
	secret := "kJ8mN2pQ5tR7vX1zB4cE6gH9jL3nP8qS2uW5yA7bD0fG3hK6"
	for i := 0; i < b.N; i++ {
		_ = calculateEntropy(secret)
	}
}
