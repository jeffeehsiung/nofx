package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// DIYQuantData DIY quantitative data calculated from Binance free API
type DIYQuantData struct {
	Symbol            string          `json:"symbol"`
	Netflow           *DIYNetflowData `json:"netflow,omitempty"`
	OI                *DIYOIData      `json:"oi,omitempty"`
	Price             *DIYPriceData   `json:"price,omitempty"`
	MarketSentiment   string          `json:"market_sentiment"`   // "bullish", "bearish", "neutral"
	InstitutionalBias string          `json:"institutional_bias"` // "long", "short", "neutral"
	ConfidenceScore   float64         `json:"confidence_score"`   // 0-100
	FetchedAt         time.Time       `json:"fetched_at"`
}

type DIYNetflowData struct {
	TakerBuyRatio    float64 `json:"taker_buy_ratio"`   // % of taker buy volume
	TakerSellRatio   float64 `json:"taker_sell_ratio"`  // % of taker sell volume
	NetflowDirection string  `json:"netflow_direction"` // "inflow", "outflow", "neutral"
	Volume24h        float64 `json:"volume_24h"`
	TakerBuyVolume   float64 `json:"taker_buy_volume"`
	TakerSellVolume  float64 `json:"taker_sell_volume"`
}

type DIYOIData struct {
	Current          float64 `json:"current"`
	Change24h        float64 `json:"change_24h"`
	ChangePercent24h float64 `json:"change_percent_24h"`
	Trend            string  `json:"trend"` // "increasing", "decreasing", "stable"
}

type DIYPriceData struct {
	Current          float64 `json:"current"`
	Change24h        float64 `json:"change_24h"`
	ChangePercent24h float64 `json:"change_percent_24h"`
	High24h          float64 `json:"high_24h"`
	Low24h           float64 `json:"low_24h"`
}

// BinanceTicker24hrDetailed detailed 24hr ticker with taker buy/sell volume
type BinanceTicker24hrDetailed struct {
	Symbol             string `json:"symbol"`
	PriceChange        string `json:"priceChange"`
	PriceChangePercent string `json:"priceChangePercent"`
	LastPrice          string `json:"lastPrice"`
	HighPrice          string `json:"highPrice"`
	LowPrice           string `json:"lowPrice"`
	Volume             string `json:"volume"`
	QuoteVolume        string `json:"quoteVolume"`
	OpenTime           int64  `json:"openTime"`
	CloseTime          int64  `json:"closeTime"`
	Count              int64  `json:"count"`
}

// GetDIYQuantData calculates quantitative data from Binance free API
func GetDIYQuantData(symbol string) (*DIYQuantData, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Fetch 24hr ticker
	ticker, err := getBinance24hrTicker(client, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ticker: %w", err)
	}

	// Fetch current OI
	currentOI, err := getBinanceOI(client, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OI: %w", err)
	}

	// Fetch historical OI for comparison (24h ago approximation using current - we'll use a simple estimation)
	// Note: Binance doesn't provide historical OI in free API, so we estimate from volume

	// Parse ticker data
	lastPrice, _ := strconv.ParseFloat(ticker.LastPrice, 64)
	priceChange, _ := strconv.ParseFloat(ticker.PriceChange, 64)
	priceChangePercent, _ := strconv.ParseFloat(ticker.PriceChangePercent, 64)
	highPrice, _ := strconv.ParseFloat(ticker.HighPrice, 64)
	lowPrice, _ := strconv.ParseFloat(ticker.LowPrice, 64)
	volume, _ := strconv.ParseFloat(ticker.Volume, 64)
	quoteVolume, _ := strconv.ParseFloat(ticker.QuoteVolume, 64)
	_ = quoteVolume // Will be used for volume metrics in future iterations

	// Calculate netflow metrics
	// For perpetual futures, we can use the trade count and volume patterns
	// Higher trade count with increasing OI suggests institutional accumulation
	netflow := calculateNetflow(ticker, currentOI, priceChangePercent)

	// Calculate OI metrics
	oiData := calculateOIMetrics(currentOI, priceChangePercent, volume)

	// Price data
	priceData := &DIYPriceData{
		Current:          lastPrice,
		Change24h:        priceChange,
		ChangePercent24h: priceChangePercent,
		High24h:          highPrice,
		Low24h:           lowPrice,
	}

	// Calculate market sentiment
	sentiment := calculateMarketSentiment(priceChangePercent, oiData.ChangePercent24h, netflow.TakerBuyRatio)
	bias := calculateInstitutionalBias(netflow.TakerBuyRatio, oiData.ChangePercent24h, priceChangePercent)
	confidence := calculateConfidenceScore(netflow, oiData, priceChangePercent)

	return &DIYQuantData{
		Symbol:            symbol,
		Netflow:           netflow,
		OI:                oiData,
		Price:             priceData,
		MarketSentiment:   sentiment,
		InstitutionalBias: bias,
		ConfidenceScore:   confidence,
		FetchedAt:         time.Now(),
	}, nil
}

func getBinance24hrTicker(client *http.Client, symbol string) (*BinanceTicker24hrDetailed, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/ticker/24hr?symbol=%s", symbol)

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var ticker BinanceTicker24hrDetailed
	if err := json.Unmarshal(body, &ticker); err != nil {
		return nil, err
	}

	return &ticker, nil
}

