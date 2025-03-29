package viewmodel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2/data/binding"

	"govdupes/internal/application"
	"govdupes/internal/models"
	"govdupes/internal/vm"
)

// duplicates list logic (filtering, selection, flattening).
// bindings that update various UI elements FileCount, etc.).
type viewModel struct {
	items           []*models.DuplicateListItemViewModel
	DuplicateGroups binding.UntypedList
	mutex           sync.RWMutex

	FileCount             binding.String
	AcceptedFiles         binding.String
	GetFileInfoProgress   binding.Float
	GenPHashesProgress    binding.Float
	TotalGroupSize        binding.String
	PotentialSpaceSavings binding.String

	Application *application.App
}

func NewViewModel(app *application.App) vm.ViewModel {
	slog.Info("newviewmodel")
	return &viewModel{
		DuplicateGroups:       binding.NewUntypedList(),
		items:                 make([]*models.DuplicateListItemViewModel, 0),
		FileCount:             binding.NewString(),
		AcceptedFiles:         binding.NewString(),
		GetFileInfoProgress:   binding.NewFloat(),
		GenPHashesProgress:    binding.NewFloat(),
		TotalGroupSize:        binding.NewString(),
		PotentialSpaceSavings: binding.NewString(),
		Application:           app,
	}
}

// Flattening / List
// _________________

// InterfaceToVideoData safely returns the [][]*models.VideoData from the DuplicateGroups list.
func (vm *viewModel) InterfaceToVideoData() [][]*models.VideoData {
	items, err := vm.DuplicateGroups.Get()
	if err != nil {
		slog.Error("vm.DuplicateGroups.Get() failed", "err", err)
		return nil
	}
	newGroups := make([][]*models.VideoData, len(items))
	for i, it := range items {
		newGroups[i] = it.([]*models.VideoData)
	}
	return newGroups
}

// SetData receives raw duplicates data from outside and flattens into vm.items.
func (vm *viewModel) SetData(videoData [][]*models.VideoData) {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()

	vm.items = vm.items[:0]

	hasAnyVideos := false
	for _, group := range videoData {
		if len(group) > 1 {
			hasAnyVideos = true
			break
		}
	}

	var filteredGroups [][]*models.VideoData
	for _, group := range videoData {
		if len(group) > 1 {
			filteredGroups = append(filteredGroups, group)
		}
	}

	if hasAnyVideos {
		vm.items = append(vm.items, &models.DuplicateListItemViewModel{
			IsColumnsHeader: true,
		})
	}

	for i, group := range filteredGroups {
		uniqueInodeDeviceID := make(map[string]bool)
		uniquePaths := make(map[string]bool)
		var totalSize int64

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

		groupHeaderText := fmt.Sprintf(
			"Group %d (Total %d duplicates, Size: %s)",
			i+1, len(group), formatFileSize(totalSize),
		)

		vm.items = append(vm.items, &models.DuplicateListItemViewModel{
			IsGroupHeader: true,
			GroupIndex:    i,
			HeaderText:    groupHeaderText,
		})

		for _, vd := range group {
			vm.items = append(vm.items, &models.DuplicateListItemViewModel{
				GroupIndex: i,
				VideoData:  vd,
			})
		}
	}
	vm.UpdateStatistics(filteredGroups)
}

// GetItems safely returns a copy of vm.items for the View to read.
func (vm *viewModel) GetItems() []*models.DuplicateListItemViewModel {
	vm.mutex.RLock()
	defer vm.mutex.RUnlock()
	itemsCopy := make([]*models.DuplicateListItemViewModel, len(vm.items))
	copy(itemsCopy, vm.items)
	return itemsCopy
}

// UpdateSelection toggles the Selected field on a row.
func (vm *viewModel) UpdateSelection(itemIndex int, selected bool) {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	if itemIndex < 0 || itemIndex >= len(vm.items) {
		return
	}
	item := vm.items[itemIndex]
	if item.IsColumnsHeader || item.IsGroupHeader {
		return
	}
	item.Selected = selected
}

// ClearSelection sets Selected=false on all rows.
func (vm *viewModel) ClearSelection() {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	for _, item := range vm.items {
		item.Selected = false
	}
}

// Filtering
// _________

