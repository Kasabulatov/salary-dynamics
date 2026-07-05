package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"dynamics-dashboard/database"
	"dynamics-dashboard/services"
)

const maxPublicEntries = 50

// PublicComputeStore aggregates everything the stateless compute needs.
type PublicComputeStore interface {
	services.EventsStore
	UpsertInflationRate(ctx context.Context, country string, year int, ratePercent float64, source string) error
	GetInflationRates(ctx context.Context, country string) (map[int]float64, error)
}

// PublicHandler serves guest mode and the landing-page demo: it computes
// series/conversion/inflation/events for entries supplied in the request
// body and stores nothing. Rates and CPI come from the shared cache.
type PublicHandler struct {
	Store     PublicComputeStore
	Converter CurrencyConverter
	WB        services.CPIFetcher
	Frank     services.USDSeriesFetcher
	NBK       services.DayFetcher
}

type publicEntry struct {
	Amount        float64 `json:"amount"`
	CurrencyCode  string  `json:"currency_code"`
	EffectiveDate string  `json:"effective_date"`
	Note          *string `json:"note"`
}

type publicComputeRequest struct {
	Entries          []publicEntry `json:"entries"`
	DisplayCurrency  string        `json:"display_currency"`
	From             string        `json:"from"`
	To               string        `json:"to"`
	IncludeInflation bool          `json:"include_inflation"`
	IncludeEvents    bool          `json:"include_events"`
}

func (h *PublicHandler) Compute(w http.ResponseWriter, r *http.Request) {
	var req publicComputeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(req.Entries) == 0 {
		writeErr(w, http.StatusBadRequest, "entries must not be empty")
		return
	}
	if len(req.Entries) > maxPublicEntries {
		writeErr(w, http.StatusBadRequest, "too many entries (max 50)")
		return
	}

	display := strings.ToUpper(req.DisplayCurrency)
	if display == "" {
		display = "USD"
	}
	if !currencyRe.MatchString(display) {
		writeErr(w, http.StatusBadRequest, "display_currency must be a 3-letter ISO 4217 code")
		return
	}

	// Validate and convert to model entries (same rules as the real form).
	entries := make([]database.SalaryEntry, 0, len(req.Entries))
	for i, pe := range req.Entries {
		in := salaryInput{Amount: pe.Amount, CurrencyCode: pe.CurrencyCode,
			EffectiveDate: pe.EffectiveDate, Note: pe.Note}
		date, msg, ok := in.validate()
		if !ok {
			writeErr(w, http.StatusBadRequest, msg)
			return
		}
		if pe.Note != nil && len(*pe.Note) > 500 {
			writeErr(w, http.StatusBadRequest, "note is too long (max 500 characters)")
			return
		}
		entries = append(entries, database.SalaryEntry{
			ID: int64(i + 1), Amount: pe.Amount, CurrencyCode: pe.CurrencyCode,
			EffectiveDate: date, Note: pe.Note,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].EffectiveDate.Before(entries[j].EffectiveDate)
	})

	now := time.Now().UTC().Truncate(24 * time.Hour)
	from := now.AddDate(-5, 0, 0)
	to := now
	if req.From != "" {
		d, err := time.Parse("2006-01-02", req.From)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "from must be YYYY-MM-DD")
			return
		}
		from = d
	}
	if req.To != "" {
		d, err := time.Parse("2006-01-02", req.To)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "to must be YYYY-MM-DD")
			return
		}
		to = d
	}

	ctx := r.Context()

	// Per-entry historical conversion (same shape the private list returns).
	outEntries := make([]entryResponse, 0, len(entries))
	for _, e := range entries {
		resp := entryResponse{SalaryEntry: e, DisplayCurrency: display}
		if h.Converter != nil {
			converted, rateDate, err := h.Converter.Convert(ctx, e.Amount, e.CurrencyCode, display, e.EffectiveDate)
			if err != nil {
				resp.ConversionError = "no exchange rate available for this date"
			} else {
				rd := rateDate.Format("2006-01-02")
				resp.ConvertedAmount = &converted
				resp.RateDate = &rd
			}
		}
		outEntries = append(outEntries, resp)
	}

	seriesPoints, err := services.BuildDailySeries(ctx, h.Store, h.Frank, h.NBK, entries, display, from, to)
	if err != nil {
		log.Printf("public compute: series: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	var inflation *services.InflationTarget
	if req.IncludeInflation {
		target, err := services.BuildInflationTarget(ctx, h.Store, h.WB, h.Frank, h.NBK, entries, display, from, to)
		if err != nil {
			log.Printf("public compute: inflation: %v", err) // non-critical: omit
		} else {
			inflation = &target
		}
	}

	events := []database.CurrencyEvent{}
	if req.IncludeEvents {
		seen := map[string]bool{}
		for _, e := range entries {
			if e.CurrencyCode == display || seen[e.CurrencyCode] {
				continue
			}
			seen[e.CurrencyCode] = true
			evs, err := services.RecomputeEvents(ctx, h.Store, h.Frank, h.NBK, e.CurrencyCode, display, from, to)
			if err != nil {
				log.Printf("public compute: events %s/%s: %v", e.CurrencyCode, display, err)
				continue
			}
			events = append(events, evs...)
		}
		sort.Slice(events, func(i, j int) bool { return events[i].Date.Before(events[j].Date) })
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"entries":          outEntries,
		"series":           map[string]any{"points": seriesPoints},
		"inflation":        inflation,
		"events":           events,
		"display_currency": display,
	})
}
