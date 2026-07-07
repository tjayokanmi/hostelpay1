package models

import "time"

type Subscription struct {
	ID                string
	StudentIdentifier string
	CardToken         string
	OccupancyType     string
	Active            bool
	NextChargeDate    time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}