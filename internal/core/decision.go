package core

import (
	"fmt"
	"time"

	"crypto-alert/internal/price"
)

// Direction indicates the comparison operator for price threshold
type Direction string

const (
	DirectionGreaterThanOrEqual Direction = ">="
	DirectionGreaterThan        Direction = ">"
	DirectionEqual              Direction = "="
	DirectionLessThanOrEqual    Direction = "<="
	DirectionLessThan           Direction = "<"
)

// FrequencyUnit represents the unit for frequency
type FrequencyUnit string

const (
	FrequencyUnitDay   FrequencyUnit = "DAY"
	FrequencyUnitHour  FrequencyUnit = "HOUR"
	FrequencyUnitOnce  FrequencyUnit = "ONCE"
	FrequencyUnitNever FrequencyUnit = "NEVER"
)

// Frequency represents the frequency configuration for an alert rule
type Frequency struct {
	Number int           // Number of units (required for DAY/HOUR, ignored for ONCE and NEVER)
	Unit   FrequencyUnit // DAY, HOUR, ONCE, NEVER
}

// AlertRule defines a price alert rule
type AlertRule struct {
	Symbol         string
	PriceFeedID    string // Pyth price feed ID for this symbol
	Threshold      float64
	Direction      Direction // >=, >, =, <=, <
	Enabled        bool
	RecipientEmail string // Email address to send alerts to
	LastTriggered  *time.Time
	Frequency      *Frequency // Optional frequency configuration
}

// DeFiAlertRule defines a DeFi protocol alert rule
type DeFiAlertRule struct {
	Protocol            string
	Category            string // "market" or "vault" (for Morpho), empty for others
	Version             string
	ChainID             string
	MarketTokenContract string // For Aave: token contract, For Morpho market: market_id, For Morpho vault: vault_token_address
	Field               string // "TVL", "APY", "UTILIZATION", "LIQUIDITY"
	Threshold           float64
	Direction           Direction // >=, >, =, <=, <
	Enabled             bool
	RecipientEmail      string
	LastTriggered       *time.Time
	Frequency           *Frequency
	// Display names (optional, for better logging/alert messages)
	MarketTokenName     string // For Aave: display name of the token (e.g., "USDC")
	MarketTokenPair     string // For Morpho market: display pair (e.g., "USDC/WETH")
	VaultName           string // For Morpho vault: display name of the vault
	// Morpho-specific fields
	BorrowTokenContract   string // For Morpho market (loan token)
	CollateralTokenContract string // For Morpho market
	OracleAddress         string // For Morpho market: oracle contract address
	IRMAddress            string // For Morpho market: Interest Rate Model address
	LLTV                  string // For Morpho market: Loan-to-Liquidation Value
	MarketContractAddress string // For Morpho market: Market contract address (optional)
	VaultTokenAddress     string // For Morpho vault (same as MarketTokenContract)
	DepositTokenContract  string // For Morpho vault
}

// AlertDecision represents the result of evaluating an alert rule
type AlertDecision struct {
	ShouldAlert  bool
	Rule         *AlertRule
	CurrentPrice *price.PriceData
	Message      string
}

// DeFiAlertDecision represents the result of evaluating a DeFi alert rule
type DeFiAlertDecision struct {
	ShouldAlert bool
	Rule        *DeFiAlertRule
	CurrentValue float64
	ChainName   string
	Message     string
}

// DecisionEngine handles price comparison and alert decisions
type DecisionEngine struct {
	rules      []*AlertRule
	defiRules  []*DeFiAlertRule
}

// NewDecisionEngine creates a new decision engine
func NewDecisionEngine() *DecisionEngine {
	return &DecisionEngine{
		rules:     make([]*AlertRule, 0),
		defiRules: make([]*DeFiAlertRule, 0),
	}
}

// AddRule adds an alert rule to the engine
func (e *DecisionEngine) AddRule(rule *AlertRule) {
	e.rules = append(e.rules, rule)
}

// AddDeFiRule adds a DeFi alert rule to the engine
func (e *DecisionEngine) AddDeFiRule(rule *DeFiAlertRule) {
	e.defiRules = append(e.defiRules, rule)
}

// RemoveRule removes an alert rule by symbol
func (e *DecisionEngine) RemoveRule(symbol string) {
	for i, rule := range e.rules {
		if rule.Symbol == symbol {
			e.rules = append(e.rules[:i], e.rules[i+1:]...)
			return
		}
	}
}

