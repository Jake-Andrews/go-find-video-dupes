package ui

import (
	"log/slog"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"

	"govdupes/internal/models"
	"govdupes/internal/vm"
)

type DuplicatesListView struct {
	widget.BaseWidget

	vm vm.ViewModel

	list        *widget.List
	mutex       sync.RWMutex
	onRowTapped func(itemID int, selected bool)
}

// constructs the view, binds to vm, and sets up listeners.
func NewDuplicatesListView(vm vm.ViewModel) *DuplicatesListView {
	slog.Info("Creating DuplicatesListView")

	dl := &DuplicatesListView{
		vm: vm,
		onRowTapped: func(itemID int, selected bool) {
			vm.UpdateSelection(itemID, selected)
		},
	}

	// call before using dl.BaseWidget methods ?
	dl.ExtendBaseWidget(dl)

	dl.list = widget.NewList(
		func() int {
			items := dl.vm.GetItems()
			return visibleCount(items)
		},
		func() fyne.CanvasObject {
			return NewDuplicatesListRow(dl.onRowTapped)
		},
		func(itemID widget.ListItemID, co fyne.CanvasObject) {
			dl.updateListRow(itemID, co)
		},
	)

	// Whenever vm.DuplicateGroups changes, rebuild the flattened items, then refresh
	vm.AddDuplicateGroupsListener(binding.NewDataListener(func() {
		groups := vm.InterfaceToVideoData()
		vm.SetData(groups)
		dl.Refresh()
	}))

	return dl
}

// calls into the VM’s ApplyFilter function and refreshes the UI.
func (dl *DuplicatesListView) ApplyFilter(query models.SearchQuery) {
	dl.vm.ApplyFilter(query)
	dl.Refresh()
}

// calls into the VM’s ClearSelection function and refreshes the UI.
func (dl *DuplicatesListView) ClearSelection() {
	dl.vm.ClearSelection()
	dl.Refresh()
}

func (dl *DuplicatesListView) CreateRenderer() fyne.WidgetRenderer {
	// Render the entire widget as a simple container with the embedded list
	return widget.NewSimpleRenderer(dl.list)
}

// updateListRow sets the data for a specific row in the List.
func (dl *DuplicatesListView) updateListRow(itemID widget.ListItemID, co fyne.CanvasObject) {
	dl.mutex.RLock()
	defer dl.mutex.RUnlock()

	items := dl.vm.GetItems()
	realIndex := visibleIndexToItemIndex(items, itemID)
	if realIndex < 0 || realIndex >= len(items) {
		slog.Warn("Item ID out of bounds", "itemID", itemID)
		return
	}

	row, ok := co.(*DuplicatesListRow)
	if !ok {
		slog.Warn("Type assertion failed for itemID", "itemID", itemID)
		return
	}

	item := items[realIndex]
	row.itemID = realIndex
	row.Update(item)

	// Adjust row height for headers vs. normal rows
	if item.IsColumnsHeader || item.IsGroupHeader {
		dl.list.SetItemHeight(itemID, 50)
		return
	}

	rowMin := row.MinSize()
	totalRowHeight := fyne.Max(148, rowMin.Height)
	dl.list.SetItemHeight(itemID, totalRowHeight)
}

// ensures the underlying list is updated.
func (dl *DuplicatesListView) Refresh() {
	dl.list.Refresh()
	dl.BaseWidget.Refresh()
}

// Helpers for mapping “visible index” to the actual item in VM.

func visibleIndexToItemIndex(items []models.DuplicateListItemViewModel, visibleIndex int) int {
	count := 0
	for i, it := range items {
		if !it.Hidden {
			if count == visibleIndex {
				return i
			}
			count++
		}
	}
	return -1
}

func visibleCount(items []models.DuplicateListItemViewModel) int {
	count := 0
	for _, it := range items {
		if !it.Hidden {
			count++
		}
	}
	return count
}