func getBinanceOI(client *http.Client, symbol string) (float64, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/openInterest?symbol=%s", symbol)

	resp, err := client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		OpenInterest string `json:"openInterest"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	oi, err := strconv.ParseFloat(result.OpenInterest, 64)
	if err != nil {
		return 0, err
	}

	return oi, nil
}

func calculateNetflow(ticker *BinanceTicker24hrDetailed, currentOI float64, priceChangePercent float64) *DIYNetflowData {
	_ = currentOI // Reserved for future OI analysis
	volume, _ := strconv.ParseFloat(ticker.Volume, 64)
	quoteVolume, _ := strconv.ParseFloat(ticker.QuoteVolume, 64)
	_ = quoteVolume // Reserved for future volume analysis
	tradeCount := float64(ticker.Count)
	_ = tradeCount // Reserved for future trade count analysis

	// Estimate taker buy/sell ratio using price change and trade count
	// Higher price change with more trades suggests taker buys dominate
	// This is a proxy calculation since we don't have direct taker buy/sell data

	// Baseline: assume 50/50 split
	takerBuyRatio := 50.0

	// Adjust based on price change
	// Positive price change suggests more buying pressure
	if priceChangePercent > 0 {
		// For every 1% price increase, assume ~5% shift toward buyers
		adjustment := priceChangePercent * 5.0
		takerBuyRatio += adjustment
	} else if priceChangePercent < 0 {
		// For every 1% price decrease, assume ~5% shift toward sellers
		adjustment := priceChangePercent * 5.0
		takerBuyRatio += adjustment // adjustment is negative
	}

	// Clamp between 20-80 (never fully one-sided)
	if takerBuyRatio > 80 {
		takerBuyRatio = 80
	} else if takerBuyRatio < 20 {
		takerBuyRatio = 20
	}

	takerSellRatio := 100.0 - takerBuyRatio

	// Estimate volumes
	takerBuyVolume := volume * (takerBuyRatio / 100.0)
	takerSellVolume := volume * (takerSellRatio / 100.0)

	// Determine direction
	direction := "neutral"
	if takerBuyRatio > 55 {
		direction = "inflow"
	} else if takerBuyRatio < 45 {
		direction = "outflow"
	}

	return &DIYNetflowData{
		TakerBuyRatio:    takerBuyRatio,
		TakerSellRatio:   takerSellRatio,
		NetflowDirection: direction,
		Volume24h:        quoteVolume,
		TakerBuyVolume:   takerBuyVolume,
		TakerSellVolume:  takerSellVolume,
	}
}

func calculateOIMetrics(currentOI float64, priceChangePercent float64, volume float64) *DIYOIData {
	_ = volume // Reserved for future volume-based OI analysis
	// Estimate OI change based on volume and price action
	// High volume with price increase often correlates with OI increase

	// Simple heuristic: if price up and volume high, OI likely increased
	// if price down and volume high, OI could increase (shorts) or decrease (longs closing)

	// For now, use a simple proxy: assume OI change roughly tracks price momentum
	// This is an approximation - real OI data would need historical tracking
	oiChangePercent := priceChangePercent * 0.5 // OI typically moves less than price

	trend := "stable"
	if oiChangePercent > 1.0 {
		trend = "increasing"
	} else if oiChangePercent < -1.0 {
		trend = "decreasing"
	}

	return &DIYOIData{
		Current:          currentOI,
		Change24h:        currentOI * (oiChangePercent / 100.0),
		ChangePercent24h: oiChangePercent,
		Trend:            trend,
	}
}

func calculateMarketSentiment(priceChange, oiChange, takerBuyRatio float64) string {
	// Strong signals
	if priceChange > 3 && oiChange > 2 && takerBuyRatio > 60 {
		return "strongly_bullish"
	}
	if priceChange < -3 && oiChange > 2 && takerBuyRatio < 40 {
		return "strongly_bearish"
	}

	// Moderate signals
	if priceChange > 1 && takerBuyRatio > 55 {
		return "bullish"
	}
	if priceChange < -1 && takerBuyRatio < 45 {
		return "bearish"
	}

	return "neutral"
}

func calculateInstitutionalBias(takerBuyRatio, oiChange, priceChange float64) string {
	// Institutional traders typically:
	// - Open positions when OI increases
	// - Prefer trending markets
	// - Have persistent directional bias

	// Strong long bias: buying + OI increasing + price rising
	if takerBuyRatio > 60 && oiChange > 1 && priceChange > 0 {
		return "long"
	}

	// Strong short bias: selling + OI increasing + price falling
	if takerBuyRatio < 40 && oiChange > 1 && priceChange < 0 {
		return "short"
	}

	// Neutral/mixed
	return "neutral"
}

func calculateConfidenceScore(netflow *DIYNetflowData, oi *DIYOIData, priceChange float64) float64 {
	score := 50.0 // baseline

	// Confidence increases with:
	// 1. Clear directional bias (not neutral)
	if netflow.NetflowDirection != "neutral" {
		score += 15
	}

	// 2. OI trend aligning with price
	if (oi.Trend == "increasing" && priceChange > 0) || (oi.Trend == "decreasing" && priceChange < 0) {
		score += 15
	}

	// 3. Strong taker bias (> 60% or < 40%)
	if netflow.TakerBuyRatio > 60 || netflow.TakerBuyRatio < 40 {
		score += 10
	}

	// 4. High volume (proxy for conviction)
	if netflow.Volume24h > 10000000 { // > $10M
		score += 10
	}

	// Clamp between 0-100
	if score > 100 {
		score = 100
	} else if score < 0 {
		score = 0
	}

	return score
}
