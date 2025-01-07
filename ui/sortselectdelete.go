package ui

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"govdupes/internal/models"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func buildSortSelectDeleteTab(duplicatesList *DuplicatesList, videoData [][]*models.VideoData) fyne.CanvasObject {
	// HIDE

	// HARDLINK
	hardlinkLabel := widget.NewLabel("Hardlink selected videos to first video in group. Need 2+ videos selected per group.")
	hardlinkButton := widget.NewButton("Hardlink", func() {
		slog.Info("Hardlink button pressed")
		hardlinkVideos(duplicatesList, videoData)
		// Remove selected videos from the list
		deleteVideosFromList(duplicatesList, videoData)
		duplicatesList.SetData(videoData)
	})

	// DELETE
	deleteOptions := []string{"From list", "From list & DB", "From disk"}
	deleteLabel := widget.NewLabel("Delete selected")
	deleteDropdown := widget.NewSelect(deleteOptions, nil)
	deleteDropdown.PlaceHolder = "Select an option"

	deleteButton := widget.NewButton("Delete", func() {
		if deleteDropdown.Selected == "" {
			slog.Warn("No delete option selected")
			return
		}

		switch deleteDropdown.Selected {
		case "From list":
			deleteVideosFromList(duplicatesList, videoData)
			slog.Info("Deleted from list")
		case "From list & DB":
			slog.Info("Deleted from list & database")
		case "From disk":
			slog.Info("Deleted from disk")
			// Add a modal here
			deleteVideosFromDisk(duplicatesList, videoData)
		}

		duplicatesList.SetData(videoData)
	})

	// SELECT
	selectOptions := []string{"Select identical except path/name", "Select all but the largest", "Select all but the smallest", "Select all but the newest", "Select all but the oldest", "Select all but the highest bitrate", "Select all symbolic links", "Select all"}
	selectLabel := widget.NewLabel("Select")
	selectDropdown := widget.NewSelect(selectOptions, nil)
	selectDropdown.PlaceHolder = "Select an option"
	selectButton := widget.NewButton("Select", func() {
		if selectDropdown.Selected == "" {
			return
		}

		switch selectDropdown.Selected {
		case "Select identical except path/name":
			selectIdentical(duplicatesList)
		case "Select all but the largest":
			selectAllButLargest(duplicatesList)
		case "Select all but the smallest":
			selectAllButSmallest(duplicatesList)
		case "Select all but the newest":
			selectAllButNewest(duplicatesList)
		case "Select all but the oldest":
			selectAllButOldest(duplicatesList)
		case "Select all but the highest bitrate":
			selectAllButHighestBitrate(duplicatesList)
		case "Select all symbolic links":
			selectAllSymbolicLinks(duplicatesList)
		case "Select all":
			selectAllVideos(duplicatesList)
		}
		duplicatesList.Refresh()
	})

	// SORT
	sortOptions := []string{"Size", "Bitrate", "Resolution", "Group Size", "Total Videos"}
	sortLabel := widget.NewLabel("Sort")
	dropdown := widget.NewSelect(sortOptions, nil)
	dropdown.PlaceHolder = "Select an option"

	sortOrder := map[string]bool{
		"Size":                true, // true = ascending, false = descending
		"Bitrate":             true,
		"Resolution":          true,
		"(Group) Size":        true,
		"(Group) Video Count": true,
	}

	sortButton := widget.NewButton("Sort", func() {
		if dropdown.Selected == "" {
			return
		}

		switch dropdown.Selected {
		case "Size":
			if sortOrder["Size"] {
				sortVideosBySize(videoData, true)
			} else {
				sortVideosBySize(videoData, false)
			}
			sortOrder["Size"] = !sortOrder["Size"]
		case "Bitrate":
			if sortOrder["Bitrate"] {
				sortVideosByBitrate(videoData, true)
			} else {
				sortVideosByBitrate(videoData, false)
			}
			sortOrder["Bitrate"] = !sortOrder["Bitrate"]
		case "Resolution":
			if sortOrder["Resolution"] {
				sortVideosByResolution(videoData, true)
			} else {
				sortVideosByResolution(videoData, false)
			}
			sortOrder["Resolution"] = !sortOrder["Resolution"]
		case "Group Size":
			if sortOrder["(Group) Size"] {
				sortVideosByGroupSize(videoData, true)
			} else {
				sortVideosByGroupSize(videoData, false)
			}
			sortOrder["(Group) Size"] = !sortOrder["(Group) Size"]
		case "Total Videos":
			if sortOrder["(Group) Video Count"] {
				sortVideosByTotalVideos(videoData, true)
			} else {
				sortVideosByTotalVideos(videoData, false)
			}
			sortOrder["(Group) Video Count"] = !sortOrder["(Group) Video Count"]
		}

		duplicatesList.SetData(videoData)
	})

	content := container.NewVBox(sortLabel, dropdown, sortButton, deleteLabel, deleteDropdown, deleteButton, selectLabel, selectDropdown, selectButton, hardlinkLabel, hardlinkButton)
	return content
}

