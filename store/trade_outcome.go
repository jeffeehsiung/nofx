package store

import (
	"database/sql"
	"fmt"
	"time"
)

// TradeOutcome represents a completed trade with metrics for failure analysis
type TradeOutcome struct {
	Symbol            string
	Profitable        bool
	VolumeAtEntry     float64
	OIAtEntry         float64
	VolumeDuringTrade float64
	OIDuringTrade     float64
	EntrySpread       float64
	ExitSpread        float64
	EntryDepth        float64
	ExitDepth         float64
	HoldingMinutes    int
	PnLPct            float64
}

// TradeOutcomeStore manages trade outcome data for failure analysis
type TradeOutcomeStore struct {
	db *sql.DB
}

// NewTradeOutcomeStore creates a new trade outcome store
func NewTradeOutcomeStore(db *sql.DB) *TradeOutcomeStore {
	return &TradeOutcomeStore{db: db}
}

// InitTables initializes trade outcome tables
func (t *TradeOutcomeStore) InitTables() error {
	if _, err := t.db.Exec(`
		CREATE TABLE IF NOT EXISTS trade_outcomes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			symbol TEXT NOT NULL,
			profitable BOOLEAN NOT NULL,
			volume_at_entry REAL,
			oi_at_entry REAL,
			volume_during_trade REAL,
			oi_during_trade REAL,
			entry_spread REAL,
			exit_spread REAL,
			entry_depth REAL,
			exit_depth REAL,
			holding_minutes INTEGER,
			pnl_pct REAL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return err
	}

	// Add indexes separately for SQLite compatibility.
	if _, err := t.db.Exec(`CREATE INDEX IF NOT EXISTS idx_trade_outcomes_symbol ON trade_outcomes(symbol)`); err != nil {
		return err
	}
	if _, err := t.db.Exec(`CREATE INDEX IF NOT EXISTS idx_trade_outcomes_created_at ON trade_outcomes(created_at)`); err != nil {
		return err
	}

	return nil
}

// Save persists a trade outcome to the database
func (t *TradeOutcomeStore) Save(outcome *TradeOutcome) error {
	_, err := t.db.Exec(`
		INSERT INTO trade_outcomes (
			symbol, profitable, volume_at_entry, oi_at_entry,
			volume_during_trade, oi_during_trade, entry_spread, exit_spread,
			entry_depth, exit_depth, holding_minutes, pnl_pct
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		outcome.Symbol,
		outcome.Profitable,
		outcome.VolumeAtEntry,
		outcome.OIAtEntry,
		outcome.VolumeDuringTrade,
		outcome.OIDuringTrade,
		outcome.EntrySpread,
		outcome.ExitSpread,
		outcome.EntryDepth,
		outcome.ExitDepth,
		outcome.HoldingMinutes,
		outcome.PnLPct,
	)
	return err
}

// GetRecent retrieves the most recent trade outcomes (for calibration)
func (t *TradeOutcomeStore) GetRecent(limit int) ([]TradeOutcome, error) {
	rows, err := t.db.Query(`
		SELECT symbol, profitable, volume_at_entry, oi_at_entry,
		       volume_during_trade, oi_during_trade, entry_spread, exit_spread,
		       entry_depth, exit_depth, holding_minutes, pnl_pct
		FROM trade_outcomes
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var outcomes []TradeOutcome
	for rows.Next() {
		var outcome TradeOutcome
		err := rows.Scan(
			&outcome.Symbol,
			&outcome.Profitable,
			&outcome.VolumeAtEntry,
			&outcome.OIAtEntry,
			&outcome.VolumeDuringTrade,
			&outcome.OIDuringTrade,
			&outcome.EntrySpread,
			&outcome.ExitSpread,
			&outcome.EntryDepth,
			&outcome.ExitDepth,
			&outcome.HoldingMinutes,
			&outcome.PnLPct,
		)
		if err != nil {
			return nil, err
		}
		outcomes = append(outcomes, outcome)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return outcomes, nil
}

// GetSince retrieves trade outcomes since a specific time (for periodic recalibration)
func (t *TradeOutcomeStore) GetSince(since time.Time) ([]TradeOutcome, error) {
	rows, err := t.db.Query(`
		SELECT symbol, profitable, volume_at_entry, oi_at_entry,
		       volume_during_trade, oi_during_trade, entry_spread, exit_spread,
		       entry_depth, exit_depth, holding_minutes, pnl_pct
		FROM trade_outcomes
		WHERE created_at >= ?
		ORDER BY created_at DESC
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var outcomes []TradeOutcome
	for rows.Next() {
		var outcome TradeOutcome
		err := rows.Scan(
			&outcome.Symbol,
			&outcome.Profitable,
			&outcome.VolumeAtEntry,
			&outcome.OIAtEntry,
			&outcome.VolumeDuringTrade,
			&outcome.OIDuringTrade,
			&outcome.EntrySpread,
			&outcome.ExitSpread,
			&outcome.EntryDepth,
			&outcome.ExitDepth,
			&outcome.HoldingMinutes,
			&outcome.PnLPct,
		)
		if err != nil {
			return nil, err
		}
		outcomes = append(outcomes, outcome)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return outcomes, nil
}

// GetLosingTrades retrieves unprofitable trades for failure analysis
func (t *TradeOutcomeStore) GetLosingTrades(limit int) ([]TradeOutcome, error) {
	rows, err := t.db.Query(`
		SELECT symbol, profitable, volume_at_entry, oi_at_entry,
		       volume_during_trade, oi_during_trade, entry_spread, exit_spread,
		       entry_depth, exit_depth, holding_minutes, pnl_pct
		FROM trade_outcomes
		WHERE profitable = false
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var outcomes []TradeOutcome
	for rows.Next() {
		var outcome TradeOutcome
		err := rows.Scan(
			&outcome.Symbol,
			&outcome.Profitable,
			&outcome.VolumeAtEntry,
			&outcome.OIAtEntry,
			&outcome.VolumeDuringTrade,
			&outcome.OIDuringTrade,
			&outcome.EntrySpread,
			&outcome.ExitSpread,
			&outcome.EntryDepth,
			&outcome.ExitDepth,
			&outcome.HoldingMinutes,
			&outcome.PnLPct,
		)
		if err != nil {
			return nil, err
		}
		outcomes = append(outcomes, outcome)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return outcomes, nil
}

// GetCount returns total number of trade outcomes
func (t *TradeOutcomeStore) GetCount() (int, error) {
	var count int
	err := t.db.QueryRow(`SELECT COUNT(*) FROM trade_outcomes`).Scan(&count)
	return count, err
}

// GetCountBySymbol returns count of trades for a specific symbol
func (t *TradeOutcomeStore) GetCountBySymbol(symbol string) (int, error) {
	var count int
	err := t.db.QueryRow(`
		SELECT COUNT(*) FROM trade_outcomes WHERE symbol = ?
	`, symbol).Scan(&count)
	return count, err
}

// GetWinRate returns win rate (% of profitable trades)
func (t *TradeOutcomeStore) GetWinRate() (float64, error) {
	var totalCount, winCount int
	err := t.db.QueryRow(`
		SELECT COUNT(*), COUNT(*) FILTER (WHERE profitable = true)
		FROM trade_outcomes
	`).Scan(&totalCount, &winCount)
	if err != nil || totalCount == 0 {
		return 0, err
	}
	return float64(winCount) / float64(totalCount), nil
}

// GetAveragePnL returns average PnL percentage
func (t *TradeOutcomeStore) GetAveragePnL() (float64, error) {
	var avgPnL sql.NullFloat64
	err := t.db.QueryRow(`
		SELECT AVG(pnl_pct) FROM trade_outcomes
	`).Scan(&avgPnL)
	if err != nil {
		return 0, err
	}
	if avgPnL.Valid {
		return avgPnL.Float64, nil
	}
	return 0, nil
}

// Cleanup removes old trade outcome records (keep last N months)
func (t *TradeOutcomeStore) Cleanup(daysToKeep int) error {
	cutoffTime := time.Now().AddDate(0, 0, -daysToKeep)
	result, err := t.db.Exec(`
		DELETE FROM trade_outcomes WHERE created_at < ?
	`, cutoffTime)
	if err != nil {
		return err
	}
	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsDeleted > 0 {
		fmt.Printf("🗑️  Cleaned up %d old trade outcome records\n", rowsDeleted)
	}
	return nil
}
