// ui.go
package ui

import (
	"image/color"
	"log/slog"
	"os"

	"govdupes/internal/config"
	"govdupes/internal/models"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func CreateUI(videoData [][]*models.VideoData) {
	slog.Info("Starting CreateUI")

	a := app.New()
	cfg := &config.Config{}
	cfg.SetDefaults()

	// copy of the original data so we can re-filter repeatedly hacky
	originalVideoData := videoData

	duplicatesListWidget := NewDuplicatesList(videoData)
	if duplicatesListWidget == nil {
		slog.Error("Failed to create DuplicatesList widget")
		os.Exit(1)
	}
	duplicatesTab := container.NewVScroll(duplicatesListWidget)
	duplicatesTab.SetMinSize(fyne.NewSize(1200, 768))

	themeTab := buildThemeTab(a)
	sortSelectTab := buildSortSelectDeleteTab(duplicatesListWidget, videoData)
	filterForm, checkWidget := buildFilter(duplicatesListWidget, originalVideoData)
	configTab := buildConfigTab(cfg, checkWidget)

	searchBtn := widget.NewButtonWithIcon("Search", theme.Icon(theme.IconNameSearch), func() { slog.Info("tapped") })
	searchTab := container.NewBorder(searchBtn, nil, nil, nil)

	// Tabs section
	tabs := container.NewAppTabs(
		container.NewTabItem("Search", searchTab),
		container.NewTabItem("Duplicates", duplicatesTab),
		container.NewTabItem("Theme", themeTab),
		container.NewTabItem("Sort/Select/Delete", sortSelectTab),
		container.NewTabItem("Settings", configTab),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	// Main content including tabs and filter box
	mainContent := container.NewVBox(
		tabs,
		filterForm,
	)

	// Main window setup
	window := a.NewWindow("govdupes")
	window.SetContent(mainContent)
	window.SetMaster()
	window.Resize(fyne.NewSize(1300, 900))
	window.ShowAndRun()
}

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
