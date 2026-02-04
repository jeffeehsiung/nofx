#!/bin/bash

##############################################################################
# A/B Testing Execution Script
# Purpose: Run baseline and treatment backtests with identical conditions
# Usage: ./run_ab_tests.sh [baseline|treatment|both|analyze]
##############################################################################

set -e

# Configuration
API_BASE="${API_BASE:-http://localhost:8080/api}"
TOKEN="${NOFX_TOKEN:-your_auth_token_here}"

# Test timestamps (7-day period: Jan 11-18, 2026)
START_TS=1736553600    # 2026-01-11 00:00:00 UTC
END_TS=1737158400      # 2026-01-18 00:00:00 UTC
DURATION_DAYS=7

# Run IDs
BASELINE_RUN="ab_baseline_20260112"
TREATMENT_RUN="ab_treatment_20260112"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

##############################################################################
# Helper Functions
##############################################################################

log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warn() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_separator() {
    echo "════════════════════════════════════════════════════════════"
}

##############################################################################
# API Functions
##############################################################################

check_api_health() {
    log_info "Checking API health..."

    response=$(curl -s -w "%{http_code}" -o /tmp/health.json \
        -X GET "${API_BASE}/backtest/runs?limit=1" \
        -H "Authorization: Bearer ${TOKEN}")

    if [ "$response" = "200" ]; then
        log_success "API is healthy"
        return 0
    else
        log_error "API returned status $response"
        return 1
    fi
}

start_backtest() {
    local run_id=$1
    local use_smart=$2

    log_info "Starting backtest: $run_id (use_smart_heuristics=$use_smart)"

    config=$(cat <<EOF
{
  "config": {
    "run_id": "$run_id",
    "symbols": ["BTCUSDT", "ETHUSDT"],
    "timeframes": ["3m", "15m", "4h"],
    "decision_timeframe": "3m",
    "decision_cadence_nbars": 20,
    "start_ts": $START_TS,
    "end_ts": $END_TS,
    "initial_balance": 10000,
    "fee_bps": 10,
    "slippage_bps": 5,
    "fill_policy": "market",
    "ai_model_id": "gpt-4",
    "cache_ai": true,
    "use_smart_heuristics": $use_smart,
    "ai": {
      "provider": "openai",
      "model": "gpt-4",
      "temperature": 0.7
    }
  }
}
EOF
)

    response=$(curl -s -w "\n%{http_code}" -X POST "${API_BASE}/backtest/start" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${TOKEN}" \
        -d "$config")

    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n-1)

    if [ "$http_code" = "200" ]; then
        log_success "Backtest started: $run_id"
        echo "$body" | jq '.' 2>/dev/null || echo "$body"
        return 0
    else
        log_error "Failed to start backtest (HTTP $http_code)"
        echo "$body"
        return 1
    fi
}

wait_for_backtest() {
    local run_id=$1
    local max_wait=14400  # 4 hours max
    local interval=30     # Check every 30 seconds
    local elapsed=0

    log_info "Waiting for backtest to complete: $run_id (max wait: 4 hours)"

    while [ $elapsed -lt $max_wait ]; do
        response=$(curl -s -X GET "${API_BASE}/backtest/status?run_id=$run_id" \
            -H "Authorization: Bearer ${TOKEN}")

        state=$(echo "$response" | jq -r '.state' 2>/dev/null)
        progress=$(echo "$response" | jq -r '.progress' 2>/dev/null)

        if [ "$state" = "COMPLETED" ]; then
            log_success "Backtest completed: $run_id"
            return 0
        elif [ "$state" = "FAILED" ] || [ "$state" = "ERROR" ]; then
            log_error "Backtest failed: $run_id"
            log_error "Error: $(echo "$response" | jq -r '.error' 2>/dev/null)"
            return 1
        fi

        echo -ne "\rProgress: $progress (elapsed: ${elapsed}s)\033[K"

        sleep $interval
        elapsed=$((elapsed + interval))
    done

    log_error "Backtest timeout after $max_wait seconds"
    return 1
}

get_metrics() {
    local run_id=$1

    log_info "Fetching metrics for: $run_id"

    response=$(curl -s -X GET "${API_BASE}/backtest/metrics?run_id=$run_id" \
        -H "Authorization: Bearer ${TOKEN}")

    echo "$response"
}

get_trades() {
    local run_id=$1

    log_info "Fetching trades for: $run_id"

    response=$(curl -s -X GET "${API_BASE}/backtest/trades?run_id=$run_id&limit=1000" \
        -H "Authorization: Bearer ${TOKEN}")

    echo "$response"
}

##############################################################################
# Analysis Functions
##############################################################################

