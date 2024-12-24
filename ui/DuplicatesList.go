package ui

import (
	"fmt"
	"log"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"govdupes/internal/models"
)

// duplicateListItem is a flattened structure representing either a “group header”
// or a “video row” within our duplicates list.
type duplicateListItem struct {
	IsHeader   bool
	GroupIndex int    // which group this belongs to
	HeaderText string // only used if IsHeader == true

	VideoData *models.VideoData // only used if IsHeader == false
	Selected  bool
}

// DuplicatesList is a custom widget that displays all duplicates in a list,
// supports selection, and groups them with a header row.
type DuplicatesList struct {
	widget.BaseWidget

	// Data
	mutex sync.RWMutex
	items []duplicateListItem

	// The underlying Fyne List
	list *widget.List
}

// NewDuplicatesList creates and returns our custom DuplicatesList
func NewDuplicatesList(videoData [][]*models.VideoData) *DuplicatesList {
	log.Println("Creating DuplicatesList")

	dl := &DuplicatesList{}
	// Let Fyne know this is a custom widget
	dl.ExtendBaseWidget(dl)

	// ------------------------------------------------------
	// Create the Fyne widget.List, which handles scrolling
	// and calls create/update item for visible rows.
	// ------------------------------------------------------
	dl.list = widget.NewList(
		// lengthFunc
		func() int {
			dl.mutex.RLock()
			defer dl.mutex.RUnlock()
			return len(dl.items)
		},
		// createItemFunc
		func() fyne.CanvasObject {
			return NewDuplicatesListRow()
		},
		// updateItemFunc
		func(itemID widget.ListItemID, co fyne.CanvasObject) {
			dl.updateListRow(itemID, co)
		},
	)

	// Single selection for now:
	dl.list.OnSelected = func(id widget.ListItemID) {
		dl.handleSelection(id)
	}
	dl.list.OnUnselected = func(id widget.ListItemID) {
		dl.handleUnselection(id)
	}

	// Flatten the initial data
	log.Println("Setting video data")
	dl.SetData(videoData)

	log.Println("DuplicatesList created successfully")
	return dl
}

// SetData flattens videoData into our items slice: each group becomes
// 1 header row + N video rows.
func (dl *DuplicatesList) SetData(videoData [][]*models.VideoData) {
	log.Println("Flattening videoData into items")

	// Lock -> modify -> unlock, then refresh the list
	dl.mutex.Lock()
	dl.items = nil

	for i, group := range videoData {
		log.Printf("Processing group %d with %d items", i, len(group))

		// Add group header
		headerText := fmt.Sprintf("Group %d (Total %d duplicates)", i+1, len(group))
		dl.items = append(dl.items, duplicateListItem{
			IsHeader:   true,
			GroupIndex: i,
			HeaderText: headerText,
		})

		// Add video rows
		for _, vd := range group {
			log.Printf("Adding video row for group %d: %+v", i, vd)
			dl.items = append(dl.items, duplicateListItem{
				IsHeader:   false,
				GroupIndex: i,
				VideoData:  vd,
			})
		}
	}

	log.Printf("Total items flattened: %d", len(dl.items))
	dl.mutex.Unlock()

	// Now refresh the list after unlocking
	dl.list.Refresh()
}

// CreateRenderer is part of Fyne’s custom widget interface
func (dl *DuplicatesList) CreateRenderer() fyne.WidgetRenderer {
	// We only need to render the underlying “list” widget
	return widget.NewSimpleRenderer(dl.list)
}

// updateListRow populates the row’s visuals based on the item data
func (dl *DuplicatesList) updateListRow(itemID widget.ListItemID, co fyne.CanvasObject) {
	log.Printf("Updating item with ID: %d", itemID)

	dl.mutex.RLock()
	defer dl.mutex.RUnlock()

	if itemID < 0 || itemID >= len(dl.items) {
		log.Printf("Item ID %d out of bounds", itemID)
		return
	}

	row, ok := co.(*DuplicatesListRow)
	if !ok {
		// Non-fatal. Just skip it.
		log.Printf("Type assertion failed for itemID %d", itemID)
		return
	}

	item := dl.items[itemID]
	row.Update(item)
}

// handleSelection is called when the list item is selected
func (dl *DuplicatesList) handleSelection(itemID int) {
	dl.mutex.Lock()
	if itemID < 0 || itemID >= len(dl.items) {
		dl.mutex.Unlock()
		return
	}
	// If the item is a header, disallow selection
	if dl.items[itemID].IsHeader {
		dl.mutex.Unlock()
		dl.list.Unselect(itemID)
		return
	}

	// Single-select: Unselect all, then select this one
	for i := range dl.items {
		dl.items[i].Selected = false
	}
	dl.items[itemID].Selected = true
	dl.mutex.Unlock()

	// Refresh outside the lock
	dl.list.Refresh()
}

// handleUnselection if you want multi-select or toggling
func (dl *DuplicatesList) handleUnselection(itemID int) {
	dl.mutex.Lock()
	if itemID < 0 || itemID >= len(dl.items) {
		dl.mutex.Unlock()
		return
	}

	if dl.items[itemID].Selected {
		dl.items[itemID].Selected = false
	}
	dl.mutex.Unlock()

	// Refresh outside the lock
	dl.list.Refresh()
}

// selectedItems returns all selected rows that are not headers
func (dl *DuplicatesList) selectedItems() []duplicateListItem {
	dl.mutex.RLock()
	defer dl.mutex.RUnlock()

	var sel []duplicateListItem
	for _, it := range dl.items {
		if !it.IsHeader && it.Selected {
			sel = append(sel, it)
		}
	}
	return sel
}

// ClearSelection unselects all items
func (dl *DuplicatesList) ClearSelection() {
	dl.mutex.Lock()
	for i := range dl.items {
		dl.items[i].Selected = false
	}
	dl.mutex.Unlock()

	dl.list.Refresh()
}

