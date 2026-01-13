package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// StrategyStore strategy storage
type StrategyStore struct {
	db *sql.DB
}

// Strategy strategy configuration
type Strategy struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`  // whether it is active (a user can only have one active strategy)
	IsDefault   bool      `json:"is_default"` // whether it is a system default strategy
	Config      string    `json:"config"`     // strategy configuration in JSON format
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// StrategyConfig strategy configuration details (JSON structure)
type StrategyConfig struct {
	// coin source configuration
	CoinSource CoinSourceConfig `json:"coin_source"`
	// quantitative data configuration
	Indicators IndicatorConfig `json:"indicators"`
	// custom prompt (appended at the end)
	CustomPrompt string `json:"custom_prompt,omitempty"`
	// risk control configuration
	RiskControl RiskControlConfig `json:"risk_control"`
	// editable sections of System Prompt
	PromptSections PromptSectionsConfig `json:"prompt_sections,omitempty"`
}

// PromptSectionsConfig editable sections of System Prompt
type PromptSectionsConfig struct {
	// role definition (title + description)
	RoleDefinition string `json:"role_definition,omitempty"`
	// trading frequency awareness
	TradingFrequency string `json:"trading_frequency,omitempty"`
	// entry standards
	EntryStandards string `json:"entry_standards,omitempty"`
	// decision process
	DecisionProcess string `json:"decision_process,omitempty"`
}

// CoinSourceConfig coin source configuration
type CoinSourceConfig struct {
	// source type: "static" | "coinpool" | "oi_top" | "mixed"
	SourceType string `json:"source_type"`
	// static coin list (used when source_type = "static")
	StaticCoins []string `json:"static_coins,omitempty"`
	// whether to use AI500 coin pool
	UseCoinPool bool `json:"use_coin_pool"`
	// AI500 coin pool maximum count
	CoinPoolLimit int `json:"coin_pool_limit,omitempty"`
	// AI500 coin pool API URL (strategy-level configuration)
	CoinPoolAPIURL string `json:"coin_pool_api_url,omitempty"`
	// whether to use OI Top
	UseOITop bool `json:"use_oi_top"`
	// OI Top maximum count
	OITopLimit int `json:"oi_top_limit,omitempty"`
	// OI Top API URL (strategy-level configuration)
	OITopAPIURL string `json:"oi_top_api_url,omitempty"`
}

// IndicatorConfig indicator configuration
type IndicatorConfig struct {
	// K-line configuration
	Klines KlineConfig `json:"klines"`
	// raw kline data (OHLCV) - always enabled, required for AI analysis
	EnableRawKlines bool `json:"enable_raw_klines"`
	// technical indicator switches
	EnableEMA         bool `json:"enable_ema"`
	EnableMACD        bool `json:"enable_macd"`
	EnableRSI         bool `json:"enable_rsi"`
	EnableATR         bool `json:"enable_atr"`
	EnableBOLL        bool `json:"enable_boll"` // Bollinger Bands
	EnableVolume      bool `json:"enable_volume"`
	EnableOI          bool `json:"enable_oi"`           // open interest
	EnableFundingRate bool `json:"enable_funding_rate"` // funding rate
	// EMA period configuration
	EMAPeriods []int `json:"ema_periods,omitempty"` // default [20, 50]
	// RSI period configuration
	RSIPeriods []int `json:"rsi_periods,omitempty"` // default [7, 14]
	// ATR period configuration
	ATRPeriods []int `json:"atr_periods,omitempty"` // default [14]
	// MACD period configuration (fast, slow)
	MACDFastPeriod int `json:"macd_fast_period,omitempty"` // default 12
	MACDSlowPeriod int `json:"macd_slow_period,omitempty"` // default 26
	// BOLL period configuration (period, standard deviation multiplier is fixed at 2)
	BOLLPeriods []int `json:"boll_periods,omitempty"` // default [20] - can select multiple timeframes
	// external data sources
	ExternalDataSources []ExternalDataSource `json:"external_data_sources,omitempty"`
	// quantitative data sources (capital flow, position changes, price changes)
	EnableQuantData    bool   `json:"enable_quant_data"`            // whether to enable quantitative data
	QuantDataAPIURL    string `json:"quant_data_api_url,omitempty"` // quantitative data API address
	EnableQuantOI      bool   `json:"enable_quant_oi"`              // whether to show OI data
	EnableQuantNetflow bool   `json:"enable_quant_netflow"`         // whether to show Netflow data
	// OI ranking data (market-wide open interest increase/decrease rankings)
	EnableOIRanking   bool   `json:"enable_oi_ranking"`             // whether to enable OI ranking data
	OIRankingAPIURL   string `json:"oi_ranking_api_url,omitempty"`  // OI ranking API base URL
	OIRankingDuration string `json:"oi_ranking_duration,omitempty"` // duration: 1h, 4h, 24h
	OIRankingLimit    int    `json:"oi_ranking_limit,omitempty"`    // number of entries (default 10)
}

// KlineConfig K-line configuration
type KlineConfig struct {
	// primary timeframe: "1m", "3m", "5m", "15m", "1h", "4h"
	PrimaryTimeframe string `json:"primary_timeframe"`
	// primary timeframe K-line count
	PrimaryCount int `json:"primary_count"`
	// longer timeframe
	LongerTimeframe string `json:"longer_timeframe,omitempty"`
	// longer timeframe K-line count
	LongerCount int `json:"longer_count,omitempty"`
	// whether to enable multi-timeframe analysis
	EnableMultiTimeframe bool `json:"enable_multi_timeframe"`
	// selected timeframe list (new: supports multi-timeframe selection)
	SelectedTimeframes []string `json:"selected_timeframes,omitempty"`
}

// ExternalDataSource external data source configuration
type ExternalDataSource struct {
	Name        string            `json:"name"`   // data source name
	Type        string            `json:"type"`   // type: "api" | "webhook"
	URL         string            `json:"url"`    // API URL
	Method      string            `json:"method"` // HTTP method
	Headers     map[string]string `json:"headers,omitempty"`
	DataPath    string            `json:"data_path,omitempty"`    // JSON data path
	RefreshSecs int               `json:"refresh_secs,omitempty"` // refresh interval (seconds)
}

// RiskControlConfig risk control configuration
// All parameters are clearly defined without ambiguity:
//
// Position Limits:
//   - MaxPositions: max number of coins held simultaneously (CODE ENFORCED)
//
// Trading Leverage (exchange leverage for opening positions):
//   - BTCETHMaxLeverage: BTC/ETH max exchange leverage (AI guided)
//   - AltcoinMaxLeverage: Altcoin max exchange leverage (AI guided)
//
// Position Value Limits (single position notional value / account equity):
//   - BTCETHMaxPositionValueRatio: BTC/ETH max = equity × ratio (CODE ENFORCED)
//   - AltcoinMaxPositionValueRatio: Altcoin max = equity × ratio (CODE ENFORCED)
//
// Risk Controls:
//   - MaxMarginUsage: max margin utilization percentage (CODE ENFORCED)
//   - MinPositionSize: minimum position size in USDT (CODE ENFORCED)
//   - MinRiskRewardRatio: min take_profit / stop_loss ratio (AI guided)
//   - MinConfidence: min AI confidence to open position (AI guided)
type RiskControlConfig struct {
	// Max number of coins held simultaneously (CODE ENFORCED)
	MaxPositions int `json:"max_positions"`

	// BTC/ETH exchange leverage for opening positions (AI guided)
	BTCETHMaxLeverage int `json:"btc_eth_max_leverage"`
	// Altcoin exchange leverage for opening positions (AI guided)
	AltcoinMaxLeverage int `json:"altcoin_max_leverage"`

	// BTC/ETH single position max value = equity × this ratio (CODE ENFORCED, default: 5)
	BTCETHMaxPositionValueRatio float64 `json:"btc_eth_max_position_value_ratio"`
	// Altcoin single position max value = equity × this ratio (CODE ENFORCED, default: 1)
	AltcoinMaxPositionValueRatio float64 `json:"altcoin_max_position_value_ratio"`

	// Max margin utilization (e.g. 0.9 = 90%) (CODE ENFORCED)
	MaxMarginUsage float64 `json:"max_margin_usage"`
	// Min position size in USDT (CODE ENFORCED)
	MinPositionSize float64 `json:"min_position_size"`

	// Min take_profit / stop_loss ratio (AI guided)
	MinRiskRewardRatio float64 `json:"min_risk_reward_ratio"`
	// Min AI confidence to open position (AI guided)
	MinConfidence int `json:"min_confidence"`

	// Drawdown monitoring configuration (CODE ENFORCED - trailing stop for profit protection)
	DrawdownMonitoringEnabled bool    `json:"drawdown_monitoring_enabled"` // Enable/disable drawdown monitoring (default: true)
	DrawdownCheckInterval     int     `json:"drawdown_check_interval"`     // Check interval in seconds (default: 60, min: 15, max: 300)
	MinProfitThreshold        float64 `json:"min_profit_threshold"`        // Minimum profit % to start monitoring (default: 5.0)
	DrawdownCloseThreshold    float64 `json:"drawdown_close_threshold"`    // Drawdown % from peak to trigger close (default: 40.0, e.g. peak 10% -> 6% triggers close)
}

func (s *StrategyStore) initTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS strategies (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			is_active BOOLEAN DEFAULT 0,
			is_default BOOLEAN DEFAULT 0,
			config TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// create indexes
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_strategies_user_id ON strategies(user_id)`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_strategies_is_active ON strategies(is_active)`)

	// trigger: automatically update updated_at on update
	_, err = s.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_strategies_updated_at
		AFTER UPDATE ON strategies
		BEGIN
			UPDATE strategies SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END
	`)

	return err
}

