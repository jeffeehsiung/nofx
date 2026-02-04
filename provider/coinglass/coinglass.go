package coinglass

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"
)

const (
	// CoinGlassAPIEndpoint current V4 endpoint
	CoinGlassAPIEndpoint = "https://open-api-v4.coinglass.com/api"
	// DefaultTimeout for API requests
	DefaultTimeout = 10 * time.Second
	// DefaultLimit default number of positions to fetch
	DefaultLimit = 30
)

// Client CoinGlass API client
type Client struct {
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
	apiKey     string // Optional API key for premium endpoints
}

// NewClient creates a new CoinGlass API client with default settings
func NewClient() *Client {
	return &Client{
		baseURL: CoinGlassAPIEndpoint,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		timeout: DefaultTimeout,
	}
}

// NewClientWithAPIKey creates a new CoinGlass API client with an API key
func NewClientWithAPIKey(apiKey string) *Client {
	client := NewClient()
	client.apiKey = apiKey
	return client
}

// CoinsMarketsResponse CoinGlass coins markets API response
type CoinsMarketsResponse struct {
	Code    string           `json:"code"`
	Message string           `json:"msg"`
	Data    []CoinMarketData `json:"data"`
}

// CoinMarketData single coin's market data from coins-markets endpoint
type CoinMarketData struct {
	Symbol                       string  `json:"symbol"`
	CurrentPrice                 float64 `json:"current_price"`
	OpenInterestUSD              float64 `json:"open_interest_usd"`
	OpenInterestQuantity         float64 `json:"open_interest_quantity"`
	OpenInterestChangePercent24h float64 `json:"open_interest_change_percent_24h"`
	PriceChangePercent24h        float64 `json:"price_change_percent_24h"`
	VolumeUSD24h                 float64 `json:"volume_usd_24h"`
	AvgFundingRateByOI           float64 `json:"avg_funding_rate_by_oi"`
	LongShortRatio24h            float64 `json:"long_short_ratio_24h"`
	TakerBuyRatio24h             float64 `json:"taker_buy_ratio_24h"`
}

// PositionCG Position data (backward compatibility with old structure)
type PositionCG struct {
	Symbol           string  `json:"symbol"`
	Pair             string  `json:"pair"`
	OpenInterest     float64 `json:"openInterest"`
	OpenInterestUsd  float64 `json:"openInterestUsd"`
	Change           float64 `json:"change"`
	ChangePercent    float64 `json:"changePercent"`
	PriceChange      float64 `json:"priceChange"`
	Funding          float64 `json:"funding"`
	Volume24h        float64 `json:"volume24h"`
	VolumeUsd24h     float64 `json:"volumeUsd24h"`
	TakerLongRatio   float64 `json:"takerLongRatio"`
	TakerShortRatio  float64 `json:"takerShortRatio"`
	NetLongPosition  float64 `json:"netLongPosition"`
	NetShortPosition float64 `json:"netShortPosition"`
	Leverage         string  `json:"leverage"`
	Exchange         string  `json:"exchange"`
}

// GetTopOISymbols fetches top symbols by open interest increase
// This uses the coins-markets endpoint if API key is available,
// otherwise returns empty list to use fallback system
func (c *Client) GetTopOISymbols(duration string, limit int) ([]PositionCG, error) {
	// If no API key, return error to use fallback
	if c.apiKey == "" {
		return nil, fmt.Errorf("CoinGlass premium API requires API key - using Binance fallback instead")
	}
	durationParam := duration
	if durationParam == "" {
		durationParam = "24h"
	}
	if limit <= 0 || limit > 50 {
		limit = DefaultLimit
	}

	// coins-markets endpoint is the primary source for OI data
	url := fmt.Sprintf("%s/futures/coins-markets?per_page=%d&page=1&duration=%s", c.baseURL, limit, durationParam)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add API key header if present
	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch CoinGlass OI data: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read CoinGlass response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CoinGlass API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp CoinsMarketsResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse CoinGlass response: %w", err)
	}

	// CoinGlass returns code "0" on success
	if apiResp.Code != "0" {
		return nil, fmt.Errorf("CoinGlass API returned error: %s", apiResp.Message)
	}

	if len(apiResp.Data) == 0 {
		return nil, fmt.Errorf("no OI data returned from CoinGlass")
	}

	// Convert and sort by OI increase percentage
	positions := make([]PositionCG, 0, len(apiResp.Data))
	for i, data := range apiResp.Data {
		pos := PositionCG{
			Symbol:          data.Symbol,
			Pair:            data.Symbol + "/USDT",
			OpenInterest:    data.OpenInterestQuantity,
			OpenInterestUsd: data.OpenInterestUSD,
			Change:          data.OpenInterestUSD * (data.OpenInterestChangePercent24h / 100),
			ChangePercent:   data.OpenInterestChangePercent24h,
			PriceChange:     data.PriceChangePercent24h,
			Funding:         data.AvgFundingRateByOI,
			VolumeUsd24h:    data.VolumeUSD24h,
			TakerLongRatio:  data.LongShortRatio24h / (1 + data.LongShortRatio24h), // Convert ratio to percentage
			TakerShortRatio: 1 / (1 + data.LongShortRatio24h),                      // Convert ratio to percentage
			Leverage:        "cross",
			Exchange:        "Aggregated",
		}
		positions = append(positions, pos)
		_ = i // Track iteration for ranking
	}

	// Sort by OI change percent (descending)
	sort.Slice(positions, func(i, j int) bool {
		return positions[i].ChangePercent > positions[j].ChangePercent
	})

	// Return top limit
	if len(positions) > limit {
		positions = positions[:limit]
	}

	return positions, nil
}

// GetLongShortRatio fetches long/short ratio for a specific symbol
// Note: Requires API key for premium features
func (c *Client) GetLongShortRatio(symbol string) (float64, error) {
	if c.apiKey == "" {
		return 0, fmt.Errorf("CoinGlass premium API requires API key")
	}

	url := fmt.Sprintf("%s/futures/global-longshort-account-ratio?symbol=%s&exchange=Binance", c.baseURL, symbol)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch long/short ratio: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Code string `json:"code"`
		Data struct {
			LongRatio float64 `json:"longRatio"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Code != "0" {
		return 0, fmt.Errorf("API returned error")
	}

	return result.Data.LongRatio, nil
}

// GetFundingRate fetches current funding rate for a symbol
// Note: Requires API key for premium features
func (c *Client) GetFundingRate(symbol string) (float64, error) {
	if c.apiKey == "" {
		return 0, fmt.Errorf("CoinGlass premium API requires API key")
	}

	url := fmt.Sprintf("%s/futures/funding-rate-current?symbol=%s&exchange=Binance", c.baseURL, symbol)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch funding rate: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Code string `json:"code"`
		Data struct {
			FundingRate float64 `json:"fundingRate"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Code != "0" {
		return 0, fmt.Errorf("API returned error")
	}

	return result.Data.FundingRate, nil
}
