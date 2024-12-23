package ui

import (
	"fmt"
	"image/color"
	"log"
	"strconv"

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

func buildThemeTab(a fyne.App) fyne.CanvasObject {
	themes := container.NewGridWithColumns(2,
		widget.NewButton("Dark", func() {
			a.Settings().SetTheme(&forcedVariant{Theme: theme.DefaultTheme(), isDark: true})
		}),
		widget.NewButton("Light", func() {
			a.Settings().SetTheme(&forcedVariant{Theme: theme.DefaultTheme(), isDark: false})
		}),
	)
	themeContainer := container.NewWithoutLayout(themes)
	themeContainer.Layout = &zeroLayout{}
	return themeContainer
}

func buildDuplicatesTab(videoData [][]*models.VideoData) fyne.CanvasObject {
	if len(videoData) == 0 {
		return widget.NewLabel("No duplicate videos found.")
	}

	mainVBox := container.NewVBox()

	// global header row (not repeated per group).
	header := container.New(layout.NewGridLayout(5),
		largeLabel("Screenshots"),
		largeLabel("Path"),
		largeLabel("Size\nBitrate\nFPS\nResolution\nDuration)"),
		largeLabel("Codecs\n(Audio / Video)"),
		largeLabel("Links\nSymbolic\nSymbolicPath\nHard\n#Hard"),
	)
	headerContainer := container.NewWithoutLayout(header)
	headerContainer.Layout = &zeroLayout{}
	mainVBox.Add(headerContainer)
	mainVBox.Add(widget.NewSeparator())

	for i, group := range videoData {
		if len(group) == 0 {
			continue
		}
		groupVBox := container.NewVBox()

		title := largeLabel(fmt.Sprintf("Group %d (Total %d duplicates)", i+1, len(group)))
		title.TextStyle.Bold = true
		groupVBox.Add(title)

		for _, vd := range group {
			if vd == nil {
				continue
			}
			row := newVideoRow(vd)
			groupVBox.Add(row)
		}

		groupVBox.Add(widget.NewLabel(""))
		groupVBox.Add(widget.NewSeparator())

		mainVBox.Add(groupVBox)
	}

	// Wrap everything in a scroll container
	outer := container.NewWithoutLayout(mainVBox)
	outer.Layout = &zeroLayout{}

	scrollable := container.NewVScroll(outer)
	scrollable.SetMinSize(fyne.NewSize(0, 0))

	return scrollable
}

func largeLabel(txt string) *canvas.Text {
	c := canvas.NewText(txt, color.Black)
	c.TextSize = 15
	return c
}

type zeroLayout struct{}

func (z *zeroLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		osize := o.MinSize()
		o.Move(fyne.NewPos(0, 0))
		o.Resize(osize)
	}
}

func (z *zeroLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objects {
		ms := o.MinSize()
		if ms.Width > w {
			w = ms.Width
		}
		if ms.Height > h {
			h = ms.Height
		}
	}
	return fyne.NewSize(w, h)
}

type videoRow struct {
	widget.BaseWidget
	vd         *models.VideoData
	isSelected bool
}

func newVideoRow(vd *models.VideoData) *videoRow {
	row := &videoRow{vd: vd}
	row.ExtendBaseWidget(row)
	return row
}

func (r *videoRow) Tapped(_ *fyne.PointEvent) {
	r.isSelected = !r.isSelected
	r.Refresh()
}
func (r *videoRow) TappedSecondary(_ *fyne.PointEvent) {}

func (r *videoRow) CreateRenderer() fyne.WidgetRenderer {
	log.Println("Screenshots MinSize:", r.buildScreenshotsColumn().MinSize())
	log.Println("Path MinSize:", r.buildPathColumn().MinSize())
	log.Println("Stats MinSize:", r.buildStatsColumn().MinSize())
	log.Println("Codecs MinSize:", r.buildCodecsColumn().MinSize())
	log.Println("Links MinSize:", r.buildLinksColumn().MinSize())

	columns := container.New(layout.NewGridLayout(5),
		r.buildScreenshotsColumn(),
		r.buildPathColumn(),
		r.buildStatsColumn(),
		r.buildCodecsColumn(),
		r.buildLinksColumn(),
	)

	// Transparent background rectangle for selection highlight
	bg := canvas.NewRectangle(color.RGBA{0, 0, 0, 0})

	overlay := container.NewWithoutLayout(bg, columns)
	overlay.Layout = &rowOverlayLayout{}

	return &videoRowRenderer{
		row:        r,
		background: bg,
		content:    columns,
		overlay:    overlay,
		objects:    []fyne.CanvasObject{bg, columns},
	}
}

type rowOverlayLayout struct{}

func (l *rowOverlayLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) != 2 {
		return
	}
	bg := objs[0]
	cols := objs[1]

	log.Println(size)
	bg.Resize(size)
	bg.Move(fyne.NewPos(0, 0))

	log.Println(cols.MinSize())
	cols.Resize(cols.MinSize())
	cols.Move(fyne.NewPos(0, 0))
}

