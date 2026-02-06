package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"nofx/provider/coinglass"
	"nofx/security"
	"os"
	"strconv"
	"strings"
	"time"
)

// AI500Config AI500 data provider configuration
type AI500Config struct {
	APIURL  string
	Timeout time.Duration
}

var ai500Config = AI500Config{
	APIURL:  "",
	Timeout: 30 * time.Second,
}

// CoinData coin information
type CoinData struct {
	Pair            string  `json:"pair"`             // Trading pair symbol (e.g.: BTCUSDT)
	Score           float64 `json:"score"`            // Current score
	StartTime       int64   `json:"start_time"`       // Start time (Unix timestamp)
	StartPrice      float64 `json:"start_price"`      // Start price
	LastScore       float64 `json:"last_score"`       // Latest score
	MaxScore        float64 `json:"max_score"`        // Highest score
	MaxPrice        float64 `json:"max_price"`        // Highest price
	IncreasePercent float64 `json:"increase_percent"` // Increase percentage
	IsAvailable     bool    `json:"-"`                // Whether tradable (internal use)
}

// AI500APIResponse raw data structure returned by AI500 API
type AI500APIResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Coins []CoinData `json:"coins"`
		Count int        `json:"count"`
	} `json:"data"`
}

// SetAI500API sets AI500 data provider API
func SetAI500API(apiURL string) {
	// Migrate old URL to new base
	if strings.Contains(apiURL, "nofxaios.com:30006") {
		apiURL = strings.Replace(apiURL, "http://nofxaios.com:30006", "https://nofxos.ai", 1)
		log.Printf("🔄 Migrated AI500 API URL to new base: https://nofxos.ai")
	}
	ai500Config.APIURL = apiURL
}

// SetOITopAPI sets OI Top API
func SetOITopAPI(apiURL string) {
	// Migrate old URL to new base
	if strings.Contains(apiURL, "nofxaios.com:30006") {
		apiURL = strings.Replace(apiURL, "http://nofxaios.com:30006", "https://nofxos.ai", 1)
		log.Printf("🔄 Migrated OI Top API URL to new base: https://nofxos.ai")
	}
	oiTopConfig.APIURL = apiURL
}

// GetAI500Data retrieves AI500 coin list (with retry mechanism)
func GetAI500Data() ([]CoinData, error) {
	// Check if API URL is configured
	if strings.TrimSpace(ai500Config.APIURL) == "" {
		return nil, fmt.Errorf("AI500 API URL not configured")
	}

	maxRetries := 3
	var lastErr error

	// Try to fetch from API
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			log.Printf("⚠️  Retry attempt %d of %d to fetch AI500 data...", attempt, maxRetries)
			time.Sleep(2 * time.Second)
		}

		coins, err := fetchAI500()
		if err == nil {
			if attempt > 1 {
				log.Printf("✓ Retry attempt %d succeeded", attempt)
			}
			return coins, nil
		}

		lastErr = err
		log.Printf("❌ Request attempt %d failed: %v", attempt, err)
	}

	return nil, fmt.Errorf("all API requests failed: %w", lastErr)
}

