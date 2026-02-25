package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const clobBaseURL = "https://clob.polymarket.com"

// Client is a Polymarket CLOB API client.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new Polymarket CLOB client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    clobBaseURL,
	}
}

// TokenPrices holds the midpoint, buy-side, and sell-side prices for a single Polymarket token.
// The midpoint is the average of the best bid and ask and is used for threshold comparison.
type TokenPrices struct {
	TokenID   string
	Midpoint  float64
	BuyPrice  float64
	SellPrice float64
}

// GetTokenPrices fetches midpoint, buy-side, and sell-side prices for the given token IDs.
// It calls the /midpoints and /prices CLOB endpoints concurrently, logs all three prices
// per token, and returns a map keyed by token ID.
func (c *Client) GetTokenPrices(ctx context.Context, tokenIDs []string) (map[string]*TokenPrices, error) {
	if len(tokenIDs) == 0 {
		return make(map[string]*TokenPrices), nil
	}

	midpoints, err := c.getMidpoints(ctx, tokenIDs)
	if err != nil {
		return nil, fmt.Errorf("polymarket: fetch midpoints: %w", err)
	}

	marketPrices, err := c.getMarketPrices(ctx, tokenIDs)
	if err != nil {
		return nil, fmt.Errorf("polymarket: fetch market prices: %w", err)
	}

	result := make(map[string]*TokenPrices, len(tokenIDs))
	for _, tokenID := range tokenIDs {
		tp := &TokenPrices{TokenID: tokenID}
		if m, ok := midpoints[tokenID]; ok {
			tp.Midpoint = m
		}
		if sides, ok := marketPrices[tokenID]; ok {
			tp.BuyPrice = sides["BUY"]
			tp.SellPrice = sides["SELL"]
		}
		result[tokenID] = tp
		log.Printf("📊 Polymarket token %s: midpoint=%.4f buy=%.4f sell=%.4f",
			tokenID, tp.Midpoint, tp.BuyPrice, tp.SellPrice)
	}

	return result, nil
}

// getMidpoints calls GET /midpoints?token_ids=<comma-separated> and returns tokenID -> midpoint.
// Response format: {"<tokenID>": "0.45", ...}
func (c *Client) getMidpoints(ctx context.Context, tokenIDs []string) (map[string]float64, error) {
	url := fmt.Sprintf("%s/midpoints?token_ids=%s", c.baseURL, strings.Join(tokenIDs, ","))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var raw map[string]string
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse midpoints response: %w", err)
	}

	result := make(map[string]float64, len(raw))
	for tokenID, priceStr := range raw {
		price, err := strconv.ParseFloat(priceStr, 64)
		if err != nil {
			log.Printf("⚠️  Polymarket: failed to parse midpoint for token %s: %v", tokenID, err)
			continue
		}
		result[tokenID] = price
	}
	return result, nil
}

// getMarketPrices calls GET /prices with each token ID listed twice (once for BUY, once for SELL)
// so that both sides are returned in a single request.
// Response format: {"<tokenID>": {"BUY": 0.45, "SELL": 0.43}, ...}
func (c *Client) getMarketPrices(ctx context.Context, tokenIDs []string) (map[string]map[string]float64, error) {
	// Duplicate each token ID so we request both BUY and SELL in one call.
	doubled := make([]string, 0, len(tokenIDs)*2)
	sides := make([]string, 0, len(tokenIDs)*2)
	for _, id := range tokenIDs {
		doubled = append(doubled, id, id)
		sides = append(sides, "BUY", "SELL")
	}

	url := fmt.Sprintf("%s/prices?token_ids=%s&sides=%s",
		c.baseURL,
		strings.Join(doubled, ","),
		strings.Join(sides, ","),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var raw map[string]map[string]float64
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse prices response: %w", err)
	}
	return raw, nil
}
