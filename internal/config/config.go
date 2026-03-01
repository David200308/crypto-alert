package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

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
	CheckInterval int    // in seconds
	MySQLDSN      string // MySQL DSN for web3 database

	// Logging Configuration
	LogDir string // Directory for log files (default: "logs")

	// Elasticsearch Configuration (optional, for log shipping)
	ESEnabled   bool     // Enable shipping logs to Elasticsearch
	ESAddresses []string // ES endpoints, e.g. []string{"http://localhost:9200"}
	ESIndex     string   // Index name for logs (default: "crypto-alert-logs")

	// Kafka Configuration
	KafkaBrokers []string // Kafka broker addresses, e.g. []string{"localhost:9092"}

	// Hot-swap Configuration
	RuleReloadInterval int // seconds between MySQL rule re-reads (0 = disabled)
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// Try to load .env file (ignore error if it doesn't exist)
	_ = godotenv.Load()

	config := &Config{
		PythAPIURL:       getEnv("PYTH_API_URL", "https://hermes.pyth.network"),
		PythAPIKey:       getEnv("PYTH_API_KEY", ""),
		ResendAPIKey:     getEnv("RESEND_API_KEY", ""),
		ResendFromEmail:  getEnv("RESEND_FROM_EMAIL", ""),
		CheckInterval: 60, // Default 60 seconds
		MySQLDSN:      getEnv("MYSQL_DSN", ""),
		LogDir:           getEnv("LOG_DIR", "logs"), // Default log directory
		ESEnabled:        getEnvBool("ES_ENABLED", true),
		ESAddresses:      getEnvSlice("ES_ADDRESSES", []string{"http://localhost:9200"}),
		ESIndex:          getEnv("ES_INDEX", "crypto-alert-logs"),
		KafkaBrokers:       getEnvSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
		RuleReloadInterval: getEnvInt("RULE_RELOAD_INTERVAL", 60),
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
	Symbol           string           `json:"symbol,omitempty"`
	PriceFeedID      string           `json:"price_feed_id,omitempty"` // Pyth price feed ID for this symbol
	Threshold        float64          `json:"threshold"`
	Direction        string           `json:"direction"` // ">=", ">", "=", "<=", "<"
	Enabled          bool             `json:"enabled"`
	RecipientEmail   string           `json:"recipient_email"`           // Email address to send alerts to
	TelegramChatID   string           `json:"telegram_chat_id,omitempty"` // Optional Telegram chat ID
	Frequency        *FrequencyConfig `json:"frequency,omitempty"`       // Optional frequency configuration
}

// DeFiAlertRuleParams holds protocol-specific parameters nested under "params" in JSON
type DeFiAlertRuleParams struct {
	// Common
	MarketTokenContract string `json:"market_token_contract,omitempty"` // Token contract address (Aave) or market_id (Morpho market)
	// Display names (optional, for better logging/alert messages)
	MarketTokenName string `json:"market_token_name,omitempty"` // For Aave: display name of the token (e.g., "USDC")
	MarketTokenPair string `json:"market_token_pair,omitempty"` // For Morpho market: display pair (e.g., "USDC/WETH")
	VaultName       string `json:"vault_name,omitempty"`        // For Morpho vault / Kamino vault: display name
	// Morpho-specific fields
	MarketID                string `json:"market_id,omitempty"`                 // For Morpho market
	BorrowTokenContract     string `json:"borrow_token_contract,omitempty"`     // For Morpho market (loan token)
	CollateralTokenContract string `json:"collateral_token_contract,omitempty"` // For Morpho market
	OracleAddress           string `json:"oracle_address,omitempty"`            // For Morpho market: oracle contract address
	IRMAddress              string `json:"irm_address,omitempty"`               // For Morpho market: Interest Rate Model address
	LLTV                    string `json:"lltv,omitempty"`                      // For Morpho market: Loan-to-Liquidation Value (as string to preserve precision)
	MarketContractAddress   string `json:"market_contract_address,omitempty"`   // For Morpho market: Market contract address (optional, uses default if not provided)
	VaultTokenAddress       string `json:"vault_token_address,omitempty"`       // For Morpho vault / Kamino vault
	DepositTokenContract    string `json:"deposit_token_contract,omitempty"`    // For Morpho vault / Kamino vault
}

// DeFiAlertRuleConfig represents a DeFi protocol alert rule in JSON format
type DeFiAlertRuleConfig struct {
	Protocol         string              `json:"protocol"`           // e.g., "aave", "morpho"
	Category         string              `json:"category,omitempty"` // "market" or "vault" (for Morpho)
	Version          string              `json:"version"`            // e.g., "v3", "v1"
	ChainID          string              `json:"chain_id"`           // Chain ID: "1", "8453", "42161"
	Field            string              `json:"field"`              // "TVL", "APY", "UTILIZATION", "LIQUIDITY"
	Threshold        float64             `json:"threshold"`
	Direction        string              `json:"direction"` // ">=", ">", "=", "<=", "<"
	Enabled          bool                `json:"enabled"`
	RecipientEmail   string              `json:"recipient_email"`            // Email address to send alerts to
	TelegramChatID   string              `json:"telegram_chat_id,omitempty"` // Optional Telegram chat ID
	Frequency        *FrequencyConfig    `json:"frequency,omitempty"`        // Optional frequency configuration
	Params           DeFiAlertRuleParams `json:"params"`                     // Protocol-specific parameters
}

