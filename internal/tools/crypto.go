package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const coinGeckoBaseURL = "https://api.coingecko.com/api/v3"

// CryptoTool is a tool for getting cryptocurrency prices.
type CryptoTool struct {
	client  *http.Client
	apiKey  string
	baseURL string
}

// NewCryptoTool creates a new CryptoTool instance.
func NewCryptoTool() *CryptoTool {
	return &CryptoTool{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: coinGeckoBaseURL,
	}
}

// SetAPIKey sets the optional CoinGecko API key.
func (t *CryptoTool) SetAPIKey(key string) {
	t.apiKey = key
}

func (t *CryptoTool) setBaseURLForTest(baseURL string) {
	t.baseURL = strings.TrimRight(baseURL, "/")
}

// Name returns the tool name.
func (t *CryptoTool) Name() string {
	return "get_crypto_price"
}

// Description returns a human-readable description of the tool.
func (t *CryptoTool) Description() string {
	return "Get the current price of a cryptocurrency by symbol, ticker, name, or CoinGecko id."
}

// Schema returns the JSON schema for tool parameters.
func (t *CryptoTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]Parameter{
			"query": {
				Type:        "string",
				Description: "Cryptocurrency symbol, ticker, name, or CoinGecko id (e.g. ETH, ethereum, HYPE, hyperliquid) (required)",
			},
			"currency": {Type: "string", Description: "Fiat currency (e.g. usd, eur, default: usd)"},
		},
		Required: []string{"query"},
	}
}

// Run executes the crypto price check.
func (t *CryptoTool) Run(ctx context.Context, workspace string, params map[string]interface{}) (string, error) {
	query, ok := params["query"].(string)
	query = strings.TrimSpace(query)
	if !ok || query == "" {
		return "", fmt.Errorf("missing required parameter 'query' (symbol, ticker, name, or CoinGecko id; e.g. query=ETH, query=HYPE)")
	}

	vsCurrency := "usd"
	if currency, ok := params["currency"].(string); ok && strings.TrimSpace(currency) != "" {
		vsCurrency = strings.ToLower(strings.TrimSpace(currency))
	}

	return t.GetCryptoPrice(ctx, query, vsCurrency)
}

// GetCryptoPrice gets the price of a cryptocurrency from CoinGecko.
func (t *CryptoTool) GetCryptoPrice(ctx context.Context, query, vsCurrency string) (string, error) {
	normalized := normalizeCryptoID(query)
	simple, ok, err := t.fetchSimplePrice(ctx, normalized, vsCurrency)
	if err != nil {
		return "", err
	}
	if ok {
		return formatSimplePrice(simple, vsCurrency), nil
	}

	resolved, err := t.resolveCoin(ctx, query)
	if err != nil {
		return "", err
	}
	if resolved.ID == "" {
		return "", fmt.Errorf("cryptocurrency %q not found by id, symbol, or name on CoinGecko", query)
	}

	market, ok, err := t.fetchMarketPrice(ctx, resolved.ID, vsCurrency)
	if err != nil {
		return "", err
	}
	if ok {
		return formatMarketPrice(market, vsCurrency, query), nil
	}

	simple, ok, err = t.fetchSimplePrice(ctx, resolved.ID, vsCurrency)
	if err != nil {
		return "", err
	}
	if ok {
		return formatSimplePrice(simple, vsCurrency), nil
	}

	return "", fmt.Errorf("cryptocurrency %q resolved to CoinGecko id %q, but no %s price was returned", query, resolved.ID, strings.ToUpper(vsCurrency))
}

type simplePrice struct {
	ID    string
	Name  string
	Price float64
}

func (t *CryptoTool) fetchSimplePrice(ctx context.Context, coinID, vsCurrency string) (simplePrice, bool, error) {
	u := t.endpoint("/simple/price")
	q := u.Query()
	q.Set("ids", coinID)
	q.Set("vs_currencies", vsCurrency)
	u.RawQuery = q.Encode()

	body, err := t.get(ctx, u)
	if err != nil {
		return simplePrice{}, false, err
	}

	var result map[string]map[string]float64
	if err := json.Unmarshal(body, &result); err != nil {
		return simplePrice{}, false, fmt.Errorf("failed to parse CoinGecko price response: %w", err)
	}

	cryptoData, ok := result[coinID]
	if !ok {
		return simplePrice{}, false, nil
	}
	price, ok := cryptoData[vsCurrency]
	if !ok {
		return simplePrice{}, false, fmt.Errorf("currency %q not found for cryptocurrency %q", vsCurrency, coinID)
	}

	return simplePrice{
		ID:    coinID,
		Name:  titleCoinID(coinID),
		Price: price,
	}, true, nil
}

type resolvedCoin struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Symbol        string `json:"symbol"`
	MarketCapRank int    `json:"market_cap_rank"`
}

