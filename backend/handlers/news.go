package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"dynamics-dashboard/services"
)

// NewsHandler exposes the two halves of the news pipeline to the daily
// GitHub Actions workflow, which queries GDELT itself: cloud egress IPs
// (Render's included) are chronically rate-limited by GDELT, while GitHub
// runners get a fresh IP every run. Both endpoints require the refresh secret.
type NewsHandler struct {
	Store  services.NewsStore
	Secret string
}

func (h *NewsHandler) authorized(r *http.Request) bool {
	secret := r.Header.Get("X-Refresh-Secret")
	return subtle.ConstantTimeCompare([]byte(secret), []byte(h.Secret)) == 1
}

// Pending lists events awaiting a news check, with the GDELT query
// pre-built (terms + datetime window) so the workflow stays a dumb pipe.
func (h *NewsHandler) Pending(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "invalid refresh secret")
		return
	}
	limit := 20
	if s := r.URL.Query().Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 50 {
			writeErr(w, http.StatusBadRequest, "limit must be 1-50")
			return
		}
		limit = n
	}

	events, err := h.Store.GetEventsMissingNews(r.Context(), limit)
	if err != nil {
		log.Printf("news pending: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	type pendingItem struct {
		ID    int64  `json:"id"`
		Pair  string `json:"pair"`
		Terms string `json:"terms"`
		Start string `json:"start"` // GDELT datetime format
		End   string `json:"end"`
	}
	items := make([]pendingItem, 0, len(events))
	for _, e := range events {
		items = append(items, pendingItem{
			ID:    e.ID,
			Pair:  e.BaseCurrency + "/" + e.QuoteCurrency,
			Terms: services.NewsTermsForPair(e.BaseCurrency, e.QuoteCurrency),
			Start: e.RefDate.Format("20060102") + "000000",
			// The compared window plus a day of follow-up coverage.
			End: e.Date.AddDate(0, 0, 1).Format("20060102") + "235959",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type newsSubmission struct {
	Items []struct {
		ID       int64  `json:"id"`
		Headline string `json:"headline"`
		URL      string `json:"url"`
	} `json:"items"`
}

// Submit stores fetched headlines. An empty headline means "checked GDELT,
// nothing found" and is remembered so the event is never queried again.
func (h *NewsHandler) Submit(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "invalid refresh secret")
		return
	}
	var sub newsSubmission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(sub.Items) > 50 {
		writeErr(w, http.StatusBadRequest, "too many items (max 50)")
		return
	}

	stored := 0
	for _, item := range sub.Items {
		if item.ID <= 0 || len(item.Headline) > 300 || len(item.URL) > 2000 {
			continue
		}
		if err := h.Store.UpdateEventNews(r.Context(), item.ID, item.Headline, item.URL); err != nil {
			log.Printf("news submit %d: %v", item.ID, err)
			continue
		}
		stored++
	}
	writeJSON(w, http.StatusOK, map[string]any{"stored": stored})
}