// fetchAI500 actually executes AI500 request
func fetchAI500() ([]CoinData, error) {
	log.Printf("🔄 Requesting AI500 data...")

	// SSRF Protection: Validate URL before making request
	resp, err := security.SafeGet(ai500Config.APIURL, ai500Config.Timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to request AI500 API: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse API response
	var response AI500APIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("JSON parsing failed: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("API returned failure status")
	}

	if len(response.Data.Coins) == 0 {
		return nil, fmt.Errorf("coin list is empty")
	}

	// Set IsAvailable flag
	coins := response.Data.Coins
	for i := range coins {
		coins[i].IsAvailable = true
	}

	log.Printf("✓ Successfully fetched %d coins", len(coins))
	return coins, nil
}

// GetAvailableCoins retrieves available coin list (filters out unavailable ones)
func GetAvailableCoins() ([]string, error) {
	coins, err := GetAI500Data()
	if err != nil {
		return nil, err
	}

	var symbols []string
	for _, coin := range coins {
		if coin.IsAvailable {
			symbol := normalizeSymbol(coin.Pair)
			symbols = append(symbols, symbol)
		}
	}

	if len(symbols) == 0 {
		return nil, fmt.Errorf("no available coins")
	}

	return symbols, nil
}

// GetTopRatedCoins retrieves top N coins by score (sorted by score descending)
func GetTopRatedCoins(limit int) ([]string, error) {
	coins, err := GetAI500Data()
	if err != nil {
		return nil, err
	}

	// Filter available coins
	var availableCoins []CoinData
	for _, coin := range coins {
		if coin.IsAvailable {
			availableCoins = append(availableCoins, coin)
		}
	}

	if len(availableCoins) == 0 {
		return nil, fmt.Errorf("no available coins")
	}

	// Sort by Score descending (bubble sort)
	for i := 0; i < len(availableCoins); i++ {
		for j := i + 1; j < len(availableCoins); j++ {
			if availableCoins[i].Score < availableCoins[j].Score {
				availableCoins[i], availableCoins[j] = availableCoins[j], availableCoins[i]
			}
		}
	}

	// Take top N
	maxCount := limit
	if len(availableCoins) < maxCount {
		maxCount = len(availableCoins)
	}

	var symbols []string
	for i := 0; i < maxCount; i++ {
		symbol := normalizeSymbol(availableCoins[i].Pair)
		symbols = append(symbols, symbol)
	}

	return symbols, nil
}

// normalizeSymbol normalizes coin symbol
func normalizeSymbol(symbol string) string {
	symbol = trimSpaces(symbol)
	symbol = toUpper(symbol)
	if !endsWith(symbol, "USDT") {
		symbol = symbol + "USDT"
	}
	return symbol
}

// Helper functions
func trimSpaces(s string) string {
	result := ""
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' {
			result += string(s[i])
		}
	}
	return result
}

func toUpper(s string) string {
	result := ""
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c = c - 'a' + 'A'
		}
		result += string(c)
	}
	return result
}

func endsWith(s, suffix string) bool {
	if len(s) < len(suffix) {
		return false
	}
	return s[len(s)-len(suffix):] == suffix
}

// ========== OI Top (Open Interest Growth Top 20) Data ==========

// OIPosition open interest data
type OIPosition struct {
	Symbol            string  `json:"symbol"`
	Rank              int     `json:"rank"`
	CurrentOI         float64 `json:"current_oi"`
	OIDelta           float64 `json:"oi_delta"`
	OIDeltaPercent    float64 `json:"oi_delta_percent"`
	OIDeltaValue      float64 `json:"oi_delta_value"`
	PriceDeltaPercent float64 `json:"price_delta_percent"`
	NetLong           float64 `json:"net_long"`
	NetShort          float64 `json:"net_short"`
}

// OITopAPIResponse data structure returned by OI Top API
type OITopAPIResponse struct {
	Code int `json:"code"`
	Data struct {
		Positions      []OIPosition `json:"positions"`
		Count          int          `json:"count"`
		Exchange       string       `json:"exchange"`
		TimeRange      string       `json:"time_range"`
		TimeRangeParam string       `json:"time_range_param"`
		RankType       string       `json:"rank_type"`
		Limit          int          `json:"limit"`
	} `json:"data"`
}

var oiTopConfig = struct {
	APIURL  string
	Timeout time.Duration
}{
	APIURL:  "",
	Timeout: 30 * time.Second,
}

// GetOITopPositions retrieves OI Top 20 data (with retry)
func GetOITopPositions() ([]OIPosition, error) {
	if strings.TrimSpace(oiTopConfig.APIURL) == "" {
		log.Printf("⚠️  OI Top API URL not configured, skipping OI Top data fetch")
		return []OIPosition{}, nil
	}

	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			log.Printf("⚠️  Retry attempt %d of %d to fetch OI Top data...", attempt, maxRetries)
			time.Sleep(2 * time.Second)
		}

		positions, err := fetchOITop()
		if err == nil {
			if attempt > 1 {
				log.Printf("✓ Retry attempt %d succeeded", attempt)
			}
			return positions, nil
		}

		lastErr = err
		log.Printf("❌ OI Top request attempt %d failed: %v", attempt, err)
	}

	log.Printf("⚠️  All OI Top API requests failed (last error: %v), skipping OI Top data", lastErr)
	return []OIPosition{}, nil
}

