package hash

import (
	"fmt"
	"image"
	"image/draw"
	"log/slog"

	"govdupes/internal/models"

	"github.com/corona10/goimagehash"
)

type FastHasher struct{}

func (fh *FastHasher) CreateHash(imgs []image.Image, v *models.Video) (*models.Videohash, error) {
	img, err := createCollage(imgs)
	if err != nil {
		slog.Error("Error creating collage", slog.Any("error", err))
		return nil, err
	}

	hash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		slog.Error("Error creating phash", slog.Any("error", err))
		return nil, err
	}

	h := hash.ToString()
	slog.Info("File has pHash", slog.String("file", v.FileName), slog.String("pHash", hash.ToString()))

	pHash := createPhash(v, h, models.FastHash)
	slog.Debug("Created pHash", slog.Any("pHash", *pHash))

	return pHash, nil
}

func createCollage(images []image.Image) (image.Image, error) {
	if len(images) != models.NumImages {
		return nil, fmt.Errorf("expected %d images, got %d", models.NumImages, len(images))
	}

	for i, img := range images {
		if img == nil {
			return nil, fmt.Errorf("image at index %d is nil", i)
		}
		bounds := img.Bounds()
		width, height := bounds.Dx(), bounds.Dy()
		if width != models.Width || height != models.Height {
			return nil, fmt.Errorf("image at index %d has invalid dimensions: %dx%d, expected %dx%d", i, width, height, models.Width, models.Height)
		}
	}

	collageWidth := models.GridSize * models.Width
	collageHeight := models.GridSize * models.Height

	collage := image.NewRGBA(image.Rect(0, 0, collageWidth, collageHeight))

	for i, img := range images {
		x := (i % models.GridSize) * models.Width
		y := (i / models.GridSize) * models.Height
		draw.Draw(collage, image.Rect(x, y, x+models.Width, y+models.Height), img, img.Bounds().Min, draw.Src)
	}

	return collage, nil
}
