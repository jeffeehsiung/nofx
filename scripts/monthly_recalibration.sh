#!/usr/bin/env bash
set -euo pipefail

# NOFX Monthly Threshold Recalibration
# Usage: ./scripts/monthly_recalibration.sh

ROOT_DIR="$(cd "$(dirname "$0")"/.. && pwd)"
cd "$ROOT_DIR"

echo "[monthly] Starting threshold recalibration..."

go run ./cmd/recalibrate \
  --lookback-days 60 \
  --min-trades 100 \
  --max-trades 2000 \
  --output ./config/calibrated_thresholds.json

echo "[monthly] Done. Review config/calibrated_thresholds.json and deploy."
