package trader

import (
	"encoding/json"
	"fmt"
	"math"
	"nofx/backtest"
	"nofx/config"
	"nofx/decision"
	"nofx/experience"
	"nofx/logger"
	"nofx/market"
	"nofx/mcp"
	"nofx/store"
	"strings"
	"sync"
	"time"
)

// AutoTraderConfig auto trading configuration (simplified version - AI makes all decisions)
type AutoTraderConfig struct {
	// Trader identification
	ID      string // Trader unique identifier (for log directory, etc.)
	Name    string // Trader display name
	AIModel string // AI model: "qwen" or "deepseek"

	// Trading platform selection
	Exchange   string // Exchange type: "binance", "bybit", "okx", "bitget", "hyperliquid", "aster"
	ExchangeID string // Exchange account UUID (for multi-account support)

	// Binance API configuration
	BinanceAPIKey    string
	BinanceSecretKey string
	BinanceTestnet   bool

	// Bybit API configuration
	BybitAPIKey    string
	BybitSecretKey string
	BybitTestnet   bool

	// OKX API configuration
	OKXAPIKey     string
	OKXSecretKey  string
	OKXPassphrase string

	// Bitget API configuration
	BitgetAPIKey     string
	BitgetSecretKey  string
	BitgetPassphrase string

	// Hyperliquid configuration
	HyperliquidPrivateKey string
	HyperliquidWalletAddr string
	HyperliquidTestnet    bool

	// Aster configuration
	AsterUser       string // Aster main wallet address
	AsterSigner     string // Aster API wallet address
	AsterPrivateKey string // Aster API wallet private key

	// LIGHTER configuration
	LighterWalletAddr       string // LIGHTER wallet address (L1 wallet)
	LighterPrivateKey       string // LIGHTER L1 private key (for account identification)
	LighterAPIKeyPrivateKey string // LIGHTER API Key private key (40 bytes, for transaction signing)
	LighterAPIKeyIndex      int    // LIGHTER API Key index (0-255)
	LighterTestnet          bool   // Whether to use testnet

	// AI configuration
	UseQwen     bool
	DeepSeekKey string
	QwenKey     string

	// Custom AI API configuration
	CustomAPIURL    string
	CustomAPIKey    string
	CustomModelName string

	// Scan configuration
	ScanInterval      time.Duration // Scan interval (recommended 3 minutes)
	OrderSyncInterval time.Duration // Order sync interval (default 30 seconds, can be adjusted per exchange)

	// Account configuration
	InitialBalance float64 // Initial balance (for P&L calculation, must be set manually)

	// Risk control (only as hints, AI can make autonomous decisions)
	MaxDailyLoss    float64       // Maximum daily loss percentage (hint)
	MaxDrawdown     float64       // Maximum drawdown percentage (hint)
	StopTradingTime time.Duration // Pause duration after risk control triggers

	// Position mode
	IsCrossMargin bool // true=cross margin mode, false=isolated margin mode

	// Competition visibility
	ShowInCompetition bool // Whether to show in competition page

	// Trading mode / prompt variant
	TradingMode string // Trading mode: "" (default/balanced), "aggressive", "conservative", or prompt variant ID

	// Strategy configuration (use complete strategy config)
	StrategyConfig *store.StrategyConfig // Strategy configuration (includes coin sources, indicators, risk control, prompts, etc.)
}

// AutoTrader automatic trader
type AutoTrader struct {
	id                      string // Trader unique identifier
	name                    string // Trader display name
	aiModel                 string // AI model name
	exchange                string // Trading platform type (binance/bybit/etc)
	exchangeID              string // Exchange account UUID
	showInCompetition       bool   // Whether to show in competition page
	config                  AutoTraderConfig
	trader                  Trader // Use Trader interface (supports multiple platforms)
	mcpClient               mcp.AIClient
	store                   *store.Store             // Data storage (decision records, etc.)
	strategyEngine          *decision.StrategyEngine // Strategy engine (uses strategy configuration)
	cycleNumber             int                      // Current cycle number
	initialBalance          float64
	dailyPnL                float64
	customPrompt            string // Custom trading strategy prompt
	overrideBasePrompt      bool   // Whether to override base prompt
	lastResetTime           time.Time
	stopUntil               time.Time
	isRunning               bool
	isRunningMutex          sync.RWMutex       // Mutex to protect isRunning flag
	startTime               time.Time          // System start time
	callCount               int                // AI call count
	positionFirstSeenTime   map[string]int64   // Position first seen time (symbol_side -> timestamp in milliseconds)
	stopMonitorCh           chan struct{}      // Used to stop monitoring goroutine
	monitorWg               sync.WaitGroup     // Used to wait for monitoring goroutine to finish
	peakPnLCache            map[string]float64 // Peak profit cache (symbol -> peak P&L percentage)
	peakPnLCacheMutex       sync.RWMutex       // Cache read-write lock
	lastBalanceSyncTime     time.Time          // Last balance sync time
	userID                  string             // User ID
	successfulClosesInCycle int                // Track successful close positions in current cycle (for expected net position calculation)

	// Market microstructure analysis (ADD THIS)
	entryMicrostructure    map[string]*market.MarketMicrostructure // orderID -> full microstructure analysis
	entryMicrostructureMu  sync.RWMutex
	microstructureAnalyzer *market.MarketMicrostructureAnalyzer
	microstructureCache    map[string]*market.MarketMicrostructure
	microstructureCacheMu  sync.RWMutex

	// Market monitoring for adaptive triggers (Phase 1.3)
	lastPrices          map[string]float64                  // Last observed prices (symbol -> price)
	lastPricesMutex     sync.RWMutex                        // Mutex for price map
	volumeBaseline      map[string]float64                  // Average volume baseline (symbol -> volume)
	volumeBaselineMutex sync.RWMutex                        // Mutex for volume baseline
	lastMarketCheckTime time.Time                           // Last time we checked for market triggers
	orderBookMonitors   map[string]*market.OrderBookMonitor // Order book monitors per symbol
	orderBookMonitorsMu sync.RWMutex                        // Mutex for order book monitors

	// Event-driven architecture (Phase 2)
	eventBus              *EventBus              // Centralized event bus for trading signals
	orderWebSocketManager *OrderWebSocketManager // WebSocket manager for real-time order updates (Phase 2.2)

	// Prompt optimization (genetic evolution)
	promptOptimizer *backtest.PromptOptimizer // Evolves prompt strategies based on performance
	promptVariantID string                    // Current prompt variant ID
	// Analysis systems (same as backtests)
	feedbackGenerator *backtest.FeedbackGenerator // Analyzes trading feedback
	factorOptimizer   *backtest.FactorOptimizer   // Analyzes performance factors
	complianceTracker *backtest.ComplianceTracker // Tracks compliance metrics

	// Feedback cycle management (avoid regenerating every cycle)
	lastFeedback      *backtest.FeedbackAnalysis // Last generated feedback analysis
	feedbackCycle     int                        // Cycle number when feedback was last generated
	failureThresholds decision.FailureThresholds // Calibrated failure detection thresholds
}

