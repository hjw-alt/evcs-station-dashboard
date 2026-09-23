package main

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func stationKeys(items []Station) []string {
	keys := make([]string, len(items))
	for i, station := range items {
		keys[i] = station.SourceKey
	}
	return keys
}

func TestIdleOrderRateThenIdleCountAndUnknownLast(t *testing.T) {
	items := []Station{
		{SourceKey: "unknown", MatchedName: "A", PileTotal: 50, PileIdle: 50, IdleRate: 100},
		{SourceKey: "busy", HasPileDetails: true, PileTotal: 20},
		{SourceKey: "moderate", HasPileDetails: true, PileTotal: 100, PileIdle: 40, IdleRate: 40},
		{SourceKey: "idle-small", HasPileDetails: true, PileTotal: 10, PileIdle: 8, IdleRate: 80},
		{SourceKey: "full-idle", HasPileDetails: true, PileTotal: 2, PileIdle: 2, IdleRate: 100},
		{SourceKey: "idle-large", HasPileDetails: true, PileTotal: 100, PileIdle: 80, IdleRate: 80},
		{SourceKey: "empty-details", MatchedName: "B", HasPileDetails: true, IdleRate: 100},
	}
	want := []string{"full-idle", "idle-large", "idle-small", "moderate", "busy", "unknown", "empty-details"}
	if got := stationKeys(idleOrderedPage(items, 1, 3000)); !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestIdleOrderStableTiesAcrossRefresh(t *testing.T) {
	items := []Station{
		{SourceKey: "b", MatchedName: "Same", HasPileDetails: true, PileTotal: 10, PileIdle: 5, IdleRate: 50},
		{SourceKey: "c", RequestedName: "Alpha", HasPileDetails: true, PileTotal: 10, PileIdle: 5, IdleRate: 50},
		{SourceKey: "a", MatchedName: "Same", HasPileDetails: true, PileTotal: 10, PileIdle: 5, IdleRate: 50},
	}
	want := []string{"c", "a", "b"}
	for range 3 {
		slices.Reverse(items)
		if got := stationKeys(idleOrderedPage(items, 1, 3000)); !reflect.DeepEqual(got, want) {
			t.Fatalf("refreshed order = %v, want %v", got, want)
		}
	}
}

func TestIdleOrderAcrossAll2523StationsBeforePagination(t *testing.T) {
	items := make([]Station, 2523)
	for i := range items {
		items[i] = Station{SourceKey: fmt.Sprintf("station-%04d", i), HasPileDetails: true, PileTotal: 10000, PileIdle: i, IdleRate: float64(i) / 100}
	}
	all := idleOrderedPage(slices.Clone(items), 1, 3000)
	if len(all) != 2523 || all[0].SourceKey != "station-2522" || all[2522].SourceKey != "station-0000" {
		t.Fatal("all-station card order is incomplete or not descending")
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].IdleRate < all[i].IdleRate {
			t.Fatalf("order increased at index %d", i)
		}
	}
	for _, page := range []int{1, 2, 101, 102} {
		got := idleOrderedPage(slices.Clone(items), page, 25)
		start := min((page-1)*25, len(all))
		end := min(start+25, len(all))
		if !reflect.DeepEqual(stationKeys(got), stationKeys(all[start:end])) {
			t.Fatalf("page %d does not match globally sorted cards", page)
		}
	}
}

func TestIdleOrderEmptyAndInvalidPages(t *testing.T) {
	for _, input := range []struct{ page, size int }{{1, 25}, {0, 25}, {1, 0}, {1, -1}, {int(^uint(0) >> 1), 3000}} {
		if got := idleOrderedPage([]Station{}, input.page, input.size); len(got) != 0 {
			t.Errorf("empty page = %v", got)
		}
	}
}
