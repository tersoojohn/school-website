// Package repository is the only layer that talks to SQL. Every query is parameterised.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrConflict   = errors.New("already exists")
	ErrIncomplete = errors.New("some students have no scores yet")
)

type Store struct{ db *sql.DB }

func New(db *sql.DB) *Store { return &Store{db: db} }

type rowScanner interface{ Scan(dest ...any) error }

func mapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, sql.ErrNoRows):
		return ErrNotFound
	case strings.Contains(err.Error(), "UNIQUE constraint failed"):
		return ErrConflict
	}
	return err
}

func (s *Store) tx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return mapErr(err)
	}
	return tx.Commit()
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func ts(sec int64) time.Time { return time.Unix(sec, 0) }