// NewAutoTrader creates an automatic trader
// st parameter is used to store decision records to database
func NewAutoTrader(config AutoTraderConfig, st *store.Store, userID string) (*AutoTrader, error) {
	// Set default values
	if config.ID == "" {
		config.ID = "default_trader"
	}
	if config.Name == "" {
		config.Name = "Default Trader"
	}
	if config.AIModel == "" {
		if config.UseQwen {
			config.AIModel = "qwen"
		} else {
			config.AIModel = "deepseek"
		}
	}

	// Initialize AI client based on provider
	var mcpClient mcp.AIClient
	aiModel := config.AIModel
	if config.UseQwen && aiModel == "" {
		aiModel = "qwen"
	}

	switch aiModel {
	case "claude":
		mcpClient = mcp.NewClaudeClient()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using Claude AI", config.Name)

	case "kimi":
		mcpClient = mcp.NewKimiClient()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using Kimi (Moonshot) AI", config.Name)

	case "gemini":
		mcpClient = mcp.NewGeminiClient()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using Google Gemini AI", config.Name)

	case "grok":
		mcpClient = mcp.NewGrokClient()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using xAI Grok AI", config.Name)

	case "openai":
		mcpClient = mcp.NewOpenAIClient()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using OpenAI", config.Name)

	case "qwen":
		mcpClient = mcp.NewQwenClient()
		apiKey := config.QwenKey
		if apiKey == "" {
			apiKey = config.CustomAPIKey
		}
		mcpClient.SetAPIKey(apiKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using Alibaba Cloud Qwen AI", config.Name)

	case "custom":
		mcpClient = mcp.New()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using custom AI API: %s (model: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)

	default: // deepseek or empty
		mcpClient = mcp.NewDeepSeekClient()
		apiKey := config.DeepSeekKey
		if apiKey == "" {
			apiKey = config.CustomAPIKey
		}
		mcpClient.SetAPIKey(apiKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using DeepSeek AI", config.Name)
	}

	if config.CustomAPIURL != "" || config.CustomModelName != "" {
		logger.Infof("🔧 [%s] Custom config - URL: %s, Model: %s", config.Name, config.CustomAPIURL, config.CustomModelName)
	}

	// Set default trading platform
	if config.Exchange == "" {
		config.Exchange = "binance"
	}

	// Create corresponding trader based on configuration
	var trader Trader
	var err error

	// Record position mode (general)
	marginModeStr := "Cross Margin"
	if !config.IsCrossMargin {
		marginModeStr = "Isolated Margin"
	}
	logger.Infof("📊 [%s] Position mode: %s", config.Name, marginModeStr)

	switch config.Exchange {
	case "binance":
		logger.Infof("🏦 [%s] Using Binance Futures trading (testnet: %v)", config.Name, config.BinanceTestnet)
		trader = NewFuturesTrader(config.BinanceAPIKey, config.BinanceSecretKey, userID, config.BinanceTestnet)
	case "bybit":
		logger.Infof("🏦 [%s] Using Bybit Futures trading (testnet: %v)", config.Name, config.BybitTestnet)
		trader = NewBybitTrader(config.BybitAPIKey, config.BybitSecretKey)
	case "okx":
		logger.Infof("🏦 [%s] Using OKX Futures trading", config.Name)
		trader = NewOKXTrader(config.OKXAPIKey, config.OKXSecretKey, config.OKXPassphrase)
	case "bitget":
		logger.Infof("🏦 [%s] Using Bitget Futures trading", config.Name)
		trader = NewBitgetTrader(config.BitgetAPIKey, config.BitgetSecretKey, config.BitgetPassphrase)
	case "hyperliquid":
		logger.Infof("🏦 [%s] Using Hyperliquid trading", config.Name)
		trader, err = NewHyperliquidTrader(config.HyperliquidPrivateKey, config.HyperliquidWalletAddr, config.HyperliquidTestnet)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Hyperliquid trader: %w", err)
		}
	case "aster":
		logger.Infof("🏦 [%s] Using Aster trading", config.Name)
		trader, err = NewAsterTrader(config.AsterUser, config.AsterSigner, config.AsterPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Aster trader: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported trading platform: %s", config.Exchange)
	}

	// Validate initial balance configuration, auto-fetch from exchange if 0
	if config.InitialBalance <= 0 {
		logger.Infof("📊 [%s] Initial balance not set, attempting to fetch current balance from exchange...", config.Name)
		account, err := trader.GetBalance()
		if err != nil {
			return nil, fmt.Errorf("initial balance not set and unable to fetch balance from exchange: %w", err)
		}
		// Try multiple balance field names (different exchanges return different formats)
		balanceKeys := []string{"total_equity", "totalWalletBalance", "wallet_balance", "totalEq", "balance"}
		var foundBalance float64
		for _, key := range balanceKeys {
			if balance, ok := account[key].(float64); ok && balance > 0 {
				foundBalance = balance
				break
			}
		}
		if foundBalance > 0 {
			config.InitialBalance = foundBalance
			logger.Infof("✓ [%s] Auto-fetched initial balance: %.2f USDT", config.Name, foundBalance)
			// Save to database so it persists across restarts
			if st != nil {
				if err := st.Trader().UpdateInitialBalance(userID, config.ID, foundBalance); err != nil {
					logger.Infof("⚠️  [%s] Failed to save initial balance to database: %v", config.Name, err)
				} else {
					logger.Infof("✓ [%s] Initial balance saved to database", config.Name)
				}
			}
		} else {
			return nil, fmt.Errorf("initial balance must be greater than 0, please set InitialBalance in config or ensure exchange account has balance")
		}
	}

	// Get last cycle number (for recovery)
	var cycleNumber int
	if st != nil {
		cycleNumber, _ = st.Decision().GetLastCycleNumber(config.ID)
		logger.Infof("📊 [%s] Decision records will be stored to database", config.Name)
	}

	// Create strategy engine (must have strategy config)
	if config.StrategyConfig == nil {
		return nil, fmt.Errorf("[%s] strategy not configured", config.Name)
	}
	strategyEngine := decision.NewStrategyEngine(config.StrategyConfig)
	strategyLang := "en"
	if strings.Contains(strings.ToLower(config.StrategyConfig.PromptSections.RoleDefinition), "交易") {
		strategyLang = "zh"
	}
	config.StrategyConfig.SetConfigPromptSectionsByModeAndLang(config.TradingMode, strategyLang)
	logger.Infof("✓ [%s] Using strategy engine (strategy configuration loaded)", config.Name)

	at := &AutoTrader{
		id:                     config.ID,
		name:                   config.Name,
		aiModel:                config.AIModel,
		exchange:               config.Exchange,
		exchangeID:             config.ExchangeID,
		showInCompetition:      config.ShowInCompetition,
		config:                 config,
		trader:                 trader,
		mcpClient:              mcpClient,
		store:                  st,
		strategyEngine:         strategyEngine,
		cycleNumber:            cycleNumber,
		initialBalance:         config.InitialBalance,
		lastResetTime:          time.Now(),
		startTime:              time.Now(),
		callCount:              0,
		isRunning:              false,
		positionFirstSeenTime:  make(map[string]int64),
		stopMonitorCh:          make(chan struct{}),
		monitorWg:              sync.WaitGroup{},
		peakPnLCache:           make(map[string]float64),
		peakPnLCacheMutex:      sync.RWMutex{},
		lastBalanceSyncTime:    time.Now(),
		userID:                 userID,
		lastPrices:             make(map[string]float64),
		lastPricesMutex:        sync.RWMutex{},
		volumeBaseline:         make(map[string]float64),
		volumeBaselineMutex:    sync.RWMutex{},
		lastMarketCheckTime:    time.Now(),
		microstructureAnalyzer: market.NewMarketMicrostructureAnalyzer(),
		microstructureCache:    make(map[string]*market.MarketMicrostructure),
		entryMicrostructure:    make(map[string]*market.MarketMicrostructure), // Use full type
		orderBookMonitors:      make(map[string]*market.OrderBookMonitor),
		orderBookMonitorsMu:    sync.RWMutex{},
		eventBus:               NewEventBus(),
	}

	// Initialize WebSocket manager with the same EventBus
	at.orderWebSocketManager = NewOrderWebSocketManager(at.eventBus)

	// Initialize PromptOptimizer for live strategy evolution with persistence
	basePrompt := config.StrategyConfig.PromptSections
	optimizerConfig := backtest.DefaultPromptOptimizerConfig()
	optimizerConfig.PopulationSize = 3      // Smaller population for live trading
	optimizerConfig.EvaluationCycles = 5    // Evolve every 5 trades
	optimizerConfig.MinDecisionsPerTest = 5 // Min 5 trades per variant

	// Pass trader ID as runID and backtestStore for variant persistence
	var backtestStore *store.BacktestStore
	if st != nil {
		backtestStore = st.Backtest()
	}
	at.promptOptimizer = backtest.NewPromptOptimizerWithAI(&basePrompt, optimizerConfig, mcpClient, config.ID, backtestStore)
	at.promptVariantID = at.promptOptimizer.GetCurrentVariant().ID
	// Try to load saved optimizer state
	if st != nil {
		if err := at.promptOptimizer.LoadState(config.ID); err != nil {
			logger.Infof("[%s] No saved optimizer state found, starting fresh", config.Name)
		}
	}

	// Initialize analysis systems (feedback, factor, compliance) for live trading
	at.feedbackGenerator = backtest.NewFeedbackGenerator(at.id, at.initialBalance, backtest.DefaultFeedbackConfig())
	at.factorOptimizer = backtest.NewFactorOptimizer(&config.StrategyConfig.RiskControl, backtest.DefaultFactorOptimizerConfig())
	at.complianceTracker = backtest.NewComplianceTracker(backtest.DefaultComplianceConfig())
	at.lastFeedback = nil
	at.feedbackCycle = 0
	// Initialize default failure thresholds (will be calibrated over time)
	at.failureThresholds = decision.DefaultFailureThresholds()
	logger.Infof("✓ [%s] Analysis systems initialized: Feedback, Factor Optimizer, Compliance Tracker", config.Name)

	return at, nil
}

// Run runs the automatic trading main loop
func (at *AutoTrader) Run() error {
	at.isRunningMutex.Lock()
	at.isRunning = true
	at.isRunningMutex.Unlock()

	at.stopMonitorCh = make(chan struct{})
	at.startTime = time.Now()

	// Save live trading config for feedback analysis (backtest-compatible format)
	at.saveLiveTradingConfig()

	logger.Info("🚀 AI-driven automatic trading system started")
	logger.Infof("💰 Initial balance: %.2f USDT", at.initialBalance)
	logger.Infof("⚙️  Scan interval: %v", at.config.ScanInterval)
	logger.Info("🤖 AI will make full decisions on leverage, position size, stop loss/take profit, etc.")
	at.monitorWg.Add(1)
	defer at.monitorWg.Done()

	// Start drawdown monitoring
	at.startDrawdownMonitor()

	// Determine order sync interval (use configured value, default to 30 seconds if not set)
	orderSyncInterval := at.config.OrderSyncInterval
	if orderSyncInterval == 0 {
		orderSyncInterval = 30 * time.Second
	}

	// Start Hyperliquid order sync if using Hyperliquid exchange
	if at.exchange == "hyperliquid" {
		if hyperliquidTrader, ok := at.trader.(*HyperliquidTrader); ok && at.store != nil {
			hyperliquidTrader.StartOrderSync(at.id, at.exchangeID, at.exchange, at.store, orderSyncInterval)
			logger.Infof("🔄 [%s] Hyperliquid order+position sync enabled (every %v)", at.name, orderSyncInterval)
		}
	}

	// Start Bybit order sync if using Bybit exchange
	if at.exchange == "bybit" {
		if bybitTrader, ok := at.trader.(*BybitTrader); ok && at.store != nil {
			bybitTrader.StartOrderSync(at.id, at.exchangeID, at.exchange, at.store, orderSyncInterval)
			logger.Infof("🔄 [%s] Bybit order+position sync enabled (every %v)", at.name, orderSyncInterval)
		}
	}

	// Start OKX order sync if using OKX exchange
	if at.exchange == "okx" {
		if okxTrader, ok := at.trader.(*OKXTrader); ok && at.store != nil {
			okxTrader.StartOrderSync(at.id, at.exchangeID, at.exchange, at.store, orderSyncInterval)
			logger.Infof("🔄 [%s] OKX order+position sync enabled (every %v)", at.name, orderSyncInterval)
		}
	}

	// Start Bitget order sync if using Bitget exchange
	if at.exchange == "bitget" {
		if bitgetTrader, ok := at.trader.(*BitgetTrader); ok && at.store != nil {
			bitgetTrader.StartOrderSync(at.id, at.exchangeID, at.exchange, at.store, orderSyncInterval)
			logger.Infof("🔄 [%s] Bitget order+position sync enabled (every %v)", at.name, orderSyncInterval)
		}
	}

	// Start Aster order sync if using Aster exchange
	if at.exchange == "aster" {
		if asterTrader, ok := at.trader.(*AsterTrader); ok && at.store != nil {
			asterTrader.StartOrderSync(at.id, at.exchangeID, at.exchange, at.store, orderSyncInterval)
			logger.Infof("🔄 [%s] Aster order+position sync enabled (every %v)", at.name, orderSyncInterval)
		}
	}

	// Start Binance order sync if using Binance exchange
	if at.exchange == "binance" {
		if binanceTrader, ok := at.trader.(*FuturesTrader); ok && at.store != nil {
			binanceTrader.StartOrderSync(at.id, at.exchangeID, at.exchange, at.store, orderSyncInterval)
			logger.Infof("🔄 [%s] Binance order+position sync enabled (every %v)", at.name, orderSyncInterval)
		}
	}

	// Initialize WebSocket order streams (Phase 2.2) - Replace polling with real-time updates
	at.initializeOrderWebSockets()

	// Start with initial scan interval (will be adjusted dynamically)
	currentInterval := at.calculateAdaptiveScanInterval()
	ticker := time.NewTicker(currentInterval)
	defer ticker.Stop()

	// Execute immediately on first run
	if err := at.runCycle(); err != nil {
		logger.Infof("❌ Execution failed: %v", err)
	}

	// Track when we last adjusted the interval
	lastIntervalAdjustment := time.Now()

	for {
		at.isRunningMutex.RLock()
		running := at.isRunning
		at.isRunningMutex.RUnlock()

		if !running {
			break
		}

		select {
		case <-ticker.C:
			if err := at.runCycle(); err != nil {
				logger.Infof("❌ Execution failed: %v", err)
			}

			// Recalculate scan interval every 5 minutes or after significant market changes
			if time.Since(lastIntervalAdjustment) > 5*time.Minute {
				newInterval := at.calculateAdaptiveScanInterval()
				if newInterval != currentInterval {
					logger.Infof("🔄 Adjusting scan interval from %v to %v based on market conditions", currentInterval, newInterval)
					currentInterval = newInterval
					ticker.Reset(currentInterval)
					lastIntervalAdjustment = time.Now()
				}
			}
		case <-at.stopMonitorCh:
			logger.Infof("[%s] ⏹ Stop signal received, exiting automatic trading main loop", at.name)
			return nil
		}
	}

	return nil
}

// calculateAdaptiveScanInterval calculates optimal scan interval based on market conditions
// Returns interval shorter during volatile periods or when positions are open
// (Phase 1.2 Optimization: Adaptive Scan Intervals)
func (at *AutoTrader) calculateAdaptiveScanInterval() time.Duration {
	// Get current positions count
	positions, err := at.trader.GetPositions()
	openPositionCount := 0
	if err == nil && positions != nil {
		openPositionCount = len(positions)
	}

	// Calculate recent volatility from position P&L if available
	var recentVolatility float64 = 0.5 // Default medium volatility

	if at.store != nil {
		// Get recent closed positions to estimate volatility
		recentPositions, err := at.store.Position().GetClosedPositions(at.id, 10)
		if err == nil && len(recentPositions) > 0 {
			// Calculate average win/loss percentage as volatility indicator
			totalPnLPercent := 0.0
			for _, pos := range recentPositions {
				if pos.EntryPrice > 0 && pos.ExitPrice > 0 {
					pnlPercent := math.Abs((pos.ExitPrice - pos.EntryPrice) / pos.EntryPrice)
					totalPnLPercent += pnlPercent
				}
			}
			recentVolatility = totalPnLPercent / float64(len(recentPositions))
		}
	}

	// Determine scan interval based on conditions
	baseInterval := at.config.ScanInterval
	if baseInterval == 0 {
		baseInterval = 3 * time.Minute // Default 3 minutes
	}

	// High volatility threshold (> 2% average move per position)
	if recentVolatility > 0.02 {
		logger.Infof("⚡ High volatility detected (%.2f%%): using aggressive scan interval (30s)", recentVolatility*100)
		return 30 * time.Second // Fast response during volatile periods
	}

	// Medium volatility with open positions
	if recentVolatility > 0.01 || openPositionCount > 0 {
		adjustedInterval := baseInterval / 2 // Cut scan interval in half
		logger.Infof("📊 Medium activity (volatility: %.2f%%, positions: %d): using %v scan interval",
			recentVolatility*100, openPositionCount, adjustedInterval)
		return adjustedInterval
	}

	// Low volatility and no open positions - use default or longer interval
	logger.Infof("🟢 Low activity (volatility: %.2f%%, positions: %d): using standard %v scan interval",
		recentVolatility*100, openPositionCount, baseInterval)
	return baseInterval
}

// checkMarketTriggers checks for market conditions that should trigger immediate AI analysis
// Returns true if analysis should be triggered (Phase 1.3 Optimization: Order Book Monitoring)
func (at *AutoTrader) checkMarketTriggers() bool {
	// Don't check too frequently (at least 5 seconds between checks)
	if time.Since(at.lastMarketCheckTime) < 5*time.Second {
		return false
	}
	at.lastMarketCheckTime = time.Now()

	// Get trading context
	ctx, err := at.buildTradingContext()
	if err != nil {
		return false
	}

	if ctx == nil {
		return false
	}

	triggered := false
	triggerReasons := []string{}

	// Check market data for trigger conditions
	for symbol, marketData := range ctx.MarketDataMap {
		if marketData == nil {
			continue
		}

		currentPrice := marketData.CurrentPrice
		if currentPrice <= 0 {
			continue
		}

		// Update last price
		at.lastPricesMutex.Lock()
		lastPrice, exists := at.lastPrices[symbol]
		at.lastPrices[symbol] = currentPrice
		at.lastPricesMutex.Unlock()

		// Skip check if we haven't seen this price before
		if !exists {
			continue
		}

		// Trigger 1: Significant price movement (> 0.5% since last check)
		priceMovePercent := math.Abs((currentPrice - lastPrice) / lastPrice)
		if priceMovePercent > 0.005 { // 0.5%
			triggered = true
			triggerReasons = append(triggerReasons, fmt.Sprintf("Price spike on %s: %.2f%%", symbol, priceMovePercent*100))
		}

		// Trigger 2: Significant volume from latest kline (if available)
		if marketData.IntradaySeries != nil && len(marketData.IntradaySeries.MidPrices) > 0 {
			// Use intraday data as proxy for volume monitoring
			// Actual volume would come from exchange WebSocket in production
			at.volumeBaselineMutex.Lock()
			baseline, hasBaseline := at.volumeBaseline[symbol]
			if !hasBaseline || baseline == 0 {
				// Initialize baseline
				at.volumeBaseline[symbol] = 1.0 // Normalized baseline
			}
			at.volumeBaselineMutex.Unlock()
		}

		// Check order book monitors if available (Phase 1.3)
		at.orderBookMonitorsMu.RLock()
		monitor, hasMonitor := at.orderBookMonitors[symbol]
		at.orderBookMonitorsMu.RUnlock()

		if hasMonitor && monitor != nil {
			anomalies := monitor.CheckTriggers()
			if len(anomalies) > 0 {
				triggered = true
				for _, anomaly := range anomalies {
					triggerReasons = append(triggerReasons, fmt.Sprintf("Order book anomaly on %s: %s", symbol, anomaly.TriggerType))
				}
			}
		}
	}

	if triggered && len(triggerReasons) > 0 {
		logger.Infof("🚨 Market trigger detected: %s", strings.Join(triggerReasons, " | "))

		// Publish market anomaly events to event bus
		for _, reason := range triggerReasons {
			at.publishMarketEvent(EventTypeMarketAnomaly, "", 0.8, map[string]interface{}{
				"reason":    reason,
				"timestamp": time.Now(),
			})
		}

		return true
	}

	return false
}

// Stop stops the automatic trading
func (at *AutoTrader) Stop() {
	at.isRunningMutex.Lock()
	if !at.isRunning {
		at.isRunningMutex.Unlock()
		return
	}
	at.isRunning = false
	at.isRunningMutex.Unlock()

	// Save prompt optimizer state before stopping
	if at.promptOptimizer != nil && at.store != nil {
		if err := at.promptOptimizer.SaveState(at.id); err != nil {
			logger.Infof("⚠️ Failed to save optimizer state on stop: %v", err)
		} else {
			logger.Info("💾 Prompt optimizer state saved")
		}
	}

	close(at.stopMonitorCh) // Notify monitoring goroutine to stop
	at.monitorWg.Wait()     // Wait for monitoring goroutine to finish
	logger.Info("⏹ Automatic trading system stopped")
}

// runCycle runs one trading cycle (using AI full decision-making)
func (at *AutoTrader) runCycle() error {
	at.callCount++
	at.successfulClosesInCycle = 0 // Reset successful closes counter at start of each cycle

	logger.Info("\n" + strings.Repeat("=", 70) + "\n")
	logger.Infof("⏰ %s - AI decision cycle #%d", time.Now().Format("2006-01-02 15:04:05"), at.callCount)
	logger.Info(strings.Repeat("=", 70))

	// 0. Check if trader is stopped (early exit to prevent trades after Stop() is called)
	at.isRunningMutex.RLock()
	running := at.isRunning
	at.isRunningMutex.RUnlock()
	if !running {
		logger.Infof("⏹ Trader is stopped, aborting cycle #%d", at.callCount)
		return nil
	}

	// Create decision record
	record := &store.DecisionRecord{
		ExecutionLog: []string{},
		Success:      true,
	}

	// 1. Check if trading needs to be stopped
	if time.Now().Before(at.stopUntil) {
		remaining := time.Until(at.stopUntil)
		logger.Infof("⏸ Risk control: Trading paused, remaining %.0f minutes", remaining.Minutes())
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("Risk control paused, remaining %.0f minutes", remaining.Minutes())
		if err := at.saveDecision(record); err != nil {
			logger.Infof("⚠️  Failed to save decision: %v", err)
		}
		return nil
	}

	// 2. Reset daily P&L (reset every day)
	if time.Since(at.lastResetTime) > 24*time.Hour {
		at.dailyPnL = 0
		at.lastResetTime = time.Now()
		logger.Info("📅 Daily P&L reset")
	}

	// 3. Check for order book triggers (Phase 1.3 - event-driven triggers)
	at.checkOrderBookTriggers()

	// 3.5. Check for market triggers (event-driven monitoring)
	if at.checkMarketTriggers() {
		logger.Info("🚨 Market trigger detected, proceeding with AI decision")
	}

	// 4. Collect trading context
	ctx, err := at.buildTradingContext()
	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("Failed to build trading context: %v", err)
		if saveErr := at.saveDecision(record); saveErr != nil {
			logger.Infof("⚠️  Failed to save decision: %v", saveErr)
		}
		return fmt.Errorf("failed to build trading context: %w", err)
	}

	// Save equity snapshot independently (decoupled from AI decision, used for drawing profit curve)
	at.saveEquitySnapshot(ctx)

	// Save checkpoint for feedback analysis (backtest-compatible format)
	at.saveCheckpoint()

	logger.Info(strings.Repeat("=", 70))
	for _, coin := range ctx.CandidateCoins {
		record.CandidateCoins = append(record.CandidateCoins, coin.Symbol)
	}

	logger.Infof("📊 Account equity: %.2f USDT | Available: %.2f USDT | Positions: %d",
		ctx.Account.TotalEquity, ctx.Account.AvailableBalance, ctx.Account.PositionCount)

	// 5. Use strategy engine to call AI for decision
	logger.Infof("🤖 Requesting AI analysis and decision... [Strategy Engine]")
	aiDecision, err := decision.GetFullDecisionWithStrategy(ctx, at.mcpClient, at.strategyEngine, at.config.TradingMode)

	if aiDecision != nil && aiDecision.AIRequestDurationMs > 0 {
		record.AIRequestDurationMs = aiDecision.AIRequestDurationMs
		logger.Infof("⏱️ AI call duration: %.2f seconds", float64(record.AIRequestDurationMs)/1000)
		record.ExecutionLog = append(record.ExecutionLog,
			fmt.Sprintf("AI call duration: %d ms", record.AIRequestDurationMs))
	}

	// Save chain of thought, decisions, and input prompt even if there's an error (for debugging)
	if aiDecision != nil {
		record.SystemPrompt = aiDecision.SystemPrompt // Save system prompt
		record.InputPrompt = aiDecision.UserPrompt
		record.CoTTrace = aiDecision.CoTTrace
		record.RawResponse = aiDecision.RawResponse // Save raw AI response for debugging
		at.complianceTracker.CheckCompliance(at.callCount, &aiDecision.Decisions[0], at.lastFeedback)
		if len(aiDecision.Decisions) > 0 {
			decisionJSON, _ := json.MarshalIndent(aiDecision.Decisions, "", "  ")
			record.DecisionJSON = string(decisionJSON)
		}
	}

	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("Failed to get AI decision: %v", err)

		// Print system prompt and AI chain of thought (output even with errors for debugging)
		if aiDecision != nil {
			logger.Info("\n" + strings.Repeat("=", 70) + "\n")
			logger.Infof("📋 System prompt (error case)")
			logger.Info(strings.Repeat("=", 70))
			logger.Info(aiDecision.SystemPrompt)
			logger.Info(strings.Repeat("=", 70))

			if aiDecision.CoTTrace != "" {
				logger.Info("\n" + strings.Repeat("-", 70) + "\n")
				logger.Info("💭 AI chain of thought analysis (error case):")
				logger.Info(strings.Repeat("-", 70))
				logger.Info(aiDecision.CoTTrace)
				logger.Info(strings.Repeat("-", 70))
			}
		}

		if saveErr := at.saveDecision(record); saveErr != nil {
			logger.Infof("⚠️  Failed to save decision: %v", saveErr)
		}
		return fmt.Errorf("failed to get AI decision: %w", err)
	}

	logger.Info()
	logger.Info(strings.Repeat("-", 70))

	// 6. Sort decisions: ensure close positions first, then open positions (prevent position stacking overflow)
	sortedDecisions := sortDecisionsByPriority(aiDecision.Decisions)

	logger.Info("🔄 Execution order (optimized): Close positions first → Open positions later")
	for i, d := range sortedDecisions {
		logger.Infof("  [%d] %s %s", i+1, d.Symbol, d.Action)
	}
	logger.Info()

	// 7. Check if trader is stopped before executing any decisions (prevent trades after Stop())
	at.isRunningMutex.RLock()
	running = at.isRunning
	at.isRunningMutex.RUnlock()
	if !running {
		logger.Infof("⏹ Trader stopped before decision execution, aborting cycle #%d", at.callCount)
		return nil
	}

	// Execute decisions and record results
	for _, d := range sortedDecisions {
		// Ensure stop loss and take profit are positive
		if d.StopLoss < 0 {
			logger.Warnf("⚠️ Stop loss value %.4f is negative, converting to absolute.", d.StopLoss)
			d.StopLoss = math.Abs(d.StopLoss)
		}
		if d.TakeProfit < 0 {
			logger.Warnf("⚠️ Take profit value %.4f is negative, converting to absolute.", d.TakeProfit)
			d.TakeProfit = math.Abs(d.TakeProfit)
		}

		// Check if trader is stopped before each decision (allow immediate stop during execution)
		at.isRunningMutex.RLock()
		running = at.isRunning
		at.isRunningMutex.RUnlock()
		if !running {
			logger.Infof("⏹ Trader stopped during decision execution, aborting remaining decisions")
			break
		}

		actionRecord := store.DecisionAction{
			Action:     d.Action,
			Symbol:     d.Symbol,
			Quantity:   0,
			Leverage:   d.Leverage,
			Price:      0,
			StopLoss:   d.StopLoss,
			TakeProfit: d.TakeProfit,
			Confidence: d.Confidence,
			Reasoning:  d.Reasoning,
			Timestamp:  time.Now(),
			Success:    false,
		}

		if err := at.executeDecisionWithRecord(&d, &actionRecord); err != nil {
			logger.Infof("❌ Failed to execute decision (%s %s): %v", d.Symbol, d.Action, err)
			actionRecord.Error = err.Error()
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("❌ %s %s failed: %v", d.Symbol, d.Action, err))
		} else {
			actionRecord.Success = true
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("✓ %s %s succeeded", d.Symbol, d.Action))
			// Track successful closes for expected net position calculation (Issue #3 fix)
			if d.Action == "close_long" || d.Action == "close_short" {
				at.successfulClosesInCycle++
				logger.Infof("  📊 Successful closes in cycle: %d", at.successfulClosesInCycle)
			}
			// Brief delay after successful execution
			time.Sleep(1 * time.Second)
		}

		record.Decisions = append(record.Decisions, actionRecord)
	}

	// 9. Save decision record
	if err := at.saveDecision(record); err != nil {
		logger.Infof("⚠ Failed to save decision record: %v", err)
	}

	return nil
}

