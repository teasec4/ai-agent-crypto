package agent

import (
	"strings"
)

type Planner interface {
	Plan(input string, state *State) Plan
}

type SimplePlanner struct{}

func NewPlanner() Planner {
	return &SimplePlanner{}
}

func (p *SimplePlanner) Plan(input string, state *State) Plan {
	// Convert input to lowercase for easier matching
	lowerInput := strings.ToLower(input)

	// Check for cryptocurrency price queries
	if strings.Contains(lowerInput, "цена") || strings.Contains(lowerInput, "курс") || strings.Contains(lowerInput, "price") ||
		strings.Contains(lowerInput, "стоит") || strings.Contains(lowerInput, "купить") || strings.Contains(lowerInput, "продать") ||
		strings.Contains(lowerInput, "buy") || strings.Contains(lowerInput, "sell") {
		// Try to identify which cryptocurrency
		if strings.Contains(lowerInput, "btc") || strings.Contains(lowerInput, "bitcoin") || strings.Contains(lowerInput, "биткоин") {
			return Plan{
				Action: "get_crypto_price",
				Parameters: map[string]interface{}{
					"query":    "bitcoin",
					"currency": "usd",
				},
				Input: input,
			}
		}
		if strings.Contains(lowerInput, "eth") || strings.Contains(lowerInput, "ethereum") || strings.Contains(lowerInput, "эфир") {
			return Plan{
				Action: "get_crypto_price",
				Parameters: map[string]interface{}{
					"query":    "ethereum",
					"currency": "usd",
				},
				Input: input,
			}
		}
		if strings.Contains(lowerInput, "crypto") || strings.Contains(lowerInput, "крипто") ||
			strings.Contains(lowerInput, "cryptocurrency") || strings.Contains(lowerInput, "криптовалют") {
			// Generic crypto query - will use default bitcoin
			return Plan{
				Action: "get_crypto_price",
				Parameters: map[string]interface{}{
					"query":    "bitcoin",
					"currency": "usd",
				},
				Input: input,
			}
		}
		// If we have a crypto-related query but can't identify specific coin, still try bitcoin
		if strings.Contains(lowerInput, "coin") || strings.Contains(lowerInput, "монет") {
			return Plan{
				Action: "get_crypto_price",
				Parameters: map[string]interface{}{
					"query":    "bitcoin",
					"currency": "usd",
				},
				Input: input,
			}
		}
	}

	// Check for greeting
	if strings.Contains(lowerInput, "привет") || strings.Contains(lowerInput, "hello") || strings.Contains(lowerInput, "hi") {
		return Plan{
			Action: "greeting",
			Parameters: map[string]interface{}{
				"name": extractName(input),
			},
			Input: input,
		}
	}

	// Default action for unknown queries
	return Plan{
		Action: "unknown",
		Parameters: map[string]interface{}{
			"query": input,
		},
		Input: input,
	}
}

// Helper function to extract name from greeting
func extractName(input string) string {
	// Simple extraction - can be improved
	words := strings.Fields(input)
	for i, word := range words {
		if strings.ToLower(word) == "меня" && i+1 < len(words) {
			return words[i+1]
		}
		if strings.ToLower(word) == "я" && i+1 < len(words) {
			return words[i+1]
		}
	}
	return ""
}
