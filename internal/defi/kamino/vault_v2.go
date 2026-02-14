package kamino

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	"crypto-alert/internal/utils"
)

// VaultFieldType represents the type of field to monitor for vaults
type VaultFieldType string

const (
	VaultFieldTVL         VaultFieldType = "TVL"
	VaultFieldLiquidity   VaultFieldType = "LIQUIDITY"
	VaultFieldUtilization VaultFieldType = "UTILIZATION"
	VaultFieldAPY         VaultFieldType = "APY"
)

// VaultData holds vault data from Kamino
type VaultData struct {
	TotalAssets     *big.Int // TVL (total assets in vault)
	AvailableAssets *big.Int // Available liquidity (not allocated)
	AllocatedAssets *big.Int // Assets allocated to strategies
	Utilization     float64  // Calculated: (allocated / total) * 100
	APY             float64  // APY from vault
}

// ChainInfo holds chain information for Solana
type ChainInfo struct {
	ChainID   string
	ChainName string
	APIURL    string
	RPCURL    string // Optional Solana RPC URL (for future direct on-chain queries)
}

// Supported chains mapping for Solana
var supportedChains = map[string]ChainInfo{
	"solana": {
		ChainID:   "solana",
		ChainName: "Solana Mainnet",
		APIURL:    "https://api.kamino.finance",
		RPCURL:    "", // Will be loaded from environment when creating client
	},
	"101": {
		ChainID:   "101",
		ChainName: "Solana Mainnet",
		APIURL:    "https://api.kamino.finance",
		RPCURL:    "", // Will be loaded from environment when creating client
	},
}

// getRPCURLForChain returns a randomly selected RPC URL for Solana chain ID.
// Supports comma-separated RPC URLs in env vars for load balancing.
func getRPCURLForChain(chainID string) string {
	if chainID == "solana" || chainID == "101" {
		return utils.GetSolanaRPCURL()
	}
	return ""
}

// KaminoVaultClient handles interactions with Kamino Vaults via REST API
type KaminoVaultClient struct {
	chainID          string
	chainInfo        ChainInfo
	httpClient       *http.Client
	vaultPubkey      string // Solana public key of the vault
	depositTokenMint string // Underlying deposit token mint address
}

// NewKaminoVaultClient creates a new Kamino vault client
func NewKaminoVaultClient(chainID, vaultPubkey, depositTokenMint string) (*KaminoVaultClient, error) {
	chainInfo, ok := supportedChains[chainID]
	if !ok {
		// Try to normalize chainID
		if chainID == "solana" || chainID == "101" {
			chainInfo = supportedChains["solana"]
		} else {
			return nil, fmt.Errorf("unsupported chain ID: %s. Supported chains: solana, 101 (Solana Mainnet)", chainID)
		}
	}

	// Load RPC URL from environment (optional - REST API works without it)
	rpcURL := getRPCURLForChain(chainID)
	if rpcURL != "" {
		chainInfo.RPCURL = rpcURL
	}
	// Note: RPC URL is optional since we use Kamino REST API
	// It's stored for potential future direct Solana RPC queries

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &KaminoVaultClient{
		chainID:          chainID,
		chainInfo:        chainInfo,
		httpClient:       httpClient,
		vaultPubkey:      vaultPubkey,
		depositTokenMint: depositTokenMint,
	}, nil
}

// GetChainName returns the human-readable chain name
func (c *KaminoVaultClient) GetChainName() string {
	return c.chainInfo.ChainName
}

// GetChainID returns the chain ID
func (c *KaminoVaultClient) GetChainID() string {
	return c.chainID
}

// Close closes the HTTP client (no-op for HTTP client, but kept for interface consistency)
func (c *KaminoVaultClient) Close() {
	// HTTP client doesn't need explicit closing
}

// KaminoVaultAPIResponse represents the response from Kamino API
type KaminoVaultAPIResponse struct {
	Address   string `json:"address"`
	ProgramID string `json:"programId"`
	State     struct {
		TokenMint               string `json:"tokenMint"`
		TokenMintDecimals       int    `json:"tokenMintDecimals"`
		TokenAvailable          string `json:"tokenAvailable"` // Available tokens (as string to preserve precision)
		PrevAum                 string `json:"prevAum"`        // Previous Assets Under Management
		VaultAllocationStrategy []struct {
			Reserve          string `json:"reserve"`
			CtokenAllocation string `json:"ctokenAllocation"` // Allocated tokens per reserve
		} `json:"vaultAllocationStrategy"`
		Name string `json:"name"`
	} `json:"state"`
}

