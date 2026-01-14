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

	// Logging Configuration
	LogDir string // Directory for log files (default: "logs")
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
		LogDir:          getEnv("LOG_DIR", "logs"), // Default log directory
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
	Protocol            string           `json:"protocol"`                        // e.g., "aave", "morpho"
	Category            string           `json:"category,omitempty"`              // "market" or "vault" (for Morpho)
	Version             string           `json:"version"`                         // e.g., "v3", "v1"
	ChainID             string           `json:"chain_id"`                        // Chain ID: "1", "8453", "42161"
	MarketTokenContract string           `json:"market_token_contract,omitempty"` // Token contract address (Aave) or market_id (Morpho market)
	Field               string           `json:"field"`                           // "TVL", "APY", "UTILIZATION", "LIQUIDITY"
	Threshold           float64          `json:"threshold"`
	Direction           string           `json:"direction"` // ">=", ">", "=", "<=", "<"
	Enabled             bool             `json:"enabled"`
	RecipientEmail      string           `json:"recipient_email"`     // Email address to send alerts to
	Frequency           *FrequencyConfig `json:"frequency,omitempty"` // Optional frequency configuration
	// Display names (optional, for better logging/alert messages)
	MarketTokenName string `json:"market_token_name,omitempty"` // For Aave: display name of the token (e.g., "USDC")
	MarketTokenPair string `json:"market_token_pair,omitempty"` // For Morpho market: display pair (e.g., "USDC/WETH")
	VaultName       string `json:"vault_name,omitempty"`        // For Morpho vault: display name of the vault
	// Morpho-specific fields
	MarketID                string `json:"market_id,omitempty"`                 // For Morpho market
	BorrowTokenContract     string `json:"borrow_token_contract,omitempty"`     // For Morpho market (loan token)
	CollateralTokenContract string `json:"collateral_token_contract,omitempty"` // For Morpho market
	OracleAddress           string `json:"oracle_address,omitempty"`            // For Morpho market: oracle contract address
	IRMAddress              string `json:"irm_address,omitempty"`               // For Morpho market: Interest Rate Model address
	LLTV                    string `json:"lltv,omitempty"`                      // For Morpho market: Loan-to-Liquidation Value (as string to preserve precision)
	MarketContractAddress   string `json:"market_contract_address,omitempty"`   // For Morpho market: Market contract address (optional, uses default if not provided)
	VaultTokenAddress       string `json:"vault_token_address,omitempty"`       // For Morpho vault
	DepositTokenContract    string `json:"deposit_token_contract,omitempty"`    // For Morpho vault
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

	// Validate protocol-specific fields
	if rc.Protocol == "morpho" {
		// Morpho requires category
		if rc.Category != "market" && rc.Category != "vault" {
			return nil, fmt.Errorf("category must be 'market' or 'vault' for Morpho protocol")
		}

		if rc.Category == "market" {
			// For Morpho market, validate market_id or market_token_contract
			if rc.MarketID == "" && rc.MarketTokenContract == "" {
				return nil, fmt.Errorf("market_id or market_token_contract is required for Morpho market")
			}
			// Use market_id if provided, otherwise use market_token_contract
			if rc.MarketID != "" && rc.MarketTokenContract == "" {
				rc.MarketTokenContract = rc.MarketID
			}
		} else if rc.Category == "vault" {
			// For Morpho vault, validate vault_token_address
			if rc.VaultTokenAddress == "" {
				return nil, fmt.Errorf("vault_token_address is required for Morpho vault")
			}
			// Use vault_token_address as MarketTokenContract for consistency
			if rc.MarketTokenContract == "" {
				rc.MarketTokenContract = rc.VaultTokenAddress
			}
		}
	} else if rc.Protocol == "kamino" {
		// Kamino requires category
		if rc.Category != "vault" {
			return nil, fmt.Errorf("category must be 'vault' for Kamino protocol")
		}

		// For Kamino vault, validate vault_token_address (Solana pubkey)
		if rc.VaultTokenAddress == "" {
			return nil, fmt.Errorf("vault_token_address is required for Kamino vault")
		}
		// Use vault_token_address as MarketTokenContract for consistency
		if rc.MarketTokenContract == "" {
			rc.MarketTokenContract = rc.VaultTokenAddress
		}
		// Validate deposit_token_contract (Solana mint address)
		if rc.DepositTokenContract == "" {
			return nil, fmt.Errorf("deposit_token_contract is required for Kamino vault")
		}
	} else {
		// For other protocols (e.g., Aave), validate market token contract
		if rc.MarketTokenContract == "" {
			return nil, fmt.Errorf("market_token_contract cannot be empty in DeFi alert rule")
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

	rule := &core.DeFiAlertRule{
		Protocol:            rc.Protocol,
		Category:            rc.Category,
		Version:             rc.Version,
		ChainID:             rc.ChainID,
		MarketTokenContract: rc.MarketTokenContract,
		Field:               rc.Field,
		Threshold:           rc.Threshold,
		Direction:           direction,
		Enabled:             rc.Enabled,
		RecipientEmail:      rc.RecipientEmail,
		Frequency:           frequency,
		// Display names
		MarketTokenName: rc.MarketTokenName,
		MarketTokenPair: rc.MarketTokenPair,
		VaultName:       rc.VaultName,
	}

	// Set Morpho-specific fields
	if rc.Protocol == "morpho" {
		rule.BorrowTokenContract = rc.BorrowTokenContract
		rule.CollateralTokenContract = rc.CollateralTokenContract
		rule.OracleAddress = rc.OracleAddress
		rule.IRMAddress = rc.IRMAddress
		rule.LLTV = rc.LLTV
		rule.MarketContractAddress = rc.MarketContractAddress
		rule.VaultTokenAddress = rc.VaultTokenAddress
		rule.DepositTokenContract = rc.DepositTokenContract
	}

	// Set Kamino-specific fields
	if rc.Protocol == "kamino" {
		rule.VaultTokenAddress = rc.VaultTokenAddress
		rule.DepositTokenContract = rc.DepositTokenContract
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
