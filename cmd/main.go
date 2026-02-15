package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"crypto-alert/internal/store"
	"crypto-alert/internal/config"
	"crypto-alert/internal/core"
	"crypto-alert/internal/defi"
	"crypto-alert/internal/logger"
	"crypto-alert/internal/message"
	"crypto-alert/internal/price"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger with date-based file rotation and optional Elasticsearch
	esConfig := &logger.ESConfig{
		Enabled:   cfg.ESEnabled,
		Addresses: cfg.ESAddresses,
		Index:     cfg.ESIndex,
	}
	if err := logger.InitLogger(cfg.LogDir, esConfig); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.GetLogger().Close()

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

	// Load alert rules from file or MySQL
	switch cfg.AlertRulesSource {
	case "mysql":
		if err := loadAlertRulesFromMySQL(decisionEngine, cfg.MySQLDSN); err != nil {
			log.Fatalf("Failed to load alert rules from MySQL: %v", err)
		}
	default:
		if err := loadAlertRules(decisionEngine, cfg.AlertRulesFile); err != nil {
			log.Fatalf("Failed to load alert rules: %v", err)
		}
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start the alert monitoring loops
	go monitorPrices(ctx, pythClient, decisionEngine, emailSender, cfg)
	go monitorDeFi(ctx, decisionEngine, emailSender, cfg)

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
		log.Printf("📊 Monitoring price symbols: %v", symbols)
	}

	// Get DeFi rules for logging
	defiRules := decisionEngine.GetDeFiRules()
	defi.LogDeFiRules(defiRules)

	if len(symbols) == 0 && len(defiRules) == 0 {
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
			// if err := sender.SendAlert(decision.Rule.RecipientEmail, decision); err != nil {
			// 	log.Printf("❌ Failed to send alert to %s: %v", decision.Rule.RecipientEmail, err)
			// }
		}
	}

	return nil
}

// monitorDeFi continuously monitors DeFi protocols and triggers alerts
func monitorDeFi(
	ctx context.Context,
	decisionEngine *core.DecisionEngine,
	sender message.MessageSender,
	cfg *config.Config,
) {
	ticker := time.NewTicker(time.Duration(cfg.CheckInterval) * time.Second)
	defer ticker.Stop()

	// Run immediately on startup
	if err := checkAndAlertDeFi(ctx, decisionEngine, sender); err != nil {
		log.Printf("Error checking DeFi: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := checkAndAlertDeFi(ctx, decisionEngine, sender); err != nil {
				log.Printf("Error checking DeFi: %v", err)
			}
		}
	}
}

// checkAndAlertDeFi checks DeFi values and sends alerts if conditions are met
func checkAndAlertDeFi(
	ctx context.Context,
	decisionEngine *core.DecisionEngine,
	sender message.MessageSender,
) error {
	defiRules := decisionEngine.GetDeFiRules()
	if len(defiRules) == 0 {
		return nil
	}

	clientManager := defi.NewClientManager()
	defer clientManager.Close()

	log.Printf("🔍 Checking DeFi protocols for %d rule(s)...", len(defiRules))

	for _, rule := range defiRules {
		if !rule.Enabled {
			continue
		}

		value, chainName, err := clientManager.GetFieldValue(ctx, rule)
		if err != nil {
			log.Printf("⚠️  %v", err)
			continue
		}

		categoryStr := defi.GetCategoryString(rule)
		displayName := defi.GetDisplayName(rule)
		log.Printf("💰 %s%s %s on %s - %s%s: %g", rule.Protocol, categoryStr, rule.Version, chainName, rule.Field, displayName, value)

		// Evaluate alert rules
		identifier := defi.GetIdentifier(rule)
		decisions := decisionEngine.EvaluateDeFi(rule.ChainID, identifier, rule.Field, value, chainName)

		// Send alerts for triggered rules
		for _, decision := range decisions {
			if decision.ShouldAlert {
				log.Printf("🚨 Alert triggered: %s", decision.Message)
				// Send email to the recipient specified in the alert rule
				if err := sender.SendDeFiAlert(decision.Rule.RecipientEmail, decision); err != nil {
					log.Printf("❌ Failed to send alert to %s: %v", decision.Rule.RecipientEmail, err)
				}
			}
		}
	}

	return nil
}

// loadAlertRules loads alert rules from JSON config file
func loadAlertRules(engine *core.DecisionEngine, filePath string) error {
	priceRules, defiRules, err := config.LoadAlertRules(filePath)
	if err != nil {
		return fmt.Errorf("failed to load alert rules: %w", err)
	}
	return addAlertRulesToEngine(engine, priceRules, defiRules, "file "+filePath)
}

// loadAlertRulesFromMySQL loads alert rules from MySQL (web3.alert_rule_token_config, web3.alert_rule_defi_config)
func loadAlertRulesFromMySQL(engine *core.DecisionEngine, dsn string) error {
	priceRules, defiRules, err := store.LoadAlertRulesFromMySQL(dsn)
	if err != nil {
		return err
	}
	return addAlertRulesToEngine(engine, priceRules, defiRules, "MySQL")
}

func addAlertRulesToEngine(engine *core.DecisionEngine, priceRules []*core.AlertRule, defiRules []*core.DeFiAlertRule, source string) error {
	for _, rule := range priceRules {
		engine.AddRule(rule)
	}
	for _, rule := range defiRules {
		engine.AddDeFiRule(rule)
	}
	totalRules := len(priceRules) + len(defiRules)
	log.Printf("✅ Loaded %d price rule(s) and %d DeFi rule(s) from %s", len(priceRules), len(defiRules), source)
	if totalRules == 0 {
		return fmt.Errorf("no alert rules found in %s", source)
	}
	return nil
}