func (vm *viewModel) ApplyFilter(query models.SearchQuery) {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()

	if len(query.OrGroups) == 0 {
		for i := range vm.items {
			vm.items[i].Hidden = false
		}
		return
	}

	for i, it := range vm.items {
		if it.IsColumnsHeader {
			vm.items[i].Hidden = false
			continue
		}
		if it.IsGroupHeader {
			vm.items[i].Hidden = true
			continue
		}
		if it.VideoData == nil {
			vm.items[i].Hidden = true
			continue
		}
		// Filter check
		if rowMatchesQuery(it, query) {
			vm.items[i].Hidden = false
		} else {
			vm.items[i].Hidden = true
		}
	}

	groupHasVisible := make(map[int]bool)
	for _, it := range vm.items {
		if !it.IsColumnsHeader && !it.IsGroupHeader && !it.Hidden {
			groupHasVisible[it.GroupIndex] = true
		}
	}
	for i, it := range vm.items {
		if it.IsGroupHeader && groupHasVisible[it.GroupIndex] {
			vm.items[i].Hidden = false
		}
	}
}

func rowMatchesQuery(it *models.DuplicateListItemViewModel, query models.SearchQuery) bool {
	if it.VideoData == nil {
		return false
	}
	checkStr := strings.ToLower(it.VideoData.Video.Path + " " + it.VideoData.Video.FileName)

	for _, andGroup := range query.OrGroups {
		if andGroupSatisfied(checkStr, andGroup) {
			return true
		}
	}
	return false
}

func andGroupSatisfied(checkStr string, andGroup []string) bool {
	for _, s := range andGroup {
		s = strings.ToLower(s)
		if !strings.Contains(checkStr, s) {
			return false
		}
	}
	return true
}

// Sorting / Flattening
// ___________________

func (vm *viewModel) SortVideoData(sortKey string, ascending bool) {
	// changing videoDataGroups changes DuplicateGroups since
	// vm.InterfaceToVideoData doesn't make a deep copy
	videoDataGroups := vm.InterfaceToVideoData()
	defer vm.SetViewModelDuplicateGroups(videoDataGroups)

	for _, group := range videoDataGroups {
		sort.SliceStable(group, func(i, j int) bool {
			var less bool
			switch sortKey {
			case "path":
				less = group[i].Video.Path < group[j].Video.Path
			case "filename":
				less = group[i].Video.FileName < group[j].Video.FileName
			case "createdat":
				less = group[i].Video.CreatedAt.Before(group[j].Video.CreatedAt)
			case "modifiedat":
				less = group[i].Video.ModifiedAt.Before(group[j].Video.ModifiedAt)
			case "duration":
				less = group[i].Video.Duration < group[j].Video.Duration
			case "size":
				less = group[i].Video.Size < group[j].Video.Size
			case "bitrate":
				less = group[i].Video.BitRate < group[j].Video.BitRate
			case "resolution":
				resI := group[i].Video.Width * group[i].Video.Height
				resJ := group[j].Video.Width * group[j].Video.Height
				less = resI < resJ
			default:
				slog.Warn("Unknown sortKey", "sortKey", sortKey)
				return false
			}

			if ascending {
				return less
			}
			return !less
		})
	}
}

