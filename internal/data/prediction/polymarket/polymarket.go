package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
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
	}

	return result, nil
}

// getMidpoints calls GET /midpoint?token_id=<id> for each token and returns tokenID -> midpoint.
// Response format: {"mid": "0.45"}
func (c *Client) getMidpoints(ctx context.Context, tokenIDs []string) (map[string]float64, error) {
	result := make(map[string]float64, len(tokenIDs))
	for _, tokenID := range tokenIDs {
		url := fmt.Sprintf("%s/midpoint?token_id=%s", c.baseURL, tokenID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}

		var raw struct {
			Mid string `json:"mid"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, fmt.Errorf("parse midpoint response: %w", err)
		}
		price, err := strconv.ParseFloat(raw.Mid, 64)
		if err != nil {
			log.Printf("⚠️  Polymarket: failed to parse midpoint for token %s: %v", tokenID, err)
			continue
		}
		result[tokenID] = price
	}
	return result, nil
}

// getMarketPrices calls GET /price?token_id=<id>&side=BUY and GET /price?token_id=<id>&side=SELL
// for each token and returns tokenID -> map[side]price.
// Response format: {"price": "0.45"}
func (c *Client) getMarketPrices(ctx context.Context, tokenIDs []string) (map[string]map[string]float64, error) {
	result := make(map[string]map[string]float64, len(tokenIDs))
	for _, tokenID := range tokenIDs {
		sides := map[string]float64{}
		for _, side := range []string{"BUY", "SELL"} {
			url := fmt.Sprintf("%s/price?token_id=%s&side=%s", c.baseURL, tokenID, side)
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Accept", "application/json")

			resp, err := c.httpClient.Do(req)
			if err != nil {
				return nil, err
			}
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return nil, err
			}
			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
			}

			var raw struct {
				Price string `json:"price"`
			}
			if err := json.Unmarshal(body, &raw); err != nil {
				return nil, fmt.Errorf("parse price response: %w", err)
			}
			p, err := strconv.ParseFloat(raw.Price, 64)
			if err != nil {
				log.Printf("⚠️  Polymarket: failed to parse %s price for token %s: %v", side, tokenID, err)
				continue
			}
			sides[side] = p
		}
		result[tokenID] = sides
	}
	return result, nil
}
