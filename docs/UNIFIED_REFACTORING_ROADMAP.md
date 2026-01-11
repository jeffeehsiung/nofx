# Unified Refactoring Roadmap - Phase 2 Complete

**Date**: January 11, 2026  
**Status**: Phase 2.1-2.3 Complete | Phase 2.4-3 Ready to Start  
**Total Issues**: 87 | **Hours Remaining**: 15-22

---

## Executive Status

### ✅ Completed (Phase 2.1-2.3)
- **Constants Consolidation**: 100+ magic numbers → `backtest/constants.go` (164 lines)
- **Utility Functions**: 20+ functions → `backtest/utils.go` (318 lines)
- **Error Handling Fixes**: 14 silent errors fixed across 4 files
- **Build Status**: ✅ PASS (46MB, 0 errors)
- **Code Added**: 700+ lines of high-quality code

### 🔄 In Progress (Phase 2.4-3)
- **Constants Application**: 28 locations need constant references
- **Code Duplication**: 12 patterns identified for consolidation
- **Long Function Extraction**: 15 functions >100 lines
- **Goroutine Safety**: 5 edge cases identified

---

## Implementation Timeline

```
PHASE 2.1-2.3: COMPLETE ✅
├─ Constants created (backtest/constants.go)
├─ Utilities created (backtest/utils.go)
├─ Errors fixed (14 locations)
└─ Documentation prepared

PHASE 2.4: IN PROGRESS 🔄 (4-6 hours)
├─ Tier 1: Decision engine (decision/engine.go, backtest/runner.go)
├─ Tier 2: Traders (trader/auto_trader.go, trader/hyperliquid_trader.go)
├─ Tier 3: Market (market/data.go, market/websocket.go)
├─ Tier 4: API (api/server.go, api/crypto_handler.go)
└─ Tier 5: Optimization (backtest/feedback.go, factor_optimizer.go)

PHASE 2.5: NEXT (3-4 hours)
├─ Drawdown calculation consolidation (4 locations)
├─ Default value setting consolidation (3 locations)
├─ Error handling consolidation (3 locations)
└─ Timeframe validation consolidation (2 locations)

PHASE 3: PLANNED (8-12 hours)
├─ Long function extraction (15 functions)
├─ Goroutine safety fixes (5 issues)
└─ Code quality improvements (logging, formatting)
```

---

## Phase 2.4 - Constants Application Checklist

### Tier 1: Core Decision-Making (1-2 hours)

**File: decision/engine.go**
- [ ] Line 805: Apply MinConfidenceForBatching (70)
- [ ] Line 813-820: Apply DefaultBTCETHPosRatio (5.0), DefaultAltcoinPosRatio (1.0)
- [ ] Line 841-843: Apply ConfidenceHigh (85), ConfidenceMedium (70-84), ConfidenceLow (60-69)

**File: backtest/runner.go**
- [ ] Line ~450: Apply DefaultMaxDrawdownPct in drawdown calculation
- [ ] Line ~987: Apply CriticalDrawdownThreshold
- [ ] Line ~metricsWrite: Apply MetricsUpdateInterval

**Effort**: ~1-2 hours | **Priority**: HIGH | **Impact**: Core trading logic becomes configurable

---

### Tier 2: Position & Leverage Management (1-2 hours)

**File: trader/auto_trader.go**
- [ ] Line 1978: Apply DefaultLeverage (10)
- [ ] Position sizing constants: Apply DefaultMinPositionSize, DefaultMaxPositionSize
- [ ] Risk thresholds: Apply DefaultMaxDrawdownPct, CriticalDrawdownThreshold
- [ ] Monitoring: Apply DefaultCheckpointIntervalSeconds

**File: trader/hyperliquid_trader.go**
- [ ] Apply DefaultBTCETHLeverage, DefaultAltcoinLeverage
- [ ] Apply leverage validation limits (MaxBTCETHLeverage, MaxAltcoinLeverage)

**File: api/server.go**
- [ ] Line 540-548: Apply DefaultBTCETHLeverage (10), DefaultAltcoinLeverage (5)
- [ ] Line 497-502: Apply MaxBTCETHLeverage (50), MaxAltcoinLeverage (20), MinLeverage (1)
- [ ] Validation constraints: Use constants instead of magic numbers

**Effort**: ~1-2 hours | **Priority**: HIGH | **Impact**: Consistent position sizing across exchanges

---

### Tier 3: Market Data & Timeframes (30 min - 1 hour)

**File: market/data.go**
- [ ] Line 40: Apply MaxPriceDeviationThreshold (0.02)
- [ ] Line 179: Apply MinOIThresholdMillions (15.0)
- [ ] Apply MinimumKlineCount where used

**File: market/websocket.go**
- [ ] Apply WebsocketReconnectDelay
- [ ] Apply MaxConcurrentRequests
- [ ] Apply DefaultTimeframe, ConfirmationTimeframe

**File: market/microstructure.go**
- [ ] Line 412-421: Apply ImbalanceStrongBuyThreshold (0.65)
- [ ] Apply ImbalanceModBuyThreshold (0.55)
- [ ] Apply ImbalanceStrongSellThreshold (0.35)
- [ ] Apply ImbalanceModSellThreshold (0.45)

**Effort**: ~30 min - 1 hour | **Priority**: MEDIUM | **Impact**: Data quality becomes configurable

---

### Tier 4: API & Configuration (1 hour)

**File: api/server.go**
- [ ] Apply AI decision max retries constant
- [ ] Apply timeout configurations
- [ ] Apply rate limiting constants

**File: api/crypto_handler.go**
- [ ] Apply API-specific constants
- [ ] Apply retry configurations

**Effort**: ~1 hour | **Priority**: MEDIUM | **Impact**: API behavior becomes tunable

---

### Tier 5: Feedback & Optimization (30 min)

**File: backtest/feedback.go**
- [ ] Apply MinTradesForFeedback (10)
- [ ] Apply FeedbackUpdateInterval (5)

