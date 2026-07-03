package services

import (
	"context"
	"testing"
	"time"

	"dynamics-dashboard/database"
)

// fakeEventsStore adds event storage to fakeSeriesStore.
type fakeEventsStore struct {
	*fakeSeriesStore
	events []database.CurrencyEvent
}

func (f *fakeEventsStore) ReplaceEvents(_ context.Context, base, quote string, _, _ time.Time, events []database.CurrencyEvent) error {
	for i := range events {
		events[i].BaseCurrency = base
		events[i].QuoteCurrency = quote
	}
	f.events = events
	return nil
}

func (f *fakeEventsStore) GetEvents(_ context.Context, _, _ string, _, _ time.Time) ([]database.CurrencyEvent, error) {
	return f.events, nil
}

func rp(dateStr string, rate float64) database.RatePoint {
	return database.RatePoint{Date: date(dateStr), Rate: rate}
}

func TestDetectDailyEvent(t *testing.T) {
	// KZT/USD as USD->KZT rates: 6.7% jump in one day (a devaluation day).
	points := []database.RatePoint{
		rp("2024-01-10", 450), rp("2024-01-11", 452), rp("2024-01-12", 482), rp("2024-01-13", 483),
	}
	events := detectEvents(points, date("2024-01-01"), date("2024-01-31"))

	var daily []database.CurrencyEvent
	for _, e := range events {
		if e.ChangeWindow == "daily" {
			daily = append(daily, e)
		}
	}
	if len(daily) != 1 {
		t.Fatalf("got %d daily events, want 1: %+v", len(daily), daily)
	}
	e := daily[0]
	if e.Date.Format("2006-01-02") != "2024-01-12" {
		t.Errorf("event date = %s, want 2024-01-12", e.Date.Format("2006-01-02"))
	}
	if e.PercentChange < 6.5 || e.PercentChange > 6.8 {
		t.Errorf("percent = %v, want ~6.64", e.PercentChange)
	}
	if e.AbsoluteChange != 30 {
		t.Errorf("absolute = %v, want 30", e.AbsoluteChange)
	}
}

func TestDetectWeeklyEvent(t *testing.T) {
	// Slow drift: no single day exceeds 2%, but the week does (+3.1%).
	points := []database.RatePoint{
		rp("2024-01-01", 450), rp("2024-01-02", 452), rp("2024-01-03", 455),
		rp("2024-01-04", 458), rp("2024-01-05", 460), rp("2024-01-08", 464),
	}
	events := detectEvents(points, date("2024-01-01"), date("2024-01-31"))

	for _, e := range events {
		if e.ChangeWindow == "daily" {
			t.Errorf("unexpected daily event: %+v", e)
		}
	}
	var weekly []database.CurrencyEvent
	for _, e := range events {
		if e.ChangeWindow == "weekly" {
			weekly = append(weekly, e)
		}
	}
	if len(weekly) == 0 {
		t.Fatal("no weekly event detected for a 3.1% weekly move")
	}
	if weekly[0].Date.Format("2006-01-02") != "2024-01-08" {
		t.Errorf("weekly event date = %s, want 2024-01-08", weekly[0].Date.Format("2006-01-02"))
	}
}

func TestNoEventsOnQuietMarket(t *testing.T) {
	points := []database.RatePoint{
		rp("2024-01-10", 450), rp("2024-01-11", 451), rp("2024-01-12", 450.5),
		rp("2024-01-17", 452), rp("2024-01-18", 453),
	}
	events := detectEvents(points, date("2024-01-01"), date("2024-01-31"))
	if len(events) != 0 {
		t.Errorf("got %d events on a quiet market, want 0: %+v", len(events), events)
	}
}

