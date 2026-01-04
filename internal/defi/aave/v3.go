package aave

import (
	"context"
	_ "embed"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
)

var envLoaded bool

// ensureEnvLoaded ensures the .env file is loaded (idempotent)
func ensureEnvLoaded() {
	if !envLoaded {
		_ = godotenv.Load() // Ignore error if .env doesn't exist
		envLoaded = true
	}
}

//go:embed abi/aave_pool_data_provider.json
var poolDataProviderABIJSON string

// ChainInfo holds chain information
type ChainInfo struct {
	ChainID   int64
	ChainName string
	RPCURL    string
}

// Supported chains mapping (RPC URLs are loaded lazily when creating clients)
var supportedChains = map[string]ChainInfo{
	"1": {
		ChainID:   1,
		ChainName: "Ethereum Mainnet",
		RPCURL:    "", // Will be loaded from environment when creating client
	},
	"8453": {
		ChainID:   8453,
		ChainName: "Base",
		RPCURL:    "", // Will be loaded from environment when creating client
	},
	"42161": {
		ChainID:   42161,
		ChainName: "Arbitrum One",
		RPCURL:    "", // Will be loaded from environment when creating client
	},
}

// getRPCURLForChain returns the RPC URL for a given chain ID
func getRPCURLForChain(chainID string) string {
	ensureEnvLoaded() // Ensure .env is loaded before reading env vars
	switch chainID {
	case "1":
		return getEnv("ETH_RPC_URL", "")
	case "8453":
		return getEnv("BASE_RPC_URL", "")
	case "42161":
		return getEnv("ARB_RPC_URL", "")
	default:
		return ""
	}
}

// PoolDataProvider addresses for each chain
// Source: https://docs.aave.com/developers/deployed-contracts/v3-mainnet
var poolDataProviderAddresses = map[string]common.Address{
	"1":     common.HexToAddress("0x7B4EB56E7CD4b454BA8ff71E4518426369a138a3"), // Ethereum Mainnet
	"8453":  common.HexToAddress("0x2d8A3C567718a3cDbC0c0A2f5C86ffA1308c4dA6"), // Base
	"42161": common.HexToAddress("0x69FA688f1Dc47d4B5d8029D5a35FB7a548310654"), // Arbitrum One
}

// FieldType represents the type of field to monitor
type FieldType string

const (
	FieldTVL         FieldType = "TVL"
	FieldAPY         FieldType = "APY"
	FieldUtilization FieldType = "UTILIZATION"
)

// ReserveData holds reserve data from Aave
type ReserveData struct {
	TotalAToken       *big.Int // TVL (total supply)
	TotalStableDebt   *big.Int
	TotalVariableDebt *big.Int
	LiquidityRate     *big.Int // Used for APY calculation
	Utilization       float64  // Calculated: (totalDebt / totalSupply) * 100
	APY               float64  // Calculated from liquidityRate
}

// AaveV3Client handles interactions with Aave v3 protocol
type AaveV3Client struct {
	chainID   string
	chainInfo ChainInfo
	client    *ethclient.Client
	contract  *bind.BoundContract
	abi       abi.ABI
}

