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

// FrequencyUnit represents the unit for frequency
type FrequencyUnit string

const (
	FrequencyUnitDay  FrequencyUnit = "DAY"
	FrequencyUnitHour FrequencyUnit = "HOUR"
	FrequencyUnitOnce FrequencyUnit = "ONCE"
)

// FrequencyConfig represents the frequency configuration for an alert rule
type FrequencyConfig struct {
	Number *int          `json:"number,omitempty"` // Required for DAY and HOUR, not needed for ONCE
	Unit   FrequencyUnit `json:"unit"`             // DAY, HOUR, or ONCE
}

// AlertRuleConfig represents a price alert rule in JSON format
type AlertRuleConfig struct {
	Symbol         string           `json:"symbol,omitempty"`
	PriceFeedID    string           `json:"price_feed_id,omitempty"` // Pyth price feed ID for this symbol
	Threshold      float64          `json:"threshold"`
	Direction      string           `json:"direction"` // ">=", ">", "=", "<=", "<"
	Enabled        bool             `json:"enabled"`
	RecipientEmail string           `json:"recipient_email"`     // Email address to send alerts to
	Frequency      *FrequencyConfig `json:"frequency,omitempty"` // Optional frequency configuration
}

// DeFiAlertRuleConfig represents a DeFi protocol alert rule in JSON format
type DeFiAlertRuleConfig struct {
	Protocol            string           `json:"protocol"`              // e.g., "aave"
	Version             string           `json:"version"`               // e.g., "v3"
	ChainID             string           `json:"chain_id"`              // Chain ID: "1", "8453", "42161"
	MarketTokenContract string           `json:"market_token_contract"` // Token contract address
	Field               string           `json:"field"`                 // "TVL", "APY", "UTILIZATION"
	Threshold           float64          `json:"threshold"`
	Direction           string           `json:"direction"` // ">=", ">", "=", "<=", "<"
	Enabled             bool             `json:"enabled"`
	RecipientEmail      string           `json:"recipient_email"`     // Email address to send alerts to
	Frequency           *FrequencyConfig `json:"frequency,omitempty"` // Optional frequency configuration
}

// LoadAlertRules loads alert rules from a JSON file (supports both price and DeFi rules)
func LoadAlertRules(filePath string) ([]*core.AlertRule, []*core.DeFiAlertRule, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("alert rules file not found: %s", filePath)
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read alert rules file: %w", err)
	}

	// Parse JSON as array of raw JSON objects to determine type
	var rawRules []json.RawMessage
	if err := json.Unmarshal(data, &rawRules); err != nil {
		return nil, nil, fmt.Errorf("failed to parse alert rules JSON: %w", err)
	}

	priceRules := make([]*core.AlertRule, 0)
	defiRules := make([]*core.DeFiAlertRule, 0)

	for i, rawRule := range rawRules {
		// Try to determine if it's a price rule or DeFi rule
		var priceRule AlertRuleConfig
		var defiRule DeFiAlertRuleConfig

		// Try parsing as DeFi rule first (check for protocol field)
		if err := json.Unmarshal(rawRule, &defiRule); err == nil && defiRule.Protocol != "" {
			// It's a DeFi rule
			rule, err := parseDeFiRule(defiRule)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse DeFi rule at index %d: %w", i, err)
			}
			defiRules = append(defiRules, rule)
			continue
		}

		// Try parsing as price rule
		if err := json.Unmarshal(rawRule, &priceRule); err == nil && priceRule.Symbol != "" {
			// It's a price rule
			rule, err := parsePriceRule(priceRule)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse price rule at index %d: %w", i, err)
			}
			priceRules = append(priceRules, rule)
			continue
		}

		return nil, nil, fmt.Errorf("unable to determine rule type at index %d (must be either price rule with 'symbol' or DeFi rule with 'protocol')", i)
	}

	return priceRules, defiRules, nil
}

