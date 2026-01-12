package config

import (
	"fmt"
	"os"
	"strconv"
)

// FeatureFlags controls experimental and adaptive behavior
type FeatureFlags struct {
	// EnableAdaptiveMicrostructure uses percentile-based dynamic multipliers
	// for large order and support/resistance detection instead of fixed thresholds.
	// Falls back to static 1.6x/5.0x if disabled.
	EnableAdaptiveMicrostructure bool

	// CalibrateOnStartup loads and applies calibrated thresholds from disk
	// if available (config/calibrated_thresholds.json). Gracefully falls back
	// to defaults if file doesn't exist or calibration insufficient.
	CalibrateOnStartup bool

	// DriftAlertThresholdPct triggers notification when new calibration
	// differs from current thresholds by more than this percentage (default 10%).
	DriftAlertThresholdPct float64

	// VerboseDriftLogging logs detailed threshold changes for debugging
	// calibration drift and anomalies.
	VerboseDriftLogging bool
}

// DefaultFeatureFlags returns sensible defaults
func DefaultFeatureFlags() FeatureFlags {
	return FeatureFlags{
		EnableAdaptiveMicrostructure: true, // ENABLED: adaptive multipliers by default (can be toggled via frontend)
		CalibrateOnStartup:           true, // Load calibrated thresholds if available
		DriftAlertThresholdPct:       10.0, // Alert on >10% drift
		VerboseDriftLogging:          false,
	}
}

// LoadFeatureFlagsFromEnv loads feature flags from environment variables
// Supports: FEATURE_ADAPTIVE_MICROSTRUCTURE=true/false
//
//	FEATURE_CALIBRATE_STARTUP=true/false
//	FEATURE_DRIFT_ALERT_PCT=<float>
//	FEATURE_VERBOSE_DRIFT_LOGGING=true/false
func LoadFeatureFlagsFromEnv() FeatureFlags {
	flags := DefaultFeatureFlags()

	if val := os.Getenv("FEATURE_ADAPTIVE_MICROSTRUCTURE"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			flags.EnableAdaptiveMicrostructure = b
		}
	}

	if val := os.Getenv("FEATURE_CALIBRATE_STARTUP"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			flags.CalibrateOnStartup = b
		}
	}

	if val := os.Getenv("FEATURE_DRIFT_ALERT_PCT"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil && f > 0 {
			flags.DriftAlertThresholdPct = f
		}
	}

	if val := os.Getenv("FEATURE_VERBOSE_DRIFT_LOGGING"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			flags.VerboseDriftLogging = b
		}
	}

	return flags
}

// String returns human-readable summary of enabled features
func (ff FeatureFlags) String() string {
	return fmt.Sprintf(
		"[AdaptiveMicrostructure: %v, CalibrateStartup: %v, DriftAlertPct: %.1f%%, VerboseDriftLogging: %v]",
		ff.EnableAdaptiveMicrostructure,
		ff.CalibrateOnStartup,
		ff.DriftAlertThresholdPct,
		ff.VerboseDriftLogging,
	)
}

// Global feature flags instance (lazy-loaded)
var features *FeatureFlags

// Features returns the global feature flags (initialized on first call)
func Features() FeatureFlags {
	if features == nil {
		f := LoadFeatureFlagsFromEnv()
		features = &f
	}
	return *features
}

// SetFeatures allows tests to override feature flags
func SetFeatures(ff FeatureFlags) {
	features = &ff
}

// SetAdaptiveMicrostructure allows runtime toggling of adaptive microstructure
// (e.g., from frontend UI or API endpoints)
func SetAdaptiveMicrostructure(enabled bool) {
	if features == nil {
		f := LoadFeatureFlagsFromEnv()
		features = &f
	}
	features.EnableAdaptiveMicrostructure = enabled
}

// IsAdaptiveMicrostructureEnabled returns current state of adaptive microstructure
func IsAdaptiveMicrostructureEnabled() bool {
	return Features().EnableAdaptiveMicrostructure
}
