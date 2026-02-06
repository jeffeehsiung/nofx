/**
 * NOFX API Configuration
 * Updated base URL from http://nofxaios.com:30006 to https://nofxos.ai
 */

// Base configuration
export const DEFAULT_BASE_URL = 'https://nofxos.ai'
export const DEFAULT_AUTH_KEY = 'cm_568c67eae410d912c54c'

/**
 * Get the default AI500 coin pool API URL
 */
export function getDefaultCoinPoolAPIURL(): string {
  return `${DEFAULT_BASE_URL}/api/ai500/list?auth=${DEFAULT_AUTH_KEY}`
}

/**
 * Get the default OI top ranking API URL
 */
export function getDefaultOITopAPIURL(limit: number = 20, duration: string = '1h'): string {
  return `${DEFAULT_BASE_URL}/api/oi/top-ranking?limit=${limit}&duration=${duration}&auth=${DEFAULT_AUTH_KEY}`
}

/**
 * Get the default quant data API URL with symbol placeholder
 */
export function getDefaultQuantDataAPIURL(): string {
  return `${DEFAULT_BASE_URL}/api/coin/{symbol}?include=netflow,oi,price&auth=${DEFAULT_AUTH_KEY}`
}

/**
 * Get the default OI ranking base URL
 */
export function getDefaultOIRankingBaseURL(): string {
  return DEFAULT_BASE_URL
}
