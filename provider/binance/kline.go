package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// Binance API endpoints
	SpotAPIURL    = "https://api.binance.com"
	FuturesAPIURL = "https://fapi.binance.com"
)

// Kline represents a single OHLCV candle from Binance
type Kline struct {
	OpenTime  int64   `json:"openTime"`  // Open time in milliseconds
	CloseTime int64   `json:"closeTime"` // Close time in milliseconds
	Symbol    string  `json:"symbol"`    // Trading pair symbol
	Interval  string  `json:"interval"`  // Kline interval
	Open      float64 `json:"open"`      // Open price
	High      float64 `json:"high"`      // High price
	Low       float64 `json:"low"`       // Low price
	Close     float64 `json:"close"`     // Close price
	Volume    float64 `json:"volume"`    // Volume in base asset
}

// MarketType represents the type of Binance market
type MarketType string

const (
	MarketTypeSpot    MarketType = "spot"
	MarketTypeFutures MarketType = "futures"
)

// Client is the Binance API client
type Client struct {
	spotURL    string
	futuresURL string
	client     *http.Client
}

// NewClient creates a new Binance client
func NewClient() *Client {
	return &Client{
		spotURL:    SpotAPIURL,
		futuresURL: FuturesAPIURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetKlines fetches historical candlestick data for a symbol
// symbol: trading pair (e.g., "BTCUSDT", "ETHUSDT")
// interval: "1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "6h", "8h", "12h", "1d", "3d", "1w", "1M"
// limit: number of candles to fetch (max 1500, default 500)
// marketType: "spot" or "futures" (default: futures)
func (c *Client) GetKlines(ctx context.Context, symbol string, interval string, limit int, marketType MarketType) ([]Kline, error) {
	// Normalize symbol
	symbol = NormalizeSymbol(symbol)

	// Default to futures if not specified
	if marketType == "" {
		marketType = MarketTypeFutures
	}

	// Validate and set limit
	if limit <= 0 {
		limit = 500
	}
	if limit > 1500 {
		limit = 1500
	}

	// Validate interval
	interval = MapInterval(interval)

	// Build API URL based on market type
	var apiURL string
	var endpoint string
	if marketType == MarketTypeSpot {
		apiURL = c.spotURL
		endpoint = "/api/v3/klines"
	} else {
		apiURL = c.futuresURL
		endpoint = "/fapi/v1/klines"
	}

	// Build request URL with query parameters
	url := fmt.Sprintf("%s%s?symbol=%s&interval=%s&limit=%d",
		apiURL, endpoint, symbol, interval, limit)

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance API error for symbol '%s' (status %d): %s", symbol, resp.StatusCode, string(body))
	}

	// Parse response
	// Binance returns: [[openTime, open, high, low, close, volume, closeTime, ...], ...]
	var rawKlines [][]interface{}
	if err := json.Unmarshal(body, &rawKlines); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert to Kline structs
	klines := make([]Kline, 0, len(rawKlines))
	for _, raw := range rawKlines {
		if len(raw) < 7 {
			continue
		}

		kline, err := parseRawKline(raw, symbol, interval)
		if err != nil {
			continue // Skip invalid klines
		}

		klines = append(klines, kline)
	}

	return klines, nil
}

// GetFuturesKlines is a convenience method for fetching futures market klines
func (c *Client) GetFuturesKlines(ctx context.Context, symbol string, interval string, limit int) ([]Kline, error) {
	return c.GetKlines(ctx, symbol, interval, limit, MarketTypeFutures)
}

// GetSpotKlines is a convenience method for fetching spot market klines
func (c *Client) GetSpotKlines(ctx context.Context, symbol string, interval string, limit int) ([]Kline, error) {
	return c.GetKlines(ctx, symbol, interval, limit, MarketTypeSpot)
}

// parseRawKline converts raw Binance kline data to Kline struct
func parseRawKline(raw []interface{}, symbol, interval string) (Kline, error) {
	// Binance kline format:
	// [
	//   0: Open time (ms)
	//   1: Open price (string)
	//   2: High price (string)
	//   3: Low price (string)
	//   4: Close price (string)
	//   5: Volume (string)
	//   6: Close time (ms)
	//   7: Quote asset volume (string)
	//   8: Number of trades
	//   9: Taker buy base asset volume (string)
	//   10: Taker buy quote asset volume (string)
	//   11: Ignore
	// ]

	openTime, ok := raw[0].(float64)
	if !ok {
		return Kline{}, fmt.Errorf("invalid open time")
	}

	closeTime, ok := raw[6].(float64)
	if !ok {
		return Kline{}, fmt.Errorf("invalid close time")
	}

	open, err := parseFloatFromInterface(raw[1])
	if err != nil {
		return Kline{}, fmt.Errorf("invalid open price: %w", err)
	}

	high, err := parseFloatFromInterface(raw[2])
	if err != nil {
		return Kline{}, fmt.Errorf("invalid high price: %w", err)
	}

	low, err := parseFloatFromInterface(raw[3])
	if err != nil {
		return Kline{}, fmt.Errorf("invalid low price: %w", err)
	}

	close, err := parseFloatFromInterface(raw[4])
	if err != nil {
		return Kline{}, fmt.Errorf("invalid close price: %w", err)
	}

	volume, err := parseFloatFromInterface(raw[5])
	if err != nil {
		return Kline{}, fmt.Errorf("invalid volume: %w", err)
	}

	return Kline{
		OpenTime:  int64(openTime),
		CloseTime: int64(closeTime),
		Symbol:    symbol,
		Interval:  interval,
		Open:      open,
		High:      high,
		Low:       low,
		Close:     close,
		Volume:    volume,
	}, nil
}

// parseFloatFromInterface parses float64 from interface{} (handles both string and float64)
func parseFloatFromInterface(v interface{}) (float64, error) {
	switch val := v.(type) {
	case string:
		return strconv.ParseFloat(val, 64)
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", v)
	}
}

// NormalizeSymbol normalizes symbol to Binance format
// Examples:
//   - "BTC" -> "BTCUSDT"
//   - "BTCUSDT" -> "BTCUSDT"
//   - "btcusdt" -> "BTCUSDT"
//   - "ETH" -> "ETHUSDT"
func NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))

	// Already has USDT suffix
	if strings.HasSuffix(symbol, "USDT") {
		return symbol
	}

	// Already has BUSD suffix (legacy)
	if strings.HasSuffix(symbol, "BUSD") {
		return symbol
	}

	// Add USDT suffix for crypto base assets
	return symbol + "USDT"
}

