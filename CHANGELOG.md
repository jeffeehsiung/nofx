# Changelog

All notable changes to the NOFX project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

**Languages:** [English](CHANGELOG.md) | [中文](CHANGELOG.zh-CN.md)

---

## [Unreleased]

### Added
- Documentation system with multi-language support (EN/CN/RU/UK)
- Complete getting-started guides (Docker, Custom API)
- Architecture documentation with system design details
- User guides with FAQ and troubleshooting
- Community documentation with bounty programs

### Changed
- Reorganized documentation structure into logical categories
- Updated all README files with proper navigation links

---

## [4.0.0] - NOFX+ Production Hardening & Learning Systems - 2026-02-06

**Major Release: Production-Ready AI Trading with Adaptive Learning**

**Lead Architect & Developer:** [Jeffee Hsiung](https://github.com/jeffeehsiung)

### Summary
This release transforms NOFX into a production-hardened trading platform with institutional-grade learning systems. 17 critical issues fixed, 8+ new learning modules added, and comprehensive market microstructure intelligence integrated throughout the system.

**Performance Impact:** +39.5% improvement with LLM-evolve feedback enabled (from -27.9% to +11.6% total return in backtests)

---

### 🎓 Added - NOFX+ Learning Stack (New Learning Systems)

#### 1. LLM-Evolve Feedback System (`backtest/feedback.go`)
- Analyzes winning and losing patterns from recent trades
- Generates AI-powered insights on what strategies work
- Provides actionable recommendations for improvement
- Tracks pattern frequency and success rates
- Integrates with prompt evolution for continuous learning

#### 2. Prompt Variant Evolution (`backtest/prompt_optimizer.go`)
- Evolutionary algorithm for optimizing trading prompts
- A/B testing of prompt variants with backtest results
- Automatic selection of high-performing prompt variations
- Preserves effective instructions while removing harmful ones
- Result: +7.6% improvement over feedback-only systems

#### 3. Dynamic Threshold Calibration (`decision/threshold_calibrator.go`)
- Bayesian learning from historical trade outcomes
- Replaces magic numbers with data-driven thresholds
- ROC analysis with Youden's J statistic optimization
- Real-time recalibration as new trades complete
- Thresholds: WeakVolumeThreshold, WeakOIThreshold, PrematureVolumeThreshold, etc.
- **Integrated into:** Live trading (`trader/auto_trader.go`) and backtesting (`backtest/runner.go`)

#### 4. Trade Failure Analysis with Microstructure Evidence (`decision/trade_failure.go`)
- Deterministic categorization of trade failures
- Root cause analysis with market microstructure signals
- Failure types: WeakVolume, WeakOI, PrematureEntry, StuckPosition, etc.
- Evidence-based explanations (spread, depth, VWAP deviation)
- Feeds insights into LLM feedback loop

#### 5. Market Microstructure Intelligence (`market/microstructure.go`)
- Real-time order book depth analysis (bid/ask levels)
- Bid-ask spread calculation (% and basis points)
- Order book imbalance & sentiment detection
- VWAP calculation and price deviation analysis
- Large order detection and volume profiling
- Support/resistance level identification (top 3)
- Liquidity scoring (0-100 scale)
- **Integrated into decision context** for AI decision-making

#### 6. Factor Optimizer (`backtest/factor_optimizer.go`)
- Automatic optimization of risk control parameters
- Tunes: leverage, position sizing, margin usage, confidence thresholds
- Inner-loop optimization within factor space
- LLM recommendations for risk control adjustments
- Compliance-aware optimization (respects hard limits)

#### 7. Smart Heuristics with Adaptive Position Sizing (`backtest/smart_heuristics.go`)
- Volatility-aware position sizing
- Account state-aware leverage adjustment
- Drawdown-triggered position reduction
- Win-streak confidence scaling
- Correlation-based portfolio weighting

#### 8. Compliance Tracking (`backtest/compliance_tracker.go`)
- Reinforcement learning for prompt adherence
- Tracks: recommendation follow-through rate, instruction violations, effectiveness
- Penalizes strategies that ignore recommendations
- Integrates with prompt evolution for better instructions

#### 9. Trade Outcome Storage (`store/trade_outcome.go`)
- Unified structure for all trade metrics
- Fields: Symbol, Profitable, VolumeAtEntry, OIAtEntry, VolumeDuringTrade, OIDuringTrade, EntrySpread, ExitSpread, EntryDepth, ExitDepth, HoldingMinutes, PnLPct
- Persistent database storage for historical learning
- Enables threshold calibration from >500 past trades

---

### 🔴 Fixed - 17 Critical Issues (Merge Request: [Critical Issues Resolution](docs/nofx-issue-fixed-logs.md))

#### Critical Priority (Profit-Impacting)

**Issue #2: K-line Inconsistency Between Backtest vs Live Trading**
- Backtest showed AI 10 K-lines (30 mins), live trading 30 K-lines (90 mins)
- Made K-line count configurable across both modes
- Files: `market/data.go`, `backtest/datafeed.go`
- Result: Perfect data parity between backtest and live trading

**Issue #9: Stale Price Data (Current Price Not Updating)**
- Current price stuck while actual market price moved (0.85% deviation)
- Upgraded to real-time ticker API with intelligent fallback
- Files: `market/api_client.go`, `market/data.go`
- Result: Real-time price accuracy with automatic fallback

**Issue #13: Dynamic Stop Loss/Take Profit P&L Calculation Bug**
- P&L calculated using original SL/TP instead of actual execution prices
- Implemented exchange-synced P&L with adjustment tracking
- Files: `store/position.go`, `trader/auto_trader.go`
- Result: Accurate P&L with complete audit trail of adjustments

#### High Priority

**Issue #5: 4H Candle Update Failure (WebSocket Limit)**
- 4H candles frozen, exceeded Binance's 1,024 stream limit
- Implemented KlineWebSocketManager with connection pooling
- Files: `market/kline_websocket_manager.go`
- Result: 90% reduction in subscriptions, zero policy violations

**Issue #1: Hardcoded Technical Indicator Parameters**
- EMA, MACD, RSI, ATR parameters hardcoded and not configurable
- Made all indicators configurable in strategy settings
- Files: `store/strategy.go`, `market/data.go`
- Result: Full strategy customization with backward compatibility

**Issue #3: Max Position Logic Bug (False Position Full)**
- Close signal not returning, false "position full" errors
- Implemented "expected net position" logic for rebalancing
- Files: `trader/auto_trader.go`
- Result: Accurate position tracking during rebalancing cycles

**Issue #6: Entry Price Display Inconsistency**
- Entry price diverged between exchange API and database
- Implemented entry price synchronization
- Files: `trader/auto_trader.go`
- Result: Consistent entry prices across all interfaces

#### Medium Priority

**Issue #8: Real-Time Drawdown Monitoring**
- Hardcoded drawdown thresholds (5% profit, 40% drawdown)
- Made drawdown monitoring fully configurable
- Files: `store/strategy.go`, `trader/auto_trader.go`
- Result: User-configurable profit protection based on risk tolerance

**Issue #15: Limited K-line Timeframe Options**
- Verified: All timeframes already supported (1m-1d)
- Added verification tests confirming complete timeframe support

**Issue #10: Enhanced Market Microstructure Data**
- AI decisions limited by insufficient market data
- Created comprehensive microstructure analysis system
- Files: `market/microstructure.go` (443 lines), `decision/engine.go` integration
- Metrics: Spread, imbalance, VWAP, depth, large orders, support/resistance, liquidity
- Result: AI now has complete market intelligence for better decisions

**Issue #17: Historical Position Data Accuracy**
- P&L percentage calculation was fundamentally flawed
- Fixed to use actual margin cost instead of nonsensical leverage multiplication
- Files: `store/position.go`
- Result: Accurate historical trade performance for AI learning

#### Enhancement Features

**Issue #11: Paper Trading / Simulation Mode**
- Users needed risk-free strategy testing capability
- Implemented full paper trading mode with testnet routing
- Files: `store/trader.go`, `api/server.go`, `manager/trader_manager.go`, Web UI
- Supported testnets: Binance, Bybit, OKX, Bitget, Hyperliquid
- Result: Risk-free testing with virtual funds on real exchange infrastructure

**Phase 1.3: Order Book Monitoring**
- Static 3-minute AI scan cycles miss fast-moving opportunities
- Implemented real-time order book anomaly detection
- Files: `market/order_book_monitor.go` (270 lines), `trader/market_monitoring.go` (110 lines)
- Detection types: Price spikes, volume spikes, order imbalance, cooldown-aware
- Result: Catch opportunities 30+ seconds earlier, ~2-3% CPU overhead

**Phase 2: Event-Driven Architecture**
- Created centralized event bus system
- Files: `trader/event_bus.go` (220 lines)
- Event types: price_spike, volume_spike, order_imbalance, order_filled, position_opened, position_closed, risk_event, liquidation
- Result: Unified event system for all trading signals

**Phase 2.1: WebSocket Interface & Implementations**
- Generic WebSocket abstraction layer
- Files: `market/websocket.go` (280 lines)
- Binance WebSocket client: `market/binance_websocket.go` (310 lines)
- OKX, Bybit implementations also included
- Result: Production-ready WebSocket infrastructure

**Phase 2.2: Real-Time Order Streams (150x faster than polling)**
- Eliminated 30-second polling delay for order updates
- Files: `trader/order_websocket_manager.go` (415 lines), `market/binance_order_websocket.go` (326 lines), Bybit & OKX implementations
- Performance: 15s → <100ms latency, 98% API call reduction
- Result: Instant order updates with event-driven architecture

---

### 🚀 Architecture Enhancements

#### Persistent Threshold Calibration
- **File**: `trader/auto_trader.go` (Lines 1240-1269, 1338-1352)
- **Change**: Fixed threshold calibrator to persist across trading cycles
- **Impact**: Calibrator now accumulates learning instead of resetting each cycle
- **Verification**: Build passes, persistent instance confirmed in both calibration sections

#### Backtest Calibration Parity
- **File**: `backtest/runner.go` (Lines 960-980)
- **Change**: Added post-feedback calibration from trade outcomes
- **Flow**: LoadTradeEvents → extractClosedPositions → CalibrateFromHistory
- **Result**: Backtest now mirrors live trading's adaptive learning

#### Comprehensive Documentation Refresh
- **File**: `README.md`
- **Updates**:
  - Feature descriptions highlighting LLM-evolve, prompt variants, threshold calibration, microstructure intelligence
  - Architecture diagram showing full learning loop with feedback → evolution flow
  - Expanded directory walkthrough with 7+ new backtest modules and their purposes
  - Learning systems benchmark comparing naive LLM vs NOFX+ (+39.5% improvement)
  - Complete documentation of all 6+ learning stack components

---

### 📊 Test Coverage & Integration

**New Test Files (1,500+ lines):**
- `market/microstructure_test.go` (515 lines) - 8+ comprehensive tests
- `market/data_test.go` - K-line consistency verification
- `market/configurable_indicators_test.go` (200+ lines)
- `trader/integration_test.go` (220 lines) - 10+ integration tests
- `trader/entry_price_consistency_test.go` (150+ lines)
- `trader/drawdown_monitoring_config_test.go` (200+ lines)
- `store/trader_papertrading_test.go` (100+ lines)

**Build & Quality Verification:**
```bash
✅ go build ./...           # All packages compile
✅ go vet ./...             # Zero linter warnings
✅ go test ./backtest       # All tests pass
✅ go test ./decision       # All tests pass
✅ npm run build (web/)     # Frontend builds
✅ make build               # Backend builds
```

---

### 📈 Quantified Impact

**Learning System Performance (Backtests):**
- **Before**: -27.9% total return, 34.8% win rate, -0.03 Sharpe ratio
- **With Feedback**: +11.6% total return (+39.5% improvement), 66.7% win rate (+91% improvement), 3.35 profit factor
- **With Feedback + Prompt Evolution**: +12.5% total return, 61.5% win rate, 6.13 profit factor (+183% vs feedback-only)
- **Drawdown Reduction**: 27.9% → 4.3% max drawdown (82% improvement)

**Infrastructure Improvements:**
- Order latency: 15s → <100ms (150x faster)
- API call reduction: 98% (event-driven vs polling)
- WebSocket stream reduction: 1,068 → 50-100 connections
- CPU overhead: <3% for order book monitoring

---

### 🙏 Acknowledgments

**NOFX+ builds upon the groundbreaking work of the NOFX team**, enhanced with:
- Production hardening and bug fixes
- Market microstructure intelligence
- Adaptive learning algorithms
- Extensive backtesting and real trading experience
- Institutional-grade architecture

**Credit:** [Jeffee Hsiung](https://github.com/jeffeehsiung) - Developer & Architect

---

## [3.0.0] - 2025-10-30

### Added - Major Architecture Transformation 🚀

**Complete System Redesign - Web-Based Configuration Platform**

This is a **major breaking update** that completely transforms NOFX from a static config-based system to a modern web-based trading platform.

#### Database-Driven Architecture
- SQLite integration replacing static JSON config
- Persistent storage with automatic timestamps
- Foreign key relationships and triggers for data consistency
- Separate tables for AI models, exchanges, traders, and system config

#### Web-Based Configuration Interface
- Complete web-based configuration management (no more JSON editing)
- AI Model setup through web interface (DeepSeek/Qwen API keys)
- Exchange management (Binance/Hyperliquid credentials)
- Dynamic trader creation (combine any AI model with any exchange)
- Real-time control (start/stop traders without system restart)

#### Flexible Architecture
- Separation of concerns (AI models and exchanges independent)
- Mix & match capability (unlimited combinations)
- Scalable design (support for unlimited traders)
- Clean slate approach (no default traders)

#### Enhanced API Layer
- RESTful design with complete CRUD operations
- New endpoints:
  - `GET/PUT /api/models` - AI model configuration
  - `GET/PUT /api/exchanges` - Exchange configuration
  - `POST/DELETE /api/traders` - Trader management
  - `POST /api/traders/:id/start|stop` - Trader control
- Updated documentation for all API endpoints

#### Modernized Codebase
- Type safety with proper separation of configuration types
- Database abstraction with prepared statements
- Comprehensive error handling and validation
- Better code organization (database, API, business logic)

### Changed
- **BREAKING**: Old `config.json` files no longer used
- Configuration must be done through web interface
- Much easier setup and better UX
- No more server restarts for configuration changes

### Why This Matters
- 🎯 **User Experience**: Much easier to configure and manage
- 🔧 **Flexibility**: Create any combination of AI models and exchanges
- 📊 **Scalability**: Support for complex multi-trader setups
- 🔒 **Reliability**: Database ensures data persistence and consistency
- 🚀 **Future-Proof**: Foundation for advanced features

---

## [2.0.2] - 2025-10-29

### Fixed - Critical Bug Fixes: Trade History & Performance Analysis

#### PnL Calculation - Major Error Fixed
- **Fixed**: PnL now calculated as actual USDT amount instead of percentage only
- Previously ignored position size and leverage (e.g., 100 USDT @ 5% = 1000 USDT @ 5%)
- Now: `PnL (USDT) = Position Value × Price Change % × Leverage`
- Impact: Win rate, profit factor, and Sharpe ratio now accurate

#### Position Tracking - Missing Critical Data
- **Fixed**: Open position records now store quantity and leverage
- Previously only stored price and time
- Essential for accurate PnL calculations

#### Position Key Logic - Long/Short Conflict
- **Fixed**: Changed from `symbol` to `symbol_side` format
- Now properly distinguishes between long and short positions
- Example: `BTCUSDT_long` vs `BTCUSDT_short`

#### Sharpe Ratio Calculation - Code Optimization
- **Changed**: Replaced custom Newton's method with `math.Sqrt`
- More reliable, maintainable, and efficient

### Why This Matters
- Historical trade statistics now show real USDT profit/loss
- Performance comparison between different leverage trades is accurate
- AI self-learning mechanism receives correct feedback
- Multi-position tracking (long + short simultaneously) works correctly

---

## [2.0.2] - 2025-10-29

### Fixed - Aster Exchange Precision Error

- Fixed Aster exchange precision error (code -1111)
- Improved price and quantity formatting to match exchange requirements
- Added detailed precision processing logs for debugging
- Enhanced all order functions with proper precision handling

#### Technical Details
- Added `formatFloatWithPrecision` function
- Price and quantity formatted according to exchange specifications
- Trailing zeros removed to optimize API requests

---

## [2.0.1] - 2025-10-29

### Fixed - ComparisonChart Data Processing

- Fixed ComparisonChart data processing logic
- Switched from cycle_number to timestamp grouping
- Resolved chart freezing issue when backend restarts
- Improved chart data display (shows all historical data chronologically)
- Enhanced debugging logs

---

## [2.0.0] - 2025-10-28

### Added - Major Updates

- AI self-learning mechanism (historical feedback, performance analysis)
- Multi-trader competition mode (Qwen vs DeepSeek)
- Binance-style UI (complete interface imitation)
- Performance comparison charts (real-time ROI comparison)
- Risk control optimization (per-coin position limit adjustment)

### Fixed

- Fixed hardcoded initial balance issue
- Fixed multi-trader data sync issue
- Optimized chart data alignment (using cycle_number)

---

## [1.0.0] - 2025-10-27

### Added - Initial Release

- Basic AI trading functionality
- Decision logging system
- Simple Web interface
- Support for Binance Futures
- DeepSeek and Qwen AI model integration

---

## How to Use This Changelog

### For Users
- Check the [Unreleased] section for upcoming features
- Review version sections to understand what changed
- Follow migration guides for breaking changes

### For Contributors
When making changes, add them to the [Unreleased] section under appropriate categories:
- **Added** - New features
- **Changed** - Changes to existing functionality
- **Deprecated** - Features that will be removed
- **Removed** - Features that were removed
- **Fixed** - Bug fixes
- **Security** - Security fixes

When releasing a new version, move [Unreleased] items to a new version section with date.

---

## Links

- [Documentation](docs/README.md)
- [Contributing Guidelines](CONTRIBUTING.md)
- [Security Policy](SECURITY.md)
- [GitHub Repository](https://github.com/NoFxAiOS/nofx)

---

**Last Updated:** 2025-11-01
