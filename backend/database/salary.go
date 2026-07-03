package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type SalaryEntry struct {
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	Amount        float64   `json:"amount"`
	CurrencyCode  string    `json:"currency_code"`
	EffectiveDate time.Time `json:"effective_date"`
	Note          *string   `json:"note"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

const salaryCols = `id, user_id, amount, currency_code, effective_date, note, created_at, updated_at`

func scanSalary(row pgx.Row) (SalaryEntry, error) {
	var e SalaryEntry
	err := row.Scan(&e.ID, &e.UserID, &e.Amount, &e.CurrencyCode, &e.EffectiveDate,
		&e.Note, &e.CreatedAt, &e.UpdatedAt)
	return e, err
}

func (s *Store) CreateSalaryEntry(ctx context.Context, userID int64, amount float64,
	currency string, effectiveDate time.Time, note *string) (SalaryEntry, error) {
	return scanSalary(s.Pool.QueryRow(ctx,
		`INSERT INTO salary_entries (user_id, amount, currency_code, effective_date, note)
		 VALUES ($1, $2, $3, $4, $5) RETURNING `+salaryCols,
		userID, amount, currency, effectiveDate, note))
}

func (s *Store) GetSalaryEntriesByUserID(ctx context.Context, userID int64) ([]SalaryEntry, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+salaryCols+` FROM salary_entries
		 WHERE user_id = $1 ORDER BY effective_date ASC, id ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []SalaryEntry{}
	for rows.Next() {
		e, err := scanSalary(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// UpdateSalaryEntry updates an entry only if it belongs to userID.
func (s *Store) UpdateSalaryEntry(ctx context.Context, id, userID int64, amount float64,
	currency string, effectiveDate time.Time, note *string) (SalaryEntry, error) {
	e, err := scanSalary(s.Pool.QueryRow(ctx,
		`UPDATE salary_entries
		 SET amount = $3, currency_code = $4, effective_date = $5, note = $6, updated_at = now()
		 WHERE id = $1 AND user_id = $2 RETURNING `+salaryCols,
		id, userID, amount, currency, effectiveDate, note))
	if errors.Is(err, pgx.ErrNoRows) {
		return SalaryEntry{}, ErrNotFound
	}
	return e, err
}

// DeleteSalaryEntry deletes an entry only if it belongs to userID.
func (s *Store) DeleteSalaryEntry(ctx context.Context, id, userID int64) error {
	tag, err := s.Pool.Exec(ctx,
		`DELETE FROM salary_entries WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
