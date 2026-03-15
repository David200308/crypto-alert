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

// hyperliquidVaultAPIResponse represents the Hyperliquid API response for vaultDetails
type hyperliquidVaultAPIResponse struct {
	Name     string  `json:"name"`
	Leader   string  `json:"leader"`
	APR      float64 `json:"apr"`
	IsClosed bool    `json:"isClosed"`
}

// clearinghouseStateRequest is the POST body for Hyperliquid clearinghouseState API
type clearinghouseStateRequest struct {
	Type string `json:"type"`
	User string `json:"user"`
}

// clearinghouseStateResponse represents the relevant fields from clearinghouseState
type clearinghouseStateResponse struct {
	MarginSummary struct {
		AccountValue string `json:"accountValue"`
	} `json:"marginSummary"`
}

// postInfo sends a POST request to the Hyperliquid /info endpoint and decodes the response into dst.
func (c *HyperliquidVaultClient) postInfo(ctx context.Context, body interface{}, dst interface{}) error {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.chainInfo.APIURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "crypto-alert/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call Hyperliquid API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Hyperliquid API returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("failed to parse Hyperliquid API response: %w", err)
	}
	return nil
}

// GetVaultData fetches vault data from Hyperliquid API.
// APR comes from vaultDetails; TVL comes from clearinghouseState.accountValue.
func (c *HyperliquidVaultClient) GetVaultData(ctx context.Context) (*VaultData, error) {
	// Fetch APR via vaultDetails
	var vaultResp hyperliquidVaultAPIResponse
	if err := c.postInfo(ctx, vaultDetailsRequest{Type: "vaultDetails", VaultAddress: c.ledgerAddress}, &vaultResp); err != nil {
		return nil, fmt.Errorf("vaultDetails: %w", err)
	}

	// Fetch TVL via clearinghouseState (marginSummary.accountValue is the vault's total equity in USD)
	var stateResp clearinghouseStateResponse
	if err := c.postInfo(ctx, clearinghouseStateRequest{Type: "clearinghouseState", User: c.ledgerAddress}, &stateResp); err != nil {
		return nil, fmt.Errorf("clearinghouseState: %w", err)
	}

	tvl, _ := strconv.ParseFloat(stateResp.MarginSummary.AccountValue, 64)

	return &VaultData{
		Name:   vaultResp.Name,
		APR:    vaultResp.APR * 100, // Convert decimal to percentage (0.15 → 15.0)
		TVL:    tvl,
		Closed: vaultResp.IsClosed,
	}, nil
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