// Sort by size
func sortVideosBySize(videoData [][]*models.VideoData, ascending bool) {
	for _, group := range videoData {
		sort.SliceStable(group, func(i, j int) bool {
			if ascending {
				return group[i].Video.Size < group[j].Video.Size
			}
			return group[i].Video.Size > group[j].Video.Size
		})
	}
}

// Sort by bitrate
func sortVideosByBitrate(videoData [][]*models.VideoData, ascending bool) {
	for _, group := range videoData {
		sort.SliceStable(group, func(i, j int) bool {
			if ascending {
				return group[i].Video.BitRate < group[j].Video.BitRate
			}
			return group[i].Video.BitRate > group[j].Video.BitRate
		})
	}
}

// Sort by resolution (width × height)
func sortVideosByResolution(videoData [][]*models.VideoData, ascending bool) {
	for _, group := range videoData {
		sort.SliceStable(group, func(i, j int) bool {
			resI := group[i].Video.Width * group[i].Video.Height
			resJ := group[j].Video.Width * group[j].Video.Height
			if ascending {
				return resI < resJ
			}
			return resI > resJ
		})
	}
}

func deleteVideosFromDisk(duplicatesList *DuplicatesList, videoData2d [][]*models.VideoData) {
	//
}

func deleteVideosFromList(duplicatesList *DuplicatesList, videoData2d [][]*models.VideoData) {
	duplicatesList.mutex.Lock()
	defer duplicatesList.mutex.Unlock()
	selectedIDs := make(map[int64]struct{})
	for _, item := range duplicatesList.items {
		if item.Selected && item.VideoData != nil {
			selectedIDs[item.VideoData.Video.ID] = struct{}{}
		}
	}
	for i := range videoData2d {
		filteredRow := videoData2d[i][:0]
		for _, videoData := range videoData2d[i] {
			if _, found := selectedIDs[videoData.Video.ID]; !found {
				filteredRow = append(filteredRow, videoData)
			}
		}
		videoData2d[i] = filteredRow
	}
}

func selectIdentical(duplicatesList *DuplicatesList) {
	duplicatesList.ClearSelection()

	duplicatesList.mutex.Lock()
	defer duplicatesList.mutex.Unlock()

	// Group items by GroupIndex.
	groupedItems := make(map[int][]*duplicateListItem)
	for i := range duplicatesList.items {
		item := &duplicatesList.items[i]
		if item.IsColumnsHeader || item.IsGroupHeader || item.VideoData == nil {
			continue
		}
		groupedItems[item.GroupIndex] = append(groupedItems[item.GroupIndex], item)
	}

	// Process each group to find and select identical items.
	for _, items := range groupedItems {
		identicalGroups := findIdenticalGroups(items)

		// Select the first pair of identical items in each identical group.
		for _, identicalItems := range identicalGroups {
			if len(identicalItems) >= 2 {
				identicalItems[0].Selected = true
				identicalItems[1].Selected = true
			}
		}
	}
}

// findIdenticalGroups identifies groups of items that are identical except for their path.
func findIdenticalGroups(items []*duplicateListItem) [][]*duplicateListItem {
	identicalMap := make(map[string][]*duplicateListItem)

	for _, item := range items {
		// Create a key representing the item's attributes excluding the path.
		key := generateKeyExcludingPath(item.VideoData)
		identicalMap[key] = append(identicalMap[key], item)
	}

	// Collect groups of identical items.
	var identicalGroups [][]*duplicateListItem
	for _, group := range identicalMap {
		if len(group) > 1 {
			identicalGroups = append(identicalGroups, group)
		}
	}

	return identicalGroups
}

