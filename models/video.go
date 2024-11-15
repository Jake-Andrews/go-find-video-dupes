package models

import (
	"time"

	"github.com/google/uuid"
)

type Video struct {
	id        uuid.UUID   `db:"videoid"`
	path      string      `db:"path" validate:"required"`
	fileName  string      `db:"fileName" validate:"required"`
	hash      []Videohash `db:"hash"`
	createdAt time.Time   `db:"createdAt" validate:"required"`
}