// buildTradingContext builds trading context
func (at *AutoTrader) buildTradingContext() (*decision.Context, error) {
	// 1. Get account information
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("failed to get account balance: %w", err)
	}

	// Get account fields
	totalWalletBalance := 0.0
	totalUnrealizedProfit := 0.0
	availableBalance := 0.0
	totalEquity := 0.0

	if wallet, ok := balance["totalWalletBalance"].(float64); ok {
		totalWalletBalance = wallet
	}
	if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
		totalUnrealizedProfit = unrealized
	}
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Use totalEquity directly if provided by trader (more accurate)
	if eq, ok := balance["totalEquity"].(float64); ok && eq > 0 {
		totalEquity = eq
	} else {
		// Fallback: Total Equity = Wallet balance + Unrealized profit
		totalEquity = totalWalletBalance + totalUnrealizedProfit
	}

	// 2. Get position information
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var positionInfos []decision.PositionInfo
	totalMarginUsed := 0.0

	// Current position key set (for cleaning up closed position records)
	currentPositionKeys := make(map[string]bool)

	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity // Short position quantity is negative, convert to positive
		}

		// Skip closed positions (quantity = 0), prevent "ghost positions" from being passed to AI
		if quantity == 0 {
			continue
		}

		unrealizedPnl := pos["unRealizedProfit"].(float64)
		liquidationPrice := pos["liquidationPrice"].(float64)

		// Calculate margin used (estimated)
		leverage := int(config.DefaultMaxLeverage) // Default value, should actually be fetched from position info
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed

		// Calculate P&L percentage (based on margin, considering leverage)
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)

		// Get position open time from exchange (preferred) or fallback to local tracking
		posKey := symbol + "_" + side
		currentPositionKeys[posKey] = true

		var updateTime int64
		// Priority 1: Get from database (trader_positions table) - most accurate
		if at.store != nil {
			if dbPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, symbol, side); err == nil && dbPos != nil {
				if !dbPos.EntryTime.IsZero() {
					updateTime = dbPos.EntryTime.UnixMilli()
				}
			}
		}
		// Priority 2: Get from exchange API (Bybit: createdTime, OKX: createdTime)
		if updateTime == 0 {
			if createdTime, ok := pos["createdTime"].(int64); ok && createdTime > 0 {
				updateTime = createdTime
			}
		}
		// Priority 3: Fallback to local tracking
		if updateTime == 0 {
			if _, exists := at.positionFirstSeenTime[posKey]; !exists {
				at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()
			}
			updateTime = at.positionFirstSeenTime[posKey]
		}

		// Get peak profit rate for this position
		at.peakPnLCacheMutex.RLock()
		peakPnlPct := at.peakPnLCache[posKey]
		at.peakPnLCacheMutex.RUnlock()

		positionInfos = append(positionInfos, decision.PositionInfo{
			Symbol:           symbol,
			Side:             side,
			EntryPrice:       entryPrice,
			MarkPrice:        markPrice,
			Quantity:         quantity,
			Leverage:         leverage,
			UnrealizedPnL:    unrealizedPnl,
			UnrealizedPnLPct: pnlPct,
			PeakPnLPct:       peakPnlPct,
			LiquidationPrice: liquidationPrice,
			MarginUsed:       marginUsed,
			UpdateTime:       updateTime,
		})
	}

	// Clean up closed position records
	for key := range at.positionFirstSeenTime {
		if !currentPositionKeys[key] {
			delete(at.positionFirstSeenTime, key)
		}
	}

	// 3. Use strategy engine to get candidate coins (must have strategy engine)
	if at.strategyEngine == nil {
		return nil, fmt.Errorf("trader has no strategy engine configured")
	}
	candidateCoins, err := at.strategyEngine.GetCandidateCoins()
	if err != nil {
		return nil, fmt.Errorf("failed to get candidate coins: %w", err)
	}
	logger.Infof("📋 [%s] Strategy engine fetched candidate coins: %d", at.name, len(candidateCoins))

	// 4. Calculate total P&L
	totalPnL := totalEquity - at.initialBalance
	totalPnLPct := 0.0
	if at.initialBalance > 0 {
		totalPnLPct = (totalPnL / at.initialBalance) * 100
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	// 5. Get leverage from strategy config
	strategyConfig := at.strategyEngine.GetConfig()
	btcEthLeverage := strategyConfig.RiskControl.BTCETHMaxLeverage
	altcoinLeverage := strategyConfig.RiskControl.AltcoinMaxLeverage
	logger.Infof("📋 [%s] Strategy leverage config: BTC/ETH=%dx, Altcoin=%dx", at.name, btcEthLeverage, altcoinLeverage)

	// Detect strategy language
	strategyLang := "en"
	if strings.Contains(strings.ToLower(strategyConfig.PromptSections.RoleDefinition), "交易") {
		strategyLang = "zh"
	}

	// 6. Build context
	ctx := &decision.Context{
		CurrentTime:     time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		RuntimeMinutes:  int(time.Since(at.startTime).Minutes()),
		CallCount:       at.callCount,
		BTCETHLeverage:  btcEthLeverage,
		AltcoinLeverage: altcoinLeverage,
		Account: decision.AccountInfo{
			TotalEquity:      totalEquity,
			AvailableBalance: availableBalance,
			UnrealizedPnL:    totalUnrealizedProfit,
			TotalPnL:         totalPnL,
			TotalPnLPct:      totalPnLPct,
			MarginUsed:       totalMarginUsed,
			MarginUsedPct:    marginUsedPct,
			PositionCount:    len(positionInfos),
		},
		Positions:      positionInfos,
		CandidateCoins: candidateCoins,
	}

	// Surface current risk-control parameters will be populated from stats below
	// See ctx.OptimizedWeights assignment in the stats block above

	// 7. Add recent closed trades (if store is available)
	if at.store != nil {
		// Feed the feedback to LLM
		lang := "en"
		if strings.Contains(strings.ToLower(at.strategyEngine.GetConfig().PromptSections.RoleDefinition), "交易") {
			lang = "zh"
		}
		// Get recent 10 closed trades for AI context
		recentTrades, err := at.store.Position().GetRecentTrades(at.id, 10)
		if err != nil {
			logger.Infof("⚠️ [%s] Failed to get recent trades: %v", at.name, err)
		} else {
			logger.Infof("📊 [%s] Found %d recent closed trades for AI context", at.name, len(recentTrades))
			for _, trade := range recentTrades {
				// Convert Unix timestamps to formatted strings for AI readability
				entryTimeStr := ""
				if trade.EntryTime > 0 {
					entryTimeStr = time.Unix(trade.EntryTime, 0).UTC().Format("01-02 15:04 UTC")
				}
				exitTimeStr := ""
				if trade.ExitTime > 0 {
					exitTimeStr = time.Unix(trade.ExitTime, 0).UTC().Format("01-02 15:04 UTC")
				}

				ctx.RecentOrders = append(ctx.RecentOrders, decision.RecentOrder{
					Symbol:       trade.Symbol,
					Side:         trade.Side,
					EntryPrice:   trade.EntryPrice,
					ExitPrice:    trade.ExitPrice,
					RealizedPnL:  trade.RealizedPnL,
					PnLPct:       trade.PnLPct,
					EntryTime:    entryTimeStr,
					ExitTime:     exitTimeStr,
					HoldDuration: trade.HoldDuration,
				})
			}
		}
		// Get trading statistics for AI context
		stats, err := at.store.Position().GetFullStats(at.id)
		if err != nil {
			logger.Infof("⚠️ [%s] Failed to get trading stats: %v", at.name, err)
		} else if stats == nil {
			logger.Infof("⚠️ [%s] GetFullStats returned nil", at.name)
		} else if stats.TotalTrades == 0 {
			logger.Infof("⚠️ [%s] GetFullStats returned 0 trades (traderID=%s)", at.name, at.id)
		} else {
			ctx.TradingStats = &decision.TradingStats{
				TotalTrades:    stats.TotalTrades,
				WinRate:        stats.WinRate,
				ProfitFactor:   stats.ProfitFactor,
				SharpeRatio:    stats.SharpeRatio,
				TotalPnL:       stats.TotalPnL,
				AvgWin:         stats.AvgWin,
				AvgLoss:        stats.AvgLoss,
				MaxDrawdownPct: stats.MaxDrawdownPct,
			}

			// Generate feedback if enabled and enough trades have been made
			// Regenerate feedback every FeedbackWindowCycles cycles to avoid constant recalculation
			if at.feedbackGenerator != nil && stats.TotalTrades >= backtest.DefaultFeedbackConfig().FeedbackWindowCycles {
				if at.lastFeedback == nil || (stats.TotalTrades-at.feedbackCycle) >= backtest.DefaultFeedbackConfig().FeedbackWindowCycles {
					feedback, err := at.feedbackGenerator.GenerateFeedback()
					if err != nil {
						logger.Warnf("⚠️ [%s] Failed to generate feedback analysis: %v", at.name, err)
					} else if feedback != nil {
						at.lastFeedback = feedback
						at.feedbackCycle = stats.TotalTrades
						// Save feedback analysis for later reference and analytics
						if err := at.feedbackGenerator.SaveFeedbackAnalysis(feedback); err != nil {
							logger.Infof("⚠️ [%s] Failed to save feedback analysis: %v", at.name, err)
						}
						logger.Infof("✅ [%s] Generated feedback analysis at cycle %d: Total Return %.2f%%, Win Rate %.1f%%",
							at.name, stats.TotalTrades, feedback.TotalReturnPct, feedback.WinRate)
						// Calibrate failure thresholds from trading history (every 5 trades)
						if stats.TotalTrades >= 10 {
							if recentTrades, err := at.store.Position().GetRecentTrades(at.id, 500); err == nil && len(recentTrades) > 0 {
								calibrator := decision.NewThresholdCalibrator()
								outcomes := make([]decision.TradeOutcome, 0, len(recentTrades))
								for _, trade := range recentTrades {
									holdingMinutes := 0
									if trade.ExitTime > 0 && trade.EntryTime > 0 && trade.ExitTime > trade.EntryTime {
										holdingMinutes = int(time.Unix(trade.ExitTime, 0).Sub(time.Unix(trade.EntryTime, 0)).Minutes())
									}
									outcomes = append(outcomes, decision.TradeOutcome{
										Symbol:            trade.Symbol,
										Profitable:        trade.RealizedPnL > 0,
										VolumeAtEntry:     1.0,
										OIAtEntry:         0.0,
										VolumeDuringTrade: 0.0,
										OIDuringTrade:     0.0,
										EntrySpread:       0.0,
										ExitSpread:        0.0,
										EntryDepth:        0.0,
										ExitDepth:         0.0,
										HoldingMinutes:    holdingMinutes,
										PnLPct:            trade.PnLPct,
									})
								}
								if err := calibrator.CalibrateFromHistory(outcomes); err == nil {
									at.failureThresholds = calibrator.ApplyToAnalyzer()
									logger.Infof("📊 [%s] Calibrated failure thresholds from %d trades: %s",
										at.name, len(outcomes), calibrator.GetCalibrationSummary())
								}
							}
						}

						// Optimize factor weights based on feedback
						if at.factorOptimizer != nil && at.factorOptimizer.ShouldOptimize(stats.TotalTrades, len(positionInfos)) {
							if err := at.factorOptimizer.OptimizeWeights(feedback, stats.TotalTrades); err != nil {
								logger.Infof("⚠️ [%s] Failed to optimize factor weights: %v", at.name, err)
							} else {
								// Save optimizer state
								if err := at.factorOptimizer.SaveState(at.id); err != nil {
									logger.Infof("⚠️ [%s] Failed to save factor optimizer state: %v", at.name, err)
								}
							}
							// Update context with optimized weights
							at.strategyEngine.GetConfig().RiskControl = *at.factorOptimizer.GetRiskControlConfig()
						}

						// Evolve prompts based on performance
						if at.promptOptimizer != nil && at.promptOptimizer.ShouldEvolve(stats.TotalTrades) {
							metrics := &backtest.Metrics{
								TotalReturnPct: feedback.TotalReturnPct,
								WinRate:        feedback.WinRate,
								ProfitFactor:   feedback.ProfitFactor,
								SharpeRatio:    feedback.SharpeRatio,
								MaxDrawdownPct: feedback.MaxDrawdown,
							}
							at.promptOptimizer.RecordDecisionOutcome(at.promptVariantID, metrics)

							if err := at.promptOptimizer.EvolvePrompts(at.promptVariantID, &strategyConfig.PromptSections); err != nil {
								logger.Infof("⚠️ [%s] Failed to evolve prompts: %v", at.name, err)
							} else {
								// Save optimizer state
								if err := at.promptOptimizer.SaveState(at.id); err != nil {
									logger.Infof("⚠️ [%s] Failed to save prompt optimizer state: %v", at.name, err)
								}

								// CRITICAL: Update strategy engine with evolved prompt
								evolvedPrompt := at.promptOptimizer.GetCurrentPrompt()
								at.strategyEngine.SetStrategyPrompt(evolvedPrompt)
								// CRITICAL: Reset prompt variant ID to use new prompt
								at.promptVariantID = at.promptOptimizer.GetCurrentVariant().ID
								logger.Infof("✅ [%s] Applied evolved prompt to live trading (gen %d)", at.name, at.promptOptimizer.GetGeneration())
							}
						}

						// Update compliance tracker with active recommendations
						if at.complianceTracker != nil {
							at.complianceTracker.SetRecommendations(feedback.RecommendedActions)
						}
					}
				}
			}

			// Attach feedback to context
			if at.lastFeedback != nil {
				ctx.PerformanceFeedback = at.feedbackGenerator.FormatFeedbackForPrompt(at.lastFeedback, lang, false)

				// Attach optimized factor weights
				if at.factorOptimizer != nil {
					ctx.OptimizedWeights = at.factorOptimizer.GetCurrentWeights()
				}

				// Attach compliance feedback (reinforcement learning)
				if at.complianceTracker != nil {
					at.complianceTracker.SetRecommendations(at.lastFeedback.RecommendedActions)
					ctx.ComplianceFeedback = at.complianceTracker.GetComplianceFeedback(strategyLang)
					ctx.ComplianceFeedback = at.complianceTracker.GetComplianceFeedback(strategyLang)
				}

				// Attach calibrated thresholds (learned risk detection thresholds)
				if at.failureThresholds != (decision.FailureThresholds{}) {
					calibrator := decision.NewThresholdCalibrator()
					calibrator.WeakVolumeThreshold = at.failureThresholds.WeakVolumeThreshold
					calibrator.WeakOIThreshold = at.failureThresholds.WeakOIThreshold
					calibrator.PrematureVolumeThreshold = at.failureThresholds.PrematureVolumeThreshold
					calibrator.PrematureOIThreshold = at.failureThresholds.PrematureOIThreshold
					calibrator.VolumeDecayThreshold = at.failureThresholds.VolumeDecayThreshold
					calibrator.OIDecayThreshold = at.failureThresholds.OIDecayThreshold
					calibrator.SpreadWorseningMultiple = at.failureThresholds.SpreadWorseningMultiple
					calibrator.DepthReductionThreshold = at.failureThresholds.DepthReductionThreshold
					calibrator.SampleSize = stats.TotalTrades
					ctx.CalibratedThresholds = calibrator.GetThresholdsForLLM(strategyLang, 35)
				}
			}

			logger.Infof("📈 [%s] Trading stats: %d trades, %.1f%% win rate, PF=%.2f, Sharpe=%.2f, DD=%.1f%%",
				at.name, stats.TotalTrades, stats.WinRate, stats.ProfitFactor, stats.SharpeRatio, stats.MaxDrawdownPct)
		}
	} else {
		logger.Infof("⚠️ [%s] Store is nil, cannot get recent trades", at.name)
	}

	// 8. Get quantitative data (if enabled in strategy config)
	if strategyConfig.Indicators.EnableQuantData && strategyConfig.Indicators.QuantDataAPIURL != "" {
		// Collect symbols to query (candidate coins + position coins)
		symbolsToQuery := make(map[string]bool)
		for _, coin := range candidateCoins {
			symbolsToQuery[coin.Symbol] = true
		}
		for _, pos := range positionInfos {
			symbolsToQuery[pos.Symbol] = true
		}

		symbols := make([]string, 0, len(symbolsToQuery))
		for sym := range symbolsToQuery {
			symbols = append(symbols, sym)
		}

		logger.Infof("📊 [%s] Fetching quantitative data for %d symbols...", at.name, len(symbols))
		ctx.QuantDataMap = at.strategyEngine.FetchQuantDataBatch(symbols)
		logger.Infof("📊 [%s] Successfully fetched quantitative data for %d symbols", at.name, len(ctx.QuantDataMap))
	}

	// 9. Get OI ranking data (market-wide position changes)
	if strategyConfig.Indicators.EnableOIRanking {
		logger.Infof("📊 [%s] Fetching OI ranking data...", at.name)
		ctx.OIRankingData = at.strategyEngine.FetchOIRankingData()
		if ctx.OIRankingData != nil {
			logger.Infof("📊 [%s] OI ranking data ready: %d top, %d low positions",
				at.name, len(ctx.OIRankingData.TopPositions), len(ctx.OIRankingData.LowPositions))
		}
	}

	// Update market data monitors with fresh data
	if ctx.MarketDataMap != nil {
		for _, marketData := range ctx.MarketDataMap {
			at.updateMarketData(marketData)
		}
	}

	return ctx, nil
}