func (s *StrategyStore) initDefaultData() error {
	// No longer pre-populate strategies - create on demand when user configures
	return nil
}

// GetDefaultStrategyConfig returns the default strategy configuration for the given language
func GetDefaultStrategyConfig(lang string) StrategyConfig {
	config := StrategyConfig{
		CoinSource: CoinSourceConfig{
			SourceType:     "coinpool",
			UseCoinPool:    true,
			CoinPoolLimit:  10,
			CoinPoolAPIURL: "http://nofxaios.com:30006/api/ai500/list?auth=cm_568c67eae410d912c54c",
			UseOITop:       false,
			OITopLimit:     20,
			OITopAPIURL:    "http://nofxaios.com:30006/api/oi/top-ranking?limit=20&duration=1h&auth=cm_568c67eae410d912c54c",
		},
		Indicators: IndicatorConfig{
			Klines: KlineConfig{
				PrimaryTimeframe:     "5m",
				PrimaryCount:         30,
				LongerTimeframe:      "4h",
				LongerCount:          10,
				EnableMultiTimeframe: true,
				SelectedTimeframes:   []string{"5m", "15m", "1h", "4h"},
			},
			EnableRawKlines:    true, // Required - raw OHLCV data for AI analysis
			EnableEMA:          false,
			EnableMACD:         false,
			EnableRSI:          false,
			EnableATR:          false,
			EnableBOLL:         false,
			EnableVolume:       true,
			EnableOI:           true,
			EnableFundingRate:  true,
			EMAPeriods:         []int{20, 50},
			RSIPeriods:         []int{7, 14},
			ATRPeriods:         []int{14},
			MACDFastPeriod:     12, // default MACD fast period
			MACDSlowPeriod:     26, // default MACD slow period
			BOLLPeriods:        []int{20},
			EnableQuantData:    true,
			QuantDataAPIURL:    "http://nofxaios.com:30006/api/coin/{symbol}?include=netflow,oi,price&auth=cm_568c67eae410d912c54c",
			EnableQuantOI:      true,
			EnableQuantNetflow: true,
			// OI ranking data - market-wide OI increase/decrease rankings
			EnableOIRanking:   true,
			OIRankingAPIURL:   "http://nofxaios.com:30006",
			OIRankingDuration: "1h",
			OIRankingLimit:    10,
		},
		RiskControl: RiskControlConfig{
			MaxPositions:                 3,   // Max 3 coins simultaneously (CODE ENFORCED)
			BTCETHMaxLeverage:            5,   // BTC/ETH exchange leverage (AI guided)
			AltcoinMaxLeverage:           5,   // Altcoin exchange leverage (AI guided)
			BTCETHMaxPositionValueRatio:  5.0, // BTC/ETH: max position = 5x equity (CODE ENFORCED)
			AltcoinMaxPositionValueRatio: 1.0, // Altcoin: max position = 1x equity (CODE ENFORCED)
			MaxMarginUsage:               0.9, // Max 90% margin usage (CODE ENFORCED)
			MinPositionSize:              12,  // Min 12 USDT per position (CODE ENFORCED)
			MinRiskRewardRatio:           3.0, // Min 3:1 profit/loss ratio (AI guided)
			MinConfidence:                75,  // Min 75% confidence (AI guided)
			// Drawdown monitoring defaults (CODE ENFORCED - automatic profit protection)
			DrawdownMonitoringEnabled: true, // Enable drawdown monitoring by default
			DrawdownCheckInterval:     60,   // Check every 60 seconds (1 minute)
			MinProfitThreshold:        5.0,  // Start monitoring when profit > 5%
			DrawdownCloseThreshold:    40.0, // Close when profit drops 40% from peak (e.g. 10% -> 6%)
		},
	}

	if lang == "zh" {
		config.PromptSections = PromptSectionsConfig{
			RoleDefinition: `# 你是一个专业的加密货币交易AI

你的任务是根据提供的市场数据做出交易决策。你是一个经验丰富的量化交易员，擅长技术分析和风险管理。`,
			TradingFrequency: `# ⏱️ 交易频率意识

- 优秀交易员：每天2-4笔 ≈ 每小时0.1-0.2笔
- 每小时超过2笔 = 过度交易
- 单笔持仓时间 ≥ 30-60分钟
如果你发现自己每个周期都在交易 → 标准太低；如果持仓不到30分钟就平仓 → 太冲动。`,
			EntryStandards: `# 🎯 入场标准（严格）

只在多个信号共振时入场。自由使用任何有效的分析方法，避免单一指标、信号矛盾、横盘震荡、或平仓后立即重新开仓等低质量行为。`,
			DecisionProcess: `# 📋 决策流程

1. 检查持仓 → 是否止盈/止损
2. 扫描候选币种 + 多时间框架 → 是否存在强信号
3. 先写思维链，再输出结构化JSON`,
		}
	} else {
		config.PromptSections = PromptSectionsConfig{
			RoleDefinition: `# You are a professional cryptocurrency trading AI

Your task is to make trading decisions based on the provided market data. You are an experienced quantitative trader skilled in technical analysis and risk management.`,
			TradingFrequency: `# ⏱️ Trading Frequency Awareness

- Excellent trader: 2-4 trades per day ≈ 0.1-0.2 trades per hour
- >2 trades per hour = overtrading
- Single position holding time ≥ 30-60 minutes
If you find yourself trading every cycle → standards are too low; if closing positions in <30 minutes → too impulsive.`,
			EntryStandards: `# 🎯 Entry Standards (Strict)

Only enter positions when multiple signals resonate. Freely use any effective analysis methods, avoid low-quality behaviors such as single indicators, contradictory signals, sideways oscillation, or immediately restarting after closing positions.`,
			DecisionProcess: `# 📋 Decision Process

1. Check positions → whether to take profit/stop loss
2. Scan candidate coins + multi-timeframe → whether strong signals exist
3. Write chain of thought first, then output structured JSON`,
		}
	}

	return config
}

