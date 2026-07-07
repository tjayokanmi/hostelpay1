package models

import "time"

type Student struct {
	ID                string
	StudentIdentifier string
	FullName          string
	PasswordHash      string
	CreatedAt         time.Time
}

type Session struct {
	Token     string
	StudentID string
	ExpiresAt time.Time
}
