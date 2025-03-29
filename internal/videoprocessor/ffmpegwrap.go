package videoprocessor

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"govdupes/internal/config"
	"govdupes/internal/models"

	ffmpeg "github.com/u2takey/ffmpeg-go"
)

/*
'-ss'
 - if used before -i guarantees a keyframe before the specified time
 - if used after -i does not guarantee a keyframe and decodes the entire stream

eq(t\,...) in select
 - frame-accurate
 - decodes entire stream
 - can extract multiple frames in one ffmpeg call
 - doesn't guarantee a keyframe

:/
Obviously non-keyframes are not usable to compare two videos with different bytes.
And since keyframes are not stable if a video is reencoded it's not a guarnteed way to detect duplicates.
*/

var ErrBMPDecode = errors.New("failed trying to decode the BMP images generated from FFmpeg")

type FFmpegWrapper struct {
	silent bool
}

func NewFFmpegInstance(cfg *config.Config) *FFmpegWrapper {
	return &FFmpegWrapper{silent: cfg.SilentFFmpeg}
}

func (f *FFmpegWrapper) ScreenshotAtTime(filePath string, scWriter io.Writer, timeStamp string) error {
	width := models.Width
	height := models.Height

	/*
		slog.Info("Creating screenshot",
			slog.Int("Width", width),
			slog.Int("Height", height),
			slog.String("Timestamp", timeStamp),
			slog.String("FilePath", filePath))
	*/

	err := ffmpeg.
		Input(filePath, ffmpeg.KwArgs{"ss": timeStamp, "hide_banner": "", "nostats": "", "nostdin": ""}).
		Output("pipe:",
			ffmpeg.KwArgs{
				"vcodec":  "bmp",
				"vframes": 1,
				"format":  "image2",
				"vf":      fmt.Sprintf("scale=%d:%d", width, height),
			}).
		WithOutput(scWriter).
		Silent(f.silent).
		Run()
	if err != nil {
		slog.Error("Error creating screenshot", slog.Any("error", err))
		return err
	}
	return nil
}

func (f *FFmpegWrapper) ScreenshotsAtTimestamps(filePath string, scWriter io.Writer, timeStamps []string) ([][]byte, error) {
	var buf bytes.Buffer

	// build the select expression
	selectExpr, err := buildSelectExpr(timeStamps)
	if err != nil {
		return nil, err
	}

	// "vf" filter: select='eq(t,...)'+...,scale=...
	vf := fmt.Sprintf("select='%s',scale=%d:%d", selectExpr, models.Width, models.Height)

	err = ffmpeg.
		Input(filePath).
		Output("pipe:",
			ffmpeg.KwArgs{
				"vcodec":  "bmp",
				"vframes": len(timeStamps),
				"format":  "image2pipe",
				"vf":      vf,
				"pix_fmt": "rgb24",
				"vsync":   "vfr",
			},
		).
		WithOutput(&buf).
		Run()
	if err != nil {
		slog.Error("Error creating screenshot", slog.Any("error", err))
		return nil, err
	}

	// BMP decoding logic
	var screenshots [][]byte
	bmpData := buf.Bytes()
	offset := 0

	for range timeStamps {
		if len(bmpData) < 2 {
			return nil, fmt.Errorf("invalid BMP data: missing BMP header, bmpData size < 2")
		}
		if offset+6 > len(bmpData) {
			return nil, ErrBMPDecode
		}
		if bmpData[offset] != 'B' || bmpData[offset+1] != 'M' {
			return nil, ErrBMPDecode
		}

		// little-endian size
		b := bmpData[offset+2 : offset+6]
		size := int(b[0]) | int(b[1])<<8 | int(b[2])<<16 | int(b[3])<<24

		if offset+size > len(bmpData) {
			return nil, ErrBMPDecode
		}

		screenshot := bmpData[offset : offset+size]
		screenshots = append(screenshots, screenshot)

		offset += size
	}

	return screenshots, nil
}

// expects string to be something like "00:00:05.000"
func buildSelectExpr(timeStamps []string) (string, error) {
	parts := make([]string, 0, len(timeStamps))
	for _, ts := range timeStamps {
		dur, err := parseColonTimestamp(ts)
		if err != nil {
			return "", err
		}
		parts = append(parts, fmt.Sprintf("gte(t,%.3f)", dur.Seconds()))
	}
	// join them with '+' => gte(t,0.000)+gte(t,1.000)+...
	return strings.Join(parts, "+"), nil
}

// parses strings like "00:00:05.123" into a time.Duration
func parseColonTimestamp(ts string) (time.Duration, error) {
	var h, m, s, ms int
	_, err := fmt.Sscanf(ts, "%02d:%02d:%02d.%03d", &h, &m, &s, &ms)
	if err != nil {
		return 0, fmt.Errorf("parse error for %q: %w", ts, err)
	}
	// Convert parsed fields to a duration
	return time.Duration(h)*time.Hour +
		time.Duration(m)*time.Minute +
		time.Duration(s)*time.Second +
		time.Duration(ms)*time.Millisecond, nil
}

func (f *FFmpegWrapper) ScreenshotAtTimeSave(filePath string, scWriter io.Writer, timeStamp string, saveToFile bool, outputPath string) error {
	slog.Info("Creating screenshot with save option",
		slog.String("Timestamp", timeStamp),
		slog.String("FilePath", filePath),
		slog.Bool("SaveToFile", saveToFile),
		slog.String("OutputPath", outputPath))

	var fileWriter io.Writer
	if saveToFile {
		file, err := os.Create(outputPath)
		if err != nil {
			slog.Error("Error creating output file", slog.String("OutputPath", outputPath), slog.Any("error", err))
			return err
		}
		defer file.Close()
		fileWriter = file
	}

	ffmpegCmd := ffmpeg.Input(filePath, ffmpeg.KwArgs{"ss": timeStamp}).
		Output("pipe:",
			ffmpeg.KwArgs{
				"vcodec":  "bmp",
				"vframes": 1,
				"format":  "image2",
			}).
		WithOutput(scWriter).
		ErrorToStdOut()

	if saveToFile {
		ffmpegCmd = ffmpegCmd.WithOutput(fileWriter).OverWriteOutput()
	}

	err := ffmpegCmd.Run()
	if err != nil {
		slog.Error("Error creating screenshot with save", slog.Any("error", err))
		return err
	}

	if saveToFile {
		slog.Info("Screenshot saved to file", slog.String("OutputPath", outputPath))
	}

	return nil
}

func (f *FFmpegWrapper) NormalizeVideo(vWriter io.Writer, v *models.Video, kwargs ffmpeg.KwArgs) {
	slog.Info("Normalizing video", slog.String("Path", v.Path))
	ffErr := ffmpeg.
		Input(v.Path).
		Filter("scale", ffmpeg.Args{"64:64"}).
		Filter("fps", ffmpeg.Args{"15"}).
		Output("pipe:",
			ffmpeg.KwArgs{
				"pix_fmt":  "yuv444p",
				"vcodec":   "libx264",
				"movflags": "+faststart",
				"an":       "",
				"f":        "mpegts",
			}).
		WithOutput(vWriter).
		GlobalArgs("-loglevel", "verbose").
		ErrorToStdOut().
		Run()
	if ffErr != nil {
		slog.Error("Error normalizing video", slog.Any("error", ffErr), slog.Any("video", v))
	}
}