// MapInterval maps common interval strings to Binance format
// Binance supports: 1m, 3m, 5m, 15m, 30m, 1h, 2h, 4h, 6h, 8h, 12h, 1d, 3d, 1w, 1M
func MapInterval(interval string) string {
	// Normalize input while preserving monthly interval ("1M")
	interval = strings.TrimSpace(interval)
	if interval == "1M" || strings.EqualFold(interval, "1mo") {
		return "1M"
	}
	interval = strings.ToLower(interval)

	// Binance uses specific formats
	validIntervals := map[string]string{
		"1m":  "1m",
		"3m":  "3m",
		"5m":  "5m",
		"15m": "15m",
		"30m": "30m",
		"1h":  "1h",
		"2h":  "2h",
		"4h":  "4h",
		"6h":  "6h",
		"8h":  "8h",
		"12h": "12h",
		"1d":  "1d",
		"3d":  "3d",
		"1w":  "1w",
		"1mo": "1M",
	}

	if mapped, ok := validIntervals[interval]; ok {
		return mapped
	}

	// Default to 5m if invalid
	return "5m"
}

// GetIntervalDuration returns the duration for a given interval
func GetIntervalDuration(interval string) time.Duration {
	switch interval {
	case "1m":
		return time.Minute
	case "3m":
		return 3 * time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "1h":
		return time.Hour
	case "2h":
		return 2 * time.Hour
	case "4h":
		return 4 * time.Hour
	case "6h":
		return 6 * time.Hour
	case "8h":
		return 8 * time.Hour
	case "12h":
		return 12 * time.Hour
	case "1d":
		return 24 * time.Hour
	case "3d":
		return 3 * 24 * time.Hour
	case "1w":
		return 7 * 24 * time.Hour
	case "1M":
		return 30 * 24 * time.Hour
	default:
		return 5 * time.Minute
	}
}

// ExtractBaseAsset extracts the base asset from a trading pair
// Examples:
//   - "BTCUSDT" -> "BTC"
//   - "ETHUSDT" -> "ETH"
//   - "DOGEUSDT" -> "DOGE"
func ExtractBaseAsset(symbol string) string {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))

	// Remove USDT suffix
	if strings.HasSuffix(symbol, "USDT") {
		return strings.TrimSuffix(symbol, "USDT")
	}

	// Remove BUSD suffix (legacy)
	if strings.HasSuffix(symbol, "BUSD") {
		return strings.TrimSuffix(symbol, "BUSD")
	}

	return symbol
}

// GetKlinesFromBinance is a package-level convenience function to fetch klines from Binance
// This provides a simple API similar to other providers like coinank_api.Kline()
func GetKlinesFromBinance(ctx context.Context, symbol, interval string, limit int) ([]Kline, error) {
	// Validate symbol - no commas, spaces, or special characters
	if strings.Contains(symbol, ",") || strings.Contains(symbol, " ") {
		return nil, fmt.Errorf("invalid symbol format '%s': symbol cannot contain commas or spaces (use single symbol like 'BTCUSDT')", symbol)
	}

	client := NewClient()
	klines, err := client.GetFuturesKlines(ctx, symbol, interval, limit)
	if err != nil {
		// Add symbol to error message for debugging
		return nil, fmt.Errorf("failed to get klines for symbol '%s': %w", symbol, err)
	}
	return klines, nil
}

// GetKlinesFromBinanceSpot fetches klines from Binance spot market
func GetKlinesFromBinanceSpot(ctx context.Context, symbol, interval string, limit int) ([]Kline, error) {
	// Validate symbol - no commas, spaces, or special characters
	if strings.Contains(symbol, ",") || strings.Contains(symbol, " ") {
		return nil, fmt.Errorf("invalid symbol format '%s': symbol cannot contain commas or spaces (use single symbol like 'BTCUSDT')", symbol)
	}

	client := NewClient()
	klines, err := client.GetSpotKlines(ctx, symbol, interval, limit)
	if err != nil {
		// Add symbol to error message for debugging
		return nil, fmt.Errorf("failed to get spot klines for symbol '%s': %w", symbol, err)
	}
	return klines, nil
}
