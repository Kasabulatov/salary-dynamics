package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"dynamics-dashboard/database"
)

// fakeRateStore is an in-memory RateStore.
type fakeRateStore struct {
	rates map[string]map[string]float64 // "BASE|QUOTE" -> date -> rate
}

func newFakeRateStore() *fakeRateStore {
	return &fakeRateStore{rates: map[string]map[string]float64{}}
}

func (f *fakeRateStore) UpsertRate(_ context.Context, date time.Time, base, quote string, rate float64, _ string) error {
	key := base + "|" + quote
	if f.rates[key] == nil {
		f.rates[key] = map[string]float64{}
	}
	f.rates[key][date.Format("2006-01-02")] = rate
	return nil
}

func (f *fakeRateStore) GetLatestRateOnOrBefore(_ context.Context, base, quote string, date time.Time) (float64, time.Time, error) {
	key := base + "|" + quote
	best := ""
	for dateStr := range f.rates[key] {
		if dateStr <= date.Format("2006-01-02") && dateStr > best {
			best = dateStr
		}
	}
	if best == "" {
		return 0, time.Time{}, database.ErrNotFound
	}
	d, _ := time.Parse("2006-01-02", best)
	return f.rates[key][best], d, nil
}

// fakeFrank serves a preset series and records whether it was called.
type fakeFrank struct {
	series map[string]map[string]float64 // date -> symbol -> rate
	called bool
}

func (f *fakeFrank) FetchUSDSeries(_ context.Context, start, end time.Time, _ []string) (map[string]map[string]float64, error) {
	f.called = true
	out := map[string]map[string]float64{}
	for dateStr, symbols := range f.series {
		if dateStr >= start.Format("2006-01-02") && dateStr <= end.Format("2006-01-02") {
			out[dateStr] = symbols
		}
	}
	return out, nil
}

type fakeNBK struct {
	days map[string]map[string]float64 // date -> code -> per-USD rate
}

func (f *fakeNBK) FetchDay(_ context.Context, date time.Time) (map[string]float64, error) {
	if day, ok := f.days[date.Format("2006-01-02")]; ok {
		return day, nil
	}
	return map[string]float64{}, nil
}

func date(s string) time.Time {
	d, _ := time.Parse("2006-01-02", s)
	return d
}

func TestConvertSameCurrency(t *testing.T) {
	c := NewConverter(newFakeRateStore(), &fakeFrank{}, &fakeNBK{})
	got, rd, err := c.Convert(context.Background(), 1000, "KZT", "KZT", date("2024-01-15"))
	if err != nil || got != 1000 || !rd.Equal(date("2024-01-15")) {
		t.Errorf("same currency: got %v, %v, %v; want 1000, 2024-01-15, nil", got, rd, err)
	}
}

func TestConvertExactDate(t *testing.T) {
	store := newFakeRateStore()
	// 450 KZT per USD on 2024-01-15.
	store.UpsertRate(context.Background(), date("2024-01-15"), "USD", "KZT", 450, "test")
	c := NewConverter(store, &fakeFrank{}, &fakeNBK{})

	// 450,000 KZT -> USD must be exactly 1,000 using that day's rate.
	got, rd, err := c.Convert(context.Background(), 450000, "KZT", "USD", date("2024-01-15"))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if got != 1000 {
		t.Errorf("got %v USD, want 1000", got)
	}
	if !rd.Equal(date("2024-01-15")) {
		t.Errorf("rate date = %v, want 2024-01-15", rd)
	}
}

func TestConvertCarryForward(t *testing.T) {
	store := newFakeRateStore()
	// Rate only exists on Friday 2024-01-12; entry is dated Sunday 2024-01-14.
	store.UpsertRate(context.Background(), date("2024-01-12"), "USD", "KZT", 400, "test")
	c := NewConverter(store, &fakeFrank{}, &fakeNBK{})

	got, rd, err := c.Convert(context.Background(), 400000, "KZT", "USD", date("2024-01-14"))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if got != 1000 {
		t.Errorf("got %v USD, want 1000 (carried forward from Friday)", got)
	}
	if !rd.Equal(date("2024-01-12")) {
		t.Errorf("rate date = %v, want 2024-01-12 (the last known rate)", rd)
	}
}

func TestConvertCrossViaUSD(t *testing.T) {
	store := newFakeRateStore()
	ctx := context.Background()
	store.UpsertRate(ctx, date("2024-01-15"), "USD", "KZT", 450, "test")
	store.UpsertRate(ctx, date("2024-01-15"), "USD", "EUR", 0.9, "test")
	c := NewConverter(store, &fakeFrank{}, &fakeNBK{})

	// 450,000 KZT = 1,000 USD = 900 EUR.
	got, _, err := c.Convert(ctx, 450000, "KZT", "EUR", date("2024-01-15"))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if got < 899.999 || got > 900.001 {
		t.Errorf("got %v EUR, want 900", got)
	}
}

func TestConvertFetchesOnDemand(t *testing.T) {
	frank := &fakeFrank{series: map[string]map[string]float64{
		"2024-01-15": {"EUR": 0.9},
	}}
	c := NewConverter(newFakeRateStore(), frank, &fakeNBK{})

	// Store is empty: converter must fetch from Frankfurter and then succeed.
	got, _, err := c.Convert(context.Background(), 1000, "USD", "EUR", date("2024-01-15"))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if !frank.called {
		t.Error("Frankfurter was not called on cache miss")
	}
	if got < 899.999 || got > 900.001 {
		t.Errorf("got %v EUR, want 900", got)
	}
}

func TestConvertNBKOnDemandWithWalkBack(t *testing.T) {
	// NBK has data only on 2024-01-12; request is for 2024-01-14.
	nbk := &fakeNBK{days: map[string]map[string]float64{
		"2024-01-12": {"KZT": 400, "RUB": 90},
	}}
	c := NewConverter(newFakeRateStore(), &fakeFrank{}, nbk)

	got, rd, err := c.Convert(context.Background(), 400000, "KZT", "USD", date("2024-01-14"))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if got != 1000 {
		t.Errorf("got %v USD, want 1000", got)
	}
	if !rd.Equal(date("2024-01-12")) {
		t.Errorf("rate date = %v, want 2024-01-12", rd)
	}
}

func TestConvertUnknownCurrency(t *testing.T) {
	c := NewConverter(newFakeRateStore(), &fakeFrank{}, &fakeNBK{})
	_, _, err := c.Convert(context.Background(), 100, "XXX", "USD", date("2024-01-15"))
	if err == nil {
		t.Fatal("unknown currency accepted, want error")
	}
	if !errors.Is(err, ErrNoRate) {
		t.Errorf("error = %v, want ErrNoRate", err)
	}
}

func TestConverterMemo(t *testing.T) {
	store := newFakeRateStore()
	store.UpsertRate(context.Background(), date("2024-01-15"), "USD", "KZT", 450, "test")
	c := NewConverter(store, &fakeFrank{}, &fakeNBK{})

	ctx := context.Background()
	c.Convert(ctx, 100, "KZT", "USD", date("2024-01-15"))
	// Wipe the store: second identical conversion must hit the memo.
	store.rates = map[string]map[string]float64{}
	got, _, err := c.Convert(ctx, 450, "KZT", "USD", date("2024-01-15"))
	if err != nil {
		t.Fatalf("memoized convert: %v", err)
	}
	if got != 1 {
		t.Errorf("got %v, want 1", got)
	}
}
