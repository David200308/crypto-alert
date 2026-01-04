package defi

import (
	"context"
	"fmt"
	"log"

	"crypto-alert/internal/core"

	"github.com/ethereum/go-ethereum/common"

	"crypto-alert/internal/defi/aave"
	"crypto-alert/internal/defi/kamino"
	"crypto-alert/internal/defi/morpho"
)

// ClientManager manages DeFi protocol clients
type ClientManager struct {
	clients map[clientKey]interface{}
}

// clientKey uniquely identifies a DeFi client
type clientKey struct {
	protocol string
	category string
	chainID  string
}

// NewClientManager creates a new client manager
func NewClientManager() *ClientManager {
	return &ClientManager{
		clients: make(map[clientKey]interface{}),
	}
}

// Close closes all managed clients
func (cm *ClientManager) Close() {
	for _, client := range cm.clients {
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
		case *morpho.MorphoV2VaultClient:
			if c != nil {
				c.Close()
			}
		case *kamino.KaminoVaultClient:
			if c != nil {
				c.Close()
			}
		}
	}
}

// GetFieldValue fetches the field value for a DeFi rule
func (cm *ClientManager) GetFieldValue(ctx context.Context, rule *core.DeFiAlertRule) (float64, string, error) {
	var chainName string
	var value float64
	var err error

	// Handle Aave v3
	if rule.Protocol == "aave" && rule.Version == "v3" {
		key := clientKey{protocol: "aave", chainID: rule.ChainID}
		client, ok := cm.clients[key].(*aave.AaveV3Client)
		if !ok {
			client, err = aave.NewAaveV3Client(rule.ChainID)
			if err != nil {
				return 0, "", fmt.Errorf("failed to create Aave client for chain %s: %w", rule.ChainID, err)
			}
			cm.clients[key] = client
		}

		chainName, err = aave.GetChainNameFromID(rule.ChainID)
		if err != nil {
			return 0, "", fmt.Errorf("failed to get chain name for chain %s: %w", rule.ChainID, err)
		}

		tokenAddress := common.HexToAddress(rule.MarketTokenContract)
		fieldType := aave.FieldType(rule.Field)
		value, err = client.GetFieldValue(ctx, tokenAddress, fieldType)
		if err != nil {
			return 0, chainName, fmt.Errorf("failed to fetch %s for token %s on %s: %w", rule.Field, rule.MarketTokenContract, chainName, err)
		}

	} else if rule.Protocol == "morpho" && rule.Version == "v1" {
		// Handle Morpho v1
		if rule.Category == "market" {
			key := clientKey{protocol: "morpho", category: "market", chainID: rule.ChainID}
			client, ok := cm.clients[key].(*morpho.MorphoV1MarketClient)
			if !ok {
				loanToken := rule.BorrowTokenContract
				collateralToken := rule.CollateralTokenContract
				if loanToken == "" || collateralToken == "" {
					return 0, "", fmt.Errorf("missing required fields for Morpho market: borrow_token_contract and collateral_token_contract are required")
				}
				client, err = morpho.NewMorphoV1MarketClient(rule.ChainID, rule.MarketTokenContract, loanToken, collateralToken, rule.OracleAddress, rule.IRMAddress, rule.LLTV, rule.MarketContractAddress)
				if err != nil {
					return 0, "", fmt.Errorf("failed to create Morpho market client for chain %s: %w", rule.ChainID, err)
				}
				cm.clients[key] = client
			}

			chainName, err = morpho.GetChainNameFromID(rule.ChainID)
			if err != nil {
				return 0, "", fmt.Errorf("failed to get chain name for chain %s: %w", rule.ChainID, err)
			}

			fieldType := morpho.MarketFieldType(rule.Field)
			value, err = client.GetFieldValue(ctx, fieldType)
			if err != nil {
				marketDisplay := rule.MarketTokenContract
				if rule.MarketTokenPair != "" {
					marketDisplay = rule.MarketTokenPair
				}
				return 0, chainName, fmt.Errorf("failed to fetch %s for Morpho market %s on %s: %w", rule.Field, marketDisplay, chainName, err)
			}

		} else if rule.Category == "vault" {
			key := clientKey{protocol: "morpho", category: "vault", chainID: rule.ChainID}
			client, ok := cm.clients[key].(*morpho.MorphoV1VaultClient)
			if !ok {
				vaultToken := rule.VaultTokenAddress
				if vaultToken == "" {
					vaultToken = rule.MarketTokenContract
				}
				depositToken := rule.DepositTokenContract
				if vaultToken == "" || depositToken == "" {
					return 0, "", fmt.Errorf("missing required fields for Morpho vault: vault_token_address and deposit_token_contract are required")
				}
				client, err = morpho.NewMorphoV1VaultClient(rule.ChainID, vaultToken, depositToken)
				if err != nil {
					return 0, "", fmt.Errorf("failed to create Morpho vault client for chain %s: %w", rule.ChainID, err)
				}
				cm.clients[key] = client
			}

			chainName, err = morpho.GetChainNameFromID(rule.ChainID)
			if err != nil {
				return 0, "", fmt.Errorf("failed to get chain name for chain %s: %w", rule.ChainID, err)
			}

			fieldType := morpho.VaultFieldType(rule.Field)
			value, err = client.GetFieldValue(ctx, fieldType)
			if err != nil {
				vaultDisplay := rule.VaultTokenAddress
				if rule.VaultName != "" {
					vaultDisplay = rule.VaultName
				}
				return 0, chainName, fmt.Errorf("failed to fetch %s for Morpho vault %s on %s: %w", rule.Field, vaultDisplay, chainName, err)
			}

		} else {
			return 0, "", fmt.Errorf("invalid category '%s' for Morpho protocol (must be 'market' or 'vault')", rule.Category)
		}

	} else if rule.Protocol == "morpho" && rule.Version == "v2" {
		// Handle Morpho v2
		if rule.Category == "vault" {
			key := clientKey{protocol: "morpho", category: "vault", chainID: rule.ChainID}
			client, ok := cm.clients[key].(*morpho.MorphoV2VaultClient)
			if !ok {
				vaultToken := rule.VaultTokenAddress
				if vaultToken == "" {
					vaultToken = rule.MarketTokenContract
				}
				depositToken := rule.DepositTokenContract
				if vaultToken == "" || depositToken == "" {
					return 0, "", fmt.Errorf("missing required fields for Morpho v2 vault: vault_token_address and deposit_token_contract are required")
				}
				client, err = morpho.NewMorphoV2VaultClient(rule.ChainID, vaultToken, depositToken)
				if err != nil {
					return 0, "", fmt.Errorf("failed to create Morpho v2 vault client for chain %s: %w", rule.ChainID, err)
				}
				cm.clients[key] = client
			}

			chainName, err = morpho.GetChainNameFromID(rule.ChainID)
			if err != nil {
				return 0, "", fmt.Errorf("failed to get chain name for chain %s: %w", rule.ChainID, err)
			}

			fieldType := morpho.VaultFieldType(rule.Field)
			value, err = client.GetFieldValue(ctx, fieldType)
			if err != nil {
				vaultDisplay := rule.VaultTokenAddress
				if rule.VaultName != "" {
					vaultDisplay = rule.VaultName
				}
				return 0, chainName, fmt.Errorf("failed to fetch %s for Morpho v2 vault %s on %s: %w", rule.Field, vaultDisplay, chainName, err)
			}

		} else {
			return 0, "", fmt.Errorf("invalid category '%s' for Morpho v2 protocol (must be 'vault')", rule.Category)
		}

	} else if rule.Protocol == "kamino" {
		// Handle Kamino vault
		if rule.Category == "vault" {
			key := clientKey{protocol: "kamino", category: "vault", chainID: rule.ChainID}
			client, ok := cm.clients[key].(*kamino.KaminoVaultClient)
			if !ok {
				vaultPubkey := rule.VaultTokenAddress
				if vaultPubkey == "" {
					vaultPubkey = rule.MarketTokenContract
				}
				depositTokenMint := rule.DepositTokenContract
				if vaultPubkey == "" || depositTokenMint == "" {
					return 0, "", fmt.Errorf("missing required fields for Kamino vault: vault_token_address and deposit_token_contract are required")
				}
				client, err = kamino.NewKaminoVaultClient(rule.ChainID, vaultPubkey, depositTokenMint)
				if err != nil {
					return 0, "", fmt.Errorf("failed to create Kamino vault client for chain %s: %w", rule.ChainID, err)
				}
				cm.clients[key] = client
			}

			chainName, err = kamino.GetChainNameFromID(rule.ChainID)
			if err != nil {
				return 0, "", fmt.Errorf("failed to get chain name for chain %s: %w", rule.ChainID, err)
			}

			fieldType := kamino.VaultFieldType(rule.Field)
			value, err = client.GetFieldValue(ctx, fieldType)
			if err != nil {
				vaultDisplay := rule.VaultTokenAddress
				if rule.VaultName != "" {
					vaultDisplay = rule.VaultName
				}
				return 0, chainName, fmt.Errorf("failed to fetch %s for Kamino vault %s on %s: %w", rule.Field, vaultDisplay, chainName, err)
			}

		} else {
			return 0, "", fmt.Errorf("invalid category '%s' for Kamino protocol (must be 'vault')", rule.Category)
		}

	} else {
		return 0, "", fmt.Errorf("unsupported protocol: %s %s (supported: aave v3, morpho v1, morpho v2, kamino)", rule.Protocol, rule.Version)
	}

	return value, chainName, nil
}

