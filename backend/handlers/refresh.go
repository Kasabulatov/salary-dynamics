package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"dynamics-dashboard/services"
)

// RefreshHandler triggers rate ingestion. It is called by a scheduled job
// (GitHub Actions cron in production) and protected by a shared secret.
type RefreshHandler struct {
	Store  services.RateStore
	Frank  services.USDSeriesFetcher
	NBK    services.DayFetcher
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
	writeJSON(w, http.StatusOK, res)
}
