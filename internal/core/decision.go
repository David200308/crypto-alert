package core

import (
	"fmt"
	"sync"
	"time"

	"crypto-alert/internal/data/price"
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
	ID             int64 // MySQL row ID — used for hot-swap matching
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
	ID                      int64 // MySQL row ID — used for hot-swap matching
	Protocol                string
	Category                string // "market" or "vault" (for Morpho), empty for others
	Version                 string
	ChainID                 string
	MarketTokenContract     string // For Aave: token contract, For Morpho market: market_id, For Morpho vault: vault_token_address
	Field                   string // "TVL", "APY", "UTILIZATION", "LIQUIDITY"
	Threshold               float64
	Direction               Direction // >=, >, =, <=, <
	Enabled                 bool
	RecipientEmail          string
	LastTriggered           *time.Time
	Frequency               *Frequency
	// Display names (optional, for better logging/alert messages)
	MarketTokenName         string // For Aave: display name of the token (e.g., "USDC")
	MarketTokenPair         string // For Morpho market: display pair (e.g., "USDC/WETH")
	VaultName               string // For Morpho vault: display name of the vault
	// Morpho-specific fields
	BorrowTokenContract     string // For Morpho market (loan token)
	CollateralTokenContract string // For Morpho market
	OracleAddress           string // For Morpho market: oracle contract address
	IRMAddress              string // For Morpho market: Interest Rate Model address
	LLTV                    string // For Morpho market: Loan-to-Liquidation Value
	MarketContractAddress   string // For Morpho market: Market contract address (optional)
	VaultTokenAddress       string // For Morpho vault (same as MarketTokenContract)
	DepositTokenContract    string // For Morpho vault
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
	ShouldAlert  bool
	Rule         *DeFiAlertRule
	CurrentValue float64
	ChainName    string
	Message      string
}

// PredictMarketAlertRule defines a prediction market alert rule.
// Threshold comparison is performed against the midpoint price.
type PredictMarketAlertRule struct {
	ID             int64 // MySQL row ID — used for hot-swap matching
	PredictMarket  string     // e.g., "polymarket"
	TokenID        string     // CLOB token ID to monitor
	Field          string     // "MIDPOINT"
	Threshold      float64
	Direction      Direction
	Enabled        bool
	RecipientEmail string
	LastTriggered  *time.Time
	Frequency      *Frequency
	// Display context (populated from params)
	NegRisk     bool
	QuestionID  string
	Question    string
	ConditionID string
	Outcome     string // "YES" or "NO"
}

// PredictMarketAlertDecision represents the result of evaluating a prediction market alert rule.
type PredictMarketAlertDecision struct {
	ShouldAlert      bool
	Rule             *PredictMarketAlertRule
	CurrentMidpoint  float64
	CurrentBuyPrice  float64
	CurrentSellPrice float64
	Message          string
}

// DecisionEngine handles price comparison and alert decisions.
// All exported methods are thread-safe.
type DecisionEngine struct {
	mu                 sync.Mutex
	rules              []*AlertRule
	defiRules          []*DeFiAlertRule
	predictMarketRules []*PredictMarketAlertRule
}

// NewDecisionEngine creates a new decision engine
func NewDecisionEngine() *DecisionEngine {
	return &DecisionEngine{
		rules:              make([]*AlertRule, 0),
		defiRules:          make([]*DeFiAlertRule, 0),
		predictMarketRules: make([]*PredictMarketAlertRule, 0),
	}
}

// AddRule adds an alert rule to the engine
func (e *DecisionEngine) AddRule(rule *AlertRule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = append(e.rules, rule)
}

// AddDeFiRule adds a DeFi alert rule to the engine
func (e *DecisionEngine) AddDeFiRule(rule *DeFiAlertRule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.defiRules = append(e.defiRules, rule)
}

// AddPredictMarketRule adds a prediction market alert rule to the engine
func (e *DecisionEngine) AddPredictMarketRule(rule *PredictMarketAlertRule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.predictMarketRules = append(e.predictMarketRules, rule)
}

