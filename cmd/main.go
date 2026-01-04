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
	"crypto-alert/internal/defi/aave"
	"crypto-alert/internal/defi/morpho"
	"crypto-alert/internal/message"
	"crypto-alert/internal/price"

	"github.com/ethereum/go-ethereum/common"
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
	if len(defiRules) > 0 {
		log.Printf("📊 Monitoring DeFi protocols: %d rule(s)", len(defiRules))
		for _, rule := range defiRules {
			if rule.Enabled {
				var chainName string
				if rule.Protocol == "aave" {
					chainName, _ = aave.GetChainNameFromID(rule.ChainID)
				} else if rule.Protocol == "morpho" {
					chainName, _ = morpho.GetChainNameFromID(rule.ChainID)
				}
				categoryStr := ""
				if rule.Category != "" {
					categoryStr = " " + rule.Category
				}
				// Use display names if available
				displayName := ""
				if rule.Protocol == "aave" && rule.MarketTokenName != "" {
					displayName = " (" + rule.MarketTokenName + ")"
				} else if rule.Protocol == "morpho" && rule.Category == "market" && rule.MarketTokenPair != "" {
					displayName = " (" + rule.MarketTokenPair + ")"
				} else if rule.Protocol == "morpho" && rule.Category == "vault" && rule.VaultName != "" {
					displayName = " (" + rule.VaultName + ")"
				}
				log.Printf("  - %s%s %s on %s (%s)%s: %s", rule.Protocol, categoryStr, rule.Version, chainName, rule.ChainID, displayName, rule.Field)
			}
		}
	}

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
			if err := sender.SendAlert(decision.Rule.RecipientEmail, decision); err != nil {
				log.Printf("❌ Failed to send alert to %s: %v", decision.Rule.RecipientEmail, err)
			}
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

	// Group clients by protocol, category, and chain ID
	// Key format: "protocol:category:chainID" or "protocol:chainID" for protocols without categories
	type clientKey struct {
		protocol string
		category string
		chainID  string
	}
	clients := make(map[clientKey]interface{})
	defer func() {
		// Close all clients
		for _, client := range clients {
			switch c := client.(type) {
			case *aave.AaveV3Client:
				if c != nil {
					c.Close()
				}
			case *morpho.MorphoV1MarketClient:
				if c != nil {
					c.Close()
				}
			case *morpho.MorphoV1VaultClient:
				if c != nil {
					c.Close()
				}
			}
		}
	}()

	log.Printf("🔍 Checking DeFi protocols for %d rule(s)...", len(defiRules))

	for _, rule := range defiRules {
		if !rule.Enabled {
			continue
		}

		var chainName string
		var value float64
		var err error

		// Handle Aave v3
		if rule.Protocol == "aave" && rule.Version == "v3" {
			key := clientKey{protocol: "aave", chainID: rule.ChainID}
			client, ok := clients[key].(*aave.AaveV3Client)
			if !ok {
				client, err = aave.NewAaveV3Client(rule.ChainID)
				if err != nil {
					log.Printf("⚠️  Failed to create Aave client for chain %s: %v", rule.ChainID, err)
					continue
				}
				clients[key] = client
			}

			chainName, err = aave.GetChainNameFromID(rule.ChainID)
			if err != nil {
				log.Printf("⚠️  Failed to get chain name for chain %s: %v", rule.ChainID, err)
				continue
			}

			tokenAddress := common.HexToAddress(rule.MarketTokenContract)
			fieldType := aave.FieldType(rule.Field)
			value, err = client.GetFieldValue(ctx, tokenAddress, fieldType)
			if err != nil {
				log.Printf("⚠️  Failed to fetch %s for token %s on %s: %v", rule.Field, rule.MarketTokenContract, chainName, err)
				continue
			}

		} else if rule.Protocol == "morpho" && rule.Version == "v1" {
			// Handle Morpho v1
			if rule.Category == "market" {
				key := clientKey{protocol: "morpho", category: "market", chainID: rule.ChainID}
				client, ok := clients[key].(*morpho.MorphoV1MarketClient)
				if !ok {
					// Create market client
					loanToken := rule.BorrowTokenContract
					collateralToken := rule.CollateralTokenContract
					if loanToken == "" || collateralToken == "" {
						log.Printf("⚠️  Missing required fields for Morpho market: borrow_token_contract and collateral_token_contract are required")
						continue
					}
					// Debug: log the values being passed
					client, err = morpho.NewMorphoV1MarketClient(rule.ChainID, rule.MarketTokenContract, loanToken, collateralToken, rule.OracleAddress, rule.IRMAddress, rule.LLTV, rule.MarketContractAddress)
					if err != nil {
						log.Printf("⚠️  Failed to create Morpho market client for chain %s: %v", rule.ChainID, err)
						continue
					}
					clients[key] = client
				}

				chainName, err = morpho.GetChainNameFromID(rule.ChainID)
				if err != nil {
					log.Printf("⚠️  Failed to get chain name for chain %s: %v", rule.ChainID, err)
					continue
				}

				fieldType := morpho.MarketFieldType(rule.Field)
				value, err = client.GetFieldValue(ctx, fieldType)
				if err != nil {
					marketDisplay := rule.MarketTokenContract
					if rule.MarketTokenPair != "" {
						marketDisplay = rule.MarketTokenPair
					}
					log.Printf("⚠️  Failed to fetch %s for Morpho market %s on %s: %v", rule.Field, marketDisplay, chainName, err)
					continue
				}

			} else if rule.Category == "vault" {
				key := clientKey{protocol: "morpho", category: "vault", chainID: rule.ChainID}
				client, ok := clients[key].(*morpho.MorphoV1VaultClient)
				if !ok {
					// Create vault client
					vaultToken := rule.VaultTokenAddress
					if vaultToken == "" {
						vaultToken = rule.MarketTokenContract
					}
					depositToken := rule.DepositTokenContract
					if vaultToken == "" || depositToken == "" {
						log.Printf("⚠️  Missing required fields for Morpho vault: vault_token_address and deposit_token_contract are required")
						continue
					}
					client, err = morpho.NewMorphoV1VaultClient(rule.ChainID, vaultToken, depositToken)
					if err != nil {
						log.Printf("⚠️  Failed to create Morpho vault client for chain %s: %v", rule.ChainID, err)
						continue
					}
					clients[key] = client
				}

				chainName, err = morpho.GetChainNameFromID(rule.ChainID)
				if err != nil {
					log.Printf("⚠️  Failed to get chain name for chain %s: %v", rule.ChainID, err)
					continue
				}

				fieldType := morpho.VaultFieldType(rule.Field)
				value, err = client.GetFieldValue(ctx, fieldType)
				if err != nil {
					vaultDisplay := rule.VaultTokenAddress
					if rule.VaultName != "" {
						vaultDisplay = rule.VaultName
					}
					log.Printf("⚠️  Failed to fetch %s for Morpho vault %s on %s: %v", rule.Field, vaultDisplay, chainName, err)
					continue
				}

			} else {
				log.Printf("⚠️  Invalid category '%s' for Morpho protocol (must be 'market' or 'vault')", rule.Category)
				continue
			}

		} else {
			log.Printf("⚠️  Unsupported protocol: %s %s (supported: aave v3, morpho v1)", rule.Protocol, rule.Version)
			continue
		}

		categoryStr := ""
		if rule.Category != "" {
			categoryStr = " " + rule.Category
		}
		// Use display names if available
		displayName := ""
		if rule.Protocol == "aave" && rule.MarketTokenName != "" {
			displayName = " (" + rule.MarketTokenName + ")"
		} else if rule.Protocol == "morpho" && rule.Category == "market" && rule.MarketTokenPair != "" {
			displayName = " (" + rule.MarketTokenPair + ")"
		} else if rule.Protocol == "morpho" && rule.Category == "vault" && rule.VaultName != "" {
			displayName = " (" + rule.VaultName + ")"
		}
		log.Printf("💰 %s%s %s on %s - %s%s: %g", rule.Protocol, categoryStr, rule.Version, chainName, rule.Field, displayName, value)

		// Evaluate alert rules
		// Use MarketTokenContract as the identifier (or VaultTokenAddress for vaults)
		identifier := rule.MarketTokenContract
		if rule.Protocol == "morpho" && rule.Category == "vault" && rule.VaultTokenAddress != "" {
			identifier = rule.VaultTokenAddress
		}
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

	// Add all price rules to the decision engine
	for _, rule := range priceRules {
		engine.AddRule(rule)
	}

	// Add all DeFi rules to the decision engine
	for _, rule := range defiRules {
		engine.AddDeFiRule(rule)
	}

	totalRules := len(priceRules) + len(defiRules)
	log.Printf("✅ Loaded %d price rule(s) and %d DeFi rule(s) from %s", len(priceRules), len(defiRules), filePath)
	if totalRules == 0 {
		return fmt.Errorf("no alert rules found in %s", filePath)
	}

	return nil
}
