package services

import (
	"context"
	"log"
	"time"

	"dynamics-dashboard/database"
)

// HeadlineFetcher fetches the top headline for search terms in a window.
type HeadlineFetcher interface {
	FetchTopHeadline(ctx context.Context, terms string, from, to time.Time) (*Headline, error)
}

// NewsStore persists fetched headlines and lists events awaiting one.
type NewsStore interface {
	UpdateEventNews(ctx context.Context, id int64, headline, url string) error
	GetEventsMissingNews(ctx context.Context, limit int) ([]database.CurrencyEvent, error)
}

// EnrichPendingNews fills missing headlines for stored events. It runs from
// the daily refresh job — NOT the request path — because GDELT rate-limits
// aggressively (~1 request / 5s): calls are spaced by `delay` and capped at
// `limit` per run. Each event is only ever queried once: a stored empty
// headline records "checked, nothing found". Failures are logged and left
// NULL so the next run retries — news is decoration, never a blocker.
func EnrichPendingNews(ctx context.Context, store NewsStore, gdelt HeadlineFetcher, limit int, delay time.Duration) int {
	if gdelt == nil {
		return 0
	}
	pending, err := store.GetEventsMissingNews(ctx, limit)
	if err != nil {
		log.Printf("news: list pending: %v", err)
		return 0
	}

	filled := 0
	for i := range pending {
		if i > 0 {
			select {
			case <-time.After(delay): // respect GDELT's rate limit
			case <-ctx.Done():
				return filled
			}
		}
		e := &pending[i]
		terms := NewsTermsForPair(e.BaseCurrency, e.QuoteCurrency)
		// Search the compared window plus a day of follow-up coverage.
		hl, err := gdelt.FetchTopHeadline(ctx, terms, e.RefDate, e.Date.AddDate(0, 0, 1))
		if err != nil {
			log.Printf("news: %s/%s %s: %v", e.BaseCurrency, e.QuoteCurrency, e.Date.Format("2006-01-02"), err)
			continue // transient failure: leave NULL so the next run retries
		}

		headline, newsURL := "", ""
		if hl != nil {
			headline, newsURL = hl.Title, hl.URL
		}
		if err := store.UpdateEventNews(ctx, e.ID, headline, newsURL); err != nil {
			log.Printf("news: store %d: %v", e.ID, err)
			continue
		}
		filled++
	}
	return filled
}