// GetRules returns all alert rules
func (e *DecisionEngine) GetRules() []*AlertRule {
	return e.rules
}

// GetDeFiRules returns all DeFi alert rules
func (e *DecisionEngine) GetDeFiRules() []*DeFiAlertRule {
	return e.defiRules
}

// Evaluate checks if a price should trigger an alert based on rules
func (e *DecisionEngine) Evaluate(priceData *price.PriceData) []*AlertDecision {
	decisions := make([]*AlertDecision, 0)

	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}

		if rule.Symbol != priceData.Symbol {
			continue
		}

		shouldAlert := false
		message := ""

		switch rule.Direction {
		case DirectionGreaterThanOrEqual:
			if priceData.Price >= rule.Threshold {
				shouldAlert = true
				message = fmt.Sprintf(
					"🚨 Alert: %s price is %g, which is >= threshold of %g",
					priceData.Symbol,
					priceData.Price,
					rule.Threshold,
				)
			}
		case DirectionGreaterThan:
			if priceData.Price > rule.Threshold {
				shouldAlert = true
				message = fmt.Sprintf(
					"🚨 Alert: %s price is %g, which is > threshold of %g",
					priceData.Symbol,
					priceData.Price,
					rule.Threshold,
				)
			}
		case DirectionEqual:
			// Use a small epsilon for floating point comparison
			epsilon := 0.01
			if priceData.Price >= rule.Threshold-epsilon && priceData.Price <= rule.Threshold+epsilon {
				shouldAlert = true
				message = fmt.Sprintf(
					"🚨 Alert: %s price is %g, which equals threshold of %g",
					priceData.Symbol,
					priceData.Price,
					rule.Threshold,
				)
			}
		case DirectionLessThanOrEqual:
			if priceData.Price <= rule.Threshold {
				shouldAlert = true
				message = fmt.Sprintf(
					"🚨 Alert: %s price is %g, which is <= threshold of %g",
					priceData.Symbol,
					priceData.Price,
					rule.Threshold,
				)
			}
		case DirectionLessThan:
			if priceData.Price < rule.Threshold {
				shouldAlert = true
				message = fmt.Sprintf(
					"🚨 Alert: %s price is %g, which is < threshold of %g",
					priceData.Symbol,
					priceData.Price,
					rule.Threshold,
				)
			}
		}

		if shouldAlert {
			// Handle frequency-based alert suppression
			if rule.Frequency != nil {
				switch rule.Frequency.Unit {
				case FrequencyUnitOnce:
					// ONCE: If already triggered, disable the rule
					if rule.LastTriggered != nil {
						rule.Enabled = false
						continue // Rule already triggered, don't alert again
					}
				case FrequencyUnitNever:
					// NEVER: continue to alert
					continue
				case FrequencyUnitDay:
					// DAY: Check if enough days have passed since last trigger
					if rule.LastTriggered != nil {
						requiredDuration := time.Duration(rule.Frequency.Number) * 24 * time.Hour
						if time.Since(*rule.LastTriggered) < requiredDuration {
							continue // Suppress duplicate alert - not enough time has passed
						}
					}
				case FrequencyUnitHour:
					// HOUR: Check if enough hours have passed since last trigger
					if rule.LastTriggered != nil {
						requiredDuration := time.Duration(rule.Frequency.Number) * time.Hour
						if time.Since(*rule.LastTriggered) < requiredDuration {
							continue // Suppress duplicate alert - not enough time has passed
						}
					}
				}
			} else {
				// Default behavior: suppress duplicate alerts within 1 hour if no frequency is specified
				if rule.LastTriggered != nil {
					if time.Since(*rule.LastTriggered) < time.Hour {
						continue // Suppress duplicate alert
					}
				}
			}

			decisions = append(decisions, &AlertDecision{
				ShouldAlert:  true,
				Rule:         rule,
				CurrentPrice: priceData,
				Message:      message,
			})

			// Update last triggered time
			now := time.Now()
			rule.LastTriggered = &now
		}
	}

	return decisions
}

// EvaluateAll evaluates all rules against multiple price data points
func (e *DecisionEngine) EvaluateAll(prices map[string]*price.PriceData) []*AlertDecision {
	allDecisions := make([]*AlertDecision, 0)

	for _, priceData := range prices {
		decisions := e.Evaluate(priceData)
		allDecisions = append(allDecisions, decisions...)
	}

	return allDecisions
}

