package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CardTokenRepository struct {
	db *pgxpool.Pool
}

func NewCardTokenRepository(db *pgxpool.Pool) *CardTokenRepository {
	return &CardTokenRepository{db: db}
}

func (r *CardTokenRepository) UpsertCardToken(ctx context.Context, studentIdentifier, tokenKey, customerEmail string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO saved_card_tokens (student_identifier, token_key, customer_email)
		VALUES ($1, $2, $3)
		ON CONFLICT (student_identifier) DO UPDATE
		SET token_key = EXCLUDED.token_key,
		    customer_email = EXCLUDED.customer_email,
		    updated_at = CURRENT_TIMESTAMP
	`, studentIdentifier, tokenKey, customerEmail)
	if err != nil {
		return fmt.Errorf("failed to upsert card token: %w", err)
	}
	return nil
}

func (r *CardTokenRepository) GetCardToken(ctx context.Context, studentIdentifier string) (string, error) {
	var tokenKey string
	err := r.db.QueryRow(ctx, `
		SELECT token_key FROM saved_card_tokens WHERE student_identifier = $1
	`, studentIdentifier).Scan(&tokenKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("failed to get card token: %w", err)
	}
	return tokenKey, nil
}
