package pendle

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// FieldType represents the type of field to monitor for Pendle PT markets
type FieldType string

const (
	FieldTVL FieldType = "TVL"
	FieldAPY FieldType = "APY"
)

// MarketData holds market data from Pendle API
type MarketData struct {
	ImpliedAPY float64 // Fixed APY for PT token holders
	TVL        float64 // Total Value Locked in USD
}

// ChainInfo holds chain information for Pendle
type ChainInfo struct {
	ChainID   string
	ChainName string
}

// Supported chains for Pendle
var supportedChains = map[string]ChainInfo{
	"1": {
		ChainID:   "1",
		ChainName: "Ethereum",
	},
	"42161": {
		ChainID:   "42161",
		ChainName: "Arbitrum",
	},
	"8453": {
		ChainID:   "8453",
		ChainName: "Base",
	},
	"5000": {
		ChainID:   "5000",
		ChainName: "Mantle",
	},
	"10": {
		ChainID:   "10",
		ChainName: "Optimism",
	},
	"56": {
		ChainID:   "56",
		ChainName: "BSC",
	},
}

const pendleAPIBaseURL = "https://api-v2.pendle.finance/core/v2"

// PendleMarketClient handles interactions with Pendle PT markets via REST API
type PendleMarketClient struct {
	chainID       string
	chainInfo     ChainInfo
	httpClient    *http.Client
	marketAddress string // Pendle market contract address
	marketName    string // Optional display name (e.g., "PT-weETH-26DEC2024")
}

// NewPendleMarketClient creates a new Pendle market client
func NewPendleMarketClient(chainID, marketAddress, marketName string) (*PendleMarketClient, error) {
	chainInfo, ok := supportedChains[chainID]
	if !ok {
		return nil, fmt.Errorf("unsupported chain ID: %s. Supported chains: 1 (Ethereum), 42161 (Arbitrum), 8453 (Base), 5000 (Mantle), 10 (Optimism), 56 (BSC)", chainID)
	}

	if marketAddress == "" {
		return nil, fmt.Errorf("marketAddress cannot be empty")
	}

	return &PendleMarketClient{
		chainID:       chainID,
		chainInfo:     chainInfo,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		marketAddress: marketAddress,
		marketName:    marketName,
	}, nil
}

// Close closes the HTTP client (no-op, kept for interface consistency)
func (c *PendleMarketClient) Close() {}

// pendleMarketAPIResponse represents the Pendle API response for a single market
type pendleMarketAPIResponse struct {
	Details struct {
		ImpliedAPY float64 `json:"impliedApy"`
		Liquidity  struct {
			USD float64 `json:"usd"`
		} `json:"liquidity"`
		PT struct {
			Address string `json:"address"`
			Symbol  string `json:"symbol"`
		} `json:"pt"`
	} `json:"details"`
}

// GetMarketData fetches market data from Pendle API
func (c *PendleMarketClient) GetMarketData(ctx context.Context) (*MarketData, error) {
	url := fmt.Sprintf("%s/%s/markets/%s", pendleAPIBaseURL, c.chainID, c.marketAddress)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "crypto-alert/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch market data from Pendle API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Pendle API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var apiResp pendleMarketAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse Pendle API response: %w", err)
	}

	return &MarketData{
		ImpliedAPY: apiResp.Details.ImpliedAPY * 100, // Convert to percentage
		TVL:        apiResp.Details.Liquidity.USD,
	}, nil
}

// GetFieldValue retrieves the value for a specific field (APY or TVL)
func (c *PendleMarketClient) GetFieldValue(ctx context.Context, field FieldType) (float64, error) {
	marketData, err := c.GetMarketData(ctx)
	if err != nil {
		return 0, err
	}

	switch field {
	case FieldAPY:
		return marketData.ImpliedAPY, nil
	case FieldTVL:
		return marketData.TVL, nil
	default:
		return 0, fmt.Errorf("unsupported field type: %s (supported: APY, TVL)", field)
	}
}

// GetChainNameFromID returns the chain name for a given chain ID
func GetChainNameFromID(chainID string) (string, error) {
	chainInfo, ok := supportedChains[chainID]
	if !ok {
		return "", fmt.Errorf("unsupported chain ID: %s", chainID)
	}
	return chainInfo.ChainName, nil
}
