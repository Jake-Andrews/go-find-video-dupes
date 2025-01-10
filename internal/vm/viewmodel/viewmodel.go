package viewmodel

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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
	items           []models.DuplicateListItemViewModel
	DuplicateGroups binding.UntypedList
	mutex           sync.RWMutex

	FileCount           binding.String
	AcceptedFiles       binding.String
	GetFileInfoProgress binding.Float
	GenPHashesProgress  binding.Float

	Application *application.App
}

func NewViewModel(app *application.App) vm.ViewModel {
	return &viewModel{
		DuplicateGroups:     binding.NewUntypedList(),
		items:               make([]models.DuplicateListItemViewModel, 0),
		FileCount:           binding.NewString(),
		AcceptedFiles:       binding.NewString(),
		GetFileInfoProgress: binding.NewFloat(),
		GenPHashesProgress:  binding.NewFloat(),
		Application:         app,
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

	// Check if we have at least one non-empty group
	hasAnyVideos := false
	for _, group := range videoData {
		if len(group) > 1 {
			hasAnyVideos = true
			break
		}
	}

	// Filter out groups that have 0 or 1 videos
	var filteredGroups [][]*models.VideoData
	for _, group := range videoData {
		if len(group) > 1 {
			filteredGroups = append(filteredGroups, group)
		}
	}

	// Add columns header row if we have any
	if hasAnyVideos {
		vm.items = append(vm.items, models.DuplicateListItemViewModel{
			IsColumnsHeader: true,
		})
	}

	// For each group, add group header + one row per video
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

		vm.items = append(vm.items, models.DuplicateListItemViewModel{
			IsGroupHeader: true,
			GroupIndex:    i,
			HeaderText:    groupHeaderText,
		})

		for _, vd := range group {
			vm.items = append(vm.items, models.DuplicateListItemViewModel{
				GroupIndex: i,
				VideoData:  vd,
			})
		}
	}
}

// GetItems safely returns a copy of vm.items for the View to read.
func (vm *viewModel) GetItems() []models.DuplicateListItemViewModel {
	vm.mutex.RLock()
	defer vm.mutex.RUnlock()
	itemsCopy := make([]models.DuplicateListItemViewModel, len(vm.items))
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
	item := &vm.items[itemIndex]
	if item.IsColumnsHeader || item.IsGroupHeader {
		return
	}
	item.Selected = selected
}

// ClearSelection sets Selected=false on all rows.
func (vm *viewModel) ClearSelection() {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	for i := range vm.items {
		vm.items[i].Selected = false
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

func rowMatchesQuery(it models.DuplicateListItemViewModel, query models.SearchQuery) bool {
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
	videoDataGroups := vm.InterfaceToVideoData()

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
	defer vm.mutex.Unlock()

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

	// Write back
	newList := make([]interface{}, 0, len(groups))
	for _, g := range groups {
		newList = append(newList, g)
	}
	_ = vm.DuplicateGroups.Set(newList)

	// Re-run flatten
	vm.items = vm.items[:0]
	vm.SetData(groups)
}

func (vm *viewModel) HardlinkVideos() error {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()

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
		baseDir := filepath.Dir(source)

		for i := 1; i < len(selectedVideos); i++ {
			target := selectedVideos[i].Path
			tmpFilePath := fmt.Sprintf("%s/govdupes_%d.tmp", baseDir, time.Now().UnixNano())

			slog.Info("Creating hardlink",
				slog.String("source", source),
				slog.String("temp", tmpFilePath),
			)

			if err := os.Link(source, tmpFilePath); err != nil {
				slog.Error("Failed to create hardlink", "error", err)
				continue
			}
			if err := os.Rename(tmpFilePath, target); err != nil {
				slog.Error("Failed to rename hardlinked file", "error", err)
				_ = os.Remove(tmpFilePath)
				continue
			}
			slog.Info("Hardlink success ->", slog.String("from", source), slog.String("to", target))
		}
		slog.Info("Finished processing group", slog.Int("idx", groupIdx))
	}

	return nil
}

func (vm *viewModel) SelectIdentical() {
	vm.ClearSelection()
	vm.mutex.Lock()
	defer vm.mutex.Unlock()

	groupedItems := make(map[int][]*models.DuplicateListItemViewModel)
	for i := range vm.items {
		item := &vm.items[i]
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
		item := &vm.items[i]
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
		item := &vm.items[i]
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
		item := &vm.items[i]
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
		item := &vm.items[i]
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
		item := &vm.items[i]
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

func (vm *viewModel) SetDuplicateGroups(groups []interface{}) error {
	return vm.DuplicateGroups.Set(groups)
}
