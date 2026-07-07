package nomba

import "testing"

func TestBuildCheckoutPayloadRequestsTokenization(t *testing.T) {
	payload, err := buildCheckoutPayload(CheckoutRequest{
		OrderReference: "ref-123",
		CustomerID:     "student-1",
		CustomerEmail:  "student@example.com",
		AmountNaira:    "100.00",
		CallbackURL:    "https://example.com/callback",
		TokenizeCard:   true,
	}, "sub-account-1")
	if err != nil {
		t.Fatalf("buildCheckoutPayload returned error: %v", err)
	}

	if !payload.Order.IsTokenizedCardPayment {
		t.Fatal("expected checkout payload to request card tokenization")
	}
}

func TestEffectiveAccountIDPrefersSubAccount(t *testing.T) {
	client := &Client{parentAccountID: "parent-account", subAccountID: "sub-account"}
	if got := client.effectiveAccountID(); got != "sub-account" {
		t.Fatalf("expected sub-account ID to be used, got %q", got)
	}
}
