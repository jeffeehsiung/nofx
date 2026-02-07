package backtest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"nofx/config"
	"nofx/logger"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"nofx/decision"
	"nofx/market"
	"nofx/mcp"
	"nofx/store"
)

var (
	errBacktestCompleted = errors.New("backtest completed")
	errLiquidated        = errors.New("account liquidated")
)

const (
	metricsWriteInterval = 5 * time.Second
	aiDecisionMaxRetries = 3
)

// Runner encapsulates the lifecycle of a single backtest run.
type Runner struct {
	cfg            BacktestConfig
	feed           *DataFeed
	account        *BacktestAccount
	strategyEngine *decision.StrategyEngine

	decisionLogDir string
	mcpClient      mcp.AIClient

	statusMu sync.RWMutex
	status   RunState

	stateMu sync.RWMutex
	state   *BacktestState

	pauseCh  chan struct{}
	resumeCh chan struct{}
	stopCh   chan struct{}
	doneCh   chan struct{}

	err              error
	errMu            sync.RWMutex
	lastError        string
	lastCheckpoint   time.Time
	createdAt        time.Time
	lastMetricsWrite time.Time

	aiCache   *AICache
	cachePath string

	lockInfo *RunLockInfo
	lockStop chan struct{}

	// Feedback loop components
	feedbackGenerator   *FeedbackGenerator
	feedbackConfig      FeedbackConfig
	lastFeedback        *FeedbackAnalysis
	feedbackCycle       int // Track when feedback was last generated
	failureThresholds   decision.FailureThresholds
	thresholdCalibrator *decision.ThresholdCalibrator // Persistent calibrator that learns from trade history

	// Advanced optimization systems
	promptOptimizer   *PromptOptimizer
	factorOptimizer   *FactorOptimizer
	complianceTracker *ComplianceTracker

	// Position excursion tracking for MFE/MAE analysis
	excursionsMu sync.RWMutex
	excursions   map[string]*positionExcursion

	// SMART 1.2 & 1.4: Symbol stats and model performance tracking
	symbolStatsMu    sync.RWMutex
	symbolStats      map[string]*SymbolStats // Track win rates per symbol
	modelPerformance *ModelPerformance       // Track model drift for confidence thresholds
}

// positionExcursion tracks maximum favorable/adverse excursion for open positions
type positionExcursion struct {
	entryPrice   float64
	entryTime    int64
	maxFavorable float64 // Best unrealized PnL seen (in USD)
	maxAdverse   float64 // Worst unrealized PnL seen (in USD)
	lastUpdate   int64   // Last price update timestamp
}

// NewRunner constructs a backtest runner.
func NewRunner(cfg BacktestConfig, mcpClient mcp.AIClient) (*Runner, error) {
	if err := ensureRunDir(cfg.RunID); err != nil {
		return nil, err
	}

	client, err := configureMCPClient(cfg, mcpClient)
	if err != nil {
		return nil, err
	}

	feed, err := NewDataFeed(cfg)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(decisionLogDir(cfg.RunID), 0o755); err != nil {
		return nil, err
	}

	dLogDir := decisionLogDir(cfg.RunID)
	account := NewBacktestAccount(cfg.InitialBalance, cfg.FeeBps, cfg.SlippageBps)

	createdAt := time.Now().UTC()
	state := &BacktestState{
		Positions:      make(map[string]PositionSnapshot),
		Cash:           account.Cash(),
		Equity:         cfg.InitialBalance,
		UnrealizedPnL:  0,
		RealizedPnL:    0,
		MaxEquity:      cfg.InitialBalance,
		MinEquity:      cfg.InitialBalance,
		MaxDrawdownPct: 0,
		LastUpdate:     createdAt,
	}

	var (
		aiCache   *AICache
		cachePath string
	)
	if cfg.CacheAI || cfg.ReplayOnly || cfg.SharedAICachePath != "" {
		cachePath = cfg.SharedAICachePath
		if cachePath == "" {
			cachePath = filepath.Join(runDir(cfg.RunID), "ai_cache.json")
		}
		cache, err := LoadAICache(cachePath)
		if err != nil {
			return nil, fmt.Errorf("load ai cache: %w", err)
		}
		aiCache = cache
	}

	// Create strategy engine from backtest config for unified prompt generation
	strategyConfig := cfg.ToStrategyConfig()
	strategyEngine := decision.NewStrategyEngine(strategyConfig)
	// Initialize feedback loop with smart defaults
	enableFeedback := cfg.EnableFeedback
	enableLLM := enableFeedback && cfg.EnableLLMFeedback && client != nil
	feedbackConfig := SmartFeedbackConfig(cfg.DecisionCadenceNBars, enableFeedback, enableLLM)
	feedbackGenerator := NewFeedbackGenerator(cfg.RunID, cfg.InitialBalance, feedbackConfig)

	// Inject AI client for LLM-based feedback evolution
	if client != nil && enableLLM {
		feedbackGenerator.SetAIClient(client)
		logger.Infof("✅ AI client injected into FeedbackGenerator for LLM-based feedback evolution")
	}
	feedbackGenerator.config = feedbackConfig

	failureThresholds := decision.DefaultFailureThresholds()

	// Try to load calibrated thresholds from disk if enabled
	if config.Features().CalibrateOnStartup {
		if loadedJSON, valid, err := config.LoadCalibratedThresholdsJSON(""); err == nil && valid {
			// Convert from JSON to decision.FailureThresholds
			failureThresholds = decision.FailureThresholds{
				WeakVolumeThreshold:      loadedJSON.WeakVolumeThreshold,
				WeakOIThreshold:          loadedJSON.WeakOIThreshold,
				PrematureVolumeThreshold: loadedJSON.PrematureVolumeThreshold,
				PrematureOIThreshold:     loadedJSON.PrematureOIThreshold,
				VolumeDecayThreshold:     loadedJSON.VolumeDecayThreshold,
				OIDecayThreshold:         loadedJSON.OIDecayThreshold,
				SpreadWorseningMultiple:  loadedJSON.SpreadWorseningMultiple,
				DepthReductionThreshold:  loadedJSON.DepthReductionThreshold,
			}
			logger.Infof("applied calibrated failure thresholds from disk")
		} else if err != nil {
			logger.Warnf("failed to load calibrated thresholds: %v", err)
			// Fall back to runtime calibration
			if calibrated, sampleSize, summary, err := calibrateFailureThresholds(cfg.RunID, feedbackGenerator, 500); err != nil {
				logger.Infof("using default failure thresholds (calibration unavailable): %v", err)
			} else {
				failureThresholds = calibrated
				logger.Infof("applied calibrated failure thresholds from %d historical trades", sampleSize)
				logger.Info(summary)
			}
		}
	} else {
		// Offline calibration path
		if calibrated, sampleSize, summary, err := calibrateFailureThresholds(cfg.RunID, feedbackGenerator, 500); err != nil {
			logger.Infof("using default failure thresholds (calibration unavailable): %v", err)
		} else {
			failureThresholds = calibrated
			logger.Infof("applied calibrated failure thresholds from %d historical trades", sampleSize)
			logger.Info(summary)
		}
	}

	feedbackGenerator.SetFailureThresholds(failureThresholds)

	// Initialize advanced optimization systems
	// Use a default system prompt (will be overridden by StrategyEngine)
	defaultPrompt := strategyEngine.GetConfig().PromptSections
	riskcontrolConfig := strategyEngine.GetConfig().RiskControl
	promptOptimizationConfig := DefaultPromptOptimizerConfig()
	enablePromptEvolution := cfg.EnablePromptEvolution
	if enablePromptEvolution {
		promptOptimizationConfig.EnableOptimization = true
	}
	promptOptimizer := NewPromptOptimizerWithAI(&defaultPrompt, promptOptimizationConfig, client, cfg.RunID, cfg.Storage)
	factorOptimizer := NewFactorOptimizer(&riskcontrolConfig, DefaultFactorOptimizerConfig())
	complianceTracker := NewComplianceTracker(DefaultComplianceConfig())
	thresholdCalibrator := decision.NewThresholdCalibrator()

	r := &Runner{
		cfg:                 cfg,
		feed:                feed,
		account:             account,
		strategyEngine:      strategyEngine,
		decisionLogDir:      dLogDir,
		mcpClient:           client,
		status:              RunStateCreated,
		state:               state,
		pauseCh:             make(chan struct{}, 1),
		resumeCh:            make(chan struct{}, 1),
		stopCh:              make(chan struct{}, 1),
		doneCh:              make(chan struct{}),
		createdAt:           createdAt,
		aiCache:             aiCache,
		cachePath:           cachePath,
		feedbackGenerator:   feedbackGenerator,
		feedbackConfig:      feedbackConfig,
		feedbackCycle:       0,
		failureThresholds:   failureThresholds,
		thresholdCalibrator: thresholdCalibrator,
		promptOptimizer:     promptOptimizer,
		factorOptimizer:     factorOptimizer,
		complianceTracker:   complianceTracker,
		excursions:          make(map[string]*positionExcursion),
		// SMART: Initialize tracking fields for adaptive position sizing and confidence thresholds
		symbolStats:      make(map[string]*SymbolStats),
		modelPerformance: &ModelPerformance{},
	}

	if err := r.initLock(); err != nil {
		return nil, err
	}

	return r, nil
}

func (r *Runner) initLock() error {
	if r.cfg.RunID == "" {
		return fmt.Errorf("run_id required for lock")
	}
	info, err := acquireRunLock(r.cfg.RunID)
	if err != nil {
		return err
	}
	r.lockInfo = info
	r.lockStop = make(chan struct{})
	go r.lockHeartbeatLoop()
	return nil
}