// PredictMarketAlertRuleParams holds prediction market-specific parameters stored in the params JSON column.
type PredictMarketAlertRuleParams struct {
	NegRisk     bool   `json:"negRisk,omitempty"`
	QuestionID  string `json:"question_id,omitempty"`
	Question    string `json:"question,omitempty"`
	ConditionID string `json:"condition_id,omitempty"`
	Outcome     string `json:"outcome,omitempty"` // "YES" or "NO"
	TokenID     string `json:"token_id,omitempty"`
}

// PredictMarketAlertRuleConfig represents a prediction market alert rule.
type PredictMarketAlertRuleConfig struct {
	PredictMarket  string                       `json:"predict_market"`
	Params         PredictMarketAlertRuleParams `json:"params"`
	Field          string                       `json:"field"`                      // "MIDPOINT"
	Threshold      float64                      `json:"threshold"`
	Direction      string                       `json:"direction"`                  // ">=", ">", "=", "<=", "<"
	Enabled        bool                         `json:"enabled"`
	Frequency      *FrequencyConfig             `json:"frequency,omitempty"`
	RecipientEmail string                       `json:"recipient_email"`
	TelegramChatID string                       `json:"telegram_chat_id,omitempty"` // Optional Telegram chat ID
}

// ParsePredictMarketRule converts PredictMarketAlertRuleConfig to core.PredictMarketAlertRule.
func ParsePredictMarketRule(rc PredictMarketAlertRuleConfig) (*core.PredictMarketAlertRule, error) {
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
		return nil, fmt.Errorf("invalid direction '%s' for predict market rule, must be one of: >=, >, =, <=, <", rc.Direction)
	}

	if rc.PredictMarket == "" {
		return nil, fmt.Errorf("predict_market cannot be empty")
	}
	if rc.Params.TokenID == "" {
		return nil, fmt.Errorf("params.token_id cannot be empty for predict market rule")
	}
	if rc.Field != "MIDPOINT" {
		return nil, fmt.Errorf("invalid field '%s' for predict market rule, must be: MIDPOINT", rc.Field)
	}
	if rc.Threshold < 0 {
		return nil, fmt.Errorf("threshold must be non-negative for predict market rule")
	}

	var frequency *core.Frequency
	if rc.Frequency != nil {
		switch rc.Frequency.Unit {
		case FrequencyUnitDay, FrequencyUnitHour:
			if rc.Frequency.Number == nil || *rc.Frequency.Number <= 0 {
				return nil, fmt.Errorf("frequency.number is required and must be positive for unit %s", rc.Frequency.Unit)
			}
			frequency = &core.Frequency{
				Number: *rc.Frequency.Number,
				Unit:   core.FrequencyUnit(rc.Frequency.Unit),
			}
		case FrequencyUnitOnce:
			frequency = &core.Frequency{Unit: core.FrequencyUnitOnce}
		default:
			return nil, fmt.Errorf("invalid frequency.unit '%s', must be one of: DAY, HOUR, ONCE", rc.Frequency.Unit)
		}
	}

	return &core.PredictMarketAlertRule{
		PredictMarket:  rc.PredictMarket,
		TokenID:        rc.Params.TokenID,
		Field:          rc.Field,
		Threshold:      rc.Threshold,
		Direction:      direction,
		Enabled:        rc.Enabled,
		RecipientEmail: rc.RecipientEmail,
		TelegramChatID: rc.TelegramChatID,
		Frequency:      frequency,
		NegRisk:        rc.Params.NegRisk,
		QuestionID:     rc.Params.QuestionID,
		Question:       rc.Params.Question,
		ConditionID:    rc.Params.ConditionID,
		Outcome:        rc.Params.Outcome,
	}, nil
}

// ParsePriceRule converts AlertRuleConfig to core.AlertRule (exported for MySQL/store use).
func ParsePriceRule(rc AlertRuleConfig) (*core.AlertRule, error) {
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
		TelegramChatID: rc.TelegramChatID,
		Frequency:      frequency,
	}, nil
}