// generateKeyExcludingPath generates a unique key for a VideoData excluding its path.
func generateKeyExcludingPath(videoData *models.VideoData) string {
	if videoData == nil {
		return ""
	}
	return fmt.Sprintf("%v|%v|%v|%v|%v", videoData.Video.Size, videoData.Video.Duration, videoData.Video.VideoCodec, videoData.Video.AudioCodec, videoData.Video.AvgFrameRate)
}

// selectAllButLargest selects every video in the group EXCEPT the video with the largest Size
func selectAllButLargest(duplicatesList *DuplicatesList) {
	duplicatesList.ClearSelection()

	duplicatesList.mutex.Lock()
	defer duplicatesList.mutex.Unlock()

	// Find the largest size per group.
	groupMaxSize := make(map[int]int64)
	for _, item := range duplicatesList.items {
		if item.VideoData == nil {
			continue
		}
		sz := item.VideoData.Video.Size
		if sz > groupMaxSize[item.GroupIndex] {
			groupMaxSize[item.GroupIndex] = sz
		}
	}

	// Select all items that are not the largest in that group.
	for i := range duplicatesList.items {
		item := &duplicatesList.items[i]
		if item.VideoData == nil || item.IsGroupHeader || item.IsColumnsHeader {
			continue
		}
		if item.VideoData.Video.Size < groupMaxSize[item.GroupIndex] {
			item.Selected = true
		}
	}
}

// selectAllButSmallest selects every video in the group EXCEPT the video with the smallest Size
func selectAllButSmallest(duplicatesList *DuplicatesList) {
	duplicatesList.ClearSelection()

	duplicatesList.mutex.Lock()
	defer duplicatesList.mutex.Unlock()

	// Find the smallest size per group.
	groupMinSize := make(map[int]int64)
	// Initialize to a very large number so first real Size comparison will replace it
	for _, item := range duplicatesList.items {
		if item.VideoData == nil {
			continue
		}
		// only initialize if not set
		if _, ok := groupMinSize[item.GroupIndex]; !ok {
			groupMinSize[item.GroupIndex] = 1<<63 - 1 // max int64
		}
	}

	for _, item := range duplicatesList.items {
		if item.VideoData == nil {
			continue
		}
		sz := item.VideoData.Video.Size
		if sz < groupMinSize[item.GroupIndex] {
			groupMinSize[item.GroupIndex] = sz
		}
	}

	// Select all items that are not the smallest in that group.
	for i := range duplicatesList.items {
		item := &duplicatesList.items[i]
		if item.VideoData == nil || item.IsGroupHeader || item.IsColumnsHeader {
			continue
		}
		if item.VideoData.Video.Size > groupMinSize[item.GroupIndex] {
			item.Selected = true
		}
	}
}

// selectAllButNewest selects every video in the group EXCEPT the video with the newest (max) ModifiedAt
func selectAllButNewest(duplicatesList *DuplicatesList) {
	duplicatesList.ClearSelection()

	duplicatesList.mutex.Lock()
	defer duplicatesList.mutex.Unlock()

	// Find the newest (max) ModifiedAt per group.
	groupMaxModified := make(map[int]time.Time)
	for _, item := range duplicatesList.items {
		if item.VideoData == nil {
			continue
		}
		t := item.VideoData.Video.ModifiedAt
		if t.After(groupMaxModified[item.GroupIndex]) {
			groupMaxModified[item.GroupIndex] = t
		}
	}

	// Select all items that are not the newest in that group.
	for i := range duplicatesList.items {
		item := &duplicatesList.items[i]
		if item.VideoData == nil || item.IsGroupHeader || item.IsColumnsHeader {
			continue
		}
		t := item.VideoData.Video.ModifiedAt
		// If this video's ModifiedAt is before the group's max, select it
		if t.Before(groupMaxModified[item.GroupIndex]) {
			item.Selected = true
		}
	}
}