// EvaluateDeFi checks if a DeFi value should trigger an alert based on rules
func (e *DecisionEngine) EvaluateDeFi(chainID, tokenAddress, field string, currentValue float64, chainName string) []*DeFiAlertDecision {
	decisions := make([]*DeFiAlertDecision, 0)

	for _, rule := range e.defiRules {
		if !rule.Enabled {
			continue
		}

		// Match rule by chain ID, token address, and field
		if rule.ChainID != chainID || rule.MarketTokenContract != tokenAddress || rule.Field != field {
			continue
		}

		shouldAlert := false
		message := ""

		switch rule.Direction {
		case DirectionGreaterThanOrEqual:
			if currentValue >= rule.Threshold {
				shouldAlert = true
				message = fmt.Sprintf(
					"🚨 Alert: %s %s %s on %s - %s is %g, which is >= threshold of %g",
					rule.Protocol,
					rule.Version,
					rule.Field,
					chainName,
					rule.Field,
					currentValue,
					rule.Threshold,
				)
			}
		case DirectionGreaterThan:
			if currentValue > rule.Threshold {
				shouldAlert = true
				message = fmt.Sprintf(
					"🚨 Alert: %s %s %s on %s - %s is %g, which is > threshold of %g",
					rule.Protocol,
					rule.Version,
					rule.Field,
					chainName,
					rule.Field,
					currentValue,
					rule.Threshold,
				)
			}
		case DirectionEqual:
			// Use a small epsilon for floating point comparison
			epsilon := 0.01
			if currentValue >= rule.Threshold-epsilon && currentValue <= rule.Threshold+epsilon {
				shouldAlert = true
				message = fmt.Sprintf(
					"🚨 Alert: %s %s %s on %s - %s is %g, which equals threshold of %g",
					rule.Protocol,
					rule.Version,
					rule.Field,
					chainName,
					rule.Field,
					currentValue,
					rule.Threshold,
				)
			}
		case DirectionLessThanOrEqual:
			if currentValue <= rule.Threshold {
				shouldAlert = true
				message = fmt.Sprintf(
					"🚨 Alert: %s %s %s on %s - %s is %g, which is <= threshold of %g",
					rule.Protocol,
					rule.Version,
					rule.Field,
					chainName,
					rule.Field,
					currentValue,
					rule.Threshold,
				)
			}
		case DirectionLessThan:
			if currentValue < rule.Threshold {
				shouldAlert = true
				message = fmt.Sprintf(
					"🚨 Alert: %s %s %s on %s - %s is %g, which is < threshold of %g",
					rule.Protocol,
					rule.Version,
					rule.Field,
					chainName,
					rule.Field,
					currentValue,
					rule.Threshold,
				)
			}
		}

		if shouldAlert {
			// Handle frequency-based alert suppression
			if rule.Frequency != nil {
				switch rule.Frequency.Unit {
				case FrequencyUnitOnce:
					// ONCE: If already triggered, disable the rule
					if rule.LastTriggered != nil {
						rule.Enabled = false
						continue // Rule already triggered, don't alert again
					}
				case FrequencyUnitNever:
					// NEVER: continue to alert
					continue
				case FrequencyUnitDay:
					// DAY: Check if enough days have passed since last trigger
					if rule.LastTriggered != nil {
						requiredDuration := time.Duration(rule.Frequency.Number) * 24 * time.Hour
						if time.Since(*rule.LastTriggered) < requiredDuration {
							continue // Suppress duplicate alert - not enough time has passed
						}
					}
				case FrequencyUnitHour:
					// HOUR: Check if enough hours have passed since last trigger
					if rule.LastTriggered != nil {
						requiredDuration := time.Duration(rule.Frequency.Number) * time.Hour
						if time.Since(*rule.LastTriggered) < requiredDuration {
							continue // Suppress duplicate alert - not enough time has passed
						}
					}
				}
			} else {
				// Default behavior: suppress duplicate alerts within 1 hour if no frequency is specified
				if rule.LastTriggered != nil {
					if time.Since(*rule.LastTriggered) < time.Hour {
						continue // Suppress duplicate alert
					}
				}
			}

			decisions = append(decisions, &DeFiAlertDecision{
				ShouldAlert:  true,
				Rule:         rule,
				CurrentValue: currentValue,
				ChainName:    chainName,
				Message:      message,
			})

			// Update last triggered time
			now := time.Now()
			rule.LastTriggered = &now
		}
	}

	return decisions
}