func (vm *viewModel) SortVideosByGroupSize(ascending bool) {
	videoDataGroups := vm.InterfaceToVideoData()
	defer vm.SetViewModelDuplicateGroups(videoDataGroups)

	sort.Slice(videoDataGroups, func(i, j int) bool {
		calculateTotalSize := func(group []*models.VideoData) int64 {
			uniqueInodeDeviceID := make(map[string]bool)
			uniquePaths := make(map[string]bool)
			var totalSize int64
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

		sizeI := calculateTotalSize(videoDataGroups[i])
		sizeJ := calculateTotalSize(videoDataGroups[j])

		if ascending {
			return sizeI < sizeJ
		}
		return sizeI > sizeJ
	})
}

func (vm *viewModel) SortVideosByTotalVideos(ascending bool) {
	videoDataGroups := vm.InterfaceToVideoData()

	countVideos := func(group []*models.VideoData) int {
		return len(group)
	}
	sort.Slice(videoDataGroups, func(i, j int) bool {
		if ascending {
			return countVideos(videoDataGroups[i]) < countVideos(videoDataGroups[j])
		}
		return countVideos(videoDataGroups[i]) > countVideos(videoDataGroups[j])
	})
}

// Selection / Deletion / Hardlinking
// _________________________________

func (vm *viewModel) DeleteSelectedFromList() {
	vm.mutex.Lock()

	selectedIDs := make(map[int64]struct{})
	for _, item := range vm.items {
		if item.Selected && item.VideoData != nil {
			selectedIDs[item.VideoData.Video.ID] = struct{}{}
		}
	}

	groups := vm.InterfaceToVideoData()
	for gi := range groups {
		var filtered []*models.VideoData
		for _, vd := range groups[gi] {
			if _, found := selectedIDs[vd.Video.ID]; !found {
				filtered = append(filtered, vd)
			}
		}
		groups[gi] = filtered
	}

	vm.SetViewModelDuplicateGroups(groups)

	// re-run flatten
	vm.items = vm.items[:0]
	vm.mutex.Unlock()
	vm.SetData(groups)
}

func (vm *viewModel) DeleteSelectedFromListDB() {
	vm.mutex.Lock()

	// collect all selected IDs
	selectedIDs := make([]int64, 0)
	for _, item := range vm.items {
		if item.Selected && item.VideoData != nil {
			selectedIDs = append(selectedIDs, item.VideoData.Video.ID)
		}
	}
	if len(selectedIDs) == 0 {
		slog.Info("No videos selected for DB deletion")
		return
	}

	if err := vm.Application.DeleteVideosByID(selectedIDs); err != nil {
		slog.Error("Failed to delete videos from DB", "error", err)
	} else {
		slog.Info("Successfully deleted selected videos from DB")
	}

	// remove selected ids from in-memory list
	groups := vm.InterfaceToVideoData()
	for gi := range groups {
		var videosToKeep []*models.VideoData
		for _, vd := range groups[gi] {

			selected := slices.Contains(selectedIDs, vd.Video.ID)
			if !selected {
				videosToKeep = append(videosToKeep, vd)
			}
		}
		groups[gi] = videosToKeep
	}

	vm.SetViewModelDuplicateGroups(groups)

	vm.items = vm.items[:0]
	vm.mutex.Unlock()
	vm.SetData(groups)
}

func (vm *viewModel) DeleteSelectedFromListDBDisk() {
	vm.mutex.Lock()

	selectedIDs := make([]int64, 0)
	// store a mapping of ID -> file path to remove from disk
	idToPath := make(map[int64]string)

	for _, item := range vm.items {
		if item.Selected && item.VideoData != nil {
			vid := item.VideoData.Video
			selectedIDs = append(selectedIDs, vid.ID)
			idToPath[vid.ID] = vid.Path
		}
	}
	if len(selectedIDs) == 0 {
		slog.Info("No videos selected for DB+Disk deletion")
		return
	}

	// delete each file from disk
	for _, videoID := range selectedIDs {
		path := idToPath[videoID]
		slog.Info("Deleting file from disk", "path", path)
		err := os.Remove(path)
		if err != nil {
			slog.Error("Failed to remove file from disk", "path", path, "error", err)
			return
		}
	}

	// delete from DB
	if err := vm.Application.DeleteVideosByID(selectedIDs); err != nil {
		slog.Error("Failed to delete videos from DB", "error", err)
	} else {
		slog.Info("Successfully deleted selected videos from DB")
	}

	// remove videos from the in-memory list
	groups := vm.InterfaceToVideoData()
	for gi := range groups {
		var videosToKeep []*models.VideoData
		for _, vd := range groups[gi] {

			selected := slices.Contains(selectedIDs, vd.Video.ID)
			if !selected {
				videosToKeep = append(videosToKeep, vd)
			}
		}
		groups[gi] = videosToKeep
	}

	vm.SetViewModelDuplicateGroups(groups)

	vm.items = vm.items[:0]
	vm.mutex.Unlock()
	vm.SetData(groups)
}

func (vm *viewModel) HardlinkVideos() error {
	vm.mutex.Lock()

	groups := vm.InterfaceToVideoData()
	if groups == nil {
		return errors.New("no video data loaded")
	}
	selectedIDs := make(map[int64]struct{})
	for _, item := range vm.items {
		if item.Selected && item.VideoData != nil {
			selectedIDs[item.VideoData.Video.ID] = struct{}{}
		}
	}

	for groupIdx, group := range groups {
		var selectedVideos []*models.Video
		for _, vd := range group {
			if _, ok := selectedIDs[vd.Video.ID]; ok {
				selectedVideos = append(selectedVideos, &vd.Video)
			}
		}
		if len(selectedVideos) < 2 {
			continue
		}

		source := selectedVideos[0].Path
		sourceInode := selectedVideos[0].Inode
		sourceDev := selectedVideos[0].Device
		baseDir := filepath.Dir(source)

		hardLinked := []*models.Video{selectedVideos[0]}
		nonHardlinked := []*models.Video{}
		failedRemovingHardlink := 0
		for i := 1; i < len(selectedVideos); i++ {
			// if videos are already hardlinked, add to slice to later
			// update the # of hardlinks if one occurs
			if selectedVideos[i].Device == sourceDev && selectedVideos[i].Inode == sourceInode {
				hardLinked = append(hardLinked, selectedVideos[i])
				slog.Info("Skipping hardlinking file, files are already hardlinked", "Path", selectedVideos[i].Path)
				continue
			}

			target := selectedVideos[i].Path
			tmpFilePath := fmt.Sprintf("%s/govdupes_%d.tmp", baseDir, time.Now().UnixNano())

			slog.Info("Creating hardlink",
				slog.String("source", source),
				slog.String("temp", tmpFilePath),
			)
			// atomic

			if err := os.Link(source, tmpFilePath); err != nil {
				slog.Error("Failed to create hardlink", "error", err)
				continue
			}
			if err := os.Rename(tmpFilePath, target); err != nil {
				slog.Error("Failed to rename hardlinked file", "error", err)

				err = os.Remove(tmpFilePath)
				if err != nil {
					slog.Error("Failed to delete tmp", "tmpfilepath", tmpFilePath, "error", err)
					failedRemovingHardlink++
					continue
				}
				slog.Info("Successfully deleted tmpfile", "tmpfilepath", tmpFilePath)
				continue
			}
			slog.Info("Hardlink success ->", slog.String("from", source), slog.String("to", target))
			nonHardlinked = append(nonHardlinked, selectedVideos[i])
		}

		// update NumHardLinks
		slog.Info("Finished processing group", slog.Int("idx", groupIdx))
		for _, v := range hardLinked {
			additionalHardlinks := uint64(len(nonHardlinked)) + uint64(failedRemovingHardlink)
			v.NumHardLinks += additionalHardlinks
		}

		// update video info for hardlinked videos
		for _, v := range nonHardlinked {
			updateVideoFields(v, selectedVideos[0], []string{"ID", "Path", "FileName"})
		}

		// Update videos in the database
		allVideos := append(hardLinked, nonHardlinked...)
		if err := vm.Application.VideoStore.UpdateVideos(context.Background(), allVideos); err != nil {
			slog.Error("Failed to update videos in database", "error", err)
			return fmt.Errorf("update videos in database: %w", err)
		}

	}

	vm.SetViewModelDuplicateGroups(groups)

	vm.items = vm.items[:0]
	vm.mutex.Unlock()
	vm.SetData(groups)

	return nil
}

func (vm *viewModel) SelectIdentical() {
	vm.ClearSelection()
	vm.mutex.Lock()
	defer vm.mutex.Unlock()

	groupedItems := make(map[int][]*models.DuplicateListItemViewModel)
	for i := range vm.items {
		item := vm.items[i]
		if item.IsColumnsHeader || item.IsGroupHeader || item.VideoData == nil {
			continue
		}
		groupedItems[item.GroupIndex] = append(groupedItems[item.GroupIndex], item)
	}

	for _, items := range groupedItems {
		identicalGroups := findIdenticalGroups(items)
		for _, group := range identicalGroups {
			if len(group) >= 2 {
				group[0].Selected = true
				group[1].Selected = true
			}
		}
	}
}

func findIdenticalGroups(items []*models.DuplicateListItemViewModel) [][]*models.DuplicateListItemViewModel {
	identicalMap := make(map[string][]*models.DuplicateListItemViewModel)
	for _, item := range items {
		key := generateKeyExcludingPath(item.VideoData)
		identicalMap[key] = append(identicalMap[key], item)
	}

	var result [][]*models.DuplicateListItemViewModel
	for _, g := range identicalMap {
		if len(g) > 1 {
			result = append(result, g)
		}
	}
	return result
}

func generateKeyExcludingPath(videoData *models.VideoData) string {
	if videoData == nil {
		return ""
	}
	return fmt.Sprintf("%d|%v|%s|%s|%.2f",
		videoData.Video.Size,
		videoData.Video.Duration,
		videoData.Video.VideoCodec,
		videoData.Video.AudioCodec,
		videoData.Video.AvgFrameRate,
	)
}

func (vm *viewModel) SelectAllButLargest() {
	vm.ClearSelection()
	vm.mutex.Lock()
	defer vm.mutex.Unlock()

	groupMaxSize := make(map[int]int64)
	for _, item := range vm.items {
		if item.VideoData == nil {
			continue
		}
		sz := item.VideoData.Video.Size
		if sz > groupMaxSize[item.GroupIndex] {
			groupMaxSize[item.GroupIndex] = sz
		}
	}
	for i := range vm.items {
		item := vm.items[i]
		if item.VideoData != nil && !item.IsGroupHeader && !item.IsColumnsHeader {
			if item.VideoData.Video.Size < groupMaxSize[item.GroupIndex] {
				item.Selected = true
			}
		}
	}
}

func (vm *viewModel) SelectAllButSmallest() {
	vm.ClearSelection()
	vm.mutex.Lock()
	defer vm.mutex.Unlock()

	groupMinSize := make(map[int]int64)
	for i := range vm.items {
		if vm.items[i].VideoData != nil {
			if _, ok := groupMinSize[vm.items[i].GroupIndex]; !ok {
				groupMinSize[vm.items[i].GroupIndex] = 1<<63 - 1
			}
		}
	}
	for _, item := range vm.items {
		if item.VideoData == nil {
			continue
		}
		sz := item.VideoData.Video.Size
		if sz < groupMinSize[item.GroupIndex] {
			groupMinSize[item.GroupIndex] = sz
		}
	}
	for i := range vm.items {
		item := vm.items[i]
		if item.VideoData != nil && !item.IsGroupHeader && !item.IsColumnsHeader {
			if item.VideoData.Video.Size > groupMinSize[item.GroupIndex] {
				item.Selected = true
			}
		}
	}
}

func (vm *viewModel) SelectAllButNewest() {
	vm.ClearSelection()
	vm.mutex.Lock()
	defer vm.mutex.Unlock()

	groupMaxModified := make(map[int]time.Time)
	for _, item := range vm.items {
		if item.VideoData == nil {
			continue
		}
		t := item.VideoData.Video.ModifiedAt
		if t.After(groupMaxModified[item.GroupIndex]) {
			groupMaxModified[item.GroupIndex] = t
		}
	}
	for i := range vm.items {
		item := vm.items[i]
		if item.VideoData == nil || item.IsGroupHeader || item.IsColumnsHeader {
			continue
		}
		if item.VideoData.Video.ModifiedAt.Before(groupMaxModified[item.GroupIndex]) {
			item.Selected = true
		}
	}
}

func (vm *viewModel) SelectAllButOldest() {
	vm.ClearSelection()
	vm.mutex.Lock()
	defer vm.mutex.Unlock()

	groupMinModified := make(map[int]time.Time)
	for _, item := range vm.items {
		if item.VideoData != nil {
			if _, ok := groupMinModified[item.GroupIndex]; !ok {
				groupMinModified[item.GroupIndex] = time.Date(3000, 1, 1, 0, 0, 0, 0, time.UTC)
			}
		}
	}
	for _, item := range vm.items {
		if item.VideoData == nil {
			continue
		}
		t := item.VideoData.Video.ModifiedAt
		if t.Before(groupMinModified[item.GroupIndex]) {
			groupMinModified[item.GroupIndex] = t
		}
	}
	for i := range vm.items {
		item := vm.items[i]
		if item.VideoData != nil && !item.IsGroupHeader && !item.IsColumnsHeader {
			if item.VideoData.Video.ModifiedAt.After(groupMinModified[item.GroupIndex]) {
				item.Selected = true
			}
		}
	}
}

func (vm *viewModel) SelectAllButHighestBitrate() {
	vm.ClearSelection()
	vm.mutex.Lock()
	defer vm.mutex.Unlock()

	groupMaxBitrate := make(map[int]int)
	for _, item := range vm.items {
		if item.VideoData == nil {
			continue
		}
		br := item.VideoData.Video.BitRate
		if br > groupMaxBitrate[item.GroupIndex] {
			groupMaxBitrate[item.GroupIndex] = br
		}
	}
	for i := range vm.items {
		item := vm.items[i]
		if item.VideoData != nil && !item.IsGroupHeader && !item.IsColumnsHeader {
			if item.VideoData.Video.BitRate < groupMaxBitrate[item.GroupIndex] {
				item.Selected = true
			}
		}
	}
}

func (vm *viewModel) SelectAll() {
	vm.ClearSelection()
	vm.mutex.Lock()
	defer vm.mutex.Unlock()

	for i := range vm.items {
		if vm.items[i].VideoData != nil &&
			!vm.items[i].IsGroupHeader &&
			!vm.items[i].IsColumnsHeader {
			vm.items[i].Selected = true
		}
	}
}

// Setters / Getters for bindings
// _____________________________

func (vm *viewModel) UpdateFileCount(count string) {
	if err := vm.FileCount.Set(count); err != nil {
		slog.Error("Failed to update FileCount", slog.Any("error", err))
	}
}

func (vm *viewModel) UpdateAcceptedFiles(count string) {
	if err := vm.AcceptedFiles.Set(count); err != nil {
		slog.Error("Failed to update AcceptedFiles", slog.Any("error", err))
	}
}

func (vm *viewModel) UpdateGetFileInfoProgress(progress float64) {
	if err := vm.GetFileInfoProgress.Set(progress); err != nil {
		slog.Error("Failed to update GetFileInfoProgress", slog.Any("error", err))
	}
}

func (vm *viewModel) UpdateGenPHashesProgress(progress float64) {
	if err := vm.GenPHashesProgress.Set(progress); err != nil {
		slog.Error("Failed to update GenPHashesProgress", slog.Any("error", err))
	}
}

func (vm *viewModel) GetFileCountBind() binding.String {
	return vm.FileCount
}

func (vm *viewModel) GetAcceptedFilesBind() binding.String {
	return vm.AcceptedFiles
}

func (vm *viewModel) GetFileInfoProgressBind() binding.Float {
	return vm.GetFileInfoProgress
}

func (vm *viewModel) GetPHashesProgressBind() binding.Float {
	return vm.GenPHashesProgress
}

func (vm *viewModel) GetTotalGroupSizeBind() binding.String {
	return vm.TotalGroupSize
}

func (vm *viewModel) GetPotentialSpaceSavingsBind() binding.String {
	return vm.PotentialSpaceSavings
}

func (vm *viewModel) AddDuplicateGroupsListener(listener binding.DataListener) {
	vm.DuplicateGroups.AddListener(listener)
}

func formatFileSize(sizeBytes int64) string {
	const (
		MB = 1024.0 * 1024.0
		GB = 1024.0 * 1024.0 * 1024.0
	)
	gbVal := float64(sizeBytes) / GB
	if gbVal >= 1.0 {
		return fmt.Sprintf("%.2f GB", gbVal)
	}
	mbVal := float64(sizeBytes) / MB
	return fmt.Sprintf("%.2f MB", mbVal)
}

func (vm *viewModel) SetDuplicateGroups(groups []any) error {
	return vm.DuplicateGroups.Set(groups)
}

func (vm *viewModel) SetViewModelDuplicateGroups(v [][]*models.VideoData) {
	vm.UpdateStatistics(v)
	// Convert to a []interface{} to set the UntypedList
	items := make([]any, len(v))
	for i, grp := range v {
		items[i] = grp // []*models.VideoData
	}
	err := vm.SetDuplicateGroups(items)
	if err != nil {
		slog.Error("Setting vm.SetDuplicateGroups", "Error", err)
	}
	// ensure DuplicateGroups gets updated (sorting won't cause an update)
	vm.DuplicateGroups.Append(struct{}{})
	vm.DuplicateGroups.Remove(struct{}{})
}

func (vm *viewModel) ExportToJSON(path string) error {
	vm.mutex.RLock()
	defer vm.mutex.RUnlock()

	duplicateGroups := vm.InterfaceToVideoData()
	if duplicateGroups == nil {
		return fmt.Errorf("no duplicate video data to export")
	}

	jsonBytes, err := json.MarshalIndent(duplicateGroups, "", "  ")
	if err != nil {
		slog.Error("Failed to marshal duplicate video data to JSON", "error", err)
		return err
	}

	err = os.WriteFile(path, jsonBytes, 0o644)
	if err != nil {
		slog.Error("Failed to write JSON file", "path", path, "error", err)
		return err
	}

	slog.Info("Exported duplicates to JSON", "path", path)
	return nil
}

// CopyStructFields copies fields from src to dst, skipping specified fields.
func updateVideoFields(dst, src *models.Video, skipFields []string) {
	dstVal := reflect.ValueOf(dst).Elem()
	srcVal := reflect.ValueOf(src).Elem()
	skipMap := make(map[string]struct{}, len(skipFields))
	for _, field := range skipFields {
		skipMap[field] = struct{}{}
	}

	for i := range dstVal.NumField() {
		fieldName := dstVal.Type().Field(i).Name
		if _, skip := skipMap[fieldName]; skip {
			continue
		}
		dstVal.Field(i).Set(srcVal.Field(i))
	}
}

func (vm *viewModel) UpdateStatistics(groups [][]*models.VideoData) {
	totalGroupSize := int64(0)
	potentialSavings := int64(0)

	for _, group := range groups {
		uniqueVideos := make(map[string]*models.Video)
		groupSize := int64(0)

		// calculate group size and avoid double-counting hardlinked files
		largestFileSize := int64(-2)
		for _, vd := range group {
			identifier := fmt.Sprintf("%d:%d", vd.Video.Inode, vd.Video.Device)
			if _, exists := uniqueVideos[identifier]; !exists {
				uniqueVideos[identifier] = &vd.Video
				groupSize += vd.Video.Size
			}
			if vd.Video.Size > largestFileSize {
				largestFileSize = vd.Video.Size
			}
		}

		// subtract the largest video size from duplicatesSize
		// (assumes user wil keep the largest size, not perfect but...)
		// size, inode, dev #a 1, 1, 2 #b 1,1,2 #c 2,3,4
		// group size: 3, if you save #c, saved size = 1 unit
		potentialSavings += groupSize - largestFileSize
		totalGroupSize += groupSize
	}

	if err := vm.TotalGroupSize.Set(formatFileSize(totalGroupSize)); err != nil {
		slog.Error("Failed to update TotalGroupSize", "error", err)
	}
	if err := vm.PotentialSpaceSavings.Set(formatFileSize(potentialSavings)); err != nil {
		slog.Error("Failed to update PotentialSpaceSavings", "error", err)
	}
}

func (vm *viewModel) ResetSearchBindings() {
	/*
		FileCount             binding.String
		AcceptedFiles         binding.String
		GetFileInfoProgress   binding.Float
		GenPHashesProgress    binding.Float
		TotalGroupSize        binding.String
		PotentialSpaceSavings binding.String
	*/

	if err := vm.FileCount.Set("0"); err != nil {
		slog.Error("Failed to reset FileCount", "error", err)
	}
	if err := vm.AcceptedFiles.Set("0"); err != nil {
		slog.Error("Failed to reset AcceptedFiles", "error", err)
	}
	if err := vm.GetFileInfoProgress.Set(0); err != nil {
		slog.Error("Failed to reset GetFileInfoProgress", "error", err)
	}
	if err := vm.GenPHashesProgress.Set(0); err != nil {
		slog.Error("Failed to reset GenPHashesProgress", "error", err)
	}
	if err := vm.TotalGroupSize.Set(""); err != nil {
		slog.Error("Failed to reset TotalGroupSize", "error", err)
	}
	if err := vm.PotentialSpaceSavings.Set(""); err != nil {
		slog.Error("Failed to reset PotentialSpaceSavings", "error", err)
	}
}