**File: backtest/factor_optimizer.go**
- [ ] Apply DefaultFactorOptimizationCycles (15)
- [ ] Apply MinCyclesBeforeOptimization (10)

**Effort**: ~30 min | **Priority**: LOW | **Impact**: Feedback loop becomes configurable

---

## Phase 2.5 - Code Duplication Consolidation

### Issue #1: Drawdown Calculation (4 locations)
**Priority**: HIGH | **Effort**: 30 minutes

**Locations**:
1. `backtest/runner.go` L450-454
2. `trader/auto_trader.go` L2010-2013
3. `backtest/runner.go` L987-989
4. `trader/drawdown_monitoring_config_test.go` L150-160

**Solution**: Use `backtest/utils.go::CalculateDrawdown()`
```go
drawdown := utils.CalculateDrawdown(peak, current)
```

**Verification**: Run `grep -r "((.*-.*)/.*)*100" --include="*.go"` to find remaining instances

---

### Issue #2: Default Value Setting (3 locations)
**Priority**: MEDIUM | **Effort**: 1 hour

**Pattern**:
```go
// Before
if leverage <= 0 {
  leverage = 10
}

// After
leverage = utils.GetOrDefault(leverage, DefaultLeverage)
```

**Locations**:
1. `api/server.go` - leverage defaults
2. `decision/engine.go` - position ratio defaults
3. Similar patterns in trader files

---

### Issue #3: Error Handling/Retry Logic (3 locations)
**Priority**: MEDIUM | **Effort**: 1-2 hours

**Create**: `trader/retry_helper.go` or `util/retry.go`
```go
func RetryWithBackoff(ctx context.Context, maxRetries int, fn func() error) error
```

**Apply** to:
1. WebSocket reconnection logic
2. API call retries
3. Order execution retries

---

### Issue #4: Timeframe Validation (2 locations)
**Priority**: LOW | **Effort**: 30 minutes

**Create**: Validator in `util/validation.go`
```go
func IsValidTimeframe(tf string) bool {
  validTimeframes := map[string]bool{
    "1m": true, "5m": true, "15m": true, "1h": true, "4h": true, "1d": true,
  }
  return validTimeframes[tf]
}
```

**Apply** to:
1. `market/data.go` - timeframe validation
2. `market/websocket.go` - subscription validation

---

## Phase 3 - Maintainability & Safety

### Long Function Extraction (8-12 hours)

**CRITICAL** - `api/server.go::handleCreateTrader()` (269 lines)
- Extract: `setTraderDefaults()` (50 lines)
- Extract: `queryActualBalance()` (70 lines)
- Extract: `createTempTrader()` (64 lines)
- Extract: `queryExchangeBalance()` (65 lines)
- Result: ~80 lines main function

**HIGH** - `backtest/runner.go::stepOnce()` (238 lines)
- Extract: `evaluatePositions()` (40 lines)
- Extract: `executeDecisions()` (45 lines)
- Extract: `recordResults()` (30 lines)
- Extract: `updateMetrics()` (35 lines)
- Extract: `checkRiskControls()` (30 lines)
- Result: ~80 lines main function

**MEDIUM** - `decision/engine.go::BuildSystemPrompt()` (100+ lines)
- Extract: `buildModeSection()`
- Extract: `buildRiskConstraints()`
- Extract: `buildPositionSizing()`

**Other Functions** (8-10 functions, 100-200 lines each)
- `handleClosePosition()` - Extract validation, execution, result recording
- `handleSyncBalance()` - Extract per-exchange sync logic
- `BuildUserPrompt()` - Extract formatting helpers

---

### Goroutine Safety Fixes (2-3 hours)

**Issue #1**: Peak PnL cache race condition
- **File**: `trader/auto_trader.go` L1996-2006
- **Fix**: Deferred locking pattern or atomic operations
- **Effort**: 30 min

**Issue #2**: Order book monitor creation
- **File**: `trader/auto_trader.go` L117-118
- **Fix**: Double-check locking pattern
- **Effort**: 30 min

**Issue #3-5**: WebSocket & microstructure review
- **Status**: Mostly good, minor improvements only
- **Effort**: 1 hour

---

### Logging Standardization (1-2 hours)

**Goal**: Replace `fmt.Printf` with `logger` package

**Pattern**:
```go
// Before
fmt.Printf("Value: %v\n", val)

// After
logger.Infof("Value: %v", val)
```

**Files to scan**:
- `api/server.go`
- `trader/auto_trader.go`
- `backtest/runner.go`

---

## Success Criteria

### Phase 2.4 Complete ✅
- [ ] All 28 constant applications completed
- [ ] Build verification: `go build -o /tmp/nofx .` - PASS
- [ ] No functionality changes verified
- [ ] All tests passing

### Phase 2.5 Complete ✅
- [ ] Drawdown calculations consolidated (4→1)
- [ ] Default value patterns consolidated (3→1)
- [ ] Error handling patterns consolidated (3→1)
- [ ] Timeframe validation consolidated (2→1)
- [ ] 100+ lines of duplication removed

### Phase 3 Complete ✅
- [ ] All long functions extracted (15→0)
- [ ] Goroutine safety issues fixed (5→0)
- [ ] Test coverage: 95%+ for new code
- [ ] `go test -race` passes

### Overall Success ✅
- [ ] Zero magic numbers in critical files
- [ ] Code quality improved by 40%+
- [ ] Build size: ~46MB (no change)
- [ ] Performance: No regression
- [ ] Documentation: Updated throughout

---

## Quick Reference

### Constants File
**Location**: `backtest/constants.go` (164 lines)

