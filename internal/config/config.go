package config

import (
	"encoding/json"
	"fmt"
	"os"

	"crypto-alert/internal/core"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	// Pyth Oracle Configuration
	PythAPIURL string
	PythAPIKey string

	// Resend Email Configuration
	ResendAPIKey    string
	ResendFromEmail string

	// Alert Configuration
	CheckInterval  int    // in seconds
	AlertRulesFile string // Path to JSON file containing alert rules
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// Try to load .env file (ignore error if it doesn't exist)
	_ = godotenv.Load()

	config := &Config{
		PythAPIURL:      getEnv("PYTH_API_URL", "https://hermes.pyth.network"),
		PythAPIKey:      getEnv("PYTH_API_KEY", ""),
		ResendAPIKey:    getEnv("RESEND_API_KEY", ""),
		ResendFromEmail: getEnv("RESEND_FROM_EMAIL", ""),
		CheckInterval:   60, // Default 60 seconds
		AlertRulesFile:  getEnv("ALERT_RULES_FILE", "alert-rules.json"),
	}

	return config, nil
}

// AlertRuleConfig represents an alert rule in JSON format
type AlertRuleConfig struct {
	Symbol         string  `json:"symbol"`
	PriceFeedID    string  `json:"price_feed_id"` // Pyth price feed ID for this symbol
	Threshold      float64 `json:"threshold"`
	Direction      string  `json:"direction"` // ">=", ">", "=", "<=", "<"
	Enabled        bool    `json:"enabled"`
	RecipientEmail string  `json:"recipient_email"` // Email address to send alerts to
}

// LoadAlertRules loads alert rules from a JSON file
func LoadAlertRules(filePath string) ([]*core.AlertRule, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("alert rules file not found: %s", filePath)
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read alert rules file: %w", err)
	}

	// Parse JSON
	var ruleConfigs []AlertRuleConfig
	if err := json.Unmarshal(data, &ruleConfigs); err != nil {
		return nil, fmt.Errorf("failed to parse alert rules JSON: %w", err)
	}

	// Convert to core.AlertRule
	rules := make([]*core.AlertRule, 0, len(ruleConfigs))
	for _, rc := range ruleConfigs {
		// Validate direction
		var direction core.Direction
		switch rc.Direction {
		case ">=":
			direction = core.DirectionGreaterThanOrEqual
		case ">":
			direction = core.DirectionGreaterThan
		case "=":
			direction = core.DirectionEqual
		case "<=":
			direction = core.DirectionLessThanOrEqual
		case "<":
			direction = core.DirectionLessThan
		default:
			return nil, fmt.Errorf("invalid direction '%s' for symbol %s, must be one of: >=, >, =, <=, <", rc.Direction, rc.Symbol)
		}

		// Validate symbol
		if rc.Symbol == "" {
			return nil, fmt.Errorf("symbol cannot be empty in alert rule")
		}

		// Validate threshold
		if rc.Threshold <= 0 {
			return nil, fmt.Errorf("threshold must be positive for symbol %s", rc.Symbol)
		}

		// Validate recipient email
		if rc.RecipientEmail == "" {
			return nil, fmt.Errorf("recipient_email is required for symbol %s", rc.Symbol)
		}

		// Validate price feed ID
		if rc.PriceFeedID == "" {
			return nil, fmt.Errorf("price_feed_id is required for symbol %s", rc.Symbol)
		}

		rules = append(rules, &core.AlertRule{
			Symbol:         rc.Symbol,
			PriceFeedID:    rc.PriceFeedID,
			Threshold:      rc.Threshold,
			Direction:      direction,
			Enabled:        rc.Enabled,
			RecipientEmail: rc.RecipientEmail,
		})
	}

	return rules, nil
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
