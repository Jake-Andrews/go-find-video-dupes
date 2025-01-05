package ffprobe

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"

	"govdupes/internal/models"
)

type FFProbeOutput struct {
	Streams []struct {
		CodecType     string          `json:"codec_type"`
		CodecName     string          `json:"codec_name"`
		Width         int             `json:"width"`
		Height        int             `json:"height"`
		SampleRateAvg int             `json:"sample_rate_avg"`
		AvgFrameRate  FractionFloat32 `json:"avg_frame_rate"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
		Size     string `json:"size"`
		BitRate  string `json:"bit_rate"`
	} `json:"format"`
}

type FractionFloat32 float32

func (f *FractionFloat32) UnmarshalJSON(data []byte) error {
	raw := strings.Trim(string(data), `"`)
	parts := strings.Split(raw, "/")
	if len(parts) != 2 {
		return errors.New("invalid fraction format")
	}
	numerator, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return fmt.Errorf("invalid numerator: %v", err)
	}
	denominator, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return fmt.Errorf("invalid denominator: %v", err)
	}
	if numerator == 0 && denominator == 0 {
		*f = 0
		return nil
	}
	if denominator == 0 {
		return errors.New("division by zero")
	}
	*f = FractionFloat32(numerator / denominator)
	return nil
}

func GetVideoInfo(v *models.Video) error {
	slog.Info("Getting video info", slog.String("filename", v.FileName))
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration,size,bit_rate",
		"-show_entries", "stream=codec_type,codec_name,width,height,sample_rate,avg_frame_rate",
		"-of", "json",
		v.Path)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("ffprobe error: %v, stderr: %s", err, stderr.String())
	}

	var ffProbeOutput FFProbeOutput
	if err := json.Unmarshal(out.Bytes(), &ffProbeOutput); err != nil {
		return fmt.Errorf("json unmarshal error: %v", err)
	}

	if err := setVideo(&ffProbeOutput, v); err != nil {
		return err
	}
	return nil
}

func setVideo(f *FFProbeOutput, v *models.Video) error {
	for _, stream := range f.Streams {
		switch stream.CodecType {
		case "video":
			if stream.Width <= 0 || stream.Height <= 0 {
				return fmt.Errorf("invalid video dimensions: width=%d, height=%d", stream.Width, stream.Height)
			}
			v.VideoCodec = stream.CodecName
			v.Width = stream.Width
			v.Height = stream.Height
			v.AvgFrameRate = float32(stream.AvgFrameRate)
		case "audio":
			v.AudioCodec = stream.CodecName
			v.SampleRateAvg = stream.SampleRateAvg
		default:
			return fmt.Errorf("unknown codec detected: %q", stream.CodecName)
		}
	}

	size, err := strconv.Atoi(f.Format.Size)
	if err != nil {
		return fmt.Errorf("error converting size to int, filename: %q, size: %q", v.FileName, f.Format.Size)
	}
	if size <= 0 {
		return fmt.Errorf("invalid size for filename: %q, size: %d", v.FileName, size)
	}
	v.Size = int64(size)

	if bitrate, err := strconv.Atoi(f.Format.BitRate); err != nil {
		v.BitRate = 0
	} else {
		v.BitRate = bitrate
	}
	dur, err := strconv.ParseFloat(f.Format.Duration, 64)
	if err != nil {
		return fmt.Errorf("error parsing duration: %v", err)
	}
	if dur <= 0 {
		return fmt.Errorf("invalid duration from ffprobe, filename: %q, duration: %v", v.FileName, dur)
	}
	v.Duration = float32(dur)
	return nil
}

func (f *FFProbeOutput) print() {
	slog.Info("ffprobe output",
		slog.String("Duration", f.Format.Duration),
		slog.String("Size", f.Format.Size),
		slog.String("BitRate", f.Format.BitRate))
	for _, stream := range f.Streams {
		switch stream.CodecType {
		case "video":
			slog.Info("Video stream",
				slog.String("Codec", stream.CodecName),
				slog.Int("Width", stream.Width),
				slog.Int("Height", stream.Height),
				slog.Float64("AvgFrameRate", float64(stream.AvgFrameRate)))
		case "audio":
			slog.Info("Audio stream",
				slog.String("Codec", stream.CodecName),
				slog.Int("SampleRateAvg", stream.SampleRateAvg))
		default:
			slog.Warn("Unknown codec detected", slog.String("Codec", stream.CodecName))
		}
	}
}

