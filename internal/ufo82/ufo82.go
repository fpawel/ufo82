package ufo82

import (
	"time"
)

type PartyID int64
type ProductID int64

type Party struct {
	PartyID   PartyID   `db:"party_id"`
	CreatedAt time.Time `db:"created_at"`
}

type Product struct {
	ProductID     ProductID `db:"product_id"`
	PartyID       PartyID   `db:"party_id"`
	Order         int64     `db:"order_in_party"`
	ProductNumber int64     `db:"product_number"`
}

type Sensitivity struct {
	StoredAt time.Time `db:"stored_at"`
	Value    float64   `db:"value"`
}

type YearMonth struct {
	Year, Month int
}

type YearMonthDay struct {
	Year, Month, Day int
}

type ProductOrderSerial struct {
	Order, Serial int
}