func (r *Runner) lockHeartbeatLoop() {
	ticker := time.NewTicker(lockHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := updateRunLockHeartbeat(r.lockInfo); err != nil {
				logger.Infof("failed to update lock heartbeat for %s: %v", r.cfg.RunID, err)
			}
		case <-r.lockStop:
			return
		}
	}
}

// captureMicrostructure extracts execution microstructure data for Trade Failure V2 analysis
// Uses real market data to calculate spread, depth, and slippage budget
// Returns: spread, depth, signalTime, fillTime, slippageBudget
func (r *Runner) captureMicrostructure(symbol string, basePrice, execPrice float64, signalTs int64, marketData *market.Data) (float64, float64, int64, int64, float64) {
	// Calculate spread from actual market microstructure
	spread := calculateActualSpread(marketData, basePrice)

	// Calculate depth from actual volume data
	depth := calculateMarketDepth(marketData, basePrice)

	// Signal time = when decision was made
	signalTime := signalTs

	// Fill time = simulated execution time (immediate in backtest)
	fillTime := signalTs

	// Calculate slippage budget based on actual spread and volatility
	slippageBudget := calculateSlippageBudget(spread, marketData)

	// Verify execution price vs base price to detect slippage
	if basePrice > 0 && execPrice > 0 {
		slippageRatio := math.Abs(execPrice-basePrice) / basePrice
		if slippageRatio > 0.01 { // Log significant slippage (>1%)
			logger.Debugf("[Microstructure] %s: slippage %.2f%% (base=%.2f, exec=%.2f)", symbol, slippageRatio*100, basePrice, execPrice)
		}
	}

	return spread, depth, signalTime, fillTime, slippageBudget
}

// calculateActualSpread computes bid-ask spread from recent price action
// Uses intraday high-low ranges as proxy for spread in backtest environment
func calculateActualSpread(data *market.Data, currentPrice float64) float64 {
	if data == nil || currentPrice <= 0 {
		return 0
	}

	// Try to get spread from recent 1m candles (most granular)
	if data.TimeframeData != nil {
		if tfData, ok := data.TimeframeData["1m"]; ok && len(tfData.Klines) > 0 {
			// Use average high-low spread of last 5 candles
			spreadSum := 0.0
			count := 0
			start := len(tfData.Klines) - 5
			if start < 0 {
				start = 0
			}
			for i := start; i < len(tfData.Klines); i++ {
				k := tfData.Klines[i]
				if k.Close > 0 {
					spreadSum += (k.High - k.Low) / k.Close
					count++
				}
			}
			if count > 0 {
				return spreadSum / float64(count)
			}
		}
		// Fallback to 5m candles
		if tfData, ok := data.TimeframeData["5m"]; ok && len(tfData.Klines) > 0 {
			k := tfData.Klines[len(tfData.Klines)-1]
			if k.Close > 0 {
				return (k.High - k.Low) / k.Close
			}
		}
	}

	// Fallback: use ATR as spread proxy
	atr := calculateATRFromSeries(data)
	if atr > 0 && currentPrice > 0 {
		return atr / currentPrice
	}

	return 0
}

// calculateMarketDepth estimates liquidity depth from volume statistics
// Returns estimated depth in USD that can be absorbed without significant slippage
func calculateMarketDepth(data *market.Data, currentPrice float64) float64 {
	if data == nil || currentPrice <= 0 {
		return 0
	}

	// Use recent volume as proxy for depth
	// Assumption: Market depth ≈ 10-20x average 1-minute volume for liquid markets
	if data.IntradaySeries != nil && len(data.IntradaySeries.Volume) > 0 {
		// Calculate average volume over recent period
		volumeSum := 0.0
		count := 0
		// Use last 15 minutes of volume data
		start := len(data.IntradaySeries.Volume) - 15
		if start < 0 {
			start = 0
		}
		for i := start; i < len(data.IntradaySeries.Volume); i++ {
			volumeSum += data.IntradaySeries.Volume[i]
			count++
		}
		if count > 0 {
			avgVolume := volumeSum / float64(count)
			// Depth = 15x average minute volume (conservative estimate)
			return avgVolume * currentPrice * 15.0
		}
	}

	// Fallback: try timeframe volume data
	if data.TimeframeData != nil {
		if tfData, ok := data.TimeframeData["1m"]; ok && len(tfData.Klines) > 0 {
			volumeSum := 0.0
			count := 0
			start := len(tfData.Klines) - 10
			if start < 0 {
				start = 0
			}
			for i := start; i < len(tfData.Klines); i++ {
				volumeSum += tfData.Klines[i].Volume
				count++
			}
			if count > 0 {
				avgVolume := volumeSum / float64(count)
				return avgVolume * currentPrice * 15.0
			}
		}
	}

	return 0
}

// calculateSlippageBudget determines acceptable slippage based on spread and volatility
// Higher volatility = larger acceptable slippage
func calculateSlippageBudget(spread float64, data *market.Data) float64 {
	if spread <= 0 {
		return 0
	}

	// Base budget: 2x spread
	budget := spread * 2.0

	// Adjust for volatility regime
	if data != nil {
		atr := calculateATRFromSeries(data)
		if atr > 0 && data.CurrentPrice > 0 {
			atrPct := atr / data.CurrentPrice
			// High volatility (>5% ATR): increase budget by 50%
			if atrPct > 0.05 {
				budget *= 1.5
				// Very low volatility (<1% ATR): decrease budget by 30%
			} else if atrPct < 0.01 {
				budget *= 0.7
			}
		}
	}

	return budget
}

func (r *Runner) releaseLock() {
	if r.lockStop != nil {
		close(r.lockStop)
		r.lockStop = nil
	}
	if err := deleteRunLock(r.cfg.RunID); err != nil {
		logger.Infof("failed to release lock for %s: %v", r.cfg.RunID, err)
	}
	r.lockInfo = nil
}

// initExcursion initializes excursion tracking for a new position
func (r *Runner) initExcursion(symbol, side string, entryPrice float64, entryTime int64) {
	r.excursionsMu.Lock()
	defer r.excursionsMu.Unlock()

	key := positionKey(symbol, side)
	r.excursions[key] = &positionExcursion{
		entryPrice:   entryPrice,
		entryTime:    entryTime,
		maxFavorable: 0,
		maxAdverse:   0,
		lastUpdate:   entryTime,
	}
}

// trackExcursions updates MFE/MAE for all open positions based on current prices
// Call this every cycle to capture intra-position excursions
func (r *Runner) trackExcursions(priceMap map[string]float64, ts int64) {
	r.excursionsMu.Lock()
	defer r.excursionsMu.Unlock()

	positions := r.account.Positions()
	for _, pos := range positions {
		key := positionKey(pos.Symbol, pos.Side)
		exc, exists := r.excursions[key]
		if !exists {
			// Position opened before excursion tracking - initialize now
			exc = &positionExcursion{
				entryPrice:   pos.EntryPrice,
				entryTime:    pos.OpenTime,
				maxFavorable: 0,
				maxAdverse:   0,
				lastUpdate:   pos.OpenTime,
			}
			r.excursions[key] = exc
		}

		currentPrice := priceMap[pos.Symbol]
		if currentPrice <= 0 {
			continue
		}

		// Calculate unrealized PnL in USD
		var unrealizedPnL float64
		if pos.Side == "long" {
			unrealizedPnL = (currentPrice - pos.EntryPrice) * pos.Quantity
		} else {
			unrealizedPnL = (pos.EntryPrice - currentPrice) * pos.Quantity
		}

		// Update MFE/MAE
		if unrealizedPnL > exc.maxFavorable {
			exc.maxFavorable = unrealizedPnL
		}
		if unrealizedPnL < exc.maxAdverse {
			exc.maxAdverse = unrealizedPnL
		}

		exc.lastUpdate = ts
	}
}

// getExcursion retrieves and removes excursion data for a closing position
// Returns MFE, MAE in USD (0,0 if not tracked)
func (r *Runner) getExcursion(symbol, side string) (mfe, mae float64) {
	r.excursionsMu.Lock()
	defer r.excursionsMu.Unlock()

	key := positionKey(symbol, side)
	exc, exists := r.excursions[key]
	if !exists {
		return 0, 0
	}

	mfe = exc.maxFavorable
	mae = exc.maxAdverse

	// Clean up - position is closing
	delete(r.excursions, key)

	return mfe, mae
}

// Start launches the backtest loop.
func (r *Runner) Start(ctx context.Context) error {
	r.statusMu.Lock()
	if r.status != RunStateCreated && r.status != RunStatePaused {
		r.statusMu.Unlock()
		return fmt.Errorf("cannot start runner in state %s", r.status)
	}
	r.status = RunStateRunning
	r.statusMu.Unlock()

	go r.loop(ctx)
	return nil
}

// PersistMetadata writes the current snapshot to run.json.
func (r *Runner) PersistMetadata() {
	r.persistMetadata()
}

func (r *Runner) setLastError(err error) {
	r.errMu.Lock()
	defer r.errMu.Unlock()
	if err == nil {
		r.lastError = ""
		return
	}
	r.lastError = err.Error()
}

func (r *Runner) lastErrorString() string {
	r.errMu.RLock()
	defer r.errMu.RUnlock()
	return r.lastError
}

// CurrentMetadata returns the metadata corresponding to the current in-memory state.
func (r *Runner) CurrentMetadata() *RunMetadata {
	state := r.snapshotState()
	meta := r.buildMetadata(state, r.Status())
	meta.CreatedAt = r.createdAt
	meta.UpdatedAt = state.LastUpdate
	return meta
}

