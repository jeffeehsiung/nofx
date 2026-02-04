package provider

import (
	"testing"
)

func TestBinanceFallback(t *testing.T) {
	// Test GetTopCoinsByVolume
	t.Run("GetTopCoinsByVolume", func(t *testing.T) {
		symbols, err := GetTopCoinsByVolume(10)
		if err != nil {
			t.Fatalf("GetTopCoinsByVolume failed: %v", err)
		}
		if len(symbols) == 0 {
			t.Error("Expected non-empty symbol list")
		}
		t.Logf("✓ Got %d coins by volume: %v", len(symbols), symbols)
	})

	// Test GetTopCoinsByPriceChange
	t.Run("GetTopCoinsByPriceChange", func(t *testing.T) {
		symbols, err := GetTopCoinsByPriceChange(10)
		if err != nil {
			t.Fatalf("GetTopCoinsByPriceChange failed: %v", err)
		}
		if len(symbols) == 0 {
			t.Error("Expected non-empty symbol list")
		}
		t.Logf("✓ Got %d coins by price change: %v", len(symbols), symbols)
	})

	// Test GetTopCoinsWithFallback (without external API configured)
	t.Run("GetTopCoinsWithFallback", func(t *testing.T) {
		// Clear external API to force fallback
		SetAI500API("")

		symbols, source, err := GetTopCoinsWithFallback(10, true)
		if err != nil {
			t.Fatalf("GetTopCoinsWithFallback failed: %v", err)
		}
		if len(symbols) == 0 {
			t.Error("Expected non-empty symbol list")
		}
		if source != "binance_volume" {
			t.Errorf("Expected source 'binance_volume', got '%s'", source)
		}
		t.Logf("✓ Got %d coins from source '%s': %v", len(symbols), source, symbols)
	})

	// Test GetOITopSymbolsWithFallback (without external API configured)
	t.Run("GetOITopSymbolsWithFallback", func(t *testing.T) {
		// Clear external API to force fallback
		SetOITopAPI("")

		symbols, source, err := GetOITopSymbolsWithFallback(10, true)
		if err != nil {
			t.Fatalf("GetOITopSymbolsWithFallback failed: %v", err)
		}
		if len(symbols) == 0 {
			t.Error("Expected non-empty symbol list")
		}
		if source != "binance_momentum" {
			t.Errorf("Expected source 'binance_momentum', got '%s'", source)
		}
		t.Logf("✓ Got %d coins from source '%s': %v", len(symbols), source, symbols)
	})
}
