package coinglass

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.baseURL != CoinGlassAPIEndpoint {
		t.Errorf("baseURL = %v, want %v", client.baseURL, CoinGlassAPIEndpoint)
	}
	if client.timeout != DefaultTimeout {
		t.Errorf("timeout = %v, want %v", client.timeout, DefaultTimeout)
	}
}

func TestNewClientWithAPIKey(t *testing.T) {
	apiKey := "test-api-key-12345"
	client := NewClientWithAPIKey(apiKey)
	if client == nil {
		t.Fatal("NewClientWithAPIKey returned nil")
	}
	if client.apiKey != apiKey {
		t.Errorf("apiKey = %v, want %v", client.apiKey, apiKey)
	}
}

func TestGetTopOISymbolsWithoutAPIKey(t *testing.T) {
	client := NewClient()
	// Without API key, should return error pointing to use fallback
	_, err := client.GetTopOISymbols("24h", 10)
	if err == nil {
		t.Fatal("Expected error when no API key provided")
	}
	if err.Error() != "CoinGlass premium API requires API key - using Binance fallback instead" {
		t.Logf("Got expected error: %v", err)
	}
}

func TestGetTopOISymbolsWithAPIKey(t *testing.T) {
	// This test requires a valid API key to run
	apiKey := "" // Set to your test API key
	if apiKey == "" {
		t.Skip("CoinGlass API key not set - skipping premium API test")
	}

	client := NewClientWithAPIKey(apiKey)
	positions, err := client.GetTopOISymbols("24h", 10)
	if err != nil {
		t.Fatalf("GetTopOISymbols failed: %v", err)
	}

	if len(positions) == 0 {
		t.Fatal("GetTopOISymbols returned empty list")
	}

	firstPos := positions[0]
	if firstPos.Symbol == "" {
		t.Error("First position has empty symbol")
	}
	if firstPos.OpenInterestUsd <= 0 {
		t.Errorf("First position has invalid OI: %v", firstPos.OpenInterestUsd)
	}

	t.Logf("Top OI Symbol: %s, OI: %.2f USD, Change: %.2f%%",
		firstPos.Symbol, firstPos.OpenInterestUsd, firstPos.ChangePercent)
}
