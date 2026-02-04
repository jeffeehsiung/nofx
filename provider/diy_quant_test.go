package provider

import (
	"testing"
)

func TestDIYQuantData(t *testing.T) {
	t.Run("GetDIYQuantData", func(t *testing.T) {
		data, err := GetDIYQuantData("BTCUSDT")
		if err != nil {
			t.Fatalf("GetDIYQuantData failed: %v", err)
		}
		if data == nil {
			t.Fatal("Expected non-nil data")
		}
		if data.Symbol != "BTCUSDT" {
			t.Errorf("Expected symbol BTCUSDT, got %s", data.Symbol)
		}
		if data.Netflow == nil {
			t.Error("Expected netflow data")
		}
		if data.OI == nil {
			t.Error("Expected OI data")
		}
		if data.Price == nil {
			t.Error("Expected price data")
		}

		t.Logf("✓ DIY Quant Data for %s:", data.Symbol)
		t.Logf("  Market Sentiment: %s", data.MarketSentiment)
		t.Logf("  Institutional Bias: %s", data.InstitutionalBias)
		t.Logf("  Confidence Score: %.1f", data.ConfidenceScore)
		if data.Netflow != nil {
			t.Logf("  Netflow: %.1f%% buy / %.1f%% sell (%s)",
				data.Netflow.TakerBuyRatio, data.Netflow.TakerSellRatio, data.Netflow.NetflowDirection)
		}
		if data.OI != nil {
			t.Logf("  OI: %.2f (change: %.2f%%)", data.OI.Current, data.OI.ChangePercent24h)
		}
		if data.Price != nil {
			t.Logf("  Price: $%.2f (24h change: %.2f%%)", data.Price.Current, data.Price.ChangePercent24h)
		}
	})
}