**Key Sections**:
```go
// Leverage (4 constants)
DefaultMinLeverage, OptimalLeverage, DefaultMaxLeverage

// Risk Management (4 constants)
DefaultMaxDrawdownPct, DefaultDrawdownWarningLevel, StopLossPct, TakeProfitPct

// Position Sizing (5 constants)
DefaultMinPositionSize, DefaultMaxPositionSize, MaxPositionsPerAccount

// Confidence (4 constants)
MinConfidenceForEntry, MinConfidenceForSize, CriticalConfidenceLevel

// Performance (5 constants)
MinWinRateForSuccess, GoodWinRate, ExcellentWinRate, GoodProfitFactor

// Plus: Market thresholds, time intervals, monitoring constants
```

### Utilities File
**Location**: `backtest/utils.go` (318 lines)

**Key Functions**:
```go
CalculateDrawdown()           // Drawdown calculation
CalculateProfitFactor()       // Win/loss ratio
CalculateWinRate()           // Win percentage
AdjustPositionSizeByEquity() // Dynamic sizing
AdjustLeverageByDrawdown()   // Risk-based leverage
ValidateMarginUsage()        // Safety checks
CalculateUnrealizedPnL()     // Position P&L
ShouldClosePosition()        // Exit conditions
```

---

## Implementation Order

**Today**: Phase 2.4 Tier 1-2 (2-4 hours)
1. Apply constants to decision/engine.go
2. Apply constants to backtest/runner.go
3. Apply constants to trader/auto_trader.go
4. Verify build and basic functionality

**Tomorrow**: Phase 2.4 Tier 3-5 (2-3 hours)
1. Apply constants to market files
2. Apply constants to API files
3. Apply constants to feedback/optimization
4. Comprehensive testing

**Following Day**: Phase 2.5 (3-4 hours)
1. Consolidate drawdown calculations
2. Consolidate default value patterns
3. Consolidate error handling
4. Consolidate timeframe validation

**Future Sessions**: Phase 3 (8-12 hours)
1. Extract long functions
2. Fix goroutine safety
3. Standardize logging
4. Final testing and cleanup

---

## Risk Assessment

**Phase 2.4**: LOW RISK
- Constants are read-only, no behavioral changes
- Simple find/replace operations
- Easy to verify: identical behavior with constants

**Phase 2.5**: LOW RISK
- Extracting duplicates to functions
- Pure refactoring, no logic changes
- Tests should catch any issues

**Phase 3**: MEDIUM RISK
- Function extraction: Could introduce subtle bugs
- Goroutine safety: Need careful testing with `-race` flag
- Mitigation: Comprehensive testing, code review

---

## Tools & Commands

### Build Verification
```bash
go build -o /tmp/nofx .          # Full build
go build -v . 2>&1 | grep error  # Check errors
```

### Code Quality
```bash
go fmt ./...                      # Format code
go test ./...                     # Run tests
go test -race ./...              # Check race conditions
golangci-lint run ./...          # Code quality
```

### Find Issues
```bash
grep -r "magic_numbers" --include="*.go" | head -20
grep -r "((.*-.*)/.*)\*100" --include="*.go"  # Drawdown calcs
grep -r ", _.*strconv" --include="*.go"       # Error suppression
```

### Monitor Progress
```bash
wc -l backtest/constants.go backtest/utils.go  # Verify additions
ls -lh /tmp/nofx                              # Check binary
git diff --stat                               # View changes
```

---

## Documentation Reference

**For Details About**:
- Magic number locations → `PHASE_2_REFACTORING_AUDIT.md`
- Exact line numbers → Attached audit document
- Implementation examples → `PHASE_2_IMPLEMENTATION_GUIDE.md`
- Quick lookups → `PHASE_2_QUICK_REFERENCE.md`

---

## Next Immediate Actions

1. **Start Phase 2.4**: Apply constants from `backtest/constants.go`
2. **Priority**: Tier 1 files (decision/engine.go, backtest/runner.go)
3. **Verify**: Build after each tier
4. **Track**: Update this roadmap with completion status
5. **Document**: Keep audit checklist current

---

**Status**: ✅ Ready to Proceed  
**Current Phase**: 2.4 - Constants Application  
**Estimated Completion**: 15-22 hours of focused work  
**Risk Level**: LOW  

**Let's build better software! 🚀**

# Refactoring Implementation Roadmap

## Phase 2.4: Constants Application Strategy

### Overview
Apply 100+ constants from `backtest/constants.go` to eliminate remaining magic numbers throughout the codebase.

### Priority Order for Application

#### TIER 1: Core Decision-Making & Risk (HIGH IMPACT)
**Files:** `decision/engine.go`, `backtest/runner.go`  
**Estimated Locations:** 7 replacements  
**Impact:** High - Controls core trading logic

**decision/engine.go - Specific Locations to Update:**
```go
// Current: threshold := 65
// After: threshold := MinConfidenceForEntry

// Current: minSize := 75
// After: minSize := MinConfidenceForSize

// Current: maxDrawdown := 20.0
// After: maxDrawdown := DefaultMaxDrawdownPct

// Current: stopLoss := 3.0
// After: stopLoss := DefaultStopLossPct

// Current: takeProfit := 6.0
// After: takeProfit := DefaultTakeProfitPct
```

**backtest/runner.go - Specific Locations to Update:**
```go
// Current: checkpointInterval = 2 * time.Second
// After: checkpointInterval = time.Duration(DefaultCheckpointIntervalSeconds) * time.Second

// Current: maxDrawdown := 20.0
// After: maxDrawdown := DefaultMaxDrawdownPct

// Current: minTrades := 10
// After: minTrades := MinTradesForFeedback
```

#### TIER 2: Position & Leverage Management (HIGH IMPACT)
**Files:** `trader/auto_trader.go`, `trader/hyperliquid_trader.go`  
**Estimated Locations:** 8 replacements  
**Impact:** High - Controls position sizing and leverage

**trader/auto_trader.go:**
```go
// Current: minSize := 50.0
// After: minSize := DefaultMinPositionSize

// Current: maxSize := 1000.0
// After: maxSize := DefaultMaxPositionSize

// Current: maxPositions := 5
// After: maxPositions := MaxPositionsPerAccount

// Current: maxLeverage := 10
// After: maxLeverage := DefaultMaxLeverage

// Current: leverage := 3
// After: leverage := OptimalLeverage
```

