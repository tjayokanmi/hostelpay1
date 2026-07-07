package nomba

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
// and compares it against the nomba-signature header, base64-encoded.
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

	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(hashingPayload))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(receivedSignature))
}

func ParseWebhookPayload(body []byte) (WebhookPayload, error) {
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return WebhookPayload{}, fmt.Errorf("failed to parse webhook payload: %w", err)
	}
	return payload, nil
}
