package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"hostelpay/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SubscriptionRepository struct {
	db *pgxpool.Pool
}

func NewSubscriptionRepository(db *pgxpool.Pool) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

func (r *SubscriptionRepository) CreateSubscription(ctx context.Context, studentIdentifier, cardToken, occupancyType string, nextChargeDate time.Time) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO subscriptions (student_identifier, card_token, occupancy_type, next_charge_date)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, studentIdentifier, cardToken, occupancyType, nextChargeDate).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to create subscription: %w", err)
	}
	return id, nil
}

func (r *SubscriptionRepository) GetActiveSubscriptionByStudent(ctx context.Context, studentIdentifier string) (*models.Subscription, error) {
	var s models.Subscription
	err := r.db.QueryRow(ctx, `
		SELECT id, student_identifier, card_token, occupancy_type, active, next_charge_date, created_at, updated_at
		FROM subscriptions
		WHERE student_identifier = $1 AND active = true
		ORDER BY created_at DESC
		LIMIT 1
	`, studentIdentifier).Scan(&s.ID, &s.StudentIdentifier, &s.CardToken, &s.OccupancyType, &s.Active, &s.NextChargeDate, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}
	return &s, nil
}

func (r *SubscriptionRepository) CancelSubscription(ctx context.Context, subscriptionID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE subscriptions SET active = false, updated_at = CURRENT_TIMESTAMP WHERE id = $1
	`, subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to cancel subscription: %w", err)
	}
	return nil
}

// ListDueSubscriptions returns active subscriptions whose next_charge_date has arrived.
// Called by the manual billing-run endpoint (and, eventually, an automated scheduler).
func (r *SubscriptionRepository) ListDueSubscriptions(ctx context.Context) ([]models.Subscription, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, student_identifier, card_token, occupancy_type, active, next_charge_date, created_at, updated_at
		FROM subscriptions
		WHERE active = true AND next_charge_date <= CURRENT_DATE
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list due subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []models.Subscription
	for rows.Next() {
		var s models.Subscription
		if err := rows.Scan(&s.ID, &s.StudentIdentifier, &s.CardToken, &s.OccupancyType, &s.Active, &s.NextChargeDate, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan subscription row: %w", err)
		}
		subs = append(subs, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating subscription rows: %w", err)
	}
	return subs, nil
}

func (r *SubscriptionRepository) AdvanceNextChargeDate(ctx context.Context, subscriptionID string, newDate time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE subscriptions SET next_charge_date = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2
	`, newDate, subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to advance next charge date: %w", err)
	}
	return nil
}