func TestGapIsNotADailyEvent(t *testing.T) {
	// 5% move across a 14-day sampling gap: must NOT count as a daily event.
	points := []database.RatePoint{
		rp("2024-01-01", 450), rp("2024-01-15", 472),
	}
	events := detectEvents(points, date("2024-01-01"), date("2024-01-31"))
	for _, e := range events {
		if e.ChangeWindow == "daily" {
			t.Errorf("14-day gap detected as daily event: %+v", e)
		}
	}
}

func TestRecomputeEventsCrossPair(t *testing.T) {
	// KZT devalues 5% against USD while EUR is stable -> KZT/EUR event too.
	store := &fakeEventsStore{fakeSeriesStore: &fakeSeriesStore{newFakeRateStore()}}
	ctx := context.Background()
	store.UpsertRate(ctx, date("2024-06-10"), "USD", "KZT", 450, "test")
	store.UpsertRate(ctx, date("2024-06-11"), "USD", "KZT", 473, "test")
	store.UpsertRate(ctx, date("2024-06-10"), "USD", "EUR", 0.92, "test")
	store.UpsertRate(ctx, date("2024-06-11"), "USD", "EUR", 0.92, "test")

	events, err := RecomputeEvents(ctx, store, &fakeFrank{}, &fakeNBK{},
		"KZT", "EUR", date("2024-06-01"), date("2024-06-30"))
	if err != nil {
		t.Fatalf("RecomputeEvents: %v", err)
	}
	var daily []database.CurrencyEvent
	for _, e := range events {
		if e.ChangeWindow == "daily" {
			daily = append(daily, e)
		}
	}
	if len(daily) != 1 {
		t.Fatalf("got %d daily events, want 1: %+v", len(daily), events)
	}
	// EUR per KZT falls ~4.9% when KZT devalues 5.1% against a stable EUR.
	if daily[0].PercentChange > -4.5 || daily[0].PercentChange < -5.2 {
		t.Errorf("percent = %v, want ~-4.86", daily[0].PercentChange)
	}
	if daily[0].BaseCurrency != "KZT" || daily[0].QuoteCurrency != "EUR" {
		t.Errorf("pair = %s/%s, want KZT/EUR", daily[0].BaseCurrency, daily[0].QuoteCurrency)
	}
}

func TestWeeklyRunsAreCoalesced(t *testing.T) {
	// A sustained slide: every day is >2% below its week-ago baseline.
	// Without coalescing this would emit a marker on every single day.
	points := []database.RatePoint{
		rp("2024-01-01", 500), rp("2024-01-02", 500), rp("2024-01-03", 500),
		rp("2024-01-04", 500), rp("2024-01-05", 500), rp("2024-01-06", 500),
		rp("2024-01-07", 500),
		rp("2024-01-08", 492), rp("2024-01-09", 488), rp("2024-01-10", 485),
		rp("2024-01-11", 484), rp("2024-01-12", 483),
	}
	events := detectEvents(points, date("2024-01-01"), date("2024-01-31"))

	var weekly []database.CurrencyEvent
	for _, e := range events {
		if e.ChangeWindow == "weekly" {
			weekly = append(weekly, e)
		}
	}
	if len(weekly) != 1 {
		t.Fatalf("got %d weekly events, want 1 coalesced: %+v", len(weekly), weekly)
	}
	// The strongest day of the run: 483/500 - 1 = -3.4%.
	if weekly[0].PercentChange > -3.3 || weekly[0].PercentChange < -3.5 {
		t.Errorf("coalesced percent = %v, want ~-3.4 (strongest of the run)", weekly[0].PercentChange)
	}
}

func TestSamePairNoEvents(t *testing.T) {
	store := &fakeEventsStore{fakeSeriesStore: &fakeSeriesStore{newFakeRateStore()}}
	events, err := RecomputeEvents(context.Background(), store, &fakeFrank{}, &fakeNBK{},
		"USD", "USD", date("2024-01-01"), date("2024-01-31"))
	if err != nil || len(events) != 0 {
		t.Errorf("same pair: got %d events, err %v; want 0, nil", len(events), err)
	}
}
