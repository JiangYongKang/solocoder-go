package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

const (
	SignatureHeader = "X-Webhook-Signature"
	TimestampHeader = "X-Webhook-Timestamp"
)

func GenerateHMACSHA256(secret string, payload []byte, timestamp string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	if timestamp != "" {
		mac.Write([]byte(timestamp))
		mac.Write([]byte("."))
	}
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func VerifyHMACSHA256(secret string, payload []byte, timestamp string, signature string) bool {
	if len(signature) < 7 || signature[:7] != "sha256=" {
		return false
	}
	expected := GenerateHMACSHA256(secret, payload, timestamp)
	return hmac.Equal([]byte(expected), []byte(signature))
}
