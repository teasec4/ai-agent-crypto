package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// CryptoTool is a tool for getting cryptocurrency prices.
type CryptoTool struct {
	client *http.Client
	apiKey string
}

// NewCryptoTool creates a new CryptoTool instance.
func NewCryptoTool() *CryptoTool {
	return &CryptoTool{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetAPIKey sets the optional CoinGecko API key.
func (t *CryptoTool) SetAPIKey(key string) {
	t.apiKey = key
}

// Name returns the tool name.
func (t *CryptoTool) Name() string {
	return "get_crypto_price"
}

// Description returns a human-readable description of the tool.
func (t *CryptoTool) Description() string {
	return "Get the current price of a cryptocurrency. Parameters: query (crypto name, e.g. bitcoin, ethereum), currency (fiat currency, e.g. usd, eur)"
}

// Run executes the crypto price check.
func (t *CryptoTool) Run(params map[string]interface{}) (string, error) {
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("missing required parameter 'query' (e.g. query=bitcoin). Supported params: query (crypto name), currency (fiat, default: usd)")
	}

	vsCurrency := "usd"
	if currency, ok := params["currency"].(string); ok && currency != "" {
		vsCurrency = currency
	}

	return t.GetCryptoPrice(normalizeCryptoID(query), strings.ToLower(strings.TrimSpace(vsCurrency)))
}

// GetCryptoPrice gets the price of a cryptocurrency from CoinGecko.
func (t *CryptoTool) GetCryptoPrice(cryptoID, vsCurrency string) (string, error) {
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=%s", cryptoID, vsCurrency)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	if t.apiKey != "" {
		req.Header.Add("x-cg-demo-api-key", t.apiKey)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch price: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return "", fmt.Errorf("cryptocurrency '%s' not found. CoinGecko IDs are lowercase (e.g. bitcoin, ethereum, dogecoin)", cryptoID)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var result map[string]map[string]float64
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result) == 0 {
		return "", fmt.Errorf("cryptocurrency '%s' not found. Try a different name (e.g. bitcoin, ethereum, dogecoin)", cryptoID)
	}

	cryptoData, ok := result[cryptoID]
	if !ok {
		return "", fmt.Errorf("cryptocurrency '%s' not found in response", cryptoID)
	}
	price, ok := cryptoData[vsCurrency]
	if !ok {
		return "", fmt.Errorf("currency '%s' not found for cryptocurrency '%s'", vsCurrency, cryptoID)
	}

	name := strings.ToUpper(cryptoID[:1]) + cryptoID[1:]
	return fmt.Sprintf("%s price: %.2f %s", name, price, strings.ToUpper(vsCurrency)), nil
}

func normalizeCryptoID(query string) string {
	id := strings.ToLower(strings.TrimSpace(query))
	aliases := map[string]string{
		"btc":     "bitcoin",
		"xbt":     "bitcoin",
		"eth":     "ethereum",
		"sol":     "solana",
		"doge":    "dogecoin",
		"bnb":     "binancecoin",
		"ada":     "cardano",
		"xrp":     "ripple",
		"dot":     "polkadot",
		"matic":   "matic-network",
		"polygon": "matic-network",
	}
	if alias, ok := aliases[id]; ok {
		return alias
	}
	return id
}
