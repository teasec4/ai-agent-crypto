package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	// API Keys
	APIKey          string
	OpenAIApiKey    string
	CoinGeckoApiKey string

	// LLM Configuration
	LLMBaseURL     string
	LLMModel       string
	LLMTemperature float64
	LLMMaxTokens   int

	// Application Settings
	TimeoutSeconds     int
	SessionStoragePath string
	LogLevel           string
	AllowAutoApprove   bool
}

// Load loads configuration from .env file and environment variables
func Load() (*Config, error) {
	// Try .env first, then .env.local — silence "not found" errors
	_ = godotenv.Load()
	_ = godotenv.Load(".env.local")

	cfg := &Config{
		// API Keys
		APIKey:          getEnv("API_KEY", ""),
		OpenAIApiKey:    getEnv("OPENAI_API_KEY", getEnv("API_KEY", "")),
		CoinGeckoApiKey: getEnv("COINGECKO_API_KEY", ""),

		// LLM Configuration
		LLMBaseURL:     getEnv("LLM_BASE_URL", "https://api.deepseek.com/v1/chat/completions"),
		LLMModel:       getEnv("LLM_MODEL", "deepseek-chat"),
		LLMTemperature: getEnvFloat("LLM_TEMPERATURE", 0.7),
		LLMMaxTokens:   getEnvInt("LLM_MAX_TOKENS", 2048),

		// Application Settings
		TimeoutSeconds:     getEnvInt("TIMEOUT_SECONDS", 30),
		SessionStoragePath: getEnv("SESSION_STORAGE_PATH", "data/sessions.json"),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		AllowAutoApprove:   getEnvBool("ALLOW_AUTO_APPROVE", false),
	}

	return cfg, nil
}

// Helper functions to get environment variables with defaults
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	switch strings.ToLower(value) {
	case "true", "1", "yes", "y":
		return true
	default:
		return false
	}
}