func (t *CryptoTool) resolveCoin(ctx context.Context, query string) (resolvedCoin, error) {
	u := t.endpoint("/search")
	q := u.Query()
	q.Set("query", strings.TrimSpace(query))
	u.RawQuery = q.Encode()

	body, err := t.get(ctx, u)
	if err != nil {
		return resolvedCoin{}, err
	}

	var result struct {
		Coins []resolvedCoin `json:"coins"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return resolvedCoin{}, fmt.Errorf("failed to parse CoinGecko search response: %w", err)
	}
	if len(result.Coins) == 0 {
		return resolvedCoin{}, nil
	}

	queryNorm := normalizeSearchText(query)
	sort.SliceStable(result.Coins, func(i, j int) bool {
		return coinMatchScore(result.Coins[i], queryNorm) > coinMatchScore(result.Coins[j], queryNorm)
	})
	return result.Coins[0], nil
}

func coinMatchScore(coin resolvedCoin, queryNorm string) int {
	symbol := normalizeSearchText(coin.Symbol)
	name := normalizeSearchText(coin.Name)
	id := normalizeSearchText(coin.ID)

	score := 0
	switch {
	case symbol == queryNorm:
		score += 1000
	case name == queryNorm:
		score += 900
	case id == queryNorm:
		score += 850
	case strings.Contains(name, queryNorm):
		score += 300
	case strings.Contains(id, queryNorm):
		score += 250
	}

	if coin.MarketCapRank > 0 {
		score += max(0, 200-coin.MarketCapRank)
	}
	return score
}

type marketPrice struct {
	ID            string  `json:"id"`
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	CurrentPrice  float64 `json:"current_price"`
	MarketCap     float64 `json:"market_cap"`
	MarketCapRank int     `json:"market_cap_rank"`
}

func (t *CryptoTool) fetchMarketPrice(ctx context.Context, coinID, vsCurrency string) (marketPrice, bool, error) {
	u := t.endpoint("/coins/markets")
	q := u.Query()
	q.Set("vs_currency", vsCurrency)
	q.Set("ids", coinID)
	q.Set("price_change_percentage", "24h")
	u.RawQuery = q.Encode()

	body, err := t.get(ctx, u)
	if err != nil {
		return marketPrice{}, false, err
	}

	var markets []marketPrice
	if err := json.Unmarshal(body, &markets); err != nil {
		return marketPrice{}, false, fmt.Errorf("failed to parse CoinGecko market response: %w", err)
	}
	if len(markets) == 0 {
		return marketPrice{}, false, nil
	}
	return markets[0], true, nil
}

func (t *CryptoTool) get(ctx context.Context, u url.URL) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create CoinGecko request: %w", err)
	}
	if t.apiKey != "" {
		req.Header.Add("x-cg-demo-api-key", t.apiKey)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch CoinGecko data: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read CoinGecko response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CoinGecko API request failed with status %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func (t *CryptoTool) endpoint(path string) url.URL {
	base := strings.TrimRight(t.baseURL, "/")
	parsed, _ := url.Parse(base + path)
	return *parsed
}

func formatSimplePrice(price simplePrice, vsCurrency string) string {
	return fmt.Sprintf("%s (%s) price: %.4f %s", price.Name, price.ID, price.Price, strings.ToUpper(vsCurrency))
}

func formatMarketPrice(price marketPrice, vsCurrency string, originalQuery string) string {
	parts := []string{
		fmt.Sprintf("%s (%s, CoinGecko id: %s) price: %.4f %s", price.Name, strings.ToUpper(price.Symbol), price.ID, price.CurrentPrice, strings.ToUpper(vsCurrency)),
	}
	if price.MarketCapRank > 0 {
		parts = append(parts, fmt.Sprintf("market cap rank: #%d", price.MarketCapRank))
	}
	if price.MarketCap > 0 {
		parts = append(parts, fmt.Sprintf("market cap: %.0f %s", price.MarketCap, strings.ToUpper(vsCurrency)))
	}
	if !strings.EqualFold(strings.TrimSpace(originalQuery), price.ID) {
		parts = append(parts, fmt.Sprintf("resolved query %q to %q", originalQuery, price.ID))
	}
	return strings.Join(parts, "; ")
}

func normalizeCryptoID(query string) string {
	id := strings.ToLower(strings.TrimSpace(query))
	aliases := map[string]string{
		"btc":     "bitcoin",
		"xbt":     "bitcoin",
		"eth":     "ethereum",
		"hype":    "hyperliquid",
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

func normalizeSearchText(query string) string {
	return strings.ToLower(strings.TrimSpace(query))
}

func titleCoinID(id string) string {
	if id == "" {
		return ""
	}
	parts := strings.FieldsFunc(id, func(r rune) bool {
		return r == '-' || r == '_'
	})
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}
