# CoinGlass API Client

CoinGlass integration for fetching open interest (OI) rankings and market sentiment data. This client uses the CoinGlass V4 API with support for both free-tier and premium features.

## Features

- **Top OI Symbols**: Fetch coins ranked by open interest increase (requires API key)
- **Long/Short Ratio**: Get taker long/short position ratios for specific symbols (premium)
- **Funding Rates**: Retrieve current funding rates for perpetual futures (premium)
- **Automatic Fallback**: When API key not available, falls back to Binance free data
- **Error Handling**: Built-in error handling for network failures and API rate limits

## API Tiers

### Free Tier (No Key Required)
- ❌ OI Rankings not available
- ❌ Long/Short Ratios not available
- ❌ Funding Rates not available
- ✅ System automatically falls back to Binance DIY calculations

### Premium Tier (API Key Required)
- ✅ OI Rankings (coins-markets endpoint)
- ✅ Long/Short Ratios (global-longshort-account-ratio)
- ✅ Funding Rates (funding-rate-current)
- ✅ Historical data and advanced analytics

## Installation

1. Get a CoinGlass API Key:
   - Sign up at [CoinGlass](https://www.coinglass.com/signup)
   - Log in to your account
   - Navigate to [API Settings](https://www.coinglass.com/user)
   - Copy your API Key

2. Configure in your application:

```go
client := coinglass.NewClientWithAPIKey("your-api-key-here")
```

## Usage

### Basic Client Setup

```go
// Without API key (uses fallback system)
client := coinglass.NewClient()

// With API key (premium features enabled)
client := coinglass.NewClientWithAPIKey("your-api-key-12345")
```

### GetTopOISymbols(duration, limit)

Fetches top symbols by open interest increase.

**Requirements**: API Key (Premium)

**Parameters:**
- `duration`: Time period (default: "24h")
- `limit`: Number of results (max 50, default 30)

**Returns:**
- Slice of `PositionCG` structures with OI data
- Error if API key not provided or API call fails

**Example:**
```go
client := coinglass.NewClientWithAPIKey("your-api-key")
positions, err := client.GetTopOISymbols("24h", 20)
if err != nil {
    log.Fatal(err)
}
for i, pos := range positions {
    fmt.Printf("%d. %s: %.2f USD (change: %.2f%%)\n",
        i+1, pos.Symbol, pos.OpenInterestUsd, pos.ChangePercent)
}
```

### GetLongShortRatio(symbol)

Fetches taker long/short ratio for a symbol.

**Requirements**: API Key (Premium)

**Parameters:**
- `symbol`: Coin symbol (e.g., "BTC", "ETH")

**Returns:**
- Long ratio (>1 means more longs, <1 means more shorts)
- Error if API key not provided

### GetFundingRate(symbol)

Fetches current funding rate for a symbol.

**Requirements**: API Key (Premium)

**Parameters:**
- `symbol`: Coin symbol (e.g., "BTC", "ETH")

**Returns:**
- Funding rate as decimal (e.g., 0.0001 = 0.01%)
- Error if API key not provided

## Data Structures

### PositionCG

Contains OI data for a single symbol:

```go
type PositionCG struct {
    Symbol           string  // "BTC", "ETH", etc.
    Pair             string  // "BTC/USDT"
    OpenInterest     float64 // OI in contracts
    OpenInterestUsd  float64 // OI in USD value
    Change           float64 // 24h change in OI value
    ChangePercent    float64 // 24h change in percentage
    PriceChange      float64 // 24h price change in %
    Funding          float64 // Current funding rate
    VolumeUsd24h     float64 // 24h volume in USD
    TakerLongRatio   float64 // % of taker long positions
    TakerShortRatio  float64 // % of taker short positions
    Leverage         string  // Leverage type
    Exchange         string  // Exchange name
}
```

### CoinMarketData

Raw data structure from CoinGlass coins-markets endpoint:

```go
type CoinMarketData struct {
    Symbol                       string  // Cryptocurrency symbol
    CurrentPrice                 float64 // Current price in USD
    OpenInterestUSD              float64 // Open interest in USD
    OpenInterestQuantity         float64 // OI in contracts
    OpenInterestChangePercent24h float64 // 24h OI change %
    PriceChangePercent24h        float64 // 24h price change %
    VolumeUSD24h                 float64 // 24h volume in USD
    AvgFundingRateByOI           float64 // Funding rate weighted by OI
    LongShortRatio24h            float64 // 24h long/short ratio
    TakerBuyRatio24h             float64 // Taker buy ratio
}
```

## Integration with Fallback System

The CoinGlass client integrates into the data provider fallback system:

### Fallback Priority:
1. **External nofxaios.com API** - Primary source
2. **CoinGlass Premium API** - Secondary source (if API key configured)
3. **Binance DIY Calculations** - Tertiary source (always available)

### Configuration:

```go
// In decision/engine.go
func (e *StrategyEngine) FetchOIRankingData() *provider.OIRankingData {
    // Tries: external → CoinGlass → zero data gracefully
    // Uses Binance for coin discovery as fallback
}
```

When CoinGlass API key is not available, the system automatically uses the Binance fallback without failing.

## Rate Limits

**CoinGlass Limits:**
- **Free Tier**: Not applicable (API key required for data)
- **Standard Plan**: 1000 requests/day
- **Pro Plan**: 5000 requests/day
- **Enterprise**: Custom limits

**Recommended Usage:**
- Fetch OI data once per hour
- Cache results for 5 minutes
- Use batch requests where possible
- Implement exponential backoff for retries

## Error Handling

```go
client := coinglass.NewClientWithAPIKey(apiKey)
positions, err := client.GetTopOISymbols("24h", 20)
if err != nil {
    // Error types:
    // - "CoinGlass premium API requires API key..."
    // - "failed to fetch CoinGlass OI data: <network error>"
    // - "CoinGlass API error (status 403): Unauthorized"
    // - "failed to parse CoinGlass response: ..."

    // System automatically falls back to Binance data
}
```

## Testing

Run the test suite:

```bash
cd /Users/jeffeehsiung/Desktop/nofx
go test ./provider/coinglass -v

# Run specific test
go test ./provider/coinglass -run TestNewClient -v

# With coverage
go test ./provider/coinglass -cover
```

### Test Output:
```
=== RUN   TestNewClient
--- PASS: TestNewClient (0.00s)
=== RUN   TestNewClientWithAPIKey
--- PASS: TestNewClientWithAPIKey (0.00s)
=== RUN   TestGetTopOISymbolsWithoutAPIKey
--- PASS: TestGetTopOISymbolsWithoutAPIKey (0.00s)
=== RUN   TestGetTopOISymbolsWithAPIKey
    coinglass_test.go:47: CoinGlass API key not set - skipping premium API test
--- SKIP: TestGetTopOISymbolsWithAPIKey (0.00s)
PASS
```

## Configuration

The CoinGlass client uses sensible defaults:

- **Base URL**: `https://open-api-v4.coinglass.com/api` (V4 official endpoint)
- **Timeout**: 10 seconds per request
- **Default Limit**: 30 results
- **Authentication**: Bearer token in Authorization header

## Future Enhancements

Potential additions to the CoinGlass integration:

1. **Caching Layer**: Add 5-minute TTL cache for API responses
2. **Batch Requests**: Fetch multiple symbols in single request
3. **Historical Data**: Support for OI/funding rate history
4. **Alerts**: Notify when OI changes exceed thresholds
5. **Exchange Filtering**: Support specific exchange selection
6. **Custom Time Ranges**: Extended time range support

## References

- [CoinGlass Official Website](https://www.coinglass.com)
- [CoinGlass API V4 Documentation](https://docs.coinglass.com/reference)
- [API Pricing & Plans](https://www.coinglass.com/pricing)
- [OI Rankings](https://www.coinglass.com/en/oi)
- [Funding Rate Explorer](https://www.coinglass.com/en/funding-rate-explorer)

## Troubleshooting

**Issue**: "CoinGlass premium API requires API key"
- **Cause**: No API key provided to client
- **Solution**: Use `NewClientWithAPIKey("your-key")` or configure fallback is active
- **Impact**: System continues using Binance fallback automatically

**Issue**: "CoinGlass API error (status 403): Unauthorized"
- **Cause**: Invalid or expired API key
- **Solution**: Verify API key in [CoinGlass Settings](https://www.coinglass.com/user)
- **Impact**: Falls back to Binance data automatically

**Issue**: "API error (status 429): Too Many Requests"
- **Cause**: Rate limit exceeded
- **Solution**: Implement request queuing or increase fetch intervals
- **Impact**: Falls back to cached Binance data

**Issue**: "failed to fetch CoinGlass OI data: dial tcp: i/o timeout"
- **Cause**: Network connectivity issues or service unavailable
- **Solution**: System uses exponential backoff; check network connection
- **Impact**: Falls back to Binance DIY calculations

## No API Key? No Problem!

The system is designed to work perfectly without CoinGlass:

1. Binance free API provides coin discovery
2. DIY calculations estimate institutional flow
3. Automatic sentiment analysis works great
4. Fallback system is seamless and transparent

Get started immediately without any API key or upgrade to CoinGlass Premium when you need advanced OI analytics.
