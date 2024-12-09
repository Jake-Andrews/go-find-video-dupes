package models

import (
	"fmt"
	"time"
)

type Video struct {
	VideoID    int64         `db:"videoID" json:"videoID"`
	Path       string        `db:"path" json:"path"`
	FileName   string        `db:"fileName" json:"fileName"`
	CreatedAt  time.Time     `db:"createdAt" json:"createdAt"`
	ModifiedAt time.Time     `db:"modifiedAt" json:"modifiedAt"`
	FrameRate  float32       `db:"frameRate" json:"frameRate"`
	VideoCodec string        `db:"videoCodec" json:"videoCodec"`
	AudioCodec string        `db:"audioCodec" json:"audioCodec"`
	Width      int           `db:"width" json:"width"`
	Height     int           `db:"height" json:"height"`
	Duration   time.Duration `db:"duration" json:"duration"`
	Size       int64         `db:"size" json:"size"`
	BitRate    int           `db:"bitRate" json:"bitRate"`
}

func (v Video) String() string {
	return fmt.Sprintf(
		"VideoID: %d, Path: %s, FileName: %s, CreatedAt: %s, ModifiedAt: %s, FrameRate: %.2f, VideoCodec: %s, AudioCodec: %s, Width: %d, Height: %d, Duration: %s, Size: %d, BitRate: %d",
		v.VideoID, v.Path, v.FileName, v.CreatedAt.Format(time.RFC3339), v.ModifiedAt.Format(time.RFC3339), v.FrameRate, v.VideoCodec, v.AudioCodec, v.Width, v.Height, v.Duration, v.Size, v.BitRate,
	)
}

func stringJoin(arr []string, separator string) string {
	result := ""
	for i, s := range arr {
		if i > 0 {
			result += separator
		}
		result += s
	}
	return result
}
