package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type HashType string

const (
	HashTypePHash HashType = "phash"
)

type Videohash struct {
	ID               int64    `db:"id"`
	FKVideohashVideo int64    `db:"FK_videohash_video"`
	HashType         HashType `db:"hashType"`
	HashValue        string   `db:"hashValue"`
	Duration         float32  `db:"duration"`
	Neighbours       IntSlice `db:"neighbours"`
	Bucket           int      `db:"bucket"`
}

// Metadata    Metadata `db:"metadata"`
// type Metadata map[string]interface{}

type IntSlice []int

func (is *IntSlice) Scan(value interface{}) error {
	if value == nil {
		*is = nil
		return nil
	}

	// Expecting the value to be a string (e.g., "[1,2,3]" or "1,2,3")
	strValue, ok := value.(string)
	if !ok {
		return fmt.Errorf("unsupported type: %T", value)
	}

	// Parse the string into []int
	var ints []int
	if err := json.Unmarshal([]byte(strValue), &ints); err != nil {
		// Try parsing as comma-separated values if JSON unmarshalling fails
		parts := strings.Split(strValue, ",")
		for _, part := range parts {
			i, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil {
				return fmt.Errorf("failed to parse neighbours: %w", err)
			}
			ints = append(ints, i)
		}
	}

	*is = ints
	return nil
}
