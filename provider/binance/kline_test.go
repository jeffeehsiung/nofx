package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestGetKlines_BTC_Futures(t *testing.T) {
	client := NewClient()

	klines, err := client.GetFuturesKlines(context.TODO(), "BTCUSDT", "1d", 5)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("=== BTC Futures Daily Klines (Binance) ===")
	for i, k := range klines {
		openTime := time.UnixMilli(k.OpenTime).Format("2006-01-02 15:04:05")
		t.Logf("\n[%d] Time: %s", i, openTime)
		t.Logf("    Symbol:   %s", k.Symbol)
		t.Logf("    Interval: %s", k.Interval)
		t.Logf("    Open:     %.2f", k.Open)
		t.Logf("    High:     %.2f", k.High)
		t.Logf("    Low:      %.2f", k.Low)
		t.Logf("    Close:    %.2f", k.Close)
		t.Logf("    Volume:   %.4f", k.Volume)
	}

	// Print raw JSON
	res, _ := json.MarshalIndent(klines, "", "  ")
	fmt.Printf("\nRaw JSON:\n%s\n", res)
}

func TestGetKlines_ETH_Spot(t *testing.T) {
	client := NewClient()

	klines, err := client.GetSpotKlines(context.TODO(), "ETHUSDT", "1h", 10)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("=== ETH Spot Hourly Klines (Binance) ===")
	for i, k := range klines {
		openTime := time.UnixMilli(k.OpenTime).Format("2006-01-02 15:04:05")
		t.Logf("\n[%d] Time: %s", i, openTime)
		t.Logf("    Symbol:   %s", k.Symbol)
		t.Logf("    Interval: %s", k.Interval)
		t.Logf("    Open:     %.2f", k.Open)
		t.Logf("    High:     %.2f", k.High)
		t.Logf("    Low:      %.2f", k.Low)
		t.Logf("    Close:    %.2f", k.Close)
		t.Logf("    Volume:   %.4f", k.Volume)
	}
}

func TestGetKlines_MultipleSymbols(t *testing.T) {
	client := NewClient()

	symbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "DOGEUSDT"}

	for _, symbol := range symbols {
		t.Logf("\n=== %s Futures 15m Klines ===", symbol)
		klines, err := client.GetFuturesKlines(context.TODO(), symbol, "15m", 3)
		if err != nil {
			t.Errorf("%s fetch failed: %v", symbol, err)
			continue
		}

		if len(klines) == 0 {
			t.Logf("%s: No data", symbol)
			continue
		}

		latest := klines[len(klines)-1]
		openTime := time.UnixMilli(latest.OpenTime).Format("2006-01-02 15:04")
		t.Logf("%s Latest: %s Open=%.4f High=%.4f Low=%.4f Close=%.4f Vol=%.2f",
			symbol, openTime, latest.Open, latest.High, latest.Low, latest.Close, latest.Volume)
	}
}

func TestGetKlines_DifferentIntervals(t *testing.T) {
	client := NewClient()
	symbol := "BTCUSDT"

	intervals := []string{"1m", "5m", "15m", "1h", "4h", "1d"}

	for _, interval := range intervals {
		t.Logf("\n=== %s %s Klines ===", symbol, interval)
		klines, err := client.GetFuturesKlines(context.TODO(), symbol, interval, 3)
		if err != nil {
			t.Errorf("%s %s fetch failed: %v", symbol, interval, err)
			continue
		}

		if len(klines) == 0 {
			t.Logf("%s %s: No data", symbol, interval)
			continue
		}

		latest := klines[len(klines)-1]
		openTime := time.UnixMilli(latest.OpenTime).Format("2006-01-02 15:04:05")
		t.Logf("%s %s Latest: %s Close=%.2f Vol=%.4f",
			symbol, interval, openTime, latest.Close, latest.Volume)
	}
}

func TestNormalizeSymbol(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"BTC", "BTCUSDT"},
		{"BTCUSDT", "BTCUSDT"},
		{"btcusdt", "BTCUSDT"},
		{"ETH", "ETHUSDT"},
		{"eth", "ETHUSDT"},
		{"ETHUSDT", "ETHUSDT"},
		{"DOGE", "DOGEUSDT"},
		{"SOL", "SOLUSDT"},
	}

	for _, tt := range tests {
		result := NormalizeSymbol(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeSymbol(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestMapInterval(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1m", "1m"},
		{"3m", "3m"},
		{"5m", "5m"},
		{"15m", "15m"},
		{"1h", "1h"},
		{"4h", "4h"},
		{"1d", "1d"},
		{"1w", "1w"},
		{"1M", "1M"},
		{"1mo", "1M"},
		{"invalid", "5m"}, // Default fallback
	}

	for _, tt := range tests {
		result := MapInterval(tt.input)
		if result != tt.expected {
			t.Errorf("MapInterval(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestExtractBaseAsset(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"BTCUSDT", "BTC"},
		{"ETHUSDT", "ETH"},
		{"DOGEUSDT", "DOGE"},
		{"SOLUSDT", "SOL"},
		{"BTC", "BTC"},
		{"btcusdt", "BTC"},
	}

	for _, tt := range tests {
		result := ExtractBaseAsset(tt.input)
		if result != tt.expected {
			t.Errorf("ExtractBaseAsset(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestGetIntervalDuration(t *testing.T) {
	tests := []struct {
		interval string
		expected time.Duration
	}{
		{"1m", time.Minute},
		{"5m", 5 * time.Minute},
		{"15m", 15 * time.Minute},
		{"1h", time.Hour},
		{"4h", 4 * time.Hour},
		{"1d", 24 * time.Hour},
		{"1w", 7 * 24 * time.Hour},
	}

	for _, tt := range tests {
		result := GetIntervalDuration(tt.interval)
		if result != tt.expected {
			t.Errorf("GetIntervalDuration(%s) = %v, expected %v", tt.interval, result, tt.expected)
		}
	}
}

func TestGetKlines_ContextCancellation(t *testing.T) {
	client := NewClient()

	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait a bit to ensure timeout
	time.Sleep(10 * time.Millisecond)

	_, err := client.GetFuturesKlines(ctx, "BTCUSDT", "1m", 10)
	if err == nil {
		t.Error("Expected context cancellation error, got nil")
	}
	t.Logf("Expected error (context cancelled): %v", err)
}

func TestGetKlines_InvalidSymbol(t *testing.T) {
	client := NewClient()

	// Try to fetch klines for invalid symbol
	_, err := client.GetFuturesKlines(context.TODO(), "INVALIDXYZ", "1m", 10)
	if err == nil {
		t.Error("Expected error for invalid symbol, got nil")
	}
	t.Logf("Expected error (invalid symbol): %v", err)
}

func TestGetKlines_LargeLimit(t *testing.T) {
	client := NewClient()

	// Test with large limit (should be capped at 1500)
	klines, err := client.GetFuturesKlines(context.TODO(), "BTCUSDT", "1m", 2000)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Requested 2000 klines, received %d klines (should be capped at 1500)", len(klines))

	if len(klines) > 1500 {
		t.Errorf("Expected max 1500 klines, got %d", len(klines))
	}
}
