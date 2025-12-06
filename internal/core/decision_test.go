package core

import (
	"testing"
	"time"

	"crypto-alert/internal/price"
)

func TestDecisionEngine_Evaluate(t *testing.T) {
	// Test case 1: >= operator - Price above threshold should trigger alert
	engine1 := NewDecisionEngine()
	engine1.AddRule(&AlertRule{
		Symbol:      "BTC/USD",
		PriceFeedID: "0xe62df6c8b4a85fe1a67db44dc12de5db330f7ac66b72dc658afedf0f4a415b43",
		Threshold:   50000.0,
		Direction:   DirectionGreaterThanOrEqual,
		Enabled:    true,
	})

	priceData1 := &price.PriceData{
		Symbol:     "BTC/USD",
		Price:      51000.0,
		Timestamp:  time.Now(),
		Confidence: 0.95,
	}

	decisions := engine1.Evaluate(priceData1)
	if len(decisions) != 1 {
		t.Errorf("Expected 1 decision, got %d", len(decisions))
	}
	if !decisions[0].ShouldAlert {
		t.Error("Expected alert to be triggered")
	}

	// Test case 2: >= operator - Price equal to threshold should trigger alert
	engine2 := NewDecisionEngine()
	engine2.AddRule(&AlertRule{
		Symbol:      "BTC/USD",
		PriceFeedID: "0xe62df6c8b4a85fe1a67db44dc12de5db330f7ac66b72dc658afedf0f4a415b43",
		Threshold:   50000.0,
		Direction:   DirectionGreaterThanOrEqual,
		Enabled:    true,
	})

	priceData2 := &price.PriceData{
		Symbol:     "BTC/USD",
		Price:      50000.0,
		Timestamp:  time.Now(),
		Confidence: 0.95,
	}

	decisions = engine2.Evaluate(priceData2)
	if len(decisions) != 1 {
		t.Errorf("Expected 1 decision for >= with equal price, got %d", len(decisions))
	}

	// Test case 3: > operator - Price equal to threshold should NOT trigger alert
	engine3 := NewDecisionEngine()
	engine3.AddRule(&AlertRule{
		Symbol:      "ETH/USD",
		PriceFeedID: "0xff61491a931112ddf1bd8147cd1b641375f79f5825126d665480874634fd0ace",
		Threshold:   2000.0,
		Direction:   DirectionGreaterThan,
		Enabled:    true,
	})

	priceData3 := &price.PriceData{
		Symbol:     "ETH/USD",
		Price:      2000.0,
		Timestamp:  time.Now(),
		Confidence: 0.95,
	}

	decisions = engine3.Evaluate(priceData3)
	if len(decisions) != 0 {
		t.Errorf("Expected 0 decisions for > with equal price, got %d", len(decisions))
	}

	// Test case 4: > operator - Price above threshold should trigger alert
	engine4 := NewDecisionEngine()
	engine4.AddRule(&AlertRule{
		Symbol:      "ETH/USD",
		PriceFeedID: "0xff61491a931112ddf1bd8147cd1b641375f79f5825126d665480874634fd0ace",
		Threshold:   2000.0,
		Direction:   DirectionGreaterThan,
		Enabled:    true,
	})

	priceData4 := &price.PriceData{
		Symbol:     "ETH/USD",
		Price:      2100.0,
		Timestamp:  time.Now(),
		Confidence: 0.95,
	}

	decisions = engine4.Evaluate(priceData4)
	if len(decisions) != 1 {
		t.Errorf("Expected 1 decision, got %d", len(decisions))
	}

	// Test case 5: <= operator - Price below threshold should trigger alert
	engine5 := NewDecisionEngine()
	engine5.AddRule(&AlertRule{
		Symbol:      "SOL/USD",
		PriceFeedID: "0xef0d8b6fda2ceba41da15d4095d1da392a0d2f8ed0c6c7bc0f4cfac8c280b56d",
		Threshold:   100.0,
		Direction:   DirectionLessThanOrEqual,
		Enabled:    true,
	})

	priceData5 := &price.PriceData{
		Symbol:     "SOL/USD",
		Price:      90.0,
		Timestamp:  time.Now(),
		Confidence: 0.95,
	}

	decisions = engine5.Evaluate(priceData5)
	if len(decisions) != 1 {
		t.Errorf("Expected 1 decision, got %d", len(decisions))
	}

	// Test case 6: < operator - Price equal to threshold should NOT trigger alert
	engine6 := NewDecisionEngine()
	engine6.AddRule(&AlertRule{
		Symbol:      "USDC/USD",
		PriceFeedID: "0xeaa020c61cc479712813461ce153894a96a6c00b21ed0cfc2798d1f9a9e9c94a",
		Threshold:   1.0,
		Direction:   DirectionLessThan,
		Enabled:    true,
	})

	priceData6 := &price.PriceData{
		Symbol:     "USDC/USD",
		Price:      1.0,
		Timestamp:  time.Now(),
		Confidence: 0.95,
	}

	decisions = engine6.Evaluate(priceData6)
	if len(decisions) != 0 {
		t.Errorf("Expected 0 decisions for < with equal price, got %d", len(decisions))
	}

	// Test case 7: = operator - Price equal to threshold should trigger alert
	engine7 := NewDecisionEngine()
	engine7.AddRule(&AlertRule{
		Symbol:      "USDT/USD",
		PriceFeedID: "0xeaa020c61cc479712813461ce153894a96a6c00b21ed0cfc2798d1f9a9e9c94a",
		Threshold:   1.0,
		Direction:   DirectionEqual,
		Enabled:    true,
	})

	priceData7 := &price.PriceData{
		Symbol:     "USDT/USD",
		Price:      1.0,
		Timestamp:  time.Now(),
		Confidence: 0.95,
	}

	decisions = engine7.Evaluate(priceData7)
	if len(decisions) != 1 {
		t.Errorf("Expected 1 decision for = with equal price, got %d", len(decisions))
	}
}

func TestDecisionEngine_DisabledRule(t *testing.T) {
	engine := NewDecisionEngine()

	engine.AddRule(&AlertRule{
		Symbol:      "BTC/USD",
		PriceFeedID: "0xe62df6c8b4a85fe1a67db44dc12de5db330f7ac66b72dc658afedf0f4a415b43",
		Threshold:   50000.0,
		Direction:   DirectionGreaterThanOrEqual,
		Enabled:     false, // Disabled rule
	})

	priceData := &price.PriceData{
		Symbol:     "BTC/USD",
		Price:      51000.0,
		Timestamp:  time.Now(),
		Confidence: 0.95,
	}

	decisions := engine.Evaluate(priceData)
	if len(decisions) != 0 {
		t.Errorf("Expected 0 decisions for disabled rule, got %d", len(decisions))
	}
}