// GetFeedbackAnalysis returns the current feedback analysis (if available)
// This includes failure patterns, recommended actions, and top losing trades
func (r *Runner) GetFeedbackAnalysis() *FeedbackAnalysis {
	if r == nil {
		return nil
	}
	return r.lastFeedback
}

// GetPromptOptimizer returns the prompt optimizer for inspecting variants
func (r *Runner) GetPromptOptimizer() *PromptOptimizer {
	if r == nil {
		return nil
	}
	return r.promptOptimizer
}

func (r *Runner) loop(ctx context.Context) {
	defer close(r.doneCh)

	for {
		select {
		case <-ctx.Done():
			r.handleStop(fmt.Errorf("context canceled: %w", ctx.Err()))
			return
		case <-r.stopCh:
			r.handleStop(nil)
			return
		case <-r.pauseCh:
			r.handlePause()
			<-r.resumeCh
			r.resumeFromPause()
		default:
		}

		err := r.stepOnce()
		if errors.Is(err, errBacktestCompleted) {
			r.handleCompletion()
			return
		}
		if errors.Is(err, errLiquidated) {
			r.handleLiquidation()
			return
		}
		if err != nil {
			r.handleFailure(err)
			return
		}
	}
}

func (r *Runner) stepOnce() error {
	state := r.snapshotState()
	if state.BarIndex >= r.feed.DecisionBarCount() {
		return errBacktestCompleted
	}

	ts := r.feed.DecisionTimestamp(state.BarIndex)

	marketData, multiTF, err := r.feed.BuildMarketData(ts)
	if err != nil {
		return err
	}

	priceMap := make(map[string]float64, len(marketData))
	for symbol, data := range marketData {
		priceMap[symbol] = data.CurrentPrice
	}

	// Track MFE/MAE for all open positions at current prices
	r.trackExcursions(priceMap, ts)

	callCount := state.DecisionCycle + 1
	shouldDecide := r.shouldTriggerDecision(state.BarIndex)

	var (
		record          *store.DecisionRecord
		decisionActions []store.DecisionAction
		tradeEvents     = make([]TradeEvent, 0)
		execLog         []string
		hadError        bool
	)

	decisionAttempted := shouldDecide

	if shouldDecide {
		ctx, rec, err := r.buildDecisionContext(ts, marketData, multiTF, priceMap, callCount)
		if err != nil {
			rec.Success = false
			rec.ErrorMessage = fmt.Sprintf("failed to build trading context: %v", err)
			_ = r.logDecision(rec)
			return err
		}
		record = rec

		var (
			fullDecision *decision.FullDecision
			fromCache    bool
			cacheKey     string
		)
		if r.aiCache != nil {
			if key, err := computeCacheKey(ctx, r.cfg.PromptVariant, ts); err == nil {
				cacheKey = key
				if cached, ok := r.aiCache.Get(cacheKey); ok {
					fullDecision = cached
					fromCache = true
				} else if r.cfg.ReplayOnly {
					decisionErr := fmt.Errorf("replay_only enabled but cache miss at %d", ts)
					record.Success = false
					record.ErrorMessage = fmt.Sprintf("cached decision not found for ts=%d", ts)
					_ = r.logDecision(record)
					return decisionErr
				}
			} else {
				logger.Infof("failed to compute ai cache key: %v", err)
			}
		}

		if !fromCache {
			fd, err := r.invokeAIWithRetry(ctx)
			if err != nil {
				decisionAttempted = true
				hadError = true
				record.Success = false
				record.ErrorMessage = fmt.Sprintf("AI decision failed: %v", err)
				execLog = append(execLog, fmt.Sprintf("⚠️ AI decision failed: %v", err))
				r.setLastError(err)
			} else {
				fullDecision = fd
				if r.cfg.CacheAI && r.aiCache != nil && cacheKey != "" {
					if err := r.aiCache.Put(cacheKey, r.cfg.PromptVariant, ts, fullDecision); err != nil {
						logger.Infof("failed to persist ai cache for %s: %v", r.cfg.RunID, err)
					}
				}
			}
		}

		if fullDecision != nil {
			r.fillDecisionRecord(record, fullDecision)
			r.complianceTracker.CheckCompliance(state.DecisionCycle, &fullDecision.Decisions[0], r.lastFeedback)

			sorted := sortDecisionsByPriority(fullDecision.Decisions)

			prevLogs := execLog
			decisionActions = make([]store.DecisionAction, 0, len(sorted))
			execLog = make([]string, 0, len(sorted)+len(prevLogs))
			if len(prevLogs) > 0 {
				execLog = append(execLog, prevLogs...)
			}

			for _, dec := range sorted {
				actionRecord, trades, logEntry, execErr := r.executeDecision(dec, priceMap, ts, callCount)
				if execErr != nil {
					actionRecord.Success = false
					actionRecord.Error = execErr.Error()
					hadError = true
					execLog = append(execLog, fmt.Sprintf("❌ %s %s: %v", dec.Symbol, dec.Action, execErr))
				} else {
					actionRecord.Success = true
					execLog = append(execLog, fmt.Sprintf("✓ %s %s", dec.Symbol, dec.Action))
				}
				if len(trades) > 0 {
					tradeEvents = append(tradeEvents, trades...)
				}
				if logEntry != "" {
					execLog = append(execLog, logEntry)
				}
				decisionActions = append(decisionActions, actionRecord)
			}
		}
	}

	cycleForLog := state.DecisionCycle
	if decisionAttempted {
		cycleForLog = callCount
	}

	liquidationEvents, liquidationNote, err := r.checkLiquidation(ts, priceMap, cycleForLog)
	if err != nil {
		if record != nil {
			record.Success = false
			record.ErrorMessage = err.Error()
			_ = r.logDecision(record)
		}
		return err
	}
	if len(liquidationEvents) > 0 {
		hadError = true
		tradeEvents = append(tradeEvents, liquidationEvents...)
		if record != nil {
			execLog = append(execLog, fmt.Sprintf("⚠️ Forced liquidation: %s", liquidationNote))
		}
	}

	if record != nil {
		record.Decisions = decisionActions
		record.ExecutionLog = execLog
		record.Success = !hadError && liquidationNote == ""
		if liquidationNote != "" {
			record.ErrorMessage = liquidationNote
		}
	}

	equity, unrealized, _ := r.account.TotalEquity(priceMap)
	marginUsed := r.totalMarginUsed()

	r.updateState(ts, equity, unrealized, marginUsed, priceMap, decisionAttempted)

	snapshot := r.snapshotState()
	drawdownPct := CalculateDrawdown(snapshot.Equity, snapshot.MaxEquity)

	equityPoint := EquityPoint{
		Timestamp:   ts,
		Equity:      snapshot.Equity,
		Available:   snapshot.Cash,
		PnL:         snapshot.Equity - r.account.InitialBalance(),
		PnLPct:      ((snapshot.Equity - r.account.InitialBalance()) / r.account.InitialBalance()) * 100,
		DrawdownPct: drawdownPct,
		Cycle:       snapshot.DecisionCycle,
	}

	if err := appendEquityPoint(r.cfg.RunID, equityPoint); err != nil {
		return err
	}

	for _, evt := range tradeEvents {
		if err := appendTradeEvent(r.cfg.RunID, evt); err != nil {
			return err
		}
	}

	if record != nil {
		if err := r.logDecision(record); err != nil {
			return err
		}
	}

	if err := saveProgress(r.cfg.RunID, &snapshot, &r.cfg); err != nil {
		return err
	}

	if err := r.maybeCheckpoint(); err != nil {
		return err
	}

	r.persistMetadata()
	r.persistMetrics(false)

	if !hadError && liquidationNote == "" {
		r.setLastError(nil)
	}

	if snapshot.Liquidated {
		return errLiquidated
	}

	return nil
}