**trader/hyperliquid_trader.go:**
```go
// Current: btcLeverage := 5
// After: btcLeverage := DefaultMaxLeverage * BTCETHLeverageMultiplier

// Current: altLeverage := 3
// After: altLeverage := OptimalLeverage * AltcoinLeverageMultiplier
```

#### TIER 3: Market Data & Timeframes (MEDIUM IMPACT)
**Files:** `market/data.go`, `market/websocket.go`  
**Estimated Locations:** 4 replacements  
**Impact:** Medium - Controls data quality thresholds

**market/data.go:**
```go
// Current: if klineCount < 50
// After: if klineCount < MinCandlesForValidAnalysis

// Current: if count < 10
// After: if count < MinimumKlineCount
```

**market/websocket.go:**
```go
// Current: timeout := 5 * time.Second
// After: timeout := time.Duration(WebsocketReconnectDelay) * time.Second

// Current: maxConcurrent := 10
// After: maxConcurrent := MaxConcurrentRequests
```

#### TIER 4: API & Configuration (MEDIUM IMPACT)
**Files:** `api/server.go`, `api/crypto_handler.go`  
**Estimated Locations:** 5 replacements  
**Impact:** Medium - Controls API behavior

**api/server.go:**
```go
// Current: maxRetries := 3
// After: maxRetries := aiDecisionMaxRetries

// Current: timeout := 30 * time.Second
// After: timeout := time.Duration(DefaultErrorWaitTime) * time.Second
```

#### TIER 5: Monitoring & Feedback (LOWER IMPACT)
**Files:** `backtest/feedback.go`, `backtest/factor_optimizer.go`  
**Estimated Locations:** 3 replacements  
**Impact:** Lower - Controls optimization behavior

---

## Phase 2.5: Code Duplication Consolidation Strategy

### Duplicate Pattern #1: Drawdown Calculations
**Current State:** 3 locations with similar drawdown calculation logic
**Files Affected:**
- `backtest/runner.go` (Line ~)
- `trader/auto_trader.go` (Line ~)
- `decision/engine.go` (Line ~)

**Current Pattern:**
```go
// Pattern 1
drawdown := ((peakEquity - currentEquity) / peakEquity) * 100.0

// Pattern 2
ratio := (peak - current) / peak * 100.0

// Pattern 3
dd := math.Abs((equity - peak) / peak) * 100.0
```

**Consolidation Solution:**
Already created in `backtest/utils.go`:
```go
func CalculateDrawdown(currentEquity, peakEquity float64) float64 {
    if peakEquity <= 0 { return 0 }
    if currentEquity >= peakEquity { return 0 }
    drawdown := ((peakEquity - currentEquity) / peakEquity) * 100.0
    return math.Max(0, drawdown)
}
```

**Implementation:**
1. Replace all 3 occurrences with call to `CalculateDrawdown()`
2. Verify behavior identical
3. Test edge cases

---

### Duplicate Pattern #2: Default Parameter Initialization
**Current State:** 4 locations initializing default parameters
**Files Affected:**
- `store/trader.go`
- `trader/auto_trader.go`
- `api/server.go`
- `market/data.go`

**Current Pattern:**
```go
// Pattern 1: Database defaults with COALESCE
COALESCE(btc_eth_leverage, 5), COALESCE(altcoin_leverage, 5)

// Pattern 2: Code-level defaults
if leverage == 0 { leverage = 3 }

// Pattern 3: Ternary-like defaults
leverage := leverage; if leverage == 0 { leverage = DefaultMaxLeverage }
```

**Consolidation Solution:**
Create helper function in `store/store.go`:
```go
func GetDefaultLeverage(current int, asset string) int {
    if current > 0 { return current }
    if asset == "BTC" || asset == "ETH" {
        return int(float64(OptimalLeverage) * BTCETHLeverageMultiplier)
    }
    return OptimalLeverage
}
```

**Implementation:**
1. Add helper functions to store/store.go
2. Replace all 4 occurrences
3. Update database schema where applicable

---

### Duplicate Pattern #3: Error Handling for Network Operations
**Current State:** 3 locations with similar retry/backoff logic
**Files Affected:**
- `trader/websocket.go`
- `market/websocket.go`
- `api/server.go`

**Current Pattern:**
```go
// Pattern 1
if err != nil {
    logger.Warnf("error: %v", err)
    time.Sleep(time.Duration(retries * 2) * time.Second)
    retries++
    if retries > 5 { break }
}

// Pattern 2
if err != nil {
    logger.Error(err)
    if retries < 3 {
        time.Sleep(2 * time.Second)
        retries++
    } else {
        return err
    }
}
```

**Consolidation Solution:**
Create helper in `trader/helper.go` or `util/retry.go`:
```go
type RetryConfig struct {
    MaxRetries  int
    InitialWait time.Duration
    BackoffMult float64
}

func RetryWithBackoff(ctx context.Context, cfg RetryConfig, fn func() error) error {
    var lastErr error
    wait := cfg.InitialWait
    
    for i := 0; i < cfg.MaxRetries; i++ {
        if err := fn(); err == nil {
            return nil
        } else {
            lastErr = err
        }
        
        if i < cfg.MaxRetries-1 {
            select {
            case <-time.After(wait):
                wait = time.Duration(float64(wait) * cfg.BackoffMult)
            case <-ctx.Done():
                return ctx.Err()
            }
        }
    }
    return lastErr
}
```

**Implementation:**
1. Create retry helper utility
2. Replace all 3 implementations
3. Test with various failure scenarios

---

### Duplicate Pattern #4: Window/Timeframe Validation
**Current State:** 2 locations validating timeframe windows
**Files Affected:**
- `market/data.go`
- `market/websocket.go`

