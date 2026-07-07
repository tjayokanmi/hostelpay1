package nomba

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type Environment string

const (
	EnvSandbox    Environment = "sandbox"
	EnvProduction Environment = "production"
)

func baseURLFor(env Environment) string {
	if env == EnvProduction {
		return "https://api.nomba.com/v1"
	}
	return "https://sandbox.nomba.com/v1"
}

// Client handles all communication with the Nomba API.
type Client struct {
	env             Environment
	baseURL         string
	clientID        string
	clientSecret    string
	parentAccountID string
	subAccountID    string
	httpClient      *http.Client

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

func generateIdempotencyKey() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate idempotency key: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func NewClient(env Environment, clientID, clientSecret, parentAccountID, subAccountID string) *Client {
	return &Client{
		env:             env,
		baseURL:         baseURLFor(env),
		clientID:        clientID,
		clientSecret:    clientSecret,
		parentAccountID: parentAccountID,
		subAccountID:    subAccountID,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) effectiveAccountID() string {
	if c.subAccountID != "" {
		return c.subAccountID
	}
	return c.parentAccountID
}

// Environment exposes which mode this client is running in, so callers
// (e.g. pricing logic) can branch on it without duplicating the concept.
func (c *Client) Environment() Environment {
	return c.env
}

type tokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type tokenResponse struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Data        struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		BusinessID   string `json:"businessId"`
		ExpiresAt    string `json:"expiresAt"`
	} `json:"data"`
}

func (c *Client) getAccessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		token := c.accessToken
		c.mu.Unlock()
		return token, nil
	}
	c.mu.Unlock()

	body, err := json.Marshal(tokenRequest{
		GrantType:    "client_credentials",
		ClientID:     c.clientID,
		ClientSecret: c.clientSecret,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal token request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/auth/token/issue", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("accountId", c.effectiveAccountID())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("nomba auth request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read nomba auth response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("nomba auth returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	if tokenResp.Data.AccessToken == "" {
		return "", fmt.Errorf("nomba auth returned an empty access token (raw: %s)", string(respBody))
	}

	expiresAt, err := time.Parse(time.RFC3339, tokenResp.Data.ExpiresAt)
	if err != nil {
		return "", fmt.Errorf("failed to parse token expiry %q: %w", tokenResp.Data.ExpiresAt, err)
	}

	c.mu.Lock()
	c.accessToken = tokenResp.Data.AccessToken
	c.tokenExpiry = expiresAt.Add(-30 * time.Second)
	c.mu.Unlock()

	return tokenResp.Data.AccessToken, nil
}

type CheckoutRequest struct {
	OrderReference string
	CustomerID     string
	CustomerEmail  string
	AmountNaira    string
	CallbackURL    string
	TokenizeCard   bool
}

type nombaOrder struct {
	CallbackURL            string `json:"callbackUrl"`
	CustomerEmail          string `json:"customerEmail"`
	Amount                 string `json:"amount"`
	Currency               string `json:"currency"`
	OrderReference         string `json:"orderReference"`
	CustomerID             string `json:"customerId"`
	AccountID              string `json:"accountId"`
	IsTokenizedCardPayment bool   `json:"isTokenizedCardPayment"`
}

type nombaCheckoutPayload struct {
	Order        nombaOrder `json:"order"`
	TokenizeCard bool       `json:"tokenizeCard"`
}

type CheckoutResponse struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Data        struct {
		CheckoutLink   string `json:"checkoutLink"`
		OrderReference string `json:"orderReference"`
	} `json:"data"`
}

type CheckoutResult struct {
	CheckoutLink      string
	ProviderReference string
}

func buildCheckoutPayload(req CheckoutRequest, subAccountID string) (nombaCheckoutPayload, error) {
	payload := nombaCheckoutPayload{
		Order: nombaOrder{
			CallbackURL:            req.CallbackURL,
			CustomerEmail:          req.CustomerEmail,
			Amount:                 req.AmountNaira,
			Currency:               "NGN",
			OrderReference:         req.OrderReference,
			CustomerID:             req.CustomerID,
			AccountID:              subAccountID,
			IsTokenizedCardPayment: req.TokenizeCard,
		},
		TokenizeCard: req.TokenizeCard,
	}

	return payload, nil
}

