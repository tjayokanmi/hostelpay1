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

type StudentRepository struct {
	db *pgxpool.Pool
}

func NewStudentRepository(db *pgxpool.Pool) *StudentRepository {
	return &StudentRepository{db: db}
}

func (r *StudentRepository) CreateStudent(ctx context.Context, identifier, passwordHash string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO students (student_identifier, password_hash)
		VALUES ($1, $2)
		RETURNING id
	`, identifier, passwordHash).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to create student: %w", err)
	}
	return id, nil
}

func (r *StudentRepository) GetStudentByIdentifier(ctx context.Context, identifier string) (*models.Student, error) {
	var s models.Student
	err := r.db.QueryRow(ctx, `
		SELECT id, student_identifier, password_hash, created_at
		FROM students WHERE student_identifier = $1
	`, identifier).Scan(&s.ID, &s.StudentIdentifier, &s.PasswordHash, &s.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get student: %w", err)
	}
	return &s, nil
}

func (r *StudentRepository) CreateSession(ctx context.Context, studentID, token string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO sessions (token, student_id, expires_at)
		VALUES ($1, $2, $3)
	`, token, studentID, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

func (r *StudentRepository) GetSession(ctx context.Context, token string) (*models.Session, error) {
	var s models.Session
	err := r.db.QueryRow(ctx, `
		SELECT token, student_id, expires_at FROM sessions
		WHERE token = $1 AND expires_at > CURRENT_TIMESTAMP
	`, token).Scan(&s.Token, &s.StudentID, &s.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	return &s, nil
}

func (r *StudentRepository) DeleteSession(ctx context.Context, token string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, token)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

func (r *StudentRepository) GetStudentIdentifierByID(ctx context.Context, studentID string) (string, error) {
	var identifier string
	err := r.db.QueryRow(ctx, `SELECT student_identifier FROM students WHERE id = $1`, studentID).Scan(&identifier)
	if err != nil {
		return "", fmt.Errorf("failed to get student identifier: %w", err)
	}
	return identifier, nil
}
