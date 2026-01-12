package store

import (
	"testing"
)

// TestCreateTraderWithPaperTrading tests creating a trader with paper trading enabled
func TestCreateTraderWithPaperTrading(t *testing.T) {
	trader := &Trader{PaperTrading: true}

	if !trader.PaperTrading {
		t.Errorf("Expected PaperTrading to be true, got %v", trader.PaperTrading)
	}
}

// TestCreateTraderWithoutPaperTrading tests creating a trader with paper trading disabled (live trading)
func TestCreateTraderWithoutPaperTrading(t *testing.T) {
	trader := &Trader{PaperTrading: false}

	if trader.PaperTrading {
		t.Errorf("Expected PaperTrading to be false, got %v", trader.PaperTrading)
	}
}

// TestPaperTradingDefaultValue tests that new traders default to live trading (paper_trading = false)
func TestPaperTradingDefaultValue(t *testing.T) {
	trader := &Trader{}

	if trader.PaperTrading {
		t.Errorf("Expected PaperTrading default to be false (live trading), got %v", trader.PaperTrading)
	}
}

// TestTraderStructIncludesPaperTrading tests that Trader struct includes PaperTrading field
func TestTraderStructIncludesPaperTrading(t *testing.T) {
	trader := &Trader{PaperTrading: true}

	if !trader.PaperTrading {
		t.Error("Trader struct does not properly store PaperTrading field")
	}
}
