package hasher

import "govdupes/internal/models"

type Hasher interface {
	CreateHash([]byte) models.Videohash
}
