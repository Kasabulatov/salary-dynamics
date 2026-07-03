package services

import (
	"context"
	"testing"
	"time"

	"dynamics-dashboard/database"
)

// fakeSeriesStore extends fakeRateStore with the series-builder methods.
type fakeSeriesStore struct {
	*fakeRateStore
}

func (f *fakeSeriesStore) GetRatesForCurrency(_ context.Context, quote string, from, to time.Time) ([]database.RatePoint, error) {
	var out []database.RatePoint
	m := f.rates["USD|"+quote]
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if rate, ok := m[d.Format("2006-01-02")]; ok {
			out = append(out, database.RatePoint{Date: d, Rate: rate})
		}
	}
	return out, nil
}

func (f *fakeSeriesStore) GetRateCoverage(_ context.Context, quote string, from, to time.Time) (time.Time, time.Time, int64, error) {
	var minD, maxD time.Time
	var count int64
	for dateStr := range f.rates["USD|"+quote] {
		d, _ := time.Parse("2006-01-02", dateStr)
		if d.Before(from) || d.After(to) {
			continue
		}
		if count == 0 || d.Before(minD) {
			minD = d
		}
		if count == 0 || d.After(maxD) {
			maxD = d
		}
		count++
	}
	return minD, maxD, count, nil
}

func entry(amount float64, ccy, dateStr string) database.SalaryEntry {
	return database.SalaryEntry{Amount: amount, CurrencyCode: ccy, EffectiveDate: date(dateStr)}
}

func TestDailySeriesUsesEachDaysRate(t *testing.T) {
	store := &fakeSeriesStore{newFakeRateStore()}
	ctx := context.Background()
	// KZT weakens mid-window: 450 -> 460 per USD.
	store.UpsertRate(ctx, date("2024-01-10"), "USD", "KZT", 450, "test")
	store.UpsertRate(ctx, date("2024-01-12"), "USD", "KZT", 460, "test")

	entries := []database.SalaryEntry{entry(450000, "KZT", "2024-01-10")}
	points, err := BuildDailySeries(ctx, store, &fakeFrank{}, &fakeNBK{},
		entries, "USD", date("2024-01-10"), date("2024-01-14"))
	if err != nil {
		t.Fatalf("BuildDailySeries: %v", err)
	}
	if len(points) != 5 {
		t.Fatalf("got %d points, want 5 (one per day)", len(points))
	}
	// Day 1-2 at rate 450 -> 1000 USD; day 3+ at 460 -> ~978.26 (carry-forward).
	if points[0].Value != 1000 || points[1].Value != 1000 {
		t.Errorf("days 1-2 = %v, %v; want 1000, 1000", points[0].Value, points[1].Value)
	}
	want := 450000.0 / 460.0
	for i := 2; i < 5; i++ {
		if points[i].Value < want-0.01 || points[i].Value > want+0.01 {
			t.Errorf("day %d = %v, want ~%.2f (new rate carried forward)", i+1, points[i].Value, want)
		}
	}
}

func TestDailySeriesSalaryChangeMidWindow(t *testing.T) {
	store := &fakeSeriesStore{newFakeRateStore()}
	ctx := context.Background()
	store.UpsertRate(ctx, date("2024-01-10"), "USD", "KZT", 450, "test")

	entries := []database.SalaryEntry{
		entry(450000, "KZT", "2024-01-10"),
		entry(900000, "KZT", "2024-01-13"), // raise
	}
	points, err := BuildDailySeries(ctx, store, &fakeFrank{}, &fakeNBK{},
		entries, "USD", date("2024-01-10"), date("2024-01-14"))
	if err != nil {
		t.Fatalf("BuildDailySeries: %v", err)
	}
	if points[2].Value != 1000 {
		t.Errorf("day before raise = %v, want 1000", points[2].Value)
	}
	if points[3].Value != 2000 || points[4].Value != 2000 {
		t.Errorf("days after raise = %v, %v; want 2000, 2000", points[3].Value, points[4].Value)
	}
}

func TestDailySeriesSameCurrencyIsFlat(t *testing.T) {
	store := &fakeSeriesStore{newFakeRateStore()}
	entries := []database.SalaryEntry{entry(500000, "KZT", "2024-01-10")}
	points, err := BuildDailySeries(context.Background(), store, &fakeFrank{}, &fakeNBK{},
		entries, "KZT", date("2024-01-10"), date("2024-01-12"))
	if err != nil {
		t.Fatalf("BuildDailySeries: %v", err)
	}
	// KZT -> KZT needs no rates at all; flat at the raw amount.
	if len(points) != 3 {
		t.Fatalf("got %d points, want 3", len(points))
	}
	for _, p := range points {
		if p.Value != 500000 {
			t.Errorf("point %s = %v, want 500000", p.Date, p.Value)
		}
	}
}

func TestDailySeriesBackfillsFromNBK(t *testing.T) {
	store := &fakeSeriesStore{newFakeRateStore()} // empty: coverage check must trigger fetch

	// Use recent dates so the sampler fetches daily and hits the fake's days.
	day := func(offset int) time.Time {
		return time.Now().UTC().Truncate(24 * time.Hour).AddDate(0, 0, offset)
	}
	iso := func(t time.Time) string { return t.Format("2006-01-02") }

	nbk := &fakeNBK{days: map[string]map[string]float64{
		iso(day(-4)): {"KZT": 450},
		iso(day(-2)): {"KZT": 460},
	}}
	entries := []database.SalaryEntry{{Amount: 450000, CurrencyCode: "KZT", EffectiveDate: day(-4)}}

	points, err := BuildDailySeries(context.Background(), store, &fakeFrank{}, nbk,
		entries, "USD", day(-4), day(0))
	if err != nil {
		t.Fatalf("BuildDailySeries: %v", err)
	}
	if len(points) != 5 {
		t.Fatalf("got %d points, want 5 (backfill from NBK)", len(points))
	}
	if points[0].Value != 1000 || points[1].Value != 1000 {
		t.Errorf("days 1-2 = %v, %v; want 1000 (rate 450)", points[0].Value, points[1].Value)
	}
	want := 450000.0 / 460.0
	if points[2].Value < want-0.01 || points[2].Value > want+0.01 {
		t.Errorf("day 3 = %v, want ~%.2f (rate 460)", points[2].Value, want)
	}
}

func TestDailySeriesClampsToEntriesAndToday(t *testing.T) {
	store := &fakeSeriesStore{newFakeRateStore()}
	store.UpsertRate(context.Background(), date("2024-01-10"), "USD", "KZT", 450, "test")
	entries := []database.SalaryEntry{entry(450000, "KZT", "2024-01-10")}

	// Range starts long before the first entry: series must start at the entry.
	points, err := BuildDailySeries(context.Background(), store, &fakeFrank{}, &fakeNBK{},
		entries, "USD", date("2020-01-01"), date("2024-01-12"))
	if err != nil {
		t.Fatalf("BuildDailySeries: %v", err)
	}
	if len(points) == 0 || points[0].Date != "2024-01-10" {
		t.Errorf("series starts at %v, want 2024-01-10 (first entry)", points[0].Date)
	}

	// No entries -> empty series, no error.
	empty, err := BuildDailySeries(context.Background(), store, &fakeFrank{}, &fakeNBK{},
		nil, "USD", date("2024-01-01"), date("2024-01-12"))
	if err != nil || len(empty) != 0 {
		t.Errorf("no entries: got %d points, err %v; want 0, nil", len(empty), err)
	}
}
