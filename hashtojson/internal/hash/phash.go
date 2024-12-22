package hash

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"log"

	"govdupes/internal/models"
	"govdupes/internal/videoprocessor"

	"github.com/corona10/goimagehash"
	"golang.org/x/image/bmp"
)

func Create(vp *videoprocessor.FFmpegWrapper, v *models.Video) (*models.Videohash, *models.Screenshots, error) {
	timestamps := createTimeStamps(v.Duration, models.NumImages)
	images, err := createScreenshots(vp, timestamps, v)
	if err != nil {
		log.Printf("Error creating screenshots, err: %v\n", err)
		return nil, nil, err
	}

	screenshots := models.Screenshots{Screenshots: images}

	image, err := createCollage(images)
	if err != nil {
		log.Printf("Error creating collage, err: %v\n", err)
	}

	hash, err := goimagehash.PerceptionHash(image)
	if err != nil {
		log.Printf("Error creating phash, err: %v", err)
	}
	h := hash.ToString()
	log.Printf("File: %q has pHash: %q", v.FileName, hash.ToString())

	pHash := createPhash(v, h)
	log.Println(*pHash)

	return pHash, &screenshots, nil
}

func createTimeStamps(duration float32, numTimestamps int) []string {
	if numTimestamps <= 0 {
		return nil
	}
	// Skip intro and outro
	intro := duration / 10
	outro := duration * 9 / 10
	interval := (outro - intro) / float32(numTimestamps)

	var timestamps []string
	for i := 0; i < numTimestamps; i++ {
		t := durationToFFmpegTimestamp(intro + float32(i)*interval)
		timestamps = append(timestamps, t)
	}

	return timestamps
}

func durationToFFmpegTimestamp(d float32) string {
	// Convert duration in seconds to milliseconds
	totalMilliseconds := int(d * 1000)

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

func createPhash(v *models.Video, h string) *models.Videohash {
	pHash := models.Videohash{
		ID:        v.ID,
		HashType:  "pHash",
		HashValue: h,
		Duration:  v.Duration,
		Bucket:    -1,
	}
	return &pHash
}

func ConvertImagesToBase64(images []image.Image) ([]string, error) {
	var encodedStrings []string
	for _, img := range images {
		var buf bytes.Buffer

		err := bmp.Encode(&buf, img)
		if err != nil {
			return nil, fmt.Errorf("failed to encode image: %w", err)
		}

		encodedString := base64.StdEncoding.EncodeToString(buf.Bytes())
		encodedStrings = append(encodedStrings, encodedString)
	}
	return encodedStrings, nil
}
