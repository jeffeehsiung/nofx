package trader

import (
	"fmt"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"sort"
	"strings"
	"sync"
	"time"
)

// syncState stores the last sync time for incremental sync
var (
	binanceSyncState      = make(map[string]time.Time) // exchangeID -> lastSyncTime
	binanceSyncStateMutex sync.RWMutex
	binanceSyncFailures   = make(map[string]int)  // exchangeID -> consecutive failure count
	binanceSyncDisabled   = make(map[string]bool) // exchangeID -> fallback enabled (sync disabled)
)

const binanceSyncFailureThreshold = 3

func markBinanceSyncSuccess(exchangeID string) {
	binanceSyncStateMutex.Lock()
	binanceSyncFailures[exchangeID] = 0
	binanceSyncDisabled[exchangeID] = false
	binanceSyncStateMutex.Unlock()
}

func markBinanceSyncFailure(exchangeID string) {
	binanceSyncStateMutex.Lock()
	binanceSyncFailures[exchangeID]++
	if binanceSyncFailures[exchangeID] >= binanceSyncFailureThreshold {
		binanceSyncDisabled[exchangeID] = true
	}
	binanceSyncStateMutex.Unlock()
}

func isBinanceSyncDisabled(exchangeID string) bool {
	binanceSyncStateMutex.RLock()
	defer binanceSyncStateMutex.RUnlock()
	return binanceSyncDisabled[exchangeID]
}

func getBinanceSyncFailureCount(exchangeID string) int {
	binanceSyncStateMutex.RLock()
	defer binanceSyncStateMutex.RUnlock()
	return binanceSyncFailures[exchangeID]
}

