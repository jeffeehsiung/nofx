package config

import (
	"fmt"
	"time"
)

// NOFX API Configuration
// Updated base URL from http://nofxaios.com:30006 to https://nofxos.ai
const (
	// DefaultBaseURL is the base URL for NOFX data API
	DefaultBaseURL = "https://nofxos.ai"

	// DefaultAuthKey is the default authentication key for API access
	DefaultAuthKey = "cm_568c67eae410d912c54c"

	// DefaultTimeout is the default HTTP timeout for API requests
	DefaultTimeout = 30 * time.Second
)

// GetDefaultCoinPoolAPIURL returns the default AI500 coin pool API URL
func GetDefaultCoinPoolAPIURL() string {
	return fmt.Sprintf("%s/api/ai500/list?auth=%s", DefaultBaseURL, DefaultAuthKey)
}

// GetDefaultOITopAPIURL returns the default OI top ranking API URL
func GetDefaultOITopAPIURL(limit int, duration string) string {
	return fmt.Sprintf("%s/api/oi/top-ranking?limit=%d&duration=%s&auth=%s", DefaultBaseURL, limit, duration, DefaultAuthKey)
}

// GetDefaultQuantDataAPIURL returns the default quant data API URL with symbol placeholder
func GetDefaultQuantDataAPIURL() string {
	return fmt.Sprintf("%s/api/coin/{symbol}?include=netflow,oi,price&auth=%s", DefaultBaseURL, DefaultAuthKey)
}

// GetDefaultOIRankingBaseURL returns the default OI ranking base URL
func GetDefaultOIRankingBaseURL() string {
	return DefaultBaseURL
}
