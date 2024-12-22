package models

type VideoData struct {
	Video      Video       `db:"video"`
	Videohash  Videohash   `db:"videohash"`
	Screenshot Screenshots `db:"screenshot"`
}
