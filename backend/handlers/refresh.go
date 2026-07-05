package handlers

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"dynamics-dashboard/services"
)

// RefreshStore is what the daily refresh needs from the database.
type RefreshStore interface {
	services.InflationStore
	services.NewsStore
	GetDistinctSalaryCurrencies(ctx context.Context) ([]string, error)
}

// RefreshHandler triggers rate + inflation + news ingestion. It is called by
// a scheduled job (GitHub Actions cron in production) and protected by a
// shared secret.
type RefreshHandler struct {
	Store  RefreshStore
	Frank  services.USDSeriesFetcher
	NBK    services.DayFetcher
	WB     services.CPIFetcher
	GDELT  services.HeadlineFetcher
	Secret string
}

type refreshInput struct {
	Start string `json:"start"` // YYYY-MM-DD, default: end - 30 days
	End   string `json:"end"`   // YYYY-MM-DD, default: today
}

func (h *RefreshHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	secret := r.Header.Get("X-Refresh-Secret")
	if subtle.ConstantTimeCompare([]byte(secret), []byte(h.Secret)) != 1 {
		writeErr(w, http.StatusUnauthorized, "invalid refresh secret")
		return
	}

	var in refreshInput
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&in) // empty body is fine: use defaults
	}

	end := time.Now().UTC().Truncate(24 * time.Hour)
	if in.End != "" {
		d, err := time.Parse("2006-01-02", in.End)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "end must be YYYY-MM-DD")
			return
		}
		end = d
	}
	start := end.AddDate(0, 0, -30)
	if in.Start != "" {
		d, err := time.Parse("2006-01-02", in.Start)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "start must be YYYY-MM-DD")
			return
		}
		start = d
	}
	if start.After(end) {
		writeErr(w, http.StatusBadRequest, "start must be before end")
		return
	}

	res, err := services.Ingest(r.Context(), h.Store, h.Frank, h.NBK, start, end)
	if err != nil {
		log.Printf("ingest: %v", err)
		writeErr(w, http.StatusBadGateway, "rate ingestion failed")
		return
	}

	// Refresh inflation for every currency in use, so newly published annual
	// figures appear automatically (the Jan 1 step shows up when data lands).
	inflationCountries := 0
	if h.WB != nil {
		currencies, err := h.Store.GetDistinctSalaryCurrencies(r.Context())
		if err != nil {
			log.Printf("refresh inflation: list currencies: %v", err)
		}
		for _, ccy := range currencies {
			country, ok := services.CountryForCurrency(ccy)
			if !ok {
				continue
			}
			if err := services.EnsureInflation(r.Context(), h.Store, h.WB, country, true); err != nil {
				log.Printf("refresh inflation %s: %v", country, err)
				continue
			}
			inflationCountries++
		}
	}

	// Fill missing news headlines, politely spaced for GDELT's rate limit
	// (observed: bursts get 429'd for ~a minute; 10s spacing stays clear).
	// This is the only place GDELT is called — chart requests serve the cache.
	newsFilled := services.EnrichPendingNews(r.Context(), h.Store, h.GDELT, 20, 10*time.Second)

	writeJSON(w, http.StatusOK, map[string]any{
		"frankfurter_rows":    res.FrankfurterRows,
		"nbk_rows":            res.NBKRows,
		"inflation_countries": inflationCountries,
		"news_filled":         newsFilled,
	})
}