// parsePriceRule converts AlertRuleConfig to core.AlertRule
func parsePriceRule(rc AlertRuleConfig) (*core.AlertRule, error) {
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

	// Validate frequency configuration
	var frequency *core.Frequency
	if rc.Frequency != nil {
		// Validate unit
		switch rc.Frequency.Unit {
		case FrequencyUnitDay, FrequencyUnitHour:
			// DAY and HOUR require a number
			if rc.Frequency.Number == nil || *rc.Frequency.Number <= 0 {
				return nil, fmt.Errorf("frequency.number is required and must be positive for unit %s in symbol %s", rc.Frequency.Unit, rc.Symbol)
			}
			frequency = &core.Frequency{
				Number: *rc.Frequency.Number,
				Unit:   core.FrequencyUnit(rc.Frequency.Unit),
			}
		case FrequencyUnitOnce:
			// ONCE does not require a number
			frequency = &core.Frequency{
				Number: 0, // Not used for ONCE
				Unit:   core.FrequencyUnitOnce,
			}
		default:
			return nil, fmt.Errorf("invalid frequency.unit '%s' for symbol %s, must be one of: DAY, HOUR, ONCE", rc.Frequency.Unit, rc.Symbol)
		}
	}

	return &core.AlertRule{
		Symbol:         rc.Symbol,
		PriceFeedID:    rc.PriceFeedID,
		Threshold:      rc.Threshold,
		Direction:      direction,
		Enabled:        rc.Enabled,
		RecipientEmail: rc.RecipientEmail,
		Frequency:      frequency,
	}, nil
}

// parseDeFiRule converts DeFiAlertRuleConfig to core.DeFiAlertRule
func parseDeFiRule(rc DeFiAlertRuleConfig) (*core.DeFiAlertRule, error) {
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
		return nil, fmt.Errorf("invalid direction '%s' for protocol %s %s, must be one of: >=, >, =, <=, <", rc.Direction, rc.Protocol, rc.Version)
	}

	// Validate protocol
	if rc.Protocol == "" {
		return nil, fmt.Errorf("protocol cannot be empty in DeFi alert rule")
	}

	// Validate version
	if rc.Version == "" {
		return nil, fmt.Errorf("version cannot be empty in DeFi alert rule")
	}

	// Validate chain ID
	if rc.ChainID == "" {
		return nil, fmt.Errorf("chain_id cannot be empty in DeFi alert rule")
	}

	// Validate market token contract
	if rc.MarketTokenContract == "" {
		return nil, fmt.Errorf("market_token_contract cannot be empty in DeFi alert rule")
	}

	// Validate field
	if rc.Field != "TVL" && rc.Field != "APY" && rc.Field != "UTILIZATION" {
		return nil, fmt.Errorf("invalid field '%s' for protocol %s %s, must be one of: TVL, APY, UTILIZATION", rc.Field, rc.Protocol, rc.Version)
	}

	// Validate threshold
	if rc.Threshold < 0 {
		return nil, fmt.Errorf("threshold must be non-negative for protocol %s %s", rc.Protocol, rc.Version)
	}

	// Validate recipient email
	if rc.RecipientEmail == "" {
		return nil, fmt.Errorf("recipient_email is required for protocol %s %s", rc.Protocol, rc.Version)
	}

	// Validate frequency configuration
	var frequency *core.Frequency
	if rc.Frequency != nil {
		// Validate unit
		switch rc.Frequency.Unit {
		case FrequencyUnitDay, FrequencyUnitHour:
			// DAY and HOUR require a number
			if rc.Frequency.Number == nil || *rc.Frequency.Number <= 0 {
				return nil, fmt.Errorf("frequency.number is required and must be positive for unit %s in protocol %s %s", rc.Frequency.Unit, rc.Protocol, rc.Version)
			}
			frequency = &core.Frequency{
				Number: *rc.Frequency.Number,
				Unit:   core.FrequencyUnit(rc.Frequency.Unit),
			}
		case FrequencyUnitOnce:
			// ONCE does not require a number
			frequency = &core.Frequency{
				Number: 0, // Not used for ONCE
				Unit:   core.FrequencyUnitOnce,
			}
		default:
			return nil, fmt.Errorf("invalid frequency.unit '%s' for protocol %s %s, must be one of: DAY, HOUR, ONCE", rc.Frequency.Unit, rc.Protocol, rc.Version)
		}
	}

	return &core.DeFiAlertRule{
		Protocol:            rc.Protocol,
		Version:             rc.Version,
		ChainID:             rc.ChainID,
		MarketTokenContract: rc.MarketTokenContract,
		Field:               rc.Field,
		Threshold:           rc.Threshold,
		Direction:           direction,
		Enabled:             rc.Enabled,
		RecipientEmail:      rc.RecipientEmail,
		Frequency:           frequency,
	}, nil
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
