package phash

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"log"
	"time"

	"govdupes/internal/models"
	"govdupes/internal/videoprocessor"

	"github.com/corona10/goimagehash"
	"golang.org/x/image/bmp"
)

// 16 screenshots from the video (32x32) are stitched together into one 128x128
// image
var (
	numImages int = 16
	gridSize  int = 4
	scHeight  int = 480
	scWidth   int = 854
)

func Create(vp *videoprocessor.FFmpegWrapper, v *models.Video) (*uint64, error) {
	timestamps := createTimeStamps(v.Duration, numImages)
	images, err := createScreenshots(vp, timestamps, v)
	if err != nil {
		log.Printf("Error creating screenshots, err: %v\n", err)
		return nil, err
	}

	image, err := createCollage(images)
	if err != nil {
		log.Printf("Error creating collage, err: %v\n", err)
	}

	hash, err := goimagehash.PerceptionHash(image)
	if err != nil {
		log.Printf("Error creating phash, err: %v", err)
	}
	h := hash.GetHash()
	log.Printf("File: %q has pHash: %q", v.FileName, hash.ToString())

	return &h, nil
}

func createTimeStamps(duration time.Duration, numTimestamps int) []string {
	if numTimestamps <= 0 {
		return nil
	}

	intro := duration / 10
	outro := duration * 9 / 10
	interval := (outro - intro) / time.Duration(numTimestamps)

	var timestamps []string
	for i := 0; i < numTimestamps; i++ {
		t := durationToFFmpegTimestamp(intro + time.Duration(i)*interval)
		timestamps = append(timestamps, t)
	}

	return timestamps
}

func durationToFFmpegTimestamp(d time.Duration) string {
	totalMilliseconds := d.Milliseconds()
	hours := totalMilliseconds / (1000 * 60 * 60)
	minutes := (totalMilliseconds / (1000 * 60)) % 60
	seconds := (totalMilliseconds / 1000) % 60
	milliseconds := totalMilliseconds % 1000

	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, milliseconds)
}

func createScreenshots(vp *videoprocessor.FFmpegWrapper, timestamps []string, v *models.Video) ([]image.Image, error) {
	images := []image.Image{}
	buf := bytes.Buffer{}

	for _, t := range timestamps {
		vp.ScreenshotAtTime(v.Path, &buf, t)

		img, err := bmp.Decode(&buf)
		if err != nil {
			log.Printf("Error decoding image, err: %v\n", err)
			return images, err
		}

		images = append(images, img)
		buf.Reset()
	}
	return images, nil
}

func createCollage(images []image.Image) (image.Image, error) {
	if len(images) != numImages {
		return nil, fmt.Errorf("expected %d images, got %d", numImages, len(images))
	}

	// checking that the dimensions of the images matches what was expected
	for i, img := range images {
		if img == nil {
			return nil, fmt.Errorf("image at index %d is nil", i)
		}
		bounds := img.Bounds()
		width, height := bounds.Dx(), bounds.Dy()
		if width != scWidth || height != scHeight {
			return nil, fmt.Errorf("image at index %d has invalid dimensions: %dx%d, expected %dx%d", i, width, height, scWidth, scHeight)
		}
	}

	collageWidth := gridSize * scWidth
	collageHeight := gridSize * scHeight

	collage := image.NewRGBA(image.Rect(0, 0, collageWidth, collageHeight))

	for i, img := range images {
		x := (i % gridSize) * scWidth
		y := (i / gridSize) * scHeight
		draw.Draw(collage, image.Rect(x, y, x+scWidth, y+scHeight), img, img.Bounds().Min, draw.Src)
	}

	return collage, nil
}
