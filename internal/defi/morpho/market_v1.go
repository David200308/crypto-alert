package morpho

import (
	"context"
	_ "embed"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
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

//go:embed abi/erc20.json
var erc20ABIJSON string

//go:embed abi/market.json
var marketABIJSON string

// getERC20ABI returns the ERC20 ABI JSON string (shared across package)
func getERC20ABI() string {
	return erc20ABIJSON
}

// FieldType represents the type of field to monitor
type MarketFieldType string

const (
	MarketFieldTVL         MarketFieldType = "TVL"
	MarketFieldLiquidity   MarketFieldType = "LIQUIDITY"
	MarketFieldUtilization MarketFieldType = "UTILIZATION"
)

// ChainInfo holds chain information
type ChainInfo struct {
	ChainID   int64
	ChainName string
	RPCURL    string
}

// Supported chains mapping
var supportedChains = map[string]ChainInfo{
	"1": {
		ChainID:   1,
		ChainName: "Ethereum Mainnet",
		RPCURL:    "",
	},
	"8453": {
		ChainID:   8453,
		ChainName: "Base",
		RPCURL:    "",
	},
	"42161": {
		ChainID:   42161,
		ChainName: "Arbitrum One",
		RPCURL:    "",
	},
}

// Morpho Market contract addresses
// Source: Morpho v1 deployments - Morpho uses a singleton Market contract per chain
// Verified at: https://docs.morpho.org/get-started/resources/addresses
var morphoMarketAddresses = map[string]common.Address{
	"1":     common.HexToAddress("0xBBBBBbbBBb9cC5e90e3b3Af64bdAF62C37EEFFCb"), // Morpho Market on Ethereum Mainnet (verified)
	"8453":  common.HexToAddress("0xBBBBBbbBBb9cC5e90e3b3Af64bdAF62C37EEFFCb"), // Morpho Market on Base (verified - same address as Ethereum)
	"42161": common.HexToAddress("0x6c247b1F6182318877311737BaC0844bAa518F5e"), // Morpho Market on Arbitrum One (verified)
}

// getRPCURLForChain returns the RPC URL for a given chain ID
func getRPCURLForChain(chainID string) string {
	ensureEnvLoaded()
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

// MarketData holds market data from Morpho v1
type MarketData struct {
	TotalSupplyAssets *big.Int // TVL (total supply)
	TotalBorrowAssets *big.Int // Total borrowed
	Liquidity         *big.Int // Available liquidity (supply - borrow)
	Utilization       float64  // Calculated: (totalBorrow / totalSupply) * 100
}

// MorphoV1MarketClient handles interactions with Morpho v1 Markets
type MorphoV1MarketClient struct {
	chainID          string
	chainInfo        ChainInfo
	client           *ethclient.Client
	marketID         common.Hash // Market ID (bytes32)
	loanToken        common.Address
	collateralToken  common.Address
	oracle           common.Address // Oracle address (optional, needed for proper queries)
	irm              common.Address // Interest Rate Model address (optional, needed for proper queries)
	lltv             *big.Int       // Loan-to-Liquidation Value (optional, needed for proper queries)
	customMarketAddr string         // Custom Market contract address (optional, overrides default)
}

// NewMorphoV1MarketClient creates a new Morpho v1 market client
// oracleAddr, irmAddr, lltvStr are optional but required for accurate market data queries
// customMarketAddr is optional - if provided, overrides the default Market contract address
func NewMorphoV1MarketClient(chainID, marketID, loanTokenAddr, collateralTokenAddr, oracleAddr, irmAddr, lltvStr, customMarketAddr string) (*MorphoV1MarketClient, error) {
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

	// Parse market ID (should be a bytes32 hash)
	var marketIDHash common.Hash
	if strings.HasPrefix(marketID, "0x") {
		marketIDHash = common.HexToHash(marketID)
	} else {
		return nil, fmt.Errorf("invalid market ID format: %s (expected hex string)", marketID)
	}

	// Parse token addresses
	loanToken := common.HexToAddress(loanTokenAddr)
	collateralToken := common.HexToAddress(collateralTokenAddr)

	// Parse optional oracle, irm, lltv
	var oracleAddrParsed, irmAddrParsed common.Address
	var lltvValue *big.Int

	// Trim whitespace from addresses
	oracleAddr = strings.TrimSpace(oracleAddr)
	irmAddr = strings.TrimSpace(irmAddr)
	lltvStr = strings.TrimSpace(lltvStr)

	if oracleAddr != "" {
		oracleAddrParsed = common.HexToAddress(oracleAddr)
		// Validate address is not zero (common.HexToAddress returns zero address for invalid input)
		if oracleAddrParsed == (common.Address{}) && oracleAddr != "0x0000000000000000000000000000000000000000" {
			return nil, fmt.Errorf("invalid oracle_address: %s", oracleAddr)
		}
	}
	if irmAddr != "" {
		irmAddrParsed = common.HexToAddress(irmAddr)
		// Validate address is not zero
		if irmAddrParsed == (common.Address{}) && irmAddr != "0x0000000000000000000000000000000000000000" {
			return nil, fmt.Errorf("invalid irm_address: %s", irmAddr)
		}
	}
	if lltvStr != "" {
		var ok bool
		lltvValue, ok = new(big.Int).SetString(lltvStr, 10)
		if !ok {
			// Try parsing as hex
			if strings.HasPrefix(lltvStr, "0x") {
				lltvValue, ok = new(big.Int).SetString(lltvStr[2:], 16)
			}
		}
		if !ok || lltvValue == nil {
			return nil, fmt.Errorf("invalid lltv value: %s (must be a valid number)", lltvStr)
		}
		// Validate lltv is in reasonable range (0 to 1e18, i.e., 0 to 1.0 with 18 decimals)
		if lltvValue.Sign() < 0 || lltvValue.Cmp(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)) > 0 {
			return nil, fmt.Errorf("invalid lltv value: %s (must be between 0 and 1e18)", lltvStr)
		}
	}

	return &MorphoV1MarketClient{
		chainID:          chainID,
		chainInfo:        chainInfo,
		client:           client,
		marketID:         marketIDHash,
		loanToken:        loanToken,
		collateralToken:  collateralToken,
		oracle:           oracleAddrParsed,
		irm:              irmAddrParsed,
		lltv:             lltvValue,
		customMarketAddr: customMarketAddr,
	}, nil
}

// GetChainName returns the human-readable chain name
func (c *MorphoV1MarketClient) GetChainName() string {
	return c.chainInfo.ChainName
}

// GetChainID returns the chain ID
func (c *MorphoV1MarketClient) GetChainID() string {
	return c.chainID
}

// Close closes the RPC connection
func (c *MorphoV1MarketClient) Close() {
	if c.client != nil {
		c.client.Close()
	}
}

// GetMarketData fetches market data for the Morpho v1 market
// This queries the Morpho Market contract to get actual market supply/borrow data
func (c *MorphoV1MarketClient) GetMarketData(ctx context.Context) (*MarketData, error) {
	// Get Morpho Market contract address (use custom if provided, otherwise use default)
	var marketAddr common.Address
	if c.customMarketAddr != "" {
		marketAddr = common.HexToAddress(c.customMarketAddr)
	} else {
		var ok bool
		marketAddr, ok = morphoMarketAddresses[c.chainID]
		if !ok {
			return nil, fmt.Errorf("Morpho Market contract address not found for chain %s. Please provide market_contract_address in config", c.chainID)
		}
	}

	// Parse Market ABI
	marketABI, err := abi.JSON(strings.NewReader(marketABIJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to parse Market ABI: %w", err)
	}

	// Call market(marketId) which returns a struct with totalSupplyAssets and totalBorrowAssets
	// The market function takes a bytes32 market ID and returns market data
	method, exists := marketABI.Methods["market"]
	if !exists {
		return nil, fmt.Errorf("market method not found in Market ABI")
	}

	// Pack the market ID (bytes32)
	packedParams, err := method.Inputs.Pack(c.marketID)
	if err != nil {
		return nil, fmt.Errorf("failed to pack market ID: %w", err)
	}

	methodID := method.ID
	input := append(methodID, packedParams...)

	msg := ethereum.CallMsg{
		To:   &marketAddr,
		Data: input,
	}

	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		chainName := "unknown"
		if c.chainID == "8453" {
			chainName = "Base"
		} else if c.chainID == "1" {
			chainName = "Ethereum Mainnet"
		}

		usingDefault := c.customMarketAddr == ""
		defaultWarning := ""
		if usingDefault {
			defaultWarning = "\n⚠️  You're using the default contract address. For Base chain, you may need to add 'market_contract_address' to your config with the correct address."
		}

		return nil, fmt.Errorf("failed to call market on contract %s (chain %s/%s): %w.%s\n"+
			"Possible causes:\n"+
			"1. Wrong contract address for %s chain - verify and add 'market_contract_address' to config\n"+
			"2. Market ID doesn't exist on this chain\n"+
			"3. Contract exists but is deployed on a different chain\n\n"+
			"Market ID: %s\n"+
			"Find correct address at: https://docs.morpho.org/get-started/resources/addresses or inspect Morpho app on Base",
			marketAddr.Hex(), c.chainID, chainName, err, defaultWarning, chainName, c.marketID.Hex())
	}

	// Check if result is empty
	if len(result) == 0 {
		code, codeErr := c.client.CodeAt(ctx, marketAddr, nil)
		if codeErr == nil {
			if len(code) == 0 {
				return nil, fmt.Errorf("contract call returned empty data - no code at address %s. This address may not be a contract or may be incorrect", marketAddr.Hex())
			}
			return nil, fmt.Errorf("contract call returned empty data - function may not exist or call reverted. Contract: %s (has code: %d bytes), Method: market. Check if the contract address and ABI are correct.", marketAddr.Hex(), len(code))
		}
		return nil, fmt.Errorf("contract call returned empty data - function may not exist or call reverted. Contract: %s, Method: market", marketAddr.Hex())
	}

	// Unpack the result - the market function returns a struct with:
	// totalSupplyAssets (uint128), totalSupplyShares (uint128), totalBorrowAssets (uint128),
	// totalBorrowShares (uint128), lastUpdate (uint128), fee (uint128)
	unpacked, err := method.Outputs.UnpackValues(result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack market result (length: %d): %w", len(result), err)
	}

	if len(unpacked) < 3 {
		return nil, fmt.Errorf("unexpected number of return values: got %d, expected at least 3", len(unpacked))
	}

	// Extract totalSupplyAssets (index 0) and totalBorrowAssets (index 2)
	totalSupply, ok := unpacked[0].(*big.Int)
	if !ok {
		// Try uint128 - might be returned as a different type
		if val, ok := unpacked[0].(uint64); ok {
			totalSupply = new(big.Int).SetUint64(val)
		} else {
			return nil, fmt.Errorf("failed to extract totalSupplyAssets: unexpected type %T", unpacked[0])
		}
	}

	totalBorrow, ok := unpacked[2].(*big.Int)
	if !ok {
		// Try uint128 - might be returned as a different type
		if val, ok := unpacked[2].(uint64); ok {
			totalBorrow = new(big.Int).SetUint64(val)
		} else {
			// If we can't get borrow, set to 0
			totalBorrow = big.NewInt(0)
		}
	}

	liquidity := new(big.Int).Sub(totalSupply, totalBorrow)
	if liquidity.Sign() < 0 {
		liquidity = big.NewInt(0)
	}

	// Calculate utilization
	var utilization float64
	if totalSupply.Sign() > 0 {
		utilization = bigRatDiv(totalBorrow, totalSupply) * 100.0
	}

	return &MarketData{
		TotalSupplyAssets: totalSupply,
		TotalBorrowAssets: totalBorrow,
		Liquidity:         liquidity,
		Utilization:       utilization,
	}, nil
}

// getTokenBalance gets the balance of a token held by an address
func (c *MorphoV1MarketClient) getTokenBalance(ctx context.Context, holderAddr, tokenAddr common.Address, erc20ABI abi.ABI) (*big.Int, error) {
	// Call balanceOf(holderAddr) on the token
	method, exists := erc20ABI.Methods["balanceOf"]
	if !exists {
		// If balanceOf doesn't exist, try to add it to ABI
		// For now, return error
		return nil, fmt.Errorf("balanceOf method not found in ERC20 ABI")
	}

	packedParams, err := method.Inputs.Pack(holderAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to pack balanceOf input: %w", err)
	}

	methodID := method.ID
	input := append(methodID, packedParams...)

	msg := ethereum.CallMsg{
		To:   &tokenAddr,
		Data: input,
	}

	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call balanceOf on token %s: %w", tokenAddr.Hex(), err)
	}

	unpacked, err := method.Outputs.UnpackValues(result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack balanceOf result: %w", err)
	}

	if len(unpacked) < 1 {
		return nil, fmt.Errorf("unexpected number of return values: got %d, expected 1", len(unpacked))
	}

	balance, ok := unpacked[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("failed to extract balance")
	}

	return balance, nil
}

// getTokenTotalSupply calls totalSupply() on an ERC20 token contract
func (c *MorphoV1MarketClient) getTokenTotalSupply(ctx context.Context, tokenAddr common.Address, erc20ABI abi.ABI) (*big.Int, error) {
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

// GetFieldValue retrieves the value for a specific field (TVL, LIQUIDITY, or UTILIZATION)
func (c *MorphoV1MarketClient) GetFieldValue(ctx context.Context, field MarketFieldType) (float64, error) {
	marketData, err := c.GetMarketData(ctx)
	if err != nil {
		return 0, err
	}

	switch field {
	case MarketFieldTVL:
		// TVL is total supply, convert to float64
		// Note: For USDC (6 decimals), this would be in units of 1e6
		value, _ := new(big.Float).SetInt(marketData.TotalSupplyAssets).Float64()
		return value / 1000000.0, nil // Assuming 6 decimals for USDC
	case MarketFieldLiquidity:
		// Liquidity is available supply
		value, _ := new(big.Float).SetInt(marketData.Liquidity).Float64()
		return value / 1000000.0, nil // Assuming 6 decimals
	case MarketFieldUtilization:
		return marketData.Utilization, nil
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