func (l *rowOverlayLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	if len(objs) < 2 {
		return fyne.NewSize(0, 0)
	}
	return objs[1].MinSize()
}

type videoRowRenderer struct {
	row        *videoRow
	background *canvas.Rectangle
	content    fyne.CanvasObject
	overlay    fyne.CanvasObject
	objects    []fyne.CanvasObject
}

func (r *videoRowRenderer) Layout(size fyne.Size) {
	r.overlay.Resize(size)
}

func (r *videoRowRenderer) MinSize() fyne.Size {
	return r.overlay.MinSize()
}

func (r *videoRowRenderer) Refresh() {
	if r.row.isSelected {
		// Dark partially transparent blue
		r.background.FillColor = color.RGBA{R: 50, G: 50, B: 255, A: 80}
	} else {
		r.background.FillColor = color.RGBA{0, 0, 0, 0}
	}
	r.background.Refresh()
	r.content.Refresh()
}
func (r *videoRowRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *videoRowRenderer) Destroy()                     {}

func (r *videoRow) buildScreenshotsColumn() fyne.CanvasObject {
	imgs := r.vd.Screenshot.Screenshots
	if len(imgs) == 0 {
		return largeLabel("No screenshots")
	}

	var imageWidgets []fyne.CanvasObject
	for _, img := range imgs {
		fImg := canvas.NewImageFromImage(img)
		fImg.FillMode = canvas.ImageFillContain
		fImg.SetMinSize(fyne.NewSize(60, 40))
		imageWidgets = append(imageWidgets, fImg)
	}

	return container.New(&NoSpaceHBox{}, imageWidgets...)
}

func (r *videoRow) buildPathColumn() fyne.CanvasObject {
	txt := canvas.NewText(r.vd.Video.Path, color.Black)
	txt.TextSize = 14
	return txt
}

func (r *videoRow) buildStatsColumn() fyne.CanvasObject {
	v := r.vd.Video

	sizeMB := float64(v.Size) / (1024 * 1024)
	var sizeStr string
	sizeStr = fmt.Sprintf("%.2f MB", sizeMB)
	if sizeMB >= 1024 {
		sizeGB := sizeMB / 1024
		sizeStr = fmt.Sprintf("%.2f GB", sizeGB)
	}
	line1 := canvas.NewText(sizeStr, color.Black)

	bitrateMbps := (float64(v.BitRate) / (1024 * 1024)) * 8
	bitRate := fmt.Sprintf("%.2f Mbps", bitrateMbps)
	line2 := canvas.NewText(bitRate, color.Black)

	log.Println(v.FrameRate)
	fps := fmt.Sprintf("%.2f fps", v.FrameRate)
	line3 := canvas.NewText(fps, color.Black)

	resolution := fmt.Sprintf("%dx%d", v.Width, v.Height)
	line4 := canvas.NewText(resolution, color.Black)

	totalSec := int(v.Duration)
	hh := totalSec / 3600
	mm := (totalSec % 3600) / 60
	ss := totalSec % 60
	dur := fmt.Sprintf("%02d:%02d:%02d", hh, mm, ss)
	line5 := canvas.NewText(dur, color.Black)

	txt := container.NewVBox(
		line1,
		line2,
		line3,
		line4,
		line5,
	)
	return txt
}

func (r *videoRow) buildCodecsColumn() fyne.CanvasObject {
	v := r.vd.Video
	txt := fmt.Sprintf("%s / %s", v.AudioCodec, v.VideoCodec)
	c := canvas.NewText(txt, color.Black)
	c.TextSize = 14
	return c
}

// Show symbolics/hardlinks in a partially transparent colored column
func (r *videoRow) buildLinksColumn() fyne.CanvasObject {
	v := r.vd.Video
	nhl := strconv.FormatUint(v.NumHardLinks, 10)

	txt := fmt.Sprintf("%t / %s\n%t / %s",
		v.IsSymbolicLink, v.SymbolicLink,
		v.IsHardLink, nhl,
	)
	lbl := canvas.NewText(txt, color.Black)
	lbl.TextSize = 14

	bgCol := color.RGBA{0, 0, 0, 0}
	switch {
	case v.IsHardLink:
		// Dark red
		bgCol = color.RGBA{R: 180, G: 0, B: 0, A: 100}
	case v.IsSymbolicLink:
		// Dark green
		bgCol = color.RGBA{R: 0, G: 180, B: 0, A: 100}
	}

	bgRect := canvas.NewRectangle(bgCol)
	cont := container.NewWithoutLayout(bgRect, lbl)
	cont.Layout = &columnOverlayLayout{}
	return cont
}

// columnOverlayLayout places the background behind the text, no spacing
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

