package repository

import (
	"context"
	"time"

	"schoolportal/internal/models"
)

// GetBank returns the stored record; AccountNumber holds the still-encrypted value.
func (s *Store) GetBank(ctx context.Context, userID int64) (*models.BankDetails, error) {
	var b models.BankDetails
	var updated int64
	err := s.db.QueryRowContext(ctx, `SELECT bank_name, account_name, account_number, last4, updated_at
		FROM staff_bank_details WHERE user_id = ?`, userID).Scan(&b.BankName, &b.AccountName, &b.AccountNumber, &b.Last4, &updated)
	if err != nil {
		return nil, mapErr(err)
	}
	b.UpdatedAt = ts(updated)
	return &b, nil
}

func (s *Store) SaveBank(ctx context.Context, userID int64, bankName, accountName, encryptedNumber, last4 string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO staff_bank_details(user_id, bank_name, account_name, account_number, last4, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET bank_name = excluded.bank_name, account_name = excluded.account_name,
			account_number = excluded.account_number, last4 = excluded.last4, updated_at = excluded.updated_at`,
		userID, bankName, accountName, encryptedNumber, last4, time.Now().Unix())
	return err
}