**Current Pattern:**
```go
// Pattern 1
if timeframe != "1m" && timeframe != "5m" && timeframe != "15m" {
    return errors.New("invalid timeframe")
}

// Pattern 2
switch timeframe {
case "1m", "5m", "15m", "1h":
    // valid
default:
    return false
}
```

**Consolidation Solution:**
Already have constants in `backtest/constants.go`:
```go
const (
    DefaultTimeframe      = "15m"
    ConfirmationTimeframe = "5m"
    // ...
)
```

Create validator:
```go
func IsValidTimeframe(tf string) bool {
    validTimeframes := map[string]bool{
        "1m": true, "5m": true, "15m": true, "1h": true, "4h": true, "1d": true,
    }
    return validTimeframes[tf]
}
```

**Implementation:**
1. Add IsValidTimeframe() to util/validation.go
2. Replace both implementations
3. Centralize timeframe validation

---

## Implementation Priority Matrix

```
HIGH IMPACT, HIGH EFFORT:
- Constants Application (Tier 1-2): 15+ replacements, 2-3 hours
- Long Function Extraction (Phase 3.1): 15+ functions, 6-8 hours

HIGH IMPACT, LOW EFFORT:
- Drawdown Calculation Consolidation: 3 replacements, 30 minutes
- Error Handling Consolidation: 3 replacements, 1 hour
- Timeframe Validation: 2 replacements, 30 minutes

MEDIUM IMPACT, MEDIUM EFFORT:
- Parameter Initialization: 4 replacements, 1-2 hours
- Constants Application (Tier 3-4): 9+ replacements, 1-2 hours

LOW IMPACT, LOW EFFORT:
- Logging Standardization: ~20 replacements, 1-2 hours
```

---

## Testing Strategy

### Unit Tests
- Test all new utility functions before deployment
- Test edge cases (zero values, negative numbers, etc.)
- Test constants are applied correctly

### Integration Tests
- Verify decision engine still produces correct outputs
- Verify position sizing calculations unchanged
- Verify leverage adjustments work as expected

### Build Verification
- Run `go build -o /tmp/nofx .` after each phase
- Verify no compilation errors or warnings
- Run `go fmt ./...` to ensure formatting

### Behavioral Tests
- Verify trading logic unchanged after refactoring
- Compare metrics before/after refactoring
- Backtest with same parameters before/after

---

## Rollback Strategy

Each phase can be reverted independently:
```bash
# Phase 2.4 (Constants Application)
git checkout decision/engine.go backtest/runner.go trader/auto_trader.go ...

# Phase 2.5 (Duplication Consolidation)
git checkout [affected files]

# Phase 3.x (Maintainability Refactoring)
git checkout [affected files]
```

---

## Success Criteria

### Phase 2.4 Completion
- [ ] All 28 constant applications completed
- [ ] Build verification passing
- [ ] No behavioral changes in trading logic
- [ ] All tests passing
- [ ] Code review approved

### Phase 2.5 Completion
- [ ] All 12 duplication consolidations completed
- [ ] 100+ lines of code removed
- [ ] Maintenance burden reduced
- [ ] All tests passing

### Phase 3 Completion
- [ ] 15+ long functions extracted
- [ ] Cyclomatic complexity reduced
- [ ] Goroutine safety issues fixed
- [ ] Logging standardized

### Overall Success
- [ ] Code quality score improved by 30%+
- [ ] Maintenance time reduced by 20%+
- [ ] No functional changes to trading logic
- [ ] All refactoring documented

---

**Estimated Total Effort:** 18-27 hours
**Estimated Daily Progress:** 6-8 hours per session
**Estimated Completion:** 2-4 sessions

# Comprehensive Project Refactoring Plan

## Current Status ✅
- **Build**: Successful (46MB binary)
- **Go Files**: 186 files
- **Validation**: All dependencies resolved
- **Code Format**: All files properly formatted
- **Quality Issues Identified**: 33 distinct issues

## Refactoring Strategy

### Phase 1: Critical Fixes (High Priority)
**Duration**: 2-3 hours
**Impact**: Prevents crashes, improves stability

1. **Type Assertion Safety** (3 issues)
   - Files: market/api_client.go, backtest/ai_client.go
   - Issue: Unchecked type assertions that could cause panics
   - Fix: Add safety checks with `ok` boolean
   - Benefit: Eliminates crash risks

2. **Error Handling** (5 issues)
   - Files: decision/engine.go, market/data.go, backtest/runner.go
   - Issue: Silent error suppression with `_` variable
   - Fix: Proper error handling or logging
   - Benefit: Better debugging, fewer silent failures

3. **Complex Functions** (4 issues)
   - Files: backtest/runner.go (stepOnce: 238 lines)
   - Issue: Functions doing too much
   - Fix: Break into smaller, testable functions
   - Benefit: Easier to test and maintain

### Phase 2: Code Quality (Medium Priority)
**Duration**: 3-4 hours
**Impact**: Improves maintainability and consistency

4. **Magic Numbers** (6 issues)
   - Extract to named constants
   - Create configuration files
   - Benefit: Easier to adjust thresholds

5. **Duplicate Code Patterns** (4 issues)
   - Consolidate similar logic
   - Create helper functions
   - Benefit: Reduce maintenance burden

6. **Goroutine Safety** (3 issues)
   - Add proper synchronization
   - Use channels instead of shared state
   - Benefit: Eliminates race conditions

### Phase 3: Organization (Low Priority)
**Duration**: 2-3 hours
**Impact**: Better code organization

7. **Package Organization**
   - Review imports
   - Remove unused dependencies
   - Benefit: Cleaner architecture

8. **Naming Consistency**
   - Standardize variable names
   - Align with Go conventions
   - Benefit: Better readability

9. **Logging Standardization**
   - Remove fmt.Printf from production code
   - Use logger package consistently
   - Benefit: Centralized logging control

## Execution Plan

### Quick Wins (Start Here)
1. ✅ Format all code (go fmt) - DONE
2. ✅ Validate build - DONE
3. 🔄 Add type assertion safety checks
4. 🔄 Fix silent error suppressions
5. 🔄 Extract magic numbers to constants

