package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"govdupes/internal/models"
)

type forcedVariant struct {
	fyne.Theme
	isDark bool
}

func (f *forcedVariant) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if f.isDark {
		v = theme.VariantDark
	} else {
		v = theme.VariantLight
	}
	return f.Theme.Color(n, v)
}

// CreateUI is your main entry point that builds two tabs: "Duplicates" and "Theme".
func CreateUI(a fyne.App, videoData [][]*models.VideoData) fyne.CanvasObject {
	duplicatesTab := buildDuplicatesTab(videoData)
	themeTab := buildThemeTab(a)

	tabs := container.NewAppTabs(
		container.NewTabItem("Duplicates", duplicatesTab),
		container.NewTabItem("Theme", themeTab),
	)
	tabs.SetTabLocation(container.TabLocationTop)
	return tabs
}

// A tab that lets the user switch between dark/light themes
func buildThemeTab(a fyne.App) fyne.CanvasObject {
	themes := container.NewGridWithColumns(2,
		widget.NewButton("Dark", func() {
			a.Settings().SetTheme(&forcedVariant{Theme: theme.DefaultTheme(), isDark: true})
		}),
		widget.NewButton("Light", func() {
			a.Settings().SetTheme(&forcedVariant{Theme: theme.DefaultTheme(), isDark: false})
		}),
	)
	return themes
}

// ---------------------------------------------------------------------------
// Build a single table of all data using one GridLayout(5) so that
// header columns line up perfectly with data columns.
// ---------------------------------------------------------------------------
func buildDuplicatesTab(videoData [][]*models.VideoData) fyne.CanvasObject {
	if len(videoData) == 0 {
		return widget.NewLabel("No duplicate videos found.")
	}

	mainVBox := container.NewVBox()

	// Header Row
	header := container.New(layout.NewGridLayout(5),
		leftAlignedText("Screenshots"),
		leftAlignedText("Path"),
		container.NewVBox(
			leftAlignedText("Size"),
			leftAlignedText("Bitrate"),
			leftAlignedText("FPS"),
			leftAlignedText("Resolution"),
			leftAlignedText("Duration"),
		),
		container.NewVBox(
			leftAlignedText("Codecs"),
			leftAlignedText("(Audio / Video)"),
		),
		container.NewVBox(
			leftAlignedText("Links"),
			leftAlignedText("Symbolic"),
			leftAlignedText("SymbolicPath"),
			leftAlignedText("Hard"),
			leftAlignedText("#Hard"),
		),
	)
	headerContainer := container.NewPadded(header)
	headerContainer = container.NewVBox(header)
	headerContainer.Objects = []fyne.CanvasObject{header} // Ensure it contains the header
	headerContainer.Refresh()                             // Apply updates

	mainVBox.Add(headerContainer)
	mainVBox.Add(widget.NewSeparator())

	// Grouped Data Rows
	for i, group := range videoData {
		if len(group) == 0 {
			continue
		}

		groupTitle := canvas.NewText(fmt.Sprintf("Group %d (Total %d duplicates)", i+1, len(group)), color.Black)
		groupTitle.TextSize = 16
		groupTitle.TextStyle = fyne.TextStyle{Bold: true}
		groupContainer := container.NewVBox(container.NewPadded(groupTitle))

		for _, vd := range group {
			if vd == nil {
				continue
			}
			row := buildDataRow(vd)
			groupContainer.Add(row)
		}

		// Add group to main layout
		mainVBox.Add(container.NewVBox(
			widget.NewSeparator(),
			groupContainer,
			widget.NewSeparator(),
		))
	}

	// Scrollable Container
	scrollable := container.NewVScroll(mainVBox)
	scrollable.SetMinSize(fyne.NewSize(0, 0))
	return scrollable
}

type rowOverlayLayout struct{}

func (l *rowOverlayLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 2 {
		return
	}
	bg := objects[0]
	row := objects[1]

	bg.Resize(size)
	bg.Move(fyne.NewPos(0, 0))
	row.Resize(size)
	row.Move(fyne.NewPos(0, 0))
}

func (l *rowOverlayLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) < 2 {
		return fyne.NewSize(0, 0)
	}
	return objects[1].MinSize()
}

type tappableRow struct {
	widget.BaseWidget
	row      fyne.CanvasObject
	bg       *canvas.Rectangle
	isTapped bool
}

func newTappableRow(row fyne.CanvasObject) *tappableRow {
	bg := canvas.NewRectangle(color.Transparent)
	w := &tappableRow{row: row, bg: bg}
	w.ExtendBaseWidget(w)
	return w
}

func (w *tappableRow) CreateRenderer() fyne.WidgetRenderer {
	overlay := container.NewWithoutLayout(w.bg, w.row)
	return &tappableRowRenderer{row: w, content: overlay}
}

type tappableRowRenderer struct {
	row     *tappableRow
	content *fyne.Container
}

func (r *tappableRowRenderer) Layout(size fyne.Size) {
	r.row.bg.Resize(size)
	r.row.row.Resize(size)
}

func (r *tappableRowRenderer) MinSize() fyne.Size {
	return r.row.row.MinSize()
}

