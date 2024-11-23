package main

import (
	"fmt"
	"image/png"
	"io"
	"log"
	"strconv"
	"sync"
	"time"

	"govdupes/config"
	"govdupes/ffmpeg/ffprobe"
	"govdupes/filesystem"

	"github.com/corona10/goimagehash"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

var wrongArgsMsg string = "Error, your input must include only one arg which contains the path to the filedirectory to scan."

func main() {
	var config config.Config
	config.ParseArgs()

	videos := filesystem.SearchDirs(&config)

	for _, v := range *videos {
		videoReader, videoWriter := io.Pipe()
		go func() {
			defer videoWriter.Close()
			ffErr := ffmpeg.
				Input(v.Path).
				Filter("scale", ffmpeg.Args{"64:64"}). // Resize to 64x64 pixels
				Filter("fps", ffmpeg.Args{"15"}).      // Set frame rate to 15 fps
				Output("pipe:",
					ffmpeg.KwArgs{
						"pix_fmt":  "rgb24",      // RGB24 color format
						"vcodec":   "libx264",    // Video codec
						"movflags": "+faststart", // MP4 format optimization
						"an":       "",           // Disable audio
						"f":        "mpegts",
					}).
				WithOutput(videoWriter).
				GlobalArgs("-loglevel", "verbose"). // Set verbose logging
				OverWriteOutput().                  // unsure
				ErrorToStdOut().
				Run()
			if ffErr != nil {
				log.Fatalf("Error using ffmpeg to generate normalized video, video: %v, err: %v", v, ffErr)
			}
		}()

		err := ffprobe.GetVideoInfo(&v)
		if err != nil {
			log.Fatalf("Error, getting video info for path: %q, err: %v", v.Path, err)
		}

		for frameIdx := 0; frameIdx < int(v.Duration); frameIdx++ {
			screenshotReader, screenshotWriter := io.Pipe()
			frameStr := strconv.Itoa(frameIdx)

			go func() {
				defer screenshotWriter.Close()
				frameErr := ffmpeg.
					Input("pipe:").
					Output("pipe:",
						ffmpeg.KwArgs{
							"vf":      "select=eq(n\\," + frameStr + ")", // Correct string concatenation
							"vframes": "1",
							"f":       "image2",
						},
					).
					WithInput(videoReader).
					WithOutput(screenshotWriter).
					OverWriteOutput().
					ErrorToStdOut().
					Run()
				if frameErr != nil {
					log.Printf("Error extracting frame, Frame: %d, Error: %v", frameIdx, frameErr)
				}
			}()
			vrErr := videoReader.Close()
			if vrErr != nil {
				log.Printf("Error closing video reader, Error: %v", vrErr)
			}

			img, err := png.Decode(screenshotReader)
			if err != nil {
				log.Printf("Error decoding image, err: %v", err)
				continue
			}
			srErr := screenshotReader.Close()
			if srErr != nil {
				log.Printf("Error closing screenshot reader, Error: %v", srErr)
			}

			hash, err := goimagehash.PerceptionHash(img)
			if err != nil {
				log.Printf("Error generating perceptual hash, err: %v", err)
			}
			log.Println(hash.ToString())
		}
	}
}

func extractFrames(videoReader io.Reader, timestamps []time.Duration) ([]io.Reader, error) {
	var wg sync.WaitGroup
	screenshots := make([]io.Reader, len(timestamps))
	errors := make(chan error, len(timestamps))

	for i, timestamp := range timestamps {
		wg.Add(1)
		screenshotReader, screenshotWriter := io.Pipe()
		screenshots[i] = screenshotReader

		go func(i int, timestamp time.Duration) {
			defer wg.Done()
			defer screenshotWriter.Close()

			// Use FFmpeg to extract the frame at the specific timestamp
			err := ffmpeg.
				Input("pipe:").
				Output("pipe:", ffmpeg.KwArgs{
					"ss":      fmt.Sprintf("%.2f", timestamp.Seconds()),
					"vframes": "1",
					"f":       "image2",
				}).
				WithInput(videoReader).
				WithOutput(screenshotWriter).
				OverWriteOutput().
				ErrorToStdOut().
				Run()
			if err != nil {
				log.Printf("Error extracting frame at timestamp %.2f: %v", timestamp.Seconds(), err)
				errors <- fmt.Errorf("frame %d: %w", i, err)
			}
		}(i, timestamp)
	}

	// Wait for all Goroutines to finish
	wg.Wait()
	close(errors)

	// Check if there were any errors
	if len(errors) > 0 {
		for err := range errors {
			log.Println(err)
		}
		return nil, fmt.Errorf("failed to extract some frames")
	}

	return screenshots, nil
}

func generateTimestamps(duration time.Duration, fps int) []time.Duration {
	var timestamps []time.Duration
	for i := 0; i < int(duration.Seconds())*fps; i += fps {
		timestamps = append(timestamps, time.Duration(i)*time.Second)
	}
	return timestamps
}
