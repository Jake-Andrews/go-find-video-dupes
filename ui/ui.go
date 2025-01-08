// ui.go
package ui

import (
	"image/color"
	"log/slog"
	"os"
	"time"

	"govdupes/internal/application"
	"govdupes/internal/models"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func CreateUI(application *application.App) {
	slog.Info("Starting CreateUI")

	// fyne app
	a := app.New()
	window := a.NewWindow("govdupes")

	// Copy of the original data so we can re-filter repeatedly hacky
	videoData := [][]*models.VideoData{}
	originalVideoData := videoData

	// Video rows
	duplicatesListWidget := NewDuplicatesList(videoData)
	if duplicatesListWidget == nil {
		slog.Error("Failed to create DuplicatesList widget")
		os.Exit(1)
	}
	duplicatesTab := container.NewVScroll(duplicatesListWidget)
	duplicatesTab.SetMinSize(fyne.NewSize(1200, 768))

	// Tabs
	themeTab := buildThemeTab(a)
	sortSelectTab := buildSortSelectDeleteTab(duplicatesListWidget, videoData)
	filterForm, checkWidget := buildFilter(duplicatesListWidget, originalVideoData)
	configTab := buildConfigTab(application.Config, checkWidget)
	searchTab := buildSearchTab(application, duplicatesListWidget, videoData, window)

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

func buildSearchTab(a *application.App, duplicatesListWidget *DuplicatesList, videoData [][]*models.VideoData, w fyne.Window) *fyne.Container {
	searchBtn := container.NewCenter(widget.NewButtonWithIcon("Search", theme.Icon(theme.IconNameSearch), func() {
		slog.Info("Search started!")

		clockWidget := widget.NewLabel("")

		prop := canvas.NewRectangle(color.Transparent)
		prop.SetMinSize(fyne.NewSize(50, 50))
		d := dialog.NewCustomWithoutButtons("Searching...", container.NewStack(prop, clockWidget), w)

		c := Clock{}
		c.set()
		d.Show()

		stopChan := make(chan struct{})
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					c.update(clockWidget)
				case <-stopChan:
					return
				}
			}
		}()

		if vData := a.Search(); vData != nil {
			videoData = vData
		} else {
			videoData = [][]*models.VideoData{}
		}
		close(stopChan)
		duplicatesListWidget.SetData(videoData)

		d.Hide()
	}))

	searchTab := container.NewBorder(searchBtn, nil, nil, nil)

	return searchTab
}

type Clock struct {
	t time.Time
}

func (c *Clock) set() {
	c.t = time.Now()
}

func (c *Clock) update(clock *widget.Label) {
	tElapsed := time.Since(c.t)
	tStr := formatDuration(float32(tElapsed.Seconds()))
	clock.SetText(tStr)
}
