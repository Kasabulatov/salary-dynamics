package handlers

import (
	"log"
	"net/http"
	"strings"
	"time"

	"dynamics-dashboard/services"
)

// EventsHandler serves ⚡ significant exchange-rate change markers (spec §2.3):
// days when base/quote moved more than 2% within a day or a week.
type EventsHandler struct {
	Store services.EventsStore
	Frank services.USDSeriesFetcher
	NBK   services.DayFetcher
}

func (h *EventsHandler) Events(w http.ResponseWriter, r *http.Request) {
	base := strings.ToUpper(r.URL.Query().Get("base"))
	quote := strings.ToUpper(r.URL.Query().Get("quote"))
	if !currencyRe.MatchString(base) || !currencyRe.MatchString(quote) {
		writeErr(w, http.StatusBadRequest, "base and quote must be 3-letter ISO 4217 codes")
		return
	}

	now := time.Now().UTC().Truncate(24 * time.Hour)
	from := now.AddDate(-5, 0, 0)
	to := now
	if s := r.URL.Query().Get("from"); s != "" {
		d, err := time.Parse("2006-01-02", s)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "from must be YYYY-MM-DD")
			return
		}
		from = d
	}
	if s := r.URL.Query().Get("to"); s != "" {
		d, err := time.Parse("2006-01-02", s)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "to must be YYYY-MM-DD")
			return
		}
		to = d
	}

	events, err := services.RecomputeEvents(r.Context(), h.Store, h.Frank, h.NBK, base, quote, from, to)
	if err != nil {
		log.Printf("events: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"events": events,
		"base":   base,
		"quote":  quote,
	})
}
