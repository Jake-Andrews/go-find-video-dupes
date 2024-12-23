package ui

import (
	"fmt"
	"image"

	"govdupes/internal/models"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// CreateUI is the entry point for building the Fyne UI. videoData is a slice
// of slices of *models.VideoData. Each inner slice represents a group of
// duplicate videos.
func CreateUI(videoData [][]*models.VideoData) fyne.CanvasObject {
	if len(videoData) == 0 {
		return widget.NewLabel("No duplicate videos found.")
	}

	// A vertical box to hold all groups
	groupsContainer := container.NewVBox()

	// Iterate over each group (slice of duplicates)
	for i, group := range videoData {
		if len(group) == 0 {
			// Skip empty groups just in case
			continue
		}

		// Create a container for this group
		groupContainer := container.NewVBox()

		// Optionally, label each group (e.g., "Group #1")
		title := widget.NewLabel(fmt.Sprintf("Group %d (Total %d duplicates)", i+1, len(group)))
		title.TextStyle.Bold = true
		groupContainer.Add(title)

		// For each video in the group, create a row
		for _, vd := range group {
			if vd == nil {
				continue
			}
			row := createVideoRow(vd)
			groupContainer.Add(row)
		}

		// Add some vertical space and a separator to visually separate groups
		groupContainer.Add(widget.NewLabel(""))   // Blank row
		groupContainer.Add(widget.NewSeparator()) // Separator line

		// Add the fully built group container to the parent container
		groupsContainer.Add(groupContainer)
	}

	// Wrap the groups in a scroll container for when content grows large
	scrollable := container.NewVScroll(groupsContainer)
	scrollable.SetMinSize(fyne.NewSize(800, 600)) // example minimum size

	return scrollable
}

// createVideoRow constructs a single row (horizontal layout with 5 columns)
// to show data from one VideoData item.
func createVideoRow(vd *models.VideoData) fyne.CanvasObject {
	video := vd.Video

	// 1) Screenshots
	screenshotsContainer := createScreenshotsContainer(vd.Screenshot.Screenshots)

	// 2) Path
	pathLabel := widget.NewLabel(video.Path)
	pathLabel.Wrapping = fyne.TextWrapBreak

	// 3) Combined stats (Size, Bitrate, Framerate, Width, Height, Duration)
	stats := fmt.Sprintf(
		"Size: %d bytes\nBitrate: %d kbps\nFramerate: %.2f fps\nWidth x Height: %dx%d\nDuration: %.2f s",
		video.Size, video.BitRate, video.FrameRate, video.Width, video.Height, video.Duration,
	)
	statsLabel := widget.NewLabel(stats)
	statsLabel.Wrapping = fyne.TextWrapBreak

	// 4) Codecs (Audio, Video)
	codecs := fmt.Sprintf("Audio: %s\nVideo: %s", video.AudioCodec, video.VideoCodec)
	codecsLabel := widget.NewLabel(codecs)
	codecsLabel.Wrapping = fyne.TextWrapBreak

	// 5) Links (IsSymbolicLink, IsHardLink)
	linkInfo := fmt.Sprintf("SymbolicLink: %t\nHardLink: %t", video.IsSymbolicLink, video.IsHardLink)
	linkLabel := widget.NewLabel(linkInfo)
	linkLabel.Wrapping = fyne.TextWrapBreak

	// Create a horizontal row with 5 columns
	row := container.New(layout.NewAdaptiveGridLayout(5),
		screenshotsContainer,
		pathLabel,
		statsLabel,
		codecsLabel,
		linkLabel,
	)

	return row
}

// createScreenshotsContainer places each screenshot in a horizontal layout.
// If there are no screenshots, a placeholder label is shown instead.
func createScreenshotsContainer(imgs []image.Image) fyne.CanvasObject {
	if len(imgs) == 0 {
		return widget.NewLabel("No screenshots")
	}

	var imageWidgets []fyne.CanvasObject
	for _, img := range imgs {
		// Convert Go image.Image to a Fyne canvas Image
		fImg := canvas.NewImageFromImage(img)
		fImg.FillMode = canvas.ImageFillContain
		// You can optionally set a min size to keep them uniform
		fImg.SetMinSize(fyne.NewSize(120, 80))
		imageWidgets = append(imageWidgets, fImg)
	}

	return container.NewHBox(imageWidgets...)
}