// Create create a strategy
func (s *StrategyStore) Create(strategy *Strategy) error {
	_, err := s.db.Exec(`
		INSERT INTO strategies (id, user_id, name, description, is_active, is_default, config)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, strategy.ID, strategy.UserID, strategy.Name, strategy.Description, strategy.IsActive, strategy.IsDefault, strategy.Config)
	return err
}

// Update update a strategy
func (s *StrategyStore) Update(strategy *Strategy) error {
	_, err := s.db.Exec(`
		UPDATE strategies SET
			name = ?, description = ?, config = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND user_id = ?
	`, strategy.Name, strategy.Description, strategy.Config, strategy.ID, strategy.UserID)
	return err
}

// Delete delete a strategy
func (s *StrategyStore) Delete(userID, id string) error {
	// do not allow deleting system default strategy
	var isDefault bool
	s.db.QueryRow(`SELECT is_default FROM strategies WHERE id = ?`, id).Scan(&isDefault)
	if isDefault {
		return fmt.Errorf("cannot delete system default strategy")
	}

	_, err := s.db.Exec(`DELETE FROM strategies WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// List get user's strategy list
func (s *StrategyStore) List(userID string) ([]*Strategy, error) {
	// get user's own strategies + system default strategy
	rows, err := s.db.Query(`
		SELECT id, user_id, name, description, is_active, is_default, config, created_at, updated_at
		FROM strategies
		WHERE user_id = ? OR is_default = 1
		ORDER BY is_default DESC, created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var strategies []*Strategy
	for rows.Next() {
		var st Strategy
		var createdAt, updatedAt string
		err := rows.Scan(
			&st.ID, &st.UserID, &st.Name, &st.Description,
			&st.IsActive, &st.IsDefault, &st.Config,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		st.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		st.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		strategies = append(strategies, &st)
	}
	return strategies, nil
}

// Get get a single strategy
func (s *StrategyStore) Get(userID, id string) (*Strategy, error) {
	var st Strategy
	var createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT id, user_id, name, description, is_active, is_default, config, created_at, updated_at
		FROM strategies
		WHERE id = ? AND (user_id = ? OR is_default = 1)
	`, id, userID).Scan(
		&st.ID, &st.UserID, &st.Name, &st.Description,
		&st.IsActive, &st.IsDefault, &st.Config,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	st.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	st.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &st, nil
}

// GetActive get user's currently active strategy
func (s *StrategyStore) GetActive(userID string) (*Strategy, error) {
	var st Strategy
	var createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT id, user_id, name, description, is_active, is_default, config, created_at, updated_at
		FROM strategies
		WHERE user_id = ? AND is_active = 1
	`, userID).Scan(
		&st.ID, &st.UserID, &st.Name, &st.Description,
		&st.IsActive, &st.IsDefault, &st.Config,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		// no active strategy, return system default strategy
		return s.GetDefault()
	}
	if err != nil {
		return nil, err
	}
	st.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	st.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &st, nil
}

// GetDefault get system default strategy
func (s *StrategyStore) GetDefault() (*Strategy, error) {
	var st Strategy
	var createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT id, user_id, name, description, is_active, is_default, config, created_at, updated_at
		FROM strategies
		WHERE is_default = 1
		LIMIT 1
	`).Scan(
		&st.ID, &st.UserID, &st.Name, &st.Description,
		&st.IsActive, &st.IsDefault, &st.Config,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	st.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	st.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &st, nil
}

// SetActive set active strategy (will first deactivate other strategies)
func (s *StrategyStore) SetActive(userID, strategyID string) error {
	// begin transaction
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// first deactivate all strategies for the user
	_, err = tx.Exec(`UPDATE strategies SET is_active = 0 WHERE user_id = ?`, userID)
	if err != nil {
		return err
	}

	// activate specified strategy
	_, err = tx.Exec(`UPDATE strategies SET is_active = 1 WHERE id = ? AND (user_id = ? OR is_default = 1)`, strategyID, userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Duplicate duplicate a strategy (used to create custom strategy based on default strategy)
func (s *StrategyStore) Duplicate(userID, sourceID, newID, newName string) error {
	// get source strategy
	source, err := s.Get(userID, sourceID)
	if err != nil {
		return fmt.Errorf("failed to get source strategy: %w", err)
	}

	// create new strategy
	newStrategy := &Strategy{
		ID:          newID,
		UserID:      userID,
		Name:        newName,
		Description: "Created based on [" + source.Name + "]",
		IsActive:    false,
		IsDefault:   false,
		Config:      source.Config,
	}

	return s.Create(newStrategy)
}

// ParseConfig parse strategy configuration JSON
func (s *Strategy) ParseConfig() (*StrategyConfig, error) {
	var config StrategyConfig
	if err := json.Unmarshal([]byte(s.Config), &config); err != nil {
		return nil, fmt.Errorf("failed to parse strategy configuration: %w", err)
	}
	return &config, nil
}

// SetConfig set strategy configuration
func (s *Strategy) SetConfig(config *StrategyConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to serialize strategy configuration: %w", err)
	}
	s.Config = string(data)
	return nil
}
