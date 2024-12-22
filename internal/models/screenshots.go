package models

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"

	"golang.org/x/image/bmp"
)

type Screenshots struct {
	ID                    int64         `db:"id" json:"id"`
	Screenshots           []image.Image `json:"-"`
	FKScreenshotVideohash int64         `db:"FK_screenshot_videohash" json:"FK_screenshot_videohash"`
}

// EncodeImages encodes images to Base64 for database storage.
func (s *Screenshots) EncodeImages() ([]string, error) {
	var encoded []string
	for _, img := range s.Screenshots {
		var buf bytes.Buffer
		err := bmp.Encode(&buf, img)
		if err != nil {
			return nil, fmt.Errorf("error encoding image to BMP: %w", err)
		}
		encoded = append(encoded, base64.StdEncoding.EncodeToString(buf.Bytes()))
	}
	return encoded, nil
}

// DecodeImages decodes Base64 strings to images.
func (s *Screenshots) DecodeImages(base64Strings []string) error {
	for _, b64 := range base64Strings {
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return fmt.Errorf("error decoding Base64: %w", err)
		}
		img, err := bmp.Decode(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("error decoding BMP: %w", err)
		}
		s.Screenshots = append(s.Screenshots, img)
	}
	return nil
}

// Metadata    Metadata `db:"metadata"`
// type Metadata map[string]interface{}