// executeDecisionWithRecord executes AI decision and records detailed information
func (at *AutoTrader) executeDecisionWithRecord(decision *decision.Decision, actionRecord *store.DecisionAction) error {
	switch decision.Action {
	case "open_long":
		return at.executeOpenLongWithRecord(decision, actionRecord)
	case "open_short":
		return at.executeOpenShortWithRecord(decision, actionRecord)
	case "close_long":
		return at.executeCloseLongWithRecord(decision, actionRecord)
	case "close_short":
		return at.executeCloseShortWithRecord(decision, actionRecord)
	case "hold", "wait":
		// No execution needed, just record
		return nil
	default:
		return fmt.Errorf("unknown action: %s", decision.Action)
	}
}

// ExecuteDecision executes a trading decision from external sources (e.g., debate consensus)
// This is a public method that can be called by other modules
func (at *AutoTrader) ExecuteDecision(d *decision.Decision) error {
	logger.Infof("[%s] Executing external decision: %s %s", at.name, d.Action, d.Symbol)

	// Create a minimal action record for tracking
	actionRecord := &store.DecisionAction{
		Symbol:     d.Symbol,
		Action:     d.Action,
		Leverage:   d.Leverage,
		StopLoss:   d.StopLoss,
		TakeProfit: d.TakeProfit,
		Confidence: d.Confidence,
		Reasoning:  d.Reasoning,
	}

	// Execute the decision
	err := at.executeDecisionWithRecord(d, actionRecord)
	if err != nil {
		logger.Errorf("[%s] External decision execution failed: %v", at.name, err)
		return err
	}

	logger.Infof("[%s] External decision executed successfully: %s %s", at.name, d.Action, d.Symbol)
	return nil
}

// executeOpenLongWithRecord executes open long position and records detailed information
func (at *AutoTrader) executeOpenLongWithRecord(decision *decision.Decision, actionRecord *store.DecisionAction) error {
	logger.Infof("  📈 Open long: %s", decision.Symbol)

	// ⚠️ Get current positions for multiple checks
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	// [CODE ENFORCED] Check max positions limit (with expected net position calculation for Issue #3)
	if err := at.enforceMaxPositions(len(positions), at.successfulClosesInCycle); err != nil {
		return err
	}

	// Check if there's already a position in the same symbol and direction
	for _, pos := range positions {
		if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
			return fmt.Errorf("❌ %s already has long position, close it first", decision.Symbol)
		}
	}

	// Get current price
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}

	// Get balance (needed for multiple checks)
	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("failed to get account balance: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Get equity for position value ratio check
	equity := 0.0
	if eq, ok := balance["totalEquity"].(float64); ok && eq > 0 {
		equity = eq
	} else if eq, ok := balance["totalWalletBalance"].(float64); ok && eq > 0 {
		equity = eq
	} else {
		equity = availableBalance // Fallback to available balance
	}

	// [CODE ENFORCED] Position Value Ratio Check: position_value <= equity × ratio
	adjustedPositionSize, wasCapped := at.enforcePositionValueRatio(decision.PositionSizeUSD, equity, decision.Symbol)
	if wasCapped {
		decision.PositionSizeUSD = adjustedPositionSize
	}

	// ⚠️ Auto-adjust position size if insufficient margin
	// Formula: totalRequired = positionSize/leverage + positionSize*0.001 + positionSize/leverage*0.01
	//        = positionSize * (1.01/leverage + 0.001)
	marginFactor := 1.01/float64(decision.Leverage) + 0.001
	maxAffordablePositionSize := availableBalance / marginFactor

	actualPositionSize := decision.PositionSizeUSD
	if actualPositionSize > maxAffordablePositionSize {
		// Use 98% of max to leave buffer for price fluctuation
		adjustedSize := maxAffordablePositionSize * 0.98
		logger.Infof("  ⚠️ Position size %.2f exceeds max affordable %.2f, auto-reducing to %.2f",
			actualPositionSize, maxAffordablePositionSize, adjustedSize)
		actualPositionSize = adjustedSize
		decision.PositionSizeUSD = actualPositionSize
	}

	// [CODE ENFORCED] Minimum position size check
	if err := at.enforceMinPositionSize(decision.PositionSizeUSD); err != nil {
		return err
	}

	// Calculate quantity with adjusted position size
	quantity := actualPositionSize / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	// Set margin mode
	if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
		logger.Infof("  ⚠️ Failed to set margin mode: %v", err)
		// Continue execution, doesn't affect trading
	}

	// Open position
	order, err := at.trader.OpenLong(decision.Symbol, quantity, decision.Leverage)
	if err != nil {
		return err
	}

	// Record order ID
	var orderID string
	if id, ok := order["orderId"].(int64); ok {
		orderID = fmt.Sprintf("%d", id)
		actionRecord.OrderID = id
	}

	logger.Infof("  ✓ Position opened successfully, order ID: %s, quantity: %.4f", orderID, quantity)

	// Capture entry microstructure BEFORE recording order (to get clean market snapshot)
	if orderID != "" {
		at.recordEntryMicrostructure(orderID, decision.Symbol)
	}

	// Record order to database and poll for confirmation
	at.recordAndConfirmOrder(order, decision.Symbol, "open_long", quantity, marketData.CurrentPrice, decision.Leverage)

	// Record position opening time
	posKey := decision.Symbol + "_long"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

	// Set stop loss and take profit
	if err := at.trader.SetStopLoss(decision.Symbol, "LONG", quantity, decision.StopLoss); err != nil {
		logger.Infof("  ⚠ Failed to set stop loss: %v", err)
	}
	if err := at.trader.SetTakeProfit(decision.Symbol, "LONG", quantity, decision.TakeProfit); err != nil {
		logger.Infof("  ⚠ Failed to set take profit: %v", err)
	}

	return nil
}

