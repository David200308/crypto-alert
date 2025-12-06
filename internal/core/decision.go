package core

import (
	"fmt"
	"time"

	"crypto-alert/internal/price"
)

// AlertRule defines a price alert rule
type AlertRule struct {
	Symbol         string
	PriceFeedID    string // Pyth price feed ID for this symbol
	Threshold      float64
	Direction      Direction // >=, >, =, <=, <
	Enabled        bool
	RecipientEmail string // Email address to send alerts to
	LastTriggered  *time.Time
}

// Direction indicates the comparison operator for price threshold
type Direction string

const (
	DirectionGreaterThanOrEqual Direction = ">="
	DirectionGreaterThan        Direction = ">"
	DirectionEqual              Direction = "="
	DirectionLessThanOrEqual    Direction = "<="
	DirectionLessThan           Direction = "<"
)

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
					"🚨 Alert: %s price is %.2f, which is >= threshold of %.2f",
					priceData.Symbol,
					priceData.Price,
					rule.Threshold,
				)
			}
		case DirectionGreaterThan:
			if priceData.Price > rule.Threshold {
				shouldAlert = true
				message = fmt.Sprintf(
					"🚨 Alert: %s price is %.2f, which is > threshold of %.2f",
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
					"🚨 Alert: %s price is %.2f, which equals threshold of %.2f",
					priceData.Symbol,
					priceData.Price,
					rule.Threshold,
				)
			}
		case DirectionLessThanOrEqual:
			if priceData.Price <= rule.Threshold {
				shouldAlert = true
				message = fmt.Sprintf(
					"🚨 Alert: %s price is %.2f, which is <= threshold of %.2f",
					priceData.Symbol,
					priceData.Price,
					rule.Threshold,
				)
			}
		case DirectionLessThan:
			if priceData.Price < rule.Threshold {
				shouldAlert = true
				message = fmt.Sprintf(
					"🚨 Alert: %s price is %.2f, which is < threshold of %.2f",
					priceData.Symbol,
					priceData.Price,
					rule.Threshold,
				)
			}
		}

		if shouldAlert {
			// Check if we should suppress duplicate alerts (e.g., within 1 hour)
			if rule.LastTriggered != nil {
				if time.Since(*rule.LastTriggered) < time.Hour {
					continue // Suppress duplicate alert
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
