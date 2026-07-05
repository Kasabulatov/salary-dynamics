package services

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"dynamics-dashboard/database"
)

type fakeNewsStore struct {
	pending []database.CurrencyEvent
	updates map[int64][2]string // id -> [headline, url]
}

func (f *fakeNewsStore) GetEventsMissingNews(_ context.Context, limit int) ([]database.CurrencyEvent, error) {
	if len(f.pending) > limit {
		return f.pending[:limit], nil
	}
	return f.pending, nil
}

func (f *fakeNewsStore) UpdateEventNews(_ context.Context, id int64, headline, url string) error {
	if f.updates == nil {
		f.updates = map[int64][2]string{}
	}
	f.updates[id] = [2]string{headline, url}
	return nil
}

type fakeGDELT struct {
	byTerms map[string]*Headline
	err     error
	calls   int
}

func (f *fakeGDELT) FetchTopHeadline(_ context.Context, terms string, _, _ time.Time) (*Headline, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.byTerms[terms], nil
}

func newsEvent(id int64, base, quote, dateStr string) database.CurrencyEvent {
	return database.CurrencyEvent{
		ID: id, BaseCurrency: base, QuoteCurrency: quote,
		Date: date(dateStr), RefDate: date(dateStr).AddDate(0, 0, -7),
		ChangeWindow: "weekly", PercentChange: -5,
	}
}

func TestEnrichPendingFillsHeadlines(t *testing.T) {
	store := &fakeNewsStore{pending: []database.CurrencyEvent{
		newsEvent(1, "KZT", "USD", "2024-06-30"),
	}}
	gdelt := &fakeGDELT{byTerms: map[string]*Headline{
		"Kazakhstan tenge": {Title: "Tenge slides after policy shift", URL: "https://example.com/a"},
	}}

	filled := EnrichPendingNews(context.Background(), store, gdelt, 20, 0)

	if filled != 1 {
		t.Errorf("filled = %d, want 1", filled)
	}
	if got := store.updates[1]; got[0] != "Tenge slides after policy shift" || got[1] != "https://example.com/a" {
		t.Errorf("stored = %v, want fetched headline+url", got)
	}
}

func TestEnrichPendingMarksNothingFound(t *testing.T) {
	store := &fakeNewsStore{pending: []database.CurrencyEvent{
		newsEvent(1, "KZT", "USD", "2024-06-30"),
	}}
	gdelt := &fakeGDELT{} // nil headline for everything

	filled := EnrichPendingNews(context.Background(), store, gdelt, 20, 0)

	if filled != 1 {
		t.Errorf("filled = %d, want 1 (empty marker still counts as handled)", filled)
	}
	got, stored := store.updates[1]
	if !stored || got[0] != "" {
		t.Errorf("stored = %v, want empty string ('checked, nothing found') persisted", got)
	}
}

func TestEnrichPendingRespectsLimit(t *testing.T) {
	store := &fakeNewsStore{}
	for i := 0; i < 8; i++ {
		store.pending = append(store.pending,
			newsEvent(int64(i+1), "KZT", "USD", fmt.Sprintf("2024-0%d-15", i%6+1)))
	}
	gdelt := &fakeGDELT{}
	EnrichPendingNews(context.Background(), store, gdelt, 3, 0)
	if gdelt.calls != 3 {
		t.Errorf("gdelt calls = %d, want 3 (the limit)", gdelt.calls)
	}
}

func TestEnrichPendingLeavesNullOnError(t *testing.T) {
	store := &fakeNewsStore{pending: []database.CurrencyEvent{
		newsEvent(1, "KZT", "USD", "2024-06-30"),
	}}
	gdelt := &fakeGDELT{err: errors.New("gdelt 429")}

	filled := EnrichPendingNews(context.Background(), store, gdelt, 20, 0)

	if filled != 0 {
		t.Errorf("filled = %d, want 0", filled)
	}
	if len(store.updates) != 0 {
		t.Error("error result was persisted — must stay NULL so the next run retries")
	}
}

func TestNewsTermsForPair(t *testing.T) {
	cases := []struct{ base, quote, want string }{
		{"KZT", "USD", "Kazakhstan tenge"}, // non-major side wins
		{"USD", "KZT", "Kazakhstan tenge"},
		{"EUR", "TRY", "Turkish lira"},
		{"KZT", "EUR", "Kazakhstan tenge"},
		{"USD", "EUR", "US dollar"}, // both major -> base
	}
	for _, tc := range cases {
		if got := NewsTermsForPair(tc.base, tc.quote); got != tc.want {
			t.Errorf("NewsTermsForPair(%s, %s) = %q, want %q", tc.base, tc.quote, got, tc.want)
		}
	}
}
