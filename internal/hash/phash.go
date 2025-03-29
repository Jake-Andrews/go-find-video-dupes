package hash

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"log/slog"
	"math"
	"strings"

	"govdupes/internal/models"
	"govdupes/internal/videoprocessor"

	"github.com/corona10/goimagehash"
	"golang.org/x/image/bmp"
)

func Create(vp *videoprocessor.FFmpegWrapper, v *models.Video, method string) (*models.Videohash, *models.Screenshots, error) {
	switch method {
	case "SlowPhash":
		return createSlowPhash(vp, v)
	case "FastPhash":
		return createFastPhash(vp, v)
	default:
		return nil, nil, fmt.Errorf("unknown detection method: %s", method)
	}
}

func createFastPhash(vp *videoprocessor.FFmpegWrapper, v *models.Video) (*models.Videohash, *models.Screenshots, error) {
	timestamps := createTimeStamps(v.Duration, models.NumImages)
	images, err := createScreenshots(vp, timestamps, v)
	if err != nil {
		slog.Error("Error creating screenshots", slog.Any("error", err))
		return nil, nil, err
	}

	screenshots := &models.Screenshots{Screenshots: images}

	image, err := createCollage(images)
	if err != nil {
		slog.Error("Error creating collage", slog.Any("error", err))
	}

	hash, err := goimagehash.PerceptionHash(image)
	if err != nil {
		slog.Error("Error creating phash", slog.Any("error", err))
	}
	h := hash.ToString()
	slog.Info("File has pHash", slog.String("file", v.FileName), slog.String("pHash", hash.ToString()))

	pHash := createPhash(v, h)
	slog.Debug("Created pHash", slog.Any("pHash", *pHash))

	return pHash, screenshots, nil
}

func createSlowPhash(vp *videoprocessor.FFmpegWrapper, v *models.Video) (*models.Videohash, *models.Screenshots, error) {
	numFrames := int(math.Floor(float64(v.Duration)))
	if numFrames == 0 {
		return nil, nil, fmt.Errorf("error numFrames == 0 for slowPhash")
	}

	timestamps := createTimeStamps(v.Duration, numFrames)
	images, err := createScreenshots(vp, timestamps, v)
	if err != nil {
		slog.Error("Error creating screenshots", slog.Any("error", err))
		return nil, nil, err
	}

	var thumbIndex int
	if len(images) < 5 {
		thumbIndex = 0
	} else {
		thumbIndex = 5
	}
	screenshots := &models.Screenshots{Screenshots: []image.Image{images[thumbIndex]}}

	var builder strings.Builder
	for i, img := range images {
		/*
			            // troubleshoot saving images to fs
						outputPath := fmt.Sprintf("test%d.bmp", i)
						file, err := os.Create(outputPath)
						if err != nil {
							return nil, nil, fmt.Errorf("error creating file for image %d: %w", i, err)
						}
						defer file.Close()

						// pHash (it handles grayscale etc...)
						if err := bmp.Encode(file, img); err != nil {
							return nil, nil, fmt.Errorf("error writing BMP for image %d: %w", i, err)
						}
		*/

		hash, hashErr := goimagehash.PerceptionHash(img)
		if hashErr != nil {
			slog.Warn("SlowPhash: skipping frame can't compute pHash", slog.Int("frameIndex", i), slog.Any("error", hashErr))
			continue
		}

		// skip "p: " prefix in hash and append
		builder.WriteString(hash.ToString()[2:])
	}

	combinedHash := builder.String()
	if len(combinedHash) == 0 {
		return nil, nil, fmt.Errorf("SlowPhash: no valid pHashes generated")
	}

	slog.Info("SlowPhash: computed multiple pHashes",
		slog.String("file", v.FileName),
		slog.Int("framesUsed", numFrames),
		slog.String("combinedHash", combinedHash),
	)

	pHash := &models.Videohash{
		ID:        v.ID,
		HashType:  "pHash",
		HashValue: combinedHash,
		Duration:  v.Duration,
		Bucket:    -1,
	}

	return pHash, screenshots, nil
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
	for i := range numTimestamps {
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
		err := vp.ScreenshotAtTime(v.Path, &buf, t)
		if err != nil {
			return nil, fmt.Errorf("skipping file, cannot generate screenshots, err: %q", err)
		}

		img, err := bmp.Decode(&buf)
		if err != nil {
			slog.Error("Error decoding image", slog.Any("error", err))
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
		HashType:  "pHash", //**change to num that refers to a row in the hashtype table in the DB**
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

/*
func createSlowPhash(vp *videoprocessor.FFmpegWrapper, v *models.Video) (*models.Videohash, *models.Screenshots, error) {
	numFrames := int(math.Floor(float64(v.Duration)))

	// build a list of timestamps [0s, 1s, 2s, ...] up to numFrames
	timeStamps := make([]string, 0, numFrames)
	for i := range numFrames {
		// convert `i` seconds to "HH:MM:SS.mmm"
		timeStr := durationToFFmpegTimestamp(float32(i))
		timeStamps = append(timeStamps, timeStr)
	}

	if len(timeStamps) == 0 {
		return nil, nil, fmt.Errorf("no timestamps generated for slowPhash")
	}

	// returns at least one screenshot or errors
	var buf bytes.Buffer
	bmpBytes, err := vp.ScreenshotsAtTimestamps(v.Path, &buf, timeStamps)
	if err != nil {
		slog.Error("SlowPhash: error retrieving frames", slog.Any("error", err))
		return nil, nil, err
	}

	var thumbIndex int
	if len(bmpBytes) < 5 {
		thumbIndex = 0
	} else {
		thumbIndex = 5
	}
	slog.Info("thumbIndex", "value:", thumbIndex)

	var screenshot *models.Screenshots
	var builder strings.Builder
	for i, bmpData := range bmpBytes {
		img, decErr := bmpDecodeBytes(bmpData)
		if decErr != nil {
			slog.Warn("SlowPhash: skipping broken frame", slog.Int("frameIndex", i), slog.Any("error", decErr))
			continue
		}
		// store the decoded frame so we can display in UI
		if i == thumbIndex {
			screenshot = &models.Screenshots{Screenshots: []image.Image{img}}
		}
		os.WriteFile(fmt.Sprintf("test%d.bmp", i), bmpData, os.ModePerm)

		// compute pHash (it handles grayscale etc...)
		hash, hashErr := goimagehash.PerceptionHash(img)
		if hashErr != nil {
			slog.Warn("SlowPhash: skipping frame can't compute pHash", slog.Int("frameIndex", i), slog.Any("error", hashErr))
			continue
		}

		// skip "p: "
		builder.WriteString(hash.ToString()[2:])
	}

	combinedHash := builder.String()

	if len(combinedHash) == 0 {
		return nil, nil, fmt.Errorf("SlowPhash: no valid pHashes generated")
	}

		slog.Info("SlowPhash: computed multiple pHashes",
			slog.String("file", v.FileName),
			slog.Int("framesUsed", len(bmpBytes)),
			slog.String("combinedHash", combinedHash),
		)

	pHash := &models.Videohash{
		ID:        v.ID,
		HashType:  "pHash",
		HashValue: combinedHash,
		Duration:  v.Duration,
		Bucket:    -1,
	}

	return pHash, screenshot, nil
}

func bmpDecodeBytes(bmpData []byte) (image.Image, error) {
	buf := bytes.NewReader(bmpData)
	img, err := bmp.Decode(buf)
	if err != nil {
		return nil, fmt.Errorf("bmp decode failed: %w", err)
	}
	return img, nil
}

*/