func (r *tappableRowRenderer) Refresh() {
	r.row.bg.FillColor = color.Transparent
	if r.row.isTapped {
		r.row.bg.FillColor = color.RGBA{50, 150, 255, 80} // Light transparent blue
	}
	r.row.bg.Refresh()
}

func (r *tappableRowRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.row.bg, r.row.row}
}

func (r *tappableRowRenderer) Destroy() {}

func (w *tappableRow) Tapped(_ *fyne.PointEvent) {
	w.isTapped = !w.isTapped
	w.Refresh()
}

func (w *tappableRow) TappedSecondary(_ *fyne.PointEvent) {}

// Builds a single row of data, with click highlighting

func buildDataRow(vd *models.VideoData) fyne.CanvasObject {
	columns := container.New(layout.NewGridLayout(5),
		buildScreenshotsColumn(vd),
		buildPathColumn(vd),
		buildStatsColumn(vd),
		buildCodecsColumn(vd),
		buildLinksColumn(vd),
	)

	// Use tappableRow for the clickable functionality
	row := newTappableRow(columns)
	return row
}

// Column Builders
func buildScreenshotsColumn(vd *models.VideoData) fyne.CanvasObject {
	imgs := vd.Screenshot.Screenshots
	if len(imgs) == 0 {
		return leftAlignedText("No screenshots")
	}

	var imageWidgets []fyne.CanvasObject
	for _, img := range imgs {
		fImg := canvas.NewImageFromImage(img)
		fImg.FillMode = canvas.ImageFillContain
		fImg.SetMinSize(fyne.NewSize(64, 64))
		imageWidgets = append(imageWidgets, fImg)
	}

	return container.New(&NoSpaceHBox{}, imageWidgets...)
}

func buildPathColumn(vd *models.VideoData) fyne.CanvasObject {
	return leftAlignedText(vd.Video.Path)
}

func buildStatsColumn(vd *models.VideoData) fyne.CanvasObject {
	v := vd.Video
	totalSec := int(v.Duration) // Cast to int for modulo operations
	hh := totalSec / 3600
	mm := (totalSec % 3600) / 60
	ss := totalSec % 60

	stats := container.NewVBox(
		leftAlignedText(fmt.Sprintf("%.2f MB", float64(v.Size)/(1024*1024))),
		leftAlignedText(fmt.Sprintf("%.2f Mbps", (float64(v.BitRate)/(1024*1024))*8)),
		leftAlignedText(fmt.Sprintf("%.2f fps", v.FrameRate)),
		leftAlignedText(fmt.Sprintf("%dx%d", v.Width, v.Height)),
		leftAlignedText(fmt.Sprintf("%02d:%02d:%02d", hh, mm, ss)),
	)
	return stats
}

func buildCodecsColumn(vd *models.VideoData) fyne.CanvasObject {
	v := vd.Video
	return leftAlignedText(fmt.Sprintf("%s / %s", v.AudioCodec, v.VideoCodec))
}

func buildLinksColumn(vd *models.VideoData) fyne.CanvasObject {
	v := vd.Video
	links := container.NewVBox(
		leftAlignedText(fmt.Sprintf("Symbolic: %t", v.IsSymbolicLink)),
		leftAlignedText(fmt.Sprintf("SymbolicPath: %s", v.SymbolicLink)),
		leftAlignedText(fmt.Sprintf("Hard: %t", v.IsHardLink)),
		leftAlignedText(fmt.Sprintf("#Hard: %d", v.NumHardLinks)),
	)
	return links
}

// ---------------------------------------------------------------------------
// Helper Functions
// ---------------------------------------------------------------------------
func largeLabel(txt string) *canvas.Text {
	c := canvas.NewText(txt, color.Black)
	c.TextSize = 15
	return c
}

// A simple left-aligned text
func leftAlignedText(text string) *canvas.Text {
	txt := canvas.NewText(text, color.Black)
	txt.TextSize = 14
	txt.Alignment = fyne.TextAlignLeading
	return txt
}

// The same columnOverlayLayout as before
type columnOverlayLayout struct{}

func (l *columnOverlayLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) != 2 {
		return
	}
	bg := objs[0]
	txt := objs[1]

	bg.Resize(size)
	bg.Move(fyne.NewPos(0, 0))

	minSize := txt.MinSize()
	txt.Move(fyne.NewPos(0, 0))
	txt.Resize(minSize)
}

func (l *columnOverlayLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	if len(objs) != 2 {
		return fyne.NewSize(0, 0)
	}
	return objs[1].MinSize()
}

// NoSpaceHBox: a layout that places children horizontally at x offsets
// with zero spacing.
type NoSpaceHBox struct{}

func (l *NoSpaceHBox) Layout(objects []fyne.CanvasObject, containerSize fyne.Size) {
	x := float32(0)
	for _, o := range objects {
		objSize := o.MinSize()
		o.Resize(objSize)
		o.Move(fyne.NewPos(x, 0))
		x += objSize.Width
	}
}

func (l *NoSpaceHBox) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w float32
	var h float32
	for _, o := range objects {
		sz := o.MinSize()
		w += sz.Width
		if sz.Height > h {
			h = sz.Height
		}
	}
	return fyne.NewSize(w, h)
}
