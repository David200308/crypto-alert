package hyperliquid

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// FieldType represents the type of field to monitor for Hyperliquid vaults
type FieldType string

const (
	FieldAPY FieldType = "APY" // Maps to APR in Hyperliquid API
	FieldTVL FieldType = "TVL"
)

// VaultData holds vault data from Hyperliquid API
type VaultData struct {
	Name   string
	APR    float64 // Annual Percentage Rate (exposed as APY in our system)
	TVL    float64 // Total Value Locked in USD
	Closed bool
}

// ChainInfo holds chain information for Hyperliquid
type ChainInfo struct {
	ChainID   string
	ChainName string
	APIURL    string
}

var supportedChains = map[string]ChainInfo{
	"hyperliquid": {
		ChainID:   "hyperliquid",
		ChainName: "Hyperliquid",
		APIURL:    "https://api.hyperliquid.xyz/info",
	},
}

// HyperliquidVaultClient handles interactions with Hyperliquid vaults via REST API
type HyperliquidVaultClient struct {
	chainID       string
	chainInfo     ChainInfo
	httpClient    *http.Client
	ledgerAddress string // Hyperliquid vault ledger address
	vaultName     string // Optional display name
}

// NewHyperliquidVaultClient creates a new Hyperliquid vault client
func NewHyperliquidVaultClient(chainID, ledgerAddress, vaultName string) (*HyperliquidVaultClient, error) {
	chainInfo, ok := supportedChains[chainID]
	if !ok {
		// Default to hyperliquid mainnet if unrecognized
		if chainID == "" {
			chainInfo = supportedChains["hyperliquid"]
		} else {
			return nil, fmt.Errorf("unsupported chain ID: %s. Supported chains: hyperliquid", chainID)
		}
	}

	if ledgerAddress == "" {
		return nil, fmt.Errorf("ledgerAddress cannot be empty")
	}

	return &HyperliquidVaultClient{
		chainID:       chainID,
		chainInfo:     chainInfo,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		ledgerAddress: ledgerAddress,
		vaultName:     vaultName,
	}, nil
}

// Close closes the HTTP client (no-op, kept for interface consistency)
func (c *HyperliquidVaultClient) Close() {}

// vaultDetailsRequest is the POST body for Hyperliquid vaultDetails API
type vaultDetailsRequest struct {
	Type         string `json:"type"`
	VaultAddress string `json:"vaultAddress"`
}

// hyperliquidVaultAPIResponse represents the Hyperliquid API response for vaultDetails.
// APR (apr) and TVL (portfolio[day] latest accountValue) are both available in a single call.
// portfolio format: [[periodString, {accountValueHistory: [[timestampMs, valueStr], ...]}], ...]
type hyperliquidVaultAPIResponse struct {
	Name      string        `json:"name"`
	Leader    string        `json:"leader"`
	APR       float64       `json:"apr"`
	IsClosed  bool          `json:"isClosed"`
	Portfolio []interface{} `json:"portfolio"`
}

// GetVaultData fetches vault data from Hyperliquid API using a single vaultDetails call.
// APR comes from the top-level apr field.
// TVL comes from portfolio[day] latest accountValueHistory entry — matches the app exactly.
func (c *HyperliquidVaultClient) GetVaultData(ctx context.Context) (*VaultData, error) {
	bodyBytes, err := json.Marshal(vaultDetailsRequest{Type: "vaultDetails", VaultAddress: c.ledgerAddress})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.chainInfo.APIURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "crypto-alert/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch vault data from Hyperliquid API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Hyperliquid API returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var apiResp hyperliquidVaultAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse Hyperliquid API response: %w", err)
	}

	// TVL = latest accountValue from portfolio[period="day"].accountValueHistory
	// This is the same value shown in the Hyperliquid app and works for both parent and child vaults.
	tvl := latestAccountValueFromPortfolio(apiResp.Portfolio)

	return &VaultData{
		Name:   apiResp.Name,
		APR:    apiResp.APR * 100, // Convert decimal to percentage (0.06 → 6.0)
		TVL:    tvl,
		Closed: apiResp.IsClosed,
	}, nil
}

// latestAccountValueFromPortfolio extracts the most recent accountValue from the portfolio field.
// portfolio structure: [[periodString, {accountValueHistory: [[timestampMs, valueStr], ...]}], ...]
// We use the "day" period and take the last entry, which matches what the app displays.
func latestAccountValueFromPortfolio(portfolio []interface{}) float64 {
	for _, entry := range portfolio {
		pair, ok := entry.([]interface{})
		if !ok || len(pair) < 2 {
			continue
		}
		period, ok := pair[0].(string)
		if !ok || period != "day" {
			continue
		}
		periodData, ok := pair[1].(map[string]interface{})
		if !ok {
			continue
		}
		history, ok := periodData["accountValueHistory"].([]interface{})
		if !ok || len(history) == 0 {
			continue
		}
		// Take the last (most recent) entry: [timestampMs, valueStr]
		last, ok := history[len(history)-1].([]interface{})
		if !ok || len(last) < 2 {
			continue
		}
		valStr, ok := last[1].(string)
		if !ok {
			continue
		}
		tvl, _ := strconv.ParseFloat(valStr, 64)
		return tvl
	}
	return 0
}

// GetFieldValue retrieves the value for a specific field (APY or TVL)
func (c *HyperliquidVaultClient) GetFieldValue(ctx context.Context, field FieldType) (float64, error) {
	vaultData, err := c.GetVaultData(ctx)
	if err != nil {
		return 0, err
	}

	switch field {
	case FieldAPY:
		return vaultData.APR, nil
	case FieldTVL:
		return vaultData.TVL, nil
	default:
		return 0, fmt.Errorf("unsupported field type: %s (supported: APY, TVL)", field)
	}
}

// GetChainNameFromID returns the chain name for a given chain ID
func GetChainNameFromID(chainID string) (string, error) {
	if chainID == "" || chainID == "hyperliquid" {
		return "Hyperliquid", nil
	}
	chainInfo, ok := supportedChains[chainID]
	if !ok {
		return "", fmt.Errorf("unsupported chain ID: %s", chainID)
	}
	return chainInfo.ChainName, nil
}
