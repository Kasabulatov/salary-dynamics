package services

import (
	"context"
	"testing"

	"dynamics-dashboard/database"
)

// fakeInflationStore adds inflation storage to fakeSeriesStore.
type fakeInflationStore struct {
	*fakeSeriesStore
	inflation map[string]map[int]float64 // country -> year -> rate
}

func newFakeInflationStore() *fakeInflationStore {
	return &fakeInflationStore{
		fakeSeriesStore: &fakeSeriesStore{newFakeRateStore()},
		inflation:       map[string]map[int]float64{},
	}
}

func (f *fakeInflationStore) UpsertInflationRate(_ context.Context, country string, year int, rate float64, _ string) error {
	if f.inflation[country] == nil {
		f.inflation[country] = map[int]float64{}
	}
	f.inflation[country][year] = rate
	return nil
}

func (f *fakeInflationStore) GetInflationRates(_ context.Context, country string) (map[int]float64, error) {
	out := map[int]float64{}
	for y, r := range f.inflation[country] {
		out[y] = r
	}
	return out, nil
}

type fakeWB struct {
	rates  map[string]map[int]float64
	called bool
}

func (f *fakeWB) FetchCPI(_ context.Context, country string) (map[int]float64, error) {
	f.called = true
	if r, ok := f.rates[country]; ok {
		return r, nil
	}
	return map[int]float64{}, nil
}

func valueOn(t *testing.T, points []SeriesPoint, dateStr string) float64 {
	t.Helper()
	for _, p := range points {
		if p.Date == dateStr {
			return p.Value
		}
	}
	t.Fatalf("no point on %s", dateStr)
	return 0
}

func TestInflationTargetStepsOnJan1(t *testing.T) {
	store := newFakeInflationStore()
	wb := &fakeWB{rates: map[string]map[int]float64{
		"US": {2023: 10, 2024: 5},
	}}
	entries := []database.SalaryEntry{entry(1000, "USD", "2023-06-01")}

	target, err := BuildInflationTarget(context.Background(), store, wb, &fakeFrank{}, &fakeNBK{},
		entries, "USD", date("2023-06-01"), date("2025-03-01"))
	if err != nil {
		t.Fatalf("BuildInflationTarget: %v", err)
	}
	if !wb.called {
		t.Error("World Bank not fetched on empty cache")
	}
	if target.CountryCode != "US" || target.BaseCcy != "USD" {
		t.Errorf("meta = %s/%s, want US/USD", target.CountryCode, target.BaseCcy)
	}
	if target.Rates[2023] != 10 || target.Rates[2024] != 5 {
		t.Errorf("rates = %v, want 2023:10 2024:5 (for tooltips)", target.Rates)
	}

	// Flat at 1000 through 2023; ×1.10 from Jan 1 2024; ×1.155 from Jan 1 2025.
	if v := valueOn(t, target.Points, "2023-06-01"); v != 1000 {
		t.Errorf("baseline day = %v, want 1000", v)
	}
	if v := valueOn(t, target.Points, "2023-12-31"); v != 1000 {
		t.Errorf("day before first step = %v, want 1000 (flat)", v)
	}
	if v := valueOn(t, target.Points, "2024-01-01"); v < 1099.99 || v > 1100.01 {
		t.Errorf("after 2024 step = %v, want 1100 (2023's 10%%)", v)
	}
	if v := valueOn(t, target.Points, "2024-12-31"); v < 1099.99 || v > 1100.01 {
		t.Errorf("end of 2024 = %v, want 1100 (flat between steps)", v)
	}
	if v := valueOn(t, target.Points, "2025-01-01"); v < 1154.99 || v > 1155.01 {
		t.Errorf("after 2025 step = %v, want 1155 (compounded)", v)
	}
}

func TestInflationTargetRangeStartsMidHistory(t *testing.T) {
	// Requesting only 2025 must arrive with prior years' factor pre-applied.
	store := newFakeInflationStore()
	wb := &fakeWB{rates: map[string]map[int]float64{
		"US": {2023: 10, 2024: 5},
	}}
	entries := []database.SalaryEntry{entry(1000, "USD", "2023-06-01")}

	target, err := BuildInflationTarget(context.Background(), store, wb, &fakeFrank{}, &fakeNBK{},
		entries, "USD", date("2025-02-01"), date("2025-03-01"))
	if err != nil {
		t.Fatalf("BuildInflationTarget: %v", err)
	}
	if v := valueOn(t, target.Points, "2025-02-01"); v < 1154.99 || v > 1155.01 {
		t.Errorf("mid-history start = %v, want 1155 (1000 × 1.10 × 1.05)", v)
	}
}

func TestInflationTargetUnpublishedYearStaysFlat(t *testing.T) {
	// 2024's rate is not published: no step on Jan 1 2025.
	store := newFakeInflationStore()
	wb := &fakeWB{rates: map[string]map[int]float64{
		"US": {2023: 10},
	}}
	entries := []database.SalaryEntry{entry(1000, "USD", "2023-06-01")}

	target, err := BuildInflationTarget(context.Background(), store, wb, &fakeFrank{}, &fakeNBK{},
		entries, "USD", date("2024-12-01"), date("2025-02-01"))
	if err != nil {
		t.Fatalf("BuildInflationTarget: %v", err)
	}
	if v := valueOn(t, target.Points, "2025-01-15"); v < 1099.99 || v > 1100.01 {
		t.Errorf("unpublished year = %v, want 1100 (no fabricated step)", v)
	}
}

func TestInflationTargetConvertsToDisplayCurrency(t *testing.T) {
	// KZT baseline, USD display: target grows by KZT CPI, converted at 450.
	store := newFakeInflationStore()
	ctx := context.Background()
	store.UpsertRate(ctx, date("2023-06-01"), "USD", "KZT", 450, "test")
	wb := &fakeWB{rates: map[string]map[int]float64{
		"KZ": {2023: 20},
	}}
	entries := []database.SalaryEntry{entry(450000, "KZT", "2023-06-01")}

	target, err := BuildInflationTarget(ctx, store, wb, &fakeFrank{}, &fakeNBK{},
		entries, "USD", date("2023-06-01"), date("2024-02-01"))
	if err != nil {
		t.Fatalf("BuildInflationTarget: %v", err)
	}
	if target.CountryCode != "KZ" {
		t.Errorf("country = %s, want KZ (from salary currency)", target.CountryCode)
	}
	// Before step: 450000/450 = 1000 USD. After Jan 1: ×1.20 = 1200 USD.
	if v := valueOn(t, target.Points, "2023-07-01"); v != 1000 {
		t.Errorf("pre-step = %v, want 1000", v)
	}
	if v := valueOn(t, target.Points, "2024-01-15"); v < 1199.99 || v > 1200.01 {
		t.Errorf("post-step = %v, want 1200", v)
	}
}

func TestInflationTargetNoEntries(t *testing.T) {
	store := newFakeInflationStore()
	target, err := BuildInflationTarget(context.Background(), store, &fakeWB{}, &fakeFrank{}, &fakeNBK{},
		nil, "USD", date("2024-01-01"), date("2024-02-01"))
	if err != nil || len(target.Points) != 0 {
		t.Errorf("no entries: got %d points, err %v; want 0, nil", len(target.Points), err)
	}
}
