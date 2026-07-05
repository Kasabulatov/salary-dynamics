package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dynamics-dashboard/database"
)

type fakeNewsStore struct {
	pending []database.CurrencyEvent
	updates map[int64][2]string
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

func newsRequest(t *testing.T, h http.HandlerFunc, method, path, secret, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	if secret != "" {
		req.Header.Set("X-Refresh-Secret", secret)
	}
	w := httptest.NewRecorder()
	h(w, req)
	return w
}

func TestNewsEndpointsRequireSecret(t *testing.T) {
	h := &NewsHandler{Store: &fakeNewsStore{}, Secret: "right"}
	if w := newsRequest(t, h.Pending, http.MethodGet, "/api/internal/news/pending", "wrong", ""); w.Code != http.StatusUnauthorized {
		t.Errorf("pending with wrong secret: %d, want 401", w.Code)
	}
	if w := newsRequest(t, h.Submit, http.MethodPost, "/api/internal/news", "", `{"items":[]}`); w.Code != http.StatusUnauthorized {
		t.Errorf("submit without secret: %d, want 401", w.Code)
	}
}

func TestNewsPendingBuildsGDELTQuery(t *testing.T) {
	date := time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC)
	store := &fakeNewsStore{pending: []database.CurrencyEvent{{
		ID: 7, BaseCurrency: "KZT", QuoteCurrency: "USD",
		Date: date, RefDate: date.AddDate(0, 0, -14), ChangeWindow: "weekly",
	}}}
	h := &NewsHandler{Store: store, Secret: "s"}

	w := newsRequest(t, h.Pending, http.MethodGet, "/api/internal/news/pending", "s", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", w.Code, w.Body)
	}
	body := w.Body.String()
	for _, want := range []string{
		`"id":7`, `"pair":"KZT/USD"`, `"terms":"Kazakhstan tenge"`,
		`"start":"20240616000000"`, `"end":"20240701235959"`, // window + 1 day
	} {
		if !strings.Contains(body, want) {
			t.Errorf("pending response missing %s; body: %s", want, body)
		}
	}
}

func TestNewsSubmitStoresHeadlines(t *testing.T) {
	store := &fakeNewsStore{}
	h := &NewsHandler{Store: store, Secret: "s"}

	w := newsRequest(t, h.Submit, http.MethodPost, "/api/internal/news", "s",
		`{"items":[
			{"id":7,"headline":"Tenge slides after policy shift","url":"https://example.com/a"},
			{"id":8,"headline":"","url":""},
			{"id":0,"headline":"invalid id","url":""}
		]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), `"stored":2`) {
		t.Errorf("stored count wrong; body: %s", w.Body)
	}
	if got := store.updates[7]; got[0] != "Tenge slides after policy shift" {
		t.Errorf("event 7 = %v, want headline stored", got)
	}
	// Empty headline = "checked, nothing found" — must be persisted too.
	if got, ok := store.updates[8]; !ok || got[0] != "" {
		t.Errorf("event 8 = %v, want empty marker stored", got)
	}
	if _, ok := store.updates[0]; ok {
		t.Error("invalid id was stored")
	}
}
