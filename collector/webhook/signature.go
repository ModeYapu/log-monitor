package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// GenerateSignature generates an HMAC-SHA256 signature for a webhook payload
func GenerateSignature(payload []byte, secret string) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("secret cannot be empty")
	}

	// Create HMAC-SHA256 hash
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)

	// Return hex-encoded signature
	signature := hex.EncodeToString(h.Sum(nil))
	return signature, nil
}

// VerifySignature verifies an HMAC-SHA256 signature for a webhook payload
func VerifySignature(payload []byte, secret string, signature string) bool {
	if secret == "" || signature == "" {
		return false
	}

	// Generate expected signature
	expectedSignature, err := GenerateSignature(payload, secret)
	if err != nil {
		return false
	}

	// Compare signatures using constant-time comparison
	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}

// GenerateSignatureHeader generates the signature header value
func GenerateSignatureHeader(payload []byte, secret string) string {
	signature, err := GenerateSignature(payload, secret)
	if err != nil {
		return ""
	}
	return "sha256=" + signature
}