// fetchOITop actually executes OI Top request
func fetchOITop() ([]OIPosition, error) {
	log.Printf("🔄 Requesting OI Top data...")

	// SSRF Protection: Validate URL before making request
	resp, err := security.SafeGet(oiTopConfig.APIURL, oiTopConfig.Timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to request OI Top API: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read OI Top response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OI Top API returned error (status %d): %s", resp.StatusCode, string(body))
	}

	// Try new format first (https://nofxos.ai uses "success": true)
	var newResponse struct {
		Success bool `json:"success"`
		Data    struct {
			Positions      []OIPosition `json:"positions"`
			Count          int          `json:"count"`
			Exchange       string       `json:"exchange"`
			TimeRange      string       `json:"time_range"`
			TimeRangeParam string       `json:"time_range_param"`
			RankType       string       `json:"rank_type"`
			Limit          int          `json:"limit"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &newResponse); err == nil && newResponse.Success {
		if len(newResponse.Data.Positions) == 0 {
			return nil, fmt.Errorf("OI Top position list is empty")
		}
	}

	log.Printf("✓ Successfully fetched %d OI Top coins (time range: %s, type: %s)",
		len(newResponse.Data.Positions), newResponse.Data.TimeRange, newResponse.Data.RankType)
	return newResponse.Data.Positions, nil
}

// GetOITopSymbols retrieves OI Top coin symbol list
func GetOITopSymbols() ([]string, error) {
	positions, err := GetOITopPositions()
	if err != nil {
		return nil, err
	}

	var symbols []string
	for _, pos := range positions {
		symbol := normalizeSymbol(pos.Symbol)
		symbols = append(symbols, symbol)
	}

	return symbols, nil
}

// MergedData merged data (AI500 + OI Top)
type MergedData struct {
	AI500Coins    []CoinData
	OITopCoins    []OIPosition
	AllSymbols    []string
	SymbolSources map[string][]string
}

// OIRankingData OI ranking data for debate (includes both top and low)
type OIRankingData struct {
	TimeRange    string       `json:"time_range"`
	Duration     string       `json:"duration"`
	TopPositions []OIPosition `json:"top_positions"`
	LowPositions []OIPosition `json:"low_positions"`
	FetchedAt    time.Time    `json:"fetched_at"`
}

// GetOIRankingData retrieves OI ranking data (both top increase and low decrease)
func GetOIRankingData(baseURL, authKey string, duration string, limit int) (*OIRankingData, error) {
	if baseURL == "" || authKey == "" {
		return nil, fmt.Errorf("OI API URL or auth key not configured")
	}

	if duration == "" {
		duration = "1h"
	}
	if limit <= 0 {
		limit = 20
	}

	// Migrate legacy base URL if needed
	if strings.Contains(baseURL, "nofxaios.com:30006") {
		baseURL = strings.Replace(baseURL, "http://nofxaios.com:30006", "https://nofxos.ai", 1)
		log.Printf("🔄 Migrated OI ranking base URL to new base: https://nofxos.ai")
	}

	result := &OIRankingData{
		Duration:  duration,
		FetchedAt: time.Now(),
	}

	// Fetch top ranking
	topURL := fmt.Sprintf("%s/api/oi/top-ranking?limit=%d&duration=%s&auth=%s", baseURL, limit, duration, authKey)
	topPositions, timeRange, err := fetchOIRanking(topURL)
	if err != nil {
		log.Printf("⚠️  Failed to fetch OI top ranking: %v", err)
	} else {
		result.TopPositions = topPositions
		result.TimeRange = timeRange
	}

	// Fetch low ranking
	lowURL := fmt.Sprintf("%s/api/oi/low-ranking?limit=%d&duration=%s&auth=%s", baseURL, limit, duration, authKey)
	lowPositions, _, err := fetchOIRanking(lowURL)
	if err != nil {
		log.Printf("⚠️  Failed to fetch OI low ranking: %v", err)
	} else {
		result.LowPositions = lowPositions
	}

	log.Printf("✓ Fetched OI ranking data: %d top, %d low (duration: %s)",
		len(result.TopPositions), len(result.LowPositions), duration)

	return result, nil
}

// fetchOIRanking fetches OI ranking from a single endpoint
func fetchOIRanking(url string) ([]OIPosition, string, error) {
	// SSRF Protection: Validate URL before making request
	resp, err := security.SafeGet(url, 30*time.Second)
	if err != nil {
		return nil, "", fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("API returned error (status %d): %s", resp.StatusCode, string(body))
	}

	var response OITopAPIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, "", fmt.Errorf("JSON parsing failed: %w", err)
	}

	if response.Code != 0 {
		return nil, "", fmt.Errorf("API returned error code: %d", response.Code)
	}

	return response.Data.Positions, response.Data.TimeRange, nil
}

// GetMergedData retrieves merged data (AI500 + OI Top, deduplicated)
func GetMergedData(ai500Limit int) (*MergedData, error) {
	ai500TopSymbols, err := GetTopRatedCoins(ai500Limit)
	if err != nil {
		log.Printf("⚠️  Failed to get AI500 data: %v", err)
		ai500TopSymbols = []string{}
	}

	oiTopSymbols, err := GetOITopSymbols()
	if err != nil {
		log.Printf("⚠️  Failed to get OI Top data: %v", err)
		oiTopSymbols = []string{}
	}

	symbolSet := make(map[string]bool)
	symbolSources := make(map[string][]string)

	for _, symbol := range ai500TopSymbols {
		symbolSet[symbol] = true
		symbolSources[symbol] = append(symbolSources[symbol], "ai500")
	}

	for _, symbol := range oiTopSymbols {
		if !symbolSet[symbol] {
			symbolSet[symbol] = true
		}
		symbolSources[symbol] = append(symbolSources[symbol], "oi_top")
	}

	var allSymbols []string
	for symbol := range symbolSet {
		allSymbols = append(allSymbols, symbol)
	}

	ai500Coins, _ := GetAI500Data()
	oiTopPositions, _ := GetOITopPositions()

	merged := &MergedData{
		AI500Coins:    ai500Coins,
		OITopCoins:    oiTopPositions,
		AllSymbols:    allSymbols,
		SymbolSources: symbolSources,
	}

	log.Printf("📊 Data merge complete: AI500=%d, OI_Top=%d, Total(deduplicated)=%d",
		len(ai500TopSymbols), len(oiTopSymbols), len(allSymbols))

	return merged, nil
}

// ========== Binance Free API Fallback ==========

// BinanceTicker24hr represents Binance 24hr ticker statistics
type BinanceTicker24hr struct {
	Symbol             string `json:"symbol"`
	PriceChange        string `json:"priceChange"`
	PriceChangePercent string `json:"priceChangePercent"`
	WeightedAvgPrice   string `json:"weightedAvgPrice"`
	LastPrice          string `json:"lastPrice"`
	Volume             string `json:"volume"`
	QuoteVolume        string `json:"quoteVolume"`
	OpenTime           int64  `json:"openTime"`
	CloseTime          int64  `json:"closeTime"`
	Count              int64  `json:"count"`
}

// BinanceOITicker represents Binance open interest ticker
type BinanceOITicker struct {
	Symbol       string `json:"symbol"`
	OpenInterest string `json:"openInterest"`
	Time         int64  `json:"time"`
}

// GetBinance24hrTickers retrieves all 24hr ticker stats from Binance (free API)
func GetBinance24hrTickers() ([]BinanceTicker24hr, error) {
	url := "https://fapi.binance.com/fapi/v1/ticker/24hr"

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("binance ticker request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read ticker response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance API error (status %d): %s", resp.StatusCode, string(body))
	}

	var tickers []BinanceTicker24hr
	if err := json.Unmarshal(body, &tickers); err != nil {
		return nil, fmt.Errorf("ticker JSON parsing failed: %w", err)
	}

	return tickers, nil
}

// GetBinanceOITickers retrieves all open interest data from Binance (free API)
func GetBinanceOITickers() ([]BinanceOITicker, error) {
	url := "https://fapi.binance.com/fapi/v1/openInterest"

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("binance OI request failed: %w", err)
	}
	defer resp.Body.Close()

	// The endpoint returns single OI for a symbol if symbol param provided,
	// but without symbol it returns all symbols
	// Actually, Binance doesn't have a batch endpoint, so we need to call ticker/24hr
	// and then call individual OI for each symbol. For simplicity, we'll use volume-based ranking.
	// Let's return empty and rely on volume ranking instead.
	return nil, fmt.Errorf("binance OI batch endpoint not available, use volume-based ranking")
}

// GetTopCoinsByVolume retrieves top N coins by 24hr volume from Binance (free fallback)
func GetTopCoinsByVolume(limit int) ([]string, error) {
	log.Printf("🔄 Fetching top coins by volume from Binance free API (fallback)...")

	tickers, err := GetBinance24hrTickers()
	if err != nil {
		return nil, err
	}

	// Filter USDT perpetual futures only
	var usdtTickers []BinanceTicker24hr
	for _, ticker := range tickers {
		if endsWith(ticker.Symbol, "USDT") {
			usdtTickers = append(usdtTickers, ticker)
		}
	}

	// Parse and sort by quote volume (descending)
	type volumeEntry struct {
		symbol string
		volume float64
	}
	var entries []volumeEntry

	for _, ticker := range usdtTickers {
		volume := 0.0
		if ticker.QuoteVolume != "" {
			parsed, err := strconv.ParseFloat(ticker.QuoteVolume, 64)
			if err == nil {
				volume = parsed
			}
		}
		entries = append(entries, volumeEntry{
			symbol: ticker.Symbol,
			volume: volume,
		})
	}

	// Sort by volume descending (bubble sort)
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].volume < entries[j].volume {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// Take top N
	maxCount := limit
	if len(entries) < maxCount {
		maxCount = len(entries)
	}

	var symbols []string
	for i := 0; i < maxCount; i++ {
		symbols = append(symbols, entries[i].symbol)
	}

	log.Printf("✓ Binance fallback: fetched top %d coins by volume", len(symbols))
	return symbols, nil
}

// GetTopCoinsByPriceChange retrieves top N coins by 24hr price change % from Binance (free fallback)
func GetTopCoinsByPriceChange(limit int) ([]string, error) {
	log.Printf("🔄 Fetching top coins by price change from Binance free API (fallback)...")

	tickers, err := GetBinance24hrTickers()
	if err != nil {
		return nil, err
	}

	// Filter USDT perpetual futures only
	var usdtTickers []BinanceTicker24hr
	for _, ticker := range tickers {
		if endsWith(ticker.Symbol, "USDT") {
			usdtTickers = append(usdtTickers, ticker)
		}
	}

	// Parse and sort by price change % (descending)
	type changeEntry struct {
		symbol        string
		volume        float64
		changePercent float64
	}
	var entries []changeEntry

	for _, ticker := range usdtTickers {
		changePercent := 0.0
		if ticker.PriceChangePercent != "" {
			parsed, err := strconv.ParseFloat(ticker.PriceChangePercent, 64)
			if err == nil {
				changePercent = parsed
			}
		}

		volume := 0.0
		if ticker.QuoteVolume != "" {
			parsed, err := strconv.ParseFloat(ticker.QuoteVolume, 64)
			if err == nil {
				volume = parsed
			}
		}

		// Filter: require positive change and reasonable volume (> $1M)
		if changePercent > 0 && volume > 1000000 {
			entries = append(entries, changeEntry{
				symbol:        ticker.Symbol,
				changePercent: changePercent,
				volume:        volume,
			})
		}
	}

	// Sort by price change % descending (bubble sort)
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].changePercent < entries[j].changePercent {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// Take top N
	maxCount := limit
	if len(entries) < maxCount {
		maxCount = len(entries)
	}

	var symbols []string
	for i := 0; i < maxCount; i++ {
		symbols = append(symbols, entries[i].symbol)
	}

	log.Printf("✓ Binance fallback: fetched top %d coins by price change (> 0%% with >$1M volume)", len(symbols))
	return symbols, nil
}

// GetTopCoinsWithFallback retrieves top coins with automatic fallback
// Priority: 1) External API (if configured), 2) Binance volume-based ranking
func GetTopCoinsWithFallback(limit int, useExternalFirst bool) ([]string, string, error) {
	// Try external API first if requested and configured
	if useExternalFirst && strings.TrimSpace(ai500Config.APIURL) != "" {
		symbols, err := GetTopRatedCoins(limit)
		if err == nil && len(symbols) > 0 {
			log.Printf("✓ Using external AI500 API data (%d coins)", len(symbols))
			return symbols, "external_api", nil
		}
		log.Printf("⚠️  External API failed or empty: %v, falling back to Binance", err)
	}

	// Fallback: Binance volume-based ranking
	symbols, err := GetTopCoinsByVolume(limit)
	if err != nil {
		return nil, "", fmt.Errorf("both external API and Binance fallback failed: %w", err)
	}

	return symbols, "binance_volume", nil
}

// GetOITopSymbolsWithFallback retrieves OI top symbols with automatic fallback
// Priority: 1) External OI API, 2) Binance price change % ranking
func GetOITopSymbolsWithFallback(limit int, useExternalFirst bool) ([]string, string, error) {
	// Try external OI Top API first if requested and configured
	if useExternalFirst && strings.TrimSpace(oiTopConfig.APIURL) != "" {
		symbols, err := GetOITopSymbols()
		if err == nil && len(symbols) > 0 {
			log.Printf("✓ Using external OI Top API data (%d coins)", len(symbols))
			return symbols, "external_oi_api", nil
		}
		log.Printf("⚠️  External OI API failed or empty: %v, falling back to Binance", err)
	}

	// Fallback: Binance price change % ranking (momentum proxy)
	symbols, err := GetTopCoinsByPriceChange(limit)
	if err != nil {
		return nil, "", fmt.Errorf("both external OI API and Binance fallback failed: %w", err)
	}

	return symbols, "binance_momentum", nil
}

// GetOIRankingFromCoinGlass retrieves OI ranking from CoinGlass free API
func GetOIRankingFromCoinGlass(duration string, limit int) (*OIRankingData, error) {
	apiKey := strings.TrimSpace(os.Getenv("COINGLASS_API_KEY"))
	client := coinglass.NewClient()
	if apiKey != "" {
		client = coinglass.NewClientWithAPIKey(apiKey)
	}
	positions, err := client.GetTopOISymbols(duration, limit)
	if err != nil {
		return nil, fmt.Errorf("CoinGlass fetch failed: %w", err)
	}

	// Convert CoinGlass positions to OIPosition format
	oiPositions := make([]OIPosition, 0, len(positions))
	for i, pos := range positions {
		oiPos := OIPosition{
			Symbol:            pos.Symbol,
			Rank:              i + 1,
			CurrentOI:         pos.OpenInterestUsd,
			OIDelta:           pos.Change,
			OIDeltaPercent:    pos.ChangePercent,
			OIDeltaValue:      pos.Change,
			PriceDeltaPercent: pos.PriceChange,
			NetLong:           pos.TakerLongRatio * 100,  // Convert ratio to percentage
			NetShort:          pos.TakerShortRatio * 100, // Convert ratio to percentage
		}
		oiPositions = append(oiPositions, oiPos)
	}

	return &OIRankingData{
		Duration:     duration,
		TopPositions: oiPositions,
		FetchedAt:    time.Now(),
	}, nil
}

// ========== Backward Compatibility Aliases ==========

// Deprecated: Use SetAI500API instead
func SetCoinPoolAPI(apiURL string) {
	SetAI500API(apiURL)
}

// Deprecated: Use GetAI500Data instead
func GetCoinPool() ([]CoinData, error) {
	return GetAI500Data()
}

// Deprecated: Use MergedData instead
type MergedCoinPool = MergedData

// Deprecated: Use GetMergedCoinPool instead
func GetMergedCoinPool(ai500Limit int) (*MergedData, error) {
	return GetMergedData(ai500Limit)
}

// Deprecated: Use CoinData instead
type CoinInfo = CoinData
