package models

import (
	"time"

	"github.com/google/uuid"
)

type Video struct {
	VideoID    uuid.UUID   `db:"videoID"`
	Path       string      `db:"path"`     // validate:"required"`
	FileName   string      `db:"fileName"` // validate:"required"`
	Hash       []Videohash `db:"hash"`
	CreatedAt  time.Time   `db:"createdAt"`
	ModifiedAt time.Time   `db:"modifiedAt"` // validate:"required"`
	Size       int64       `db:"size"`       // validate:"required"`
	FrameRate  float32     `db:"frameRate"`  // validate:"required"`
	Format     string      `db:"format"`     // validate:"required"`
	BitRate    int         `db:"bitRate"`    // validate:"required"`
	VideoCodec string      `db:"videoCodec"`
	AudioCodec string      `db:"audioCodec"`
	Duration   time.Time   `db:"duration"`
}