func (r *Runner) buildDecisionContext(ts int64, marketData map[string]*market.Data, multiTF map[string]map[string]*market.Data, priceMap map[string]float64, callCount int) (*decision.Context, *store.DecisionRecord, error) {
	equity, unrealized, _ := r.account.TotalEquity(priceMap)
	available := r.account.Cash()
	marginUsed := r.totalMarginUsed()
	marginPct := 0.0
	if equity > 0 {
		marginPct = (marginUsed / equity) * 100
	}

	accountInfo := decision.AccountInfo{
		TotalEquity:      equity,
		AvailableBalance: available,
		TotalPnL:         equity - r.account.InitialBalance(),
		TotalPnLPct:      ((equity - r.account.InitialBalance()) / r.account.InitialBalance()) * 100,
		MarginUsed:       marginUsed,
		MarginUsedPct:    marginPct,
		PositionCount:    len(r.account.Positions()),
	}

	positions := r.convertPositions(priceMap)

	// Get candidate coins from strategy engine (includes source info)
	candidateCoins, err := r.strategyEngine.GetCandidateCoins()
	if err != nil {
		// Fallback to simple list if strategy engine fails
		candidateCoins = make([]decision.CandidateCoin, 0, len(r.cfg.Symbols))
		for _, sym := range r.cfg.Symbols {
			candidateCoins = append(candidateCoins, decision.CandidateCoin{Symbol: sym, Sources: []string{"backtest"}})
		}
	}

	runtime := int((ts - int64(r.cfg.StartTS*1000)) / 60000)
	ctx := &decision.Context{
		CurrentTime:     time.UnixMilli(ts).UTC().Format("2006-01-02 15:04:05 UTC"),
		RuntimeMinutes:  runtime,
		CallCount:       callCount,
		Account:         accountInfo,
		Positions:       positions,
		CandidateCoins:  candidateCoins,
		PromptVariant:   r.cfg.PromptVariant,
		MarketDataMap:   marketData,
		MultiTFMarket:   multiTF,
		BTCETHLeverage:  r.cfg.Leverage.BTCETHLeverage,
		AltcoinLeverage: r.cfg.Leverage.AltcoinLeverage,
		Timeframes:      r.cfg.Timeframes,
	}

	// Fetch quantitative data if enabled in strategy (uses current data as approximation)
	strategyConfig := r.strategyEngine.GetConfig()
	lang := "en"
	if strings.Contains(strings.ToLower(strategyConfig.PromptSections.RoleDefinition), "交易") {
		lang = "zh"
	}
	if strategyConfig.Indicators.EnableQuantData && strategyConfig.Indicators.QuantDataAPIURL != "" {
		// Collect symbols to query (candidate coins + position coins)
		symbolSet := make(map[string]bool)
		for _, sym := range r.cfg.Symbols {
			symbolSet[sym] = true
		}
		for _, pos := range positions {
			symbolSet[pos.Symbol] = true
		}
		symbols := make([]string, 0, len(symbolSet))
		for sym := range symbolSet {
			symbols = append(symbols, sym)
		}
		ctx.QuantDataMap = r.strategyEngine.FetchQuantDataBatch(symbols)
		if len(ctx.QuantDataMap) > 0 {
			logger.Infof("📊 Backtest: fetched quant data for %d symbols", len(ctx.QuantDataMap))
		}
	}

	// Fetch OI ranking data if enabled in strategy (uses current data as approximation)
	if strategyConfig.Indicators.EnableOIRanking {
		ctx.OIRankingData = r.strategyEngine.FetchOIRankingData()
		if ctx.OIRankingData != nil {
			logger.Infof("📊 Backtest: OI ranking data ready: %d top, %d low positions",
				len(ctx.OIRankingData.TopPositions), len(ctx.OIRankingData.LowPositions))
		}
	}

	record := &store.DecisionRecord{
		AccountState: store.AccountSnapshot{
			TotalBalance:          accountInfo.TotalEquity,
			AvailableBalance:      accountInfo.AvailableBalance,
			TotalUnrealizedProfit: unrealized,
			PositionCount:         accountInfo.PositionCount,
			MarginUsedPct:         accountInfo.MarginUsedPct,
		},
		CandidateCoins: make([]string, 0, len(candidateCoins)),
		Positions:      r.snapshotPositions(priceMap),
		CycleNumber:    callCount, // Set the decision cycle number
	}
	for _, coin := range candidateCoins {
		record.CandidateCoins = append(record.CandidateCoins, coin.Symbol)
	}
	record.Timestamp = time.UnixMilli(ts).UTC()
	// Generate feedback if enabled and enough decisions have been made
	if r.feedbackConfig.EnableFeedback && callCount >= r.feedbackConfig.MinDecisionsForFeedback {
		// Regenerate feedback every FeedbackWindowCycles cycles
		if r.lastFeedback == nil || (callCount-r.feedbackCycle) >= r.feedbackConfig.FeedbackWindowCycles {
			var feedback *FeedbackAnalysis
			var err error
			if r.feedbackGenerator != nil && r.feedbackGenerator.AIClient != nil && (r.feedbackConfig.EnableLLMPatterns || r.feedbackConfig.EnableLLMInsights) {
				feedback, err = r.feedbackGenerator.GenerateFeedbackWithLLM()
			} else {
				feedback, err = r.feedbackGenerator.GenerateFeedback()
			}
			if err != nil {
				logger.Infof("Failed to generate feedback: %v", err)
			} else if feedback != nil {
				r.lastFeedback = feedback
				r.feedbackCycle = callCount
				// Save feedback analysis
				if err := r.feedbackGenerator.SaveFeedbackAnalysis(feedback); err != nil {
					logger.Infof("Failed to save feedback analysis: %v", err)
				}
				logger.Infof("✅ Generated feedback analysis at cycle %d: Total Return %.2f%%, Win Rate %.1f%%",
					callCount, feedback.TotalReturnPct, feedback.WinRate)
				// Calibrate failure thresholds from recent backtest outcomes
				if r.feedbackGenerator != nil {
					if events, err := LoadTradeEvents(r.cfg.RunID); err == nil && len(events) > 0 {
						closed := r.feedbackGenerator.extractClosedPositions(events)
						if len(closed) >= 30 {
							outcomes := make([]decision.TradeOutcome, 0, len(closed))
							for _, pos := range closed {
								outcomes = append(outcomes, tradeOutcomeFromClosedPosition(pos))
							}
							// Keep only the most recent samples for calibration
							if len(outcomes) > 500 {
								outcomes = outcomes[len(outcomes)-500:]
							}
							// Use persistent calibrator (reuse across cycles)
							if err := r.thresholdCalibrator.CalibrateFromHistory(outcomes); err == nil {
								r.failureThresholds = r.thresholdCalibrator.ApplyToAnalyzer()
								logger.Infof("📊 Calibrated backtest failure thresholds from %d trades: %s",
									len(outcomes), r.thresholdCalibrator.GetCalibrationSummary())
							}
						}
					}
				}
				// Optimize factor weights based on feedback
				if r.factorOptimizer.ShouldOptimize(callCount, len(r.account.Positions())) {
					if err := r.factorOptimizer.OptimizeWeights(feedback, callCount); err != nil {
						logger.Infof("Failed to optimize factor weights: %v", err)
					} else {
						// Save optimizer state
						if err := r.factorOptimizer.SaveState(r.cfg.RunID); err != nil {
							logger.Infof("Failed to save factor optimizer state: %v", err)
						}
					}
				}

				// Evolve prompts based on performance
				metrics := &Metrics{
					TotalReturnPct: feedback.TotalReturnPct,
					WinRate:        feedback.WinRate,
					ProfitFactor:   feedback.ProfitFactor,
					SharpeRatio:    feedback.SharpeRatio,
					MaxDrawdownPct: feedback.MaxDrawdown,
				}
				r.cfg.PromptVariant = r.promptOptimizer.GetCurrentVariant().ID
				r.promptOptimizer.RecordDecisionOutcome(r.cfg.PromptVariant, metrics)
				if r.promptOptimizer.ShouldEvolve(callCount) {
					// Use the generic EvolvePrompts method for backtest
					// (Live trading uses meta-prompting via EvolvePromptsWithMetaLearning)
					if err := r.promptOptimizer.EvolvePrompts(r.cfg.PromptVariant); err != nil {
						logger.Infof("Failed to evolve prompts: %v", err)
					} else {
						// Save optimizer state
						if err := r.promptOptimizer.SaveState(r.cfg.RunID); err != nil {
							logger.Infof("Failed to save prompt optimizer state: %v", err)
						}

						// CRITICAL: Update strategy engine with the evolved prompt
						evolvedPrompt := r.promptOptimizer.GetCurrentPrompt()
						r.strategyEngine.SetStrategyPrompt(evolvedPrompt)
						// CRITICAL: Update current prompt variant to evolved one
						r.cfg.PromptVariant = r.promptOptimizer.GetCurrentVariant().ID
						logger.Infof("✅ Applied evolved prompt variant to strategy engine (gen %d)", r.promptOptimizer.GetGeneration())
					}
				}

				// Update compliance tracker with active recommendations
				r.complianceTracker.SetRecommendations(feedback.RecommendedActions)

			}
		}

		// Attach feedback to context
		if r.lastFeedback != nil {
			ctx.PerformanceFeedback = r.feedbackGenerator.FormatFeedbackForPrompt(r.lastFeedback, lang, false)

			// Attach optimized factor weights
			ctx.OptimizedWeights = r.factorOptimizer.GetCurrentWeights()

			// Attach compliance feedback (reinforcement learning)
			lang := "en"
			if strings.Contains(strings.ToLower(r.strategyEngine.GetConfig().PromptSections.RoleDefinition), "交易") {
				lang = "zh"
			}
			ctx.ComplianceFeedback = r.complianceTracker.GetComplianceFeedback(lang)

			// Attach calibrated thresholds (learned risk detection thresholds)
			// Use persistent calibrator (avoids recreating every cycle)
			r.thresholdCalibrator.WeakVolumeThreshold = r.failureThresholds.WeakVolumeThreshold
			r.thresholdCalibrator.WeakOIThreshold = r.failureThresholds.WeakOIThreshold
			r.thresholdCalibrator.PrematureVolumeThreshold = r.failureThresholds.PrematureVolumeThreshold
			r.thresholdCalibrator.PrematureOIThreshold = r.failureThresholds.PrematureOIThreshold
			r.thresholdCalibrator.VolumeDecayThreshold = r.failureThresholds.VolumeDecayThreshold
			r.thresholdCalibrator.OIDecayThreshold = r.failureThresholds.OIDecayThreshold
			r.thresholdCalibrator.SpreadWorseningMultiple = r.failureThresholds.SpreadWorseningMultiple
			r.thresholdCalibrator.DepthReductionThreshold = r.failureThresholds.DepthReductionThreshold
			// Use callCount as approximation for number of trades
			r.thresholdCalibrator.SampleSize = callCount
			ctx.CalibratedThresholds = r.thresholdCalibrator.GetThresholdsForLLM(lang, 35)
		}
	}

	return ctx, record, nil
}

