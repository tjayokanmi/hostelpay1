package nomba

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

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

// VerifySignature recreates Nomba's HMAC-SHA256 signature over the specific
// concatenated fields (per https://developer.nomba.com/docs/api-basics/webhook)
// and compares it against the signature header, base64-encoded.
func VerifySignature(payload WebhookPayload, timestamp, receivedSignature, signingKey string) bool {
	responseCode := payload.Data.Transaction.ResponseCode
	if responseCode == "null" {
		responseCode = ""
	}

	hashingPayload := fmt.Sprintf(
		"%s:%s:%s:%s:%s:%s:%s:%s:%s",
		payload.EventType,
		payload.RequestID,
		payload.Data.Merchant.UserID,
		payload.Data.Merchant.WalletID,
		payload.Data.Transaction.TransactionID,
		payload.Data.Transaction.Type,
		payload.Data.Transaction.Time,
		responseCode,
		timestamp,
	)

	return verifySignatureWithPayload(receivedSignature, signingKey, hashingPayload)
}

// VerifySignatureWithBody accepts the same webhook payload plus the raw body bytes.
// Some Nomba webhook implementations sign the raw request body as well, so we accept
// either the documented concatenated payload or the raw body bytes.
func VerifySignatureWithBody(payload WebhookPayload, body []byte, timestamp, receivedSignature, signingKey string) bool {
	if VerifySignature(payload, timestamp, receivedSignature, signingKey) {
		return true
	}
	if len(body) == 0 {
		return false
	}
	return verifySignatureWithPayload(receivedSignature, signingKey, string(body))
}

func verifySignatureWithPayload(receivedSignature, signingKey, payload string) bool {
	normalizedSignature := strings.TrimSpace(receivedSignature)
	if normalizedSignature == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(payload))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(normalizedSignature))
}

func ParseWebhookPayload(body []byte) (WebhookPayload, error) {
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return WebhookPayload{}, fmt.Errorf("failed to parse webhook payload: %w", err)
	}
	return payload, nil
}
