package sampler

import (
	"image"

	"govdupes/internal/models"
	"govdupes/internal/videoprocessor"
)

type Sampler interface {
	Sample(vp *videoprocessor.FFmpegWrapper, v *models.Video) (*models.Screenshots, []image.Image, error)
	Name() string
}
