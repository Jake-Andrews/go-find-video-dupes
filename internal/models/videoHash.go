package models

type HashType string

const (
	HashTypePHash HashType = "phash"
)

type Videohash struct {
	VideohashID int64    `db:"videohashID"` // primary key
	VideoID     int64    `db:"videoID"`     // foreign key
	HashType    HashType `db:"hashType"`
	HashValue   string   `db:"hashValue"`
	Duration    float32  `db:"duration"`
	Neighbours  []int    `db:"neighbours"`
	Bucket      int      `db:"bucket"`
	// Metadata    Metadata `db:"metadata"`
}

type Metadata map[string]interface{}