// GetVaultData fetches vault data from Kamino API
func (c *KaminoVaultClient) GetVaultData(ctx context.Context) (*VaultData, error) {
	// Construct API URL
	apiURL := fmt.Sprintf("%s/kvaults/vaults/%s", c.chainInfo.APIURL, c.vaultPubkey)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "crypto-alert/1.0")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch vault data from Kamino API: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Kamino API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var apiResp KaminoVaultAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse Kamino API response: %w", err)
	}

	// Get decimals from API response
	decimals := apiResp.State.TokenMintDecimals
	if decimals == 0 {
		decimals = 6 // Default to 6 decimals for USDC-like tokens
	}

	// Parse available tokens (already in smallest unit)
	availableAssets, ok := new(big.Int).SetString(apiResp.State.TokenAvailable, 10)
	if !ok {
		return nil, fmt.Errorf("failed to parse tokenAvailable: %s", apiResp.State.TokenAvailable)
	}

	// Calculate total allocated tokens from all reserves
	allocatedAssets := big.NewInt(0)
	for _, allocation := range apiResp.State.VaultAllocationStrategy {
		allocationAmount, ok := new(big.Int).SetString(allocation.CtokenAllocation, 10)
		if !ok {
			continue // Skip invalid allocations
		}
		allocatedAssets.Add(allocatedAssets, allocationAmount)
	}

	// Calculate total assets
	// Use prevAum if available (it's the most accurate TVL metric)
	// Otherwise calculate from available + allocated
	var totalAssets *big.Int
	if apiResp.State.PrevAum != "" {
		prevAumFloat, ok := new(big.Float).SetString(apiResp.State.PrevAum)
		if ok {
			// prevAum appears to be in smallest units already (based on API response)
			// Convert to big.Int (rounding down)
			prevAumInt, _ := prevAumFloat.Int(nil)
			totalAssets = prevAumInt
		} else {
			// Fallback: calculate from available + allocated
			totalAssets = new(big.Int).Add(availableAssets, allocatedAssets)
		}
	} else {
		// Calculate total from available + allocated
		totalAssets = new(big.Int).Add(availableAssets, allocatedAssets)
	}

	// Calculate utilization
	var utilization float64
	if totalAssets.Sign() > 0 {
		utilization = bigRatDiv(allocatedAssets, totalAssets) * 100.0
	}

	// APY is not directly available in this endpoint
	// Would need to query a different endpoint or calculate from historical data
	// For now, set to 0 (can be enhanced later)
	apy := 0.0

	return &VaultData{
		TotalAssets:     totalAssets,
		AvailableAssets: availableAssets,
		AllocatedAssets: allocatedAssets,
		Utilization:     utilization,
		APY:             apy,
	}, nil
}

// bigRatDiv divides two big.Ints and returns a float64
func bigRatDiv(numerator, denominator *big.Int) float64 {
	if denominator.Sign() == 0 {
		return 0
	}

	num := new(big.Float).SetInt(numerator)
	den := new(big.Float).SetInt(denominator)
	result := new(big.Float).Quo(num, den)

	value, _ := result.Float64()
	return value
}

// GetFieldValue retrieves the value for a specific field (TVL, LIQUIDITY, UTILIZATION, or APY)
func (c *KaminoVaultClient) GetFieldValue(ctx context.Context, field VaultFieldType) (float64, error) {
	vaultData, err := c.GetVaultData(ctx)
	if err != nil {
		return 0, err
	}

	switch field {
	case VaultFieldTVL:
		// TVL is total assets, convert to float64 and divide by decimals
		value, _ := new(big.Float).SetInt(vaultData.TotalAssets).Float64()
		return value / float64(1e6), nil // Assuming 6 decimals
	case VaultFieldLiquidity:
		// Liquidity is available assets
		value, _ := new(big.Float).SetInt(vaultData.AvailableAssets).Float64()
		return value / float64(1e6), nil // Assuming 6 decimals
	case VaultFieldUtilization:
		return vaultData.Utilization, nil
	case VaultFieldAPY:
		return vaultData.APY, nil
	default:
		return 0, fmt.Errorf("unsupported field type: %s", field)
	}
}

// GetChainNameFromID returns the chain name for a given chain ID
func GetChainNameFromID(chainID string) (string, error) {
	chainInfo, ok := supportedChains[chainID]
	if !ok {
		// Try to normalize
		if chainID == "solana" || chainID == "101" {
			return "Solana Mainnet", nil
		}
		return "", fmt.Errorf("unsupported chain ID: %s", chainID)
	}
	return chainInfo.ChainName, nil
}