// executeOpenShortWithRecord executes open short position and records detailed information
func (at *AutoTrader) executeOpenShortWithRecord(decision *decision.Decision, actionRecord *store.DecisionAction) error {
	logger.Infof("  📉 Open short: %s", decision.Symbol)

	// ⚠️ Get current positions for multiple checks
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	// [CODE ENFORCED] Check max positions limit
	if err := at.enforceMaxPositions(len(positions), at.successfulClosesInCycle); err != nil {
		return err
	}

	// Check if there's already a position in the same symbol and direction
	for _, pos := range positions {
		if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
			return fmt.Errorf("❌ %s already has short position, close it first", decision.Symbol)
		}
	}

	// Get current price
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}

	// Get balance (needed for multiple checks)
	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("failed to get account balance: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Get equity for position value ratio check
	equity := 0.0
	if eq, ok := balance["totalEquity"].(float64); ok && eq > 0 {
		equity = eq
	} else if eq, ok := balance["totalWalletBalance"].(float64); ok && eq > 0 {
		equity = eq
	} else {
		equity = availableBalance // Fallback to available balance
	}

	// [CODE ENFORCED] Position Value Ratio Check: position_value <= equity × ratio
	adjustedPositionSize, wasCapped := at.enforcePositionValueRatio(decision.PositionSizeUSD, equity, decision.Symbol)
	if wasCapped {
		decision.PositionSizeUSD = adjustedPositionSize
	}

	// ⚠️ Auto-adjust position size if insufficient margin
	// Formula: totalRequired = positionSize/leverage + positionSize*0.001 + positionSize/leverage*0.01
	//        = positionSize * (1.01/leverage + 0.001)
	marginFactor := 1.01/float64(decision.Leverage) + 0.001
	maxAffordablePositionSize := availableBalance / marginFactor

	actualPositionSize := decision.PositionSizeUSD
	if actualPositionSize > maxAffordablePositionSize {
		// Use 98% of max to leave buffer for price fluctuation
		adjustedSize := maxAffordablePositionSize * 0.98
		logger.Infof("  ⚠️ Position size %.2f exceeds max affordable %.2f, auto-reducing to %.2f",
			actualPositionSize, maxAffordablePositionSize, adjustedSize)
		actualPositionSize = adjustedSize
		decision.PositionSizeUSD = actualPositionSize
	}

	// [CODE ENFORCED] Minimum position size check
	if err := at.enforceMinPositionSize(decision.PositionSizeUSD); err != nil {
		return err
	}

	// Calculate quantity with adjusted position size
	quantity := actualPositionSize / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	// Set margin mode
	if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
		logger.Infof("  ⚠️ Failed to set margin mode: %v", err)
		// Continue execution, doesn't affect trading
	}

	// Open position
	order, err := at.trader.OpenShort(decision.Symbol, quantity, decision.Leverage)
	if err != nil {
		return err
	}

	// Record order ID
	var orderID string
	if id, ok := order["orderId"].(int64); ok {
		orderID = fmt.Sprintf("%d", id)
		actionRecord.OrderID = id
	}

	logger.Infof("  ✓ Position opened successfully, order ID: %s, quantity: %.4f", orderID, quantity)

	// Capture entry microstructure BEFORE recording order (to get clean market snapshot)
	if orderID != "" {
		at.recordEntryMicrostructure(orderID, decision.Symbol)
	}

	// Record order to database and poll for confirmation
	at.recordAndConfirmOrder(order, decision.Symbol, "open_short", quantity, marketData.CurrentPrice, decision.Leverage)

	// Record position opening time
	posKey := decision.Symbol + "_short"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

	// Set stop loss and take profit
	if err := at.trader.SetStopLoss(decision.Symbol, "SHORT", quantity, decision.StopLoss); err != nil {
		logger.Infof("  ⚠ Failed to set stop loss: %v", err)
	}
	if err := at.trader.SetTakeProfit(decision.Symbol, "SHORT", quantity, decision.TakeProfit); err != nil {
		logger.Infof("  ⚠ Failed to set take profit: %v", err)
	}

	return nil
}

// executeCloseLongWithRecord executes close long position and records detailed information
// executeCloseLongWithRecord executes close long position and records detailed information
func (at *AutoTrader) executeCloseLongWithRecord(decision *decision.Decision, actionRecord *store.DecisionAction) error {
	logger.Infof("  🔄 Close long: %s", decision.Symbol)

	// Get current price
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// Normalize symbol for database lookup
	normalizedSymbol := market.Normalize(decision.Symbol)

	// Get entry price and quantity - prioritize local database for accurate quantity
	var entryPrice float64
	var quantity float64
	var entryOrderID string

	// First try to get from local database (more accurate for quantity)
	if at.store != nil {
		if openPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, normalizedSymbol, "LONG"); err == nil && openPos != nil {
			quantity = openPos.Quantity
			entryPrice = openPos.EntryPrice
			entryOrderID = openPos.EntryOrderID
			logger.Infof("  📊 Using local position data: qty=%.8f, entry=%.2f", quantity, entryPrice)
		}
	}

	// Fallback to exchange API if local data not found
	if quantity == 0 {
		positions, err := at.trader.GetPositions()
		if err == nil {
			for _, pos := range positions {
				if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
					if ep, ok := pos["entryPrice"].(float64); ok {
						entryPrice = ep
					}
					if amt, ok := pos["positionAmt"].(float64); ok && amt > 0 {
						quantity = amt
					}
					break
				}
			}
		}
		logger.Infof("  📊 Using exchange position data: qty=%.8f, entry=%.2f", quantity, entryPrice)
	}

	// Close position
	order, err := at.trader.CloseLong(decision.Symbol, 0) // 0 = close all
	if err != nil {
		return err
	}

	// Record order ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	// Record order to database and poll for confirmation
	at.recordAndConfirmOrder(order, decision.Symbol, "close_long", quantity, marketData.CurrentPrice, 0)

	// Analyze the closed trade for failure patterns (Issue #2: Live Trade Failure Analysis)
	if at.store != nil {
		pnlPct := 0.0
		if entryPrice > 0 {
			pnlPct = ((marketData.CurrentPrice - entryPrice) / entryPrice) * 100
		}

		// Calculate realized PnL in dollars
		realizedPnL := (marketData.CurrentPrice - entryPrice) * quantity

		// Get entry microstructure if available
		var entryMicro *market.MarketMicrostructure
		if entryOrderID != "" {
			at.entryMicrostructureMu.RLock()
			entryMicro = at.entryMicrostructure[entryOrderID]
			at.entryMicrostructureMu.RUnlock()
		}

		// Get exit microstructure for comparison
		exitMicro, err := at.GetMicrostructureAnalysis(decision.Symbol)
		if err != nil {
			logger.Debugf("⚠️ Failed to get exit microstructure: %v", err)
		}

		// Save comprehensive trade outcome for failure analysis
		outcome := &store.TradeOutcome{
			Symbol:     decision.Symbol,
			Profitable: pnlPct >= 0,
			PnLPct:     pnlPct,
		}

		// Extract entry microstructure metrics (if available)
		if entryMicro != nil {
			outcome.EntrySpread = entryMicro.BidAskSpread
			outcome.EntryDepth = float64(entryMicro.BidDepth + entryMicro.AskDepth)
			if vol, ok := entryMicro.Details["current_volume"].(float64); ok {
				outcome.VolumeAtEntry = vol
			}

			logger.Infof("  📊 Entry: spread=%.2f%%, depth=%.0f, VWAP=%.4f",
				entryMicro.BidAskSpread, outcome.EntryDepth, entryMicro.VWAP)
		}

		// Extract exit microstructure metrics (if available)
		if exitMicro != nil {
			outcome.ExitSpread = exitMicro.BidAskSpread
			outcome.ExitDepth = float64(exitMicro.BidDepth + exitMicro.AskDepth)
			if vol, ok := exitMicro.Details["current_volume"].(float64); ok {
				outcome.VolumeDuringTrade = vol
			}

			logger.Infof("  📊 Exit: spread=%.2f%%, depth=%.0f, VWAP=%.4f",
				exitMicro.BidAskSpread, outcome.ExitDepth, exitMicro.VWAP)

			// Calculate spread worsening
			if entryMicro != nil && entryMicro.BidAskSpread > 0 {
				spreadChange := (exitMicro.BidAskSpread - entryMicro.BidAskSpread) / entryMicro.BidAskSpread * 100
				logger.Infof("  📊 Spread change: %.2f%% (%.4f → %.4f)",
					spreadChange, entryMicro.BidAskSpread, exitMicro.BidAskSpread)
			}
		}

		// Save trade outcome
		if err := at.store.TradeOutcome().Save(outcome); err != nil {
			logger.Warnf("⚠️ Failed to save trade outcome: %v", err)
		}

		// Clean up entry microstructure after closing position
		if entryOrderID != "" {
			at.entryMicrostructureMu.Lock()
			delete(at.entryMicrostructure, entryOrderID)
			at.entryMicrostructureMu.Unlock()
		}

		// Log realized PnL for feedback (no separate recordTradeEvent needed)
		logger.Infof("  💰 Position closed: PnL %.2f%% (%.2f USDT)", pnlPct, realizedPnL)
	}

	logger.Infof("  ✓ Position closed successfully")
	return nil
}

// executeCloseShortWithRecord executes close short position and records detailed information
func (at *AutoTrader) executeCloseShortWithRecord(decision *decision.Decision, actionRecord *store.DecisionAction) error {
	logger.Infof("  🔄 Close short: %s", decision.Symbol)

	// Get current price
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// Normalize symbol for database lookup
	normalizedSymbol := market.Normalize(decision.Symbol)

	// Get entry price and quantity - prioritize local database for accurate quantity
	var entryPrice float64
	var quantity float64
	var entryOrderID string

	// First try to get from local database (more accurate for quantity)
	if at.store != nil {
		if openPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, normalizedSymbol, "SHORT"); err == nil && openPos != nil {
			quantity = openPos.Quantity
			entryPrice = openPos.EntryPrice
			entryOrderID = openPos.EntryOrderID
			logger.Infof("  📊 Using local position data: qty=%.8f, entry=%.2f", quantity, entryPrice)
		}
	}

	// Fallback to exchange API if local data not found
	if quantity == 0 {
		positions, err := at.trader.GetPositions()
		if err == nil {
			for _, pos := range positions {
				if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
					if ep, ok := pos["entryPrice"].(float64); ok {
						entryPrice = ep
					}
					if amt, ok := pos["positionAmt"].(float64); ok {
						quantity = -amt // positionAmt is negative for short
					}
					break
				}
			}
		}
		logger.Infof("  📊 Using exchange position data: qty=%.8f, entry=%.2f", quantity, entryPrice)
	}

	// Close position
	order, err := at.trader.CloseShort(decision.Symbol, 0) // 0 = close all
	if err != nil {
		return err
	}

	// Record order ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	// Record order to database and poll for confirmation
	at.recordAndConfirmOrder(order, decision.Symbol, "close_short", quantity, marketData.CurrentPrice, 0)

	// Analyze the closed trade for failure patterns (Issue #2: Live Trade Failure Analysis)
	if at.store != nil {
		pnlPct := 0.0
		if entryPrice > 0 {
			// For short positions, profit = (entry - exit) / entry * 100
			pnlPct = ((entryPrice - marketData.CurrentPrice) / entryPrice) * 100
		}

		// Calculate realized PnL in dollars (for shorts: positive when price drops)
		realizedPnL := (entryPrice - marketData.CurrentPrice) * quantity

		// Get entry microstructure if available
		var entryMicro *market.MarketMicrostructure
		if entryOrderID != "" {
			at.entryMicrostructureMu.RLock()
			entryMicro = at.entryMicrostructure[entryOrderID]
			at.entryMicrostructureMu.RUnlock()
		}

		// Get exit microstructure for comparison
		exitMicro, err := at.GetMicrostructureAnalysis(decision.Symbol)
		if err != nil {
			logger.Debugf("⚠️ Failed to get exit microstructure: %v", err)
		}

		// Save comprehensive trade outcome for failure analysis
		outcome := &store.TradeOutcome{
			Symbol:     decision.Symbol,
			Profitable: pnlPct >= 0,
			PnLPct:     pnlPct,
		}

		// Extract entry microstructure metrics (if available)
		if entryMicro != nil {
			outcome.EntrySpread = entryMicro.BidAskSpread
			outcome.EntryDepth = float64(entryMicro.BidDepth + entryMicro.AskDepth)
			if vol, ok := entryMicro.Details["current_volume"].(float64); ok {
				outcome.VolumeAtEntry = vol
			}

			logger.Infof("  📊 Entry: spread=%.2f%%, depth=%.0f, VWAP=%.4f",
				entryMicro.BidAskSpread, outcome.EntryDepth, entryMicro.VWAP)
		}

		// Extract exit microstructure metrics (if available)
		if exitMicro != nil {
			outcome.ExitSpread = exitMicro.BidAskSpread
			outcome.ExitDepth = float64(exitMicro.BidDepth + exitMicro.AskDepth)
			if vol, ok := exitMicro.Details["current_volume"].(float64); ok {
				outcome.VolumeDuringTrade = vol
			}

			logger.Infof("  📊 Exit: spread=%.2f%%, depth=%.0f, VWAP=%.4f",
				exitMicro.BidAskSpread, outcome.ExitDepth, exitMicro.VWAP)

			// Calculate spread worsening
			if entryMicro != nil && entryMicro.BidAskSpread > 0 {
				spreadChange := (exitMicro.BidAskSpread - entryMicro.BidAskSpread) / entryMicro.BidAskSpread * 100
				logger.Infof("  📊 Spread change: %.2f%% (%.4f → %.4f)",
					spreadChange, entryMicro.BidAskSpread, exitMicro.BidAskSpread)
			}
		}

		// Save trade outcome
		if err := at.store.TradeOutcome().Save(outcome); err != nil {
			logger.Warnf("⚠️ Failed to save trade outcome: %v", err)
		}

		// Clean up entry microstructure after closing position
		if entryOrderID != "" {
			at.entryMicrostructureMu.Lock()
			delete(at.entryMicrostructure, entryOrderID)
			at.entryMicrostructureMu.Unlock()
		}

		// Log realized PnL for feedback (no separate recordTradeEvent needed)
		logger.Infof("  💰 Position closed: PnL %.2f%% (%.2f USDT)", pnlPct, realizedPnL)
	}

	logger.Infof("  ✓ Position closed successfully")
	return nil
}

// GetID gets trader ID
func (at *AutoTrader) GetID() string {
	return at.id
}

// GetName gets trader name
func (at *AutoTrader) GetName() string {
	return at.name
}

// GetStrategyEngine returns the active strategy engine.
func (at *AutoTrader) GetStrategyEngine() *decision.StrategyEngine {
	return at.strategyEngine
}

// BuildContextSnapshot builds the current trading context without executing a cycle.
func (at *AutoTrader) BuildContextSnapshot() (*decision.Context, error) {
	return at.buildTradingContext()
}

// GetAIModel gets AI model
func (at *AutoTrader) GetAIModel() string {
	return at.aiModel
}

// GetExchange gets exchange
func (at *AutoTrader) GetExchange() string {
	return at.exchange
}

// GetShowInCompetition returns whether trader should be shown in competition
func (at *AutoTrader) GetShowInCompetition() bool {
	return at.showInCompetition
}

// SetShowInCompetition sets whether trader should be shown in competition
func (at *AutoTrader) SetShowInCompetition(show bool) {
	at.showInCompetition = show
}

