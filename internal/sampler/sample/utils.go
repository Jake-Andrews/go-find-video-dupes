package sample

import (
	"fmt"
	"log"
	"math"
)

func createTimeStamps(duration float64, numFrames int, skipPercent int) []string {
	if numFrames <= 0 {
		log.Fatalf("createTimeStamps: invalid numFrames (%d), must be > 0", numFrames)
	}
	if skipPercent < 0 || skipPercent >= 50 {
		log.Fatalf("createTimeStamps: invalid skipPercent (%d), must be in [0, 49]", skipPercent)
	}

	skipRatio := float64(skipPercent) / 100.0
	intro := duration * skipRatio
	outro := duration * (1.0 - skipRatio)
	interval := (outro - intro) / float64(numFrames)

	timestamps := make([]string, numFrames)
	for i := range numFrames {
		t := intro + float64(i)*interval
		timestamps[i] = durationToFFmpegTimestamp(t)
	}

	return timestamps
}

func durationToFFmpegTimestamp(seconds float64) string {
	totalMilliseconds := int(math.Round(seconds * 1000))

	hours := totalMilliseconds / (1000 * 60 * 60)
	minutes := (totalMilliseconds / (1000 * 60)) % 60
	secs := (totalMilliseconds / 1000) % 60
	millis := totalMilliseconds % 1000

	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, secs, millis)
}
