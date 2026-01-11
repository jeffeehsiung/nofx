package decision

import (
	"math"
	"nofx/market"
)

// MapMicrostructureToRecentOrderV2 maps producer microstructure metrics into the V2 consumer struct.
func MapMicrostructureToRecentOrderV2(order *RecentOrderV2, ms *market.MarketMicrostructure, isEntry bool) {
	if order == nil || ms == nil {
		return
	}

	spreadDecimal := ms.BidAskSpread / 100.0 // source is percent; consumer expects decimal fraction
	depth := math.Min(ms.BidDepth, ms.AskDepth)

	if isEntry {
		order.EntrySpread = spreadDecimal
		order.EntryDepth = depth
	} else {
		order.ExitSpread = spreadDecimal
		order.ExitDepth = depth
	}
}

// MapMicrostructureToRecentOrder applies the same mapping for the legacy RecentOrder struct.
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