func (c *Client) GenerateCheckoutLink(ctx context.Context, req CheckoutRequest) (CheckoutResult, error) {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return CheckoutResult{}, fmt.Errorf("failed to obtain access token: %w", err)
	}

	payload, err := buildCheckoutPayload(req, c.subAccountID)
	if err != nil {
		return CheckoutResult{}, err
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return CheckoutResult{}, fmt.Errorf("failed to marshal nomba request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/checkout/order", bytes.NewReader(payloadBytes))
	if err != nil {
		return CheckoutResult{}, fmt.Errorf("failed to create checkout request: %w", err)
	}

	idempotencyKey, err := generateIdempotencyKey()
	if err != nil {
		return CheckoutResult{}, fmt.Errorf("failed to generate idempotency key: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("accountId", c.effectiveAccountID())
	httpReq.Header.Set("X-Idempotent-key", idempotencyKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return CheckoutResult{}, fmt.Errorf("nomba checkout request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return CheckoutResult{}, fmt.Errorf("nomba checkout returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var checkoutResp CheckoutResponse
	if err := json.NewDecoder(resp.Body).Decode(&checkoutResp); err != nil {
		return CheckoutResult{}, fmt.Errorf("failed to decode nomba checkout response: %w", err)
	}

	return CheckoutResult{
		CheckoutLink: checkoutResp.Data.CheckoutLink,
	}, nil
}

type TokenListResponse struct {
	Code string `json:"code"`
	Data struct {
		TokenizedCardDataList []struct {
			TokenKey string `json:"tokenKey"`
		} `json:"tokenizedCardDataList"`
	} `json:"data"`
}

func (c *Client) ListTokens(ctx context.Context, customerID string) ([]string, error) {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain access token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/checkout/tokenized-card-data", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create token list request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("accountId", c.effectiveAccountID())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nomba token list request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token list response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nomba token list returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var listResp TokenListResponse
	if err := json.Unmarshal(respBody, &listResp); err != nil {
		return nil, fmt.Errorf("failed to decode token list response: %w", err)
	}

	var tokens []string
	for _, t := range listResp.Data.TokenizedCardDataList {
		tokens = append(tokens, t.TokenKey)
	}
	return tokens, nil
}

type chargeOrderRequest struct {
	OrderReference string `json:"orderReference"`
	CustomerID     string `json:"customerId"`
	CallbackURL    string `json:"callbackUrl"`
	CustomerEmail  string `json:"customerEmail"`
	Amount         string `json:"amount"`
	Currency       string `json:"currency"`
}

type chargeRequest struct {
	Order    chargeOrderRequest `json:"order"`
	TokenKey string             `json:"tokenKey"`
}

type ChargeResponse struct {
	Code string `json:"code"`
	Data struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
	} `json:"data"`
}

type ChargeResult struct {
	Success bool
	Message string
}

func (c *Client) ChargeToken(ctx context.Context, amountNaira, cardToken, customerID, customerEmail, callbackURL, orderReference string) (ChargeResult, error) {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return ChargeResult{}, fmt.Errorf("failed to obtain access token: %w", err)
	}

	payload := chargeRequest{
		Order: chargeOrderRequest{
			OrderReference: orderReference,
			CustomerID:     customerID,
			CallbackURL:    callbackURL,
			CustomerEmail:  customerEmail,
			Amount:         amountNaira,
			Currency:       "NGN",
		},
		TokenKey: cardToken,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return ChargeResult{}, fmt.Errorf("failed to marshal charge request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/checkout/tokenized-card-payment", bytes.NewReader(payloadBytes))
	if err != nil {
		return ChargeResult{}, fmt.Errorf("failed to create charge request: %w", err)
	}

	idempotencyKey, err := generateIdempotencyKey()
	if err != nil {
		return ChargeResult{}, fmt.Errorf("failed to generate idempotency key: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("accountId", c.effectiveAccountID())
	req.Header.Set("X-Idempotent-key", idempotencyKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ChargeResult{}, fmt.Errorf("nomba charge request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChargeResult{}, fmt.Errorf("failed to read charge response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return ChargeResult{}, fmt.Errorf("nomba charge returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var chargeResp ChargeResponse
	if err := json.Unmarshal(respBody, &chargeResp); err != nil {
		return ChargeResult{}, fmt.Errorf("failed to decode charge response: %w", err)
	}

	return ChargeResult{
		Success: chargeResp.Data.Status,
		Message: chargeResp.Data.Message,
	}, nil
}

// RevokeToken deletes a stored card token.
func (c *Client) RevokeToken(ctx context.Context, cardToken string) error {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to obtain access token: %w", err)
	}

	url := fmt.Sprintf("%s/tokenized-card/%s", c.baseURL, cardToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create revoke request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("accountId", c.effectiveAccountID())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("nomba revoke request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("nomba revoke returned status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
