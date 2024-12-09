package models

type Videohash struct {
	VideohashID int64  `db:"videohashID" json:"videohashID"` // Primary key
	VideoID     int64  `db:"videoID" json:"videoID"`         // Foreign key referencing Video
	Value       string `db:"value" json:"value"`
	HashType    string `db:"hashType" json:"hashType"`
}
