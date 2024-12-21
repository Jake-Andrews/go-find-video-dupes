package main

import (
	"fmt"
	"govdupes/models"
	"log"
	"sort"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type DuplicateGroup [][]models.Video

func createVideoCard(video models.Video) *widget.Card {
	content := fmt.Sprintf(
		"Path: %s\nFileName: %s\nModifiedAt: %s\nVideoCodec: %s\nAudioCodec: %s\n"+
			"Resolution: %dx%d\nDuration: %.3f seconds\nSize: %d bytes\nBitRate: %d\n"+
			"HardLinks: %d | SymbolicLink: %v\nInode: %d | Device: %d",
		video.Path, video.FileName, video.ModifiedAt.Format("2006-01-02 15:04:05"), video.VideoCodec, video.AudioCodec,
		video.Width, video.Height, video.Duration, video.Size, video.BitRate,
		video.NumHardLinks, video.IsSymbolicLink, video.Inode, video.Device,
	)
	return widget.NewCard(
		fmt.Sprintf("Video ID: %d", video.VideoID),
		"Video Details",
		widget.NewLabel(content),
	)
}

func sortVideosBySize(videos []models.Video) {
	sort.SliceStable(videos, func(i, j int) bool {
		return videos[i].Size > videos[j].Size
	})
}

func sortVideosByBitrate(videos []models.Video) {
	sort.SliceStable(videos, func(i, j int) bool {
		return videos[i].BitRate > videos[j].BitRate
	})
}

func renderDuplicateGroups(groups DuplicateGroup, container *fyne.Container) {
	container.Objects = nil // Clear previous content
	for _, group := range groups {
		separator := widget.NewSeparator() // Add separator for clarity
		container.Objects = append(container.Objects, separator)

		// Group container
		groupContainer := &fyne.Container{
			Layout:  layout.NewVBoxLayout(),
			Objects: []fyne.CanvasObject{},
		}

		for _, video := range group {
			card := createVideoCard(video)
			groupContainer.Objects = append(groupContainer.Objects, card)
		}

		container.Objects = append(container.Objects, groupContainer)
	}
	container.Refresh()
}

func main() {
	dbPath := "../"
	duplicateGroups, err := models.GetDuplicateGroupsFromDB(dbPath)
	if err != nil {
		log.Fatalf("Error reading from database: %v", err)
	}

	a := app.NewWithID("govdupes")
	w := a.NewWindow("Duplicate Videos Viewer")
	w.Resize(fyne.NewSize(800, 600))

	// Top-level container for all videos
	allVideosContainer := &fyne.Container{
		Layout: layout.NewVBoxLayout(),
	}

	scrollableContainer := container.NewScroll(allVideosContainer)

	var mutex sync.Mutex // Mutex for thread-safe updates
	renderDuplicateGroups(duplicateGroups, allVideosContainer)

	// Menu actions
	sizeAction := func() {
		mutex.Lock()
		defer mutex.Unlock()
		for _, group := range duplicateGroups {
			sortVideosBySize(group)
		}
		renderDuplicateGroups(duplicateGroups, allVideosContainer)
	}

	bitrateAction := func() {
		mutex.Lock()
		defer mutex.Unlock()
		for _, group := range duplicateGroups {
			sortVideosByBitrate(group)
		}
		renderDuplicateGroups(duplicateGroups, allVideosContainer)
	}

	// Create menu
	menu := fyne.NewMenu("Sort",
		fyne.NewMenuItem("By Size", sizeAction),
		fyne.NewMenuItem("By Bitrate", bitrateAction),
	)
	w.SetMainMenu(fyne.NewMainMenu(menu))

	w.SetContent(scrollableContainer)
	w.ShowAndRun()
}