func (r *Runner) fillDecisionRecord(record *store.DecisionRecord, full *decision.FullDecision) {
	record.InputPrompt = full.UserPrompt
	record.CoTTrace = full.CoTTrace
	if len(full.Decisions) > 0 {
		if data, err := json.MarshalIndent(full.Decisions, "", "  "); err == nil {
			record.DecisionJSON = string(data)
		}
	}
}

func (r *Runner) invokeAIWithRetry(ctx *decision.Context) (*decision.FullDecision, error) {
	var lastErr error
	for attempt := 0; attempt < aiDecisionMaxRetries; attempt++ {
		// Use GetFullDecisionWithStrategy with the pre-configured strategy engine
		// This ensures backtest uses the same unified prompt generation as live trading
		fd, err := decision.GetFullDecisionWithStrategy(
			ctx,
			r.mcpClient,
			r.strategyEngine,
		)
		if err == nil {
			return fd, nil
		}
		lastErr = err
		delay := time.Duration(attempt+1) * 500 * time.Millisecond
		time.Sleep(delay)
	}
	return nil, lastErr
}

func (r *Runner) executeDecision(dec decision.Decision, priceMap map[string]float64, ts int64, cycle int) (store.DecisionAction, []TradeEvent, string, error) {
	symbol := dec.Symbol
	usedLeverage := r.resolveLeverage(dec.Leverage, symbol)
	actionRecord := store.DecisionAction{
		Action:    dec.Action,
		Symbol:    symbol,
		Leverage:  usedLeverage,
		Timestamp: time.UnixMilli(ts).UTC(),
	}

	basePrice := priceMap[symbol]
	if basePrice <= 0 {
		return actionRecord, nil, "", fmt.Errorf("price unavailable for %s", symbol)
	}
	fillPrice := r.executionPrice(symbol, basePrice, ts)

	switch dec.Action {
	case "open_long":
		// Get market data for this symbol
		marketData, _, _ := r.feed.BuildMarketData(ts)
		symbolMarketData := marketData[symbol]

		// Use market-aware position sizing if feature flag enabled
		var qty float64
		if r.cfg.UseSmartHeuristics {
			// SMART 1.1-1.4: Use market-aware position sizing with smart heuristics
			qty = r.determineQuantityWithMarketData(dec, basePrice, symbolMarketData)
		} else {
			// Legacy position sizing (hardcoded defaults)
			qty = r.determineQuantity(dec, basePrice)
		}
		if qty <= 0 {
			return actionRecord, nil, "", fmt.Errorf("invalid qty")
		}
		pos, fee, execPrice, err := r.account.Open(symbol, "long", qty, usedLeverage, fillPrice, ts)
		if err != nil {
			return actionRecord, nil, "", err
		}

		// Initialize excursion tracking for this position
		r.initExcursion(symbol, "long", execPrice, ts)

		actionRecord.Quantity = qty
		actionRecord.Price = execPrice
		actionRecord.Leverage = pos.Leverage

		// Capture microstructure data for Trade Failure V2 analysis
		spread, depth, signalTime, fillTime, slippageBudget := r.captureMicrostructure(symbol, basePrice, execPrice, ts, symbolMarketData)

		trade := TradeEvent{
			Timestamp:     ts,
			Symbol:        symbol,
			Action:        dec.Action,
			Side:          "long",
			Quantity:      qty,
			Price:         execPrice,
			Fee:           fee,
			Slippage:      execPrice - basePrice,
			OrderValue:    execPrice * qty,
			RealizedPnL:   0,
			Leverage:      pos.Leverage,
			Cycle:         cycle,
			PositionAfter: pos.Quantity,
			// Microstructure fields
			Spread:         spread,
			Depth:          depth,
			SignalTime:     signalTime,
			FillTime:       fillTime,
			SlippageBudget: slippageBudget,
		}
		return actionRecord, []TradeEvent{trade}, "", nil

	case "open_short":
		// Get market data for this symbol
		marketData, _, _ := r.feed.BuildMarketData(ts)
		symbolMarketData := marketData[symbol]

		// Use market-aware position sizing if feature flag enabled
		var qty float64
		if r.cfg.UseSmartHeuristics {
			// SMART 1.1-1.4: Use market-aware position sizing with smart heuristics
			qty = r.determineQuantityWithMarketData(dec, basePrice, symbolMarketData)
		} else {
			// Legacy position sizing (hardcoded defaults)
			qty = r.determineQuantity(dec, basePrice)
		}
		if qty <= 0 {
			return actionRecord, nil, "", fmt.Errorf("invalid qty")
		}
		pos, fee, execPrice, err := r.account.Open(symbol, "short", qty, usedLeverage, fillPrice, ts)
		if err != nil {
			return actionRecord, nil, "", err
		}

		// Initialize excursion tracking for this position
		r.initExcursion(symbol, "short", execPrice, ts)

		actionRecord.Quantity = qty
		actionRecord.Price = execPrice
		actionRecord.Leverage = pos.Leverage

		// Capture microstructure data for Trade Failure V2 analysis
		spread, depth, signalTime, fillTime, slippageBudget := r.captureMicrostructure(symbol, basePrice, execPrice, ts, symbolMarketData)

		trade := TradeEvent{
			Timestamp:     ts,
			Symbol:        symbol,
			Action:        dec.Action,
			Side:          "short",
			Quantity:      qty,
			Price:         execPrice,
			Fee:           fee,
			Slippage:      basePrice - execPrice,
			OrderValue:    execPrice * qty,
			RealizedPnL:   0,
			Leverage:      pos.Leverage,
			Cycle:         cycle,
			PositionAfter: pos.Quantity,
			// Microstructure fields
			Spread:         spread,
			Depth:          depth,
			SignalTime:     signalTime,
			FillTime:       fillTime,
			SlippageBudget: slippageBudget,
		}
		return actionRecord, []TradeEvent{trade}, "", nil

	case "close_long":
		qty := r.determineCloseQuantity(symbol, "long", dec)
		if qty <= 0 {
			return actionRecord, nil, "", fmt.Errorf("invalid close qty")
		}
		posLev := r.account.positionLeverage(symbol, "long")
		realized, fee, execPrice, err := r.account.Close(symbol, "long", qty, fillPrice)
		if err != nil {
			return actionRecord, nil, "", err
		}

		// SMART 1.2: Record trade outcome for adaptive position sizing
		r.recordTradeOutcome(symbol, realized-fee)

		// Capture MFE/MAE before removing excursion tracking
		mfe, mae := r.getExcursion(symbol, "long")

		actionRecord.Quantity = qty
		actionRecord.Price = execPrice
		actionRecord.Leverage = posLev

		// Get market data for this symbol
		marketData, _, _ := r.feed.BuildMarketData(ts)
		symbolMarketData := marketData[symbol]

		// Capture microstructure data for Trade Failure V2 analysis
		spread, depth, signalTime, fillTime, slippageBudget := r.captureMicrostructure(symbol, basePrice, execPrice, ts, symbolMarketData)

		trade := TradeEvent{
			Timestamp:     ts,
			Symbol:        symbol,
			Action:        dec.Action,
			Side:          "long",
			Quantity:      qty,
			Price:         execPrice,
			Fee:           fee,
			Slippage:      basePrice - execPrice,
			OrderValue:    execPrice * qty,
			RealizedPnL:   realized - fee,
			Leverage:      posLev,
			Cycle:         cycle,
			PositionAfter: r.remainingPosition(symbol, "long"),
			// Microstructure fields
			Spread:         spread,
			Depth:          depth,
			SignalTime:     signalTime,
			FillTime:       fillTime,
			SlippageBudget: slippageBudget,
			// Excursion tracking
			MaxFavorableExcursion: mfe,
			MaxAdverseExcursion:   mae,
		}
		return actionRecord, []TradeEvent{trade}, "", nil

	case "close_short":
		qty := r.determineCloseQuantity(symbol, "short", dec)
		if qty <= 0 {
			return actionRecord, nil, "", fmt.Errorf("invalid close qty")
		}
		posLev := r.account.positionLeverage(symbol, "short")
		realized, fee, execPrice, err := r.account.Close(symbol, "short", qty, fillPrice)
		if err != nil {
			return actionRecord, nil, "", err
		}

		// SMART 1.2: Record trade outcome for adaptive position sizing
		r.recordTradeOutcome(symbol, realized-fee)

		// Capture MFE/MAE before removing excursion tracking
		mfe, mae := r.getExcursion(symbol, "short")

		actionRecord.Quantity = qty
		actionRecord.Price = execPrice
		actionRecord.Leverage = posLev

		// Get market data for this symbol
		marketData, _, _ := r.feed.BuildMarketData(ts)
		symbolMarketData := marketData[symbol]

		// Capture microstructure data for Trade Failure V2 analysis
		spread, depth, signalTime, fillTime, slippageBudget := r.captureMicrostructure(symbol, basePrice, execPrice, ts, symbolMarketData)

		trade := TradeEvent{
			Timestamp:     ts,
			Symbol:        symbol,
			Action:        dec.Action,
			Side:          "short",
			Quantity:      qty,
			Price:         execPrice,
			Fee:           fee,
			Slippage:      execPrice - basePrice,
			OrderValue:    execPrice * qty,
			RealizedPnL:   realized - fee,
			Leverage:      posLev,
			Cycle:         cycle,
			PositionAfter: r.remainingPosition(symbol, "short"),
			// Microstructure fields
			Spread:         spread,
			Depth:          depth,
			SignalTime:     signalTime,
			FillTime:       fillTime,
			SlippageBudget: slippageBudget,
			// Excursion tracking
			MaxFavorableExcursion: mfe,
			MaxAdverseExcursion:   mae,
		}
		return actionRecord, []TradeEvent{trade}, "", nil

	case "hold", "wait":
		return actionRecord, nil, fmt.Sprintf("hold position: %s", dec.Action), nil
	default:
		return actionRecord, nil, "", fmt.Errorf("unsupported action %s", dec.Action)
	}
}

