package morpho

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// FieldType represents the type of field to monitor for vaults
type VaultFieldType string

const (
	VaultFieldTVL         VaultFieldType = "TVL"
	VaultFieldLiquidity    VaultFieldType = "LIQUIDITY"
	VaultFieldUtilization  VaultFieldType = "UTILIZATION"
	VaultFieldAPY          VaultFieldType = "APY"
)

// VaultData holds vault data from Morpho v1
type VaultData struct {
	TotalAssets      *big.Int // TVL (total assets in vault)
	AvailableAssets  *big.Int // Available liquidity (not allocated to markets)
	AllocatedAssets  *big.Int // Assets allocated to markets
	Utilization      float64  // Calculated: (allocated / total) * 100
	APY              float64  // APY (calculated from vault performance or market rates)
}

// MorphoV1VaultClient handles interactions with Morpho v1 Vaults
type MorphoV1VaultClient struct {
	chainID         string
	chainInfo       ChainInfo
	client          *ethclient.Client
	vaultTokenAddr  common.Address // ERC-4626 vault token address
	depositTokenAddr common.Address // Underlying deposit token address
}

// NewMorphoV1VaultClient creates a new Morpho v1 vault client
func NewMorphoV1VaultClient(chainID, vaultTokenAddr, depositTokenAddr string) (*MorphoV1VaultClient, error) {
	chainInfo, ok := supportedChains[chainID]
	if !ok {
		return nil, fmt.Errorf("unsupported chain ID: %s. Supported chains: 1 (Ethereum), 8453 (Base), 42161 (Arbitrum One)", chainID)
	}

	// Load RPC URL from environment
	rpcURL := getRPCURLForChain(chainID)
	if rpcURL == "" {
		return nil, fmt.Errorf("RPC URL not configured for chain %s (%s). Please set the appropriate environment variable", chainID, chainInfo.ChainName)
	}

	chainInfo.RPCURL = rpcURL

	// Connect to RPC
	client, err := ethclient.Dial(chainInfo.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s RPC: %w", chainInfo.ChainName, err)
	}

	// Parse token addresses
	vaultToken := common.HexToAddress(vaultTokenAddr)
	depositToken := common.HexToAddress(depositTokenAddr)

	return &MorphoV1VaultClient{
		chainID:         chainID,
		chainInfo:       chainInfo,
		client:          client,
		vaultTokenAddr:  vaultToken,
		depositTokenAddr: depositToken,
	}, nil
}

// GetChainName returns the human-readable chain name
func (c *MorphoV1VaultClient) GetChainName() string {
	return c.chainInfo.ChainName
}

// GetChainID returns the chain ID
func (c *MorphoV1VaultClient) GetChainID() string {
	return c.chainID
}

// Close closes the RPC connection
func (c *MorphoV1VaultClient) Close() {
	if c.client != nil {
		c.client.Close()
	}
}

// GetVaultData fetches vault data for the Morpho v1 vault
// Note: This is a simplified implementation. In production, you would need to:
// 1. Call ERC-4626 totalAssets() function on the vault
// 2. Query the vault's allocation to markets
// 3. Calculate available liquidity and utilization
func (c *MorphoV1VaultClient) GetVaultData(ctx context.Context) (*VaultData, error) {
	// Get totalAssets from vault token using ERC-4626 totalAssets() function
	// This returns the total amount of underlying assets managed by the vault
	totalAssets, err := c.getVaultTotalAssets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault totalAssets: %w", err)
	}

	// For now, we'll use simplified calculations
	// In production, you'd query the Morpho Vault contract directly for allocations
	allocatedAssets := big.NewInt(0) // Placeholder - would need to query Morpho vault contract
	availableAssets := new(big.Int).Sub(totalAssets, allocatedAssets)
	if availableAssets.Sign() < 0 {
		availableAssets = big.NewInt(0)
	}

	// Calculate utilization
	var utilization float64
	if totalAssets.Sign() > 0 {
		utilization = bigRatDiv(allocatedAssets, totalAssets) * 100.0
	}

	// Calculate APY
	// Note: In production, this would be calculated from the vault's actual performance
	// or by aggregating APY from the markets the vault is allocated to
	// For now, using a placeholder calculation based on utilization
	var apy float64
	// Simplified APY calculation: higher utilization typically means higher returns
	// This is a placeholder - in production, query the vault contract or aggregate market APYs
	if totalAssets.Sign() > 0 && allocatedAssets.Sign() > 0 {
		// Placeholder: assume base APY increases with utilization
		// Real implementation would query vault contract or market rates
		apy = utilization * 0.1 // Placeholder calculation
	}

	return &VaultData{
		TotalAssets:     totalAssets,
		AvailableAssets: availableAssets,
		AllocatedAssets: allocatedAssets,
		Utilization:     utilization,
		APY:             apy,
	}, nil
}

