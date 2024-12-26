package ui

import (
	"image/color"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"govdupes/internal/models"
)

func CreateUI(videoData [][]*models.VideoData) {
	log.Println("Starting CreateUI")

	a := app.New()
	log.Println("Fyne app initialized")

	// Create our custom DuplicatesList widget
	log.Println("Creating DuplicatesList widget")
	duplicatesListWidget := NewDuplicatesList(videoData)
	if duplicatesListWidget == nil {
		log.Fatal("Failed to create DuplicatesList widget")
	}

	// Wrap the DuplicatesList in a scroll container
	scroll := container.NewVScroll(duplicatesListWidget)
	scroll.SetMinSize(fyne.NewSize(1024, 768))

	// Create a tab for the duplicates view
	duplicatesTab := scroll

	// Create a second tab for switching themes
	themeTab := buildThemeTab(a)

	// Combine into an AppTabs container
	tabs := container.NewAppTabs(
		container.NewTabItem("Duplicates", duplicatesTab),
		container.NewTabItem("Theme", themeTab),
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

