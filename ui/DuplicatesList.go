package ui

import (
	"fmt"
	"log"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"govdupes/internal/models"
)

// duplicateListItem represents:
//   - The single columns header row (shown once, if any videos exist)
//   - Group header row
//   - Video row
type duplicateListItem struct {
	IsColumnsHeader bool // Only one instance at the top
	IsGroupHeader   bool
	GroupIndex      int

	HeaderText string // used if IsGroupHeader == true
	VideoData  *models.VideoData
	Selected   bool
}

// DuplicatesList is a custom widget that displays videos
// in groups of duplicates, with a single header row at the top.
type DuplicatesList struct {
	widget.BaseWidget

	mutex sync.RWMutex
	items []duplicateListItem

	list        *widget.List
	OnRowTapped func(itemID int, selected bool)
}

// NewDuplicatesList creates and returns our custom DuplicatesList.
func NewDuplicatesList(videoData [][]*models.VideoData) *DuplicatesList {
	log.Println("Creating DuplicatesList")

	dl := &DuplicatesList{}
	dl.ExtendBaseWidget(dl)

	dl.OnRowTapped = func(itemID int, selected bool) {
		dl.handleRowTapped(itemID, selected)
	}

	dl.list = widget.NewList(
		func() int {
			dl.mutex.RLock()
			defer dl.mutex.RUnlock()
			return len(dl.items)
		},
		func() fyne.CanvasObject {
			return NewDuplicatesListRow(dl.OnRowTapped)
		},
		func(itemID widget.ListItemID, co fyne.CanvasObject) {
			dl.updateListRow(itemID, co)
		},
	)

	dl.SetData(videoData)
	return dl
}

// SetData flattens the groups into a single list.
//
//  1. If there is at least one non-empty group, add the columns header row (once).
//  2. For each non-empty group:
//     a) Group header item
//     b) N video items
func (dl *DuplicatesList) SetData(videoData [][]*models.VideoData) {
	dl.mutex.Lock()
	defer dl.mutex.Unlock()

	dl.items = nil

	// Check if we have at least one non-empty group
	hasAnyVideos := false
	for _, group := range videoData {
		if len(group) > 0 {
			hasAnyVideos = true
			break
		}
	}
	// Add the columns header row once if we have any videos
	if hasAnyVideos {
		dl.items = append(dl.items, duplicateListItem{
			IsColumnsHeader: true,
		})
	}

	// Now add group headers and video items
	for i, group := range videoData {
		if len(group) == 0 {
			continue
		}

		dl.items = append(dl.items, duplicateListItem{
			IsGroupHeader: true,
			GroupIndex:    i,
			HeaderText:    fmt.Sprintf("Group %d (Total %d duplicates)", i+1, len(group)),
		})

		for _, vd := range group {
			dl.items = append(dl.items, duplicateListItem{
				GroupIndex: i,
				VideoData:  vd,
			})
		}
	}
}

// CreateRenderer is part of Fyne’s custom widget interface.
// We only need to render the underlying “list” widget.
func (dl *DuplicatesList) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(dl.list)
}

func (dl *DuplicatesList) updateListRow(itemID widget.ListItemID, co fyne.CanvasObject) {
	dl.mutex.RLock()
	defer dl.mutex.RUnlock()

	if itemID < 0 || itemID >= len(dl.items) {
		log.Printf("Item ID %d out of bounds", itemID)
		return
	}

	row, ok := co.(*DuplicatesListRow)
	if !ok {
		log.Printf("Type assertion failed for itemID %d", itemID)
		return
	}

	item := dl.items[itemID]
	row.itemID = itemID

	// Unlock before calling row.Update() to avoid potential deadlock
	row.Update(item)
}

// ClearSelection unselects all items.
func (dl *DuplicatesList) ClearSelection() {
	dl.mutex.Lock()
	for i := range dl.items {
		dl.items[i].Selected = false
	}
	dl.mutex.Unlock()

	dl.list.Refresh()
}

func (dl *DuplicatesList) handleRowTapped(itemID int, selected bool) {
	dl.mutex.Lock()

	if itemID < 0 || itemID >= len(dl.items) {
		dl.mutex.Unlock()
		return
	}

	item := &dl.items[itemID]

	// Ignore the columns header or group headers
	if item.IsColumnsHeader || item.IsGroupHeader {
		dl.mutex.Unlock()
		return
	}

	item.Selected = selected
	dl.mutex.Unlock()

	dl.list.Refresh()
}

