package ui

import (
	"log/slog"

	"govdupes/internal/vm"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func buildSortSelectDeleteTab(duplicatesView *DuplicatesListView, vm vm.ViewModel) fyne.CanvasObject {
	// Delete
	deleteOptions := []string{
		"Delete from list",
		"Delete from list & DB",
		"Delete from disk/list/DB",
	}
	deleteLabel := widget.NewLabel("Delete")
	deleteDropdown := widget.NewSelect(deleteOptions, nil)
	deleteDropdown.PlaceHolder = "Select an option"
	deleteButton := widget.NewButton("Delete", func() {
		if deleteDropdown.Selected == "" {
			return
		}
		switch deleteDropdown.Selected {
		case "Delete selected from list":
			duplicatesView.vm.DeleteSelectedFromList()
		case "Delete from list & DB":
			duplicatesView.vm.DeleteSelectedFromListDB()
		case "Delete from disk/list/DB":
			duplicatesView.vm.DeleteSelectedFromListDBDisk()
		}
	})

	// Hardlink
	hardlinkLabel := widget.NewLabel("Hardlink selected videos. Need 2+ videos selected per group.")
	hardlinkButton := widget.NewButton("Hardlink", func() {
		err := duplicatesView.vm.HardlinkVideos()
		if err != nil {
			slog.Error("Hardlink error", "error", err)
		}
		duplicatesView.Refresh()
	})

	// SELECT
	selectOptions := []string{
		"Select identical except path/name",
		"Select all but the largest",
		"Select all but the smallest",
		"Select all but the newest",
		"Select all but the oldest",
		"Select all but the highest bitrate",
		"Select all symbolic links",
		"Select all",
	}
	selectLabel := widget.NewLabel("Select")
	selectDropdown := widget.NewSelect(selectOptions, nil)
	selectDropdown.PlaceHolder = "Select an option"
	selectButton := widget.NewButton("Select", func() {
		if selectDropdown.Selected == "" {
			return
		}
		switch selectDropdown.Selected {
		case "Select identical except path/name":
			duplicatesView.vm.SelectIdentical()
		case "Select all but the largest":
			duplicatesView.vm.SelectAllButLargest()
		case "Select all but the smallest":
			duplicatesView.vm.SelectAllButSmallest()
		case "Select all but the newest":
			duplicatesView.vm.SelectAllButNewest()
		case "Select all but the oldest":
			duplicatesView.vm.SelectAllButOldest()
		case "Select all but the highest bitrate":
			duplicatesView.vm.SelectAllButHighestBitrate()
		case "Select all symbolic links":
			// duplicatesView.vm.SelectAllSymbolicLinks()
		case "Select all":
			duplicatesView.vm.SelectAll()
		}
		duplicatesView.Refresh()
	})

	// SORT
	sortOptions := []string{"Size", "Bitrate", "Resolution", "Group Size", "Group Video Count"}
	sortLabel := widget.NewLabel("Sort")
	dropdown := widget.NewSelect(sortOptions, nil)
	dropdown.PlaceHolder = "Select an option"

	sortOrder := map[string]bool{
		"Size":              true,
		"Bitrate":           true,
		"Resolution":        true,
		"Group Size":        true,
		"Group Video Count": true,
	}
	sortButton := widget.NewButton("Sort", func() {
		if dropdown.Selected == "" {
			return
		}
		sKey := dropdown.Selected
		ascending := sortOrder[sKey]

		switch sKey {
		case "Size":
			vm.SortVideoData("size", ascending)
		case "Bitrate":
			vm.SortVideoData("bitrate", ascending)
		case "Resolution":
			vm.SortVideoData("resolution", ascending)
		case "Group Size":
			vm.SortVideosByGroupSize(ascending)
		case "Group Video Count":
			vm.SortVideosByTotalVideos(ascending)
		}
		sortOrder[sKey] = !sortOrder[sKey] // flip sorting
		// duplicatesView.Refresh()
	})

	content := container.NewVBox(
		sortLabel, dropdown, sortButton,
		deleteLabel, deleteDropdown, deleteButton,
		hardlinkLabel, hardlinkButton,
		selectLabel, selectDropdown, selectButton,
	)
	return content
}
