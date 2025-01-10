package ui

import (
	"fmt"
	"image/color"
	"log/slog"
	"os"
	"strings"
	"time"

	"govdupes/internal/application"
	"govdupes/internal/models"
	"govdupes/internal/vm"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func CreateUI(appInstance *application.App, vm vm.ViewModel) {
	slog.Info("Starting CreateUI")

	// fyne app
	a := app.New()
	window := a.NewWindow("govdupes")

	duplicatesView := NewDuplicatesListView(vm)
	if duplicatesView == nil {
		slog.Error("Failed to create DuplicatesListView")
		os.Exit(1)
	}

	duplicatesTab := container.NewVScroll(duplicatesView)
	duplicatesTab.SetMinSize(fyne.NewSize(1200, 768))

	// Tabs
	themeTab := buildThemeTab(a)
	sortSelectTab := buildSortSelectDeleteTab(duplicatesView, vm)
	filterForm, checkWidget := buildFilter(duplicatesView)

	configTab := buildConfigTab(appInstance.Config, checkWidget)
	searchTab := buildSearchTab(appInstance, window, vm)

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

func buildSearchTab(appInstance *application.App, parent fyne.Window, vm vm.ViewModel) *fyne.Container {
	getInfoBar := widget.NewProgressBarWithData(vm.GetFileInfoProgressBind())
	getInfoLabel := widget.NewLabel("GetInfo Progress:")
	getInfoLabelBar := container.NewGridWithColumns(2, getInfoLabel, getInfoBar)

	genPHashesBar := widget.NewProgressBarWithData(vm.GetPHashesProgressBind())
	genPHashesLabel := widget.NewLabel("Generate PHashes Progress:")
	genPHashesLabelBar := container.NewGridWithColumns(2, genPHashesLabel, genPHashesBar)

	labelFileCount := widget.NewLabelWithData(vm.GetFileCountBind())
	labelAcceptedFiles := widget.NewLabelWithData(vm.GetAcceptedFilesBind())

	searchBtn := container.NewCenter(
		widget.NewButtonWithIcon("Search", theme.Icon(theme.IconNameSearch), func() {
			slog.Info("Search started!")

			clockWidget := widget.NewLabel("")
			d := dialog.NewCustomWithoutButtons(
				"Searching...",
				container.NewVBox(clockWidget, labelFileCount, labelAcceptedFiles,
					getInfoLabelBar, genPHashesLabelBar),
				parent,
			)

			c := clock{}
			c.set()
			d.Show()

			stopChan := make(chan struct{})
			go runClock(&c, clockWidget, stopChan)

			err := appInstance.Search(vm)
			if err != nil {
				slog.Error("Error calling search", "error", err)
			}

			close(stopChan)
			d.Hide()
		}),
	)

	searchTab := container.NewBorder(searchBtn, nil, nil, nil)
	return searchTab
}

// timer to show elapsed time in a label
type clock struct {
	t time.Time
}

func (c *clock) set() {
	c.t = time.Now()
}

func (c *clock) update(clockWidget *widget.Label) {
	tElapsed := time.Since(c.t)
	tStr := formatDuration(float32(tElapsed.Seconds()))
	clockWidget.SetText(fmt.Sprintf("Time elapsed: %s", tStr))
}

func runClock(c *clock, clockWidget *widget.Label, stopChan chan struct{}) {
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
}

// returns hh:mm:ss from seconds
func formatDuration(seconds float32) string {
	hours := int(seconds) / 3600
	mins := (int(seconds) % 3600) / 60
	secs := int(seconds) % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, mins, secs)
}

func buildFilter(duplicatesView *DuplicatesListView) (fyne.CanvasObject, *widget.Check) {
	filterEntry := widget.NewEntry()
	filterEntry.SetPlaceHolder("Enter path/file search...")

	filterButton := widget.NewButton("Apply", func() {
		slog.Info("Filter applied", slog.String("filter", filterEntry.Text))

		// parseSearchQuery: a helper that returns a viewmodel.SearchQuery
		query := parseSearchQuery(filterEntry.Text)

		duplicatesView.ApplyFilter(query)
	})

	filterForm := container.NewVBox(
		widget.NewLabel("Filter:"),
		filterEntry,
		filterButton,
	)

	filterForm.Hide()
	filterVisible := false

	checkWidget := widget.NewCheck("Show Filter Box", func(checked bool) {
		filterVisible = checked
		if filterVisible {
			filterForm.Show()
		} else {
			filterForm.Hide()
		}
		filterForm.Refresh()
	})

	return filterForm, checkWidget
}

// splits the user input into tokens, creating a single AND-group.
func parseSearchQuery(text string) models.SearchQuery {
	if text == "" {
		return models.SearchQuery{}
	}
	tokens := strings.Fields(text) // split on spaces
	return models.SearchQuery{
		OrGroups: [][]string{tokens},
	}
}

