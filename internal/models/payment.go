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
	Block             string    `json:"block"`
	FloorLevel        string    `json:"floorLevel"`
	RoomNumber        string    `json:"roomNumber"`
	OccupancyType     string    `json:"occupancyType"`
	AmountPaid        string    `json:"amountPaid"` // e.g. "15000.00" — safe string mapping to NUMERIC(12,2)
	PaymentStatus     string    `json:"paymentStatus"`
	ProviderReference string    `json:"providerReference"` // populated once the webhook confirms payment; empty for PENDING rows
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}
