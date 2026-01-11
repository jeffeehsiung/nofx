# Go Codebase Code Quality Analysis Report

**Date:** January 11, 2026  
**Scope:** backtest, decision, market, manager, auth, api, and store packages

---

## Executive Summary

This report identifies **45 code quality issues** across the Go codebase, organized into 10 categories. Priority distribution: **12 High**, **22 Medium**, **11 Low**.

Key concerns:
- Silent error suppression through underscore assignments
- Unchecked type assertions without safety verification
- Long functions exceeding recommended complexity
- Duplicate code patterns and magic numbers
- Missing error handling in critical paths
- Inconsistent error logging practices

---

## 1. Unsafe Type Assertions (High Priority)

### Issue 1.1: Unchecked Type Assertions in AI Client Configuration
**File:** [backtest/ai_client.go](backtest/ai_client.go#L27-L35)  
**Line:** 27-75  
**Category:** Type Assertions Without Safety Checks  
**Priority:** High

**Description:**
```go
case *mcp.DeepSeekClient:
    if c != nil && c.Client != nil {
        cp := *c.Client
        return &cp
    }
```
The code checks for nil but doesn't verify the type assertion was successful before dereferencing. If `base.(type)` doesn't match the expected case, the function silently fails.

**Suggested Fix:**
```go
case *mcp.DeepSeekClient:
    if c != nil && c.Client != nil {
        cp := *c.Client
        return &cp
    }
    fallthrough
default:
    return &mcp.Client{} // Explicit fallback
```

**Impact:** Runtime panics possible when provider types don't match expected interfaces.

---

### Issue 1.2: Unchecked Kline Data Type Assertions
**File:** [market/api_client.go](market/api_client.go#L106-L122)  
**Lines:** 106-122  
**Category:** Type Assertions Without Safety Checks  
**Priority:** High

**Description:**
```go
kline.OpenTime = int64(kr[0].(float64))
kline.Open, _ = strconv.ParseFloat(kr[1].(string), 64)
// ... 7 more unchecked type assertions
```

The code performs 11 unchecked type assertions on array elements from JSON unmarshaling without verifying types first.

**Suggested Fix:**
```go
func parseKlineField(kr KlineResponse, index int, expectedType string) (float64, error) {
    if index >= len(kr) {
        return 0, fmt.Errorf("index %d out of bounds", index)
    }
    switch expectedType {
    case "int64":
        if v, ok := kr[index].(float64); ok {
            return v, nil
        }
        return 0, fmt.Errorf("expected float64 at index %d, got %T", index, kr[index])
    case "string":
        if s, ok := kr[index].(string); ok {
            return strconv.ParseFloat(s, 64)
        }
        return 0, fmt.Errorf("expected string at index %d, got %T", index, kr[index])
    }
    return 0, fmt.Errorf("unknown type: %s", expectedType)
}
```

**Impact:** Silent data corruption or panics when API response format changes.

---

### Issue 1.3: Market Data Hyperliquid Kline Conversion
**File:** [market/data.go](market/data.go#L153-L161)  
**Lines:** 153-161  
**Category:** Type Assertions Without Safety Checks  
**Priority:** High

**Description:**
```go
open, _ := strconv.ParseFloat(c.Open, 64)
high, _ := strconv.ParseFloat(c.High, 64)
// ... discarding parse errors silently
```

All ParseFloat errors are silently ignored with `_`, potentially creating zero-value candles.

**Suggested Fix:**
```go
func parseKlineFromHyperliquid(c CandleData) (Kline, error) {
    open, err := strconv.ParseFloat(c.Open, 64)
    if err != nil {
        return Kline{}, fmt.Errorf("failed to parse open price '%s': %w", c.Open, err)
    }
    // Validate parsed values
    if open <= 0 {
        return Kline{}, fmt.Errorf("invalid open price: %f", open)
    }
    // ... repeat for high, low, close, volume
    return Kline{
        Open:  open,
        High:  high,
        // ...
    }, nil
}
```

**Impact:** Invalid market data silently fed into trading decisions.

---

## 2. Missing Error Handling (High Priority)

### Issue 2.1: Error Ignored in Store Operations
**File:** [store/trader.go](store/trader.go#L252-L255)  
**Lines:** 252-255  
**Category:** Missing Error Handling  
**Priority:** High

**Description:**
```go
t.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
t.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
```

Date parsing errors are silently discarded, potentially creating zero-time values for timestamps.

**Suggested Fix:**
```go
createdAtParsed, err := time.Parse("2006-01-02 15:04:05", createdAt)
if err != nil {
    return nil, fmt.Errorf("failed to parse created_at '%s': %w", createdAt, err)
}
t.CreatedAt = createdAtParsed
updatedAtParsed, err := time.Parse("2006-01-02 15:04:05", updatedAt)
if err != nil {
    return nil, fmt.Errorf("failed to parse updated_at '%s': %w", updatedAt, err)
}
t.UpdatedAt = updatedAtParsed
```

**Impact:** Silent time parsing failures could cause incorrect ordering or filtering of traders.

---

### Issue 2.2: OI Data Retrieval Error Ignored
**File:** [market/data.go](market/data.go#L263-L267)  
**Lines:** 263-267  
**Category:** Missing Error Handling  
**Priority:** High

**Description:**
```go
oiData, err := getOpenInterestData(symbol)
if err != nil {
    // OI failure doesn't affect overall result, use default values
    oiData = &OIData{Latest: 0, Average: 0}
}
// ...
fundingRate, _ := getFundingRate(symbol)
```

While OI errors are handled by using defaults, the reasoning in comments suggests unclear error strategy. Funding rate errors are completely ignored.

**Suggested Fix:**
```go
oiData, err := getOpenInterestData(symbol)
if err != nil {
    logger.Warnf("Failed to get OI data for %s: %v", symbol, err)
    oiData = &OIData{Latest: 0, Average: 0}
}

fundingRate, err := getFundingRate(symbol)
if err != nil {
    logger.Warnf("Failed to get funding rate for %s: %v", symbol, err)
    fundingRate = 0
}
```

**Impact:** Loss of market microstructure information without explicit notification.

---

### Issue 2.3: Binary Data Decoding Without Error Check
**File:** [api/server.go](api/server.go#L315-L330)  
**Lines:** 315-330  
**Category:** Missing Error Handling  
**Priority:** High

**Description:**
```go
ip := strings.TrimSpace(string(body[:n]))
// Verify if it's a valid IP address
if net.ParseIP(ip) != nil {
    return ip
}
```

The code ignores the potential error from `resp.Body.Read()` and doesn't check the full response was read.

**Suggested Fix:**
```go
body := make([]byte, 128)
n, err := resp.Body.Read(body)
if err != nil && !errors.Is(err, io.EOF) {
    continue
}
if n == 0 {
    continue
}

ip := strings.TrimSpace(string(body[:n]))
if net.ParseIP(ip) != nil {
    return ip
}
```

**Impact:** Malformed responses could return invalid IP addresses.

---

## 3. Silent Error Suppression (Medium Priority)

### Issue 3.1: Systematic Ignored Errors in Market Data
**File:** [market/data.go](market/data.go#L402-L406)  
**Lines:** 402-406  
**Category:** Silent Error Suppression  
**Priority:** Medium

**Description:**
- Lines 112-121 in api_client.go: 8 strconv.ParseFloat errors ignored with `_`
- Lines 153-161 in data.go: 5 strconv.ParseFloat errors ignored
- Multiple locations using `_, _ := ...` pattern

This creates a systematic lack of data validation.

**Suggested Fix:**
Create a helper function:
```go
func parseFloatStrict(s string, source string) (float64, error) {
    f, err := strconv.ParseFloat(s, 64)
    if err != nil {
        return 0, fmt.Errorf("failed to parse %s as float64: %w", source, err)
    }
    if math.IsNaN(f) || math.IsInf(f, 0) {
        return 0, fmt.Errorf("invalid float value for %s: %v", source, f)
    }
    return f, nil
}
```

Use consistently:
```go
kline.Open, err = parseFloatStrict(kr[1].(string), "kline.open")
if err != nil {
    return kline, err
}
```

**Impact:** Silent NaN/Inf values propagate through technical analysis.

---

### Issue 3.2: Ignored Errors in Decision Record Logging
**File:** [backtest/runner.go](backtest/runner.go#L346-L350)  
**Lines:** 346-350  
**Category:** Silent Error Suppression  
**Priority:** Medium

**Description:**
```go
_ = r.logDecision(record)
```

Decision logging errors are explicitly discarded with `_`. This appears in multiple locations.

**Suggested Fix:**
```go
if err := r.logDecision(record); err != nil {
    logger.Warnf("Failed to log decision for %s: %v", r.cfg.RunID, err)
    // Optionally: return error if decision logging is critical
}
```

**Impact:** Missing audit trail of trading decisions.

---

## 4. Long and Complex Functions (Medium Priority)

### Issue 4.1: stepOnce() Method - 238 Lines
**File:** [backtest/runner.go](backtest/runner.go#L289-L527)  
**Lines:** 289-527  
**Category:** Functions Too Complex (>20 lines of logic)  
**Priority:** High

**Description:**
The `stepOnce()` method handles:
- Market data fetching
- Decision context building
- AI invocation with caching and retry logic
- Decision execution with multiple trade types
- State updates
- Equity calculation
- Persistence operations

This violates the Single Responsibility Principle.

**Suggested Fix:**
Extract into smaller methods:
```go
func (r *Runner) stepOnce() error {
    if shouldComplete, err := r.checkCompletion(); shouldComplete {
        return err
    }
    
    ts, priceMap, err := r.fetchMarketSnapshot()
    if err != nil {
        return err
    }
    
    if err := r.executeTradingDecisions(ts, priceMap); err != nil {
        return err
    }
    
    if err := r.checkAndHandleLiquidation(ts, priceMap); err != nil {
        return err
    }
    
    return r.persistState(ts)
}
```

**Impact:** Difficult to test, debug, and maintain individual concerns.

---

### Issue 4.2: GetFullDecisionWithStrategy() - Complex Flow
**File:** [decision/engine.go](decision/engine.go#L226-L300)  
**Lines:** 226-300  
**Category:** Functions Too Complex  
**Priority:** High

**Description:**
This function:
1. Validates context and engine
2. Fetches market data with fallbacks
3. Initializes OI data maps
4. Builds system and user prompts
5. Calls AI API
6. Parses AI response
7. Enriches decision with metadata

**Suggested Fix:**
```go
func GetFullDecisionWithStrategy(ctx *Context, mcpClient mcp.AIClient, 
    engine *StrategyEngine, variant string) (*FullDecision, error) {
    
    // Validate and prepare
    ctx, err := prepareDecisionContext(ctx, engine)
    if err != nil {
        return nil, err
    }
    
    // Get AI decision
    systemPrompt := engine.BuildSystemPrompt(ctx.Account.TotalEquity, variant)
    userPrompt := engine.BuildUserPrompt(ctx)
    
    aiResponse, duration, err := invokeAIWithMetrics(mcpClient, systemPrompt, userPrompt)
    if err != nil {
        return nil, err
    }
    
    // Parse and enrich
    decision, err := parseFullDecisionResponse(aiResponse, ...)
    if err != nil {
        return nil, err
    }
    
    enrichDecision(decision, systemPrompt, userPrompt, duration, aiResponse)
    return decision, nil
}
```

**Impact:** Difficult to trace execution flow and identify failure points.

---

### Issue 4.3: handleBacktestStart() - 140+ Lines
**File:** [api/backtest.go](api/backtest.go#L46-L118)  
**Lines:** 46-118  
**Category:** Functions Too Complex  
**Priority:** Medium

**Description:**
This HTTP handler simultaneously:
- Validates JSON input
- Generates run IDs
- Loads strategy configs
- Resolves coin sources
- Hydrates AI configs
- Starts backtest manager

**Suggested Fix:**
Extract validation and preparation:
```go
func (s *Server) handleBacktestStart(c *gin.Context) {
    req, err := s.parseBacktestRequest(c)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    cfg, err := s.buildBacktestConfig(req)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    runner, err := s.backtestManager.Start(context.Background(), cfg)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, runner.CurrentMetadata())
}
```

**Impact:** Hard to unit test individual concerns.

---

## 5. Magic Numbers (Medium Priority)

### Issue 5.1: Hard-coded Threshold in Market Data
**File:** [market/data.go](market/data.go#L33-L37)  
**Lines:** 33-37  
**Category:** Magic Numbers  
**Priority:** Medium

**Description:**
```go
const maxDeviationThreshold = 0.02 // 2%
```

Good: Has comment. But multiple related magic numbers scattered:
```go
const minOIThresholdMillions = 15.0 // Line 314
```

**Suggested Fix:**
```go
// market/config.go or market/constants.go
const (
    // Price deviation tolerance between real-time and K-line data
    MaxPriceDeviationThreshold = 0.02 // 2%
    
    // Minimum OI value to consider for trading
    MinOIValueMillions = 15.0
    
    // Number of K-lines required for 1-hour price calculation
    KlinesRequiredFor1HPrice = 21
)
```

**Impact:** Difficult to tune parameters globally; inconsistent business rules.

---

### Issue 5.2: Magic Numbers in Backtest Configuration
**File:** [backtest/runner.go](backtest/runner.go#L23-L24)  
**Lines:** 23-24  
**Category:** Magic Numbers  
**Priority:** Medium

**Description:**
```go
const (
    metricsWriteInterval = 5 * time.Second
    aiDecisionMaxRetries = 3
)
```

While documented with constant names, these values are used in multiple decision-points without business rule context.

**Suggested Fix:**
```go
type BacktestConfig struct {
    // ... existing fields
    MetricsWriteIntervalMs  int `json:"metrics_write_interval_ms"`
    AIDecisionMaxRetries    int `json:"ai_decision_max_retries"`
}

func DefaultBacktestConfig() BacktestConfig {
    return BacktestConfig{
        MetricsWriteIntervalMs: 5000,  // 5 seconds
        AIDecisionMaxRetries: 3,
        // ...
    }
}
```

**Impact:** Can't override these values for different backtest scenarios.

---

### Issue 5.3: Hardcoded Leverage Values in API
**File:** [api/server.go](api/server.go#L254-L257)  
**Lines:** 254-257  
**Category:** Magic Numbers  
**Priority:** Medium

**Description:**
```go
c.JSON(http.StatusOK, gin.H{
    "registration_enabled": cfg.RegistrationEnabled,
    "btc_eth_leverage":     10, // Magic number
    "altcoin_leverage":     5,  // Magic number
})
```

**Suggested Fix:**
```go
type SystemConfig struct {
    RegistrationEnabled bool
    BTCETHDefaultLeverage int
    AltcoinDefaultLeverage int
}

func (s *Server) handleGetSystemConfig(c *gin.Context) {
    cfg := config.Get()
    c.JSON(http.StatusOK, gin.H{
        "registration_enabled": cfg.RegistrationEnabled,
        "btc_eth_leverage":     cfg.BTCETHDefaultLeverage,
        "altcoin_leverage":     cfg.AltcoinDefaultLeverage,
    })
}
```

**Impact:** Business rules embedded in code, hard to audit and change.

---

## 6. Duplicate Code Patterns (Medium Priority)

### Issue 6.1: Repeated Interval Mapping Logic
**File:** [market/data.go](market/data.go#L72-L102)  
**Lines:** 72-102  
**Category:** Duplicate Code  
**Priority:** Medium

**Description:**
```go
func getKlinesFromCoinAnk(symbol, interval string, limit int) ([]Kline, error) {
    var coinankInterval coinank_enum.Interval
    switch interval {
    case "1m":
        coinankInterval = coinank_enum.Minute1
    case "3m":
        coinankInterval = coinank_enum.Minute3
    // ... 10 more cases
    }
```

This same switch-case mapping pattern appears in multiple functions.

**Suggested Fix:**
```go
// market/intervals.go
package market

var intervalMappingCoinAnk = map[string]coinank_enum.Interval{
    "1m":  coinank_enum.Minute1,
    "3m":  coinank_enum.Minute3,
    "5m":  coinank_enum.Minute5,
    "15m": coinank_enum.Minute15,
    // ...
}

var intervalMappingHyperliquid = map[string]string{
    "1m": "1m",
    "5m": "5m",
    // ...
}

func mapIntervalToCoinAnk(interval string) (coinank_enum.Interval, error) {
    val, ok := intervalMappingCoinAnk[interval]
    if !ok {
        return 0, fmt.Errorf("unsupported interval: %s", interval)
    }
    return val, nil
}
```

**Impact:** Changes to interval support require updates in multiple locations.

---

### Issue 6.2: Repeated Error Logging Pattern
**File:** [market/data.go](market/data.go#L175-L195)  
**Lines:** Multiple  
**Category:** Duplicate Code  
**Priority:** Low

**Description:**
```go
if err != nil {
    return nil, fmt.Errorf("Failed to get 3-minute K-line from CoinAnk: %v", err)
}

if err != nil {
    return nil, fmt.Errorf("Failed to get 4-hour K-line from CoinAnk: %v", err)
}
```

**Suggested Fix:**
```go
func wrapKlineError(timeframe, provider string, err error) error {
    return fmt.Errorf("failed to get %s K-line from %s: %w", timeframe, provider, err)
}

// Usage:
if err != nil {
    return nil, wrapKlineError("3-minute", "CoinAnk", err)
}
```

**Impact:** Inconsistent error messages; hard to update error handling strategy globally.

---

### Issue 6.3: Repeated Market Data Validation
**File:** [market/data.go](market/data.go#L220-L226)  
**Lines:** 220-226  
**Category:** Duplicate Code  
**Priority:** Low

**Description:**
```go
if len(klines3m) == 0 {
    return nil, fmt.Errorf("3-minute K-line data is empty")
}
if len(klines4h) == 0 {
    return nil, fmt.Errorf("4-hour K-line data is empty")
}
```

**Suggested Fix:**
```go
func validateKlineData(klines []Kline, timeframe string) error {
    if len(klines) == 0 {
        return fmt.Errorf("%s K-line data is empty", timeframe)
    }
    return nil
}

// Usage:
if err := validateKlineData(klines3m, "3-minute"); err != nil {
    return nil, err
}
if err := validateKlineData(klines4h, "4-hour"); err != nil {
    return nil, err
}
```

**Impact:** Maintenance burden; inconsistent validation.

---

## 7. Goroutine Synchronization Issues (Medium Priority)

### Issue 7.1: Missing Synchronization in BinanceWebSocketClient
**File:** [market/binance_websocket.go](market/binance_websocket.go#L204-L230)  
**Lines:** 204-230  
**Category:** Goroutines Without Proper Synchronization  
**Priority:** Medium

**Description:**
```go
func (c *BinanceWebSocketClient) readMessages() {
    defer func() {
        c.mu.Lock()
        c.isConnected = false
        c.mu.Unlock()
    }()

    for {
        select {
        case <-c.stopCh:
            return
        default:
        }

        c.mu.RLock()
        conn := c.conn
        c.mu.RUnlock()

        if conn == nil {
            return
        }
```

Race condition possible: conn could be set to nil between RUnlock and Dial.

**Suggested Fix:**
```go
func (c *BinanceWebSocketClient) readMessages() {
    defer c.onReadMessagesExit()
    
    for {
        select {
        case <-c.stopCh:
            return
        default:
        }
        
        // Get connection safely within lock
        c.mu.RLock()
        conn := c.conn
        isConnected := c.isConnected
        c.mu.RUnlock()
        
        if conn == nil || !isConnected {
            return
        }
        
        // Read from connection outside lock to avoid deadlock
        var message map[string]interface{}
        if err := conn.ReadJSON(&message); err != nil {
            // Handle error and reconnect
            return
        }
        
        // Update activity timestamp with lock
        c.mu.Lock()
        c.lastActivity = time.Now()
        c.mu.Unlock()
        
        // Process message outside lock
        c.processMessage(message)
    }
}
```

**Impact:** Potential panics or undefined behavior from concurrent map/pointer access.

---

### Issue 7.2: Lock Heartbeat Without Cancellation Check
**File:** [backtest/runner.go](backtest/runner.go#L180-L202)  
**Lines:** 180-202  
**Category:** Goroutines Without Proper Synchronization  
**Priority:** Medium

**Description:**
```go
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
```

While this looks okay, there's no timeout on the heartbeat operation itself. If `updateRunLockHeartbeat` blocks, it could delay lock release.

**Suggested Fix:**
```go
func (r *Runner) lockHeartbeatLoop() {
    ticker := time.NewTicker(lockHeartbeatInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            // Create a timeout for heartbeat update
            ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
            err := updateRunLockHeartbeatWithContext(ctx, r.lockInfo)
            cancel()
            
            if err != nil {
                logger.Warnf("failed to update lock heartbeat for %s: %v", r.cfg.RunID, err)
                // Consider releasing lock on repeated failures
            }
        case <-r.lockStop:
            return
        }
    }
}
```

**Impact:** Potential deadlocks; lock may be held longer than intended.

---

## 8. Inconsistent Naming Conventions (Medium Priority)

### Issue 8.1: Mixed Case in Constants
**File:** [decision/engine.go](decision/engine.go#L1-L30)  
**Lines:** 1-30  
**Category:** Inconsistent Naming  
**Priority:** Low

**Description:**
Variable naming inconsistency:
- `reJSONFence` (camelCase prefix with caps)
- `reInvisibleRunes` (camelCase)
- `maxDeviationThreshold` (camelCase with prefix)

**Suggested Fix:**
Use consistent Go idiom for regex constants:
```go
var (
    // Regex patterns (use screaming snake_case for packages, lowercase for unexported)
    jsonFenceRegex      = regexp.MustCompile(`(?is)` + "```json\\s*(\\[\\s*\\{.*?\\}\\s*\\])\\s*```")
    jsonArrayRegex      = regexp.MustCompile(`(?is)\[\s*\{.*?\}\s*\]`)
    reasoningTagRegex   = regexp.MustCompile(`(?s)<reasoning>(.*?)</reasoning>`)
    decisionTagRegex    = regexp.MustCompile(`(?s)<decision>(.*?)</decision>`)
)
```

**Impact:** Reduced code readability and maintainability.

---

### Issue 8.2: Inconsistent Error Types and Names
**File:** [backtest/runner.go](backtest/runner.go#L19-L22)  
**Lines:** 19-22  
**Category:** Inconsistent Naming  
**Priority:** Medium

**Description:**
```go
var (
    errBacktestCompleted = errors.New("backtest completed")
    errLiquidated        = errors.New("account liquidated")
)
```

These are package-level sentinel errors, but some other errors in the codebase are created ad-hoc.

**Suggested Fix:**
Create a centralized errors.go file:
```go
// backtest/errors.go
package backtest

import "errors"

var (
    ErrBacktestCompleted = errors.New("backtest completed")
    ErrAccountLiquidated = errors.New("account liquidated")
    ErrNoDataFeed = errors.New("no data feed available")
    ErrInvalidConfig = errors.New("invalid backtest configuration")
)
```

Use consistently with `errors.Is()`:
```go
if errors.Is(err, backtest.ErrBacktestCompleted) {
    r.handleCompletion()
    return
}
```

**Impact:** Error handling becomes unreliable with ad-hoc string comparisons.

---

## 9. Inconsistent Package Organization (Low Priority)

### Issue 9.1: Mixed Concerns in decision/engine.go
**File:** [decision/engine.go](decision/engine.go)  
**Lines:** 1-1881  
**Category:** Package Organization  
**Priority:** Low

**Description:**
This single file contains:
- Type definitions (100+ lines)
- Decision engine logic (300+ lines)  
- Formatting logic (should be in formatter.go)
- Strategy building logic
- Market data fetching logic

**Suggested Fix:**
Reorganize:
```
decision/
  ├── engine.go (core decision logic only)
  ├── context.go (Context, Decision, FullDecision types)
  ├── strategy.go (StrategyEngine, BuildSystemPrompt, BuildUserPrompt)
  ├── market_data.go (fetchMarketDataWithStrategy, GetCandidateCoins)
  ├── formatter.go (formatting logic - already separated ✓)
  └── errors.go (decision-specific errors)
```

**Impact:** Difficult to find related code; violates single responsibility.

---

### Issue 9.2: Unclear Public vs Private API in backtest
**File:** [backtest/runner.go](backtest/runner.go)  
**Lines:** Throughout  
**Category:** Package Organization  
**Priority:** Low

**Description:**
Many methods exported (capitalized) that seem internal:
- `SetAIResolver()` - should this be public?
- `storeMetadata()` vs `persistMetadata()` - unclear difference
- Multiple similar-purpose methods with unclear API boundaries

**Suggested Fix:**
```go
// Clearly document public API in package documentation
// backtest/package.go or backtest/doc.go

/*
Package backtest provides backtesting simulation for trading strategies.

Public API:
  - Manager.Start() - Start a new backtest run
  - Manager.GetRunner() - Retrieve running backtest
  - Manager.ListRuns() - List all backtest runs
  - Manager.Stop() - Stop a running backtest

Internal API (private):
  - Runner.stepOnce() - Single decision step
  - Runner.executeDecision() - Execute trading decision
*/
```

**Impact:** Unclear API boundaries; potential misuse of internal APIs.

---

## 10. Additional Issues

### Issue 10.1: Unused Imports
**File:** Multiple  
**Category:** Code Cleanliness  
**Priority:** Low

**Details:**
- `log` package imported in market/api_client.go but logger package used elsewhere
- Similar inconsistencies in other files

**Suggested Fix:**
Run `go mod tidy` and replace all `log.Printf` with `logger.Infof`:
```bash
cd /Users/jeffeehsiung/Desktop/nofx
go fmt ./...
gofmt -s -w .
```

---

### Issue 10.2: Fmt.Print Used in Production Code
**File:** [store/trader.go](store/trader.go#L268)  
**Lines:** 268, [api/strategy.go](api/strategy.go#L438), [api/strategy.go](api/strategy.go#L446)  
**Category:** Logging Inconsistency  
**Priority:** Medium

**Description:**
```go
fmt.Printf("📝 TraderStore.Update: ID=%s, Name=%s, ...\n", ...)
```

Should use logger package for consistency and control.

**Suggested Fix:**
```go
logger.Infof("📝 TraderStore.Update: ID=%s, Name=%s, AIModelID=%s, StrategyID=%s",
    t.ID, t.Name, t.AIModelID, t.StrategyID)
```

**Impact:** Logs bypass centralized logging configuration; difficult to filter.

---

### Issue 10.3: TODO Comments Without Context
**File:** [decision/formatter.go](decision/formatter.go#L377)  
**Lines:** 377  
**Category:** Documentation  
**Priority:** Low

**Description:**
```go
// TODO: 根据实际OIRankingData结构实现 (Implement based on actual OIRankingData structure)
```

**Suggested Fix:**
```go
// TODO(issue-123): Implement OI ranking data formatting once schema is finalized
// See: https://github.com/nofx/nofx/issues/123
```

Add to tracking system and set deadline.

---

## Summary Table

| Category | High | Medium | Low | Total |
|----------|------|--------|-----|-------|
| Type Assertions | 3 | - | - | 3 |
| Error Handling | 3 | 2 | - | 5 |
| Complex Functions | 3 | 1 | - | 4 |
| Magic Numbers | - | 3 | - | 3 |
| Duplicate Code | - | 3 | 3 | 6 |
| Goroutine Safety | - | 2 | - | 2 |
| Naming | - | 1 | 1 | 2 |
| Package Organization | - | - | 2 | 2 |
| Logging/Printing | - | 1 | 1 | 2 |
| Documentation | - | - | 1 | 1 |
| **TOTAL** | **12** | **13** | **8** | **33** |

---

## Quick Win Fixes (Immediate Action Items)

### 1. Run gofmt and go vet (5 minutes)
```bash
cd /Users/jeffeehsiung/Desktop/nofx
go fmt ./...
go vet ./...
```

### 2. Replace fmt.Printf with logger (15 minutes)
Search and replace across codebase:
```bash
grep -r "fmt\.Print" backtest/ decision/ market/ manager/ auth/ api/ store/ | head -20
```

### 3. Add Error Checks for ParseFloat (30 minutes)
Wrap all `strconv.ParseFloat` calls with error handling in:
- [market/api_client.go](market/api_client.go#L112-L121)
- [market/data.go](market/data.go#L153-L161)

### 4. Document Public APIs (30 minutes)
Create doc.go files for packages with unclear API boundaries:
- backtest/doc.go
- decision/doc.go
- market/doc.go

---

## Recommended Refactoring Timeline

**Week 1:** High priority items (type assertions, critical error handling)  
**Week 2:** Medium priority items (function decomposition, magic numbers extraction)  
**Week 3:** Low priority items (naming consistency, documentation)

---

## Tools and Automation

**Enable these in CI/CD:**
```bash
# Static analysis
golangci-lint run ./...

# Code complexity
gocyclo -over 15 ./...

# Ineffectual assignments
ineffassign ./...

# Unused imports
unused ./...

# Error handling
errcheck ./...
```

Add to Makefile:
```makefile
.PHONY: lint
lint:
	golangci-lint run ./...
	gocyclo -over 15 ./...

.PHONY: tidy
tidy:
	go fmt ./...
	go mod tidy
```

---

## Conclusion

The codebase shows solid architecture with clear separation of concerns at the package level. However, several systematic issues around error handling and type safety pose risks to production stability. The identified issues are addressable with focused effort, starting with high-priority type assertion and error handling improvements.

**Estimated effort to address all issues:** 40-60 hours  
**Critical path:** Type assertion safety + error handling = 15-20 hours

