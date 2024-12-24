package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type DuplicatesListRow struct {
	widget.BaseWidget

	headerLabel         *widget.Label
	videoLayout         *fyne.Container
	screenshotContainer *fyne.Container
	pathLabel           *widget.Label
	statsContainer      *fyne.Container
	codecsLabel         *widget.Label
	linksContainer      *fyne.Container

	isHeader bool
	selected bool
}

func NewDuplicatesListRow() *DuplicatesListRow {
	row := &DuplicatesListRow{
		headerLabel:         widget.NewLabel(""),
		screenshotContainer: container.NewHBox(), // Horizontal layout for screenshots
		pathLabel:           widget.NewLabel(""),
		statsContainer:      container.NewVBox(), // Use VBox for compact vertical alignment
		codecsLabel:         widget.NewLabel(""),
		linksContainer:      container.NewVBox(), // Use VBox for compact vertical alignment
	}

	// Grid layout with 5 columns for consistent alignment
	row.videoLayout = container.New(layout.NewGridLayoutWithColumns(5),
		row.screenshotContainer,
		row.pathLabel,
		row.statsContainer,
		row.codecsLabel,
		row.linksContainer,
	)

	row.ExtendBaseWidget(row)
	return row
}

func (r *DuplicatesListRow) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(r.backgroundColor())
	overlay := container.NewStack(r.headerLabel, r.videoLayout)
	c := container.NewStack(bg, overlay)

	return &duplicatesListRowRenderer{
		row:          r,
		background:   bg,
		containerAll: c,
	}
}

func (r *DuplicatesListRow) backgroundColor() color.Color {
	if r.selected {
		return color.RGBA{R: 173, G: 216, B: 230, A: 128}
	}
	return color.RGBA{0, 0, 0, 0}
}

func (r *DuplicatesListRow) Update(item duplicateListItem) {
	r.isHeader = item.IsHeader
	r.selected = item.Selected

	if item.IsHeader {
		r.headerLabel.SetText(item.HeaderText)
		r.headerLabel.Show()
		r.videoLayout.Hide()
		return
	}

	r.headerLabel.Hide()
	r.videoLayout.Show()

	vd := item.VideoData
	if vd == nil {
		r.pathLabel.SetText("(no data)")
		return
	}

	// Update screenshot container
	r.screenshotContainer.Objects = nil
	if len(vd.Screenshot.Screenshots) == 0 {
		r.screenshotContainer.Add(widget.NewLabel("No screenshots"))
	} else {
		for _, img := range vd.Screenshot.Screenshots {
			fImg := canvas.NewImageFromImage(img)
			fImg.FillMode = canvas.ImageFillContain
			fImg.SetMinSize(fyne.NewSize(100, 100))
			r.screenshotContainer.Add(fImg)
		}
	}

	// Path
	r.pathLabel.SetText(vd.Video.Path)

	// Stats with reduced spacing
	r.statsContainer.Objects = []fyne.CanvasObject{
		widget.NewLabel(fmt.Sprintf("%.2f MB", float64(vd.Video.Size)/(1024.0*1024.0))),
		widget.NewLabel(fmt.Sprintf("%.2f Mbps", float64(vd.Video.BitRate)/1024.0/1024.0*8.0)),
		widget.NewLabel(fmt.Sprintf("%.2f fps", vd.Video.FrameRate)),
		widget.NewLabel(fmt.Sprintf("%dx%d", vd.Video.Width, vd.Video.Height)),
		widget.NewLabel(fmt.Sprintf("%02d:%02d:%02d", int(vd.Video.Duration)/3600, (int(vd.Video.Duration)%3600)/60, int(vd.Video.Duration)%60)),
	}

	// Codecs
	r.codecsLabel.SetText(fmt.Sprintf("%s / %s", vd.Video.AudioCodec, vd.Video.VideoCodec))

	// Links with reduced spacing
	r.linksContainer.Objects = []fyne.CanvasObject{
		widget.NewLabel(fmt.Sprintf("Symbolic? %t", vd.Video.IsSymbolicLink)),
		widget.NewLabel(fmt.Sprintf("Link: %q", vd.Video.SymbolicLink)),
		widget.NewLabel(fmt.Sprintf("Hard? %t", vd.Video.IsHardLink)),
		widget.NewLabel(fmt.Sprintf("Count: %d", vd.Video.NumHardLinks-1)),
	}

	r.Refresh()
}

type duplicatesListRowRenderer struct {
	row          *DuplicatesListRow
	background   *canvas.Rectangle
	containerAll *fyne.Container
}

func (r *duplicatesListRowRenderer) Destroy() {}

func (r *duplicatesListRowRenderer) Layout(size fyne.Size) {
	r.background.Resize(size)
	r.containerAll.Resize(size)
}

func (r *duplicatesListRowRenderer) MinSize() fyne.Size {
	minSize := r.containerAll.MinSize()
	if minSize.Height < 120 {
		minSize.Height = 120
	}
	return minSize
}

func (r *duplicatesListRowRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.background, r.containerAll}
}

func (r *duplicatesListRowRenderer) Refresh() {
	r.background.FillColor = r.row.backgroundColor()
	r.background.Refresh()
	r.containerAll.Refresh()
}

