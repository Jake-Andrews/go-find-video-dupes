package ui

import (
	"fmt"
	"image/color"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"govdupes/internal/models"
)

// forcedVariant forces dark/light theme.
type forcedVariant struct {
	fyne.Theme
	isDark bool
}

func (f *forcedVariant) Color(n fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	if f.isDark {
		return f.Theme.Color(n, theme.VariantDark)
	}
	return f.Theme.Color(n, theme.VariantLight)
}

func CreateUI(videoData [][]*models.VideoData) {
	log.Println("Starting CreateUI")

	a := app.New()
	window := a.NewWindow("sneed")
	log.Println("Fyne app initialized")

	log.Println("Creating DuplicatesList widget")
	duplicatesListWidget := NewDuplicatesList(videoData)
	if duplicatesListWidget == nil {
		log.Fatal("Failed to create DuplicatesList widget")
	}

	log.Println("Wrapping DuplicatesList in a scroll container")
	scroll := container.NewVScroll(duplicatesListWidget)
	scroll.SetMinSize(fyne.NewSize(1024, 768))

	duplicatesTab := scroll
	themeTab := buildThemeTab(a)

	tabs := container.NewAppTabs(
		container.NewTabItem("Duplicates", duplicatesTab),
		container.NewTabItem("Theme", themeTab),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	log.Println("Creating main application window")
	window.SetContent(tabs)
	window.Resize(fyne.NewSize(1024, 768))

	log.Println("Showing application window")
	window.ShowAndRun()
}

// A tab that lets the user switch between dark/light themes.
func buildThemeTab(a fyne.App) fyne.CanvasObject {
	return container.NewGridWithColumns(2,
		widget.NewButton("Dark", func() {
			a.Settings().SetTheme(&forcedVariant{Theme: theme.DefaultTheme(), isDark: true})
		}),
		widget.NewButton("Light", func() {
			a.Settings().SetTheme(&forcedVariant{Theme: theme.DefaultTheme(), isDark: false})
		}),
	)
}

// buildDuplicatesTab creates a scrollable view containing each group of duplicates.
func buildDuplicatesTab(videoData [][]*models.VideoData) fyne.CanvasObject {
	if len(videoData) == 0 {
		return widget.NewLabel("No duplicate videos found.")
	}

	mainContainer := container.NewVBox()

	// Iterate each duplicate group
	for i, group := range videoData {
		if len(group) == 0 {
			continue
		}
		// Add a label for the group
		title := widget.NewLabelWithStyle(
			fmt.Sprintf("Group %d (Total %d duplicates)", i+1, len(group)),
			fyne.TextAlignCenter,
			fyne.TextStyle{Bold: true},
		)
		mainContainer.Add(title)

		// Build a grid for the group
		grid := container.NewGridWithColumns(5)

		// Add headers
		headers := []string{
			"Screenshots",
			"Path",
			"Size / Bitrate / FPS / Res / Duration",
			"Audio / Video",
			"Symbolic / Hard Links",
		}
		for _, header := range headers {
			headerLabel := widget.NewLabelWithStyle(header, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
			grid.Add(headerLabel)
		}

		// Add rows for each video in the group
		for _, vd := range group {
			grid.Add(buildScreenshotCell(vd))
			grid.Add(widget.NewLabel(vd.Video.Path))
			grid.Add(buildStatsCell(vd))
			grid.Add(widget.NewLabel(fmt.Sprintf("%s / %s", vd.Video.AudioCodec, vd.Video.VideoCodec)))
			grid.Add(buildLinksCell(vd))
		}

		// Add the grid to the main container
		mainContainer.Add(grid)
		mainContainer.Add(widget.NewSeparator())
	}

	// Wrap everything in a scroll container
	scrollable := container.NewVScroll(mainContainer)
	scrollable.SetMinSize(fyne.NewSize(1024, 768)) // Ensure the main view is large enough

	return scrollable
}

// buildScreenshotCell centers all screenshots (if any) vertically & horizontally.
func buildScreenshotCell(vd *models.VideoData) fyne.CanvasObject {
	if len(vd.Screenshot.Screenshots) == 0 {
		return widget.NewLabel("No screenshots")
	}
	// If multiple screenshots, stack them vertically.
	box := container.NewVBox()
	for _, img := range vd.Screenshot.Screenshots {
		fImg := canvas.NewImageFromImage(img)
		fImg.FillMode = canvas.ImageFillContain
		fImg.SetMinSize(fyne.NewSize(100, 100))
		box.Add(container.NewCenter(fImg))
	}
	return box
}

// buildStatsCell shows size, bitrate, fps, resolution, duration vertically.
func buildStatsCell(vd *models.VideoData) fyne.CanvasObject {
	v := vd.Video
	sizeMB := float64(v.Size) / (1024.0 * 1024.0)
	bitrateMbps := (float64(v.BitRate) / (1024.0 * 1024.0)) * 8.0
	dur := int(v.Duration)
	hh, mm, ss := dur/3600, (dur%3600)/60, dur%60

	return container.NewVBox(
		widget.NewLabel(fmt.Sprintf("%.2f MB", sizeMB)),
		widget.NewLabel(fmt.Sprintf("%.2f Mbps", bitrateMbps)),
		widget.NewLabel(fmt.Sprintf("%.2f fps", v.FrameRate)),
		widget.NewLabel(fmt.Sprintf("%dx%d", v.Width, v.Height)),
		widget.NewLabel(fmt.Sprintf("%02d:%02d:%02d", hh, mm, ss)),
	)
}

// buildLinksCell shows symbolic/hard link info.
func buildLinksCell(vd *models.VideoData) fyne.CanvasObject {
	v := vd.Video
	numHard := v.NumHardLinks - 1 // subtract 1 to exclude the file itself
	return container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Symbolic? %t", v.IsSymbolicLink)),
		widget.NewLabel(fmt.Sprintf("Link: %q", v.SymbolicLink)),
		widget.NewLabel(fmt.Sprintf("Hard? %t", v.IsHardLink)),
		widget.NewLabel(fmt.Sprintf("Count: %d", numHard)),
	)
}

