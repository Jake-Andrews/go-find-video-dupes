package models

type Screenshots struct {
	ScreenshotID int64    `db:"screenshotID" json:"screenshotID"`
	Screenshots  []string `db:"screenshots" json:"screenshots"`
	VideohashID  int64    `db:"videohashID"` // foreign key
}

// Metadata    Metadata `db:"metadata"`

// type Metadata map[string]interface{}
