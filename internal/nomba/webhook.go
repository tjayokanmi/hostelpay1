package nomba

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// WebhookPayload mirrors the payment notification shape forwarded by the
// hackathon's webhook relay.
type WebhookPayload struct {
	EventType string `json:"event_type"`
	RequestID string `json:"requestId"`
	Data      struct {
		Merchant struct {
			UserID   string `json:"userId"`
			WalletID string `json:"walletId"`
		} `json:"merchant"`
		Transaction struct {
			TransactionID string `json:"transactionId"`
			Type          string `json:"type"`
			Time          string `json:"time"`
			ResponseCode  string `json:"responseCode"`
		} `json:"transaction"`
		Order struct {
			OrderReference string `json:"orderReference"`
		} `json:"order"`
	} `json:"data"`
}

// VerifySignature computes HMAC-SHA256 over the RAW request body (not a
// reconstructed field string) using the hackathon-provided signing key,
// hex-encodes it, and compares it against the "nomba-signature" header
// using a constant-time comparison to prevent timing attacks.
func VerifySignature(rawBody []byte, receivedSignature, signingKey string) bool {
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write(rawBody)
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(receivedSignature))
}

func ParseWebhookPayload(body []byte) (WebhookPayload, error) {
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return WebhookPayload{}, fmt.Errorf("failed to parse webhook payload: %w", err)
	}
	return payload, nil
}
