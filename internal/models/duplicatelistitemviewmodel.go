package models

type DuplicateListItemViewModel struct {
	IsColumnsHeader bool
	IsGroupHeader   bool
	GroupIndex      int
	HeaderText      string
	VideoData       *VideoData
	Selected        bool
	Hidden          bool
}