// getTokenTotalSupply calls totalSupply() on an ERC20 token contract
func (c *MorphoV1VaultClient) getTokenTotalSupply(ctx context.Context, tokenAddr common.Address, erc20ABI abi.ABI) (*big.Int, error) {
	method, exists := erc20ABI.Methods["totalSupply"]
	if !exists {
		return nil, fmt.Errorf("totalSupply method not found in ERC20 ABI")
	}

	methodID := method.ID
	msg := ethereum.CallMsg{
		To:   &tokenAddr,
		Data: methodID,
	}

	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call totalSupply on token %s: %w", tokenAddr.Hex(), err)
	}

	unpacked, err := method.Outputs.UnpackValues(result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack totalSupply result: %w", err)
	}

	if len(unpacked) < 1 {
		return nil, fmt.Errorf("unexpected number of return values: got %d, expected 1", len(unpacked))
	}

	totalSupply, ok := unpacked[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("failed to extract totalSupply")
	}

	return totalSupply, nil
}

// getVaultTotalAssets calls totalAssets() on an ERC-4626 vault contract
// This returns the total amount of underlying assets managed by the vault
func (c *MorphoV1VaultClient) getVaultTotalAssets(ctx context.Context) (*big.Int, error) {
	// ERC-4626 totalAssets() function signature: function totalAssets() external view returns (uint256)
	// Create a minimal ABI for just this function
	totalAssetsABI := `[{"constant":true,"inputs":[],"name":"totalAssets","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`
	
	parsedABI, err := abi.JSON(strings.NewReader(totalAssetsABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse totalAssets ABI: %w", err)
	}

	method, exists := parsedABI.Methods["totalAssets"]
	if !exists {
		return nil, fmt.Errorf("totalAssets method not found in ABI")
	}

	methodID := method.ID
	msg := ethereum.CallMsg{
		To:   &c.vaultTokenAddr,
		Data: methodID,
	}

	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call totalAssets on vault %s: %w", c.vaultTokenAddr.Hex(), err)
	}

	// Unpack the result
	unpacked, err := method.Outputs.UnpackValues(result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack totalAssets result: %w", err)
	}

	if len(unpacked) < 1 {
		return nil, fmt.Errorf("unexpected number of return values: got %d, expected 1", len(unpacked))
	}

	totalAssets, ok := unpacked[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("failed to extract totalAssets, got type %T", unpacked[0])
	}

	return totalAssets, nil
}

// GetFieldValue retrieves the value for a specific field (TVL, LIQUIDITY, UTILIZATION, or APY)
func (c *MorphoV1VaultClient) GetFieldValue(ctx context.Context, field VaultFieldType) (float64, error) {
	vaultData, err := c.GetVaultData(ctx)
	if err != nil {
		return 0, err
	}

	switch field {
	case VaultFieldTVL:
		// TVL is total assets, convert to float64
		// Note: For USDC (6 decimals), this would be in units of 1e6
		value, _ := new(big.Float).SetInt(vaultData.TotalAssets).Float64()
		return value / 1000000.0, nil // Assuming 6 decimals for USDC
	case VaultFieldLiquidity:
		// Liquidity is available assets
		value, _ := new(big.Float).SetInt(vaultData.AvailableAssets).Float64()
		return value / 1000000.0, nil // Assuming 6 decimals
	case VaultFieldUtilization:
		return vaultData.Utilization, nil
	case VaultFieldAPY:
		return vaultData.APY, nil
	default:
		return 0, fmt.Errorf("unsupported field type: %s", field)
	}
}

