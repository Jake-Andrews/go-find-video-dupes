package videoprocessor

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"govdupes/internal/models"

	ffmpeg "github.com/u2takey/ffmpeg-go"
)

type FFmpegWrapper struct {
	logLevel string
}

func NewFFmpegInstance(logLevel string) *FFmpegWrapper {
	if logLevel == "" {
		logLevel = "verbose"
	}
	return &FFmpegWrapper{logLevel: logLevel}
}

func (f *FFmpegWrapper) ScreenshotAtTime(filePath string, scWriter io.Writer, timeStamp string) error {
	width := models.Width
	height := models.Height

	slog.Info("Creating screenshot",
		slog.Int("Width", width),
		slog.Int("Height", height),
		slog.String("Timestamp", timeStamp),
		slog.String("FilePath", filePath))

	err := ffmpeg.
		Input(filePath, ffmpeg.KwArgs{"ss": timeStamp, "hide_banner": "", "loglevel": f.logLevel}).
		Output("pipe:",
			ffmpeg.KwArgs{
				"vcodec":  "bmp",
				"vframes": 1,
				"format":  "image2",
				"vf":      fmt.Sprintf("scale=%d:%d", width, height),
			}).
		WithOutput(scWriter).
		ErrorToStdOut().
		Run()
	if err != nil {
		slog.Error("Error creating screenshot", slog.Any("error", err))
		return err
	}
	return nil
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
