package hash

import (
	"govdupes/internal/models"
)

func createPhash(v *models.Video, hash string, hashType models.HashType) *models.Videohash {
	pHash := models.Videohash{
		ID:        v.ID,
		HashType:  hashType,
		HashValue: hash,
		Duration:  v.Duration,
		Bucket:    -1,
	}
	return &pHash
}