compare_metrics() {
    local baseline_metrics=$1
    local treatment_metrics=$2

    print_separator
    log_info "A/B TEST COMPARISON ANALYSIS"
    print_separator

    # Extract key metrics
    baseline_wr=$(echo "$baseline_metrics" | jq -r '.win_rate' 2>/dev/null | awk '{printf "%.2f%%", $1*100}')
    treatment_wr=$(echo "$treatment_metrics" | jq -r '.win_rate' 2>/dev/null | awk '{printf "%.2f%%", $1*100}')

    baseline_sharpe=$(echo "$baseline_metrics" | jq -r '.sharpe_ratio' 2>/dev/null)
    treatment_sharpe=$(echo "$treatment_metrics" | jq -r '.sharpe_ratio' 2>/dev/null)

    baseline_dd=$(echo "$baseline_metrics" | jq -r '.max_drawdown' 2>/dev/null | awk '{printf "%.2f%%", $1*100}')
    treatment_dd=$(echo "$treatment_metrics" | jq -r '.max_drawdown' 2>/dev/null | awk '{printf "%.2f%%", $1*100}')

    baseline_pnl=$(echo "$baseline_metrics" | jq -r '.total_pnl' 2>/dev/null | awk '{printf "$%.2f", $1}')
    treatment_pnl=$(echo "$treatment_metrics" | jq -r '.total_pnl' 2>/dev/null | awk '{printf "$%.2f", $1}')

    # Print comparison
    echo ""
    echo "METRIC COMPARISON:"
    echo "  Metric              │  Baseline  │  Treatment │  Change"
    echo "  ─────────────────────┼────────────┼────────────┼─────────────"

    printf "  Win Rate            │  %8s  │  %8s  │ " "$baseline_wr" "$treatment_wr"
    baseline_wr_num=$(echo "$baseline_metrics" | jq -r '.win_rate' 2>/dev/null)
    treatment_wr_num=$(echo "$treatment_metrics" | jq -r '.win_rate' 2>/dev/null)
    wr_change=$(echo "$treatment_wr_num - $baseline_wr_num" | bc | awk '{printf "%.2f%%", $1*100}')
    echo "$wr_change"

    printf "  Sharpe Ratio        │  %8s  │  %8s  │ " "$baseline_sharpe" "$treatment_sharpe"
    sharpe_change=$(echo "$treatment_sharpe - $baseline_sharpe" | bc)
    if [ $(echo "$sharpe_change > 0" | bc) -eq 1 ]; then
        echo "+$sharpe_change ✅"
    else
        echo "$sharpe_change ❌"
    fi

    printf "  Max Drawdown        │  %8s  │  %8s  │ " "$baseline_dd" "$treatment_dd"
    echo "(lower is better)"

    printf "  Total P&L           │  %8s  │  %8s  │ " "$baseline_pnl" "$treatment_pnl"
    baseline_pnl_num=$(echo "$baseline_metrics" | jq -r '.total_pnl' 2>/dev/null)
    treatment_pnl_num=$(echo "$treatment_metrics" | jq -r '.total_pnl' 2>/dev/null)
    pnl_change=$(echo "$treatment_pnl_num - $baseline_pnl_num" | bc | awk '{printf "$%.2f", $1}')
    echo "$pnl_change"

    echo ""
    print_separator

    # Success criteria
    log_info "CHECKING SUCCESS CRITERIA..."
    echo ""

    # Criterion 1: Win rate improvement ≥5%
    wr_improvement=$(echo "$treatment_wr_num - $baseline_wr_num" | bc)
    if [ $(echo "$wr_improvement >= 0.05" | bc) -eq 1 ]; then
        log_success "Win Rate Improvement: +$(echo "$wr_improvement" | awk '{printf "%.2f%%", $1*100}') (≥5% target)"
    else
        log_warn "Win Rate Improvement: +$(echo "$wr_improvement" | awk '{printf "%.2f%%", $1*100}') (< 5% target)"
    fi

    # Criterion 2: Sharpe ratio improvement
    if [ $(echo "$sharpe_change > 0" | bc) -eq 1 ]; then
        log_success "Sharpe Ratio Improvement: +$sharpe_change"
    else
        log_warn "Sharpe Ratio Degradation: $sharpe_change"
    fi

    # Criterion 3: No major drawdown regression
    baseline_dd_num=$(echo "$baseline_metrics" | jq -r '.max_drawdown' 2>/dev/null)
    treatment_dd_num=$(echo "$treatment_metrics" | jq -r '.max_drawdown' 2>/dev/null)
    dd_change=$(echo "$treatment_dd_num - $baseline_dd_num" | bc)

    if [ $(echo "$dd_change <= 0.03" | bc) -eq 1 ]; then
        log_success "Max Drawdown No Major Regression: $(echo "$dd_change" | awk '{printf "%.2f%%", $1*100}')"
    else
        log_warn "Max Drawdown Regression: $(echo "$dd_change" | awk '{printf "%.2f%%", $1*100}')"
    fi

    echo ""
    print_separator
}

