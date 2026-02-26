package price

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// PriceData represents price information from Pyth oracle
type PriceData struct {
	Symbol    string
	Price     float64
	Timestamp time.Time
}

// PythClient handles interactions with Pyth oracle
type PythClient struct {
	apiURL  string
	apiKey  string
	timeout time.Duration
}

// NewPythClient creates a new Pyth oracle client
func NewPythClient(apiURL, apiKey string) *PythClient {
	return &PythClient{
		apiURL:  apiURL,
		apiKey:  apiKey,
		timeout: 10 * time.Second,
	}
}

// PythAPIResponse represents the response from Pyth Hermes API
type PythAPIResponse struct {
	Parsed struct {
		Price string `json:"price"`
	} `json:"parsed"`
	PublishTime int64 `json:"publish_time"`
}

// GetPrice fetches the current price for a given symbol and price feed ID from Pyth oracle
func (c *PythClient) GetPrice(ctx context.Context, symbol string, priceFeedID string) (*PriceData, error) {
	// Create a context with timeout
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Use the provided price feed ID
	feedID := priceFeedID

	// Construct API URL - Pyth Hermes API endpoint
	// Format: https://hermes.pyth.network/v2/updates/price/latest?ids[]=<feed_id>
	apiURL := fmt.Sprintf("%s/v2/updates/price/latest?ids[]=%s", c.apiURL, feedID)

	// Create HTTP request
	req, err := http.NewRequestWithContext(reqCtx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// Make HTTP request
	client := &http.Client{Timeout: c.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch price for %s: %w", symbol, err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d for %s: %s", resp.StatusCode, symbol, string(body))
	}

	// Parse response
	var apiResponse struct {
		Parsed []struct {
			ID    string `json:"id"`
			Price struct {
				Price       string `json:"price"`
				Expo        int    `json:"expo"`
				PublishTime int64  `json:"publish_time"`
			} `json:"price"`
		} `json:"parsed"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response for %s: %w", symbol, err)
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response for %s: %w", symbol, err)
	}

	// Extract price data from response
	if len(apiResponse.Parsed) == 0 {
		return nil, fmt.Errorf("no price data found for symbol %s", symbol)
	}

	priceInfo := apiResponse.Parsed[0].Price

	// Parse price (price is in fixed-point format with expo)
	// Price comes as a string integer, parse it exactly and adjust for exponent
	priceInt, err := strconv.ParseInt(priceInfo.Price, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse price for %s: %w", symbol, err)
	}

	// Convert to float and adjust for exponent (10^expo) - use exact calculation
	price := float64(priceInt) * math.Pow(10, float64(priceInfo.Expo))

	// Convert publish time to timestamp
	publishTime := time.Unix(priceInfo.PublishTime, 0)

	priceData := &PriceData{
		Symbol:    symbol,
		Price:     price,
		Timestamp: publishTime,
	}

	return priceData, nil
}

// GetMultiplePrices fetches prices for multiple symbols using their price feed IDs concurrently
// symbolToFeedID maps symbol (e.g., "BTC/USD") to its Pyth price feed ID
// If a price fetch fails for a symbol, it is skipped and logged, but the function continues
// Uses goroutines to fetch prices in parallel for better performance
func (c *PythClient) GetMultiplePrices(ctx context.Context, symbolToFeedID map[string]string) (map[string]*PriceData, error) {
	prices := make(map[string]*PriceData)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Fetch prices concurrently using goroutines
	for symbol, feedID := range symbolToFeedID {
		wg.Add(1)
		go func(sym string, fid string) {
			defer wg.Done()

			priceData, err := c.GetPrice(ctx, sym, fid)
			if err != nil {
				// Log error but continue with other symbols
				log.Printf("⚠️  Failed to fetch price for %s: %v", sym, err)
				return
			}

			// Safely add to map using mutex
			mu.Lock()
			prices[sym] = priceData
			mu.Unlock()
		}(symbol, feedID)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	return prices, nil
}

// ValidatePrice checks if the price data is valid
func (p *PriceData) Validate() error {
	if p.Price <= 0 {
		return fmt.Errorf("invalid price: %f", p.Price)
	}
	if p.Symbol == "" {
		return fmt.Errorf("symbol cannot be empty")
	}
	return nil
}
