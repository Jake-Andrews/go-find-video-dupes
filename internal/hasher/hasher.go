package hasher

import (
	"image"

	"govdupes/internal/models"
)

type Hasher interface {
	CreateHash([]image.Image, *models.Video) (*models.Videohash, error)
}
