package main

import (
	"log"

	"govdupes/internal/config"
	"govdupes/internal/filesystem"
	phash "govdupes/internal/hash"
	"govdupes/internal/models"
	"govdupes/internal/videoprocessor"
	"govdupes/internal/videoprocessor/ffprobe"
)

var (
	wrongArgsMsg string = "Error, your input must include only one arg which contains the path to the filedirectory to scan."
	logLevel     string = "error"
)

func main() {
	var config config.Config
	config.ParseArgs()
	videos := filesystem.SearchDirs(&config)
	vp := videoprocessor.NewFFmpegInstance(logLevel)
	pHashes := []models.Videohash{}

	for _, v := range *videos {
		err := ffprobe.GetVideoInfo(&v)
		if err != nil {
			log.Fatalf("Error, getting video info for path: %q, err: %v\n", v.Path, err)
		}
		hashVal, err := phash.Create(vp, &v)
		if err != nil {
			log.Printf("Error trying to generate pHash, fileName: %q, err: %v", v.FileName, err)
		}
		log.Println(hashVal)

		pHash := models.Videohash{
			Value:    *hashVal,
			HashType: "pHash",
		}

		pHashes = append(pHashes, pHash)
	}
	for _, h := range pHashes {
		log.Println(h)
	}
}

/*
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
				log.Printf("Error extracting frame at timestamp %.2f: %v\n", timestamp.Seconds(), err)
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
*/
