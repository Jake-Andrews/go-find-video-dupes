package sample

import (
	"image"
	"log/slog"

	"govdupes/internal/models"
	"govdupes/internal/videoprocessor"
)

type FastSampler struct {
	SkipPercent float64
	Frames      int
}

func (s *FastSampler) Sample(vp *videoprocessor.FFmpegWrapper, v *models.Video) (*models.Screenshots, []image.Image, error) {
	timestamps := createTimeStamps(v.Duration, models.NumImages, int(s.SkipPercent))
	images, err := createScreenshots(vp, timestamps, v)
	if err != nil {
		slog.Error("Error creating screenshots", slog.Any("error", err))
		return nil, nil, err
	}

	thumbnail := images[0]

	return &models.Screenshots{Screenshots: []image.Image{thumbnail}}, images, nil
}

func (s *FastSampler) Name() string {
	return "FastSampler"
}
