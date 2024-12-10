package ffprobe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"time"

	"govdupes/internal/models"
)

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

type FFProbeOutput struct {
	Streams []struct {
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
		Width     int    `json:"width,omitempty"`
		Height    int    `json:"height,omitempty"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
		Size     string `json:"size"`
		BitRate  string `json:"bit_rate"`
	} `json:"format"`
}

func GetVideoInfo(v *models.Video) error {
	log.Printf("Getting info from Video, videoname: %q\n", v.FileName)
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
			if !validVideoCodecs[stream.CodecName] {
				return fmt.Errorf("Invalid video codec: %q", stream.CodecName)
			}
			if stream.Width <= 0 || stream.Height <= 0 {
				return fmt.Errorf("Invalid video dimensions: width=%d, height=%d", stream.Width, stream.Height)
			}
			v.VideoCodec = stream.CodecName
			v.Width = stream.Width
			v.Height = stream.Height
		case "audio":
			if !validAudioCodecs[stream.CodecName] {
				return fmt.Errorf("Invalid audio codec: %q", stream.CodecName)
			}
			v.AudioCodec = stream.CodecName
		default:
			return fmt.Errorf("Unknown codec detected: %q\n", stream.CodecName)
		}
	}

	dur, err := time.ParseDuration(f.Format.Duration + "s")
	if err != nil {
		log.Printf("Error, parsing duration\n")
		return err
	}
	if dur <= 0 {
		return fmt.Errorf("Invalid time.Duration from ffprobe, filename: %q, duration: %v", v.FileName, dur)
	}
	v.Duration = dur

	size, err := strconv.Atoi(f.Format.Size)
	if err != nil {
		return fmt.Errorf("Error converting size to int, filename: %q, size: %q", v.FileName, size)
	}
	if size <= 0 {
		return fmt.Errorf("Invalid size for, filename: %q, size: %d", v.FileName, size)
	}
	v.Size = int64(size)

	if bitrate, err := strconv.Atoi(f.Format.BitRate); err != nil {
		v.BitRate = 0
	} else {
		v.BitRate = bitrate
	}

	return nil
}

func (f *FFProbeOutput) print() {
	log.Printf("Duration: %s seconds\n", f.Format.Duration)
	log.Printf("Size: %q bytes\n", f.Format.Size)
	log.Printf("Bitrate: %q bps\n", f.Format.BitRate)

	for _, stream := range f.Streams {
		switch stream.CodecType {
		case "video":
			log.Printf("Video Codec: %s, Resolution: %dx%d\n",
				stream.CodecName, stream.Width, stream.Height)
		case "audio":
			log.Printf("Audio Codec: %s\n", stream.CodecName)
		default:
			log.Printf("Unknown codec detected: %q\n", stream.CodecName)
		}
	}
}
