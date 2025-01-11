package vm

import (
	"fyne.io/fyne/v2/data/binding"

	"govdupes/internal/models"
)

type ViewModel interface {
	// Data Flattening / Groups
	SetData(videoData [][]*models.VideoData)
	InterfaceToVideoData() [][]*models.VideoData

	// Filtering
	ApplyFilter(query models.SearchQuery)

	// Get the flattenedritems for the UI to display
	GetItems() []*models.DuplicateListItemViewModel

	// Sorting
	SortVideoData(sortKey string, ascending bool)
	SortVideosByGroupSize(ascending bool)
	SortVideosByTotalVideos(ascending bool)

	// Selection & Manipulation
	UpdateSelection(itemIndex int, selected bool)
	ClearSelection()
	DeleteSelectedFromList()
	DeleteSelectedFromListDB()
	DeleteSelectedFromListDBDisk()
	HardlinkVideos() error

	// Selection methods
	SelectIdentical()
	SelectAllButLargest()
	SelectAllButSmallest()
	SelectAllButNewest()
	SelectAllButOldest()
	SelectAllButHighestBitrate()
	SelectAll()

	// UntypedList
	SetDuplicateGroups(groups []interface{}) error

	// Progress / Count fields
	UpdateFileCount(count string)
	UpdateAcceptedFiles(count string)
	UpdateGetFileInfoProgress(progress float64)
	UpdateGenPHashesProgress(progress float64)

	// Fyne binding
	GetFileInfoProgressBind() binding.Float
	GetPHashesProgressBind() binding.Float
	GetFileCountBind() binding.String
	GetAcceptedFilesBind() binding.String
	AddDuplicateGroupsListener(listener binding.DataListener)
}
