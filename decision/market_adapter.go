package decision

import (
	"math"
	"nofx/market"
)

// MapMicrostructureToRecentOrder applies the mapping for the unified RecentOrder struct.
func MapMicrostructureToRecentOrder(order *RecentOrder, ms *market.MarketMicrostructure, isEntry bool) {
	if order == nil || ms == nil {
		return
	}

	spreadDecimal := ms.BidAskSpread / 100.0
	depth := math.Min(ms.BidDepth, ms.AskDepth)

	if isEntry {
		order.EntrySpread = spreadDecimal
		order.EntryDepth = depth
	} else {
		order.ExitSpread = spreadDecimal
		order.ExitDepth = depth
	}
}