// SetCustomPrompt sets custom trading strategy prompt
func (at *AutoTrader) SetCustomPrompt(prompt string) {
	at.customPrompt = prompt
}

// SetOverrideBasePrompt sets whether to override base prompt
func (at *AutoTrader) SetOverrideBasePrompt(override bool) {
	at.overrideBasePrompt = override
}

// GetSystemPromptTemplate gets current system prompt template name (from strategy config)
func (at *AutoTrader) GetSystemPromptTemplate() string {
	if at.strategyEngine != nil {
		config := at.strategyEngine.GetConfig()
		if config.CustomPrompt != "" {
			return "custom"
		}
	}
	return "strategy"
}

// saveEquitySnapshot saves equity snapshot independently (for drawing profit curve, decoupled from AI decision)
func (at *AutoTrader) saveEquitySnapshot(ctx *decision.Context) {
	if at.store == nil || ctx == nil {
		return
	}

	snapshot := &store.EquitySnapshot{
		TraderID:      at.id,
		Timestamp:     time.Now().UTC(),
		TotalEquity:   ctx.Account.TotalEquity,
		Balance:       ctx.Account.TotalEquity - ctx.Account.UnrealizedPnL,
		UnrealizedPnL: ctx.Account.UnrealizedPnL,
		PositionCount: ctx.Account.PositionCount,
		MarginUsedPct: ctx.Account.MarginUsedPct,
	}

	if err := at.store.Equity().Save(snapshot); err != nil {
		logger.Infof("⚠️ Failed to save equity snapshot: %v", err)
	}
}

// saveDecision saves AI decision log to database (only records AI input/output, for debugging)
func (at *AutoTrader) saveDecision(record *store.DecisionRecord) error {
	if at.store == nil {
		return nil
	}

	at.cycleNumber++
	record.CycleNumber = at.cycleNumber
	record.TraderID = at.id

	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	}

	if err := at.store.Decision().LogDecision(record); err != nil {
		logger.Infof("⚠️ Failed to save decision record: %v", err)
		return err
	}

	logger.Infof("📝 Decision record saved: trader=%s, cycle=%d", at.id, at.cycleNumber)
	return nil
}

// GetStore gets data store (for external access to decision records, etc.)
func (at *AutoTrader) GetStore() *store.Store {
	return at.store
}

// GetPromptOptimizer gets prompt optimizer for live trading evolution (nil if not initialized)
func (at *AutoTrader) GetPromptOptimizer() *backtest.PromptOptimizer {
	return at.promptOptimizer
}

// GetFeedbackAnalysis gets feedback analysis for live trading evolution (nil if not ready)
func (at *AutoTrader) GetFeedbackAnalysis() *backtest.FeedbackAnalysis {
	if at.feedbackGenerator == nil {
		return nil
	}
	analysis, err := at.feedbackGenerator.GenerateFeedback()
	if err != nil {
		logger.Warnf("[%s] Error generating feedback analysis: %v", at.config.Name, err)
		return nil
	}
	return analysis
}

// GetStatus gets system status (for API)
func (at *AutoTrader) GetStatus() map[string]interface{} {
	aiProvider := "DeepSeek"
	if at.config.UseQwen {
		aiProvider = "Qwen"
	}

	at.isRunningMutex.RLock()
	isRunning := at.isRunning
	at.isRunningMutex.RUnlock()

	return map[string]interface{}{
		"trader_id":                     at.id,
		"trader_name":                   at.name,
		"ai_model":                      at.aiModel,
		"exchange":                      at.exchange,
		"is_running":                    isRunning,
		"start_time":                    at.startTime.Format(time.RFC3339),
		"runtime_minutes":               int(time.Since(at.startTime).Minutes()),
		"call_count":                    at.callCount,
		"initial_balance":               at.initialBalance,
		"scan_interval":                 at.config.ScanInterval.String(),
		"stop_until":                    at.stopUntil.Format(time.RFC3339),
		"last_reset_time":               at.lastResetTime.Format(time.RFC3339),
		"ai_provider":                   aiProvider,
		"prompt_optimization_active":    at.promptOptimizer != nil,
		"feedback_analysis_active":      at.feedbackGenerator != nil,
		"trade_failure_analysis_active": at.factorOptimizer != nil,
		"compliance_tracking_active":    at.complianceTracker != nil,
	}
}

// GetAccountInfo gets account information (for API)
func (at *AutoTrader) GetAccountInfo() (map[string]interface{}, error) {
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	// Get account fields
	totalWalletBalance := 0.0
	totalUnrealizedProfit := 0.0
	availableBalance := 0.0
	totalEquity := 0.0

	if wallet, ok := balance["totalWalletBalance"].(float64); ok {
		totalWalletBalance = wallet
	}
	if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
		totalUnrealizedProfit = unrealized
	}
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Use totalEquity directly if provided by trader (more accurate)
	if eq, ok := balance["totalEquity"].(float64); ok && eq > 0 {
		totalEquity = eq
	} else {
		// Fallback: Total Equity = Wallet balance + Unrealized profit
		totalEquity = totalWalletBalance + totalUnrealizedProfit
	}

	// Get positions to calculate total margin
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	totalMarginUsed := 0.0
	totalUnrealizedPnLCalculated := 0.0
	for _, pos := range positions {
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl := pos["unRealizedProfit"].(float64)
		totalUnrealizedPnLCalculated += unrealizedPnl

		leverage := int(config.DefaultMaxLeverage)
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed
	}

	// Verify unrealized P&L consistency (API value vs calculated from positions)
	// Note: Lighter API may return 0 for unrealized PnL, this is a known limitation
	diff := math.Abs(totalUnrealizedProfit - totalUnrealizedPnLCalculated)
	if diff > config.MinPositionSize { // Only warn if difference is significant
		logger.Infof("⚠️ Unrealized P&L inconsistency (Lighter API limitation): API=%.4f, Calculated=%.4f, Diff=%.4f",
			totalUnrealizedProfit, totalUnrealizedPnLCalculated, diff)
	}

	totalPnL := totalEquity - at.initialBalance
	totalPnLPct := 0.0
	if at.initialBalance > 0 {
		totalPnLPct = (totalPnL / at.initialBalance) * 100
	} else {
		logger.Infof("⚠️ Initial Balance abnormal: %.2f, cannot calculate P&L percentage", at.initialBalance)
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	return map[string]interface{}{
		// Core fields
		"total_equity":      totalEquity,           // Account equity = wallet + unrealized
		"wallet_balance":    totalWalletBalance,    // Wallet balance (excluding unrealized P&L)
		"unrealized_profit": totalUnrealizedProfit, // Unrealized P&L (official value from exchange API)
		"available_balance": availableBalance,      // Available balance

		// P&L statistics
		"total_pnl":       totalPnL,          // Total P&L = equity - initial
		"total_pnl_pct":   totalPnLPct,       // Total P&L percentage
		"initial_balance": at.initialBalance, // Initial balance
		"daily_pnl":       at.dailyPnL,       // Daily P&L

		// Position information
		"position_count":  len(positions),  // Position count
		"margin_used":     totalMarginUsed, // Margin used
		"margin_used_pct": marginUsedPct,   // Margin usage rate
	}, nil
}

// GetPositions gets position list (for API)
func (at *AutoTrader) GetPositions() ([]map[string]interface{}, error) {
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	// Sync entry prices with local database for consistency
	// This ensures weighted average entry prices are used when positions are accumulated
	synced := at.syncEntryPricesWithDatabase(positions)

	var result []map[string]interface{}
	for _, pos := range synced {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl := pos["unRealizedProfit"].(float64)
		liquidationPrice := pos["liquidationPrice"].(float64)

		leverage := int(config.DefaultMaxLeverage)
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}

		// Calculate margin used
		marginUsed := (quantity * markPrice) / float64(leverage)

		// Calculate P&L percentage (based on margin)
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)

		result = append(result, map[string]interface{}{
			"symbol":             symbol,
			"side":               side,
			"entry_price":        entryPrice,
			"mark_price":         markPrice,
			"quantity":           quantity,
			"leverage":           leverage,
			"unrealized_pnl":     unrealizedPnl,
			"unrealized_pnl_pct": pnlPct,
			"liquidation_price":  liquidationPrice,
			"margin_used":        marginUsed,
		})
	}

	return result, nil
}

// GetMicrostructureAnalysis retrieves cached or fresh microstructure analysis for a symbol
func (at *AutoTrader) GetMicrostructureAnalysis(symbol string) (*market.MarketMicrostructure, error) {
	// Check cache first (valid for 5 seconds)
	at.microstructureCacheMu.RLock()
	cached, exists := at.microstructureCache[symbol]
	at.microstructureCacheMu.RUnlock()

	if exists && cached != nil && time.Since(cached.Timestamp) < 5*time.Second {
		return cached, nil
	}

	// Fetch fresh data
	depth, err := at.microstructureAnalyzer.FetchOrderBookDepth(symbol, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch order book: %w", err)
	}

	// Get klines for VWAP calculation
	klines, err := market.GetKlinesCoinank(symbol, "1m", at.exchange, 20)
	if err != nil {
		return nil, fmt.Errorf("failed to get klines: %w", err)
	}

	currentPrice := 0.0
	if len(klines) > 0 {
		currentPrice = klines[len(klines)-1].Close
	}

	// Analyze microstructure
	analysis, err := at.microstructureAnalyzer.AnalyzeMarketMicrostructure(symbol, depth, currentPrice, klines)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze microstructure: %w", err)
	}

	// Update cache
	at.microstructureCacheMu.Lock()
	at.microstructureCache[symbol] = analysis
	at.microstructureCacheMu.Unlock()

	return analysis, nil
}

// syncEntryPricesWithDatabase syncs entry prices with local database for consistency
// This ensures entry prices reflect weighted average calculations from position accumulation
// rather than relying solely on exchange entry prices which may differ
func (at *AutoTrader) syncEntryPricesWithDatabase(positions []map[string]interface{}) []map[string]interface{} {
	if at.store == nil || at.store.Position() == nil {
		return positions
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		exchangeEntryPrice := pos["entryPrice"].(float64)

		// Try to find corresponding position in local database
		localPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, symbol, side)
		if err == nil && localPos != nil && localPos.EntryPrice > 0 {
			// Found local position record with valid entry price
			// Use local entry price (weighted average) instead of exchange price
			if localPos.EntryPrice != exchangeEntryPrice {
				priceDiff := ((exchangeEntryPrice - localPos.EntryPrice) / localPos.EntryPrice) * 100
				logger.Debugf(
					"  📊 Entry price sync: %s %s - exchange: %.2f, local: %.2f (diff: %.2f%%)",
					symbol, side, exchangeEntryPrice, localPos.EntryPrice, priceDiff,
				)
			}
			pos["entryPrice"] = localPos.EntryPrice
		}

		result = append(result, pos)
	}

	return result
}

// calculatePnLPercentage calculates P&L percentage (based on margin, automatically considers leverage)
// Return rate = Unrealized P&L / Margin × 100%
func calculatePnLPercentage(unrealizedPnl, marginUsed float64) float64 {
	if marginUsed > 0 {
		return (unrealizedPnl / marginUsed) * 100
	}
	return 0.0
}

// sortDecisionsByPriority sorts decisions: close positions first, then open positions, finally hold/wait
// This avoids position stacking overflow when changing positions
func sortDecisionsByPriority(decisions []decision.Decision) []decision.Decision {
	if len(decisions) <= 1 {
		return decisions
	}

	// Define priority
	getActionPriority := func(action string) int {
		switch action {
		case "close_long", "close_short":
			return 1 // Highest priority: close positions first
		case "open_long", "open_short":
			return 2 // Second priority: open positions later
		case "hold", "wait":
			return 3 // Lowest priority: wait
		default:
			return 999 // Unknown actions at the end
		}
	}

	// Copy decision list
	sorted := make([]decision.Decision, len(decisions))
	copy(sorted, decisions)

	// Sort by priority
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if getActionPriority(sorted[i].Action) > getActionPriority(sorted[j].Action) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// startDrawdownMonitor starts drawdown monitoring
func (at *AutoTrader) startDrawdownMonitor() {
	// Get configuration from strategy
	config := at.strategyEngine.GetConfig()
	if config == nil || !config.RiskControl.DrawdownMonitoringEnabled {
		logger.Info("📊 Drawdown monitoring is disabled in strategy configuration")
		return
	}

	// Validate and get check interval (min: 15s, max: 300s, default: 60s)
	checkInterval := config.RiskControl.DrawdownCheckInterval
	if checkInterval < 15 {
		checkInterval = 15
		logger.Infof("⚠️ Drawdown check interval too small, using minimum: 15s")
	} else if checkInterval > 300 {
		checkInterval = 300
		logger.Infof("⚠️ Drawdown check interval too large, using maximum: 300s")
	}

	at.monitorWg.Add(1)
	go func() {
		defer at.monitorWg.Done()

		ticker := time.NewTicker(time.Duration(checkInterval) * time.Second)
		defer ticker.Stop()

		logger.Infof("📊 Started position drawdown monitoring (check every %ds, profit threshold: %.1f%%, drawdown trigger: %.1f%%)",
			checkInterval, config.RiskControl.MinProfitThreshold, config.RiskControl.DrawdownCloseThreshold)

		for {
			select {
			case <-ticker.C:
				at.checkPositionDrawdown()
			case <-at.stopMonitorCh:
				logger.Info("⏹ Stopped position drawdown monitoring")
				return
			}
		}
	}()
}

// checkPositionDrawdown checks position drawdown situation
func (at *AutoTrader) checkPositionDrawdown() {
	// Get configuration from strategy
	strategyConfig := at.strategyEngine.GetConfig()
	if strategyConfig == nil {
		logger.Infof("❌ Drawdown monitoring: failed to get strategy config")
		return
	}

	// Get current positions
	positions, err := at.trader.GetPositions()
	if err != nil {
		logger.Infof("❌ Drawdown monitoring: failed to get positions: %v", err)
		return
	}

	// Get thresholds from configuration
	minProfitThreshold := strategyConfig.RiskControl.MinProfitThreshold
	drawdownTrigger := strategyConfig.RiskControl.DrawdownCloseThreshold

	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)

		// Calculate current P&L percentage
		leverage := int(config.DefaultMaxLeverage) // Default value
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}

		var currentPnLPct float64
		if side == "long" {
			currentPnLPct = ((markPrice - entryPrice) / entryPrice) * float64(leverage) * 100
		} else {
			currentPnLPct = ((entryPrice - markPrice) / entryPrice) * float64(leverage) * 100
		}

		// Construct unique position identifier (distinguish long/short)
		posKey := symbol + "_" + side

		// Get historical peak profit for this position
		at.peakPnLCacheMutex.RLock()
		peakPnLPct, exists := at.peakPnLCache[posKey]
		at.peakPnLCacheMutex.RUnlock()

		if !exists {
			// If no historical peak record, use current P&L as initial value
			peakPnLPct = currentPnLPct
			at.UpdatePeakPnL(symbol, side, currentPnLPct)
		} else {
			// Update peak cache
			at.UpdatePeakPnL(symbol, side, currentPnLPct)
		}

		// Calculate drawdown (magnitude of decline from peak)
		drawdownPct := backtest.CalculateDrawdown(currentPnLPct, peakPnLPct)

		// Close position based on drawdown: Once profit ≥ minProfitThreshold, if it drops back more than drawdownTrigger then close
		if currentPnLPct >= minProfitThreshold && drawdownPct >= drawdownTrigger {
			logger.Warnf("⚠️ Triggering drawdown close position: %s %s | Current Profit: %.2f%% | Peak Profit: %.2f%% | Drawdown from Peak: %.2f%%",
				symbol, side, currentPnLPct, peakPnLPct, drawdownPct)

			// Execute close position operation
			err := at.emergencyClosePosition(symbol, side)
			if err != nil {
				logger.Errorf("❌ Drawdown close position failed: %s %s | Error: %v", symbol, side, err)
			} else {
				logger.Infof("✅ Drawdown close position succeeded: %s %s", symbol, side)
				// Clear cache for this position after closing
				at.ClearPeakPnLCache(symbol, side)
			}
		} else if currentPnLPct > 5.0 {
			// Record situations close to close position condition (for debugging)
			logger.Infof("📊 Drawdown monitoring: %s %s | Profit: %.2f%% | Peak: %.2f%% | Drawdown: %.2f%%",
				symbol, side, currentPnLPct, peakPnLPct, drawdownPct)
		}
	}
}