// RemoveRule removes an alert rule by symbol
func (e *DecisionEngine) RemoveRule(symbol string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, rule := range e.rules {
		if rule.Symbol == symbol {
			e.rules = append(e.rules[:i], e.rules[i+1:]...)
			return
		}
	}
}

// GetRules returns a snapshot of all alert rules
func (e *DecisionEngine) GetRules() []*AlertRule {
	e.mu.Lock()
	defer e.mu.Unlock()
	cp := make([]*AlertRule, len(e.rules))
	copy(cp, e.rules)
	return cp
}

// GetDeFiRules returns a snapshot of all DeFi alert rules
func (e *DecisionEngine) GetDeFiRules() []*DeFiAlertRule {
	e.mu.Lock()
	defer e.mu.Unlock()
	cp := make([]*DeFiAlertRule, len(e.defiRules))
	copy(cp, e.defiRules)
	return cp
}

// GetPredictMarketRules returns a snapshot of all prediction market alert rules
func (e *DecisionEngine) GetPredictMarketRules() []*PredictMarketAlertRule {
	e.mu.Lock()
	defer e.mu.Unlock()
	cp := make([]*PredictMarketAlertRule, len(e.predictMarketRules))
	copy(cp, e.predictMarketRules)
	return cp
}

// ReplaceRules atomically swaps all rule sets, preserving LastTriggered from
// existing rules that share the same MySQL ID. Call this to hot-reload rules
// from the database without restarting the process.
func (e *DecisionEngine) ReplaceRules(price []*AlertRule, defi []*DeFiAlertRule, predict []*PredictMarketAlertRule) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Build lookup maps keyed by MySQL ID to carry over in-memory state.
	oldPrice := make(map[int64]*AlertRule, len(e.rules))
	for _, r := range e.rules {
		if r.ID != 0 {
			oldPrice[r.ID] = r
		}
	}
	oldDefi := make(map[int64]*DeFiAlertRule, len(e.defiRules))
	for _, r := range e.defiRules {
		if r.ID != 0 {
			oldDefi[r.ID] = r
		}
	}
	oldPredict := make(map[int64]*PredictMarketAlertRule, len(e.predictMarketRules))
	for _, r := range e.predictMarketRules {
		if r.ID != 0 {
			oldPredict[r.ID] = r
		}
	}

	// Carry LastTriggered forward so frequency suppression survives a reload.
	for _, r := range price {
		if old, ok := oldPrice[r.ID]; ok {
			r.LastTriggered = old.LastTriggered
		}
	}
	for _, r := range defi {
		if old, ok := oldDefi[r.ID]; ok {
			r.LastTriggered = old.LastTriggered
		}
	}
	for _, r := range predict {
		if old, ok := oldPredict[r.ID]; ok {
			r.LastTriggered = old.LastTriggered
		}
	}

	e.rules = price
	e.defiRules = defi
	e.predictMarketRules = predict
}

// Evaluate checks if a price should trigger an alert based on rules.
func (e *DecisionEngine) Evaluate(priceData *price.PriceData) []*AlertDecision {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.evaluateLocked(priceData)
}

// evaluateLocked runs evaluation for a single price; caller must hold e.mu.
func (e *DecisionEngine) evaluateLocked(priceData *price.PriceData) []*AlertDecision {
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
	e.mu.Lock()
	defer e.mu.Unlock()

	allDecisions := make([]*AlertDecision, 0)
	for _, priceData := range prices {
		decisions := e.evaluateLocked(priceData)
		allDecisions = append(allDecisions, decisions...)
	}

	return allDecisions
}

// EvaluatePredictMarket checks if a prediction market midpoint should trigger an alert.
// buyPrice and sellPrice are passed through to the decision for inclusion in alert emails.
func (e *DecisionEngine) EvaluatePredictMarket(tokenID string, midpoint, buyPrice, sellPrice float64) []*PredictMarketAlertDecision {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.evaluatePredictMarketLocked(tokenID, midpoint, buyPrice, sellPrice)
}