func (r *Runner) determineQuantity(dec decision.Decision, price float64) float64 {
	snapshot := r.snapshotState()
	equity := snapshot.Equity
	if equity <= 0 {
		equity = r.account.InitialBalance()
	}

	// Get leverage for this symbol
	leverage := r.resolveLeverage(dec.Leverage, dec.Symbol)
	if leverage <= 0 {
		// SMART 1.1: Dynamic leverage based on market conditions
		// TODO: Replace with: leverage = CalculateOptimalLeverage(dec.Symbol, marketData, equity)
		leverage = 5 // Current fallback; volatility-aware version coming
	}

	// Calculate available margin
	availableCash := r.account.Cash()
	// SMART 1.4: Smart margin allowance - currently static 90%
	// TODO: Replace with: marginBudget := CalculateMaxMarginAllowance(snapshot, marketData)
	maxMarginBudget := 0.9 // Will be market-aware (drawdown, position count, volatility adjusted)
	maxMarginToUse := availableCash * maxMarginBudget
	maxPositionValue := maxMarginToUse * float64(leverage)

	sizeUSD := dec.PositionSizeUSD
	if sizeUSD <= 0 {
		// SMART 1.2: Adaptive position sizing based on account state and market conditions
		// TODO: Replace with: sizeUSD := CalculateAdaptivePositionSize(dec.Symbol, dec.Confidence, snapshot, marketData, recentStats)
		sizeUSD = 0.05 * equity // Current 5% default; will become market-aware
	}

	// Cap position size to what we can actually afford
	if sizeUSD > maxPositionValue {
		logger.Infof("📊 Backtest: capping position from %.2f to %.2f (available margin: %.2f, leverage: %dx)",
			sizeUSD, maxPositionValue, maxMarginToUse, leverage)
		sizeUSD = maxPositionValue
	}

	qty := sizeUSD / price
	if qty < 0 {
		qty = 0
	}
	return qty
}

// determineQuantityWithMarketData calculates position size using smart heuristics (SMART 1.1-1.4)
// This is the market-aware version that will eventually replace determineQuantity
func (r *Runner) determineQuantityWithMarketData(dec decision.Decision, price float64, marketData *market.Data) float64 {
	snapshot := r.snapshotState()
	equity := snapshot.Equity
	if equity <= 0 {
		equity = r.account.InitialBalance()
	}

	// Convert BacktestState to AccountSnapshot for smart heuristics
	accountSnapshot := r.snapshotToAccountSnapshot(snapshot)

	// SMART 1.1: Dynamic leverage based on volatility and symbol
	var leverage int
	if dec.Leverage > 0 {
		leverage = dec.Leverage
	} else if marketData != nil {
		leverage = CalculateOptimalLeverage(dec.Symbol, marketData, equity)
	} else {
		leverage = r.resolveLeverage(0, dec.Symbol)
		if leverage <= 0 {
			leverage = 5
		}
	}

	// Calculate available margin
	availableCash := r.account.Cash()

	// SMART 1.4: Smart margin allowance based on account state and volatility
	var maxMarginBudget float64
	if marketData != nil {
		maxMarginBudget = CalculateMaxMarginAllowance(accountSnapshot, marketData)
	} else {
		maxMarginBudget = 0.9 // Fallback to default
	}

	maxMarginToUse := availableCash * maxMarginBudget
	maxPositionValue := maxMarginToUse * float64(leverage)

	// SMART 1.2: Adaptive position sizing
	var sizeUSD float64
	if dec.PositionSizeUSD > 0 {
		sizeUSD = dec.PositionSizeUSD
	} else {
		// Get symbol stats for position sizing decision
		symbolStats := r.getSymbolStats(dec.Symbol)

		if marketData != nil {
			sizeUSD = CalculateAdaptivePositionSize(
				dec.Symbol,
				dec.Confidence,
				accountSnapshot,
				marketData,
				symbolStats,
			)
		} else {
			// Fallback to 5% default
			sizeUSD = 0.05 * equity
		}
	}

	// Cap position size to what we can actually afford
	if sizeUSD > maxPositionValue {
		logger.Debugf("📊 Backtest: capping smart position from %.2f to %.2f (leverage: %dx)",
			sizeUSD, maxPositionValue, leverage)
		sizeUSD = maxPositionValue
	}

	qty := sizeUSD / price
	if qty < 0 {
		qty = 0
	}
	return qty
}

// snapshotToAccountSnapshot converts BacktestState to AccountSnapshot for smart heuristics
func (r *Runner) snapshotToAccountSnapshot(state BacktestState) *AccountSnapshot {
	positions := make([]PositionSnapshot, 0, len(state.Positions))
	for _, pos := range state.Positions {
		positions = append(positions, pos)
	}

	// Calculate current drawdown
	initialEquity := r.cfg.InitialBalance
	currentDrawdown := state.Equity - state.MaxEquity
	if state.MaxEquity <= 0 {
		currentDrawdown = 0
	}

	return &AccountSnapshot{
		Equity:          state.Equity,
		Cash:            state.Cash,
		Positions:       positions,
		CurrentDrawdown: currentDrawdown,
		MaxDrawdown:     state.MinEquity - initialEquity,
		DailyPnL:        state.RealizedPnL, // Simplified - would need actual daily tracking
		RecentTrades:    []TradeRecord{},   // TODO: Implement recent trade tracking
	}
}

func (r *Runner) determineCloseQuantity(symbol, side string, dec decision.Decision) float64 {
	for _, pos := range r.account.Positions() {
		if pos.Symbol == strings.ToUpper(symbol) && pos.Side == side {
			// Validate that decision reason supports closing this position
			if dec.Reasoning != "" && pos.Quantity > 0 {
				logger.Debugf("[CloseQuantity] Closing %s %s %.4f qty (reason: %s)",
					pos.Symbol, pos.Side, pos.Quantity, dec.Reasoning)
			}
			return pos.Quantity
		}
	}
	return 0
}

func (r *Runner) resolveLeverage(requested int, symbol string) int {
	if requested > 0 {
		return requested
	}
	sym := strings.ToUpper(symbol)
	if sym == "BTCUSDT" || sym == "ETHUSDT" {
		if r.cfg.Leverage.BTCETHLeverage > 0 {
			return r.cfg.Leverage.BTCETHLeverage
		}
	} else {
		if r.cfg.Leverage.AltcoinLeverage > 0 {
			return r.cfg.Leverage.AltcoinLeverage
		}
	}
	return 5
}

func (r *Runner) remainingPosition(symbol, side string) float64 {
	for _, pos := range r.account.Positions() {
		if pos.Symbol == strings.ToUpper(symbol) && pos.Side == side {
			return pos.Quantity
		}
	}
	return 0
}

func (r *Runner) snapshotPositions(priceMap map[string]float64) []store.PositionSnapshot {
	positions := r.account.Positions()
	list := make([]store.PositionSnapshot, 0, len(positions))
	for _, pos := range positions {
		price := priceMap[pos.Symbol]
		list = append(list, store.PositionSnapshot{
			Symbol:           pos.Symbol,
			Side:             pos.Side,
			PositionAmt:      pos.Quantity,
			EntryPrice:       pos.EntryPrice,
			MarkPrice:        price,
			UnrealizedProfit: unrealizedPnL(pos, price),
			Leverage:         float64(pos.Leverage),
			LiquidationPrice: pos.LiquidationPrice,
		})
	}
	return list
}

func (r *Runner) convertPositions(priceMap map[string]float64) []decision.PositionInfo {
	positions := r.account.Positions()
	list := make([]decision.PositionInfo, 0, len(positions))
	for _, pos := range positions {
		price := priceMap[pos.Symbol]
		list = append(list, decision.PositionInfo{
			Symbol:           pos.Symbol,
			Side:             pos.Side,
			EntryPrice:       pos.EntryPrice,
			MarkPrice:        price,
			Quantity:         pos.Quantity,
			Leverage:         pos.Leverage,
			UnrealizedPnL:    unrealizedPnL(pos, price),
			UnrealizedPnLPct: 0,
			LiquidationPrice: pos.LiquidationPrice,
			MarginUsed:       pos.Margin,
			UpdateTime:       time.Now().UnixMilli(),
		})
	}
	return list
}

func (r *Runner) executionPrice(symbol string, markPrice float64, ts int64) float64 {
	curr, next := r.feed.decisionBarSnapshot(symbol, ts)
	switch r.cfg.FillPolicy {
	case FillPolicyNextOpen:
		if next != nil && next.Open > 0 {
			return next.Open
		}
	case FillPolicyBarVWAP:
		if curr != nil {
			if vwap := barVWAP(*curr); vwap > 0 {
				return vwap
			}
		}
	case FillPolicyMidPrice:
		if curr != nil && curr.High > 0 && curr.Low > 0 {
			return (curr.High + curr.Low) / 2
		}
	}
	return markPrice
}

func (r *Runner) totalMarginUsed() float64 {
	sum := 0.0
	for _, pos := range r.account.Positions() {
		sum += pos.Margin
	}
	return sum
}

