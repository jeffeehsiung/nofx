# Quick Command Reference

## Feature Flag Controls

```bash
# Enable adaptive microstructure (percentile-based multipliers)
export FEATURE_ADAPTIVE_MICROSTRUCTURE=true

# Disable disk-based config loading (force runtime calibration)
export FEATURE_CALIBRATE_STARTUP=false

# Set drift alert threshold to 15% (default 10%)
export FEATURE_DRIFT_ALERT_THRESHOLD_PCT=15

# Enable verbose drift logging
export FEATURE_VERBOSE_DRIFT_LOGGING=true

# View current settings
echo "Flags: ${FEATURE_ADAPTIVE_MICROSTRUCTURE:-false}, ${FEATURE_CALIBRATE_STARTUP:-true}, ${FEATURE_DRIFT_ALERT_THRESHOLD_PCT:-10}, ${FEATURE_VERBOSE_DRIFT_LOGGING:-false}"
```

## Building

```bash
# Build CLI tool
go build -o nofx-recalibrate ./cmd/recalibrate

# Build all packages
go build ./config ./market ./backtest

# Run any package
go run ./cmd/main [args]
```

## Testing

```bash
# Test market package (should pass regardless of feature flags)
go test ./market -v

# Test with adaptive multipliers enabled
FEATURE_ADAPTIVE_MICROSTRUCTURE=true go test ./market -v

# Run specific test
go test ./market -v -run TestDetectLargeOrders

# Full test suite
go test ./... -count=1
```

## Monthly Recalibration

```bash
# Basic recalibration (1000 trades)
./nofx-recalibrate --max-trades 1000

# Thorough recalibration (3000 trades)
./nofx-recalibrate --max-trades 3000

# With drift comparison (compare to backup)
./nofx-recalibrate \
  --max-trades 3000 \
  --output ./config/calibrated_thresholds.json \
  --current-thresholds ./config/calibrated_thresholds.json.backup \
  --drift-report ./config/drift_report_$(date +%Y%m).json

# Check drift report
cat ./config/drift_report_*.json | jq .driftAlerts

# Exclude specific run from calibration
./nofx-recalibrate --max-trades 1000 --exclude-run "run_id_here"
```

## Configuration Management

```bash
# Backup current config
cp ./config/calibrated_thresholds.json ./config/calibrated_thresholds.json.backup

# View current config
cat ./config/calibrated_thresholds.json | jq .

# Validate JSON
jq . ./config/calibrated_thresholds.json > /dev/null && echo "✓ Valid"

# Compare old vs new
diff <(jq .thresholds ./config/calibrated_thresholds.json.backup) \
     <(jq .thresholds ./config/calibrated_thresholds.json)

# Remove old configs (keep last 3 months)
ls -t ./config/calibrated_thresholds.*.json | tail -n +4 | xargs rm
```

## Deployment Scenarios

### Scenario 1: Conservative Production
```bash
# Keep adaptive disabled, use disk cache
export FEATURE_ADAPTIVE_MICROSTRUCTURE=false
export FEATURE_CALIBRATE_STARTUP=true
go run ./cmd/main ...
```

### Scenario 2: Aggressive High-Liquidity Trading
```bash
# Enable adaptive, tight drift detection
export FEATURE_ADAPTIVE_MICROSTRUCTURE=true
export FEATURE_DRIFT_ALERT_THRESHOLD_PCT=5
go run ./cmd/main ...
```

### Scenario 3: Development/Debug
```bash
# All verbose, no disk cache
export FEATURE_ADAPTIVE_MICROSTRUCTURE=true
export FEATURE_CALIBRATE_STARTUP=false
export FEATURE_VERBOSE_DRIFT_LOGGING=true
go run ./cmd/main ...
```

## Cron Jobs

### Monthly Recalibration (1st of month, 3 AM)
```bash
0 3 1 * * /path/to/nofx-recalibrate \
  --max-trades 3000 \
  --output /var/nofx/config/calibrated_thresholds.json \
  --current-thresholds /var/nofx/config/calibrated_thresholds.json.last \
  --drift-report /var/nofx/logs/drift_$(date +\%Y\%m).json
```

### Weekly Drift Check
```bash
0 9 * * 1 grep "Threshold drift" /var/log/nofx/*.log | mail -s "Weekly Drift Report" admin@example.com
```

### Cleanup Old Reports (Keep 6 months)
```bash
0 4 1 * * find /var/nofx/logs -name "drift_*.json" -mtime +180 -delete
```

## Troubleshooting Commands

```bash
# Check if config file exists and is valid
test -f ./config/calibrated_thresholds.json && echo "✓ File exists" || echo "✗ File missing"
jq . ./config/calibrated_thresholds.json > /dev/null 2>&1 && echo "✓ Valid JSON" || echo "✗ Invalid JSON"

# Check sample size
jq .sampleSize ./config/calibrated_thresholds.json

# Check calibration status
jq .calibrationOk ./config/calibrated_thresholds.json

# View calibration timestamp
jq .generatedAt ./config/calibrated_thresholds.json

# List all threshold values
jq .thresholds ./config/calibrated_thresholds.json

# Check drift report
test -f ./config/drift_report_*.json && echo "✓ Drift report found" || echo "✗ No drift report"

# Extract triggered alerts only
jq '.driftAlerts[] | select(.alertTriggered == true)' ./config/drift_report_*.json

# Build CLI with debug info
go build -o nofx-recalibrate -ldflags="-s=false -w=false" ./cmd/recalibrate
```

## Environment Preset Files

### .env.production
```bash
FEATURE_ADAPTIVE_MICROSTRUCTURE=false
FEATURE_CALIBRATE_STARTUP=true
FEATURE_DRIFT_ALERT_THRESHOLD_PCT=10
FEATURE_VERBOSE_DRIFT_LOGGING=false
```

### .env.development
```bash
FEATURE_ADAPTIVE_MICROSTRUCTURE=true
FEATURE_CALIBRATE_STARTUP=false
FEATURE_DRIFT_ALERT_THRESHOLD_PCT=5
FEATURE_VERBOSE_DRIFT_LOGGING=true
```

### Load from file
```bash
source .env.production
go run ./cmd/main ...
```

## Monitoring & Alerts

```bash
# Watch for drift alerts in real-time
tail -f /var/log/nofx/backtest.log | grep "Threshold drift"

# Count how many times drift detected (per hour)
grep "Threshold drift" /var/log/nofx/backtest.log | \
  sed 's/ .*//' | uniq -c | tail -24

# Alert if config too old (>45 days)
AGE=$(stat -f%B ./config/calibrated_thresholds.json | xargs -I {} date -r {} +%s)
NOW=$(date +%s)
DAYS=$(( ($NOW - $AGE) / 86400 ))
if [ $DAYS -gt 45 ]; then
  echo "⚠️  Config is $DAYS days old, consider recalibration"
fi

# Check if minimum samples met
SAMPLES=$(jq .sampleSize ./config/calibrated_thresholds.json)
if [ $SAMPLES -lt 30 ]; then
  echo "⚠️  Only $SAMPLES samples, need >=30 for valid calibration"
fi
```

## Performance Checks

```bash
# Time to load disk config vs runtime calibration
time ./nofx-recalibrate --max-trades 500  # Typical: 200-500ms

# Compare startup times
time (FEATURE_CALIBRATE_STARTUP=true go run ./cmd/main ...)   # With disk cache
time (FEATURE_CALIBRATE_STARTUP=false go run ./cmd/main ...) # Runtime calibration

# Memory usage
/usr/bin/time -v go run ./cmd/main ...
```