generate_report() {
    local baseline_metrics=$1
    local treatment_metrics=$2

    echo ""
    log_info "FINAL DECISION"
    print_separator

    baseline_wr=$(echo "$baseline_metrics" | jq -r '.win_rate' 2>/dev/null)
    treatment_wr=$(echo "$treatment_metrics" | jq -r '.win_rate' 2>/dev/null)
    wr_improvement=$(echo "$treatment_wr - $baseline_wr" | bc)

    if [ $(echo "$wr_improvement >= 0.05" | bc) -eq 1 ]; then
        log_success "✅ PASS: Win rate improved by $(echo "$wr_improvement" | awk '{printf "%.2f%%", $1*100}')"
        echo ""
        log_success "Approval: GREEN LIGHT for production deployment"
        echo "Next steps:"
        echo "  1. Run Week 4 canary deployment (10% of accounts)"
        echo "  2. Monitor metrics for 3-5 days"
        echo "  3. Scale to 100% if stable"
        return 0
    else
        log_error "❌ FAIL: Win rate improvement $(echo "$wr_improvement" | awk '{printf "%.2f%%", $1*100}'} < 5% target"
        echo ""
        log_warn "Recommendation: INVESTIGATE and RETEST"
        echo "Next steps:"
        echo "  1. Debug smart heuristics implementation"
        echo "  2. Check market data integration"
        echo "  3. Verify trade outcome recording"
        echo "  4. Run A/B test again with same conditions"
        return 1
    fi
}

##############################################################################
# Main Commands
##############################################################################

run_baseline() {
    print_separator
    log_info "RUNNING BASELINE TEST"
    log_info "Config: use_smart_heuristics=false"
    log_info "Period: $DURATION_DAYS days (Jan 11-18, 2026)"
    print_separator

    check_api_health || return 1
    start_backtest "$BASELINE_RUN" "false" || return 1
    wait_for_backtest "$BASELINE_RUN" || return 1

    baseline_metrics=$(get_metrics "$BASELINE_RUN")
    echo "$baseline_metrics" | jq '.' > /tmp/baseline_metrics.json

    log_success "Baseline metrics saved to /tmp/baseline_metrics.json"
    echo ""
    echo "$baseline_metrics" | jq '.'
}

run_treatment() {
    print_separator
    log_info "RUNNING TREATMENT TEST"
    log_info "Config: use_smart_heuristics=true"
    log_info "Period: $DURATION_DAYS days (Jan 11-18, 2026)"
    print_separator

    check_api_health || return 1
    start_backtest "$TREATMENT_RUN" "true" || return 1
    wait_for_backtest "$TREATMENT_RUN" || return 1

    treatment_metrics=$(get_metrics "$TREATMENT_RUN")
    echo "$treatment_metrics" | jq '.' > /tmp/treatment_metrics.json

    log_success "Treatment metrics saved to /tmp/treatment_metrics.json"
    echo ""
    echo "$treatment_metrics" | jq '.'
}

run_both() {
    run_baseline || return 1
    echo ""
    run_treatment || return 1
    echo ""
    analyze_results
}

analyze_results() {
    print_separator
    log_info "ANALYZING RESULTS"
    print_separator

    if [ ! -f /tmp/baseline_metrics.json ] || [ ! -f /tmp/treatment_metrics.json ]; then
        log_error "Missing metrics files. Run baseline and treatment tests first."
        return 1
    fi

    baseline_metrics=$(cat /tmp/baseline_metrics.json)
    treatment_metrics=$(cat /tmp/treatment_metrics.json)

    compare_metrics "$baseline_metrics" "$treatment_metrics"
    generate_report "$baseline_metrics" "$treatment_metrics"
}

##############################################################################
# Main Entry Point
##############################################################################

main() {
    local cmd="${1:-both}"

    case "$cmd" in
        baseline)
            run_baseline
            ;;
        treatment)
            run_treatment
            ;;
        both)
            run_both
            ;;
        analyze)
            analyze_results
            ;;
        *)
            echo "Usage: $0 [baseline|treatment|both|analyze]"
            echo ""
            echo "Commands:"
            echo "  baseline   - Run baseline backtest (use_smart_heuristics=false)"
            echo "  treatment  - Run treatment backtest (use_smart_heuristics=true)"
            echo "  both       - Run both tests sequentially"
            echo "  analyze    - Analyze saved results"
            echo ""
            echo "Environment Variables:"
            echo "  API_BASE   - API base URL (default: http://localhost:8080/api)"
            echo "  NOFX_TOKEN - Authentication token"
            exit 1
            ;;
    esac
}

main "$@"