func (r *Runner) updateState(ts int64, equity, unrealized, marginUsed float64, priceMap map[string]float64, advancedDecision bool) {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()

	if r.state.MaxEquity == 0 || equity > r.state.MaxEquity {
		r.state.MaxEquity = equity
	}
	if r.state.MinEquity == 0 || equity < r.state.MinEquity {
		r.state.MinEquity = equity
	}
	if r.state.MaxEquity > 0 {
		drawdown := CalculateDrawdown(equity, r.state.MaxEquity)
		if drawdown > r.state.MaxDrawdownPct {
			r.state.MaxDrawdownPct = drawdown
		}
	}

	positions := make(map[string]PositionSnapshot)
	for _, pos := range r.account.Positions() {
		key := fmt.Sprintf("%s:%s", pos.Symbol, pos.Side)
		// Use priceMap to check current market prices for positions
		if len(priceMap) > 0 {
			if price, ok := priceMap[pos.Symbol]; ok && price > 0 {
				// Price data available for this position
				_ = price // Use price for future enhancements like real-time PnL calc
			}
		}
		positions[key] = PositionSnapshot{
			Symbol:           pos.Symbol,
			Side:             pos.Side,
			Quantity:         pos.Quantity,
			AvgPrice:         pos.EntryPrice,
			Leverage:         pos.Leverage,
			LiquidationPrice: pos.LiquidationPrice,
			MarginUsed:       pos.Margin,
			OpenTime:         pos.OpenTime,
			AccumulatedFee:   pos.AccumulatedFee,
		}
	}

	r.state.BarTimestamp = ts
	r.state.BarIndex++
	if advancedDecision {
		r.state.DecisionCycle++
	}
	r.state.Cash = r.account.Cash()
	r.state.Equity = equity
	r.state.UnrealizedPnL = unrealized
	// Track margin utilization for risk monitoring
	if marginUsed > 0 && equity > 0 {
		marginUtilization := marginUsed / equity
		if marginUtilization > 0.9 { // Log high margin usage (>90%)
			logger.Warnf("[State] High margin utilization: %.1f%% (%.2f/%.2f)", marginUtilization*100, marginUsed, equity)
		}
	}
	r.state.RealizedPnL = r.account.RealizedPnL()
	r.state.Positions = positions
	r.state.LastUpdate = time.Now().UTC()
}

func (r *Runner) maybeCheckpoint() error {
	state := r.snapshotState()
	shouldCheckpoint := r.cfg.CheckpointIntervalBars > 0 && state.BarIndex > 0 && state.BarIndex%r.cfg.CheckpointIntervalBars == 0

	interval := time.Duration(r.cfg.CheckpointIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if time.Since(r.lastCheckpoint) >= interval {
		shouldCheckpoint = true
	}

	if !shouldCheckpoint {
		return nil
	}

	if err := r.saveCheckpoint(state); err != nil {
		return err
	}

	return nil
}

func (r *Runner) snapshotForCheckpoint(state BacktestState) []PositionSnapshot {
	res := make([]PositionSnapshot, 0, len(state.Positions))
	for _, pos := range state.Positions {
		res = append(res, pos)
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].Symbol == res[j].Symbol {
			return res[i].Side < res[j].Side
		}
		return res[i].Symbol < res[j].Symbol
	})
	return res
}

func (r *Runner) checkLiquidation(ts int64, priceMap map[string]float64, cycle int) ([]TradeEvent, string, error) {
	positions := append([]*position(nil), r.account.Positions()...)
	events := make([]TradeEvent, 0)
	var noteBuilder strings.Builder

	for _, pos := range positions {
		price := priceMap[pos.Symbol]
		liqPrice := pos.LiquidationPrice
		trigger := false
		execPrice := price
		if pos.Side == "long" {
			if price <= liqPrice && liqPrice > 0 {
				trigger = true
				execPrice = liqPrice
			}
		} else {
			if price >= liqPrice && liqPrice > 0 {
				trigger = true
				execPrice = liqPrice
			}
		}
		if !trigger {
			continue
		}

		realized, fee, finalPrice, err := r.account.Close(pos.Symbol, pos.Side, pos.Quantity, execPrice)
		if err != nil {
			return nil, "", err
		}

		noteBuilder.WriteString(fmt.Sprintf("%s %s @ %.4f; ", pos.Symbol, pos.Side, finalPrice))

		evt := TradeEvent{
			Timestamp:       ts,
			Symbol:          pos.Symbol,
			Action:          "liquidated",
			Side:            pos.Side,
			Quantity:        pos.Quantity,
			Price:           finalPrice,
			Fee:             fee,
			Slippage:        0,
			OrderValue:      finalPrice * pos.Quantity,
			RealizedPnL:     realized - fee,
			Leverage:        pos.Leverage,
			Cycle:           cycle,
			PositionAfter:   0,
			LiquidationFlag: true,
			Note:            fmt.Sprintf("forced liquidation at %.4f", finalPrice),
		}
		events = append(events, evt)
	}

	if len(events) == 0 {
		return events, "", nil
	}

	note := strings.TrimSuffix(noteBuilder.String(), "; ")

	r.stateMu.Lock()
	r.state.Liquidated = true
	r.state.LiquidationNote = note
	r.stateMu.Unlock()

	return events, note, nil
}

func (r *Runner) shouldTriggerDecision(barIndex int) bool {
	if r.cfg.DecisionCadenceNBars <= 1 {
		return true
	}
	if barIndex < 0 {
		return true
	}
	return barIndex%r.cfg.DecisionCadenceNBars == 0
}

func (r *Runner) handleStop(reason error) {
	r.forceCheckpoint()
	if reason != nil {
		r.setLastError(reason)
	} else {
		r.setLastError(nil)
	}
	r.statusMu.Lock()
	r.err = reason
	r.status = RunStateStopped
	r.statusMu.Unlock()
	r.persistMetadata()
	r.persistMetrics(true)
	r.releaseLock()
}

func (r *Runner) handlePause() {
	r.forceCheckpoint()
	r.setLastError(nil)
	r.statusMu.Lock()
	r.status = RunStatePaused
	r.statusMu.Unlock()
	r.persistMetadata()
	r.persistMetrics(true)
}

func (r *Runner) resumeFromPause() {
	r.setLastError(nil)
	r.statusMu.Lock()
	r.status = RunStateRunning
	r.statusMu.Unlock()
	r.persistMetadata()
}

func (r *Runner) handleCompletion() {
	r.setLastError(nil)
	r.statusMu.Lock()
	r.status = RunStateCompleted
	r.statusMu.Unlock()
	r.persistMetadata()
	r.persistMetrics(true)
	r.releaseLock()
}

func (r *Runner) handleFailure(err error) {
	r.forceCheckpoint()
	if err != nil {
		r.setLastError(err)
	}
	r.statusMu.Lock()
	r.err = err
	r.status = RunStateFailed
	r.statusMu.Unlock()
	r.persistMetadata()
	r.persistMetrics(true)
	r.releaseLock()
}

func (r *Runner) handleLiquidation() {
	r.forceCheckpoint()
	r.setLastError(errLiquidated)
	r.statusMu.Lock()
	r.err = errLiquidated
	r.status = RunStateLiquidated
	r.statusMu.Unlock()
	r.persistMetadata()
	r.persistMetrics(true)
	r.releaseLock()
}

func (r *Runner) Pause() {
	select {
	case r.pauseCh <- struct{}{}:
	default:
	}
}

func (r *Runner) Resume() {
	select {
	case r.resumeCh <- struct{}{}:
	default:
	}
}

func (r *Runner) Stop() {
	select {
	case r.stopCh <- struct{}{}:
	default:
	}
}

func (r *Runner) Wait() error {
	<-r.doneCh
	r.statusMu.RLock()
	defer r.statusMu.RUnlock()
	return r.err
}

// Status returns the current run state.
func (r *Runner) Status() RunState {
	r.statusMu.RLock()
	defer r.statusMu.RUnlock()
	return r.status
}

// StatusPayload builds the status response for the API.
func (r *Runner) StatusPayload() StatusPayload {
	snapshot := r.snapshotState()
	progress := progressPercent(snapshot, r.cfg)

	// Build position statuses with unrealized P&L
	positions := make([]PositionStatus, 0, len(snapshot.Positions))
	for _, pos := range snapshot.Positions {
		if pos.Quantity <= 0 {
			continue
		}
		// Get mark price from feed if available
		markPrice := pos.AvgPrice // fallback to entry price
		if r.feed != nil && snapshot.BarTimestamp > 0 {
			if md, _, err := r.feed.BuildMarketData(snapshot.BarTimestamp); err == nil {
				if data, ok := md[pos.Symbol]; ok {
					markPrice = data.CurrentPrice
				}
			}
		}

		// Calculate unrealized P&L
		var unrealizedPnL float64
		if pos.Side == "long" {
			unrealizedPnL = (markPrice - pos.AvgPrice) * pos.Quantity
		} else {
			unrealizedPnL = (pos.AvgPrice - markPrice) * pos.Quantity
		}

		// Calculate P&L percentage based on margin
		pnlPct := 0.0
		if pos.MarginUsed > 0 {
			pnlPct = (unrealizedPnL / pos.MarginUsed) * 100
		}

		positions = append(positions, PositionStatus{
			Symbol:           pos.Symbol,
			Side:             pos.Side,
			Quantity:         pos.Quantity,
			EntryPrice:       pos.AvgPrice,
			MarkPrice:        markPrice,
			Leverage:         pos.Leverage,
			UnrealizedPnL:    unrealizedPnL,
			UnrealizedPnLPct: pnlPct,
			MarginUsed:       pos.MarginUsed,
		})
	}

	payload := StatusPayload{
		RunID:          r.cfg.RunID,
		State:          r.Status(),
		ProgressPct:    progress,
		ProcessedBars:  snapshot.BarIndex,
		CurrentTime:    snapshot.BarTimestamp,
		DecisionCycle:  snapshot.DecisionCycle,
		Equity:         snapshot.Equity,
		UnrealizedPnL:  snapshot.UnrealizedPnL,
		RealizedPnL:    snapshot.RealizedPnL,
		Positions:      positions,
		Note:           snapshot.LiquidationNote,
		LastError:      r.lastErrorString(),
		LastUpdatedIso: snapshot.LastUpdate.UTC().Format(time.RFC3339),
	}
	return payload
}