### Main Refactoring
6. 🔄 Refactor complex functions
7. 🔄 Consolidate duplicate patterns
8. 🔄 Fix goroutine safety issues
9. 🔄 Standardize logging

### Final Polish
10. 🔄 Package reorganization
11. 🔄 Naming consistency review
12. 🔄 Final build verification

## Files to Prioritize

### High Impact (Address First)
1. **backtest/runner.go** - 238-line stepOnce() function
2. **decision/engine.go** - Missing error handling
3. **market/api_client.go** - Unsafe type assertions
4. **market/websocket.go** - Goroutine safety issues
5. **backtest/ai_client.go** - Unsafe parsing

### Medium Impact (Address Next)
6. **store/strategy.go** - Magic numbers
7. **decision/formatter.go** - Code duplication
8. **manager/trader_manager.go** - Error handling

### Low Impact (Polish)
9. **logger/** - Logging standardization
10. **crypto/** - Naming consistency

## Success Criteria

- ✅ Zero unchecked type assertions
- ✅ All errors handled properly
- ✅ No functions >50 lines of logic
- ✅ All magic numbers replaced with constants
- ✅ No code duplication >3 lines
- ✅ No goroutine safety issues
- ✅ Consistent logging throughout
- ✅ All tests pass
- ✅ Build succeeds with no warnings
- ✅ Code coverage maintained or improved

## Timeline

- **Phase 1**: 2-3 hours (Critical fixes)
- **Phase 2**: 3-4 hours (Quality improvements)
- **Phase 3**: 2-3 hours (Organization)
- **Testing**: 1-2 hours (Verification)
- **Total**: ~9-12 hours

## Risk Mitigation

- Git commits after each phase
- Regular build verification
- Backup of original code
- No changes to external APIs
- 100% backward compatibility maintained

---

## Starting Phase 1: Critical Fixes

Now beginning systematic refactoring...

# Refactoring Implementation Roadmap

## Phase 2.4: Constants Application Strategy

### Overview
Apply 100+ constants from `backtest/constants.go` to eliminate remaining magic numbers throughout the codebase.

### Priority Order for Application

#### TIER 1: Core Decision-Making & Risk (HIGH IMPACT)
**Files:** `decision/engine.go`, `backtest/runner.go`  
**Estimated Locations:** 7 replacements  
**Impact:** High - Controls core trading logic

**decision/engine.go - Specific Locations to Update:**
```go
// Current: threshold := 65
// After: threshold := MinConfidenceForEntry

// Current: minSize := 75
// After: minSize := MinConfidenceForSize

// Current: maxDrawdown := 20.0
// After: maxDrawdown := DefaultMaxDrawdownPct

// Current: stopLoss := 3.0
// After: stopLoss := DefaultStopLossPct

// Current: takeProfit := 6.0
// After: takeProfit := DefaultTakeProfitPct
```

**backtest/runner.go - Specific Locations to Update:**
```go
// Current: checkpointInterval = 2 * time.Second
// After: checkpointInterval = time.Duration(DefaultCheckpointIntervalSeconds) * time.Second

// Current: maxDrawdown := 20.0
// After: maxDrawdown := DefaultMaxDrawdownPct

// Current: minTrades := 10
// After: minTrades := MinTradesForFeedback
```

#### TIER 2: Position & Leverage Management (HIGH IMPACT)
**Files:** `trader/auto_trader.go`, `trader/hyperliquid_trader.go`  
**Estimated Locations:** 8 replacements  
**Impact:** High - Controls position sizing and leverage

**trader/auto_trader.go:**
```go
// Current: minSize := 50.0
// After: minSize := DefaultMinPositionSize

// Current: maxSize := 1000.0
// After: maxSize := DefaultMaxPositionSize

// Current: maxPositions := 5
// After: maxPositions := MaxPositionsPerAccount

// Current: maxLeverage := 10
// After: maxLeverage := DefaultMaxLeverage

// Current: leverage := 3
// After: leverage := OptimalLeverage
```

**trader/hyperliquid_trader.go:**
```go
// Current: btcLeverage := 5
// After: btcLeverage := DefaultMaxLeverage * BTCETHLeverageMultiplier

// Current: altLeverage := 3
// After: altLeverage := OptimalLeverage * AltcoinLeverageMultiplier
```

#### TIER 3: Market Data & Timeframes (MEDIUM IMPACT)
**Files:** `market/data.go`, `market/websocket.go`  
**Estimated Locations:** 4 replacements  
**Impact:** Medium - Controls data quality thresholds

**market/data.go:**
```go
// Current: if klineCount < 50
// After: if klineCount < MinCandlesForValidAnalysis

// Current: if count < 10
// After: if count < MinimumKlineCount
```

**market/websocket.go:**
```go
// Current: timeout := 5 * time.Second
// After: timeout := time.Duration(WebsocketReconnectDelay) * time.Second

// Current: maxConcurrent := 10
// After: maxConcurrent := MaxConcurrentRequests
```

#### TIER 4: API & Configuration (MEDIUM IMPACT)
**Files:** `api/server.go`, `api/crypto_handler.go`  
**Estimated Locations:** 5 replacements  
**Impact:** Medium - Controls API behavior

**api/server.go:**
```go
// Current: maxRetries := 3
// After: maxRetries := aiDecisionMaxRetries

// Current: timeout := 30 * time.Second
// After: timeout := time.Duration(DefaultErrorWaitTime) * time.Second
```

#### TIER 5: Monitoring & Feedback (LOWER IMPACT)
**Files:** `backtest/feedback.go`, `backtest/factor_optimizer.go`  
**Estimated Locations:** 3 replacements  
**Impact:** Lower - Controls optimization behavior

---

## Phase 2.5: Code Duplication Consolidation Strategy

### Duplicate Pattern #1: Drawdown Calculations
**Current State:** 3 locations with similar drawdown calculation logic
**Files Affected:**
- `backtest/runner.go` (Line ~)
- `trader/auto_trader.go` (Line ~)
- `decision/engine.go` (Line ~)

**Current Pattern:**
```go
// Pattern 1
drawdown := ((peakEquity - currentEquity) / peakEquity) * 100.0

// Pattern 2
ratio := (peak - current) / peak * 100.0

// Pattern 3
dd := math.Abs((equity - peak) / peak) * 100.0
```

**Consolidation Solution:**
Already created in `backtest/utils.go`:
```go
func CalculateDrawdown(currentEquity, peakEquity float64) float64 {
    if peakEquity <= 0 { return 0 }
    if currentEquity >= peakEquity { return 0 }
    drawdown := ((peakEquity - currentEquity) / peakEquity) * 100.0
    return math.Max(0, drawdown)
}
```

**Implementation:**
1. Replace all 3 occurrences with call to `CalculateDrawdown()`
2. Verify behavior identical
3. Test edge cases

---

### Duplicate Pattern #2: Default Parameter Initialization
**Current State:** 4 locations initializing default parameters
**Files Affected:**
- `store/trader.go`
- `trader/auto_trader.go`
- `api/server.go`
- `market/data.go`

**Current Pattern:**
```go
// Pattern 1: Database defaults with COALESCE
COALESCE(btc_eth_leverage, 5), COALESCE(altcoin_leverage, 5)

// Pattern 2: Code-level defaults
if leverage == 0 { leverage = 3 }

// Pattern 3: Ternary-like defaults
leverage := leverage; if leverage == 0 { leverage = DefaultMaxLeverage }
```

**Consolidation Solution:**
Create helper function in `store/store.go`:
```go
func GetDefaultLeverage(current int, asset string) int {
    if current > 0 { return current }
    if asset == "BTC" || asset == "ETH" {
        return int(float64(OptimalLeverage) * BTCETHLeverageMultiplier)
    }
    return OptimalLeverage
}
```

**Implementation:**
1. Add helper functions to store/store.go
2. Replace all 4 occurrences
3. Update database schema where applicable

---

### Duplicate Pattern #3: Error Handling for Network Operations
**Current State:** 3 locations with similar retry/backoff logic
**Files Affected:**
- `trader/websocket.go`
- `market/websocket.go`
- `api/server.go`

**Current Pattern:**
```go
// Pattern 1
if err != nil {
    logger.Warnf("error: %v", err)
    time.Sleep(time.Duration(retries * 2) * time.Second)
    retries++
    if retries > 5 { break }
}

// Pattern 2
if err != nil {
    logger.Error(err)
    if retries < 3 {
        time.Sleep(2 * time.Second)
        retries++
    } else {
        return err
    }
}
```

**Consolidation Solution:**
Create helper in `trader/helper.go` or `util/retry.go`:
```go
type RetryConfig struct {
    MaxRetries  int
    InitialWait time.Duration
    BackoffMult float64
}

func RetryWithBackoff(ctx context.Context, cfg RetryConfig, fn func() error) error {
    var lastErr error
    wait := cfg.InitialWait
    
    for i := 0; i < cfg.MaxRetries; i++ {
        if err := fn(); err == nil {
            return nil
        } else {
            lastErr = err
        }
        
        if i < cfg.MaxRetries-1 {
            select {
            case <-time.After(wait):
                wait = time.Duration(float64(wait) * cfg.BackoffMult)
            case <-ctx.Done():
                return ctx.Err()
            }
        }
    }
    return lastErr
}
```

**Implementation:**
1. Create retry helper utility
2. Replace all 3 implementations
3. Test with various failure scenarios

---

### Duplicate Pattern #4: Window/Timeframe Validation
**Current State:** 2 locations validating timeframe windows
**Files Affected:**
- `market/data.go`
- `market/websocket.go`

**Current Pattern:**
```go
// Pattern 1
if timeframe != "1m" && timeframe != "5m" && timeframe != "15m" {
    return errors.New("invalid timeframe")
}

// Pattern 2
switch timeframe {
case "1m", "5m", "15m", "1h":
    // valid
default:
    return false
}
```

**Consolidation Solution:**
Already have constants in `backtest/constants.go`:
```go
const (
    DefaultTimeframe      = "15m"
    ConfirmationTimeframe = "5m"
    // ...
)
```

Create validator:
```go
func IsValidTimeframe(tf string) bool {
    validTimeframes := map[string]bool{
        "1m": true, "5m": true, "15m": true, "1h": true, "4h": true, "1d": true,
    }
    return validTimeframes[tf]
}
```

**Implementation:**
1. Add IsValidTimeframe() to util/validation.go
2. Replace both implementations
3. Centralize timeframe validation

---

## Implementation Priority Matrix

```
HIGH IMPACT, HIGH EFFORT:
- Constants Application (Tier 1-2): 15+ replacements, 2-3 hours
- Long Function Extraction (Phase 3.1): 15+ functions, 6-8 hours

HIGH IMPACT, LOW EFFORT:
- Drawdown Calculation Consolidation: 3 replacements, 30 minutes
- Error Handling Consolidation: 3 replacements, 1 hour
- Timeframe Validation: 2 replacements, 30 minutes

MEDIUM IMPACT, MEDIUM EFFORT:
- Parameter Initialization: 4 replacements, 1-2 hours
- Constants Application (Tier 3-4): 9+ replacements, 1-2 hours

LOW IMPACT, LOW EFFORT:
- Logging Standardization: ~20 replacements, 1-2 hours
```

---

## Testing Strategy

### Unit Tests
- Test all new utility functions before deployment
- Test edge cases (zero values, negative numbers, etc.)
- Test constants are applied correctly

### Integration Tests
- Verify decision engine still produces correct outputs
- Verify position sizing calculations unchanged
- Verify leverage adjustments work as expected

### Build Verification
- Run `go build -o /tmp/nofx .` after each phase
- Verify no compilation errors or warnings
- Run `go fmt ./...` to ensure formatting

### Behavioral Tests
- Verify trading logic unchanged after refactoring
- Compare metrics before/after refactoring
- Backtest with same parameters before/after

---

## Rollback Strategy

Each phase can be reverted independently:
```bash
# Phase 2.4 (Constants Application)
git checkout decision/engine.go backtest/runner.go trader/auto_trader.go ...

# Phase 2.5 (Duplication Consolidation)
git checkout [affected files]

# Phase 3.x (Maintainability Refactoring)
git checkout [affected files]
```

---

## Success Criteria

### Phase 2.4 Completion
- [ ] All 28 constant applications completed
- [ ] Build verification passing
- [ ] No behavioral changes in trading logic
- [ ] All tests passing
- [ ] Code review approved

### Phase 2.5 Completion
- [ ] All 12 duplication consolidations completed
- [ ] 100+ lines of code removed
- [ ] Maintenance burden reduced
- [ ] All tests passing

### Phase 3 Completion
- [ ] 15+ long functions extracted
- [ ] Cyclomatic complexity reduced
- [ ] Goroutine safety issues fixed
- [ ] Logging standardized

### Overall Success
- [ ] Code quality score improved by 30%+
- [ ] Maintenance time reduced by 20%+
- [ ] No functional changes to trading logic
- [ ] All refactoring documented

---

**Estimated Total Effort:** 18-27 hours
**Estimated Daily Progress:** 6-8 hours per session
**Estimated Completion:** 2-4 sessions

# Comprehensive Project Refactoring Plan

## Current Status ✅
- **Build**: Successful (46MB binary)
- **Go Files**: 186 files
- **Validation**: All dependencies resolved
- **Code Format**: All files properly formatted
- **Quality Issues Identified**: 33 distinct issues

## Refactoring Strategy

### Phase 1: Critical Fixes (High Priority)
**Duration**: 2-3 hours
**Impact**: Prevents crashes, improves stability

1. **Type Assertion Safety** (3 issues)
   - Files: market/api_client.go, backtest/ai_client.go
   - Issue: Unchecked type assertions that could cause panics
   - Fix: Add safety checks with `ok` boolean
   - Benefit: Eliminates crash risks

2. **Error Handling** (5 issues)
   - Files: decision/engine.go, market/data.go, backtest/runner.go
   - Issue: Silent error suppression with `_` variable
   - Fix: Proper error handling or logging
   - Benefit: Better debugging, fewer silent failures

3. **Complex Functions** (4 issues)
   - Files: backtest/runner.go (stepOnce: 238 lines)
   - Issue: Functions doing too much
   - Fix: Break into smaller, testable functions
   - Benefit: Easier to test and maintain

### Phase 2: Code Quality (Medium Priority)
**Duration**: 3-4 hours
**Impact**: Improves maintainability and consistency

4. **Magic Numbers** (6 issues)
   - Extract to named constants
   - Create configuration files
   - Benefit: Easier to adjust thresholds

5. **Duplicate Code Patterns** (4 issues)
   - Consolidate similar logic
   - Create helper functions
   - Benefit: Reduce maintenance burden

6. **Goroutine Safety** (3 issues)
   - Add proper synchronization
   - Use channels instead of shared state
   - Benefit: Eliminates race conditions

### Phase 3: Organization (Low Priority)
**Duration**: 2-3 hours
**Impact**: Better code organization

7. **Package Organization**
   - Review imports
   - Remove unused dependencies
   - Benefit: Cleaner architecture

8. **Naming Consistency**
   - Standardize variable names
   - Align with Go conventions
   - Benefit: Better readability

9. **Logging Standardization**
   - Remove fmt.Printf from production code
   - Use logger package consistently
   - Benefit: Centralized logging control

## Execution Plan

### Quick Wins (Start Here)
1. ✅ Format all code (go fmt) - DONE
2. ✅ Validate build - DONE
3. 🔄 Add type assertion safety checks
4. 🔄 Fix silent error suppressions
5. 🔄 Extract magic numbers to constants

### Main Refactoring
6. 🔄 Refactor complex functions
7. 🔄 Consolidate duplicate patterns
8. 🔄 Fix goroutine safety issues
9. 🔄 Standardize logging

### Final Polish
10. 🔄 Package reorganization
11. 🔄 Naming consistency review
12. 🔄 Final build verification

## Files to Prioritize

### High Impact (Address First)
1. **backtest/runner.go** - 238-line stepOnce() function
2. **decision/engine.go** - Missing error handling
3. **market/api_client.go** - Unsafe type assertions
4. **market/websocket.go** - Goroutine safety issues
5. **backtest/ai_client.go** - Unsafe parsing

### Medium Impact (Address Next)
6. **store/strategy.go** - Magic numbers
7. **decision/formatter.go** - Code duplication
8. **manager/trader_manager.go** - Error handling

### Low Impact (Polish)
9. **logger/** - Logging standardization
10. **crypto/** - Naming consistency

## Success Criteria

- ✅ Zero unchecked type assertions
- ✅ All errors handled properly
- ✅ No functions >50 lines of logic
- ✅ All magic numbers replaced with constants
- ✅ No code duplication >3 lines
- ✅ No goroutine safety issues
- ✅ Consistent logging throughout
- ✅ All tests pass
- ✅ Build succeeds with no warnings
- ✅ Code coverage maintained or improved

## Timeline

- **Phase 1**: 2-3 hours (Critical fixes)
- **Phase 2**: 3-4 hours (Quality improvements)
- **Phase 3**: 2-3 hours (Organization)
- **Testing**: 1-2 hours (Verification)
- **Total**: ~9-12 hours

## Risk Mitigation

- Git commits after each phase
- Regular build verification
- Backup of original code
- No changes to external APIs
- 100% backward compatibility maintained

---

## Starting Phase 1: Critical Fixes

Now beginning systematic refactoring...
