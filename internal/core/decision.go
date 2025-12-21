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
	FrequencyUnitDay  FrequencyUnit = "DAY"
	FrequencyUnitHour FrequencyUnit = "HOUR"
	FrequencyUnitOnce FrequencyUnit = "ONCE"
)

// Frequency represents the frequency configuration for an alert rule
type Frequency struct {
	Number int           // Number of units (required for DAY/HOUR, ignored for ONCE)
	Unit   FrequencyUnit // DAY, HOUR, or ONCE
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

// AlertDecision represents the result of evaluating an alert rule
type AlertDecision struct {
	ShouldAlert  bool
	Rule         *AlertRule
	CurrentPrice *price.PriceData
	Message      string
}

// DecisionEngine handles price comparison and alert decisions
type DecisionEngine struct {
	rules []*AlertRule
}

// NewDecisionEngine creates a new decision engine
func NewDecisionEngine() *DecisionEngine {
	return &DecisionEngine{
		rules: make([]*AlertRule, 0),
	}
}

// AddRule adds an alert rule to the engine
func (e *DecisionEngine) AddRule(rule *AlertRule) {
	e.rules = append(e.rules, rule)
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
