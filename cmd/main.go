package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"crypto-alert/internal/config"
	"crypto-alert/internal/core"
	"crypto-alert/internal/message"
	"crypto-alert/internal/price"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize components
	pythClient := price.NewPythClient(cfg.PythAPIURL, cfg.PythAPIKey)
	decisionEngine := core.NewDecisionEngine()

	// Setup Resend email sender
	if cfg.ResendAPIKey == "" {
		log.Fatal("RESEND_API_KEY is required in .env file")
	}
	if cfg.ResendFromEmail == "" {
		log.Fatal("RESEND_FROM_EMAIL is required in .env file")
	}

	emailSender := message.NewResendEmailSender(cfg.ResendAPIKey, cfg.ResendFromEmail)

	// Load alert rules from JSON config file
	if err := loadAlertRules(decisionEngine, cfg.AlertRulesFile); err != nil {
		log.Fatalf("Failed to load alert rules: %v", err)
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start the alert monitoring loop
	go monitorPrices(ctx, pythClient, decisionEngine, emailSender, cfg)

	log.Println("🚀 Crypto Alert System started")
	
	// Get symbols from alert rules for logging
	rules := decisionEngine.GetRules()
	symbols := make([]string, 0)
	for _, rule := range rules {
		if rule.Enabled {
			symbols = append(symbols, rule.Symbol)
		}
	}
	if len(symbols) > 0 {
		log.Printf("📊 Monitoring symbols: %v", symbols)
	} else {
		log.Println("⚠️  No enabled alert rules found")
	}
	log.Printf("⏱️  Check interval: %d seconds", cfg.CheckInterval)
	log.Println("Press Ctrl+C to stop...")

	// Wait for shutdown signal
	<-sigChan
	log.Println("\n🛑 Shutting down...")
	cancel()
	time.Sleep(1 * time.Second)
	log.Println("✅ Shutdown complete")
}

// monitorPrices continuously monitors prices and triggers alerts
func monitorPrices(
	ctx context.Context,
	pythClient *price.PythClient,
	decisionEngine *core.DecisionEngine,
	sender message.MessageSender,
	cfg *config.Config,
) {
	ticker := time.NewTicker(time.Duration(cfg.CheckInterval) * time.Second)
	defer ticker.Stop()

	// Run immediately on startup
	if err := checkAndAlert(ctx, pythClient, decisionEngine, sender); err != nil {
		log.Printf("Error checking prices: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := checkAndAlert(ctx, pythClient, decisionEngine, sender); err != nil {
				log.Printf("Error checking prices: %v", err)
			}
		}
	}
}

// checkAndAlert checks prices and sends alerts if conditions are met
func checkAndAlert(
	ctx context.Context,
	pythClient *price.PythClient,
	decisionEngine *core.DecisionEngine,
	sender message.MessageSender,
) error {
	// Build symbol to price feed ID mapping from alert rules
	rules := decisionEngine.GetRules()
	symbolToFeedID := make(map[string]string)

	for _, rule := range rules {
		if rule.Enabled {
			symbolToFeedID[rule.Symbol] = rule.PriceFeedID
		}
	}

	if len(symbolToFeedID) == 0 {
		log.Println("⚠️  No enabled alert rules found")
		return nil
	}

	log.Printf("🔍 Checking prices for %d symbol(s)...", len(symbolToFeedID))

	// Fetch prices from Pyth oracle using price feed IDs from rules
	prices, err := pythClient.GetMultiplePrices(ctx, symbolToFeedID)
	if err != nil {
		return fmt.Errorf("failed to fetch prices: %w", err)
	}

	// Display current prices
	for symbol, priceData := range prices {
		if err := priceData.Validate(); err != nil {
			log.Printf("⚠️  Invalid price data for %s: %v", symbol, err)
			continue
		}
		log.Printf("💰 %s: $%g (confidence: %g)", symbol, priceData.Price, priceData.Confidence)
	}

	// Evaluate alert rules
	decisions := decisionEngine.EvaluateAll(prices)

	// Send alerts for triggered rules
	for _, decision := range decisions {
		if decision.ShouldAlert {
			log.Printf("🚨 Alert triggered: %s", decision.Message)
			// Send email to the recipient specified in the alert rule using formatted template
			if err := sender.SendAlert(decision.Rule.RecipientEmail, decision); err != nil {
				log.Printf("❌ Failed to send alert to %s: %v", decision.Rule.RecipientEmail, err)
			}
		}
	}

	return nil
}

// loadAlertRules loads alert rules from JSON config file
func loadAlertRules(engine *core.DecisionEngine, filePath string) error {
	rules, err := config.LoadAlertRules(filePath)
	if err != nil {
		return fmt.Errorf("failed to load alert rules: %w", err)
	}

	// Add all rules to the decision engine
	for _, rule := range rules {
		engine.AddRule(rule)
	}

	log.Printf("✅ Loaded %d alert rule(s) from %s", len(rules), filePath)
	return nil
}
