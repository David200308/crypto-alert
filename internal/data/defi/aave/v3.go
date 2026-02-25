package aave

import (
	"context"
	_ "embed"
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"crypto-alert/internal/utils"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

//go:embed abi/pool.json
var poolABIJSON string

//go:embed abi/erc20.json
var erc20ABIJSON string

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

// getRPCURLForChain returns a randomly selected RPC URL for a given chain ID.
// Supports comma-separated RPC URLs in env vars for load balancing.
func getRPCURLForChain(chainID string) string {
	return utils.GetRPCURLForChain(chainID)
}

// Pool contract addresses for each chain (proxy contracts)
// Source: https://docs.aave.com/developers/deployed-contracts/v3-mainnet
var poolAddresses = map[string]common.Address{
	"1":     common.HexToAddress("0x87870Bca3F3fD6335C3F4ce8392D69350B4fA4E2"), // Ethereum Mainnet Pool proxy
	"8453":  common.HexToAddress("0xA238Dd80C259a72e81d7e4664a9801593F98d1c5"), // Base Pool proxy
	"42161": common.HexToAddress("0x794a61358D6845594F94dc1DB02A252b5b4814aD"), // Arbitrum One Pool proxy
}

// FieldType represents the type of field to monitor
type FieldType string

const (
	FieldTVL         FieldType = "TVL"
	FieldAPY         FieldType = "APY"
	FieldUtilization FieldType = "UTILIZATION"
	FieldLiquidity   FieldType = "LIQUIDITY"
)

// ReserveData holds reserve data from Aave
type ReserveData struct {
	TotalAToken       *big.Int // TVL (total supply)
	TotalStableDebt   *big.Int
	TotalVariableDebt *big.Int
	LiquidityRate     *big.Int // Used for APY calculation
	Liquidity         *big.Int // Available liquidity (totalSupply - totalDebt)
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
	usePool   bool // true if using Pool contract directly, false if using PoolDataProvider
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

	// Use Pool contract directly for all chains
	parsedABI, err := abi.JSON(strings.NewReader(poolABIJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to parse Pool ABI: %w", err)
	}

	contractAddr, ok := poolAddresses[chainID]
	if !ok {
		return nil, fmt.Errorf("pool address not found for chain %s", chainID)
	}

	contract := bind.NewBoundContract(contractAddr, parsedABI, client, client, client)

	return &AaveV3Client{
		chainID:   chainID,
		chainInfo: chainInfo,
		client:    client,
		contract:  contract,
		abi:       parsedABI,
		usePool:   true, // Always use Pool contract now
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
	// Always use Pool contract for all chains
	return c.getReserveDataFromPool(ctx, tokenAddress)
}

// getReserveDataFromPool fetches reserve data using Pool contract (Ethereum Mainnet)
func (c *AaveV3Client) getReserveDataFromPool(ctx context.Context, tokenAddress common.Address) (*ReserveData, error) {
	// Get the method from ABI
	method, exists := c.abi.Methods["getReserveData"]
	if !exists {
		return nil, fmt.Errorf("getReserveData method not found in Pool ABI")
	}

	// Pack the input parameters
	packedParams, err := method.Inputs.Pack(tokenAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to pack input: %w", err)
	}

	// Prepend the method selector
	methodID := method.ID
	input := append(methodID, packedParams...)

	// Get the Pool contract address for this chain
	contractAddr, ok := poolAddresses[c.chainID]
	if !ok {
		return nil, fmt.Errorf("pool address not found for chain %s", c.chainID)
	}

	// Call the contract using ethclient.CallContract
	msg := ethereum.CallMsg{
		To:   &contractAddr,
		Data: input,
	}

	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call Pool contract: %w", err)
	}

	// Unpack the output - getReserveData returns a struct (tuple)
	// UnpackValues returns the struct as a single element
	unpacked, err := method.Outputs.UnpackValues(result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack Pool contract output: %w", err)
	}

	if len(unpacked) != 1 {
		return nil, fmt.Errorf("unexpected number of return values: got %d, expected 1 (struct)", len(unpacked))
	}

	// Use reflection to access struct fields dynamically
	structValue := reflect.ValueOf(unpacked[0])
	if structValue.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct type, got %T", unpacked[0])
	}

	// Extract fields using reflection
	var aTokenAddr, stableDebtTokenAddr, variableDebtTokenAddr common.Address
	var currentLiquidityRate *big.Int

	// Field names as they appear in the struct (case-sensitive)
	fieldNames := []string{"ATokenAddress", "StableDebtTokenAddress", "VariableDebtTokenAddress", "CurrentLiquidityRate"}
	fieldValues := make([]interface{}, len(fieldNames))

	for i, fieldName := range fieldNames {
		field := structValue.FieldByName(fieldName)
		if !field.IsValid() {
			// Try lowercase version
			field = structValue.FieldByName(strings.ToLower(fieldName[:1]) + fieldName[1:])
		}
		if !field.IsValid() {
			return nil, fmt.Errorf("field %s not found in struct (type: %T)", fieldName, unpacked[0])
		}
		fieldValues[i] = field.Interface()
	}

	// Extract addresses
	if addr, ok := fieldValues[0].(common.Address); ok {
		aTokenAddr = addr
	} else {
		return nil, fmt.Errorf("failed to extract aTokenAddress, got type %T", fieldValues[0])
	}

	if addr, ok := fieldValues[1].(common.Address); ok {
		stableDebtTokenAddr = addr
	} else {
		return nil, fmt.Errorf("failed to extract stableDebtTokenAddress, got type %T", fieldValues[1])
	}

	if addr, ok := fieldValues[2].(common.Address); ok {
		variableDebtTokenAddr = addr
	} else {
		return nil, fmt.Errorf("failed to extract variableDebtTokenAddress, got type %T", fieldValues[2])
	}

	// Extract currentLiquidityRate
	if rate, ok := fieldValues[3].(*big.Int); ok {
		currentLiquidityRate = rate
	} else {
		return nil, fmt.Errorf("failed to extract currentLiquidityRate, got type %T", fieldValues[3])
	}

	// Parse ERC20 ABI for totalSupply calls
	erc20ABI, err := abi.JSON(strings.NewReader(erc20ABIJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ERC20 ABI: %w", err)
	}

	// Get totalSupply from aToken
	totalAToken, err := c.getTokenTotalSupply(ctx, aTokenAddr, erc20ABI)
	if err != nil {
		return nil, fmt.Errorf("failed to get aToken totalSupply: %w", err)
	}

	// Get totalSupply from stableDebtToken
	totalStableDebt, err := c.getTokenTotalSupply(ctx, stableDebtTokenAddr, erc20ABI)
	if err != nil {
		return nil, fmt.Errorf("failed to get stableDebtToken totalSupply: %w", err)
	}

	// Get totalSupply from variableDebtToken
	totalVariableDebt, err := c.getTokenTotalSupply(ctx, variableDebtTokenAddr, erc20ABI)
	if err != nil {
		return nil, fmt.Errorf("failed to get variableDebtToken totalSupply: %w", err)
	}

	// Calculate total debt
	totalDebt := new(big.Int).Add(totalStableDebt, totalVariableDebt)

	// Calculate liquidity: available supply (totalSupply - totalDebt)
	liquidity := new(big.Int).Sub(totalAToken, totalDebt)
	if liquidity.Sign() < 0 {
		liquidity = big.NewInt(0)
	}

	// Calculate utilization: (totalDebt / totalSupply) * 100
	var utilization float64
	if totalAToken.Sign() > 0 {
		utilization = bigRatDiv(totalDebt, totalAToken) * 100.0
	}

	// Calculate APY from currentLiquidityRate
	// currentLiquidityRate is in RAY units (1e27), so APY = (currentLiquidityRate / 1e27) * 100
	var apy float64
	if currentLiquidityRate.Sign() > 0 {
		// Convert RAY to percentage: (currentLiquidityRate / 1e27) * 100
		ray := new(big.Int).Exp(big.NewInt(10), big.NewInt(27), nil)
		apy = bigRatDiv(currentLiquidityRate, ray) * 100.0
	}

	return &ReserveData{
		TotalAToken:       totalAToken,
		TotalStableDebt:   totalStableDebt,
		TotalVariableDebt: totalVariableDebt,
		LiquidityRate:     currentLiquidityRate,
		Liquidity:         liquidity,
		Utilization:       utilization,
		APY:               apy,
	}, nil
}

// getTokenTotalSupply calls totalSupply() on an ERC20 token contract
func (c *AaveV3Client) getTokenTotalSupply(ctx context.Context, tokenAddr common.Address, erc20ABI abi.ABI) (*big.Int, error) {
	method, exists := erc20ABI.Methods["totalSupply"]
	if !exists {
		return nil, fmt.Errorf("totalSupply method not found in ERC20 ABI")
	}

	// Pack the input (no parameters for totalSupply)
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

// GetFieldValue retrieves the value for a specific field (TVL, APY, UTILIZATION, or LIQUIDITY)
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
	case FieldLiquidity:
		// Liquidity is available supply (totalSupply - totalDebt), convert to float64
		// Note: For USDC (6 decimals), this would be in units of 1e6
		value, _ := new(big.Float).SetInt(reserveData.Liquidity).Float64()
		return value / 1000000.0, nil
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
