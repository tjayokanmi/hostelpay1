package models

import "time"

const (
	StatusPending = "PENDING"
	StatusSuccess = "SUCCESS"
	StatusFailed  = "FAILED"
)

type Payment struct {
	ID                string    `json:"id"`
	OrderReference    string    `json:"orderReference"`
	StudentIdentifier string    `json:"studentIdentifier"`
	StudentFullName   string    `json:"studentFullName"` // joined from students table, not stored on payments itself
	Block             string    `json:"block"`
	FloorLevel        string    `json:"floorLevel"`
	RoomNumber        string    `json:"roomNumber"`
	OccupancyType     string    `json:"occupancyType"`
	AmountPaid        string    `json:"amountPaid"`
	PaymentStatus     string    `json:"paymentStatus"`
	ProviderReference string    `json:"providerReference"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}
