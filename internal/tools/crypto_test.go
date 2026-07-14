package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCryptoTool_HypeAliasResolvesToHyperliquid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/simple/price" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("ids"); got != "hyperliquid" {
			t.Fatalf("expected ids=hyperliquid, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hyperliquid":{"usd":42.5}}`))
	}))
	defer server.Close()

	tool := NewCryptoTool()
	tool.setBaseURLForTest(server.URL)

	result, err := tool.Run(context.Background(), "", map[string]interface{}{
		"query": "HYPE",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.Contains(result, "Hyperliquid") || !strings.Contains(result, "42.5000 USD") {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestCryptoTool_SearchFallbackBySymbol(t *testing.T) {
	var sawSearch bool
	var sawMarkets bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/simple/price":
			if got := r.URL.Query().Get("ids"); got != "newcoin" {
				t.Fatalf("expected direct ids=newcoin, got %q", got)
			}
			_, _ = w.Write([]byte(`{}`))
		case "/search":
			sawSearch = true
			if got := r.URL.Query().Get("query"); got != "NEWCOIN" {
				t.Fatalf("expected search query NEWCOIN, got %q", got)
			}
			_, _ = w.Write([]byte(`{"coins":[{"id":"new-coin","name":"New Coin","symbol":"newcoin","market_cap_rank":12}]}`))
		case "/coins/markets":
			sawMarkets = true
			if got := r.URL.Query().Get("ids"); got != "new-coin" {
				t.Fatalf("expected market ids=new-coin, got %q", got)
			}
			_, _ = w.Write([]byte(`[{"id":"new-coin","symbol":"newcoin","name":"New Coin","current_price":7.25,"market_cap":123456789,"market_cap_rank":12}]`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	tool := NewCryptoTool()
	tool.setBaseURLForTest(server.URL)

	result, err := tool.Run(context.Background(), "", map[string]interface{}{
		"query": "NEWCOIN",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !sawSearch || !sawMarkets {
		t.Fatalf("expected search and markets fallback; sawSearch=%v sawMarkets=%v", sawSearch, sawMarkets)
	}
	if !strings.Contains(result, "New Coin") || !strings.Contains(result, "market cap rank: #12") {
		t.Fatalf("unexpected result: %s", result)
	}
}
