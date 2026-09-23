package main

import "sort"

// idleOrderedPage uses the same computed values as the cards, rather than
// approximating availability from the raw fast/super/slow summary fields.
func idleOrderedPage(items []Station, page, pageSize int) []Station {
	sort.Slice(items, func(i, j int) bool {
		left, right := items[i], items[j]
		// Match the cards' no-data state and keep it behind even fully busy sites.
		leftKnown := left.HasPileDetails && left.PileTotal > 0
		rightKnown := right.HasPileDetails && right.PileTotal > 0
		if leftKnown != rightKnown {
			return leftKnown
		}
		if leftKnown {
			if left.IdleRate != right.IdleRate {
				return left.IdleRate > right.IdleRate
			}
			if left.PileIdle != right.PileIdle {
				return left.PileIdle > right.PileIdle
			}
		}
		leftName, rightName := left.MatchedName, right.MatchedName
		if leftName == "" {
			leftName = left.RequestedName
		}
		if rightName == "" {
			rightName = right.RequestedName
		}
		if leftName != rightName {
			return leftName < rightName
		}
		// A unique tie-breaker prevents equal-availability cards jumping on refresh.
		return left.SourceKey < right.SourceKey
	})

	return stationPage(items, page, pageSize)
}

func stationPage(items []Station, page, pageSize int) []Station {
	// Slice only after filtering, summarizing and sorting the complete set.
	if page < 1 || pageSize <= 0 || page-1 > len(items)/pageSize {
		return items[:0]
	}
	start := (page - 1) * pageSize
	end := start + min(pageSize, len(items)-start)
	return items[start:end]
}