// emergencyClosePosition emergency close position function
func (at *AutoTrader) emergencyClosePosition(symbol, side string) error {
	switch side {
	case "long":
		order, err := at.trader.CloseLong(symbol, 0) // 0 = close all
		if err != nil {
			return err
		}
		logger.Infof("✅ Emergency close long position succeeded, order ID: %v", order["orderId"])
	case "short":
		order, err := at.trader.CloseShort(symbol, 0) // 0 = close all
		if err != nil {
			return err
		}
		logger.Infof("✅ Emergency close short position succeeded, order ID: %v", order["orderId"])
	default:
		return fmt.Errorf("unknown position direction: %s", side)
	}

	return nil
}

// GetPeakPnLCache gets peak profit cache
func (at *AutoTrader) GetPeakPnLCache() map[string]float64 {
	at.peakPnLCacheMutex.RLock()
	defer at.peakPnLCacheMutex.RUnlock()

	// Return a copy of the cache
	cache := make(map[string]float64)
	for k, v := range at.peakPnLCache {
		cache[k] = v
	}
	return cache
}

// UpdatePeakPnL updates peak profit cache
func (at *AutoTrader) UpdatePeakPnL(symbol, side string, currentPnLPct float64) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	posKey := symbol + "_" + side
	if peak, exists := at.peakPnLCache[posKey]; exists {
		// Update peak (if long, take larger value; if short, currentPnLPct is negative, also compare)
		if currentPnLPct > peak {
			at.peakPnLCache[posKey] = currentPnLPct
		}
	} else {
		// First time recording
		at.peakPnLCache[posKey] = currentPnLPct
	}
}

// ClearPeakPnLCache clears peak cache for specified position
func (at *AutoTrader) ClearPeakPnLCache(symbol, side string) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	posKey := symbol + "_" + side
	delete(at.peakPnLCache, posKey)
}

