package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"dynamics-dashboard/database"
)

// SalaryStore is the subset of database.Store the salary handlers need.
type SalaryStore interface {
	CreateSalaryEntry(ctx context.Context, userID int64, amount float64, currency string, effectiveDate time.Time, note *string) (database.SalaryEntry, error)
	GetSalaryEntriesByUserID(ctx context.Context, userID int64) ([]database.SalaryEntry, error)
	UpdateSalaryEntry(ctx context.Context, id, userID int64, amount float64, currency string, effectiveDate time.Time, note *string) (database.SalaryEntry, error)
	DeleteSalaryEntry(ctx context.Context, id, userID int64) error
}

// CurrencyConverter converts an amount using the rate on the given date.
type CurrencyConverter interface {
	Convert(ctx context.Context, amount float64, from, to string, date time.Time) (float64, time.Time, error)
}

type SalaryHandler struct {
	Store     SalaryStore
	Converter CurrencyConverter // nil disables conversion (e.g. in CRUD tests)
}

// entryResponse is a SalaryEntry plus its historical conversion.
type entryResponse struct {
	database.SalaryEntry
	ConvertedAmount *float64 `json:"converted_amount"`
	DisplayCurrency string   `json:"display_currency"`
	RateDate        *string  `json:"rate_date"`
	ConversionError string   `json:"conversion_error,omitempty"`
}

type salaryInput struct {
	Amount        float64 `json:"amount"`
	CurrencyCode  string  `json:"currency_code"`
	EffectiveDate string  `json:"effective_date"` // YYYY-MM-DD
	Note          *string `json:"note"`
}

var currencyRe = regexp.MustCompile(`^[A-Z]{3}$`)

func (in *salaryInput) validate() (time.Time, string, bool) {
	if in.Amount <= 0 {
		return time.Time{}, "amount must be greater than 0", false
	}
	if !currencyRe.MatchString(in.CurrencyCode) {
		return time.Time{}, "currency_code must be a 3-letter ISO 4217 code (e.g. USD, KZT)", false
	}
	date, err := time.Parse("2006-01-02", in.EffectiveDate)
	if err != nil {
		return time.Time{}, "effective_date must be a valid date in YYYY-MM-DD format", false
	}
	if date.After(time.Now().AddDate(1, 0, 0)) {
		return time.Time{}, "effective_date cannot be more than a year in the future", false
	}
	return date, "", true
}

func (h *SalaryHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFrom(r.Context())

	var in salaryInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	date, msg, ok := in.validate()
	if !ok {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}

	entry, err := h.Store.CreateSalaryEntry(r.Context(), userID, in.Amount, in.CurrencyCode, date, in.Note)
	if err != nil {
		log.Printf("create salary entry: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

func (h *SalaryHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFrom(r.Context())

	display := strings.ToUpper(r.URL.Query().Get("displayCurrency"))
	if display == "" {
		display = "USD"
	}
	if !currencyRe.MatchString(display) {
		writeErr(w, http.StatusBadRequest, "displayCurrency must be a 3-letter ISO 4217 code")
		return
	}

	entries, err := h.Store.GetSalaryEntriesByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("list salary entries: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	out := make([]entryResponse, 0, len(entries))
	for _, e := range entries {
		resp := entryResponse{SalaryEntry: e, DisplayCurrency: display}
		if h.Converter != nil {
			converted, rateDate, err := h.Converter.Convert(
				r.Context(), e.Amount, e.CurrencyCode, display, e.EffectiveDate)
			if err != nil {
				resp.ConversionError = "no exchange rate available for this date"
				log.Printf("convert entry %d (%s->%s on %s): %v",
					e.ID, e.CurrencyCode, display, e.EffectiveDate.Format("2006-01-02"), err)
			} else {
				rd := rateDate.Format("2006-01-02")
				resp.ConvertedAmount = &converted
				resp.RateDate = &rd
			}
		}
		out = append(out, resp)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"entries":          out,
		"display_currency": display,
	})
}

func (h *SalaryHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFrom(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid entry ID")
		return
	}

	var in salaryInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	date, msg, ok := in.validate()
	if !ok {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}

	entry, err := h.Store.UpdateSalaryEntry(r.Context(), id, userID, in.Amount, in.CurrencyCode, date, in.Note)
	if errors.Is(err, database.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "entry not found")
		return
	}
	if err != nil {
		log.Printf("update salary entry: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func (h *SalaryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFrom(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid entry ID")
		return
	}

	err = h.Store.DeleteSalaryEntry(r.Context(), id, userID)
	if errors.Is(err, database.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "entry not found")
		return
	}
	if err != nil {
		log.Printf("delete salary entry: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
