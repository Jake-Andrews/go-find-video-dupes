package ui

import (
	"image/color"
	"log"
	"sort"

	"govdupes/internal/models"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func CreateUI(videoData [][]*models.VideoData) {
	log.Println("Starting CreateUI")

	a := app.New()
	log.Println("Fyne app initialized")

	duplicatesListWidget := NewDuplicatesList(videoData)
	if duplicatesListWidget == nil {
		log.Fatal("Failed to create DuplicatesList widget")
	}

	scroll := container.NewVScroll(duplicatesListWidget)
	scroll.SetMinSize(fyne.NewSize(1024, 768))

	duplicatesTab := scroll
	themeTab := buildThemeTab(a)
	sortSelectTab := buildSortSelectTab(duplicatesListWidget, videoData)

	tabs := container.NewAppTabs(
		container.NewTabItem("Duplicates", duplicatesTab),
		container.NewTabItem("Theme", themeTab),
		container.NewTabItem("Sort/Select", sortSelectTab),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	log.Println("Creating main application window")
	window := a.NewWindow("govdupes")
	window.SetContent(tabs)
	window.Resize(fyne.NewSize(1024, 900))

	log.Println("Showing application window")
	window.ShowAndRun()
}

// buildThemeTab is a simple tab that lets the user switch
// between dark/light themes.
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

// forcedVariant forces dark or light theme
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

func buildSortSelectTab(duplicatesList *DuplicatesList, videoData [][]*models.VideoData) fyne.CanvasObject {
	sortOptions := []string{"Size", "Bitrate", "Resolution"}
	sortLabel := widget.NewLabel("Sort")
	dropdown := widget.NewSelect(sortOptions, nil) // Selected option will be handled on button press
	dropdown.PlaceHolder = "Select an option"

	// Track the sorting order for each criterion
	sortOrder := map[string]bool{
		"Size":       true, // true = ascending, false = descending
		"Bitrate":    true,
		"Resolution": true,
	}

	// Sort button triggers the actual sorting
	sortButton := widget.NewButton("Sort", func() {
		if dropdown.Selected == "" {
			return // Do nothing if no option is selected
		}

		switch dropdown.Selected {
		case "Size":
			if sortOrder["Size"] {
				sortVideosBySize(videoData, true) // Ascending
			} else {
				sortVideosBySize(videoData, false) // Descending
			}
			sortOrder["Size"] = !sortOrder["Size"] // Toggle order
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
		}

		// Refresh the duplicates list with the sorted data
		duplicatesList.SetData(videoData)
	})

	// Combine label, dropdown, and button into a vertical layout
	content := container.NewVBox(sortLabel, dropdown, sortButton)
	return content
}