// recordAndConfirmOrder polls order status for actual fill data and records position
// action: open_long, open_short, close_long, close_short
// entryPrice: entry price when closing (0 when opening)
func (at *AutoTrader) recordAndConfirmOrder(orderResult map[string]interface{}, symbol, action string, quantity float64, price float64, leverage int) {
	if at.store == nil {
		return
	}

	// Get order ID (supports multiple types)
	var orderID string
	switch v := orderResult["orderId"].(type) {
	case int64:
		orderID = fmt.Sprintf("%d", v)
	case float64:
		orderID = fmt.Sprintf("%.0f", v)
	case string:
		orderID = v
	default:
		orderID = fmt.Sprintf("%v", v)
	}

	if orderID == "" || orderID == "0" {
		logger.Infof("  ⚠️ Order ID is empty, skipping record")
		return
	}

	// Determine positionSide
	var positionSide string
	switch action {
	case "open_long", "close_long":
		positionSide = "LONG"
	case "open_short", "close_short":
		positionSide = "SHORT"
	}

	var actualPrice = price
	var actualQty = quantity
	var fee float64

	// Exchanges with OrderSync: Skip immediate order recording, let OrderSync handle it
	// This ensures accurate data from GetTrades API and avoids duplicate records
	switch at.exchange {
	case "binance", "hyperliquid", "bybit", "okx", "bitget", "aster":
		logger.Infof("  📝 Order submitted (id: %s), will be synced by OrderSync", orderID)
		return
	}

	// For exchanges without OrderSync (e.g., Binance): record immediately and poll for fill data
	orderRecord := at.createOrderRecord(orderID, symbol, action, positionSide, quantity, price, leverage)
	if err := at.store.Order().CreateOrder(orderRecord); err != nil {
		logger.Infof("  ⚠️ Failed to record order: %v", err)
	} else {
		logger.Infof("  📝 Order recorded: %s [%s] %s", orderID, action, symbol)
	}

	// Wait for order to be filled and get actual fill data
	time.Sleep(500 * time.Millisecond)
	for i := 0; i < 5; i++ {
		status, err := at.trader.GetOrderStatus(symbol, orderID)
		if err == nil {
			statusStr, _ := status["status"].(string)
			if statusStr == "FILLED" {
				// Get actual fill price
				if avgPrice, ok := status["avgPrice"].(float64); ok && avgPrice > 0 {
					actualPrice = avgPrice
				}
				// Get actual executed quantity
				if execQty, ok := status["executedQty"].(float64); ok && execQty > 0 {
					actualQty = execQty
				}
				// Get commission/fee
				if commission, ok := status["commission"].(float64); ok {
					fee = commission
				}
				logger.Infof("  ✅ Order filled: avgPrice=%.6f, qty=%.6f, fee=%.6f", actualPrice, actualQty, fee)

				// Update order status to FILLED
				if err := at.store.Order().UpdateOrderStatus(orderRecord.ID, "FILLED", actualQty, actualPrice, fee); err != nil {
					logger.Infof("  ⚠️ Failed to update order status: %v", err)
				}

				// Record fill details
				at.recordOrderFill(orderRecord.ID, orderID, symbol, action, actualPrice, actualQty, fee)
				break
			} else if statusStr == "CANCELED" || statusStr == "EXPIRED" || statusStr == "REJECTED" {
				logger.Infof("  ⚠️ Order %s, skipping position record", statusStr)

				// Update order status
				if err := at.store.Order().UpdateOrderStatus(orderRecord.ID, statusStr, 0, 0, 0); err != nil {
					logger.Infof("  ⚠️ Failed to update order status: %v", err)
				}
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Normalize symbol for position record consistency
	normalizedSymbolForPosition := market.Normalize(symbol)

	logger.Infof("  📝 Recording position (ID: %s, action: %s, price: %.6f, qty: %.6f, fee: %.4f)",
		orderID, action, actualPrice, actualQty, fee)

	// Record position change with actual fill data (use normalized symbol)
	at.recordPositionChange(orderID, normalizedSymbolForPosition, positionSide, action, actualQty, actualPrice, leverage, fee)

	// Send anonymous trade statistics for experience improvement (async, non-blocking)
	// This helps us understand overall product usage across all deployments
	experience.TrackTrade(experience.TradeEvent{
		Exchange:  at.exchange,
		TradeType: action,
		Symbol:    symbol,
		AmountUSD: actualPrice * actualQty,
		Leverage:  leverage,
		UserID:    at.userID,
		TraderID:  at.id,
	})
}

// recordEntryMicrostructure records entry microstructure data for a position
func (at *AutoTrader) recordEntryMicrostructure(orderID, symbol string) {
	// Fetch order book depth (100 levels for comprehensive analysis)
	depth, err := at.microstructureAnalyzer.FetchOrderBookDepth(symbol, 100)
	if err != nil {
		logger.Warnf("⚠️ Failed to fetch order book depth for %s: %v", symbol, err)
		return
	}

	// Get recent klines for VWAP and volume analysis
	klines, err := market.GetKlinesCoinank(symbol, "1m", at.exchange, 20)
	if err != nil {
		logger.Warnf("⚠️ Failed to get klines for %s: %v", symbol, err)
		return
	}

	if len(klines) == 0 {
		logger.Warnf("⚠️ No kline data available for %s", symbol)
		return
	}

	// Get current price from latest kline
	currentPrice := klines[len(klines)-1].Close

	// Perform comprehensive microstructure analysis
	microstructure, err := at.microstructureAnalyzer.AnalyzeMarketMicrostructure(
		symbol, depth, currentPrice, klines,
	)
	if err != nil {
		logger.Warnf("⚠️ Failed to analyze microstructure for %s: %v", symbol, err)
		return
	}

	// Store comprehensive analysis in memory for later use during position close
	at.entryMicrostructureMu.Lock()
	at.entryMicrostructure[orderID] = microstructure
	at.entryMicrostructureMu.Unlock()

	// Log key metrics
	logger.Infof("  📊 Entry microstructure recorded for order %s:", orderID)
	logger.Infof("      Bid: %.4f | Ask: %.4f | Spread: %.2f bps",
		microstructure.Details["best_bid"],
		microstructure.Details["best_ask"],
		microstructure.BidAskSpreadBps)
	logger.Infof("      VWAP: %.4f | Deviation: %.2f%% | Imbalance: %.2f",
		microstructure.VWAP,
		microstructure.VWAPDeviation,
		microstructure.OrderBookImbalance)
	logger.Infof("      Bid Depth: %.2f | Ask Depth: %.2f | Large Orders: %d",
		microstructure.BidDepth,
		microstructure.AskDepth,
		microstructure.LargeOrderCount)
}

// saveCheckpoint saves current trading state as checkpoint for feedback analysis
func (at *AutoTrader) saveCheckpoint() {
	if at.store == nil || at.store.Backtest() == nil {
		return
	}

	// Get current account balance
	account, err := at.trader.GetBalance()
	if err != nil {
		logger.Warnf("⚠️ Failed to get balance for checkpoint: %v", err)
		return
	}

	var equity, available, unrealizedPnL, realizedPnL float64

	// Try to extract equity from account info
	if val, ok := account["total_equity"].(float64); ok {
		equity = val
	} else if val, ok := account["totalWalletBalance"].(float64); ok {
		equity = val
	} else {
		accountInfo, err := at.GetAccountInfo()
		if err == nil {
			if val, ok := accountInfo["total_equity"].(float64); ok {
				equity = val
			}
		}
	}

	if val, ok := account["available_balance"].(float64); ok {
		available = val
	} else if val, ok := account["availableBalance"].(float64); ok {
		available = val
	} else {
		accountInfo, err := at.GetAccountInfo()
		if err == nil {
			if val, ok := accountInfo["available_balance"].(float64); ok {
				available = val
			}
		}
	}

	if val, ok := account["total_unrealized_profit"].(float64); ok {
		unrealizedPnL = val
	} else if val, ok := account["totalUnrealizedProfit"].(float64); ok {
		unrealizedPnL = val
	} else {
		accountInfo, err := at.GetAccountInfo()
		if err == nil {
			if val, ok := accountInfo["unrealized_profit"].(float64); ok {
				unrealizedPnL = val
			}
		}
	}

	if val, ok := account["total_pnl"].(float64); ok {
		realizedPnL = val
	} else {
		realizedPnL = equity - at.initialBalance - unrealizedPnL
	}

	checkpoint := map[string]interface{}{
		"equity":         equity,
		"available":      available,
		"unrealized_pnl": unrealizedPnL,
		"realized_pnl":   realizedPnL,
		"timestamp":      time.Now().UnixMilli(),
		"cycle":          at.cycleNumber,
	}

	checkpointJSON, err := json.Marshal(checkpoint)
	if err != nil {
		logger.Warnf("⚠️ Failed to marshal checkpoint: %v", err)
		return
	}

	if err := at.store.Backtest().SaveCheckpoint(at.id, checkpointJSON); err != nil {
		logger.Warnf("⚠️ Failed to save checkpoint: %v", err)
	}
}

// saveLiveTradingConfig saves live trader configuration for feedback analysis compatibility
func (at *AutoTrader) saveLiveTradingConfig() {
	if at.store == nil || at.store.Backtest() == nil {
		return
	}

	// Create minimal config matching BacktestConfig structure for metrics calculation
	// Key fields: InitialBalance (required for return% calculation), UserID, AIModel
	configData := map[string]interface{}{
		"run_id":          at.id,
		"user_id":         at.userID,
		"initial_balance": at.initialBalance,
		"symbols":         []string{}, // Will be populated from actual trades
		"ai_provider":     extractProviderFromModel(at.aiModel),
		"ai_model":        at.aiModel,
		"exchange":        at.exchange,
		"is_live_trading": true,
		"start_time":      time.Now().Unix(),
	}

	configJSON, err := json.Marshal(configData)
	if err != nil {
		logger.Warnf("⚠️ Failed to marshal live trading config: %v", err)
		return
	}

	// Get strategy prompt if available
	promptTemplate := "default"
	customPrompt := ""
	overridePrompt := false
	if at.strategyEngine != nil {
		strategyConfig := at.strategyEngine.GetConfig()
		if strategyConfig != nil && strategyConfig.PromptSections.RoleDefinition != "" {
			promptTemplate = "live_strategy"
			customPrompt = strategyConfig.PromptSections.RoleDefinition
		}
	}

	provider := extractProviderFromModel(at.aiModel)

	// Save to backtest_runs table (using trader ID as runID)
	if err := at.store.Backtest().SaveConfig(
		at.id,          // runID = trader ID for live trading
		at.userID,      // userID
		promptTemplate, // template
		customPrompt,   // custom prompt
		provider,       // AI provider
		at.aiModel,     // AI model
		overridePrompt, // override flag
		configJSON,     // config JSON
	); err != nil {
		logger.Warnf("⚠️ Failed to save live trading config: %v", err)
	} else {
		logger.Infof("✓ Live trading config saved for feedback analysis")
	}
}

// extractProviderFromModel extracts provider name from model string
func extractProviderFromModel(model string) string {
	if strings.Contains(model, "qwen") {
		return "qwen"
	} else if strings.Contains(model, "deepseek") {
		return "deepseek"
	} else if strings.Contains(model, "grok") {
		return "grok"
	} else if strings.Contains(model, "gpt") || strings.Contains(model, "openai") {
		return "openai"
	} else if strings.Contains(model, "claude") {
		return "claude"
	} else if strings.Contains(model, "gemini") {
		return "gemini"
	}
	return "unknown"
}

// recordPositionChange records position change (create record on open, update record on close)
func (at *AutoTrader) recordPositionChange(orderID, symbol, side, action string, quantity, price float64, leverage int, fee float64) {
	if at.store == nil {
		return
	}

	switch action {
	case "open_long", "open_short":
		// Open position: create new position record
		pos := &store.TraderPosition{
			TraderID:     at.id,
			ExchangeID:   at.exchangeID, // Exchange account UUID
			ExchangeType: at.exchange,   // Exchange type: binance/bybit/okx/etc
			Symbol:       symbol,
			Side:         side, // LONG or SHORT
			Quantity:     quantity,
			EntryPrice:   price,
			EntryOrderID: orderID,
			EntryTime:    time.Now(),
			Leverage:     leverage,
			Status:       "OPEN",
		}
		if err := at.store.Position().Create(pos); err != nil {
			logger.Infof("  ⚠️ Failed to record position: %v", err)
		} else {
			logger.Infof("  📊 Position recorded [%s] %s %s @ %.4f", at.id[:8], symbol, side, price)
		}

	case "close_long", "close_short":
		// Close position using PositionBuilder for consistent handling
		// PositionBuilder will handle both cases:
		// 1. If open position exists: close it properly
		// 2. If no open position (e.g., table cleared): create a closed position record
		posBuilder := store.NewPositionBuilder(at.store.Position())
		if err := posBuilder.ProcessTrade(
			at.id, at.exchangeID, at.exchange,
			symbol, side, action,
			quantity, price, fee, 0, // realizedPnL will be calculated
			time.Now(), orderID,
		); err != nil {
			logger.Infof("  ⚠️ Failed to process close position: %v", err)
		} else {
			logger.Infof("  ✅ Position closed [%s] %s %s @ %.4f", at.id[:8], symbol, side, price)
		}
	}
}

// createOrderRecord creates an order record struct from order details
func (at *AutoTrader) createOrderRecord(orderID, symbol, action, positionSide string, quantity, price float64, leverage int) *store.TraderOrder {
	// Determine order type (market for auto trader)
	orderType := "MARKET"

	// Determine side (BUY/SELL) using utility function
	side := getSideFromAction(action)

	// Use action as orderAction directly (keep lowercase format)
	orderAction := action

	// Determine if it's a reduce only order
	reduceOnly := (action == "close_long" || action == "close_short")

	// Normalize symbol for consistency
	normalizedSymbol := market.Normalize(symbol)

	return &store.TraderOrder{
		TraderID:        at.id,
		ExchangeID:      at.exchangeID,
		ExchangeType:    at.exchange,
		ExchangeOrderID: orderID,
		Symbol:          normalizedSymbol,
		Side:            side,
		PositionSide:    positionSide,
		Type:            orderType,
		TimeInForce:     "GTC",
		Quantity:        quantity,
		Price:           price,
		Status:          "NEW",
		FilledQuantity:  0,
		AvgFillPrice:    0,
		Commission:      0,
		CommissionAsset: "USDT",
		Leverage:        leverage,
		ReduceOnly:      reduceOnly,
		ClosePosition:   reduceOnly,
		OrderAction:     orderAction,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

// recordOrderFill records order fill/trade details
func (at *AutoTrader) recordOrderFill(orderRecordID int64, exchangeOrderID, symbol, action string, price, quantity, fee float64) {
	if at.store == nil {
		return
	}

	// Determine side (BUY/SELL)
	var side string
	switch action {
	case "open_long", "close_short":
		side = "BUY"
	case "open_short", "close_long":
		side = "SELL"
	}

	// Generate a simple trade ID (exchange doesn't always provide one)
	tradeID := fmt.Sprintf("%s-%d", exchangeOrderID, time.Now().UnixNano())

	// Normalize symbol for consistency
	normalizedSymbol := market.Normalize(symbol)

	fill := &store.TraderFill{
		TraderID:        at.id,
		ExchangeID:      at.exchangeID,
		ExchangeType:    at.exchange,
		OrderID:         orderRecordID,
		ExchangeOrderID: exchangeOrderID,
		ExchangeTradeID: tradeID,
		Symbol:          normalizedSymbol,
		Side:            side,
		Price:           price,
		Quantity:        quantity,
		QuoteQuantity:   price * quantity,
		Commission:      fee,
		CommissionAsset: "USDT",
		RealizedPnL:     0,     // Will be calculated for close orders
		IsMaker:         false, // Market orders are usually taker
		CreatedAt:       time.Now(),
	}

	// Calculate realized PnL for close orders
	if action == "close_long" || action == "close_short" {
		// Try to get the entry price from the open position
		var positionSide string
		if action == "close_long" {
			positionSide = "LONG"
		} else {
			positionSide = "SHORT"
		}

		if openPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, symbol, positionSide); err == nil && openPos != nil {
			if positionSide == "LONG" {
				fill.RealizedPnL = (price - openPos.EntryPrice) * quantity
			} else {
				fill.RealizedPnL = (openPos.EntryPrice - price) * quantity
			}
		}
	}

	if err := at.store.Order().CreateFill(fill); err != nil {
		logger.Infof("  ⚠️ Failed to record fill: %v", err)
	} else {
		logger.Infof("  📋 Fill recorded: %.4f @ %.6f, fee: %.4f", quantity, price, fee)
	}
}

// ============================================================================
// Risk Control Helpers
// ============================================================================

// isBTCETH checks if a symbol is BTC or ETH
func isBTCETH(symbol string) bool {
	symbol = strings.ToUpper(symbol)
	return strings.HasPrefix(symbol, "BTC") || strings.HasPrefix(symbol, "ETH")
}

// enforcePositionValueRatio checks and enforces position value ratio limits (CODE ENFORCED)
// Returns the adjusted position size (capped if necessary) and whether the position was capped
// positionSizeUSD: the original position size in USD
// equity: the account equity
// symbol: the trading symbol
func (at *AutoTrader) enforcePositionValueRatio(positionSizeUSD float64, equity float64, symbol string) (float64, bool) {
	if at.config.StrategyConfig == nil {
		return positionSizeUSD, false
	}

	riskControl := at.config.StrategyConfig.RiskControl

	// Get the appropriate position value ratio limit
	var maxPositionValueRatio float64
	if isBTCETH(symbol) {
		maxPositionValueRatio = riskControl.BTCETHMaxPositionValueRatio
		if maxPositionValueRatio <= 0 {
			maxPositionValueRatio = config.DefaultBTCETHPosRatio // Default: 5x for BTC/ETH
		}
	} else {
		maxPositionValueRatio = riskControl.AltcoinMaxPositionValueRatio
		if maxPositionValueRatio <= 0 {
			maxPositionValueRatio = config.DefaultAltcoinPosRatio // Default: 1x for altcoins
		}
	}

	// Calculate max allowed position value = equity × ratio
	maxPositionValue := equity * maxPositionValueRatio

	// Check if position size exceeds limit
	if positionSizeUSD > maxPositionValue {
		logger.Infof("  ⚠️ [RISK CONTROL] Position %.2f USDT exceeds limit (equity %.2f × %.1fx = %.2f USDT max for %s), capping",
			positionSizeUSD, equity, maxPositionValueRatio, maxPositionValue, symbol)
		return maxPositionValue, true
	}

	return positionSizeUSD, false
}

// enforceMinPositionSize checks minimum position size (CODE ENFORCED)
func (at *AutoTrader) enforceMinPositionSize(positionSizeUSD float64) error {
	if at.config.StrategyConfig == nil {
		return nil
	}

	minSize := at.config.StrategyConfig.RiskControl.MinPositionSize
	if minSize <= 0 {
		minSize = 12 // Default: 12 USDT
	}

	if positionSizeUSD < minSize {
		return fmt.Errorf("❌ [RISK CONTROL] Position %.2f USDT below minimum (%.2f USDT)", positionSizeUSD, minSize)
	}
	return nil
}

// enforceMaxPositions checks maximum positions count (CODE ENFORCED)
// enforceMaxPositions checks if current position count has reached the max, accounting for pending closes in the current cycle
// successfulClosesInCycle: number of successful close positions executed in this cycle (to account for API lag)
// Issue #3 fix: Implements "expected net position" logic to handle API lag when positions haven't updated yet
func (at *AutoTrader) enforceMaxPositions(currentPositionCount int, successfulClosesInCycle int) error {
	if at.config.StrategyConfig == nil {
		return nil
	}

	maxPositions := at.config.StrategyConfig.RiskControl.MaxPositions
	if maxPositions <= 0 {
		maxPositions = 3 // Default: 3 positions
	}

	// Calculate expected net position count
	// Expected net position = current positions - successful closes in this cycle
	// This accounts for API lag when GetPositions() hasn't been updated yet
	expectedNetPositionCount := currentPositionCount - successfulClosesInCycle
	if expectedNetPositionCount < 0 {
		expectedNetPositionCount = 0 // Never negative
	}

	if expectedNetPositionCount >= maxPositions {
		return fmt.Errorf("❌ [RISK CONTROL] Already at max positions (expected net: %d, current reported: %d, pending closes: %d, max: %d)",
			expectedNetPositionCount, currentPositionCount, successfulClosesInCycle, maxPositions)
	}

	if currentPositionCount >= maxPositions && successfulClosesInCycle > 0 {
		logger.Infof("  ✓ Expected net position (%d) < max (%d) due to %d pending closes, allowing new open position",
			expectedNetPositionCount, maxPositions, successfulClosesInCycle)
	}

	return nil
}

// getSideFromAction converts order action to side (BUY/SELL)
func getSideFromAction(action string) string {
	switch action {
	case "open_long", "close_short":
		return "BUY"
	case "open_short", "close_long":
		return "SELL"
	default:
		return "BUY"
	}
}

// AdjustStopLossTakeProfitWithTracking adjusts stop loss and take profit with tracking for P&L accuracy
// This method updates the SL/TP on the exchange AND records the adjustment in the database
// Issue #13 fix: Ensures P&L is calculated using actual execution prices from exchange
func (at *AutoTrader) AdjustStopLossTakeProfitWithTracking(symbol string, positionSide string, quantity float64, newStopLoss float64, newTakeProfit float64) error {
	if at.store == nil {
		logger.Infof("  ⚠️ Store not available, skipping SL/TP adjustment tracking")
	}

	// Update on exchange
	slErr := at.trader.SetStopLoss(symbol, positionSide, quantity, newStopLoss)
	tpErr := at.trader.SetTakeProfit(symbol, positionSide, quantity, newTakeProfit)

	if slErr != nil {
		logger.Infof("  ⚠️ Failed to set stop loss on exchange: %v", slErr)
	}
	if tpErr != nil {
		logger.Infof("  ⚠️ Failed to set take profit on exchange: %v", tpErr)
	}

	// Track adjustment in database if store is available
	if at.store != nil && at.id != "" {
		// Find open position for this symbol and side
		openPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, symbol, positionSide)
		if err != nil {
			logger.Infof("  ⚠️ Failed to find position for SL/TP tracking: %v", err)
		} else if openPos != nil {
			// Update position with new SL/TP levels
			if err := at.store.Position().UpdateStopLossTakeProfit(openPos.ID, newStopLoss, newTakeProfit); err != nil {
				logger.Infof("  ⚠️ Failed to track SL/TP adjustment: %v", err)
			} else {
				newAdjCount := openPos.AdjustmentCount + 1
				logger.Infof("  📝 Tracked SL/TP adjustment: SL=%.4f, TP=%.4f (adjustment #%d)",
					newStopLoss, newTakeProfit, newAdjCount)
			}
		}
	}

	// Return error only if both failed
	if slErr != nil && tpErr != nil {
		return fmt.Errorf("failed to update both SL and TP: SL:%v, TP:%v", slErr, tpErr)
	}

	return nil
}

// SyncPositionPnLWithExchange fetches actual execution data from exchange for closed positions
// This ensures P&L reflects the actual execution price, not calculated values
// Issue #13 fix: Uses exchange as source of truth for P&L calculations
func (at *AutoTrader) SyncPositionPnLWithExchange(symbol string, positionSide string) error {
	if at.store == nil {
		return fmt.Errorf("store not available")
	}

	if at.id == "" {
		return fmt.Errorf("trader ID not set")
	}

	// Get open position for this symbol to check if recently closed
	openPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, symbol, positionSide)
	if err != nil {
		return fmt.Errorf("failed to check position status: %w", err)
	}

	// Position still open, nothing to sync
	if openPos != nil {
		return nil
	}

	// Position is closed, but we need a way to get the closed position and sync it
	// For now, log that this would be called periodically by a background sync job
	logger.Infof("  📊 Position closed for %s/%s would be synced with exchange in background", symbol, positionSide)

	return nil
}

// initializeOrderWebSockets initializes WebSocket order streams for real-time order updates (Phase 2.2)
func (at *AutoTrader) initializeOrderWebSockets() {
	// Register order handler for all exchanges
	at.orderWebSocketManager.RegisterOrderHandler(func(order market.OrderUpdate) {
		at.handleOrderUpdate(order)
	})

	// Start health check (every 10 seconds)
	at.orderWebSocketManager.StartHealthCheck(10 * time.Second)

	logger.Info("✓ WebSocket order stream health monitoring started")
}

// handleOrderUpdate processes order updates from WebSocket and publishes events
func (at *AutoTrader) handleOrderUpdate(order market.OrderUpdate) {
	logger.Debugf("📊 Order update received: %s %s (status: %s, qty: %.2f, filled: %.2f)",
		order.Symbol, order.Side, order.Status, order.OriginalQuantity, order.ExecutedQuantity)

	// Persist order status to database so UI/analytics stay in sync
	if at.store != nil {
		orderStore := at.store.Order()

		// Find existing order by exchange order ID
		existing, _ := orderStore.GetOrderByExchangeID(at.exchangeID, order.OrderID)

		if existing == nil {
			// Create minimal order record if we never saw it (e.g., placed manually or from another session)
			newOrder := &store.TraderOrder{
				TraderID:        at.id,
				ExchangeID:      at.exchangeID,
				ExchangeType:    at.exchange,
				ExchangeOrderID: order.OrderID,
				ClientOrderID:   order.ClientOrderID,
				Symbol:          order.Symbol,
				Side:            strings.ToUpper(order.Side),
				PositionSide:    strings.ToUpper(order.PositionSide),
				Type:            strings.ToUpper(order.OrderType),
				TimeInForce:     strings.ToUpper(order.TimeInForce),
				Quantity:        order.OriginalQuantity,
				Price:           order.OrderPrice,
				Status:          strings.ToUpper(order.Status),
				FilledQuantity:  order.ExecutedQuantity,
				AvgFillPrice:    order.AveragePrice,
			}
			if err := orderStore.CreateOrder(newOrder); err != nil {
				logger.Infof("[%s] ⚠️ Failed to create order record for %s: %v", at.name, order.OrderID, err)
			}
		} else {
			// Update status/fills on existing order
			if err := orderStore.UpdateOrderStatus(existing.ID, strings.ToUpper(order.Status), order.ExecutedQuantity, order.AveragePrice, existing.Commission); err != nil {
				logger.Infof("[%s] ⚠️ Failed to update order status for %s: %v", at.name, order.OrderID, err)
			}
		}
	}

	// Event publishing is already handled by OrderWebSocketManager
	// (WebSocket manager broadcasts order events on the shared EventBus)
}

// GetOrderWebSocketManager returns the order WebSocket manager
func (at *AutoTrader) GetOrderWebSocketManager() *OrderWebSocketManager {
	return at.orderWebSocketManager
}
