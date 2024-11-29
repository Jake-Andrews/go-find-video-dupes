package videoprocessor

import (
	"io"
	"log"
	"os"

	"govdupes/internal/models"

	ffmpeg "github.com/u2takey/ffmpeg-go"
)

type ffmpegWrapper struct {
	logLevel string
}

func NewFFmpegInstance(logLevel string) *ffmpegWrapper {
	if logLevel == "" {
		logLevel = "verbose"
	}
	return &ffmpegWrapper{logLevel: logLevel}
}

func (f *ffmpegWrapper) ScreenshotAtTime(filePath string, scWriter io.Writer, timeStamp string) error {
	log.Printf("Creating a screenshot at: %q, filePath: %q\n", timeStamp, filePath)
	err := ffmpeg.
		Input(filePath, ffmpeg.KwArgs{"ss": timeStamp}). // Place -ss here
		Output("pipe:",
			ffmpeg.KwArgs{
				"vcodec":  "bmp",
				"vframes": 1,
				"format":  "image2",
			}).
		WithOutput(scWriter).
		OverWriteOutput().
		ErrorToStdOut().
		Run()
	if err != nil {
		log.Println("Error ScreenshotAtTime")
		return err
	}
	return nil
}

func (f *ffmpegWrapper) ScreenshotAtTimeSave(filePath string, scWriter io.Writer, timeStamp string, saveToFile bool, outputPath string) error {
	log.Printf("Creating a screenshot at: %q, filePath: %q\n", timeStamp, filePath)

	var fileWriter io.Writer
	if saveToFile {
		file, err := os.Create(outputPath)
		if err != nil {
			log.Printf("Error creating file %q: %v\n", outputPath, err)
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
		OverWriteOutput().
		ErrorToStdOut()

	if saveToFile {
		ffmpegCmd = ffmpegCmd.WithOutput(fileWriter).OverWriteOutput()
	}

	err := ffmpegCmd.Run()
	if err != nil {
		log.Printf("Error in ScreenshotAtTime: %v\n", err)
		return err
	}

	if saveToFile {
		log.Printf("Screenshot saved to file: %q\n", outputPath)
	}

	return nil
}

func (f *ffmpegWrapper) NormalizeVideo(vWriter io.Writer, v *models.Video, kwargs ffmpeg.KwArgs) {
	ffErr := ffmpeg.
		Input(v.Path).
		Filter("scale", ffmpeg.Args{"64:64"}). // Resize to 64x64 pixels
		Filter("fps", ffmpeg.Args{"15"}).      // Set frame rate to 15 fps
		Output("pipe:",
			ffmpeg.KwArgs{
				"pix_fmt":  "yuv444p",    // RGB24 color format
				"vcodec":   "libx264",    // Video codec
				"movflags": "+faststart", // MP4 format optimization
				"an":       "",           // Disable audio
				"f":        "mpegts",
			}).
		WithOutput(vWriter).
		GlobalArgs("-loglevel", "verbose"). // Set verbose logging
		OverWriteOutput().                  // unsure
		ErrorToStdOut().
		Run()
	if ffErr != nil {
		log.Fatalf("Error using ffmpeg to generate normalized video, video: %v, err: %v", v, ffErr)
	}
}
