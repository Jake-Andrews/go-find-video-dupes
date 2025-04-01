package hash

import (
	"fmt"
	"image"
	"log/slog"
	"strings"

	"govdupes/internal/models"

	"github.com/corona10/goimagehash"
)

type SlowHasher struct{}

func (fh *SlowHasher) CreateHash(imgs []image.Image, v *models.Video) (*models.Videohash, error) {
	var builder strings.Builder

	for i, img := range imgs {

		hash, hashErr := goimagehash.PerceptionHash(img)
		if hashErr != nil {
			slog.Warn("SlowPhash: skipping frame can't compute pHash", slog.Int("frameIndex", i), slog.Any("error", hashErr))
			continue
		}

		// skip "p: " prefix in hash and append
		builder.WriteString(hash.ToString()[2:])
	}

	combinedHash := builder.String()
	if len(combinedHash) == 0 {
		return nil, fmt.Errorf("SlowPhash: no valid pHashes generated")
	}

	slog.Info("SlowPhash: computed combined pHash from n screenshots",
		slog.String("file", v.FileName),
		slog.Int("framesUsed", len(imgs)),
		slog.String("combinedHash", combinedHash),
	)

	pHash := createPhash(v, combinedHash, models.SlowHash)
	slog.Debug("Created pHash", slog.Any("pHash", *pHash))

	return pHash, nil
}
