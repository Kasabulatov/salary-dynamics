package database

import (
	"context"
	"time"
)

type CurrencyEvent struct {
	ID             int64     `json:"id"`
	Date           time.Time `json:"date"`
	RefDate        time.Time `json:"ref_date"` // the earlier date this move was measured against
	BaseCurrency   string    `json:"base_currency"`
	QuoteCurrency  string    `json:"quote_currency"`
	ChangeWindow   string    `json:"change_window"` // "daily" | "weekly"
	PercentChange  float64   `json:"percent_change"`
	AbsoluteChange float64   `json:"absolute_change"`
	NewsHeadline   *string   `json:"news_headline"`
	NewsURL        *string   `json:"news_url"`
}

// ReplaceEvents atomically replaces all detected events for a pair within
// [from, to]. Recomputation is idempotent: better data (denser rates) simply
// yields a fresh, more accurate set.
func (s *Store) ReplaceEvents(ctx context.Context, base, quote string, from, to time.Time, events []CurrencyEvent) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`DELETE FROM currency_events
		 WHERE base_currency = $1 AND quote_currency = $2 AND date BETWEEN $3 AND $4`,
		base, quote, from, to); err != nil {
		return err
	}
	for _, e := range events {
		if _, err := tx.Exec(ctx,
			`INSERT INTO currency_events
			 (date, ref_date, base_currency, quote_currency, change_window, percent_change, absolute_change)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 ON CONFLICT (date, base_currency, quote_currency, change_window) DO UPDATE
			 SET ref_date = EXCLUDED.ref_date,
			     percent_change = EXCLUDED.percent_change,
			     absolute_change = EXCLUDED.absolute_change`,
			e.Date, e.RefDate, base, quote, e.ChangeWindow, e.PercentChange, e.AbsoluteChange); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) GetEvents(ctx context.Context, base, quote string, from, to time.Time) ([]CurrencyEvent, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, date, COALESCE(ref_date, date), base_currency, quote_currency, change_window,
		        percent_change, absolute_change, news_headline, news_url
		 FROM currency_events
		 WHERE base_currency = $1 AND quote_currency = $2 AND date BETWEEN $3 AND $4
		 ORDER BY date ASC`, base, quote, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []CurrencyEvent{}
	for rows.Next() {
		var e CurrencyEvent
		if err := rows.Scan(&e.ID, &e.Date, &e.RefDate, &e.BaseCurrency, &e.QuoteCurrency, &e.ChangeWindow,
			&e.PercentChange, &e.AbsoluteChange, &e.NewsHeadline, &e.NewsURL); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
