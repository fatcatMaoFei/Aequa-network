package payload

import auction "github.com/zmlAEQ/Aequa-network/internal/payload/auction_bid_v1"
import plaintext "github.com/zmlAEQ/Aequa-network/internal/payload/plaintext_v1"

// SummarizeStats computes aggregate bids/fees for a block selection.
func SummarizeStats(items []Payload) BlockStats {
	var stats BlockStats
	stats.Items = len(items)
	for _, it := range items {
		switch tx := it.(type) {
		case *auction.AuctionBidTx:
			stats.TotalBids += tx.Bid
		case *plaintext.PlaintextTx:
			stats.TotalFees += tx.Fee
		}
	}
	return stats
}
