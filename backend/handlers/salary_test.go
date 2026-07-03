package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"dynamics-dashboard/database"
)

// fakeSalaryStore is an in-memory SalaryStore enforcing ownership like the real one.
type fakeSalaryStore struct {
	entries map[int64]database.SalaryEntry
	nextID  int64
}

func newFakeSalaryStore() *fakeSalaryStore {
	return &fakeSalaryStore{entries: map[int64]database.SalaryEntry{}, nextID: 1}
}

func (f *fakeSalaryStore) CreateSalaryEntry(_ context.Context, userID int64, amount float64,
	currency string, date time.Time, note *string) (database.SalaryEntry, error) {
	e := database.SalaryEntry{ID: f.nextID, UserID: userID, Amount: amount,
		CurrencyCode: currency, EffectiveDate: date, Note: note}
	f.nextID++
	f.entries[e.ID] = e
	return e, nil
}

func (f *fakeSalaryStore) GetSalaryEntriesByUserID(_ context.Context, userID int64) ([]database.SalaryEntry, error) {
	out := []database.SalaryEntry{}
	for _, e := range f.entries {
		if e.UserID == userID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeSalaryStore) UpdateSalaryEntry(_ context.Context, id, userID int64, amount float64,
	currency string, date time.Time, note *string) (database.SalaryEntry, error) {
	e, ok := f.entries[id]
	if !ok || e.UserID != userID {
		return database.SalaryEntry{}, database.ErrNotFound
	}
	e.Amount, e.CurrencyCode, e.EffectiveDate, e.Note = amount, currency, date, note
	f.entries[id] = e
	return e, nil
}

func (f *fakeSalaryStore) DeleteSalaryEntry(_ context.Context, id, userID int64) error {
	e, ok := f.entries[id]
	if !ok || e.UserID != userID {
		return database.ErrNotFound
	}
	delete(f.entries, id)
	return nil
}

// asUser builds a request with the user ID already in context (bypassing JWT).
func asUser(userID int64, method, path, body string) *http.Request {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	return r.WithContext(context.WithValue(r.Context(), userIDKey, userID))
}

func newSalaryRouter(h *SalaryHandler) http.Handler {
	r := chi.NewRouter()
	r.Post("/api/salary", h.Create)
	r.Get("/api/salary", h.List)
	r.Put("/api/salary/{id}", h.Update)
	r.Delete("/api/salary/{id}", h.Delete)
	return r
}

func TestSalaryCreateValidation(t *testing.T) {
	h := &SalaryHandler{Store: newFakeSalaryStore()}
	router := newSalaryRouter(h)

	cases := []struct {
		name string
		body string
		want int
	}{
		{"valid", `{"amount":500000,"currency_code":"KZT","effective_date":"2024-01-15","note":"promotion"}`, 201},
		{"zero amount", `{"amount":0,"currency_code":"KZT","effective_date":"2024-01-15"}`, 400},
		{"negative amount", `{"amount":-10,"currency_code":"KZT","effective_date":"2024-01-15"}`, 400},
		{"bad currency", `{"amount":100,"currency_code":"kzt!","effective_date":"2024-01-15"}`, 400},
		{"bad date", `{"amount":100,"currency_code":"USD","effective_date":"15/01/2024"}`, 400},
		{"far future date", `{"amount":100,"currency_code":"USD","effective_date":"2099-01-01"}`, 400},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, asUser(1, http.MethodPost, "/api/salary", tc.body))
		if w.Code != tc.want {
			t.Errorf("%s: status = %d, want %d; body: %s", tc.name, w.Code, tc.want, w.Body)
		}
	}
}

func TestSalaryOwnership(t *testing.T) {
	h := &SalaryHandler{Store: newFakeSalaryStore()}
	router := newSalaryRouter(h)

	// User 1 creates an entry.
	w := httptest.NewRecorder()
	router.ServeHTTP(w, asUser(1, http.MethodPost, "/api/salary",
		`{"amount":1000,"currency_code":"USD","effective_date":"2024-01-01"}`))
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status = %d; body: %s", w.Code, w.Body)
	}

	// User 2 must NOT be able to update or delete user 1's entry.
	w = httptest.NewRecorder()
	router.ServeHTTP(w, asUser(2, http.MethodPut, "/api/salary/1",
		`{"amount":9999,"currency_code":"USD","effective_date":"2024-01-01"}`))
	if w.Code != http.StatusNotFound {
		t.Errorf("cross-user update: status = %d, want 404", w.Code)
	}
	w = httptest.NewRecorder()
	router.ServeHTTP(w, asUser(2, http.MethodDelete, "/api/salary/1", ""))
	if w.Code != http.StatusNotFound {
		t.Errorf("cross-user delete: status = %d, want 404", w.Code)
	}

	// User 2's list must be empty.
	w = httptest.NewRecorder()
	router.ServeHTTP(w, asUser(2, http.MethodGet, "/api/salary", ""))
	if !strings.Contains(w.Body.String(), `"entries":[]`) {
		t.Errorf("cross-user list: body = %s, want empty entries", w.Body)
	}

	// Owner CAN update and delete.
	w = httptest.NewRecorder()
	router.ServeHTTP(w, asUser(1, http.MethodPut, "/api/salary/1",
		`{"amount":2000,"currency_code":"EUR","effective_date":"2024-02-01","note":"raise"}`))
	if w.Code != http.StatusOK {
		t.Errorf("owner update: status = %d, want 200; body: %s", w.Code, w.Body)
	}
	w = httptest.NewRecorder()
	router.ServeHTTP(w, asUser(1, http.MethodDelete, "/api/salary/1", ""))
	if w.Code != http.StatusOK {
		t.Errorf("owner delete: status = %d, want 200", w.Code)
	}
}

func TestSalaryListSorted(t *testing.T) {
	h := &SalaryHandler{Store: newFakeSalaryStore()}
	router := newSalaryRouter(h)

	for i, date := range []string{"2023-05-01", "2022-01-01", "2024-03-01"} {
		w := httptest.NewRecorder()
		body := fmt.Sprintf(`{"amount":%d,"currency_code":"USD","effective_date":"%s"}`, (i+1)*1000, date)
		router.ServeHTTP(w, asUser(1, http.MethodPost, "/api/salary", body))
		if w.Code != http.StatusCreated {
			t.Fatalf("create %d: status = %d", i, w.Code)
		}
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, asUser(1, http.MethodGet, "/api/salary", ""))
	if w.Code != http.StatusOK {
		t.Fatalf("list: status = %d", w.Code)
	}
}