// NewAaveV3Client creates a new Aave v3 client for the specified chain
func NewAaveV3Client(chainID string) (*AaveV3Client, error) {
	chainInfo, ok := supportedChains[chainID]
	if !ok {
		return nil, fmt.Errorf("unsupported chain ID: %s. Supported chains: 1 (Ethereum), 8453 (Base), 42161 (Arbitrum One)", chainID)
	}

	// Load RPC URL from environment (lazy loading)
	rpcURL := getRPCURLForChain(chainID)
	if rpcURL == "" {
		return nil, fmt.Errorf("RPC URL not configured for chain %s (%s). Please set the appropriate environment variable (ETH_RPC_URL, BASE_RPC_URL, or ARB_RPC_URL)", chainID, chainInfo.ChainName)
	}

	// Update chainInfo with the loaded RPC URL
	chainInfo.RPCURL = rpcURL

	// Connect to RPC
	client, err := ethclient.Dial(chainInfo.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s RPC: %w", chainInfo.ChainName, err)
	}

	// Parse ABI (embedded in binary)
	parsedABI, err := abi.JSON(strings.NewReader(poolDataProviderABIJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	// Get pool data provider address for this chain
	poolDataProviderAddr, ok := poolDataProviderAddresses[chainID]
	if !ok {
		return nil, fmt.Errorf("pool data provider address not found for chain %s", chainID)
	}

	contract := bind.NewBoundContract(poolDataProviderAddr, parsedABI, client, client, client)

	return &AaveV3Client{
		chainID:   chainID,
		chainInfo: chainInfo,
		client:    client,
		contract:  contract,
		abi:       parsedABI,
	}, nil
}

// GetChainName returns the human-readable chain name
func (c *AaveV3Client) GetChainName() string {
	return c.chainInfo.ChainName
}

// GetChainID returns the chain ID
func (c *AaveV3Client) GetChainID() string {
	return c.chainID
}

// Close closes the RPC connection
func (c *AaveV3Client) Close() {
	if c.client != nil {
		c.client.Close()
	}
}

// GetReserveData fetches reserve data for a specific token address
func (c *AaveV3Client) GetReserveData(ctx context.Context, tokenAddress common.Address) (*ReserveData, error) {
	// Get the method from ABI
	method, exists := c.abi.Methods["getReserveData"]
	if !exists {
		return nil, fmt.Errorf("getReserveData method not found in ABI")
	}

	// Pack the input parameters
	packedParams, err := method.Inputs.Pack(tokenAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to pack input: %w", err)
	}

	// Prepend the method selector (first 4 bytes of keccak256 hash of function signature)
	methodID := method.ID
	input := append(methodID, packedParams...)

	// Get the contract address
	contractAddr := poolDataProviderAddresses[c.chainID]

	// Call the contract using ethclient.CallContract
	msg := ethereum.CallMsg{
		To:   &contractAddr,
		Data: input,
	}

	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call contract: %w", err)
	}

	// Unpack the output - UnpackValues returns the unpacked values
	unpacked, err := method.Outputs.UnpackValues(result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack output: %w", err)
	}

	// Extract values from unpacked results
	// The ABI returns: unbacked, accruedToTreasuryScaled, totalAToken, totalStableDebt,
	// totalVariableDebt, liquidityRate, variableBorrowRate, stableBorrowRate,
	// averageStableBorrowRate, liquidityIndex, variableBorrowIndex, lastUpdateTimestamp
	if len(unpacked) < 12 {
		return nil, fmt.Errorf("unexpected number of return values: got %d, expected 12", len(unpacked))
	}

	var totalAToken, totalStableDebt, totalVariableDebt, liquidityRate *big.Int
	var ok bool

	// Extract totalAToken (index 2)
	if totalAToken, ok = unpacked[2].(*big.Int); !ok {
		return nil, fmt.Errorf("failed to extract totalAToken")
	}

	// Extract totalStableDebt (index 3)
	if totalStableDebt, ok = unpacked[3].(*big.Int); !ok {
		return nil, fmt.Errorf("failed to extract totalStableDebt")
	}

	// Extract totalVariableDebt (index 4)
	if totalVariableDebt, ok = unpacked[4].(*big.Int); !ok {
		return nil, fmt.Errorf("failed to extract totalVariableDebt")
	}

	// Extract liquidityRate (index 5)
	if liquidityRate, ok = unpacked[5].(*big.Int); !ok {
		return nil, fmt.Errorf("failed to extract liquidityRate")
	}

	// Calculate total debt
	totalDebt := new(big.Int).Add(totalStableDebt, totalVariableDebt)

	// Calculate utilization: (totalDebt / totalSupply) * 100
	var utilization float64
	if totalAToken.Sign() > 0 {
		utilization = bigRatDiv(totalDebt, totalAToken) * 100.0
	}

	// Calculate APY from liquidityRate
	// liquidityRate is in RAY units (1e27), so APY = (liquidityRate / 1e27) * 100
	var apy float64
	if liquidityRate.Sign() > 0 {
		// Convert RAY to percentage: (liquidityRate / 1e27) * 100
		ray := new(big.Int).Exp(big.NewInt(10), big.NewInt(27), nil)
		apy = bigRatDiv(liquidityRate, ray) * 100.0
	}

	return &ReserveData{
		TotalAToken:       totalAToken,
		TotalStableDebt:   totalStableDebt,
		TotalVariableDebt: totalVariableDebt,
		LiquidityRate:     liquidityRate,
		Utilization:       utilization,
		APY:               apy,
	}, nil
}

// GetFieldValue retrieves the value for a specific field (TVL, APY, or UTILIZATION)
func (c *AaveV3Client) GetFieldValue(ctx context.Context, tokenAddress common.Address, field FieldType) (float64, error) {
	reserveData, err := c.GetReserveData(ctx, tokenAddress)
	if err != nil {
		return 0, err
	}

	switch field {
	case FieldTVL:
		// TVL is in raw token units, convert to float64
		// Note: For USDC (6 decimals), this would be in units of 1e6
		// The threshold in config should account for token decimals
		value, _ := new(big.Float).SetInt(reserveData.TotalAToken).Float64()
		return value / 1000000.0, nil
	case FieldAPY:
		return reserveData.APY, nil
	case FieldUtilization:
		return reserveData.Utilization, nil
	default:
		return 0, fmt.Errorf("unsupported field type: %s", field)
	}
}

// ValidateChainID checks if a chain ID is supported
func ValidateChainID(chainID string) error {
	_, ok := supportedChains[chainID]
	if !ok {
		return fmt.Errorf("unsupported chain ID: %s. Supported chains: 1 (Ethereum Mainnet), 8453 (Base), 42161 (Arbitrum One)", chainID)
	}
	return nil
}

// GetChainNameFromID returns the chain name for a given chain ID
func GetChainNameFromID(chainID string) (string, error) {
	chainInfo, ok := supportedChains[chainID]
	if !ok {
		return "", fmt.Errorf("unsupported chain ID: %s", chainID)
	}
	return chainInfo.ChainName, nil
}

// bigRatDiv returns a float64 approximation of (a / b)
func bigRatDiv(a, b *big.Int) float64 {
	if b.Sign() == 0 {
		return 0
	}
	r := new(big.Rat).SetFrac(a, b)
	f, _ := r.Float64()
	return f
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
