package repository

import (
	"context"
	"fmt"

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
