package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	// API Keys
	APIKey          string
	OpenAIApiKey    string
	CoinGeckoApiKey string

	// Server Configuration
	Port  string
	Debug bool

	// Application Settings
	MaxRetries     int
	TimeoutSeconds int
}

// Load loads configuration from .env file and environment variables
func Load() (*Config, error) {
	// Try to load .env file from current directory
	_ = godotenv.Load()

	// If .env doesn't exist, try .env.local
	_ = godotenv.Load(".env.local")

	cfg := &Config{
		// API Keys
		APIKey:          getEnv("API_KEY", ""),
		OpenAIApiKey:    getEnv("OPENAI_API_KEY", getEnv("API_KEY", "")),
		CoinGeckoApiKey: getEnv("COINGECKO_API_KEY", ""),

		// Server Configuration
		Port:  getEnv("PORT", "8080"),
		Debug: getEnvBool("DEBUG", false),

		// Application Settings
		MaxRetries:     getEnvInt("MAX_RETRIES", 3),
		TimeoutSeconds: getEnvInt("TIMEOUT_SECONDS", 30),
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

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
