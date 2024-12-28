package ffprobe

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"

	"govdupes/internal/models"
)

/*
var validVideoCodecs = map[string]bool{
	"h264": true, "hevc": true, "mpeg4": true, "vp8": true, "vp9": true,
	"av1": true, "theora": true, "mpeg2video": true, "vc1": true, "wmv3": true,
	"h263": true, "prores": true, "dnxhd": true, "flv1": true, "rv40": true,
	"rawvideo": true,
}

var validAudioCodecs = map[string]bool{
	"aac": true, "mp3": true, "ac3": true, "eac3": true, "opus": true,
	"vorbis": true, "flac": true, "alac": true, "pcm_s16le": true,
	"pcm_s24le": true, "pcm_u8": true, "amr_nb": true, "amr_wb": true,
	"wavpack": true, "speex": true, "wma": true, "dts": true, "truehd": true,
	"aptx": true, "g722": true, "g729": true,
}
*/

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

// Custom type for parsing the fraction
type FractionFloat32 float32

// UnmarshalJSON implementation for FractionFloat64
// avg_frame_rate format: #/#
// ex: 30/1
// avg_frame_rate format: #/# or 0/0
func (f *FractionFloat32) UnmarshalJSON(data []byte) error {
	// Trim the quotes around the string (if any)
	raw := strings.Trim(string(data), `"`)

	// Split the string by the slash
	parts := strings.Split(raw, "/")
	if len(parts) != 2 {
		return errors.New("invalid fraction format")
	}

	// Parse numerator and denominator
	numerator, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return fmt.Errorf("invalid numerator: %v", err)
	}
	denominator, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return fmt.Errorf("invalid denominator: %v", err)
	}

	// Handle edge case for 0/0
	if numerator == 0 && denominator == 0 {
		*f = 0 // Default value for undefined fraction
		return nil
	}

	// Check for division by zero
	if denominator == 0 {
		return errors.New("division by zero")
	}

	// Calculate the float64 value
	*f = FractionFloat32(numerator / denominator)
	return nil
}

func GetVideoInfo(v *models.Video) error {
	log.Printf("Getting info from Video, videoname: %q\n", v.FileName)
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

	err = setVideo(&ffProbeOutput, v)
	if err != nil {
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
		return fmt.Errorf("error converting size to int, filename: %q, size: %q", v.FileName, size)
	}
	if size <= 0 {
		return fmt.Errorf("invalid size for, filename: %q, size: %d", v.FileName, size)
	}
	v.Size = int64(size)

	if bitrate, err := strconv.Atoi(f.Format.BitRate); err != nil {
		v.BitRate = 0
	} else {
		v.BitRate = bitrate
	}

	dur, err := strconv.ParseFloat(f.Format.Duration, 64)
	if err != nil {
		return fmt.Errorf("error parsing duration, error: %v", err)
	}
	if dur <= 0 {
		return fmt.Errorf("invalid time.Duration from ffprobe, filename: %q, duration: %v", v.FileName, dur)
	}
	v.Duration = float32(dur)

	return nil
}

func (f *FFProbeOutput) print() {
	log.Println("ffprobe:")
	log.Printf("Duration: %s seconds\n", f.Format.Duration)
	log.Printf("Size: %q bytes\n", f.Format.Size)
	log.Printf("Bitrate: %q bps\n", f.Format.BitRate)

	for _, stream := range f.Streams {
		switch stream.CodecType {
		case "video":
			log.Printf("Video Codec: %s, Resolution: %dx%d, AvgFrameRate: %.2f",
				stream.CodecName, stream.Width, stream.Height, stream.AvgFrameRate)
		case "audio":
			log.Printf("Audio Codec: %s, SampleRateAvg: %d", stream.CodecName, stream.SampleRateAvg)
		default:
			log.Printf("Unknown codec detected: %q\n", stream.CodecName)
		}
	}
}