// selectAllButOldest selects every video in the group EXCEPT the video with the oldest (min) ModifiedAt.
func selectAllButOldest(duplicatesList *DuplicatesList) {
	duplicatesList.ClearSelection()

	duplicatesList.mutex.Lock()
	defer duplicatesList.mutex.Unlock()

	// Find the oldest (min) ModifiedAt per group.
	// Initialize each group to a large time so we can compare properly
	groupMinModified := make(map[int]time.Time)
	for _, item := range duplicatesList.items {
		if item.VideoData == nil {
			continue
		}
		// Only initialize if not set (time.Time{})
		// set it to a far future date.
		if _, ok := groupMinModified[item.GroupIndex]; !ok {
			groupMinModified[item.GroupIndex] = time.Date(3000, 1, 1, 0, 0, 0, 0, time.UTC)
		}
	}

	for _, item := range duplicatesList.items {
		if item.VideoData == nil {
			continue
		}
		t := item.VideoData.Video.ModifiedAt
		if t.Before(groupMinModified[item.GroupIndex]) {
			groupMinModified[item.GroupIndex] = t
		}
	}

	// Select all items that are not the oldest in that group.
	for i := range duplicatesList.items {
		item := &duplicatesList.items[i]
		if item.VideoData == nil || item.IsGroupHeader || item.IsColumnsHeader {
			continue
		}
		t := item.VideoData.Video.ModifiedAt
		// If this video's ModifiedAt is after the group's min, select it
		if t.After(groupMinModified[item.GroupIndex]) {
			item.Selected = true
		}
	}
}

// selectAllButHighestBitrate selects every video in the group EXCEPT the video
// with the highest BitRate
func selectAllButHighestBitrate(duplicatesList *DuplicatesList) {
	duplicatesList.ClearSelection()

	duplicatesList.mutex.Lock()
	defer duplicatesList.mutex.Unlock()

	// Find the highest bitrate per group.
	groupMaxBitrate := make(map[int]int)
	for _, item := range duplicatesList.items {
		if item.VideoData == nil {
			continue
		}
		br := item.VideoData.Video.BitRate
		if br > groupMaxBitrate[item.GroupIndex] {
			groupMaxBitrate[item.GroupIndex] = br
		}
	}

	// Select all items that are not the highest in that group.
	for i := range duplicatesList.items {
		item := &duplicatesList.items[i]
		if item.VideoData == nil || item.IsGroupHeader || item.IsColumnsHeader {
			continue
		}
		br := item.VideoData.Video.BitRate
		if br < groupMaxBitrate[item.GroupIndex] {
			item.Selected = true
		}
	}
}

// selectAllButLowestBitrate selects every video in the group except the videos with the lowest .BitRate.
func selectAllButLowestBitrate(duplicatesList *DuplicatesList) {
	duplicatesList.ClearSelection()

	duplicatesList.mutex.Lock()
	defer duplicatesList.mutex.Unlock()

	// Find the lowest bitrate per group.
	// Initialize each group to a large int so we can properly compare
	groupMinBitrate := make(map[int]int)
	existingGroups := make(map[int]struct{})

	// Identify all groups first
	for _, item := range duplicatesList.items {
		if item.VideoData == nil {
			continue
		}
		existingGroups[item.GroupIndex] = struct{}{}
	}
	// Initialize min to a large number
	for g := range existingGroups {
		groupMinBitrate[g] = 1<<31 - 1 // big enough for typical 32-bit integer
	}

	// Now find the actual minimum
	for _, item := range duplicatesList.items {
		if item.VideoData == nil {
			continue
		}
		br := item.VideoData.Video.BitRate
		if br < groupMinBitrate[item.GroupIndex] {
			groupMinBitrate[item.GroupIndex] = br
		}
	}

	// Select all items that are not the lowest in that group.
	for i := range duplicatesList.items {
		item := &duplicatesList.items[i]
		if item.VideoData == nil || item.IsGroupHeader || item.IsColumnsHeader {
			continue
		}
		br := item.VideoData.Video.BitRate
		if br > groupMinBitrate[item.GroupIndex] {
			item.Selected = true
		}
	}
}