// SyncOrdersFromBinance syncs Binance Futures trade history to local database
// Uses COMMISSION detection + fromId for efficient incremental sync
// Also creates/updates position records to ensure orders/fills/positions data consistency
func (t *FuturesTrader) SyncOrdersFromBinance(traderID string, exchangeID string, exchangeType string, st *store.Store) error {
	if st == nil {
		markBinanceSyncFailure(exchangeID)
		return fmt.Errorf("store is nil")
	}

	// Get last sync time (default to 24 hours ago for first sync)
	binanceSyncStateMutex.RLock()
	lastSyncTime, exists := binanceSyncState[exchangeID]
	binanceSyncStateMutex.RUnlock()

	if !exists {
		lastSyncTime = time.Now().Add(-24 * time.Hour)
	}

	// Record current time BEFORE querying, to avoid missing trades during sync
	// This prevents race condition where trades happen between query and lastSyncTime update
	syncStartTime := time.Now()

	logger.Infof("🔄 Syncing Binance trades from: %s", lastSyncTime.Format(time.RFC3339))

	// Step 1: Get max trade IDs from local DB for incremental sync
	orderStore := st.Order()
	maxTradeIDs, err := orderStore.GetMaxTradeIDsByExchange(exchangeID)
	if err != nil {
		logger.Infof("  ⚠️ Failed to get max trade IDs: %v, will use time-based query", err)
		maxTradeIDs = make(map[string]int64)
	}

	// Step 2: Use COMMISSION to detect which symbols have new trades (1 API call)
	changedSymbols, err := t.GetCommissionSymbols(lastSyncTime)
	if err != nil {
		logger.Infof("  ⚠️ Failed to get commission symbols: %v, falling back to positions", err)
		// Fallback: only sync symbols with active positions
		changedSymbols = t.getPositionSymbols()
	} else if len(changedSymbols) == 0 {
		logger.Infof("  ⚠️ Commission history returned no symbols, falling back to positions")
		changedSymbols = t.getPositionSymbols()
	}

	if len(changedSymbols) == 0 {
		logger.Infof("📭 No symbols with new trades to sync")
		// Update last sync time even if no changes
		binanceSyncStateMutex.Lock()
		binanceSyncState[exchangeID] = syncStartTime
		binanceSyncStateMutex.Unlock()
		markBinanceSyncSuccess(exchangeID)
		return nil
	}

	logger.Infof("📊 Found %d symbols with new trades: %v", len(changedSymbols), changedSymbols)

	// Step 3: Query trades for changed symbols using fromId (incremental) or time-based (new symbols)
	var allTrades []TradeRecord
	var failedSymbols []string
	apiCalls := 0
	for _, symbol := range changedSymbols {
		var trades []TradeRecord
		var queryErr error

		if lastID, ok := maxTradeIDs[symbol]; ok && lastID > 0 {
			// Incremental sync: query from last known trade ID
			trades, queryErr = t.GetTradesForSymbolFromID(symbol, lastID+1, 500)
		} else {
			// New symbol or first sync: query by time
			trades, queryErr = t.GetTradesForSymbol(symbol, lastSyncTime, 500)
		}
		apiCalls++

		if queryErr != nil {
			logger.Infof("  ⚠️ Failed to get trades for %s: %v", symbol, queryErr)
			failedSymbols = append(failedSymbols, symbol)
			continue
		}
		allTrades = append(allTrades, trades...)
	}

	logger.Infof("📥 Received %d trades from Binance (%d API calls)", len(allTrades), apiCalls)

	// If ALL symbols failed, mark sync failure for fallback
	if len(failedSymbols) == len(changedSymbols) {
		markBinanceSyncFailure(exchangeID)
		return fmt.Errorf("binance order sync failed for all symbols")
	}

	// Otherwise, reset failure count (partial success counts as healthy)
	markBinanceSyncSuccess(exchangeID)

	// Only update last sync time if ALL symbols were successfully queried
	// This prevents data loss when some symbols fail due to rate limit or network issues
	if len(failedSymbols) == 0 {
		binanceSyncStateMutex.Lock()
		binanceSyncState[exchangeID] = syncStartTime
		binanceSyncStateMutex.Unlock()
	} else {
		logger.Infof("  ⚠️ %d symbols failed, not updating lastSyncTime to retry next time: %v", len(failedSymbols), failedSymbols)
	}

	if len(allTrades) == 0 {
		return nil
	}

	// Sort trades by time ASC (oldest first) for proper position building
	sort.Slice(allTrades, func(i, j int) bool {
		return allTrades[i].Time.Before(allTrades[j].Time)
	})

	// Process trades one by one
	positionStore := st.Position()
	posBuilder := store.NewPositionBuilder(positionStore)
	syncedCount := 0

	for _, trade := range allTrades {
		// Check if trade already exists
		orderID := trade.TradeID
		if trade.OrderID != "" {
			orderID = trade.OrderID
		}

		existing, err := orderStore.GetOrderByExchangeID(exchangeID, orderID)
		if err == nil && existing != nil {
			// If this order already exists (immediate record), still record fill but skip position builder
			updated, updateErr := orderStore.UpdateSyntheticFillForOrder(
				exchangeID, orderID, trade.TradeID,
				trade.Price, trade.Quantity, trade.Price*trade.Quantity,
				trade.Fee, trade.RealizedPnL, trade.Time,
			)
			if updateErr != nil {
				logger.Infof("  ⚠️ Failed to update synthetic fill for order %s: %v", orderID, updateErr)
			}
			if updated {
				syncedCount++
				continue
			}

			fills, fillsErr := orderStore.GetOrderFills(existing.ID)
			if fillsErr == nil && len(fills) > 0 {
				syncedCount++
				continue
			}

			fillRecord := &store.TraderFill{
				TraderID:        traderID,
				ExchangeID:      exchangeID,
				ExchangeType:    exchangeType,
				OrderID:         existing.ID,
				ExchangeOrderID: orderID,
				ExchangeTradeID: trade.TradeID,
				Symbol:          market.Normalize(trade.Symbol),
				Side:            strings.ToUpper(trade.Side),
				Price:           trade.Price,
				Quantity:        trade.Quantity,
				QuoteQuantity:   trade.Price * trade.Quantity,
				Commission:      trade.Fee,
				CommissionAsset: "USDT",
				RealizedPnL:     trade.RealizedPnL,
				IsMaker:         false,
				CreatedAt:       trade.Time,
			}

			if err := orderStore.CreateFill(fillRecord); err != nil {
				logger.Infof("  ⚠️ Failed to sync fill for trade %s: %v", trade.TradeID, err)
			}

			syncedCount++
			continue
		}

		// Normalize symbol
		symbol := market.Normalize(trade.Symbol)

		// Determine order action based on side and position side
		orderAction := t.determineOrderAction(trade.Side, trade.PositionSide, trade.RealizedPnL)

		// Determine position side for position builder
		positionSide := trade.PositionSide
		if positionSide == "" || positionSide == "BOTH" {
			// Infer from order action
			if strings.Contains(orderAction, "long") {
				positionSide = "LONG"
			} else {
				positionSide = "SHORT"
			}
		}

		// Normalize side
		side := strings.ToUpper(trade.Side)

		// Create order record
		orderRecord := &store.TraderOrder{
			TraderID:        traderID,
			ExchangeID:      exchangeID,
			ExchangeType:    exchangeType,
			ExchangeOrderID: orderID,
			Symbol:          symbol,
			Side:            side,
			PositionSide:    positionSide,
			Type:            "MARKET",
			OrderAction:     orderAction,
			Quantity:        trade.Quantity,
			Price:           trade.Price,
			Status:          "FILLED",
			FilledQuantity:  trade.Quantity,
			AvgFillPrice:    trade.Price,
			Commission:      trade.Fee,
			FilledAt:        trade.Time,
			CreatedAt:       trade.Time,
			UpdatedAt:       trade.Time,
		}

		// Insert order record
		if err := orderStore.CreateOrder(orderRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync trade %s: %v", trade.TradeID, err)
			continue
		}

		// Create fill record
		fillRecord := &store.TraderFill{
			TraderID:        traderID,
			ExchangeID:      exchangeID,
			ExchangeType:    exchangeType,
			OrderID:         orderRecord.ID,
			ExchangeOrderID: orderID,
			ExchangeTradeID: trade.TradeID,
			Symbol:          symbol,
			Side:            side,
			Price:           trade.Price,
			Quantity:        trade.Quantity,
			QuoteQuantity:   trade.Price * trade.Quantity,
			Commission:      trade.Fee,
			CommissionAsset: "USDT",
			RealizedPnL:     trade.RealizedPnL,
			IsMaker:         false,
			CreatedAt:       trade.Time,
		}

		if err := orderStore.CreateFill(fillRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync fill for trade %s: %v", trade.TradeID, err)
		}

		// Create/update position record using PositionBuilder (skip if already recorded immediately)
		if exists, err := positionStore.HasOrderID(traderID, orderID); err != nil {
			logger.Infof("  ⚠️ Failed to check position by order ID: %v", err)
		} else if !exists {
			if err := posBuilder.ProcessTrade(
				traderID, exchangeID, exchangeType,
				symbol, positionSide, orderAction,
				trade.Quantity, trade.Price, trade.Fee, trade.RealizedPnL,
				trade.Time, trade.TradeID,
			); err != nil {
				logger.Infof("  ⚠️ Failed to sync position for trade %s: %v", trade.TradeID, err)
			} else {
				logger.Infof("  📍 Position updated for trade: %s (action: %s, qty: %.6f)", trade.TradeID, orderAction, trade.Quantity)
			}
		}

		syncedCount++
		logger.Infof("  ✅ Synced trade: %s %s %s qty=%.6f price=%.6f pnl=%.2f fee=%.6f action=%s",
			trade.TradeID, symbol, side, trade.Quantity, trade.Price, trade.RealizedPnL, trade.Fee, orderAction)
	}

	logger.Infof("✅ Binance order sync completed: %d new trades synced", syncedCount)
	return nil
}

