// duplicateList.go
package ui

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"govdupes/internal/models"
)

// duplicateListItem represents header rows or a video row
// duplicateListItem represents header rows or a video row
type duplicateListItem struct {
	IsColumnsHeader bool
	IsGroupHeader   bool
	GroupIndex      int

	HeaderText string
	VideoData  *models.VideoData
	Selected   bool

	// New field: true if this row should be hidden due to filter
	Hidden bool
}

// DuplicatesList displays videos in duplicate groups, with a single header row at the top.
type DuplicatesList struct {
	widget.BaseWidget

	mutex sync.RWMutex
	items []duplicateListItem

	list        *widget.List
	OnRowTapped func(itemID int, selected bool)
}

// NewDuplicatesList creates and returns our custom DuplicatesList.
func NewDuplicatesList(videoData [][]*models.VideoData) *DuplicatesList {
	slog.Info("Creating DuplicatesList")
	dl := &DuplicatesList{}
	dl.ExtendBaseWidget(dl)

	dl.OnRowTapped = func(itemID int, selected bool) {
		dl.handleRowTapped(itemID, selected)
	}

	// Build the widget.List
	dl.list = widget.NewList(
		// 1) Return how many are visible
		func() int {
			dl.mutex.RLock()
			defer dl.mutex.RUnlock()
			return dl.visibleCount()
		},
		// 2) Create the row
		func() fyne.CanvasObject {
			return NewDuplicatesListRow(dl.OnRowTapped)
		},
		// 3) Update the row
		func(itemID widget.ListItemID, co fyne.CanvasObject) {
			dl.updateListRow(itemID, co)
		},
	)

	// Insert data
	dl.SetData(videoData)

	return dl
}

func (dl *DuplicatesList) ApplyFilter(query searchQuery) {
	dl.mutex.Lock()
	defer dl.mutex.Unlock()

	// 1) If query is empty, just unhide everything
	if len(query.orGroups) == 0 {
		for i := range dl.items {
			dl.items[i].Hidden = false
		}
		return
	}

	// 2) Hide or show each row
	for i, it := range dl.items {
		// Always show the "Columns" header
		if it.IsColumnsHeader {
			dl.items[i].Hidden = false
			continue
		}

		// Evaluate group headers AFTER we know about their items
		// For now, tentatively hide them; we’ll fix group-headers in next step
		if it.IsGroupHeader {
			dl.items[i].Hidden = true
			continue
		}

		// Normal video row
		if it.VideoData == nil {
			// e.g. no data => hide
			dl.items[i].Hidden = true
			continue
		}

		// If it matches the query, unhide; else hide
		if rowMatchesQuery(it, query) {
			dl.items[i].Hidden = false
		} else {
			dl.items[i].Hidden = true
		}
	}

	// 3) Show group-headers if at least one item in that group is not hidden
	groupHasVisible := make(map[int]bool)
	for _, it := range dl.items {
		if !it.IsColumnsHeader && !it.IsGroupHeader && !it.Hidden {
			groupHasVisible[it.GroupIndex] = true
		}
	}
	for i, it := range dl.items {
		if it.IsGroupHeader && groupHasVisible[it.GroupIndex] {
			dl.items[i].Hidden = false
		}
	}
}

func rowMatchesQuery(it duplicateListItem, query searchQuery) bool {
	if it.VideoData == nil {
		return false
	}

	// Combine path + filename
	checkStr := it.VideoData.Video.Path + " " + it.VideoData.Video.FileName
	checkStr = strings.ToLower(checkStr)

	// Must satisfy at least one OR-group
	for _, ag := range query.orGroups {
		if andGroupSatisfied(checkStr, ag) {
			return true
		}
	}
	return false
}