func selectAllSymbolicLinks(duplicatesList *DuplicatesList) {
	duplicatesList.ClearSelection()

	duplicatesList.mutex.Lock()
	defer duplicatesList.mutex.Unlock()

	for i := range duplicatesList.items {
		item := &duplicatesList.items[i]
		if item.VideoData != nil && item.VideoData.Video.IsSymbolicLink {
			item.Selected = true
		}
	}
}

func selectAllVideos(duplicatesList *DuplicatesList) {
	duplicatesList.ClearSelection()

	duplicatesList.mutex.Lock()
	defer duplicatesList.mutex.Unlock()

	for i := range duplicatesList.items {
		item := &duplicatesList.items[i]
		if item.VideoData != nil {
			item.Selected = true
		}
	}
}

func hardlinkVideos(duplicatesList *DuplicatesList, videoData2d [][]*models.VideoData) {
	slog.Info("Starting hardlinkVideos function")

	// Map to store selected video IDs to avoid processing duplicates
	selectedIDs := make(map[int64]struct{})

	// Collect selected video IDs from duplicatesList
	for _, item := range duplicatesList.items {
		slog.Info("duplicatesList.items")
		if item.Selected && item.VideoData != nil {
			selectedIDs[item.VideoData.Video.ID] = struct{}{}
			slog.Info("duplicatesList.items", slog.String("item.VideoData.Video.Path", item.VideoData.Video.Path))
		}
	}

	// Iterate over groups of video data
	for groupIdx, group := range videoData2d {
		slog.Info("Processing group", slog.Int("groupIdx", groupIdx), slog.Int("videoCount", len(group)))

		// Filter selected videos in the current group
		var selectedVideos []*models.Video
		for _, videoData := range group {
			if _, ok := selectedIDs[videoData.Video.ID]; ok {
				selectedVideos = append(selectedVideos, &videoData.Video)
			}
		}

		if len(selectedVideos) <= 1 {
			slog.Warn("Less than 2 selected videos, skipping hardlinking", slog.Int("groupIdx", groupIdx))
			continue
		}

		// Perform hardlinking
		for i := 1; i < len(selectedVideos); i++ {
			// Extract the directory from the path of the first selected
			// videos. Scrapy fix in case program is being run for a path on
			// another machine (cannot hardlink across filesystems)
			baseDir := filepath.Dir(selectedVideos[0].Path)

			// Construct the temporary file path in the same directory
			tmpFilePath := fmt.Sprintf("%s/govdupes_%d.tmp", baseDir, time.Now().UnixNano())

			slog.Info("Creating hardlink", slog.String("sourcePath", selectedVideos[0].Path), slog.String("tempPath", tmpFilePath))

			slog.Info("Creating hardlink", slog.String("sourcePath", selectedVideos[0].Path), slog.String("tempPath", tmpFilePath))

			if err := os.Link(selectedVideos[0].Path, tmpFilePath); err != nil {
				slog.Error("Failed to create hardlink", slog.String("path", selectedVideos[0].Path), slog.Any("error", err))
				continue
			}

			if err := os.Rename(tmpFilePath, selectedVideos[i].Path); err != nil {
				slog.Error("Failed to move hardlinked file", slog.String("path", selectedVideos[i].Path), slog.Any("error", err))
				_ = os.Remove(tmpFilePath)
				continue
			}

			slog.Info("Successfully hardlinked:", slog.String("path", selectedVideos[0].Path), slog.String("path", selectedVideos[i].Path))
		}
	}

	slog.Info("Finished hardlinkVideos function")
}

func sortVideosByGroupSize(videoData [][]*models.VideoData, ascending bool) {
	sort.Slice(videoData, func(i, j int) bool {
		calculateTotalSize := func(group []*models.VideoData) int64 {
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
			return totalSize
		}

		sizeI := calculateTotalSize(videoData[i])
		sizeJ := calculateTotalSize(videoData[j])

		if ascending {
			return sizeI < sizeJ
		}
		return sizeI > sizeJ
	})
}

func sortVideosByTotalVideos(videoData [][]*models.VideoData, ascending bool) {
	countVideos := func(group []*models.VideoData) int {
		return len(group)
	}

	sort.Slice(videoData, func(i, j int) bool {
		if ascending {
			return countVideos(videoData[i]) < countVideos(videoData[j])
		}
		return countVideos(videoData[i]) > countVideos(videoData[j])
	})
}
