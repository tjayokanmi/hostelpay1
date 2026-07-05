package repository

import (
	"context"
	"fmt"
	"strings"

	"hostelpay/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PaymentRepository struct {
	db *pgxpool.Pool
}

func NewPaymentRepository(db *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) CreatePayment(ctx context.Context, p models.Payment) error {
	query := `
		INSERT INTO payments (
			order_reference, student_identifier, block, floor_level,
			room_number, occupancy_type, amount_paid, payment_status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Exec(ctx, query,
		p.OrderReference,
		p.StudentIdentifier,
		p.Block,
		p.FloorLevel,
		p.RoomNumber,
		p.OccupancyType,
		p.AmountPaid,
		models.StatusPending,
	)

	if err != nil {
		return fmt.Errorf("failed to insert payment: %w", err)
	}
	return nil
}

// Add this to internal/repository/payment_repository.go

func (r *PaymentRepository) UpdatePaymentStatusByOrderRef(ctx context.Context, orderReference, status, providerReference string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE payments
		SET payment_status = $1, provider_reference = $2, updated_at = CURRENT_TIMESTAMP
		WHERE order_reference = $3
	`, status, providerReference, orderReference)
	if err != nil {
		return fmt.Errorf("failed to update payment status: %w", err)
	}
	return nil
}
func (r *PaymentRepository) GetAllPayments(ctx context.Context, blockFilter, floorFilter string) ([]models.Payment, error) {
	query := `
		SELECT order_reference, student_identifier, block, floor_level, room_number, occupancy_type, amount_paid, payment_status, provider_reference
		FROM payments
		WHERE 1=1
	`
	var args []interface{}
	argID := 1

	if blockFilter != "" {
		query += fmt.Sprintf(" AND block = $%d", argID)
		args = append(args, blockFilter)
		argID++
	}
	if floorFilter != "" {
		query += fmt.Sprintf(" AND floor_level = $%d", argID)
		args = append(args, floorFilter)
		argID++
	}

	query += " ORDER BY updated_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query payments: %w", err)
	}
	defer rows.Close()

	var payments []models.Payment
	for rows.Next() {
		var p models.Payment
		var providerRef *string // provider_reference is nullable — must scan into a pointer

		err := rows.Scan(
			&p.OrderReference,
			&p.StudentIdentifier,
			&p.Block,
			&p.FloorLevel,
			&p.RoomNumber,
			&p.OccupancyType,
			&p.AmountPaid,
			&p.PaymentStatus,
			&providerRef,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan payment row: %w", err)
		}
		if providerRef != nil {
			p.ProviderReference = *providerRef
		}
		payments = append(payments, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating payment rows: %w", err)
	}

	return payments, nil
}

// MarkWebhookProcessed records a webhook's requestId, returning false if it was
// already processed (so the caller can skip re-applying side effects).
func (r *PaymentRepository) MarkWebhookProcessed(ctx context.Context, requestID string) (bool, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO processed_webhooks (request_id) VALUES ($1)
	`, requestID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return false, nil // already processed — not a real error
		}
		return false, fmt.Errorf("failed to mark webhook processed: %w", err)
	}
	return true, nil
}