// SetData flattens the groups into a single list.
//
//  1. If there is at least one non-empty group, add a columns header row (once).
//  2. For each group with >=2 items:
//     a) Group header
//     b) One item per video
func (dl *DuplicatesList) SetData(videoData [][]*models.VideoData) {
	dl.mutex.Lock()
	dl.items = nil

	// Check if we have at least one non-empty group
	hasAnyVideos := false
	for _, group := range videoData {
		if len(group) > 1 {
			hasAnyVideos = true
			break
		}
	}

	// Remove groups that have 0 or 1 videos, since they are not duplicates
	j := 0
	for i := 0; i < len(videoData); i++ {
		group := videoData[i]
		if len(group) <= 1 {
			continue
		}
		videoData[j] = videoData[i]
		j++
	}
	videoData = videoData[:j]

	// Add the columns header row if we have any groups
	if hasAnyVideos {
		dl.items = append(dl.items, duplicateListItem{
			IsColumnsHeader: true,
		})
	}

	// Add group headers + video items
	for i, group := range videoData {
		if len(group) == 0 {
			continue
		}

		// Sum unique total size
		uniqueInodeDeviceID := make(map[string]bool)
		uniquePaths := make(map[string]bool)
		totalSize := int64(0)
		for _, vd := range group {
			uniquePaths[vd.Video.Path] = true
			if vd.Video.IsSymbolicLink {
				if uniquePaths[vd.Video.SymbolicLink] {
					continue
				}
				totalSize += vd.Video.Size
				continue
			}
			identifier := fmt.Sprintf("%d:%d", vd.Video.Inode, vd.Video.Device)
			if !uniqueInodeDeviceID[identifier] {
				uniqueInodeDeviceID[identifier] = true
				totalSize += vd.Video.Size
			}
		}

		groupHeaderText := fmt.Sprintf("Group %d (Total %d duplicates, Size: %s)",
			i+1, len(group), formatFileSize(totalSize))

		dl.items = append(dl.items, duplicateListItem{
			IsGroupHeader: true,
			GroupIndex:    i,
			HeaderText:    groupHeaderText,
		})

		for _, vd := range group {
			dl.items = append(dl.items, duplicateListItem{
				GroupIndex: i,
				VideoData:  vd,
			})
		}
	}

	dl.mutex.Unlock()
	dl.list.Refresh()
}

// CreateRenderer is part of Fyne’s custom widget interface.
// We only need to render the underlying “list” widget.
func (dl *DuplicatesList) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(dl.list)
}

func (dl *DuplicatesList) updateListRow(itemID widget.ListItemID, co fyne.CanvasObject) {
	dl.mutex.RLock()
	realIndex := dl.visibleIndexToItemIndex(itemID)
	if realIndex < 0 || realIndex >= len(dl.items) {
		slog.Warn("Item ID out of bounds", slog.Int("itemID", itemID))
		dl.mutex.RUnlock()
		return
	}

	row, ok := co.(*DuplicatesListRow)
	if !ok {
		slog.Warn("Type assertion failed for itemID", slog.Int("itemID", itemID))
		dl.mutex.RUnlock()
		return
	}

	item := dl.items[realIndex]
	row.itemID = realIndex

	// Update row
	row.Update(item)

	// If needed, set a specific item height
	if item.IsColumnsHeader || item.IsGroupHeader {
		dl.mutex.RUnlock()
		dl.list.SetItemHeight(itemID, 50)
		return
	}

	rowMin := row.MinSize()
	totalRowHeight := fyne.Max(148, rowMin.Height)
	dl.mutex.RUnlock()
	dl.list.SetItemHeight(itemID, totalRowHeight)
}

// visibleIndexToItemIndex returns the actual index in dl.items for the nth visible item
func (dl *DuplicatesList) visibleIndexToItemIndex(visibleIndex int) int {
	count := 0
	for i, it := range dl.items {
		if !it.Hidden {
			if count == visibleIndex {
				return i
			}
			count++
		}
	}
	return -1
}

func (dl *DuplicatesList) visibleCount() int {
	count := 0
	for _, it := range dl.items {
		if !it.Hidden {
			count++
		}
	}
	return count
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
