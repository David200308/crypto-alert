package utils

import (
	"math/rand"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

var envLoaded bool

func ensureEnvLoaded() {
	if !envLoaded {
		_ = godotenv.Load() // Ignore error if .env doesn't exist
		envLoaded = true
	}
}

func GetRandomRPCURL(envKey string) string {
	ensureEnvLoaded()

	raw := os.Getenv(envKey)
	if raw == "" {
		return ""
	}

	// Split by comma and trim whitespace
	parts := strings.Split(raw, ",")
	urls := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			urls = append(urls, trimmed)
		}
	}

	if len(urls) == 0 {
		return ""
	}

	if len(urls) == 1 {
		return urls[0]
	}

	// Pick a random URL
	return urls[rand.Intn(len(urls))]
}

func GetRPCURLForChain(chainID string) string {
	switch chainID {
	case "1":
		return GetRandomRPCURL("ETH_RPC_URL")
	case "8453":
		return GetRandomRPCURL("BASE_RPC_URL")
	case "42161":
		return GetRandomRPCURL("ARB_RPC_URL")
	default:
		return ""
	}
}

func GetSolanaRPCURL() string {
	return GetRandomRPCURL("SOLANA_RPC_URL")
}
