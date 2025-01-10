package models

// row in the UI
type DuplicateListItemViewModel struct {
	IsColumnsHeader bool
	IsGroupHeader   bool
	GroupIndex      int
	HeaderText      string
	VideoData       *VideoData
	Selected        bool
	Hidden          bool
}
