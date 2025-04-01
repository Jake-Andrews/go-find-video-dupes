package sample

import (
	"bytes"
	"fmt"
	"image"
	"log/slog"
	"math"

	"govdupes/internal/models"
	"govdupes/internal/videoprocessor"

	"golang.org/x/image/bmp"
)

type SlowSampler struct {
	SkipPercent float64
	FPS         float64
}

func (s *SlowSampler) Sample(vp *videoprocessor.FFmpegWrapper, v *models.Video) (*models.Screenshots, []image.Image, error) {
	numFrames := int(math.Floor(float64(v.Duration)))
	if numFrames == 0 {
		return nil, nil, fmt.Errorf("error numFrames == 0 for slowPhash")
	}

	timestamps := createTimeStamps(v.Duration, numFrames, int(s.SkipPercent))
	images, err := createScreenshots(vp, timestamps, v)
	if err != nil {
		slog.Error("Error creating screenshots", slog.Any("error", err))
		return nil, nil, err
	}

	var thumbIndex int
	if len(images) < 6 {
		thumbIndex = 0
	} else {
		thumbIndex = 5
	}
	thumbnail := images[thumbIndex]

	return &models.Screenshots{Screenshots: []image.Image{thumbnail}}, images, nil
}

// **remove**
func (s *SlowSampler) Name() string {
	return "SlowSampler"
}

func createScreenshots(vp *videoprocessor.FFmpegWrapper, timestamps []string, v *models.Video) ([]image.Image, error) {
	images := []image.Image{}
	buf := bytes.Buffer{}

	for _, t := range timestamps {
		err := vp.ScreenshotAtTime(v.Path, &buf, t)
		if err != nil {
			return nil, fmt.Errorf("skipping file, cannot generate screenshots, err: %q", err)
		}

		img, err := bmp.Decode(&buf)
		if err != nil {
			slog.Error("Error decoding image", slog.Any("error", err))
			return images, err
		}

		images = append(images, img)
		buf.Reset()
	}

	return images, nil
}
