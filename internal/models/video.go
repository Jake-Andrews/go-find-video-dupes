package models

import (
	"fmt"
	"time"
)

type Video struct {
	ID             int64     `db:"id" json:"id"`
	Path           string    `db:"path" json:"path"`
	FileName       string    `db:"fileName" json:"fileName"`
	CreatedAt      time.Time `db:"createdAt" json:"createdAt"`
	ModifiedAt     time.Time `db:"modifiedAt" json:"modifiedAt"`
	VideoCodec     string    `db:"videoCodec" json:"videoCodec"`
	AudioCodec     string    `db:"audioCodec" json:"audioCodec"`
	Width          int       `db:"width" json:"width"`
	Height         int       `db:"height" json:"height"`
	Duration       float32   `db:"duration" json:"duration"`
	Size           int64     `db:"size" json:"size"`
	BitRate        int       `db:"bitRate" json:"bitRate"`
	NumHardLinks   uint64    `db:"numHardLinks" json:"numHardLinks"`
	SymbolicLink   string    `db:"symbolicLink" json:"symbolicLink"`
	IsSymbolicLink bool      `db:"isSymbolicLink" json:"isSymbolicLink"`
	IsHardLink     bool      `db:"isHardLink" json:"isHardLink"`
	Inode          uint64    `db:"inode" json:"inode"`
	Device         uint64    `db:"device" json:"device"`
	AvgFrameRate   float32   `db:"avgFrameRate" json:"avgFrameRate"`
	SampleRateAvg  int       `db:"sampleRateAvg" json:"sampleRateAvg"`
	Corrupted      bool
}

func (v Video) String() string {
	return fmt.Sprintf(
		`ID: %d, Path: %s, FileName: %s, CreatedAt: %s, ModifiedAt: %s, AvgFrameRate: %.2f, VideoCodec: %s, AudioCodec: %s, Width: %d, Height: %d, Duration: %.2f, Size: %d, BitRate: %d, 
	NumHardLinks: %d, SymbolicLink: %s, IsSymbolicLink: %t, IsHardLink: %t, Inode: %d, Device: %d, Corrupted: %t`,
		v.ID, v.Path, v.FileName, v.CreatedAt.Format(time.RFC3339), v.ModifiedAt.Format(time.RFC3339), v.AvgFrameRate, v.VideoCodec, v.AudioCodec, v.Width, v.Height, v.Duration, v.Size, v.BitRate,
		v.NumHardLinks, v.SymbolicLink, v.IsSymbolicLink, v.IsHardLink, v.Inode, v.Device, v.Corrupted)
}