func (r *Runner) snapshotState() BacktestState {
	r.stateMu.RLock()
	defer r.stateMu.RUnlock()

	copyState := *r.state
	copyState.Positions = make(map[string]PositionSnapshot, len(r.state.Positions))
	for k, v := range r.state.Positions {
		copyState.Positions[k] = v
	}
	return copyState
}

// recordTradeOutcome records a trade result for symbol performance tracking (SMART 1.2)
// Note: Extended tracking of symbol statistics for position sizing decisions
func (r *Runner) recordTradeOutcome(symbol string, pnl float64) {
	r.symbolStatsMu.Lock()
	defer r.symbolStatsMu.Unlock()

	if _, exists := r.symbolStats[symbol]; !exists {
		r.symbolStats[symbol] = &SymbolStats{
			WinRate:    0.5, // Default 50%
			AvgProfit:  0,
			MaxLoss:    0,
			SampleSize: 0,
		}
	}

	stats := r.symbolStats[symbol]
	stats.SampleSize++

	if pnl > 0 {
		// Update win rate using weighted average
		stats.WinRate = (stats.WinRate*(float64(stats.SampleSize-1)) + 1.0) / float64(stats.SampleSize)
		stats.AvgProfit = (stats.AvgProfit*(float64(stats.SampleSize-1)) + pnl) / float64(stats.SampleSize)
	} else if pnl < 0 {
		// Update win rate using weighted average
		stats.WinRate = (stats.WinRate * float64(stats.SampleSize-1)) / float64(stats.SampleSize)
		if pnl < stats.MaxLoss {
			stats.MaxLoss = pnl
		}
	}
}

// getSymbolStats returns statistics for a symbol, used in position sizing calculations
func (r *Runner) getSymbolStats(symbol string) *SymbolStats {
	r.symbolStatsMu.RLock()
	defer r.symbolStatsMu.RUnlock()

	if stats, exists := r.symbolStats[symbol]; exists {
		return stats
	}
	// Return empty stats with 50% default win rate if symbol not yet tracked
	return &SymbolStats{WinRate: 0.5, SampleSize: 0}
}

func (r *Runner) persistMetadata() {
	state := r.snapshotState()
	meta := r.buildMetadata(state, r.Status())
	meta.CreatedAt = r.createdAt
	if err := SaveRunMetadata(meta); err != nil {
		logger.Infof("failed to save run metadata for %s: %v", r.cfg.RunID, err)
	} else {
		if err := updateRunIndex(meta, &r.cfg); err != nil {
			logger.Infof("failed to update index for %s: %v", r.cfg.RunID, err)
		}
	}
}

func (r *Runner) logDecision(record *store.DecisionRecord) error {
	if record == nil {
		return nil
	}
	persistDecisionRecord(r.cfg.RunID, record)
	return nil
}

func (r *Runner) persistMetrics(force bool) {
	if r.cfg.RunID == "" {
		return
	}

	if !force && !r.lastMetricsWrite.IsZero() {
		if time.Since(r.lastMetricsWrite) < metricsWriteInterval {
			return
		}
	}

	state := r.snapshotState()
	metrics, err := CalculateMetrics(r.cfg.RunID, &r.cfg, &state)
	if err != nil {
		logger.Infof("failed to compute metrics for %s: %v", r.cfg.RunID, err)
		return
	}
	if metrics == nil {
		return
	}
	if err := PersistMetrics(r.cfg.RunID, metrics); err != nil {
		logger.Infof("failed to persist metrics for %s: %v", r.cfg.RunID, err)
		return
	}
	r.lastMetricsWrite = time.Now()
}

func (r *Runner) buildMetadata(state BacktestState, runState RunState) *RunMetadata {
	if state.Liquidated && runState != RunStateLiquidated {
		runState = RunStateLiquidated
	}

	progress := progressPercent(state, r.cfg)

	summary := RunSummary{
		SymbolCount:     len(r.cfg.Symbols),
		DecisionTF:      r.cfg.DecisionTimeframe,
		ProcessedBars:   state.BarIndex,
		ProgressPct:     progress,
		EquityLast:      state.Equity,
		MaxDrawdownPct:  state.MaxDrawdownPct,
		Liquidated:      state.Liquidated,
		LiquidationNote: state.LiquidationNote,
	}

	meta := &RunMetadata{
		RunID:     r.cfg.RunID,
		UserID:    r.cfg.UserID,
		State:     runState,
		LastError: r.lastErrorString(),
		Summary:   summary,
	}

	return meta
}

func progressPercent(state BacktestState, cfg BacktestConfig) float64 {
	duration := cfg.Duration()
	if duration <= 0 {
		return 0
	}
	if state.BarTimestamp == 0 {
		return 0
	}

	start := time.Unix(cfg.StartTS, 0)
	end := time.Unix(cfg.EndTS, 0)
	current := time.UnixMilli(state.BarTimestamp)

	if !current.After(start) {
		return 0
	}
	if current.After(end) {
		return 100
	}

	elapsed := current.Sub(start)
	pct := float64(elapsed) / float64(duration) * 100
	if pct > 100 {
		pct = 100
	}
	if pct < 0 {
		pct = 0
	}
	return pct
}

func (r *Runner) buildCheckpointFromState(state BacktestState) *Checkpoint {
	return &Checkpoint{
		BarIndex:        state.BarIndex,
		BarTimestamp:    state.BarTimestamp,
		Cash:            state.Cash,
		Equity:          state.Equity,
		UnrealizedPnL:   state.UnrealizedPnL,
		RealizedPnL:     state.RealizedPnL,
		Positions:       r.snapshotForCheckpoint(state),
		DecisionCycle:   state.DecisionCycle,
		Liquidated:      state.Liquidated,
		LiquidationNote: state.LiquidationNote,
		MaxEquity:       state.MaxEquity,
		MinEquity:       state.MinEquity,
		MaxDrawdownPct:  state.MaxDrawdownPct,
		AICacheRef:      r.cachePath,
	}
}

func (r *Runner) saveCheckpoint(state BacktestState) error {
	ckpt := r.buildCheckpointFromState(state)
	if ckpt == nil {
		return nil
	}
	if err := SaveCheckpoint(r.cfg.RunID, ckpt); err != nil {
		return err
	}
	r.lastCheckpoint = time.Now()
	return nil
}

func (r *Runner) forceCheckpoint() {
	state := r.snapshotState()
	if err := r.saveCheckpoint(state); err != nil {
		logger.Infof("failed to save checkpoint for %s: %v", r.cfg.RunID, err)
	}
}

func (r *Runner) RestoreFromCheckpoint() error {
	ckpt, err := LoadCheckpoint(r.cfg.RunID)
	if err != nil {
		return err
	}
	return r.applyCheckpoint(ckpt)
}

func (r *Runner) applyCheckpoint(ckpt *Checkpoint) error {
	if ckpt == nil {
		return fmt.Errorf("checkpoint is nil")
	}
	r.account.RestoreFromSnapshots(ckpt.Cash, ckpt.RealizedPnL, ckpt.Positions)
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	r.state.BarIndex = ckpt.BarIndex
	r.state.BarTimestamp = ckpt.BarTimestamp
	r.state.Cash = ckpt.Cash
	r.state.Equity = ckpt.Equity
	r.state.UnrealizedPnL = ckpt.UnrealizedPnL
	r.state.RealizedPnL = ckpt.RealizedPnL
	r.state.DecisionCycle = ckpt.DecisionCycle
	r.state.Liquidated = ckpt.Liquidated
	r.state.LiquidationNote = ckpt.LiquidationNote
	r.state.MaxEquity = ckpt.MaxEquity
	r.state.MinEquity = ckpt.MinEquity
	r.state.MaxDrawdownPct = ckpt.MaxDrawdownPct
	r.state.Positions = snapshotsToMap(ckpt.Positions)
	r.state.LastUpdate = time.Now().UTC()
	r.lastCheckpoint = time.Now()
	return nil
}

func snapshotsToMap(snaps []PositionSnapshot) map[string]PositionSnapshot {
	positions := make(map[string]PositionSnapshot, len(snaps))
	for _, snap := range snaps {
		key := fmt.Sprintf("%s:%s", snap.Symbol, snap.Side)
		positions[key] = snap
	}
	return positions
}

func sortDecisionsByPriority(decisions []decision.Decision) []decision.Decision {
	if len(decisions) <= 1 {
		return decisions
	}

	priority := func(action string) int {
		switch action {
		case "close_long", "close_short":
			return 1
		case "open_long", "open_short":
			return 2
		case "hold", "wait":
			return 3
		default:
			return 99
		}
	}

	result := make([]decision.Decision, len(decisions))
	copy(result, decisions)

	sort.Slice(result, func(i, j int) bool {
		pi := priority(result[i].Action)
		pj := priority(result[j].Action)
		if pi != pj {
			return pi < pj
		}
		return i < j
	})

	return result
}

func barVWAP(k market.Kline) float64 {
	values := []float64{k.Open, k.High, k.Low, k.Close}
	sum := 0.0
	count := 0.0
	for _, v := range values {
		if v > 0 {
			sum += v
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / count
}
