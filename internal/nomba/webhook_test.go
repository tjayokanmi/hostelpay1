package nomba

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestVerifySignatureAcceptsBase64HMAC(t *testing.T) {
	payload := WebhookPayload{
		EventType: "payment_success",
		RequestID: "req-1",
		Data: struct {
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
		}{},
	}
	payload.Data.Merchant.UserID = "user-1"
	payload.Data.Merchant.WalletID = "wallet-1"
	payload.Data.Transaction.TransactionID = "tx-1"
	payload.Data.Transaction.Type = "online_checkout"
	payload.Data.Transaction.Time = "2026-01-01T00:00:00Z"
	payload.Data.Transaction.ResponseCode = ""

	secret := "super-secret"
	timestamp := "2026-02-01T00:00:00Z"
	msg := "payment_success:req-1:user-1:wallet-1:tx-1:online_checkout:2026-01-01T00:00:00Z::2026-02-01T00:00:00Z"
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if !VerifySignature(payload, timestamp, sig, secret) {
		t.Fatal("expected VerifySignature to accept a base64-encoded HMAC-SHA256 signature")
	}
}