// evaluatePredictMarketLocked is the lock-free implementation; caller must hold e.mu.
func (e *DecisionEngine) evaluatePredictMarketLocked(tokenID string, midpoint, buyPrice, sellPrice float64) []*PredictMarketAlertDecision {
	decisions := make([]*PredictMarketAlertDecision, 0)

	for _, rule := range e.predictMarketRules {
		if !rule.Enabled {
			continue
		}
		if rule.TokenID != tokenID {
			continue
		}

		shouldAlert := false
		message := ""

		switch rule.Direction {
		case DirectionGreaterThanOrEqual:
			if midpoint >= rule.Threshold {
				shouldAlert = true
				message = fmt.Sprintf(
					"🚨 Alert: Polymarket token %s midpoint is %.4f, which is >= threshold of %g",
					tokenID, midpoint, rule.Threshold,
				)
			}
		case DirectionGreaterThan:
			if midpoint > rule.Threshold {
				shouldAlert = true
				message = fmt.Sprintf(
					"🚨 Alert: Polymarket token %s midpoint is %.4f, which is > threshold of %g",
					tokenID, midpoint, rule.Threshold,
				)
			}
		case DirectionEqual:
			epsilon := 0.0001
			if midpoint >= rule.Threshold-epsilon && midpoint <= rule.Threshold+epsilon {
				shouldAlert = true
				message = fmt.Sprintf(
					"🚨 Alert: Polymarket token %s midpoint is %.4f, which equals threshold of %g",
					tokenID, midpoint, rule.Threshold,
				)
			}
		case DirectionLessThanOrEqual:
			if midpoint <= rule.Threshold {
				shouldAlert = true
				message = fmt.Sprintf(
					"🚨 Alert: Polymarket token %s midpoint is %.4f, which is <= threshold of %g",
					tokenID, midpoint, rule.Threshold,
				)
			}
		case DirectionLessThan:
			if midpoint < rule.Threshold {
				shouldAlert = true
				message = fmt.Sprintf(
					"🚨 Alert: Polymarket token %s midpoint is %.4f, which is < threshold of %g",
					tokenID, midpoint, rule.Threshold,
				)
			}
		}

		if shouldAlert {
			if rule.Frequency != nil {
				switch rule.Frequency.Unit {
				case FrequencyUnitOnce:
					if rule.LastTriggered != nil {
						rule.Enabled = false
						continue
					}
				case FrequencyUnitNever:
					continue
				case FrequencyUnitDay:
					if rule.LastTriggered != nil {
						requiredDuration := time.Duration(rule.Frequency.Number) * 24 * time.Hour
						if time.Since(*rule.LastTriggered) < requiredDuration {
							continue
						}
					}
				case FrequencyUnitHour:
					if rule.LastTriggered != nil {
						requiredDuration := time.Duration(rule.Frequency.Number) * time.Hour
						if time.Since(*rule.LastTriggered) < requiredDuration {
							continue
						}
					}
				}
			} else {
				if rule.LastTriggered != nil {
					if time.Since(*rule.LastTriggered) < time.Hour {
						continue
					}
				}
			}

			decisions = append(decisions, &PredictMarketAlertDecision{
				ShouldAlert:      true,
				Rule:             rule,
				CurrentMidpoint:  midpoint,
				CurrentBuyPrice:  buyPrice,
				CurrentSellPrice: sellPrice,
				Message:          message,
			})

			now := time.Now()
			rule.LastTriggered = &now
		}
	}

	return decisions
}

// EvaluateDeFi checks if a DeFi value should trigger an alert based on rules
func (e *DecisionEngine) EvaluateDeFi(chainID, tokenAddress, field string, currentValue float64, chainName string) []*DeFiAlertDecision {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.evaluateDeFiLocked(chainID, tokenAddress, field, currentValue, chainName)
}

// evaluateDeFiLocked is the lock-free implementation; caller must hold e.mu.
func (e *DecisionEngine) evaluateDeFiLocked(chainID, tokenAddress, field string, currentValue float64, chainName string) []*DeFiAlertDecision {
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
