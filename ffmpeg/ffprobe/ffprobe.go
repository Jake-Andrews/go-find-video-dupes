package ffprobe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"time"

	"govdupes/models"
)

type FFProbeOutput struct {
	Streams []struct {
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
		Width     int    `json:"width,omitempty"`
		Height    int    `json:"height,omitempty"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
		Size     int    `json:"size"`
		BitRate  int    `json:"bit_rate"`
	} `json:"format"`
}

func GetVideoInfo(v *models.Video) error {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration,size,bit_rate",
		"-show_entries", "stream=codec_type,codec_name,width,height",
		"-of", "json",
		v.Path)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	// Run the command
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("ffprobe error: %v, stderr: %s", err, stderr.String())
	}

	// Parse the JSON output
	var ffProbeOutput FFProbeOutput
	if err := json.Unmarshal(out.Bytes(), &ffProbeOutput); err != nil {
		return fmt.Errorf("json unmarshal error: %v", err)
	}

	setVideo(&ffProbeOutput, v)
	return nil
}

func setVideo(f *FFProbeOutput, v *models.Video) {
	for _, stream := range f.Streams {
		switch stream.CodecType {
		case "video":
			v.VideoCodec = stream.CodecName
			v.Width = stream.Width
			v.Height = stream.Height
		case "audio":
			v.AudioCodec = stream.CodecName
		default:
			log.Printf("Unknown codec detected: %q", stream.CodecName)
		}
	}
	dur, err := time.ParseDuration(f.Format.Duration + "s")
	if err != nil {
		log.Printf("Error, parsing duration, err: %v", err)
	}
	v.Duration = dur
	v.Size = int64(f.Format.Size)
	v.BitRate = f.Format.BitRate
}

func (f *FFProbeOutput) print() {
	log.Printf("Duration: %s seconds\n", f.Format.Duration)
	log.Printf("Size: %d bytes\n", f.Format.Size)
	log.Printf("Bitrate: %d bps\n", f.Format.BitRate)

	for _, stream := range f.Streams {
		switch stream.CodecType {
		case "video":
			log.Printf("Video Codec: %s, Resolution: %dx%d\n",
				stream.CodecName, stream.Width, stream.Height)
		case "audio":
			log.Printf("Audio Codec: %s\n", stream.CodecName)
		default:
			log.Printf("Unknown codec detected: %q", stream.CodecName)
		}
	}
}