// getPositionSymbols returns list of symbols that have active positions
// Used as fallback when COMMISSION detection fails
func (t *FuturesTrader) getPositionSymbols() []string {
	positions, err := t.GetPositions()
	if err != nil {
		return nil
	}

	var symbols []string
	for _, pos := range positions {
		if symbol, ok := pos["symbol"].(string); ok && symbol != "" {
			symbols = append(symbols, symbol)
		}
	}
	return symbols
}

// determineOrderAction determines the order action based on trade data
func (t *FuturesTrader) determineOrderAction(side, positionSide string, realizedPnL float64) string {
	side = strings.ToUpper(side)
	positionSide = strings.ToUpper(positionSide)

	// If there's realized PnL, it's likely a close trade
	isClose := realizedPnL != 0

	switch positionSide {
	case "LONG", "":
		if side == "BUY" {
			if isClose {
				return "close_short" // Buying to close short
			}
			return "open_long"
		} else {
			if isClose {
				return "close_long" // Selling to close long
			}
			return "open_short"
		}
	case "SHORT":
		if side == "SELL" {
			if isClose {
				return "close_long"
			}
			return "open_short"
		} else {
			if isClose {
				return "close_short"
			}
			return "open_long"
		}
	}

	// Default fallback
	if side == "BUY" {
		return "open_long"
	}
	return "open_short"
}

// StartOrderSync starts background order sync task for Binance
func (t *FuturesTrader) StartOrderSync(traderID string, exchangeID string, exchangeType string, st *store.Store, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if isBinanceSyncDisabled(exchangeID) {
				logger.Infof("⚠️  Binance order sync disabled (too many failures). Falling back to immediate recording.")
				return
			}
			if err := t.SyncOrdersFromBinance(traderID, exchangeID, exchangeType, st); err != nil {
				logger.Infof("⚠️  Binance order sync failed: %v", err)
			}
		}
	}()
	logger.Infof("🔄 Binance order sync started (interval: %v)", interval)
}
