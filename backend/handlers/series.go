package handlers

import (
	"log"
	"net/http"
	"strings"
	"time"

	"dynamics-dashboard/services"
)

// SeriesHandler serves the daily salary-dynamics line: one value per day,
// converted at that day's exchange rate (spec: "daily values of the salary
// based on the rate effective that day").
type SeriesHandler struct {
	Salary SalaryStore
	Store  services.SeriesStore
	Frank  services.USDSeriesFetcher
	NBK    services.DayFetcher
}

func (h *SeriesHandler) Series(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFrom(r.Context())

	display := strings.ToUpper(r.URL.Query().Get("displayCurrency"))
	if display == "" {
		display = "USD"
	}
	if !currencyRe.MatchString(display) {
		writeErr(w, http.StatusBadRequest, "displayCurrency must be a 3-letter ISO 4217 code")
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

	entries, err := h.Salary.GetSalaryEntriesByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("series: load entries: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	points, err := services.BuildDailySeries(r.Context(), h.Store, h.Frank, h.NBK, entries, display, from, to)
	if err != nil {
		log.Printf("series: build: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"points":           points,
		"display_currency": display,
	})
}