// ParseDeFiRule converts DeFiAlertRuleConfig to core.DeFiAlertRule (exported for MySQL/store use).
func ParseDeFiRule(rc DeFiAlertRuleConfig) (*core.DeFiAlertRule, error) {
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

	// Validate protocol-specific fields (from params)
	if rc.Protocol == "morpho" {
		// Morpho requires category
		if rc.Category != "market" && rc.Category != "vault" {
			return nil, fmt.Errorf("category must be 'market' or 'vault' for Morpho protocol")
		}

		if rc.Category == "market" {
			// For Morpho market, validate market_id or market_token_contract
			if rc.Params.MarketID == "" && rc.Params.MarketTokenContract == "" {
				return nil, fmt.Errorf("market_id or market_token_contract is required for Morpho market (in params)")
			}
			// Use market_id if provided, otherwise use market_token_contract
			if rc.Params.MarketID != "" && rc.Params.MarketTokenContract == "" {
				rc.Params.MarketTokenContract = rc.Params.MarketID
			}
		} else if rc.Category == "vault" {
			// For Morpho vault, validate vault_token_address
			if rc.Params.VaultTokenAddress == "" {
				return nil, fmt.Errorf("vault_token_address is required for Morpho vault (in params)")
			}
			// Use vault_token_address as MarketTokenContract for consistency
			if rc.Params.MarketTokenContract == "" {
				rc.Params.MarketTokenContract = rc.Params.VaultTokenAddress
			}
		}
	} else if rc.Protocol == "kamino" {
		// Kamino requires category
		if rc.Category != "vault" {
			return nil, fmt.Errorf("category must be 'vault' for Kamino protocol")
		}

		// For Kamino vault, validate vault_token_address (Solana pubkey)
		if rc.Params.VaultTokenAddress == "" {
			return nil, fmt.Errorf("vault_token_address is required for Kamino vault (in params)")
		}
		// Use vault_token_address as MarketTokenContract for consistency
		if rc.Params.MarketTokenContract == "" {
			rc.Params.MarketTokenContract = rc.Params.VaultTokenAddress
		}
		// Validate deposit_token_contract (Solana mint address)
		if rc.Params.DepositTokenContract == "" {
			return nil, fmt.Errorf("deposit_token_contract is required for Kamino vault (in params)")
		}
	} else {
		// For other protocols (e.g., Aave), validate market token contract
		if rc.Params.MarketTokenContract == "" {
			return nil, fmt.Errorf("market_token_contract cannot be empty in DeFi alert rule (in params)")
		}
	}

	// Validate field
	if rc.Field != "TVL" && rc.Field != "APY" && rc.Field != "UTILIZATION" && rc.Field != "LIQUIDITY" {
		return nil, fmt.Errorf("invalid field '%s' for protocol %s %s, must be one of: TVL, APY, UTILIZATION, LIQUIDITY", rc.Field, rc.Protocol, rc.Version)
	}

	// Validate threshold
	if rc.Threshold < 0 {
		return nil, fmt.Errorf("threshold must be non-negative for protocol %s %s", rc.Protocol, rc.Version)
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

	rule := &core.DeFiAlertRule{
		Protocol:            rc.Protocol,
		Category:            rc.Category,
		Version:             rc.Version,
		ChainID:             rc.ChainID,
		MarketTokenContract: rc.Params.MarketTokenContract,
		Field:               rc.Field,
		Threshold:           rc.Threshold,
		Direction:           direction,
		Enabled:             rc.Enabled,
		RecipientEmail:      rc.RecipientEmail,
		TelegramChatID:      rc.TelegramChatID,
		Frequency:           frequency,
		// Display names (from params)
		MarketTokenName: rc.Params.MarketTokenName,
		MarketTokenPair: rc.Params.MarketTokenPair,
		VaultName:       rc.Params.VaultName,
	}

	// Set Morpho-specific fields (from params)
	if rc.Protocol == "morpho" {
		rule.BorrowTokenContract = rc.Params.BorrowTokenContract
		rule.CollateralTokenContract = rc.Params.CollateralTokenContract
		rule.OracleAddress = rc.Params.OracleAddress
		rule.IRMAddress = rc.Params.IRMAddress
		rule.LLTV = rc.Params.LLTV
		rule.MarketContractAddress = rc.Params.MarketContractAddress
		rule.VaultTokenAddress = rc.Params.VaultTokenAddress
		rule.DepositTokenContract = rc.Params.DepositTokenContract
	}

	// Set Kamino-specific fields (from params)
	if rc.Protocol == "kamino" {
		rule.VaultTokenAddress = rc.Params.VaultTokenAddress
		rule.DepositTokenContract = rc.Params.DepositTokenContract
	}

	return rule, nil
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvBool returns true if the env var is set to "1", "true", "yes" (case-insensitive)
func getEnvBool(key string, defaultValue bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	switch v {
	case "1", "true", "yes", "TRUE", "YES":
		return true
	case "0", "false", "no", "FALSE", "NO":
		return false
	}
	return defaultValue
}

// getEnvInt returns an integer from an env var; if empty or invalid, returns defaultValue
func getEnvInt(key string, defaultValue int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	if n, err := strconv.Atoi(v); err == nil {
		return n
	}
	return defaultValue
}

// getEnvSlice returns a slice from a comma-separated env var; if empty, returns defaultSlice
func getEnvSlice(key string, defaultSlice []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return defaultSlice
	}
	var out []string
	for _, s := range strings.Split(v, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return defaultSlice
	}
	return out
}
