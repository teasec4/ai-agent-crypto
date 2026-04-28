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
func NewCryptoTool() Tool {
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
	cryptoQuery := "bitcoin"
	vsCurrency := "usd"

	if query, ok := params["query"].(string); ok && query != "" {
		cryptoQuery = query
	}
	if currency, ok := params["currency"].(string); ok && currency != "" {
		vsCurrency = currency
	}

	return t.GetPriceForQuery(cryptoQuery, vsCurrency)
}

// GetCryptoPrice gets the price of a cryptocurrency.
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
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]map[string]float64
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	cryptoData, ok := result[cryptoID]
	if !ok {
		return "", fmt.Errorf("cryptocurrency '%s' not found in response", cryptoID)
	}
	price, ok := cryptoData[vsCurrency]
	if !ok {
		return "", fmt.Errorf("currency '%s' not found for cryptocurrency '%s'", vsCurrency, cryptoID)
	}

	return fmt.Sprintf("%s price: $%.2f %s", strings.Title(cryptoID), price, strings.ToUpper(vsCurrency)), nil
}

// SearchCrypto searches for cryptocurrencies by name or symbol.
func (t *CryptoTool) SearchCrypto(query string) ([]CryptoSearchResult, error) {
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/search?query=%s", query)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if t.apiKey != "" {
		req.Header.Add("x-cg-demo-api-key", t.apiKey)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var searchResponse struct {
		Coins []CryptoSearchResult `json:"coins"`
	}
	if err := json.Unmarshal(body, &searchResponse); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	return searchResponse.Coins, nil
}

// CryptoSearchResult represents a cryptocurrency search result.
type CryptoSearchResult struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Symbol string `json:"symbol"`
}

// GetPriceForQuery tries to find and get price for a cryptocurrency query.
func (t *CryptoTool) GetPriceForQuery(query, vsCurrency string) (string, error) {
	results, err := t.SearchCrypto(query)
	if err != nil {
		return "", fmt.Errorf("failed to search for cryptocurrency: %w", err)
	}
	if len(results) == 0 {
		return "", fmt.Errorf("no cryptocurrencies found for query: %s", query)
	}

	cryptoID := results[0].ID
	return t.GetCryptoPrice(cryptoID, vsCurrency)
}