// GetChainName returns the chain name for a given protocol and chain ID
func GetChainName(protocol, chainID string) (string, error) {
	switch protocol {
	case "aave":
		return aave.GetChainNameFromID(chainID)
	case "morpho":
		return morpho.GetChainNameFromID(chainID)
	case "kamino":
		return kamino.GetChainNameFromID(chainID)
	default:
		return "", fmt.Errorf("unsupported protocol: %s", protocol)
	}
}

// GetDisplayName returns a display name for a DeFi rule
func GetDisplayName(rule *core.DeFiAlertRule) string {
	if rule.Protocol == "aave" && rule.MarketTokenName != "" {
		return " (" + rule.MarketTokenName + ")"
	} else if rule.Protocol == "morpho" && rule.Category == "market" && rule.MarketTokenPair != "" {
		return " (" + rule.MarketTokenPair + ")"
	} else if rule.Protocol == "morpho" && rule.Category == "vault" && rule.VaultName != "" {
		return " (" + rule.VaultName + ")"
	} else if rule.Protocol == "kamino" && rule.Category == "vault" && rule.VaultName != "" {
		return " (" + rule.VaultName + ")"
	}
	return ""
}

// GetCategoryString returns a formatted category string
func GetCategoryString(rule *core.DeFiAlertRule) string {
	if rule.Category != "" {
		return " " + rule.Category
	}
	return ""
}

// GetIdentifier returns the identifier for a DeFi rule (used for evaluation)
func GetIdentifier(rule *core.DeFiAlertRule) string {
	if (rule.Protocol == "morpho" || rule.Protocol == "kamino") && rule.Category == "vault" && rule.VaultTokenAddress != "" {
		return rule.VaultTokenAddress
	}
	return rule.MarketTokenContract
}

// LogDeFiRules logs information about DeFi rules
func LogDeFiRules(rules []*core.DeFiAlertRule) {
	if len(rules) == 0 {
		return
	}

	log.Printf("📊 Monitoring DeFi protocols: %d rule(s)", len(rules))
	for _, rule := range rules {
		if rule.Enabled {
			chainName, err := GetChainName(rule.Protocol, rule.ChainID)
			if err != nil {
				chainName = rule.ChainID
			}
			categoryStr := GetCategoryString(rule)
			displayName := GetDisplayName(rule)
			log.Printf("  - %s%s %s on %s (%s)%s: %s", rule.Protocol, categoryStr, rule.Version, chainName, rule.ChainID, displayName, rule.Field)
		}
	}
}
