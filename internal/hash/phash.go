package phash

import (
	"fmt"
	"image"
	"log"
	"time"

	"govdupes/internal/models"

	"github.com/corona10/goimagehash"
)

var imgsize int = 16

func Create(v *models.Video) (*uint64, error) {
	timestamps := createTimeStamps(v.Duration, imgsize)
	img, err := createScreenshot(v.Path, timestamps, v)
	if err != nil {
		log.Printf("Error creating screenshots, err: %v\n", err)
		return nil, err
	}

	hash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		log.Printf("Error creating phash, err: %v", err)
	}
	h := hash.GetHash()
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

func createScreenshot(videoPath string, timestamps []string, v *models.Video) (image.Image, error) {
}
